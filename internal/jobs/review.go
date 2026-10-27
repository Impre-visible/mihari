package jobs

import (
	"regexp"
	"strings"

	"github.com/Impre-visible/mihari/internal/gitlab"
)

var (
	mentionRe   = regexp.MustCompile(`@[\w.\-]+`)
	reviewWords = regexp.MustCompile(`(?i)\b(review|relis|relire|relecture|revue|regarde|avis)\b`)
	// Words that ask for the code to change, which a review request does not.
	changeWords = regexp.MustCompile(`(?i)\b(corrige\w*|modifie\w*|ajoute\w*|écri\w*|implémente\w*|fix\w*|pousse\w*|push\w*|change\w*|supprime\w*|renomme\w*|refactor\w*|remplace\w*|mets?|fais)\b`)
)

// reviewOnly tells a request to review a merge request, which allows reading
// and commenting but not changing the code: a review request on GitLab, a
// bare mention, or a mention asking for a review and nothing more.
func reviewOnly(j *job) bool {
	if j.TargetType != gitlab.MergeRequestTarget {
		return false
	}
	if j.todo.Action == "review_requested" {
		return true
	}
	body := strings.TrimSpace(mentionRe.ReplaceAllString(j.todo.Body, ""))
	return body == "" || reviewWords.MatchString(body) && !changeWords.MatchString(body)
}
