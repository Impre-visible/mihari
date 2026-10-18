package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Runs against the real runtime; skipped where none is ready.
func TestContainer(t *testing.T) {
	ctx := context.Background()
	if info := Detect(ctx); !info.Ready {
		t.Skip(info.Error)
	}
	if err := exec.Command("docker", "image", "inspect", "debian:stable-slim").Run(); err != nil {
		t.Skip("debian:stable-slim not pulled")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("salut"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Start(ctx, Spec{Image: "debian:stable-slim", Workspace: dir, Env: []string{"FOO=bar"}})
	if err != nil {
		t.Fatal(err)
	}

	res, err := c.Exec(ctx, "pwd; cat hello.txt; echo $FOO; echo oops >&2; exit 3", nil, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stdout != "/workspace\nsalutbar\n" || res.Stderr != "oops\n" || res.ExitCode != 3 {
		t.Errorf("got %+v", res)
	}

	res, _ = c.Exec(ctx, "cat > from-stdin.txt", []byte("via stdin"), 30*time.Second)
	if data, _ := os.ReadFile(filepath.Join(dir, "from-stdin.txt")); string(data) != "via stdin" {
		t.Errorf("stdin: %q, %+v", data, res)
	}

	res, _ = c.Exec(ctx, "sleep 30", nil, 2*time.Second)
	if !res.TimedOut {
		t.Errorf("expected a timeout, got %+v", res)
	}

	res, _ = c.Exec(ctx, "head -c 200000 /dev/zero | tr '\\0' a", nil, 30*time.Second)
	if !res.Truncated || len(res.Stdout) > outputLimit+100 || !strings.Contains(res.Stdout, "octets coupés") {
		t.Errorf("expected truncated output, got %d bytes", len(res.Stdout))
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	// Files the container wrote belong to the user again, so the test can clean up.
	info, _ := os.Stat(filepath.Join(dir, "from-stdin.txt"))
	if st, ok := fileOwner(info); ok && st != os.Getuid() {
		t.Errorf("file owned by %d after Close", st)
	}
	if out, _ := exec.Command("docker", "ps", "-a", "--filter", "name="+c.name, "-q").Output(); len(out) > 0 {
		t.Errorf("container still there: %s", out)
	}
}
