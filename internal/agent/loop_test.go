package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The loop with real tools is tested in internal/tools.

func TestIdleModelFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "je réfléchis"}}},
		})
	}))
	defer srv.Close()
	_, err := Run(context.Background(), Config{URL: srv.URL, Model: "m", MaxTurns: 10, Timeout: time.Minute},
		&Env{}, "s", "t", nil, func(Event) {})
	if err == nil || !strings.Contains(err.Error(), "sans appeler d'outil") {
		t.Fatalf("got %v", err)
	}
}

func TestSchema(t *testing.T) {
	type args struct {
		Path  string   `json:"path" desc:"Chemin"`
		Start int      `json:"start_line,omitempty"`
		Mode  string   `json:"mode,omitempty" enum:"a,b"`
		Tags  []string `json:"tags,omitempty"`
		Todos []Todo   `json:"todos,omitempty"`
	}
	s := schemaOf(reflect.TypeOf(args{}))
	props := s["properties"].(map[string]any)
	if props["path"].(map[string]any)["type"] != "string" || props["path"].(map[string]any)["description"] != "Chemin" ||
		props["start_line"].(map[string]any)["type"] != "integer" {
		t.Errorf("schema %v", s)
	}
	if enum := props["mode"].(map[string]any)["enum"].([]string); len(enum) != 2 || enum[1] != "b" {
		t.Errorf("enum %v", props["mode"])
	}
	item := props["todos"].(map[string]any)["items"].(map[string]any)
	if item["properties"].(map[string]any)["status"].(map[string]any)["enum"] == nil {
		t.Errorf("nested enum %v", item)
	}
	if req := s["required"].([]string); len(req) != 1 || req[0] != "path" {
		t.Errorf("required %v", req)
	}
}

func TestRegisterTwicePanics(t *testing.T) {
	tool := Define("test_twice", "", func(context.Context, *Env, NoArgs) (string, error) { return "", nil })
	Register(tool)
	defer func() {
		if recover() == nil {
			t.Error("a second tool under one name should panic")
		}
	}()
	Register(tool)
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

func TestPlan(t *testing.T) {
	env := &Env{Todos: []Todo{{"Lire", "completed"}, {"Corriger", "in_progress"}, {"Tester", "pending"}}}
	want := "**Plan**\n\n- [x] Lire\n- [ ] ⏳ **Corriger**\n- [ ] Tester\n"
	if got := env.Plan(); got != want {
		t.Errorf("plan %q", got)
	}
	if env.ClosePlan(context.Background()) != "" {
		t.Error("a plan never shown has nothing to close")
	}
	env.PlanOpen = true
	if got := env.ClosePlan(context.Background()); got != want+"\n" || env.PlanOpen {
		t.Errorf("close %q", got)
	}
}

func TestRetriesPassingEngineFailures(t *testing.T) {
	saved := retryAfter
	retryAfter = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { retryAfter = saved }()

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) <= 2 {
			// What a reverse proxy sends when the remote API drops the connection.
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "je réfléchis"}}},
		})
	}))
	defer srv.Close()
	var infos int
	_, err := Run(context.Background(), Config{URL: srv.URL, Model: "m", MaxTurns: 10, Timeout: time.Minute},
		&Env{}, "s", "t", nil, func(e Event) {
			if e.Kind == "info" {
				infos++
			}
		})
	// Past the two 502s, the model answers; it never calls a tool here.
	if err == nil || !strings.Contains(err.Error(), "sans appeler d'outil") || infos != 2 {
		t.Fatalf("got %v after %d retries", err, infos)
	}

	// Past the last retry, the failure ends the job.
	requests.Store(-100)
	_, err = Run(context.Background(), Config{URL: srv.URL, Model: "m", MaxTurns: 10, Timeout: time.Minute},
		&Env{}, "s", "t", nil, func(Event) {})
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("got %v", err)
	}
}
