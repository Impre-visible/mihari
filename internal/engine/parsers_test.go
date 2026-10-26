package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestVLLMParserArgs(t *testing.T) {
	model := func(config string) string {
		dir := t.TempDir()
		if config != "" {
			_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644)
		}
		return dir
	}
	for _, c := range []struct {
		name   string
		config string
		extra  []string
		want   []string
	}{
		{"DeepSeek V4", `{"model_type":"deepseek_v4"}`, nil,
			[]string{"--enable-auto-tool-choice", "--tool-call-parser", "deepseek_v4", "--reasoning-parser", "deepseek_v4"}},
		// Multimodal: the text model's type, without its _text suffix.
		{"Qwen3.8 Flash Next", `{"model_type":"qwen4_exp","text_config":{"model_type":"qwen4_exp_text"}}`, nil,
			[]string{"--enable-auto-tool-choice", "--tool-call-parser", "qwen3_xml", "--reasoning-parser", "qwen3"}},
		{"GLM-5.3 Flash", `{"model_type":"glm5_next","text_config":{"model_type":"glm5_next_text"}}`, nil,
			[]string{"--enable-auto-tool-choice", "--tool-call-parser", "glm47", "--reasoning-parser", "glm4"}},
		{"Qwen3 MoE, instruct or coder", `{"model_type":"qwen3_moe"}`, nil,
			[]string{"--enable-auto-tool-choice", "--tool-call-parser", "auto"}},
		{"no config", "", nil,
			[]string{"--enable-auto-tool-choice", "--tool-call-parser", "auto"}},
		// What the profile sets wins.
		{"set by the profile", `{"model_type":"deepseek_v4"}`, []string{"--tool-call-parser=dsml", "--enable-auto-tool-choice"},
			[]string{"--reasoning-parser", "deepseek_v4"}},
	} {
		if got := vllmParserArgs(model(c.config), c.extra); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
