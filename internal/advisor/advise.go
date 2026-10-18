package advisor

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"sync"

	"github.com/Impre-visible/mihari/internal/engine"
)

// Recommendation is one model, quantization and engine that fits the
// machine for the requested jobs and context.
type Recommendation struct {
	Model  string      `json:"model"`
	Engine engine.Kind `json:"engine"`
	// What goes in the profile's Model field.
	Ref     string `json:"ref"`
	Quant   string `json:"quant"`
	Quality int    `json:"quality"`
	// Bytes of weights, of context cache for every job, and in total with the engine's own buffers.
	Weights int64 `json:"weights"`
	Context int64 `json:"context"`
	Total   int64 `json:"total"`
	// Part of Total beyond the GPU, in system memory: much slower.
	Spill int64 `json:"spill"`
	// Rough generation speed of one job, in tokens per second; 0 when the GPU is unknown.
	TokensPerSecond float64 `json:"tokensPerSecond"`
	ExtraArgs       string  `json:"extraArgs"`
	// The model's other quantizations that fit, best first.
	Alternatives []Recommendation `json:"alternatives"`
}

// Budget is how much memory models may use.
type Budget struct {
	// Fast memory: VRAM, or a Mac's unified memory less what the system keeps.
	GPU int64 `json:"gpu"`
	// System memory a model may spill into; 0 on a Mac, where it is the same memory.
	RAM          int64   `json:"ram"`
	GPUBandwidth float64 `json:"gpuBandwidth"`
	RAMBandwidth float64 `json:"ramBandwidth"`
}

func budgetOf(h Hardware) Budget {
	if h.Unified {
		keep := max(4*gb, h.RAM/4)
		return Budget{GPU: max(0, h.RAM-keep), GPUBandwidth: h.RAMBandwidth}
	}
	b := Budget{RAM: max(0, h.RAM-4*gb), RAMBandwidth: h.RAMBandwidth}
	if vram := h.VRAM(); vram > 0 {
		b.GPU = max(0, vram-gb/2)
		for _, g := range h.GPUs {
			// Several cards split the layers; the slowest one sets the pace.
			if b.GPUBandwidth == 0 || (g.Bandwidth > 0 && g.Bandwidth < b.GPUBandwidth) {
				b.GPUBandwidth = g.Bandwidth
			}
		}
	}
	return b
}

type candidate struct {
	model  Model
	engine engine.Kind
	ref    string
	quant  string
}

// candidates lists every engine and quantization of the catalog this machine
// can run: MLX on Apple Silicon, GGUF everywhere.
func candidates(h Hardware) []candidate {
	var out []candidate
	for _, m := range Catalog {
		if h.AppleSilicon() && len(m.MLX) > 0 {
			for _, r := range m.MLX {
				out = append(out, candidate{m, engine.VLLMMLX, r.Repo, strconv.Itoa(r.Bits) + "bit"})
			}
			continue
		}
		for _, q := range m.GGUFQuants {
			out = append(out, candidate{m, engine.LlamaServer, m.GGUF + ":" + q, q})
		}
	}
	return out
}

// Sizes read from Hugging Face, kept for the session: they do not change.
var (
	sizeMu    sync.Mutex
	sizeCache = map[string]int64{}
)

func sizeOf(ctx context.Context, c candidate) (int64, error) {
	key := string(c.engine) + "|" + c.ref
	sizeMu.Lock()
	size, ok := sizeCache[key]
	sizeMu.Unlock()
	if ok {
		return size, nil
	}
	size, err := engine.RemoteSize(ctx, c.engine, c.ref)
	if err == nil {
		sizeMu.Lock()
		sizeCache[key] = size
		sizeMu.Unlock()
	}
	return size, err
}

// Recommend returns, best first, the strongest quantization of each model
// that fits jobs parallel jobs of contextPerJob tokens each.
func Recommend(ctx context.Context, h Hardware, jobs, contextPerJob int) []Recommendation {
	b := budgetOf(h)
	cands := candidates(h)
	sizes := make([]int64, len(cands))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, c := range cands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			sizes[i], _ = sizeOf(ctx, c)
		}()
	}
	wg.Wait()

	byModel := map[string][]Recommendation{}
	for i, c := range cands {
		if sizes[i] == 0 || contextPerJob > c.model.MaxContext {
			continue
		}
		r := evaluate(c, sizes[i], b, jobs, contextPerJob)
		if r.Total > b.GPU+b.RAM {
			continue
		}
		byModel[c.model.Name] = append(byModel[c.model.Name], r)
	}
	out := make([]Recommendation, 0, len(byModel))
	for _, fits := range byModel {
		slices.SortFunc(fits, func(a, b Recommendation) int {
			return cmp.Or(cmp.Compare(b.Quality, a.Quality), cmp.Compare(b.TokensPerSecond, a.TokensPerSecond))
		})
		best := fits[0]
		best.Alternatives = append([]Recommendation{}, fits[1:]...)
		out = append(out, best)
	}
	slices.SortFunc(out, func(a, b Recommendation) int {
		return cmp.Or(cmp.Compare(b.Quality, a.Quality), cmp.Compare(b.TokensPerSecond, a.TokensPerSecond))
	})
	return out
}

// The share of the memory bandwidth decoding actually achieves.
const efficiency = 0.6

func evaluate(c candidate, weights int64, b Budget, jobs, contextPerJob int) Recommendation {
	kv := c.model.KVPerToken * int64(contextPerJob) * int64(jobs)
	// Compute buffers of the engine, a little more for each parallel sequence.
	overhead := gb/2 + int64(jobs)*gb/10
	total := weights + kv + overhead
	r := Recommendation{
		Model: c.model.Name, Engine: c.engine, Ref: c.ref, Quant: c.quant,
		Quality: c.model.Quality - quantPenalty(c.quant),
		Weights: weights, Context: kv, Total: total, Spill: max(0, total-b.GPU),
	}

	// Each generated token reads the active weights and, on average, half a full context.
	read := float64(weights)*c.model.ActiveB/c.model.TotalB + float64(c.model.KVPerToken*int64(contextPerJob))/2
	onGPU := 1.0
	if total > 0 && b.GPU < total {
		onGPU = float64(b.GPU) / float64(total)
	}
	seconds := 0.0
	if onGPU > 0 {
		if b.GPUBandwidth == 0 {
			seconds = -1
		} else {
			seconds += read * onGPU / (b.GPUBandwidth * 1e9)
		}
	}
	if seconds >= 0 && onGPU < 1 {
		seconds += read * (1 - onGPU) / (b.RAMBandwidth * 1e9)
	}
	if seconds > 0 {
		r.TokensPerSecond = efficiency / seconds
	}

	switch {
	case c.engine == engine.VLLMMLX:
		r.ExtraArgs = "--use-paged-cache"
	case r.Spill > 0:
		// llama-server places what fits on the GPU itself, experts on the CPU first.
		r.ExtraArgs = "-fa on --fit on"
	default:
		r.ExtraArgs = "-ngl 99 -fa on"
	}
	return r
}
