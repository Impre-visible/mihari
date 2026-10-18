package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Impre-visible/mihari/internal/sandbox"
)

type fakeRunner struct {
	mu       sync.Mutex
	commands []string
	stdin    []string
}

func (f *fakeRunner) Exec(_ context.Context, command string, stdin []byte, _ time.Duration) (sandbox.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, command)
	f.stdin = append(f.stdin, string(stdin))
	return sandbox.Result{Stdout: "./main.go\n"}, nil
}

// scriptedModel answers each chat request with the next scripted tool calls
// and records what it was sent.
func scriptedModel(t *testing.T, script [][]map[string]any) (*httptest.Server, *[]map[string]any) {
	var mu sync.Mutex
	var requests []map[string]any
	turn := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		requests = append(requests, req)
		calls := script[min(turn, len(script)-1)]
		turn++
		mu.Unlock()
		msg := map[string]any{"role": "assistant", "content": nil}
		if len(calls) > 0 {
			msg["tool_calls"] = calls
		} else {
			msg["content"] = "je réfléchis"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": msg}},
			"usage":   map[string]any{"prompt_tokens": 100},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func call(id, name string, args any) map[string]any {
	data, _ := json.Marshal(args)
	return map[string]any{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": string(data)}}
}

func TestRunLoop(t *testing.T) {
	srv, requests := scriptedModel(t, [][]map[string]any{
		{call("a", "list_files", map[string]any{})},
		{call("b", "write_file", map[string]any{"path": "x_test.go", "content": "package x"})},
		{call("c", "finish", map[string]any{"success": true, "summary": "Test ajouté."})},
	})
	runner := &fakeRunner{}
	env := &Env{Kind: KindPing, Runner: runner}
	inbox := make(chan Event, 1)
	inbox <- Event{Kind: "message", Title: "Nouveau message de @romeo", Text: "et ajoute un cas vide"}

	var events []Event
	out, err := Run(context.Background(), Config{URL: srv.URL, Model: "m", MaxTurns: 10, Timeout: time.Minute, ContextTokens: 8192},
		env, "système", "tâche", inbox, func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	if !out.Success || out.Summary != "Test ajouté." || out.Turns != 3 {
		t.Errorf("outcome %+v", out)
	}
	if runner.stdin[1] != "package x" || !strings.Contains(runner.commands[1], "/workspace/x_test.go") {
		t.Errorf("write_file ran %q with %q", runner.commands[1], runner.stdin[1])
	}

	// The thread message rode along with the first tool result.
	second := (*requests)[1]["messages"].([]any)
	last := second[len(second)-1].(map[string]any)
	if last["role"] != "tool" || !strings.Contains(last["content"].(string), "et ajoute un cas vide") {
		t.Errorf("inbox not injected: %v", last)
	}

	// MR tools are not offered outside a merge request.
	offered := map[string]bool{}
	for _, tool := range (*requests)[0]["tools"].([]any) {
		offered[tool.(map[string]any)["function"].(map[string]any)["name"].(string)] = true
	}
	if !offered["run_command"] || !offered["finish"] || offered["reply"] || offered["push"] {
		t.Errorf("offered tools: %v", offered)
	}
}

func TestIdleModelFails(t *testing.T) {
	srv, _ := scriptedModel(t, [][]map[string]any{{}})
	_, err := Run(context.Background(), Config{URL: srv.URL, Model: "m", MaxTurns: 10, Timeout: time.Minute},
		&Env{Runner: &fakeRunner{}}, "s", "t", nil, func(Event) {})
	if err == nil || !strings.Contains(err.Error(), "sans appeler d'outil") {
		t.Fatalf("got %v", err)
	}
}

func TestSchema(t *testing.T) {
	s := schemaOf(reflectType[readFileArgs]())
	props := s["properties"].(map[string]any)
	if props["path"].(map[string]any)["type"] != "string" || props["start_line"].(map[string]any)["type"] != "integer" {
		t.Errorf("schema %v", s)
	}
	if req := s["required"].([]string); len(req) != 1 || req[0] != "path" {
		t.Errorf("required %v", req)
	}
}

func TestFitContext(t *testing.T) {
	big := strings.Repeat("x", 4000)
	msgs := []message{{Role: "system"}, {Role: "user"}, {Role: "tool", Content: big}, {Role: "tool", Content: big},
		{Role: "assistant"}, {Role: "tool", Content: big}, {Role: "assistant"}, {Role: "tool", Content: big}}
	fitContext(msgs, 2000, 3.5) // ~6000 characters of room
	if msgs[2].Content != trimmed || msgs[len(msgs)-1].Content != big {
		t.Errorf("oldest should go first, latest stays")
	}
}
