package advisor

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// AMD and Intel cards llama.cpp runs on through Vulkan, by PCI id and memory
// size: a chip often ships with several amounts of VRAM, and the size tells
// them apart. Where cards share an id and a size, as desktop and laptop
// versions do, the entry has the slowest memory of them. Cards not listed,
// integrated GPUs among them, are left out rather than guessed at.
//
// Ids from pci.ids (pciutils), bandwidths from the makers' specifications,
// checked in October 2026.
var pciTable = []struct {
	id string
	// VRAM in GB, as the card is sold.
	vram int64
	name string
	// Memory bandwidth in GB/s.
	bw float64
}{
	// AMD RDNA 4
	{"1002:7550", 12, "AMD Radeon RX 9070 GRE", 432},
	{"1002:7550", 16, "AMD Radeon RX 9070 / 9070 XT", 640},
	{"1002:7551", 32, "AMD Radeon AI PRO R9700", 640},
	// The RX 9050 8 GB has slower memory than the 9060 XT 8 GB (288 against 320).
	{"1002:7590", 8, "AMD Radeon RX 9050 / 9060 XT", 288},
	{"1002:7590", 16, "AMD Radeon RX 9060 XT", 320},
	// AMD RDNA 3
	{"1002:7448", 48, "AMD Radeon Pro W7900", 864},
	{"1002:7449", 48, "AMD Radeon Pro W7800 48GB", 864},
	{"1002:744c", 16, "AMD Radeon RX 7900 GRE / 7900M", 576},
	{"1002:744c", 20, "AMD Radeon RX 7900 XT", 800},
	{"1002:744c", 24, "AMD Radeon RX 7900 XTX", 960},
	{"1002:747e", 12, "AMD Radeon RX 7700 XT", 432},
	{"1002:747e", 16, "AMD Radeon RX 7800 XT", 624},
	// The 7600S runs its memory at 16 Gbps, the others at 18.
	{"1002:7480", 8, "AMD Radeon RX 7600 / 7600S / 7700S / Pro W7600", 256},
	{"1002:7480", 16, "AMD Radeon RX 7600 XT", 288},
	// AMD RDNA 2
	{"1002:73a5", 16, "AMD Radeon RX 6950 XT", 576},
	{"1002:73af", 16, "AMD Radeon RX 6900 XT", 512},
	{"1002:73bf", 16, "AMD Radeon RX 6800 / 6800 XT / 6900 XT", 512},
	{"1002:73df", 10, "AMD Radeon RX 6700", 320},
	{"1002:73df", 12, "AMD Radeon RX 6700 XT / 6750 XT / 6800M / 6850M XT", 384},
	// The 6700S runs its memory at 14 Gbps, the 6650 XT at 17.5.
	{"1002:73ef", 8, "AMD Radeon RX 6650 XT / 6700S / 6800S", 224},
	{"1002:73ff", 8, "AMD Radeon RX 6600 / 6600 XT / 6600M", 224},
	// Intel Arc B
	{"8086:e20b", 12, "Intel Arc B580", 456},
	{"8086:e20c", 10, "Intel Arc B570", 380},
	{"8086:e211", 24, "Intel Arc Pro B60", 456},
	{"8086:e212", 16, "Intel Arc Pro B50", 224},
	{"8086:e223", 32, "Intel Arc Pro B70", 608},
	// Intel Arc A; the smaller A770 first, taken when the size cannot be read.
	{"8086:56a0", 8, "Intel Arc A770", 512},
	{"8086:56a0", 16, "Intel Arc A770", 560},
	{"8086:56a1", 8, "Intel Arc A750", 512},
	{"8086:56a2", 8, "Intel Arc A580", 512},
	{"8086:56a5", 6, "Intel Arc A380", 186},
	{"8086:56a6", 4, "Intel Arc A310", 124},
}

// pciGPU returns the card with this PCI id and VRAM in bytes, 0 when unknown.
func pciGPU(id string, vram int64) (GPU, bool) {
	id = strings.ToLower(id)
	size := int64(math.Round(float64(vram) / gb))
	for _, e := range pciTable {
		if e.id == id && (vram == 0 || e.vram == size) {
			return GPU{Name: e.name, VRAM: e.vram * gb, Bandwidth: e.bw}, true
		}
	}
	return GPU{}, false
}

// otherGPUs lists the AMD and Intel cards; NVIDIA ones come from nvidia-smi.
func otherGPUs(ctx context.Context) []GPU {
	switch runtime.GOOS {
	case "linux":
		return sysfsGPUs("/sys/class/drm")
	case "windows":
		out, _ := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", windowsGPUQuery).Output()
		return windowsGPUs(string(out))
	}
	return nil
}

// sysfsGPUs reads each card's PCI id and, where the driver tells, its VRAM:
// amdgpu always does, Intel's drivers not always.
func sysfsGPUs(root string) []GPU {
	cards, _ := filepath.Glob(filepath.Join(root, "card[0-9]*"))
	var gpus []GPU
	for _, card := range cards {
		if strings.Contains(filepath.Base(card), "-") {
			// A connector, such as card0-HDMI-A-1.
			continue
		}
		read := func(name string) string {
			data, _ := os.ReadFile(filepath.Join(card, name))
			return strings.TrimPrefix(strings.TrimSpace(string(data)), "0x")
		}
		vendor, device := read("device/vendor"), read("device/device")
		if vendor == "" || vendor == "10de" {
			continue
		}
		var vram int64
		for _, name := range []string{"device/mem_info_vram_total", "lmem_total_bytes", "device/tile0/vram0/physical_vram_size_bytes"} {
			if v, err := strconv.ParseInt(read(name), 0, 64); err == nil && v > 0 {
				vram = v
				break
			}
		}
		if g, ok := pciGPU(vendor+":"+device, vram); ok {
			gpus = append(gpus, g)
		}
	}
	return gpus
}

// The display adapters present, with the memory size their driver records:
// Win32_VideoController alone stops at 4 GB, and the registry keeps the
// entries of cards long removed.
const windowsGPUQuery = `Get-CimInstance Win32_VideoController | ForEach-Object { $key = (Get-PnpDeviceProperty -InstanceId $_.PNPDeviceID -KeyName DEVPKEY_Device_Driver).Data; $size = (Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Control\Class\$key" -ErrorAction SilentlyContinue).'HardwareInformation.qwMemorySize'; "$($_.PNPDeviceID)|$size" }`

// windowsGPUs reads lines of "PCI\VEN_1002&DEV_744C&…|25753026560".
func windowsGPUs(out string) []GPU {
	var gpus []GPU
	for _, line := range strings.Split(out, "\n") {
		device, size, _ := strings.Cut(strings.ToLower(strings.TrimSpace(line)), "|")
		_, rest, ok1 := strings.Cut(device, "ven_")
		vendor, rest, _ := strings.Cut(rest, "&")
		_, rest, ok2 := strings.Cut(rest, "dev_")
		dev, _, _ := strings.Cut(rest, "&")
		if !ok1 || !ok2 || vendor == "10de" {
			continue
		}
		vram, _ := strconv.ParseInt(size, 10, 64)
		if g, ok := pciGPU(vendor+":"+dev, vram); ok {
			gpus = append(gpus, g)
		}
	}
	return gpus
}
