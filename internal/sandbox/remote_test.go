package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestIsRemote(t *testing.T) {
	for host, want := range map[string]bool{
		"ssh://root@bean:2700":        true,
		"tcp://10.0.0.2:2376":         true,
		"unix:///var/run/docker.sock": false,
		"npipe:////./pipe/docker":     false,
		"":                            false,
	} {
		if isRemote(host) != want {
			t.Errorf("%q: want %v", host, want)
		}
	}
}

func archive(t *testing.T, entries ...*tar.Header) *bytes.Buffer {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, h := range entries {
		body := []byte("x")
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			tw.Write(body)
		}
	}
	tw.Close()
	return &buf
}

func TestExtract(t *testing.T) {
	dir := t.TempDir()
	err := extract(archive(t,
		&tar.Header{Name: "workspace/", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "workspace/src/a.go", Typeflag: tar.TypeReg, Mode: 0o644},
		&tar.Header{Name: "workspace/.git/config", Typeflag: tar.TypeReg, Mode: 0o644},
		&tar.Header{Name: "workspace/link", Typeflag: tar.TypeSymlink, Linkname: "src/a.go"},
	), dir, "workspace")
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "src", "a.go")); string(data) != "x" {
		t.Errorf("a.go: %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		t.Error(".git must stay the host's")
	}
	if target, _ := os.Readlink(filepath.Join(dir, "link")); target != "src/a.go" {
		t.Errorf("link: %q", target)
	}
}

// The container is not trusted: nothing it sends lands outside the folder.
func TestExtractStaysInside(t *testing.T) {
	outside := t.TempDir()
	for name, entries := range map[string][]*tar.Header{
		"dot-dot": {{Name: "../escape", Typeflag: tar.TypeReg, Mode: 0o644}},
		"through a symlink": {
			{Name: "out", Typeflag: tar.TypeSymlink, Linkname: outside},
			{Name: "out/escape", Typeflag: tar.TypeReg, Mode: 0o644},
		},
	} {
		dir := t.TempDir()
		if err := extract(archive(t, entries...), dir, ""); err == nil {
			t.Errorf("%s: should fail", name)
		}
		if _, err := os.Stat(filepath.Join(outside, "escape")); err == nil {
			t.Fatalf("%s: wrote outside", name)
		}
	}
}

// Runs against the real runtime; skipped where none is ready.
func TestCopiedWorkspace(t *testing.T) {
	ctx := context.Background()
	if info := Detect(ctx); !info.Ready {
		t.Skip(info.Error)
	}
	if err := exec.Command("docker", "image", "inspect", "debian:stable-slim").Run(); err != nil {
		t.Skip("debian:stable-slim not pulled")
	}
	git := func(dir string, args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	dir := filepath.Join(t.TempDir(), "repo")
	os.Mkdir(dir, 0o755)
	git(dir, "init", "-q")
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0o644)
	os.WriteFile(filepath.Join(dir, "gone.txt"), []byte("gone"), 0o644)
	git(dir, "add", "-A")
	git(dir, "commit", "-qm", "init")

	c, err := Start(ctx, Spec{Image: "debian:stable-slim", Workspace: dir, Copy: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	res, _ := c.Exec(ctx, "test -d .git && cat keep.txt && echo new > new.txt && rm gone.txt && mkdir -p sub && echo deep > sub/d.txt", nil, 30*time.Second)
	if res.ExitCode != 0 || res.Stdout != "keep" {
		t.Fatalf("in the container: %+v", res)
	}
	// Not a mount: the host does not see it yet.
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err == nil {
		t.Fatal("the workspace should be a copy")
	}

	if err := c.Collect(ctx); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{"keep.txt": "keep", "new.txt": "new\n", "sub/d.txt": "deep\n"} {
		if data, _ := os.ReadFile(filepath.Join(dir, file)); string(data) != want {
			t.Errorf("%s: %q", file, data)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.txt")); err == nil {
		t.Error("a file deleted in the container should go on the host too")
	}

	// Mihari commits on the host; git in the container then agrees.
	git(dir, "add", "-A")
	git(dir, "commit", "-qm", "work")
	if err := c.Committed(ctx); err != nil {
		t.Fatal(err)
	}
	res, _ = c.Exec(ctx, "ls .git/refs/heads && cat .git/HEAD", nil, 30*time.Second)
	if res.ExitCode != 0 {
		t.Errorf(".git in the container: %+v", res)
	}
}
