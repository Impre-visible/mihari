//go:build !windows

package engine

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Impre-visible/mihari/internal/profiles"
)

// fakeLlamaServer answers /v1/models on the --port it is given and records its arguments.
const fakeLlamaServer = `#!/usr/bin/env python3
import http.server, sys, os
args = sys.argv[1:]
open(os.environ["FAKE_ARGS"], "w").write(" ".join(args))
port = int(args[args.index("--port") + 1])
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200 if self.path == "/v1/models" else 404)
        self.end_headers()
    def log_message(self, *a): pass
http.server.HTTPServer(("127.0.0.1", port), H).serve_forever()
`

func TestStartLlamaServer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "llama-server"), []byte(fakeLlamaServer), 0o755); err != nil {
		t.Fatal(err)
	}
	argsFile := filepath.Join(dir, "args")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ARGS", argsFile)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	server, err := Start(ctx, profiles.Profile{
		Engine: "llama-server", Model: "/models/qwen.gguf",
		MaxJobs: 4, ContextPerJob: 8192, ExtraArgs: "-ngl 99 -fa on",
	})
	if err != nil {
		t.Fatal(err)
	}

	args, _ := os.ReadFile(argsFile)
	for _, want := range []string{"-np 4", "-c 32768", "-m /models/qwen.gguf", "--host 127.0.0.1", "-ngl 99 -fa on"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("args %q lack %q", args, want)
		}
	}

	server.Stop()
	select {
	case <-server.Done():
	default:
		t.Fatal("process still running after Stop")
	}
	if _, err := http.Get(server.URL + "/v1/models"); err == nil {
		t.Fatal("API still answers after Stop")
	}
}

func TestStartNotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	_, err := Start(context.Background(), profiles.Profile{Engine: "ollama", Model: "qwen3:32b", MaxJobs: 1, ContextPerJob: 1})
	if err == nil || !strings.Contains(err.Error(), "n'est pas installé") {
		t.Fatalf("got %v", err)
	}
}
