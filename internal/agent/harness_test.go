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
)

// Tools of these tests only: a read that always gives the same answer, a
// post that changes something, and a finish.
var posts int

func init() {
	type text struct {
		Text string `json:"text"`
	}
	Register(
		Define("test_read", "", func(context.Context, *Env, text) (string, error) { return "contenu", nil }),
		Define("test_post", "", func(_ context.Context, _ *Env, a text) (string, error) {
			posts++
			return "posté : " + a.Text, nil
		}, Writes()),
		Define("test_finish", "", func(_ context.Context, env *Env, a struct {
			Success bool   `json:"success"`
			Summary string `json:"summary"`
		}) (string, error) {
			env.Finished = &Finish{Success: a.Success, Summary: a.Summary}
			return "Job terminé.", nil
		}),
	)
}

func toolCallJSON(id, name string, args any) map[string]any {
	data, _ := json.Marshal(args)
	return map[string]any{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": string(data)}}
}

// scripted answers each request with the next tool calls, with reasoning,
// and records the requests.
func scripted(t *testing.T, script [][]map[string]any) (*httptest.Server, *[]map[string]any) {
	var mu sync.Mutex
	var requests []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		turn := len(requests)
		requests = append(requests, req)
		mu.Unlock()
		msg := map[string]any{"role": "assistant", "content": "", "reasoning_content": "je pense", "tool_calls": script[min(turn, len(script)-1)]}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": msg}}})
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func lastUser(req map[string]any) string {
	msgs := req["messages"].([]any)
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i].(map[string]any)
		if m["role"] == "user" || m["role"] == "tool" {
			if c, ok := m["content"].(string); ok {
				return c
			}
		}
	}
	return ""
}

func TestRepeatedCalls(t *testing.T) {
	posts = 0
	srv, requests := scripted(t, [][]map[string]any{
		{toolCallJSON("1", "test_read", map[string]string{"text": "a"})},
		// Same read, nothing changed: run, but flagged.
		{toolCallJSON("2", "test_read", map[string]string{"text": "a"})},
		{toolCallJSON("3", "test_post", map[string]string{"text": "x"})},
		// Same post right after: not posted again.
		{toolCallJSON("4", "test_post", map[string]string{"text": "x"})},
		{toolCallJSON("5", "test_read", map[string]string{"text": "a"})},
		{toolCallJSON("6", "test_read", map[string]string{"text": "a"})},
		{toolCallJSON("7", "test_finish", map[string]any{"success": true, "summary": "fait"})},
	})
	out, err := Run(context.Background(), Config{URL: srv.URL, Model: "m", MaxTurns: 20, Timeout: time.Minute},
		&Env{}, "s", "t", nil, func(Event) {})
	if err != nil || !out.Success {
		t.Fatal(out, err)
	}
	r := *requests
	if !strings.Contains(lastUser(r[2]), "Même appel qu'au tour 1") {
		t.Errorf("a repeated read should be flagged: %q", lastUser(r[2]))
	}
	if posts != 1 || !strings.Contains(lastUser(r[4]), "Déjà fait au tour 3") {
		t.Errorf("a repeated post should not run: %d posts, %q", posts, lastUser(r[4]))
	}
	// The read at 5 follows a post, so it is not a repeat; the one at 6 is the third.
	if !strings.Contains(lastUser(r[6]), "tu tournes en rond") {
		t.Errorf("three repeats should nudge: %q", lastUser(r[6]))
	}
}

func TestCriticSendsWorkBack(t *testing.T) {
	posts = 0
	srv, requests := scripted(t, [][]map[string]any{
		{toolCallJSON("1", "test_post", map[string]string{"text": "remarque"})},
		{toolCallJSON("2", "test_finish", map[string]any{"success": true, "summary": "review faite"})},
		// The reviewer's own turn.
		{toolCallJSON("3", "test_finish", map[string]any{"success": false, "summary": "1. Bug oublié ligne 12."})},
		{toolCallJSON("4", "test_post", map[string]string{"text": "bug ligne 12"})},
		// Finished again: no second review.
		{toolCallJSON("5", "test_finish", map[string]any{"success": true, "summary": "review complétée"})},
	})
	var events []Event
	out, err := Run(context.Background(), Config{URL: srv.URL, Model: "m", MaxTurns: 20, Timeout: time.Minute, Critic: true},
		&Env{}, "s", "relis la MR", nil, func(e Event) { events = append(events, e) })
	if err != nil || !out.Success || out.Summary != "review complétée" || posts != 2 {
		t.Fatalf("%+v %v, %d posts", out, err, posts)
	}
	r := *requests
	if len(r) != 5 {
		t.Fatalf("%d requests, want 5", len(r))
	}
	critic := r[2]["messages"].([]any)
	if sys := critic[0].(map[string]any)["content"].(string); !strings.Contains(sys, "relecteur indépendant") {
		t.Errorf("the reviewer gets its own prompt: %q", sys)
	}
	if brief := critic[1].(map[string]any)["content"].(string); !strings.Contains(brief, "relis la MR") || !strings.Contains(brief, "test_post") {
		t.Errorf("the reviewer sees the task and what was done: %q", brief)
	}
	if !strings.Contains(lastUser(r[3]), "Bug oublié ligne 12") {
		t.Errorf("the model hears the review: %q", lastUser(r[3]))
	}
	var inside int
	for _, e := range events {
		if e.Agent == "critic" {
			inside++
		}
	}
	if inside == 0 {
		t.Error("the reviewer's lines are marked as its own")
	}
}

func TestKeepReasoning(t *testing.T) {
	srv, requests := scripted(t, [][]map[string]any{
		{toolCallJSON("1", "test_read", map[string]string{"text": "a"})},
		{toolCallJSON("2", "test_finish", map[string]any{"success": true, "summary": "ok"})},
	})
	for _, keep := range []bool{false, true} {
		*requests = nil
		if _, err := Run(context.Background(), Config{URL: srv.URL, Model: "m", MaxTurns: 5, Timeout: time.Minute, KeepReasoning: keep},
			&Env{}, "s", "t", nil, func(Event) {}); err != nil {
			t.Fatal(err)
		}
		assistant := (*requests)[1]["messages"].([]any)[2].(map[string]any)
		if got := assistant["reasoning_content"] == "je pense"; got != keep {
			t.Errorf("KeepReasoning %v: sent back %v", keep, assistant["reasoning_content"])
		}
	}
}

func TestFitContextOrder(t *testing.T) {
	big := strings.Repeat("x", 3000)
	call := func(id, args string) toolCall {
		c := toolCall{ID: id}
		c.Function.Name, c.Function.Arguments = "read_file", args
		return c
	}
	msgs := []message{
		{Role: "system"}, {Role: "user"},
		{Role: "assistant", ReasoningContent: big, ToolCalls: []toolCall{call("a", `{"path":"a.go"}`)}},
		{Role: "tool", ToolCallID: "a", Content: big},
		{Role: "assistant", ToolCalls: []toolCall{call("b", `{"path":"b.go"}`)}},
		{Role: "tool", ToolCallID: "b", Content: big},
		// a.go read again: its first result is superseded.
		{Role: "assistant", ToolCalls: []toolCall{call("c", `{ "path": "a.go" }`)}},
		{Role: "tool", ToolCallID: "c", Content: big},
		{Role: "assistant"}, {Role: "user", Content: "suite"},
	}
	// Below the high mark: nothing moves, so the engine's cache holds.
	if fitContext(msgs, 100000, 1)[3].Content != big {
		t.Fatal("nothing is trimmed below the high mark")
	}
	// About 12100 characters against 11900 of room (85% of 14000): down to 8400.
	fitContext(msgs, 14000, 1)
	if msgs[2].ReasoningContent != "" {
		t.Error("old thinking goes first")
	}
	if !isStub(msgs[3].Content) || !strings.Contains(msgs[3].Content, "read_file") {
		t.Errorf("the superseded read goes next, with a stub saying which: %q", msgs[3].Content)
	}
	if msgs[5].Content != big || msgs[7].Content != big {
		t.Error("the rest stays while it fits")
	}
}

func TestAgainRefusesRepeats(t *testing.T) {
	env := &Env{}
	env.Sent(Post{Kind: "comment", Path: "facture.go", Line: 52, Body: "Division entière : la remise vaut toujours 0.", Thread: "abc"})
	for _, c := range []struct {
		name string
		post Post
		want string
	}{
		{"reworded, a line further", Post{Kind: "comment", Path: "facture.go", Line: 53,
			Body: "La remise est toujours nulle à cause de la division entière."}, "déjà fait cette remarque"},
		{"as a suggestion", Post{Kind: "suggestion", Path: "facture.go", Line: 52,
			Body: "Division entière, la remise vaut toujours 0 :\n```suggestion:-0+0\nreturn total - total*f.Remise()/100\n```"}, "déjà fait cette remarque"},
		{"another problem, same line", Post{Kind: "comment", Path: "facture.go", Line: 52, Body: "Le palier devrait être >= et pas >."},
			"reply_to_thread"},
		{"another problem, next line", Post{Kind: "comment", Path: "facture.go", Line: 54, Body: "Le palier devrait être >= et pas >."}, ""},
		{"same words, other file", Post{Kind: "comment", Path: "export.go", Line: 52, Body: "Division entière : la remise vaut toujours 0."}, ""},
		{"same words, far below", Post{Kind: "comment", Path: "facture.go", Line: 90, Body: "Division entière : la remise vaut toujours 0."}, ""},
	} {
		err := env.Again(c.post)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: %v", c.name, err)
		}
	}

	env.Sent(Post{Kind: "reply", Thread: "abc", Body: "Corrigé dans le dernier commit, merci."})
	if env.Again(Post{Kind: "reply", Thread: "abc", Body: "Merci, c'est corrigé dans le dernier commit !"}) == nil {
		t.Error("the same reply in the same thread")
	}
	if env.Again(Post{Kind: "reply", Thread: "def", Body: "Corrigé dans le dernier commit, merci."}) != nil {
		t.Error("another thread is another reply")
	}
}
