package jobs

import (
	"testing"

	"github.com/Impre-visible/mihari/internal/gitlab"
)

func TestReviewOnly(t *testing.T) {
	for body, want := range map[string]bool{
		"@bot review stp":                      true,
		"@bot":                                 true,
		"@bot tu peux relire ?":                true,
		"@bot ton avis sur cette MR ?":         true,
		"@bot corrige le bug de la remise":     false,
		"@bot review et corrige ce qui va pas": false,
		"@bot ajoute des tests":                false,
		"@bot pourquoi ce choix ?":             false,
	} {
		j := &job{Summary: Summary{TargetType: gitlab.MergeRequestTarget}, todo: gitlab.Todo{Body: body}}
		if got := reviewOnly(j); got != want {
			t.Errorf("%q: %v, want %v", body, got, want)
		}
	}
	requested := &job{Summary: Summary{TargetType: gitlab.MergeRequestTarget}, todo: gitlab.Todo{Action: "review_requested"}}
	issue := &job{Summary: Summary{TargetType: gitlab.IssueTarget}, todo: gitlab.Todo{Body: "@bot review"}}
	if !reviewOnly(requested) || reviewOnly(issue) {
		t.Error("a review request on GitLab is one; nothing on an issue is")
	}
}
