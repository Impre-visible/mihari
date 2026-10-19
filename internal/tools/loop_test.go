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

func offered(request map[string]any) map[string]bool {
	names := map[string]bool{}
	for _, tool := range request["tools"].([]any) {
		names[tool.(map[string]any)["function"].(map[string]any)["name"].(string)] = true
	}
	return names
}

func TestSubagent(t *testing.T) {
	// Requests come in order: the parent, then its subagent's two, then the parent again.
	srv, requests := scriptedModel(t, [][]map[string]any{
		{call("p1", "subagent", map[string]any{"task": "Où est défini main ?"})},
		{call("c1", "search", map[string]any{"pattern": "func main"})},
		{call("c2", "finish", map[string]any{"success": true, "summary": "main est dans main.go:3."})},
		{call("p2", "finish", map[string]any{"success": true, "summary": "Fait."})},
	})
	var events []agent.Event
	out, err := agent.Run(context.Background(), agent.Config{URL: srv.URL, Model: "m"},
		&agent.Env{Kind: agent.KindPing, Runner: &fakeRunner{}}, "système", "tâche", nil, func(e agent.Event) { events = append(events, e) })
	if err != nil || !out.Success || out.Summary != "Fait." {
		t.Fatalf("outcome %+v, %v", out, err)
	}

	parent, child := offered((*requests)[0]), offered((*requests)[1])
	if !parent["subagent"] || !parent["write_file"] {
		t.Errorf("parent tools: %v", parent)
	}
	for _, name := range []string{"subagent", "write_file", "edit_file", "apply_patch", "todo_write", "memory"} {
		if child[name] {
			t.Errorf("the subagent should not get %s", name)
		}
	}
	if !child["read_file"] || !child["run_command"] || !child["finish"] {
		t.Errorf("subagent tools: %v", child)
	}
	// Its conclusion is the result of the parent's call.
	last := (*requests)[3]["messages"].([]any)
	if r := last[len(last)-1].(map[string]any); r["content"] != "main est dans main.go:3." {
		t.Errorf("parent got %v", r)
	}
	// Its lines are marked for the chat to nest them.
	marked := 0
	for _, e := range events {
		if e.Agent == "subagent" {
			marked++
		}
	}
	if marked == 0 || events[len(events)-1].Agent != "" {
		t.Errorf("%d subagent lines in %+v", marked, events)
	}
}

func TestReadImageReachesTheModel(t *testing.T) {
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 20)
	srv, requests := scriptedModel(t, [][]map[string]any{
		{call("a", "read_image", map[string]any{"path": "shot.png"})},
		{call("b", "finish", map[string]any{"success": true, "summary": "vu"})},
	})
	runner := &fakeRunner{respond: func(string) sandbox.Result { return sandbox.Result{Stdout: png} }}
	env := &agent.Env{Kind: agent.KindPing, Runner: runner, Vision: true}
	if _, err := agent.Run(context.Background(), agent.Config{URL: srv.URL, Model: "m"}, env, "s", "t", nil, func(agent.Event) {}); err != nil {
		t.Fatal(err)
	}
	msgs := (*requests)[1]["messages"].([]any)
	parts, ok := msgs[len(msgs)-1].(map[string]any)["content"].([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("last message %v", msgs[len(msgs)-1])
	}
	url := parts[1].(map[string]any)["image_url"].(map[string]any)["url"].(string)
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Errorf("image %q", url)
	}
	if offered((*requests)[0])["read_image"] != true {
		t.Error("read_image is for a model that sees images")
	}
	srv2, requests2 := scriptedModel(t, [][]map[string]any{{call("b", "finish", map[string]any{"success": true, "summary": "x"})}})
	_, _ = agent.Run(context.Background(), agent.Config{URL: srv2.URL, Model: "m"}, &agent.Env{Runner: runner}, "s", "t", nil, func(agent.Event) {})
	if offered((*requests2)[0])["read_image"] {
		t.Error("read_image offered without vision")
	}
}
