package jobs

import (
	"strings"
	"testing"
)

func TestTeachingPrompt(t *testing.T) {
	repo := t.TempDir()
	plain := systemPrompt("Relis la MR.", "", false, repo, "")
	teaching := systemPrompt("Relis la MR.", "Les tests sont en pytest.", true, repo, "")
	if strings.Contains(plain, "Mode pédagogique") {
		t.Error("off by default")
	}
	// The team's level comes before the project's own instructions, which may refine it.
	at, instructions := strings.Index(teaching, "Mode pédagogique"), strings.Index(teaching, "Les tests sont en pytest.")
	if at < 0 || instructions < at || !strings.HasPrefix(teaching, "Relis la MR.") {
		t.Errorf("teaching rules after the profile's prompt, before the project's: %q", teaching[:min(len(teaching), 300)])
	}
}
