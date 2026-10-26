package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Impre-visible/mihari/internal/settings"
	"github.com/zalando/go-keyring"
)

// fakeLab answers the calls the test projects make, and records them.
type fakeLab struct {
	mu    sync.Mutex
	calls []string
	// Projects the token's owner can see, as the API lists them.
	projects []map[string]any
	// GitLab.com schedules deletions instead of doing them.
	scheduled bool
}

func (f *fakeLab) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := r.Method + " " + r.URL.Path
	if q := r.URL.Query().Get("permanently_remove"); q != "" {
		call += "?permanently_remove"
	}
	f.calls = append(f.calls, call)
	switch {
	case call == "GET /api/v4/user":
		fmt.Fprint(w, `{"id":3,"username":"bot","name":"Bot"}`)
	case call == "GET /api/v4/projects":
		json.NewEncoder(w).Encode(f.projects)
	case call == "POST /api/v4/projects":
		fmt.Fprint(w, `{"id":42,"path":"mihari-test-x","path_with_namespace":"bot/mihari-test-x","web_url":"https://lab/bot/mihari-test-x"}`)
	case strings.HasPrefix(call, "DELETE "):
		w.WriteHeader(http.StatusAccepted)
	case strings.HasPrefix(call, "GET /api/v4/projects/") && f.scheduled:
		fmt.Fprint(w, `{"path_with_namespace":"bot/mihari-test-x-deletion_scheduled-1","marked_for_deletion_on":"2026-11-01"}`)
	case strings.HasPrefix(call, "GET /api/v4/projects/"):
		// Deleted right away, as on a self-managed instance.
		http.NotFound(w, r)
	default:
		fmt.Fprint(w, `{}`)
	}
}

func connect(t *testing.T, f *fakeLab) {
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	keyring.MockInit()
	store := &settings.Service{}
	_ = store.Set("gitlab_url", srv.URL)
	_ = store.Set("gitlab_username", "bot")
	_ = store.SetGitLabToken("tok")
}

func TestCreateTestProject(t *testing.T) {
	f := &fakeLab{}
	connect(t, f)
	spec := TestSpec{
		Path:          "mihari-test-x",
		Files:         map[string]string{"a.go": "a", "b.go": "b"},
		MergeRequests: []TestChange{{Branch: "fix", Title: "Fix", Files: map[string]string{"a.go": "a2"}}},
		Issues:        []TestIssue{{Title: "Tests", Labels: []string{"mihari::tests"}}},
	}
	p, err := CreateTestProject(context.Background(), spec)
	if err != nil || p.ID != 42 {
		t.Fatal(p, err)
	}
	want := []string{
		"GET /api/v4/user", "POST /api/v4/projects",
		"POST /api/v4/projects/42/repository/commits", "POST /api/v4/projects/42/repository/commits",
		"POST /api/v4/projects/42/merge_requests", "POST /api/v4/projects/42/issues",
	}
	if fmt.Sprint(f.calls) != fmt.Sprint(want) {
		t.Errorf("calls:\n%v\nwant:\n%v", f.calls, want)
	}
	if _, err := CreateTestProject(context.Background(), TestSpec{Path: "prod"}); err == nil {
		t.Error("a project without the test prefix must be refused")
	}
}

// Only the bot's own projects with the test prefix may go, never one of a
// group or another user, nor one already scheduled for deletion.
func TestDeleteTestProjects(t *testing.T) {
	project := func(id int, path, kind, ns string, marked bool) map[string]any {
		p := map[string]any{"id": id, "path": path, "path_with_namespace": ns + "/" + path,
			"namespace": map[string]any{"kind": kind, "path": ns}}
		if marked {
			p["marked_for_deletion_on"] = "2026-11-01"
		}
		return p
	}
	f := &fakeLab{projects: []map[string]any{
		project(1, "mihari-test-factures", "user", "bot", false),
		project(2, "mihari-test-agenda", "user", "Bot", false),
		project(3, "mihari-test-x", "group", "bot", false),
		project(4, "mihari-test-y", "user", "alice", false),
		project(5, "factures", "user", "bot", false),
		project(6, "mihari-test-old", "user", "bot", true),
	}}
	connect(t, f)
	deleted, err := DeleteTestProjects(context.Background())
	if err != nil || fmt.Sprint(deleted) != "[1 2]" {
		t.Fatalf("deleted %v, %v", deleted, err)
	}
	for _, c := range f.calls {
		if strings.HasPrefix(c, "DELETE") && c != "DELETE /api/v4/projects/1" && c != "DELETE /api/v4/projects/2" {
			t.Errorf("unexpected %s", c)
		}
	}
}

func TestDeleteScheduled(t *testing.T) {
	f := &fakeLab{scheduled: true, projects: []map[string]any{
		{"id": 1, "path": "mihari-test-x", "path_with_namespace": "bot/mihari-test-x", "namespace": map[string]any{"kind": "user", "path": "bot"}},
	}}
	connect(t, f)
	if _, err := DeleteTestProjects(context.Background()); err != nil {
		t.Fatal(err)
	}
	if last := f.calls[len(f.calls)-1]; last != "DELETE /api/v4/projects/1?permanently_remove" {
		t.Errorf("a scheduled deletion must be made permanent, last call %s", last)
	}
}
