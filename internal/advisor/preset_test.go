package advisor

import (
	"testing"

	"github.com/Impre-visible/mihari/internal/settings"
)

func TestParseChip(t *testing.T) {
	for brand, want := range map[string]string{
		"Apple M5 Ultra": "Apple M5 Ultra",
		"Apple M4 Max":   "Apple M4 Max",
		"Apple M3 Pro":   "Apple M3 Pro",
		"Apple M4":       "Apple M4",
	} {
		c, ok := ParseChip(brand)
		if !ok || c.String() != want {
			t.Errorf("%q: %+v, want %s", brand, c, want)
		}
	}
	if _, ok := ParseChip("Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz"); ok {
		t.Error("an Intel chip is not Apple Silicon")
	}
}

func TestPresetFor(t *testing.T) {
	mac := func(ram int64) Hardware { return Hardware{OS: "darwin", Arch: "arm64", Unified: true, RAM: ram * gb} }
	for _, c := range []struct {
		ram       int64
		tier      string
		dayJobs   int
		nightJobs int
		reserved  int64
	}{
		{36, "36 à 48 Go", 2, 1, 15},
		{48, "36 à 48 Go", 2, 1, 15},
		{64, "64 à 96 Go", 4, 2, 15},
		{96, "64 à 96 Go", 4, 2, 19},
		{128, "128 Go", 5, 2, 25},
		{192, "192 à 256 Go", 7, 3, 25},
		{256, "192 à 256 Go", 7, 3, 25},
		{512, "512 Go", 8, 4, 25},
	} {
		p, ok := PresetFor(mac(c.ram))
		if !ok || p.Tier != c.tier || p.Day.Jobs != c.dayJobs || p.Night.Jobs != c.nightJobs {
			t.Errorf("%d GB: %+v", c.ram, p)
		}
		// 96 GB keeps a fifth, 19.2 GB.
		if p.Reserved/gb != c.reserved || p.Budget != c.ram*gb-p.Reserved {
			t.Errorf("%d GB keeps %d GB for macOS, want %d", c.ram, p.Reserved/gb, c.reserved)
		}
	}
	if _, ok := PresetFor(mac(24)); ok {
		t.Error("24 GB goes through the generic advisor")
	}
	if _, ok := PresetFor(Hardware{OS: "linux", Arch: "amd64", RAM: 256 * gb}); ok {
		t.Error("a PC has no Apple Silicon preset")
	}
}

func TestApplyPreset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	store := &settings.Service{}
	// A Jour profile already written keeps its id and prompt.
	_ = store.Set("profiles", []map[string]any{{"id": "j", "name": "jour", "prompt": "relis", "engine": "ollama", "model": "x"}})

	reloads := 0
	s := &Service{Reload: func() { reloads++ }}
	p, _ := PresetFor(Hardware{OS: "darwin", Arch: "arm64", RAM: 256 * gb})
	if err := s.applyPreset(p, "jour", "nuit"); err != nil {
		t.Fatal(err)
	}

	config, _ := store.GetConfig()
	list := config["profiles"].([]any)
	if len(list) != 2 {
		t.Fatalf("%d profiles, want 2", len(list))
	}
	day, night := list[0].(map[string]any), list[1].(map[string]any)
	if day["id"] != "j" || day["prompt"] != "relis" || day["engine"] != "vllm-mlx" || day["model"] != p.Day.Model || day["maxJobs"] != float64(7) {
		t.Errorf("day: %v", day)
	}
	if night["name"] != "Nuit" || night["prompt"] != "nuit" || night["trigger"] != "issues" || night["label"] != NightLabel || night["contextPerJob"] != float64(131072) {
		t.Errorf("night: %v", night)
	}
	schedule := config["schedule"].([]any)
	if len(schedule) != 2 || schedule[0].(map[string]any)["profile"] != "j" || schedule[1].(map[string]any)["profile"] != night["id"] {
		t.Errorf("schedule: %v", schedule)
	}
	if config["schedule_fallback"] != night["id"] || reloads != 1 {
		t.Errorf("fallback %v, %d reloads", config["schedule_fallback"], reloads)
	}
}

func TestPick(t *testing.T) {
	recs := []Recommendation{
		{Model: "big", Quality: 90, Spill: gb, TokensPerSecond: 50},
		{Model: "slow", Quality: 85, TokensPerSecond: 5, Alternatives: []Recommendation{{Model: "slow", Quant: "Q4", Quality: 81, TokensPerSecond: 25}}},
		{Model: "fast", Quality: 70, TokensPerSecond: 100},
	}
	if r, ok := pick(recs, 20); !ok || r.Quality != 81 {
		t.Errorf("want the 4-bit of slow, never a model that spills: %+v", r)
	}
	if _, ok := pick(recs, 200); ok {
		t.Error("nothing is fast enough")
	}
}

func TestPCPreset(t *testing.T) {
	// Sizes as if read on Hugging Face, so the test needs no network.
	h := Hardware{OS: "linux", Arch: "amd64", RAM: 64 * gb, RAMBandwidth: 80, GPUs: []GPU{{Name: "RTX 4090", VRAM: 24 * gb, Bandwidth: 1008}}}
	bits := map[string]float64{"Q8_0": 8.5, "Q6_K": 6.6, "Q5_K_M": 5.7, "Q4_K_M": 4.8, "MXFP4": 4.3, "UD-Q4_K_XL": 4.8, "UD-Q3_K_XL": 3.9, "UD-Q2_K_XL": 3}
	sizeMu.Lock()
	for _, c := range candidates(h) {
		sizeCache[string(c.engine)+"|"+c.ref] = int64(c.model.TotalB * bits[c.quant] / 8 * 1e9)
	}
	sizeMu.Unlock()

	p, ok := pcPreset(t.Context(), h)
	if !ok {
		t.Fatal("a 4090 has a preset")
	}
	if p.Engine != "llama-server" || p.Tier != "24 Go de VRAM" || p.Day.Jobs != 2 || p.Night.Jobs != 1 {
		t.Errorf("%+v", p)
	}
	for _, s := range []Slot{p.Day, p.Night} {
		if s.Weights == 0 || s.Weights > p.Budget || s.TokensPerSecond < nightMinSpeed {
			t.Errorf("slot does not hold in VRAM at speed: %+v", s)
		}
	}
	if p.Day.TokensPerSecond < dayMinSpeed {
		t.Errorf("day too slow: %+v", p.Day)
	}
	t.Logf("day %s %.0f tok/s, night %s %dK %.0f tok/s", p.Day.Model, p.Day.TokensPerSecond, p.Night.Model, p.Night.ContextPerJob/1024, p.Night.TokensPerSecond)

	if _, ok := pcPreset(t.Context(), Hardware{OS: "linux", RAM: 64 * gb}); ok {
		t.Error("no GPU: generic advisor")
	}
	if _, ok := pcPreset(t.Context(), Hardware{OS: "linux", RAM: 64 * gb, GPUs: []GPU{{Name: "?", VRAM: 24 * gb}}}); ok {
		t.Error("unknown bandwidth: generic advisor")
	}
}
