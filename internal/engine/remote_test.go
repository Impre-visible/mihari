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
	var gotPath, gotAuth, gotAgent, gotSession string
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
			gotAgent, gotSession = r.Header.Get("User-Agent"), r.Header.Get(sessionHeader)
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
	if !strings.HasPrefix(gotAgent, "mihari/") || gotSession == "" {
		t.Fatalf("OpenCode Go needs a user agent and a session, got %q and %q", gotAgent, gotSession)
	}
	// A job's own session goes through untouched.
	req, _ := http.NewRequest(http.MethodPost, s.URL+"/v1/chat/completions", strings.NewReader(`{}`))
	req.Header.Set(sessionHeader, "job-1")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if gotSession != "job-1" {
		t.Fatalf("session %q, want the job's", gotSession)
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

func TestRemoteChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"error":{"message":"Invalid credential"}}`)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"OK"}}]}`)
	}))
	defer upstream.Close()

	if text, err := remoteChat(context.Background(), upstream.URL, "secret", "m", "hi"); err != nil || text != "OK" {
		t.Fatalf("got %q, %v", text, err)
	}
	if _, err := remoteChat(context.Background(), upstream.URL, "bad", "m", "hi"); err == nil ||
		!strings.Contains(err.Error(), "Invalid credential") {
		t.Fatalf("a refused key should give the API's message, got %v", err)
	}
}
