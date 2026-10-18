package jobs

import (
	"testing"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
)

func TestRestore(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	events := []agent.Event{{At: time.Now(), Kind: "tool", Title: "read_file", Text: "{}"}}
	save(record{Summary: Summary{ID: 3, State: StateSucceeded, Title: "fini"}, Events: events})
	save(record{Summary: Summary{ID: 7, State: StateRunning, Title: "coupé"}})

	s := &Service{}
	s.restore()
	list := s.List()
	if len(list) != 2 || list[0].ID != 7 || list[1].ID != 3 {
		t.Fatalf("history %+v", list)
	}
	if list[0].State != StateCanceled || list[0].Error == "" {
		t.Errorf("interrupted job: %+v", list[0])
	}
	if s.nextID != 7 {
		t.Errorf("nextID %d, want 7 so new jobs get fresh ids", s.nextID)
	}
	if got := s.Transcript(3); len(got) != 1 || got[0].Title != "read_file" {
		t.Errorf("transcript from disk: %+v", got)
	}
	// The interruption is saved, not re-detected on every start.
	if r, _ := loadRecord(7); r.Summary.State != StateCanceled {
		t.Errorf("stored state %s", r.Summary.State)
	}
}
