package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCacheSurvivesJobs(t *testing.T) {
	ctx := context.Background()
	if info := Detect(ctx); !info.Ready {
		t.Skip(info.Error)
	}
	name := "test-" + time.Now().Format("150405.000")
	defer ClearCache(ctx, name)

	run := func(command string) string {
		c, err := Start(ctx, Spec{Image: "debian:stable-slim", Workspace: t.TempDir(), Cache: name})
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		res, err := c.Exec(ctx, command, nil, 30*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		return res.Stdout + res.Stderr
	}
	run(`mkdir -p "$npm_config_cache" && echo gardé > "$npm_config_cache/marker"`)
	if out := run(`cat /cache/npm/marker; echo "$GOMODCACHE"`); !strings.Contains(out, "gardé") || !strings.Contains(out, "/cache/go/mod") {
		t.Fatalf("second job saw %q", out)
	}
	if err := ClearCache(ctx, name); err != nil {
		t.Fatal(err)
	}
	if out := run(`cat /cache/npm/marker 2>&1 || true`); strings.Contains(out, "gardé") {
		t.Fatal("cache still there after ClearCache")
	}
}
