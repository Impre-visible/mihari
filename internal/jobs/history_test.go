package jobs

import (
	"strings"
	"testing"
	"time"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
)

func TestSearchJobs(t *testing.T) {
	at := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	records := []record{
		{Summary: Summary{ID: 1, ProjectID: 5, TargetType: gitlab.IssueTarget, IID: 2, Title: "Migration Postgres", Author: "romeo", State: StateSucceeded, QueuedAt: at, Result: "MR ouverte."},
			Events: []agent.Event{{Kind: "tool", Title: "run_command", Text: `{"command":"make migrate"}`}, {Kind: "result", Title: "run_command", Text: "ok"}}},
		{Summary: Summary{ID: 2, ProjectID: 5, TargetType: gitlab.MergeRequestTarget, IID: 9, Title: "Review", Author: "romeo", State: StateFailed, QueuedAt: at, Error: "délai dépassé"},
			Events: []agent.Event{{Kind: "assistant", Title: "Modèle", Text: "Je relis la migration."}, {Kind: "tool", Title: "read_file", Text: "{}", Agent: "subagent"}}},
	}
	out := searchJobs(records, "MIGRATION")
	if !strings.Contains(out, "job 2") || !strings.Contains(out, "job 1") || strings.Index(out, "job 2") > strings.Index(out, "job 1") {
		t.Errorf("newest first: %s", out)
	}
	if out := searchJobs(records, "migrate postgres"); !strings.Contains(out, "job 1") || strings.Contains(out, "job 2") {
		t.Errorf("every word: %s", out)
	}
	if out := searchJobs(records, "kubernetes"); !strings.HasPrefix(out, "Aucun job") {
		t.Errorf("no match: %s", out)
	}
	job := readJob(records[1])
	if !strings.Contains(job, "Je relis la migration.") || strings.Contains(job, "read_file") || !strings.Contains(job, "en échec") {
		t.Errorf("read: %s", job)
	}
}
