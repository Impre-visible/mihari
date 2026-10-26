package advisor

// Model is a catalog entry. Sizes are not stored: they come from Hugging
// Face when recommending, so each quantization counts what it really weighs.
type Model struct {
	Name string
	// Rough coding and tool-use ability, 0 to 100: an estimate to rank
	// models against each other, not a benchmark score.
	Quality int
	// Parameters used per token, in billions: all of them for a dense model,
	// far fewer for a mixture of experts. Speed depends on these.
	ActiveB float64
	TotalB  float64
	// KV cache bytes per token of context at 16 bits, which only full
	// attention layers pay for.
	KVPerToken int64
	MaxContext int
	// GGUF repository and the quantizations offered from it, best first.
	GGUF       string
	GGUFQuants []string
	// MLX repository per number of bits, best first.
	MLX []MLXRepo
}

type MLXRepo struct {
	Bits int
	Repo string
}

// Catalog lists models known to write code and call tools well, from the
// largest to the smallest. Checked on Hugging Face in October 2026.
var Catalog = []Model{
	// Active parameters are an estimate: the published config does not add up to its 180B.
	{Name: "Qwen3.8 Flash Next", Quality: 92, ActiveB: 10, TotalB: 180, KVPerToken: 24576, MaxContext: 262144,
		GGUF: "unsloth/Qwen3.8-Flash-Next-GGUF", GGUFQuants: []string{"UD-Q4_K_XL", "UD-Q3_K_XL", "UD-Q2_K_XL"},
		MLX: []MLXRepo{{4, "mlx-community/Qwen3.8-Flash-Next-4bit"}}},
	// Dense, Llama architecture. Ahead of Qwen3.8 27B on most of the agentic
	// coding benchmarks its makers published; no MLX build from mlx-community
	// yet, so Macs run the GGUF.
	{Name: "Nanbeige4.3 33B", Quality: 87, ActiveB: 33.1, TotalB: 33.1, KVPerToken: 212992, MaxContext: 524288,
		GGUF: "bartowski/Nanbeige_Nanbeige4.3-33B-GGUF", GGUFQuants: []string{"Q8_0", "Q6_K", "Q4_K_M"}},
	{Name: "Qwen3.8 27B", Quality: 86, ActiveB: 27.8, TotalB: 27.8, KVPerToken: 65536, MaxContext: 262144,
		GGUF: "unsloth/Qwen3.8-27B-GGUF", GGUFQuants: []string{"Q8_0", "Q6_K", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Qwen3.8-27B-8bit"}, {4, "mlx-community/Qwen3.8-27B-4bit"}}},
	{Name: "Qwen3 Coder Next", Quality: 85, ActiveB: 3.9, TotalB: 79.7, KVPerToken: 24576, MaxContext: 262144,
		GGUF: "unsloth/Qwen3-Coder-Next-GGUF", GGUFQuants: []string{"Q8_0", "Q6_K", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Qwen3-Coder-Next-8bit"}, {6, "mlx-community/Qwen3-Coder-Next-6bit"}, {4, "mlx-community/Qwen3-Coder-Next-4bit"}}},
	{Name: "gpt-oss 120B", Quality: 82, ActiveB: 5.7, TotalB: 116.8, KVPerToken: 36864, MaxContext: 131072,
		GGUF: "ggml-org/gpt-oss-120b-GGUF", GGUFQuants: []string{"MXFP4"},
		MLX: []MLXRepo{{4, "mlx-community/gpt-oss-120b-4bit"}}},
	{Name: "Qwen3.6 27B", Quality: 82, ActiveB: 27.8, TotalB: 27.8, KVPerToken: 65536, MaxContext: 262144,
		GGUF: "unsloth/Qwen3.6-27B-GGUF", GGUFQuants: []string{"Q8_0", "Q6_K", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Qwen3.6-27B-8bit"}, {6, "mlx-community/Qwen3.6-27B-6bit"}, {4, "mlx-community/Qwen3.6-27B-4bit"}}},
	{Name: "GLM-4.5 Air", Quality: 80, ActiveB: 17.1, TotalB: 110.5, KVPerToken: 188416, MaxContext: 131072,
		GGUF: "unsloth/GLM-4.5-Air-GGUF", GGUFQuants: []string{"Q6_K", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/GLM-4.5-Air-8bit"}, {6, "mlx-community/GLM-4.5-Air-6bit"}, {4, "mlx-community/GLM-4.5-Air-4bit"}}},
	{Name: "Qwen3.6 35B A3B", Quality: 76, ActiveB: 4.7, TotalB: 36, KVPerToken: 20480, MaxContext: 262144,
		GGUF: "unsloth/Qwen3.6-35B-A3B-GGUF", GGUFQuants: []string{"Q8_0", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Qwen3.6-35B-A3B-8bit"}, {6, "mlx-community/Qwen3.6-35B-A3B-6bit"}, {4, "mlx-community/Qwen3.6-35B-A3B-4bit"}}},
	{Name: "Devstral Small 2 24B", Quality: 74, ActiveB: 24, TotalB: 24, KVPerToken: 163840, MaxContext: 393216,
		GGUF: "unsloth/Devstral-Small-2-24B-Instruct-2512-GGUF", GGUFQuants: []string{"Q8_0", "Q6_K", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Devstral-Small-2-24B-Instruct-2512-8bit"}, {6, "mlx-community/Devstral-Small-2-24B-Instruct-2512-6bit"}, {4, "mlx-community/Devstral-Small-2-24B-Instruct-2512-4bit"}}},
	{Name: "GLM-4.7 Flash", Quality: 72, ActiveB: 5.2, TotalB: 31.2, KVPerToken: 54144, MaxContext: 202752,
		GGUF: "unsloth/GLM-4.7-Flash-GGUF", GGUFQuants: []string{"Q8_0", "Q6_K", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/GLM-4.7-Flash-8bit"}, {6, "mlx-community/GLM-4.7-Flash-6bit"}, {4, "mlx-community/GLM-4.7-Flash-4bit"}}},
	{Name: "Qwen3 Coder 30B A3B", Quality: 70, ActiveB: 3.4, TotalB: 30.5, KVPerToken: 98304, MaxContext: 262144,
		GGUF: "unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF", GGUFQuants: []string{"Q8_0", "Q6_K", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Qwen3-Coder-30B-A3B-Instruct-8bit"}, {6, "mlx-community/Qwen3-Coder-30B-A3B-Instruct-6bit"}, {4, "mlx-community/Qwen3-Coder-30B-A3B-Instruct-4bit"}}},
	{Name: "gpt-oss 20B", Quality: 66, ActiveB: 4.2, TotalB: 20.9, KVPerToken: 24576, MaxContext: 131072,
		GGUF: "ggml-org/gpt-oss-20b-GGUF", GGUFQuants: []string{"MXFP4"}},
	{Name: "Qwen3.5 9B", Quality: 60, ActiveB: 9.7, TotalB: 9.7, KVPerToken: 32768, MaxContext: 262144,
		GGUF: "unsloth/Qwen3.5-9B-GGUF", GGUFQuants: []string{"Q8_0", "Q6_K", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Qwen3.5-9B-8bit"}, {6, "mlx-community/Qwen3.5-9B-6bit"}, {4, "mlx-community/Qwen3.5-9B-4bit"}}},
	{Name: "Qwen3.5 4B", Quality: 50, ActiveB: 4.7, TotalB: 4.7, KVPerToken: 32768, MaxContext: 262144,
		GGUF: "unsloth/Qwen3.5-4B-GGUF", GGUFQuants: []string{"Q8_0", "Q6_K", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Qwen3.5-4B-8bit"}, {4, "mlx-community/Qwen3.5-4B-4bit"}}},
	{Name: "Qwen3.5 2B", Quality: 38, ActiveB: 2.3, TotalB: 2.3, KVPerToken: 12288, MaxContext: 262144,
		GGUF: "unsloth/Qwen3.5-2B-GGUF", GGUFQuants: []string{"Q8_0", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Qwen3.5-2B-8bit"}, {4, "mlx-community/Qwen3.5-2B-4bit"}}},
	{Name: "Qwen3.5 0.8B", Quality: 25, ActiveB: 0.9, TotalB: 0.9, KVPerToken: 12288, MaxContext: 262144,
		GGUF: "unsloth/Qwen3.5-0.8B-GGUF", GGUFQuants: []string{"Q8_0", "Q4_K_M"},
		MLX: []MLXRepo{{8, "mlx-community/Qwen3.5-0.8B-8bit"}, {4, "mlx-community/Qwen3.5-0.8B-4bit"}}},
}

// What a quantization costs in quality, in Quality points.
func quantPenalty(quant string) int {
	switch quant {
	case "Q8_0", "8bit", "MXFP4":
		return 0
	case "Q6_K", "6bit":
		return 1
	case "Q5_K_M", "UD-Q5_K_XL":
		return 2
	case "Q4_K_M", "UD-Q4_K_XL", "4bit":
		return 4
	case "UD-Q3_K_XL", "Q3_K_M":
		return 9
	default:
		// 2 bits and below lose a lot.
		return 16
	}
}
