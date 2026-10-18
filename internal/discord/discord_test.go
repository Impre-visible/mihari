package discord

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://discord.com/api/webhooks/123/abc":        true,
		"https://ptb.discord.com/api/webhooks/123/abc":    true,
		"https://discordapp.com/api/webhooks/123/abc":     true,
		"http://discord.com/api/webhooks/123/abc":         false,
		"https://evil.example/api/webhooks/123/abc":       false,
		"https://discord.com/channels/1/2":                false,
		"https://discord.com.evil.example/api/webhooks/1": false,
	} {
		if (validate(raw) == nil) != ok {
			t.Errorf("validate(%q) ok=%v", raw, !ok)
		}
	}
}

func TestPost(t *testing.T) {
	var got message
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// Rate limited once.
			w.Header().Set("Retry-After", "0.01")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := &Notifier{client: srv.Client()}
	long := strings.Repeat("é", 5000)
	if err := s.post(srv.URL, embed{Title: "Job en échec @everyone", Description: long, Color: red}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || got.Username != "Mihari" || len(got.Embeds) != 1 {
		t.Fatalf("calls=%d message=%+v", calls, got)
	}
	if got.AllowedMentions.Parse == nil || len(got.AllowedMentions.Parse) != 0 {
		t.Error("mentions must be disabled explicitly")
	}
	if n := len([]rune(got.Embeds[0].Description)); n > 3500 {
		t.Errorf("description of %d characters", n)
	}
}
