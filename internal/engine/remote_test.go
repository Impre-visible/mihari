package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeRemote(t *testing.T) {
	var gotPath, gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/zen/v1/models":
			io.WriteString(w, `{"data":[{"id":"big-pickle"},{"id":"kimi-k2.6"}]}`)
		default:
			gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
			io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
		}
	}))
	defer upstream.Close()

	if _, err := serveRemote(context.Background(), upstream.URL+"/zen", "secret", "nope"); err == nil ||
		!strings.Contains(err.Error(), "kimi-k2.6") {
		t.Fatalf("unknown model should fail and list the others, got %v", err)
	}

	s, err := serveRemote(context.Background(), upstream.URL+"/zen", "secret", "kimi-k2.6")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s.URL, "http://127.0.0.1:") || s.Model != "kimi-k2.6" {
		t.Fatalf("server %+v", s)
	}
	// The client sends no key; the proxy adds it.
	res, err := http.Post(s.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || gotPath != "/zen/v1/chat/completions" || gotAuth != "Bearer secret" {
		t.Fatalf("status %d, upstream saw %s with %q", res.StatusCode, gotPath, gotAuth)
	}

	s.Stop()
	select {
	case <-s.Done():
	default:
		t.Fatal("Done still open after Stop")
	}
	if _, err := http.Get(s.URL + "/v1/models"); err == nil {
		t.Fatal("proxy still listening after Stop")
	}
}
