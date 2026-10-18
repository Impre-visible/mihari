package advisor

import (
	"testing"

	"github.com/Impre-visible/mihari/internal/engine"
	"github.com/Impre-visible/mihari/internal/settings"
)

func TestApply(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	store := &settings.Service{}
	_ = store.Set("profiles", []map[string]any{{"id": "a", "name": "Review", "prompt": "relis", "engine": "ollama", "model": "x"}})

	reloads := 0
	s := &Service{Reload: func() { reloads++ }}
	r := Recommendation{Model: "Qwen3.6 27B", Engine: engine.LlamaServer, Ref: "unsloth/Qwen3.6-27B-GGUF:Q4_K_M", ExtraArgs: "-ngl 99 -fa on"}

	if _, err := s.Apply("a", r, 2, 16384); err != nil {
		t.Fatal(err)
	}
	id, err := s.Apply("", r, 1, 32768)
	if err != nil || id == "" {
		t.Fatal(id, err)
	}
	if _, err := s.Apply("missing", r, 1, 4096); err == nil {
		t.Error("an unknown profile should fail")
	}

	config, _ := store.GetConfig()
	list := config["profiles"].([]any)
	first, second := list[0].(map[string]any), list[1].(map[string]any)
	if first["prompt"] != "relis" || first["model"] != r.Ref || first["engine"] != "llama-server" || first["maxJobs"] != float64(2) {
		t.Errorf("existing profile: %v", first)
	}
	if second["name"] != "Qwen3.6 27B" || second["id"] != id || second["contextPerJob"] != float64(32768) {
		t.Errorf("new profile: %v", second)
	}
	if reloads != 2 {
		t.Errorf("%d reloads, want 2", reloads)
	}
}
