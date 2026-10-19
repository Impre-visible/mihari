package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

func TestMemory(t *testing.T) {
	day := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	notes := "- 2026-01-01 Les tests se lancent avec make test\n- 2026-01-02 La CI tourne sur Node 18"
	got, err := applyMemory(notes, []memoryOp{
		{Action: "replace", OldText: "Node 18", Content: "La CI tourne sur Node 22"},
		{Action: "remove", OldText: "make test"},
		{Action: "add", Content: "Pas de   console.log  dans src/"},
		{Action: "add", Content: "Pas de console.log dans src/"},
	}, day)
	want := "- 2026-10-19 La CI tourne sur Node 22\n- 2026-10-19 Pas de console.log dans src/"
	if err != nil || got != want {
		t.Errorf("got %q, %v", got, err)
	}
	for _, ops := range [][]memoryOp{
		{{Action: "remove", OldText: "absent"}},
		{{Action: "remove", OldText: "2026"}},
		{{Action: "add"}},
		{{Action: "forget", OldText: "x"}},
	} {
		if _, err := applyMemory(notes, ops, day); err == nil || !strings.HasPrefix(err.Error(), "opération 1 (") {
			t.Errorf("should fail naming the operation: %+v, %v", ops, err)
		}
	}

	var saved string
	env := &agent.Env{Notes: func() string { return notes }, SetNotes: func(n string) error { saved = n; return nil }}
	if out, err := run(t, env, "memory", map[string]any{}); err != nil || saved != "" || out != "Notes du projet :\n"+notes {
		t.Errorf("reading should not save: %q, %v, %q", out, err, saved)
	}
	if _, err := run(t, env, "memory", map[string]any{"operations": []map[string]string{{"action": "remove", "old_text": "Node"}}}); err != nil || saved != "- 2026-01-01 Les tests se lancent avec make test" {
		t.Errorf("saved %q, %v", saved, err)
	}
}
