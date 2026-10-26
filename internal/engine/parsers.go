package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Tool call and reasoning parsers of vllm-mlx, by the model_type of the
// model's config.json. Without a tool parser vllm-mlx falls back to a
// generic one that misses several formats, DeepSeek V4's DSML and GLM's
// arg_key among them: the job then sees no tool call at all.
var vllmParsers = map[string]struct{ tool, reasoning string }{
	"deepseek_v4":   {"deepseek_v4", "deepseek_v4"},
	"deepseek_v41":  {"deepseek_v4", "deepseek_v4"},
	"glm4_moe":      {"glm47", "glm4"},
	"glm4_moe_lite": {"glm47", "glm4"},
	"glm_moe_dsa":   {"glm47", "glm4"},
	"glm5_next":     {"glm47", "glm4"},
	// <tool_call><function=…><parameter=…>, thinking in <think>.
	"qwen3_5":       {"qwen3_xml", "qwen3"},
	"qwen3_5_moe":   {"qwen3_xml", "qwen3"},
	"qwen4_exp":     {"qwen3_xml", "qwen3"},
	"mimo_v2":       {"qwen3_xml", "qwen3"},
	"mimo_v2_flash": {"qwen3_xml", "qwen3"},
	"minimax":       {"minimax", ""},
	"minimax_m2":    {"minimax", ""},
	"gpt_oss":       {"harmony", "harmony"},
	"step3p5":       {"step3p5", "step3p5"},
	"laguna":        {"poolside_v1", "poolside_v1"},
	"gemma4":        {"gemma4", "gemma4"},
}

// vllmParserArgs returns the parser arguments for the model in dir, leaving
// out those the profile's extra arguments already set. A model not in the
// table, or Qwen3 whose instruct and coder versions share a model_type but
// not a tool format, gets the auto parser, which tries the common formats.
func vllmParserArgs(dir string, extra []string) []string {
	p, ok := vllmParsers[modelType(dir)]
	if !ok {
		p.tool = "auto"
	}
	var args []string
	if !hasFlag(extra, "--tool-call-parser") {
		if !hasFlag(extra, "--enable-auto-tool-choice") {
			args = append(args, "--enable-auto-tool-choice")
		}
		args = append(args, "--tool-call-parser", p.tool)
	}
	if p.reasoning != "" && !hasFlag(extra, "--reasoning-parser") {
		args = append(args, "--reasoning-parser", p.reasoning)
	}
	return args
}

// modelType reads model_type from the config.json in dir, "" when unknown.
// Multimodal models keep the text model's under text_config.
func modelType(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return ""
	}
	var c struct {
		ModelType  string `json:"model_type"`
		TextConfig struct {
			ModelType string `json:"model_type"`
		} `json:"text_config"`
	}
	if json.Unmarshal(data, &c) != nil {
		return ""
	}
	if _, ok := vllmParsers[c.ModelType]; ok || c.TextConfig.ModelType == "" {
		return c.ModelType
	}
	return strings.TrimSuffix(c.TextConfig.ModelType, "_text")
}

// hasFlag reports whether args set flag, as "--flag value" or "--flag=value".
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag || strings.HasPrefix(a, flag+"=") {
			return true
		}
	}
	return false
}
