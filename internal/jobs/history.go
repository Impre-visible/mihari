package jobs

import (
	"fmt"
	"slices"
	"strings"
)

// What session_search shows of earlier jobs.
const (
	// Jobs a search lists, newest first.
	historyHits = 8
	// Characters of one tool result, or of a whole transcript, when reading a job.
	historyResult = 600
	historyLimit  = 30000
)

// pastJobs lets a job search the stored jobs of its project, itself aside.
func pastJobs(projectID, self int) func(query string, jobID int) (string, error) {
	return func(query string, jobID int) (string, error) {
		var mine []record
		for _, r := range loadAll() {
			if r.Summary.ProjectID == projectID && r.Summary.ID != self {
				mine = append(mine, r)
			}
		}
		if jobID != 0 {
			for _, r := range mine {
				if r.Summary.ID == jobID {
					return readJob(r), nil
				}
			}
			return "", fmt.Errorf("aucun job %d sur ce projet", jobID)
		}
		return searchJobs(mine, query), nil
	}
}

var stateWords = map[State]string{
	StateQueued: "en attente", StateRunning: "en cours", StateSucceeded: "terminé",
	StateFailed: "en échec", StateCanceled: "annulé",
}

func heading(s Summary) string {
	line := fmt.Sprintf("job %d, %s : %s « %s » par @%s, %s",
		s.ID, s.QueuedAt.Format("02/01/2006 15:04"), s.target().Ref(), s.Title, s.Author, stateWords[s.State])
	if s.OpenedMR != "" {
		line += ", MR " + s.OpenedMR
	}
	return line
}

// searchJobs lists the jobs where every word of query appears, in the
// request, the outcome or the transcript, newest first.
func searchJobs(records []record, query string) string {
	words := strings.Fields(strings.ToLower(query))
	var b strings.Builder
	hits := 0
	for _, r := range slices.Backward(records) {
		text := strings.ToLower(r.Summary.Title + "\n" + r.Summary.Request + "\n" + r.Summary.Result + "\n" + r.Summary.Error)
		for _, e := range r.Events {
			text += "\n" + strings.ToLower(e.Text)
		}
		if !containsAll(text, words) {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", heading(r.Summary))
		if outcome := strings.TrimSpace(r.Summary.Result + " " + r.Summary.Error); outcome != "" {
			fmt.Fprintf(&b, "  %s\n", clip(outcome, 300))
		}
		if hits++; hits == historyHits {
			break
		}
	}
	if hits == 0 {
		if len(words) == 0 {
			return "Aucun job passé sur ce projet."
		}
		return "Aucun job passé ne contient tous ces mots."
	}
	return b.String() + "\nLis un job en entier avec job_id."
}

func containsAll(text string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// readJob condenses a job's transcript: the request, what the model said,
// its calls and the start of their results.
func readJob(r record) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nDemande :\n%s\n", heading(r.Summary), strings.TrimSpace(r.Summary.Request))
	for _, e := range r.Events {
		if e.Agent != "" {
			continue
		}
		switch e.Kind {
		case "assistant", "message":
			fmt.Fprintf(&b, "\n[%s] %s\n", e.Title, strings.TrimSpace(e.Text))
		case "tool":
			fmt.Fprintf(&b, "\n→ %s %s\n", e.Title, clip(e.Text, historyResult))
		case "result":
			fmt.Fprintf(&b, "  %s\n", strings.ReplaceAll(clip(e.Text, historyResult), "\n", "\n  "))
		case "error":
			fmt.Fprintf(&b, "\n[erreur] %s %s\n", e.Title, e.Text)
		}
	}
	fmt.Fprintf(&b, "\nIssue : %s %s\n", r.Summary.Result, r.Summary.Error)
	return clip(b.String(), historyLimit)
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
