package advisor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPCIGPU(t *testing.T) {
	// amdgpu reports a little less than the size the card is sold with.
	if g, ok := pciGPU("1002:744C", 25753026560); !ok || g.Name != "AMD Radeon RX 7900 XTX" || g.Bandwidth != 960 {
		t.Errorf("24 GB Navi 31: %+v", g)
	}
	if g, _ := pciGPU("1002:744c", 20*gb); g.Name != "AMD Radeon RX 7900 XT" {
		t.Errorf("20 GB Navi 31: %+v", g)
	}
	// Unknown size: the smallest card with that chip.
	if g, _ := pciGPU("8086:56a0", 0); g.VRAM != 8*gb {
		t.Errorf("A770 of unknown size: %+v", g)
	}
	if _, ok := pciGPU("1002:164e", 512<<20); ok {
		t.Error("an integrated Radeon is left out")
	}
}

func TestSysfsGPUs(t *testing.T) {
	root := t.TempDir()
	card := func(name, vendor, device, vram string) {
		dir := filepath.Join(root, name, "device")
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "vendor"), []byte(vendor+"\n"), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "device"), []byte(device+"\n"), 0o644)
		if vram != "" {
			_ = os.WriteFile(filepath.Join(dir, "mem_info_vram_total"), []byte(vram+"\n"), 0o644)
		}
	}
	card("card0", "0x1002", "0x7550", "17163091968")
	card("card1", "0x8086", "0xe20b", "")
	card("card2", "0x10de", "0x2684", "")
	card("card3", "0x1002", "0x164e", "536870912")
	_ = os.MkdirAll(filepath.Join(root, "card0-DP-1"), 0o755)

	gpus := sysfsGPUs(root)
	if len(gpus) != 2 || gpus[0].Name != "AMD Radeon RX 9070 / 9070 XT" || gpus[1].Name != "Intel Arc B580" || gpus[1].VRAM != 12*gb {
		t.Errorf("%+v", gpus)
	}
}

func TestWindowsGPUs(t *testing.T) {
	out := "PCI\\VEN_10DE&DEV_2684&SUBSYS_16F310DE&REV_A1\\4&1234|25769803776\r\n" +
		"PCI\\VEN_8086&DEV_E20C&SUBSYS_00000000&REV_00\\4&5678|10737418240\r\n" +
		"PCI\\VEN_8086&DEV_A780&SUBSYS_00000000&REV_04\\3&9|\r\n"
	gpus := windowsGPUs(out)
	if len(gpus) != 1 || gpus[0].Name != "Intel Arc B570" || gpus[0].Bandwidth != 380 {
		t.Errorf("%+v", gpus)
	}
}

// Each chip lists its sizes smallest first, once each: pciGPU takes the
// first one when the size cannot be read.
func TestPCITableOrder(t *testing.T) {
	last := map[string]int64{}
	for _, e := range pciTable {
		if prev, ok := last[e.id]; ok && e.vram <= prev {
			t.Errorf("%s: %d GB after %d GB", e.id, e.vram, prev)
		}
		last[e.id] = e.vram
		if e.bw <= 0 || e.name == "" {
			t.Errorf("%s: incomplete entry", e.id)
		}
	}
}

func TestNvidiaBandwidth(t *testing.T) {
	for _, c := range []struct {
		name string
		vram int64
		want float64
	}{
		{"NVIDIA GeForce RTX 4090", 24, 1008},
		{"NVIDIA GeForce RTX 4070 Ti SUPER", 16, 672},
		{"NVIDIA GeForce RTX 3060", 12, 360},
		{"NVIDIA GeForce RTX 3060", 8, 240},
		{"NVIDIA GeForce RTX 3080", 12, 912},
		{"NVIDIA GeForce RTX 3080", 10, 760},
		{"NVIDIA A100-PCIE-40GB", 40, 1555},
		{"NVIDIA A100 80GB PCIe", 80, 1935},
		{"NVIDIA H100 PCIe", 80, 2000},
		{"NVIDIA L40S", 48, 864},
		{"NVIDIA RTX PRO 6000 Blackwell Max-Q Workstation Edition", 96, 1792},
		// Word boundaries: an A1000 is not an A100, nor an A10.
		{"NVIDIA RTX A1000", 8, 0},
		// Laptop GPUs share the desktop names with slower memory.
		{"NVIDIA GeForce RTX 4090 Laptop GPU", 16, 0},
		{"NVIDIA GeForce RTX 2080 with Max-Q Design", 8, 0},
	} {
		if got := nvidiaBandwidth(c.name, c.vram*gb); got != c.want {
			t.Errorf("%s %d GB: %.0f GB/s, want %.0f", c.name, c.vram, got, c.want)
		}
	}
}
