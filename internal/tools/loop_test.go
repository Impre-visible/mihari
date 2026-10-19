package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/sandbox"
)

// fakeRunner records the commands it gets and answers with respond, or with
// a file list.
type fakeRunner struct {
	mu       sync.Mutex
	commands []string
	stdin    []string
	respond  func(command string) sandbox.Result
}

func (f *fakeRunner) Exec(_ context.Context, command string, stdin []byte, _ time.Duration) (sandbox.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, command)
	f.stdin = append(f.stdin, string(stdin))
	if f.respond != nil {
		return f.respond(command), nil
	}
	return sandbox.Result{Stdout: "./main.go\n"}, nil
}

func (f *fakeRunner) last() string {
	return f.commands[len(f.commands)-1]
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
	env := &agent.Env{Kind: agent.KindPing, Runner: runner}
	inbox := make(chan agent.Event, 1)
	inbox <- agent.Event{Kind: "message", Title: "Nouveau message de @romeo", Text: "et ajoute un cas vide"}

	out, err := agent.Run(context.Background(), agent.Config{URL: srv.URL, Model: "m", MaxTurns: 10, Timeout: time.Minute, ContextTokens: 8192},
		env, "système", "tâche", inbox, func(agent.Event) {})
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

	// GitLab tools are not offered outside a merge request or an issue.
	offered := map[string]bool{}
	for _, tool := range (*requests)[0]["tools"].([]any) {
		offered[tool.(map[string]any)["function"].(map[string]any)["name"].(string)] = true
	}
	if !offered["run_command"] || !offered["edit_file"] || !offered["todo_write"] || !offered["finish"] ||
		offered["reply"] || offered["push"] || offered["ask_user"] || offered["get_issue"] {
		t.Errorf("offered tools: %v", offered)
	}
}

func TestRunWithoutLimits(t *testing.T) {
	script := [][]map[string]any{}
	for i := range 12 {
		script = append(script, []map[string]any{call(fmt.Sprint(i), "list_files", map[string]any{})})
	}
	script = append(script, []map[string]any{call("end", "finish", map[string]any{"success": true, "summary": "Fini."})})
	srv, _ := scriptedModel(t, script)

	// 0 lifts both limits: thirteen turns, no deadline.
	out, err := agent.Run(context.Background(), agent.Config{URL: srv.URL, Model: "m"},
		&agent.Env{Kind: agent.KindPing, Runner: &fakeRunner{}}, "système", "tâche", nil, func(agent.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Success || out.Turns != 13 {
		t.Errorf("outcome %+v", out)
	}
}
