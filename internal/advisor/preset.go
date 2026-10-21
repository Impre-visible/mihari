package advisor

import (
	"context"
	"regexp"
	"sync"

	"github.com/Impre-visible/mihari/internal/engine"
)

// Chip is an Apple Silicon chip, read from its brand string.
type Chip struct {
	// M3, M4, M5…
	Generation string `json:"generation"`
	// "", Pro, Max or Ultra.
	Variant string `json:"variant"`
}

var chipName = regexp.MustCompile(`\b(M\d+)(?:\s+(Pro|Max|Ultra))?\b`)

// ParseChip reads a brand string such as "Apple M5 Ultra".
func ParseChip(brand string) (Chip, bool) {
	m := chipName.FindStringSubmatch(brand)
	if m == nil {
		return Chip{}, false
	}
	return Chip{Generation: m[1], Variant: m[2]}, true
}

func (c Chip) String() string {
	if c.Variant == "" {
		return "Apple " + c.Generation
	}
	return "Apple " + c.Generation + " " + c.Variant
}

// Slot is what one profile of a preset runs.
type Slot struct {
	Model         string `json:"model"`
	Jobs          int    `json:"jobs"`
	ContextPerJob int    `json:"contextPerJob"`
	// Bytes of weights read on Hugging Face; 0 when unknown.
	Weights int64 `json:"weights"`
}

// Preset is the day and night setup for a Mac's amount of unified memory.
type Preset struct {
	// The memory range it is meant for, as shown to the user.
	Tier   string      `json:"tier"`
	Engine engine.Kind `json:"engine"`
	Day    Slot        `json:"day"`
	Night  Slot        `json:"night"`
	// Memory left to macOS, and what remains for the models.
	Reserved int64 `json:"reserved"`
	Budget   int64 `json:"budget"`
}

type tier struct {
	// Smallest memory the tier takes, in GB.
	min        int64
	name       string
	day, night Slot
}

// Tiers from the largest memory down. Repositories and sizes checked on
// Hugging Face in October 2026: where the planned model does not exist or
// does not fit the tier, the closest one that does takes its place.
var tiers = []tier{
	// DeepSeek V4 Pro 4bit weighs 780 GB: V4 Flash in 8 bits leaves room for 8 jobs.
	{512, "512 Go",
		Slot{Model: "mlx-community/deepseek-ai-DeepSeek-V4-Flash-8bit", Jobs: 8, ContextPerJob: 65536},
		Slot{Model: "mlx-community/deepseek-ai-DeepSeek-V4-Flash-8bit", Jobs: 4, ContextPerJob: 131072}},
	// No DeepSeek V4.1 Flash in 8 bits, and its 4 bits weigh 515 GB.
	{192, "192 à 256 Go",
		Slot{Model: "mlx-community/DeepSeek-V4-Flash-8bit", Jobs: 7, ContextPerJob: 65536},
		Slot{Model: "mlx-community/DeepSeek-V4-Flash-8bit", Jobs: 3, ContextPerJob: 131072}},
	// Qwen3.8 Flash Next 4bit weighs 104 GB, more than the 103 GB left here.
	{128, "128 Go",
		Slot{Model: "mlx-community/Qwen3-Coder-Next-8bit", Jobs: 5, ContextPerJob: 65536},
		Slot{Model: "mlx-community/Qwen3-Coder-Next-8bit", Jobs: 2, ContextPerJob: 131072}},
	{64, "64 à 96 Go",
		Slot{Model: "mlx-community/Qwen3-Coder-30B-A3B-Instruct-8bit", Jobs: 4, ContextPerJob: 49152},
		Slot{Model: "mlx-community/Qwen3-Coder-30B-A3B-Instruct-8bit", Jobs: 2, ContextPerJob: 98304}},
	{36, "36 à 48 Go",
		Slot{Model: "mlx-community/Qwen2.5-Coder-14B-Instruct-8bit", Jobs: 2, ContextPerJob: 32768},
		Slot{Model: "mlx-community/Qwen2.5-Coder-32B-Instruct-4bit", Jobs: 1, ContextPerJob: 65536}},
}

// reserved is the memory macOS keeps: a fifth of it, between 15 and 25 GB.
func reserved(ram int64) int64 {
	return min(max(ram/5, 15*gb), 25*gb)
}

// PresetFor returns the preset of an Apple Silicon Mac, or false on any other
// machine or below 36 GB, which go through the generic advisor.
func PresetFor(h Hardware) (Preset, bool) {
	if !h.AppleSilicon() {
		return Preset{}, false
	}
	for _, t := range tiers {
		if h.RAM >= t.min*gb {
			keep := reserved(h.RAM)
			return Preset{Tier: t.name, Engine: engine.VLLMMLX, Day: t.day, Night: t.night, Reserved: keep, Budget: h.RAM - keep}, true
		}
	}
	return Preset{}, false
}

// withWeights fills in the size of each model, read on Hugging Face.
func withWeights(ctx context.Context, p Preset) Preset {
	var wg sync.WaitGroup
	for _, s := range []*Slot{&p.Day, &p.Night} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Weights, _ = sizeOf(ctx, candidate{engine: p.Engine, ref: s.Model})
		}()
	}
	wg.Wait()
	return p
}
