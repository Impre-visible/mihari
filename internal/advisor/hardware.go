// Package advisor recommends models that fit this machine, from a built-in
// catalog and the real file sizes on Hugging Face.
package advisor

import (
	"context"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const gb = 1 << 30

type GPU struct {
	Name string `json:"name"`
	VRAM int64  `json:"vram"`
	// Memory bandwidth in GB/s; 0 when the card is not in the table.
	Bandwidth float64 `json:"bandwidth"`
}

type Hardware struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	// Machine identifier, such as Mac16,8 on a Mac; "" elsewhere.
	Model string `json:"model"`
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
		h.Model = sysctl(ctx, "hw.model")
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
		h.GPUs = append(nvidiaGPUs(ctx), otherGPUs(ctx)...)
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
		gpus = append(gpus, GPU{Name: name, VRAM: n << 20, Bandwidth: nvidiaBandwidth(name, n<<20)})
	}
	return gpus
}

// Memory bandwidth in GB/s of NVIDIA cards, by the name nvidia-smi gives,
// most specific names first. A name sold with several amounts of VRAM, or
// several memories, has an entry per size or else the slowest of them.
// Checked against the makers' specifications in October 2026.
var nvidiaTable = []struct {
	name string
	// VRAM in GB the entry is for; 0 for any.
	vram int64
	bw   float64
}{
	{"RTX PRO 6000", 0, 1792},
	{"RTX 5090", 0, 1792}, {"RTX 5080", 0, 960}, {"RTX 5070 Ti", 0, 896}, {"RTX 5070", 0, 672},
	{"RTX 5060 Ti", 0, 448}, {"RTX 5060", 0, 448},
	// The 4090 D has the same memory; the 4080 SUPER is faster (736); some 4070 have GDDR6 (480).
	{"RTX 4090", 0, 1008}, {"RTX 4080", 0, 717}, {"RTX 4070 Ti SUPER", 0, 672}, {"RTX 4070 Ti", 0, 504},
	{"RTX 4070", 0, 480}, {"RTX 4060 Ti", 0, 288}, {"RTX 4060", 0, 272},
	{"RTX 3090 Ti", 0, 1008}, {"RTX 3090", 0, 936}, {"RTX 3080 Ti", 0, 912}, {"RTX 3080", 12, 912}, {"RTX 3080", 0, 760},
	{"RTX 3070 Ti", 0, 608}, {"RTX 3070", 0, 448}, {"RTX 3060 Ti", 0, 448}, {"RTX 3060", 12, 360}, {"RTX 3060", 8, 240},
	{"RTX 3050", 8, 224}, {"RTX 3050", 6, 168},
	{"RTX 2080 Ti", 0, 616}, {"RTX 2080 SUPER", 0, 496}, {"RTX 2080", 0, 448}, {"RTX 2070", 0, 448},
	{"RTX 2060 SUPER", 0, 448}, {"RTX 2060", 0, 336},
	{"GTX 1660 Ti", 0, 288}, {"GTX 1660 SUPER", 0, 336}, {"GTX 1660", 0, 192},
	{"GTX 1080 Ti", 0, 484}, {"GTX 1080", 0, 320}, {"GTX 1070", 0, 256},
	{"RTX 6000 Ada", 0, 960}, {"RTX 5000 Ada", 0, 576}, {"RTX A6000", 0, 768}, {"RTX A5000", 0, 768}, {"RTX A4000", 0, 448},
	{"L40S", 0, 864}, {"L40", 0, 864}, {"L4", 0, 300}, {"A10", 0, 600},
	{"A100", 40, 1555}, {"A100", 0, 1935},
	// SXM is named after its HBM3; the PCIe card has HBM2e.
	{"H100 NVL", 0, 3900}, {"H100 PCIe", 0, 2000}, {"H100", 0, 3350}, {"H200", 0, 4800},
}

// nvidiaBandwidth returns 0 for a card not in the table, and for laptop
// GPUs, which share the desktop names with far slower memory.
func nvidiaBandwidth(name string, vram int64) float64 {
	upper := strings.ToUpper(name)
	if strings.Contains(upper, "LAPTOP") || strings.Contains(upper, "WITH MAX-Q") {
		return 0
	}
	size := int64(math.Round(float64(vram) / gb))
	for _, e := range nvidiaTable {
		if (e.vram == 0 || e.vram == size) && containsWord(upper, strings.ToUpper(e.name)) {
			return e.bw
		}
	}
	return 0
}

// containsWord reports whether name holds word not followed by a letter or
// digit, so that A10 does not match an A100, nor L4 an L40S.
func containsWord(name, word string) bool {
	for i := 0; ; i++ {
		at := strings.Index(name[i:], word)
		if at < 0 {
			return false
		}
		end := i + at + len(word)
		if end == len(name) || !unicode.IsLetter(rune(name[end])) && !unicode.IsDigit(rune(name[end])) {
			return true
		}
		i += at
	}
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
