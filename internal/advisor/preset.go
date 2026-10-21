package advisor

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
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
	// Estimated generation speed of one job; 0 when not estimated (Mac presets).
	TokensPerSecond float64 `json:"tokensPerSecond"`
}

// Preset is the day and night setup of a machine: from a table on a Mac,
// computed from the GPUs on a PC.
type Preset struct {
	// The memory it is meant for, as shown to the user.
	Tier      string      `json:"tier"`
	Engine    engine.Kind `json:"engine"`
	ExtraArgs string      `json:"extraArgs"`
	Day       Slot        `json:"day"`
	Night     Slot        `json:"night"`
	// Memory left to the system, and what remains for the models: unified
	// memory on a Mac, VRAM on a PC.
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

// presetFor returns the machine's preset, or false when it has none and goes
// through the generic advisor.
func presetFor(ctx context.Context, h Hardware) (Preset, bool) {
	if h.AppleSilicon() {
		p, ok := PresetFor(h)
		if ok {
			p = withWeights(ctx, p)
		}
		return p, ok
	}
	return pcPreset(ctx, h)
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
			return Preset{Tier: t.name, Engine: engine.VLLMMLX, ExtraArgs: "--use-paged-cache", Day: t.day, Night: t.night, Reserved: keep, Budget: h.RAM - keep}, true
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

// Slowest estimated speed of one job each profile accepts, in tokens per
// second: reviews are read by people waiting on them, tests by nobody.
const (
	dayMinSpeed   = 20
	nightMinSpeed = 10
)

// pcPreset picks, for each profile, the best model that fits entirely in
// VRAM, never spilling into RAM, and runs fast enough. It needs every GPU's
// bandwidth to estimate the speed, and gives up rather than guess.
func pcPreset(ctx context.Context, h Hardware) (Preset, bool) {
	if len(h.GPUs) == 0 {
		return Preset{}, false
	}
	for _, g := range h.GPUs {
		if g.Bandwidth == 0 {
			return Preset{}, false
		}
	}
	b := budgetOf(h)
	// Jobs and context per job for the day, jobs for the night.
	var dayJobs, dayContext, nightJobs int
	switch vram := b.GPU / gb; {
	case vram < 16:
		dayJobs, dayContext, nightJobs = 1, 32768, 1
	case vram < 32:
		dayJobs, dayContext, nightJobs = 2, 32768, 1
	case vram < 64:
		dayJobs, dayContext, nightJobs = 4, 49152, 2
	default:
		dayJobs, dayContext, nightJobs = 6, 65536, 3
	}
	day, ok := pick(Recommend(ctx, h, dayJobs, dayContext), dayMinSpeed)
	if !ok {
		return Preset{}, false
	}
	// The night takes 128K when that costs no quality, 64K otherwise: a
	// smaller model than the day's would write worse tests.
	var night Recommendation
	nightContext := 0
	for _, c := range []int{131072, 65536} {
		if r, ok := pick(Recommend(ctx, h, nightJobs, c), nightMinSpeed); ok && r.Quality > night.Quality {
			night, nightContext = r, c
		}
	}
	if nightContext == 0 {
		return Preset{}, false
	}
	slot := func(r Recommendation, jobs, perJob int) Slot {
		return Slot{Model: r.Ref, Jobs: jobs, ContextPerJob: perJob, Weights: r.Weights, TokensPerSecond: r.TokensPerSecond}
	}
	return Preset{
		Tier:   fmt.Sprintf("%d Go de VRAM", int64(math.Round(float64(h.VRAM())/gb))),
		Engine: engine.LlamaServer, ExtraArgs: day.ExtraArgs,
		Day: slot(day, dayJobs, dayContext), Night: slot(night, nightJobs, nightContext),
		Reserved: h.VRAM() - b.GPU, Budget: b.GPU,
	}, true
}

// pick returns the best quantization of any model that stays in VRAM at
// minSpeed or more.
func pick(recs []Recommendation, minSpeed float64) (Recommendation, bool) {
	var fits []Recommendation
	for _, r := range recs {
		for _, q := range append([]Recommendation{r}, r.Alternatives...) {
			if q.Spill == 0 && q.TokensPerSecond >= minSpeed {
				fits = append(fits, q)
			}
		}
	}
	if len(fits) == 0 {
		return Recommendation{}, false
	}
	return slices.MinFunc(fits, func(a, b Recommendation) int {
		return cmp.Or(cmp.Compare(b.Quality, a.Quality), cmp.Compare(b.TokensPerSecond, a.TokensPerSecond))
	}), true
}
