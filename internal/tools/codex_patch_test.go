package tools

import (
	"strings"
	"testing"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/sandbox"
)

func TestCodexChunks(t *testing.T) {
	file := "package main\n\nfunc greet() {\n\tprint(\"Hi\")\n}\n\nfunc other() {\n\tprint(\"Hi\")\n}\n"
	ops, err := parseCodexPatch(`*** Begin Patch
*** Update File: main.go
@@ func other() {
-	print("Hi")
+	print("Bye")
*** End Patch`)
	if err != nil {
		t.Fatal(err)
	}
	// The @@ line picks the second print, not the first.
	got, err := applyCodexChunks(file, "main.go", ops[0].chunks)
	if err != nil || got != strings.Replace(file, "other() {\n\tprint(\"Hi\")", "other() {\n\tprint(\"Bye\")", 1) {
		t.Errorf("got %q, %v", got, err)
	}

	// Typographic quotes and stray spaces still match.
	ops, _ = parseCodexPatch("*** Begin Patch\n*** Update File: a.txt\n-say “hi”  \n+say hello\n*** End Patch")
	if got, err := applyCodexChunks("say \"hi\"\n", "a.txt", ops[0].chunks); err != nil || got != "say hello\n" {
		t.Errorf("loose: %q, %v", got, err)
	}

	// A unified diff's hunk header is no context to look for; a blank line in a new file is one.
	ops, err = parseCodexPatch("*** Begin Patch\n*** Add File: n.md\n+# T\n\n+x\n*** Update File: a.txt\n@@ -1,2 +1,2 @@\n a\n-b\n+c\n*** End Patch")
	if err != nil || strings.Join(ops[0].lines, "|") != "# T||x" {
		t.Fatalf("lenient parse: %+v, %v", ops, err)
	}
	if got, err := applyCodexChunks("a\nb\n", "a.txt", ops[1].chunks); err != nil || got != "a\nc\n" {
		t.Errorf("unified header: %q, %v", got, err)
	}

	ops, _ = parseCodexPatch("*** Begin Patch\n*** Update File: a.txt\n-missing\n+x\n*** End Patch")
	if _, err := applyCodexChunks("abc\n", "a.txt", ops[0].chunks); err == nil {
		t.Error("lines that are not there should fail")
	}

	for _, bad := range []string{"no envelope", "*** Begin Patch\n*** Add File: a\nnot a plus line\n*** End Patch", "*** Begin Patch\n*** End Patch"} {
		if _, err := parseCodexPatch(bad); err == nil {
			t.Errorf("should fail: %q", bad)
		}
	}
}

func TestApplyCodexPatch(t *testing.T) {
	files := map[string]string{"/workspace/old.go": "a\nb\n", "/workspace/gone.txt": "x\n"}
	runner := &fakeRunner{respond: func(cmd string) sandbox.Result {
		for path, content := range files {
			if strings.Contains(cmd, "tail -c +1 '"+path+"'") {
				return sandbox.Result{Stdout: content}
			}
			if strings.Contains(cmd, "test -f '"+path+"'") {
				return sandbox.Result{}
			}
		}
		if strings.Contains(cmd, "test -f") {
			return sandbox.Result{ExitCode: 1}
		}
		return sandbox.Result{}
	}}
	out, err := run(t, &agent.Env{Runner: runner}, "apply_patch", map[string]any{"patch": `*** Begin Patch
*** Add File: new.txt
+hello
*** Update File: old.go
*** Move to: moved.go
 a
-b
+c
*** Delete File: gone.txt
*** End Patch`})
	if err != nil {
		t.Fatal(err)
	}
	if out != "Patch appliqué.\nA new.txt\nM old.go → moved.go\nD gone.txt" {
		t.Errorf("summary %q", out)
	}
	writes := map[string]string{}
	for i, cmd := range runner.commands {
		if strings.Contains(cmd, "cat > ") {
			writes[cmd[strings.LastIndex(cmd, "'/workspace/")+1:len(cmd)-1]] = runner.stdin[i]
		}
	}
	if writes["/workspace/new.txt"] != "hello\n" || writes["/workspace/moved.go"] != "a\nc\n" {
		t.Errorf("writes %v", writes)
	}

	// A hunk that does not apply leaves every file alone.
	runner.commands = nil
	if _, err := run(t, &agent.Env{Runner: runner}, "apply_patch", map[string]any{"patch": "*** Begin Patch\n*** Add File: x\n+x\n*** Update File: old.go\n-zzz\n+y\n*** End Patch"}); err == nil {
		t.Fatal("should fail")
	}
	for _, cmd := range runner.commands {
		if strings.Contains(cmd, "cat > ") || strings.HasPrefix(cmd, "rm ") {
			t.Errorf("wrote despite the failure: %q", cmd)
		}
	}
}
