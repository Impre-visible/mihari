package advisor

import (
	"testing"

	"github.com/Impre-visible/mihari/internal/engine"
)

func TestBudget(t *testing.T) {
	mac := budgetOf(Hardware{OS: "darwin", Arch: "arm64", Unified: true, RAM: 64 * gb, RAMBandwidth: 400})
	if mac.GPU != 48*gb || mac.RAM != 0 || mac.GPUBandwidth != 400 {
		t.Errorf("64 GB Mac: %+v, want 48 GB for models and no spill", mac)
	}
	small := budgetOf(Hardware{OS: "darwin", Arch: "arm64", Unified: true, RAM: 8 * gb})
	if small.GPU != 4*gb {
		t.Errorf("8 GB Mac keeps at least 4 GB: %+v", small)
	}
	pc := budgetOf(Hardware{RAM: 64 * gb, RAMBandwidth: 50, GPUs: []GPU{{VRAM: 6 * gb, Bandwidth: 192}}})
	if pc.GPU != 6*gb-gb/2 || pc.RAM != 60*gb {
		t.Errorf("PC: %+v", pc)
	}
}

func TestEvaluate(t *testing.T) {
	b := Budget{GPU: 24 * gb, RAM: 60 * gb, GPUBandwidth: 1000, RAMBandwidth: 50}
	dense := Model{Name: "d", Quality: 80, ActiveB: 27, TotalB: 27, KVPerToken: 65536, MaxContext: 262144}
	fits := evaluate(candidate{dense, engine.LlamaServer, "x:Q4_K_M", "Q4_K_M"}, 16*gb, b, 1, 32768)
	if fits.Spill != 0 || fits.ExtraArgs != "-ngl 99 -fa on" || fits.Quality != 76 {
		t.Errorf("fitting model: %+v", fits)
	}
	spills := evaluate(candidate{dense, engine.LlamaServer, "x:Q8_0", "Q8_0"}, 29*gb, b, 2, 32768)
	if spills.Spill == 0 || spills.ExtraArgs != "-fa on --fit on" || spills.TokensPerSecond >= fits.TokensPerSecond {
		t.Errorf("spilling model should be flagged and slower: %+v vs %+v", spills, fits)
	}
	// A mixture of experts reads few weights per token, so it is much faster at the same size.
	moe := Model{Name: "m", Quality: 70, ActiveB: 3, TotalB: 30, KVPerToken: 20480, MaxContext: 262144}
	fast := evaluate(candidate{moe, engine.LlamaServer, "y:Q4_K_M", "Q4_K_M"}, 16*gb, b, 1, 32768)
	if fast.TokensPerSecond < 3*fits.TokensPerSecond {
		t.Errorf("MoE %.0f tok/s vs dense %.0f", fast.TokensPerSecond, fits.TokensPerSecond)
	}
}
