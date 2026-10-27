package jobs

import (
	"strings"
	"testing"

	"github.com/Impre-visible/mihari/internal/agent"
	"github.com/Impre-visible/mihari/internal/gitlab"
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

func TestTeachingReminderInTask(t *testing.T) {
	env := &agent.Env{MR: gitlab.MergeRequest{IID: 1, Title: "Remise par paliers", SourceBranch: "remise-paliers", TargetBranch: "main"}}
	j := &job{Summary: Summary{Kind: agent.KindPing}, todo: gitlab.Todo{Body: "@bot review stp"}}
	if strings.Contains(taskPrompt(j, env), "Mode pédagogique") {
		t.Error("off by default")
	}
	j.teaching = true
	task := taskPrompt(j, env)
	// Last in the task, after the request itself.
	if at := strings.Index(task, "Mode pédagogique"); at < strings.Index(task, "review stp") || !strings.Contains(task, "**Pourquoi**") {
		t.Errorf("the reminder ends the task: %q", task)
	}

	issue := &agent.Env{Issue: &gitlab.Issue{IID: 2, Title: "Tests pour ParseMontant"}}
	night := &job{Summary: Summary{Kind: agent.KindIssues}, teaching: true}
	if task := taskPrompt(night, issue); !strings.Contains(task, "ce que vérifie chaque test") || strings.Contains(task, "**Pourquoi**") {
		t.Errorf("a night job gets the reminder about code: %q", task)
	}
}
