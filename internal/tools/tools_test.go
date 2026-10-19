package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/sandbox"
)

// run calls the registered tool name with args.
func run(t *testing.T, env *agent.Env, name string, args any) (string, error) {
	t.Helper()
	for _, tool := range agent.Registered() {
		if tool.Name() == name {
			data, _ := json.Marshal(args)
			return tool.Run(context.Background(), env, data)
		}
	}
	t.Fatalf("no tool %s", name)
	return "", nil
}

// Every tool has its view in the job's chat, named after it.
func TestEveryToolHasAView(t *testing.T) {
	dir := filepath.Join("..", "..", "frontend", "src", "pages", "components", "chat", "tools")
	for _, tool := range agent.Registered() {
		var name strings.Builder
		for _, part := range strings.Split(tool.Name(), "_") {
			name.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
		if _, err := os.Stat(filepath.Join(dir, name.String()+".svelte")); err != nil {
			t.Errorf("%s has no view: %v", tool.Name(), err)
		}
	}
}

func TestReplace(t *testing.T) {
	got, line, err := replace("a\nb\nc\nb\n", "c\nb", "C", false)
	if err != nil || got != "a\nb\nC\n" || line != 3 {
		t.Errorf("got %q, %d, %v", got, line, err)
	}
	if _, _, err := replace("b b", "b", "x", false); err == nil || !strings.Contains(err.Error(), "2 fois") {
		t.Errorf("ambiguous: %v", err)
	}
	if got, _, _ := replace("b b", "b", "x", true); got != "x x" {
		t.Errorf("all: %q", got)
	}
	if _, _, err := replace("abc", "z", "x", false); err == nil {
		t.Error("missing should fail")
	}
}

func TestEditFile(t *testing.T) {
	runner := &fakeRunner{respond: func(cmd string) sandbox.Result {
		if strings.Contains(cmd, "tail -c +1 ") {
			return sandbox.Result{Stdout: "func a() {\n\treturn 1\n}\n"}
		}
		return sandbox.Result{}
	}}
	env := &agent.Env{Runner: runner}
	out, err := run(t, env, "edit_file", map[string]any{"path": "a.go", "old_string": "return 1", "new_string": "return 2"})
	if err != nil || out != "a.go modifié à la ligne 2." {
		t.Fatalf("got %q, %v", out, err)
	}
	if runner.stdin[len(runner.stdin)-1] != "func a() {\n\treturn 2\n}\n" || !strings.Contains(runner.last(), "cat > '/workspace/a.go'") {
		t.Errorf("wrote %q with %q", runner.stdin[len(runner.stdin)-1], runner.last())
	}
}

func TestReadFileInChunks(t *testing.T) {
	big := strings.Repeat("x", chunk)
	calls := 0
	runner := &fakeRunner{respond: func(cmd string) sandbox.Result {
		calls++
		if calls == 1 {
			return sandbox.Result{Stdout: big}
		}
		return sandbox.Result{Stdout: "fin"}
	}}
	got, err := readFile(context.Background(), &agent.Env{Runner: runner}, "/f", 1<<20)
	if err != nil || got != big+"fin" || !strings.Contains(runner.commands[1], "tail -c +49153 ") {
		t.Errorf("got %d bytes, %v, %q", len(got), err, runner.commands)
	}
}

func TestGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		match         bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "internal/a/b.go", true},
		{"*.go", "main.gox", false},
		{"src/*.ts", "src/a.ts", true},
		{"src/*.ts", "src/x/a.ts", false},
		{"src/**/*.ts", "src/a.ts", true},
		{"src/**/*.ts", "src/x/y/a.ts", true},
		{"**/{Dockerfile,*.yml}", "build/ci.yml", true},
		{"**/{Dockerfile,*.yml}", "Dockerfile", true},
		{"a?c.txt", "abc.txt", true},
		{"a.c", "abc", false},
	}
	for _, c := range cases {
		re, err := globRegexp(c.pattern)
		if err != nil {
			t.Fatal(err)
		}
		if re.MatchString(c.path) != c.match {
			t.Errorf("%s on %s: want %v", c.pattern, c.path, c.match)
		}
	}
	runner := &fakeRunner{respond: func(string) sandbox.Result {
		return sandbox.Result{Stdout: "b/x.go\na.go\nc.txt\n"}
	}}
	out, err := run(t, &agent.Env{Runner: runner}, "glob", map[string]any{"pattern": "*.go"})
	if err != nil || out != "a.go\nb/x.go" || !strings.Contains(runner.last(), "-name '*.go'") {
		t.Errorf("got %q, %v, %q", out, err, runner.last())
	}
}

func TestSearchFlags(t *testing.T) {
	runner := &fakeRunner{}
	_, _ = run(t, &agent.Env{Runner: runner}, "search", map[string]any{"pattern": "foo", "glob": "*.go", "ignore_case": true, "context": 50})
	if cmd := runner.last(); !strings.Contains(cmd, "-i -C10 --include='*.go'") {
		t.Errorf("command %q", cmd)
	}
}

func TestDeleteRefusesRoot(t *testing.T) {
	if _, err := run(t, &agent.Env{Runner: &fakeRunner{}}, "delete_file", map[string]any{"path": ".", "recursive": true}); err == nil {
		t.Error("deleting the repository should fail")
	}
}

func TestGitTools(t *testing.T) {
	runner := &fakeRunner{respond: func(string) sandbox.Result { return sandbox.Result{Stdout: "diff --git a/x b/x\n"} }}
	env := &agent.Env{Runner: runner}
	if _, err := run(t, env, "git_diff", map[string]any{"path": "src"}); err != nil {
		t.Fatal(err)
	}
	cmd := runner.last()
	if !strings.Contains(cmd, "safe.directory") || !strings.Contains(cmd, "diff 'HEAD' -- '/workspace/src'") || !strings.Contains(cmd, "ls-files -z --others") {
		t.Errorf("git_diff ran %q", cmd)
	}
	if _, err := run(t, env, "git_log", map[string]any{"ref": "--output=/etc/x"}); err == nil {
		t.Error("an option as ref should fail")
	}
	if _, err := run(t, env, "apply_patch", map[string]any{"patch": "--- a/x\n+++ b/x"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(runner.stdin[len(runner.stdin)-1], "\n") || !strings.Contains(runner.last(), "apply") {
		t.Errorf("apply_patch ran %q", runner.last())
	}
}

func TestProcesses(t *testing.T) {
	runner := &fakeRunner{respond: func(string) sandbox.Result { return sandbox.Result{Stdout: "[en cours, pid 42]\nready\n"} }}
	env := &agent.Env{Runner: runner}
	out, err := run(t, env, "start_process", map[string]any{"command": "npm run dev", "name": "web"})
	if err != nil || !strings.Contains(out, `"web" lancé`) {
		t.Fatalf("got %q, %v", out, err)
	}
	if cmd := runner.last(); !strings.Contains(cmd, "setsid sh -c 'npm run dev' >/tmp/mihari/proc/web.log") {
		t.Errorf("start ran %q", cmd)
	}
	if _, err := run(t, env, "stop_process", map[string]any{"name": "../etc"}); err == nil {
		t.Error("a name outside the process directory should fail")
	}
	if _, err := run(t, env, "read_process", map[string]any{"name": "web"}); err != nil {
		t.Fatal(err)
	}
}

func TestTodoWrite(t *testing.T) {
	env := &agent.Env{}
	out, err := run(t, env, "todo_write", map[string]any{"todos": []map[string]string{
		{"content": "Lire", "status": "completed"}, {"content": "Corriger", "status": "in_progress"}}})
	if err != nil || out != "Plan à jour : 1/2 étapes faites." || !env.PlanOpen || len(env.Todos) != 2 {
		t.Fatalf("got %q, %v, %+v", out, err, env)
	}
	if _, err := run(t, env, "todo_write", map[string]any{"todos": []map[string]string{{"content": "x", "status": "done"}}}); err == nil {
		t.Error("an unknown status should fail")
	}

	// A dry run keeps the plan, and the reply carries it.
	env = &agent.Env{DryRun: true, Target: gitlab.Target{Type: gitlab.IssueTarget, IID: 3}}
	_, _ = run(t, env, "todo_write", map[string]any{"todos": []map[string]string{{"content": "Lire", "status": "completed"}}})
	_, _ = run(t, env, "reply", map[string]any{"body": "Fait."})
	if len(env.Posts) != 2 || env.Posts[0].Kind != "plan" || env.Posts[1].Body != "**Plan**\n\n- [x] Lire\n\nFait." {
		t.Errorf("posts %+v", env.Posts)
	}

	// Ticking the plan after the answer updates the answer, not a new plan comment.
	_, _ = run(t, env, "todo_write", map[string]any{"todos": []map[string]string{
		{"content": "Lire", "status": "completed"}, {"content": "Répondre", "status": "completed"}}})
	if last := env.Posts[len(env.Posts)-1]; len(env.Posts) != 3 || last.Body != "**Plan**\n\n- [x] Lire\n- [x] Répondre\n\nFait." || env.PlanOpen {
		t.Errorf("after the answer: %+v", env.Posts)
	}
	if env.ClosePlan(context.Background()) != "" {
		t.Error("the answer already shows the plan")
	}

	// A job that ends well ticks the step still under way in the answer.
	env = &agent.Env{DryRun: true, Target: gitlab.Target{Type: gitlab.IssueTarget, IID: 3}}
	_, _ = run(t, env, "todo_write", map[string]any{"todos": []map[string]string{
		{"content": "Lire", "status": "completed"}, {"content": "Répondre", "status": "in_progress"}, {"content": "Plus tard", "status": "pending"}}})
	_, _ = run(t, env, "reply", map[string]any{"body": "Fait."})
	_, _ = run(t, env, "finish", map[string]any{"success": true, "summary": "ok"})
	if last := env.Posts[len(env.Posts)-1]; last.Body != "**Plan**\n\n- [x] Lire\n- [x] Répondre\n- [ ] Plus tard\n\nFait." {
		t.Errorf("after finish: %q", last.Body)
	}
}

func TestAskUser(t *testing.T) {
	// A mention in an issue's description has no thread: the question is a new comment.
	env := &agent.Env{Kind: agent.KindPing, DryRun: true, Requester: "romeo", Target: gitlab.Target{Type: gitlab.IssueTarget, IID: 3}}
	for _, tool := range agent.Registered() {
		if tool.Name() == "ask_user" && !tool.Available(env) {
			t.Error("ask_user should be offered to a ping without a thread")
		}
	}
	out, err := run(t, env, "ask_user", map[string]any{"question": "Quelle base ?", "choices": []string{"Postgres", "SQLite"}})
	if err != nil || !strings.Contains(out, "Décide seul") {
		t.Fatalf("got %q, %v", out, err)
	}
	if env.Posts[0].Body != "@romeo Quelle base ?\n1. Postgres\n2. SQLite" {
		t.Errorf("posted %q", env.Posts[0].Body)
	}
	for _, tool := range agent.Registered() {
		if tool.Name() == "ask_user" && tool.Available(&agent.Env{Kind: agent.KindIssues, ThreadID: "t", Target: env.Target}) {
			t.Error("nobody answers an issues job: ask_user is for pings")
		}
	}
}

func TestExecuteCode(t *testing.T) {
	runner := &fakeRunner{respond: func(string) sandbox.Result { return sandbox.Result{Stdout: "42\n"} }}
	out, err := run(t, &agent.Env{Runner: runner}, "execute_code", map[string]any{"code": "print(6*7)"})
	if err != nil || !strings.HasPrefix(out, "42\n") {
		t.Fatalf("got %q, %v", out, err)
	}
	if cmd := runner.last(); !strings.Contains(cmd, "command -v python3") || !strings.Contains(cmd, ".py && python3 /tmp/mihari/code/") || runner.stdin[0] != "print(6*7)" {
		t.Errorf("ran %q", cmd)
	}
	if _, err := run(t, &agent.Env{Runner: runner}, "execute_code", map[string]any{"code": "x", "language": "ruby"}); err == nil {
		t.Error("unknown language should fail")
	}
}

func TestSessionSearch(t *testing.T) {
	var asked string
	var id int
	env := &agent.Env{PastJobs: func(q string, j int) (string, error) { asked, id = q, j; return "job 3", nil }}
	if out, err := run(t, env, "session_search", map[string]any{"query": "migration", "job_id": 3}); err != nil || out != "job 3" || asked != "migration" || id != 3 {
		t.Errorf("got %q, %v", out, err)
	}
}
