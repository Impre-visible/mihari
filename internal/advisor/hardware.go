// Package advisor recommends models that fit this machine, from a built-in
// catalog and the real file sizes on Hugging Face.
package advisor

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const gb = 1 << 30

type GPU struct {
	Name string `json:"name"`
	VRAM int64  `json:"vram"`
	// Memory bandwidth in GB/s; 0 when the card is not in the table.
	Bandwidth float64 `json:"bandwidth"`
}

type Hardware struct {
	OS    string `json:"os"`
	Arch  string `json:"arch"`
	CPU   string `json:"cpu"`
	Cores int    `json:"cores"`
	RAM   int64  `json:"ram"`
	// Apple Silicon: the GPU uses the system memory, at the chip's bandwidth.
	Unified bool  `json:"unified"`
	GPUs    []GPU `json:"gpus"`
	// Bandwidth of the system memory in GB/s, for layers that spill out of the GPU.
	RAMBandwidth float64 `json:"ramBandwidth"`
}

// AppleSilicon reports a Mac with an M-series chip, where MLX runs.
func (h Hardware) AppleSilicon() bool {
	return h.OS == "darwin" && h.Arch == "arm64"
}

func (h Hardware) VRAM() int64 {
	var total int64
	for _, g := range h.GPUs {
		total += g.VRAM
	}
	return total
}

// Detect reads the machine's memory, CPU and GPUs.
func Detect(ctx context.Context) Hardware {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	h := Hardware{OS: runtime.GOOS, Arch: runtime.GOARCH, Cores: runtime.NumCPU(), RAMBandwidth: 50}
	switch runtime.GOOS {
	case "darwin":
		h.RAM, _ = strconv.ParseInt(sysctl(ctx, "hw.memsize"), 10, 64)
		h.CPU = sysctl(ctx, "machdep.cpu.brand_string")
		if h.AppleSilicon() {
			h.Unified = true
			h.RAMBandwidth = appleBandwidth(h.CPU)
		}
	case "linux":
		h.RAM = meminfoTotal()
		h.CPU = cpuinfoModel()
	case "windows":
		out, _ := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
			"(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory; (Get-CimInstance Win32_Processor).Name").Output()
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 {
			h.RAM, _ = strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
		}
		if len(lines) > 1 {
			h.CPU = strings.TrimSpace(lines[1])
		}
	}
	if !h.Unified {
		h.GPUs = nvidiaGPUs(ctx)
	}
	return h
}

func sysctl(ctx context.Context, name string) string {
	out, _ := exec.CommandContext(ctx, "sysctl", "-n", name).Output()
	return strings.TrimSpace(string(out))
}

func meminfoTotal() int64 {
	data, _ := os.ReadFile("/proc/meminfo")
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			kb, _ := strconv.ParseInt(strings.Fields(line)[1], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

func cpuinfoModel() string {
	data, _ := os.ReadFile("/proc/cpuinfo")
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "model name") {
			if _, v, ok := strings.Cut(line, ":"); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func nvidiaGPUs(ctx context.Context) []GPU {
	out, err := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil
	}
	var gpus []GPU
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, mib, ok := strings.Cut(line, ",")
		if !ok {
			continue
		}
		n, _ := strconv.ParseInt(strings.TrimSpace(mib), 10, 64)
		name = strings.TrimSpace(name)
		gpus = append(gpus, GPU{Name: name, VRAM: n << 20, Bandwidth: nvidiaBandwidth(name)})
	}
	return gpus
}

// Memory bandwidth in GB/s of common NVIDIA cards, most specific names first.
var nvidiaTable = []struct {
	name string
	bw   float64
}{
	{"RTX 5090", 1792}, {"RTX 5080", 960}, {"RTX 5070 Ti", 896}, {"RTX 5070", 672}, {"RTX 5060 Ti", 448}, {"RTX 5060", 448},
	{"RTX 4090", 1008}, {"RTX 4080", 717}, {"RTX 4070 Ti", 504}, {"RTX 4070", 504}, {"RTX 4060 Ti", 288}, {"RTX 4060", 272},
	{"RTX 3090", 936}, {"RTX 3080 Ti", 912}, {"RTX 3080", 760}, {"RTX 3070", 448}, {"RTX 3060 Ti", 448}, {"RTX 3060", 360},
	{"RTX 2080 Ti", 616}, {"RTX 2080", 448}, {"RTX 2070", 448}, {"RTX 2060", 336},
	{"GTX 1660 Ti", 288}, {"GTX 1660 SUPER", 336}, {"GTX 1660", 192}, {"GTX 1080 Ti", 484}, {"GTX 1080", 320}, {"GTX 1070", 256},
	{"RTX A6000", 768}, {"RTX 6000 Ada", 960}, {"L40S", 864}, {"L4", 300},
	{"A100", 1935}, {"H100", 3350}, {"H200", 4800}, {"A10", 600},
}

func nvidiaBandwidth(name string) float64 {
	upper := strings.ToUpper(name)
	for _, e := range nvidiaTable {
		if strings.Contains(upper, strings.ToUpper(e.name)) {
			return e.bw
		}
	}
	return 0
}

// Unified memory bandwidth in GB/s of Apple chips, most specific names first.
var appleTable = []struct {
	name string
	bw   float64
}{
	{"M4 Max", 546}, {"M4 Pro", 273}, {"M4", 120},
	{"M3 Ultra", 819}, {"M3 Max", 400}, {"M3 Pro", 150}, {"M3", 100},
	{"M2 Ultra", 800}, {"M2 Max", 400}, {"M2 Pro", 200}, {"M2", 100},
	{"M1 Ultra", 800}, {"M1 Max", 400}, {"M1 Pro", 200}, {"M1", 68},
}

func appleBandwidth(chip string) float64 {
	for _, e := range appleTable {
		if strings.Contains(chip, e.name) {
			return e.bw
		}
	}
	// A chip newer than the table: at least a base M-series.
	return 120
}
