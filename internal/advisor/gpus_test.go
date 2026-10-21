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
