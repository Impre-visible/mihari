package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/sandbox"
)

// Kind is what started a job, which decides some of the tools it gets.
type Kind string

const (
	// A mention or a review request, answered in its thread.
	KindPing Kind = "ping"
	// An issue carrying the profile's label, worked through at night.
	KindIssues Kind = "issues"
)

// Runner executes commands in the job's container.
type Runner interface {
	Exec(ctx context.Context, command string, stdin []byte, timeout time.Duration) (sandbox.Result, error)
}

// Env is everything a tool may act on during one job. Tools read it and
// record what they did in it; the job reads the outcome when the loop ends.
type Env struct {
	Kind   Kind
	Runner Runner
	GitLab *gitlab.Client
	// Where the request came from: a merge request or an issue.
	Target gitlab.Target
	// The merge request worked on: the target, or the one the agent opened.
	MR    gitlab.MergeRequest
	Issue *gitlab.Issue
	// Thread of the comment that started the job; "" when answers go in a new comment.
	ThreadID  string
	Requester string
	// Commits the workspace and pushes it to the job's branch; nil where pushing is not allowed.
	Push func(ctx context.Context, message string) (string, error)
	// Pushes the job's branch and opens a draft merge request; nil where not allowed.
	OpenMR func(ctx context.Context, title, description string) (gitlab.MergeRequest, error)
	// The project's notes, kept across jobs, and how to replace them; nil
	// where they are not kept, as in a dry run.
	Notes    func() string
	SetNotes func(notes string) error
	// Finds earlier jobs of the project matching query, or reads job jobID
	// when it is not 0; nil where there is no history.
	PastJobs func(query string, jobID int) (string, error)
	// The model sees images, so read_image may hand it some.
	Vision bool
	// A subagent's: only tools that change nothing are offered.
	ReadOnly bool
	// Runs task in a read-only subagent and returns its conclusion; nil in a subagent.
	Subagent func(ctx context.Context, task string) (string, error)
	// Set for an incremental review: the commit the last review covered.
	ReviewedSHA string
	// Dry run: what would be posted on GitLab is kept in Posts instead.
	DryRun bool
	Posts  []Post
	// Messages from the thread not yet handed to the model; Run sets it.
	Inbox <-chan Event

	// What is already on GitLab, for Again.
	posted []Post

	// Set by tools.
	Replied  bool
	Finished *Finish
	Pushes   []string
	OpenedMR *gitlab.MergeRequest
	// Images read_image loaded, as data URLs, for the loop to hand the model.
	Images []string
	// The model's plan, as todo_write last left it.
	Todos []Todo
	// The comment showing the plan on GitLab, 0 when there is none to update:
	// the plan's own, or the answer that took it in.
	PlanNoteID int
	// What follows the plan in that comment: the answer, "" in the plan's own.
	PlanBody string
	// Whether the plan's own comment waits for an answer to take it in.
	PlanOpen bool
}

// Todo is a step of the model's plan.
type Todo struct {
	Content string `json:"content"`
	// pending, in_progress or completed.
	Status string `json:"status" enum:"pending,in_progress,completed"`
}

// Respond posts body where the requester looks: in the thread that started
// the job, or as a new comment.
func (e *Env) Respond(ctx context.Context, body string) (gitlab.Note, error) {
	if e.ThreadID != "" {
		return e.GitLab.Reply(ctx, e.Target, e.ThreadID, body)
	}
	return e.GitLab.Comment(ctx, e.Target, body)
}

// Plan writes the plan as a GitLab task list.
func (e *Env) Plan() string {
	var b strings.Builder
	b.WriteString("**Plan**\n\n")
	for _, t := range e.Todos {
		switch t.Status {
		case "completed":
			fmt.Fprintf(&b, "- [x] %s\n", t.Content)
		case "in_progress":
			fmt.Fprintf(&b, "- [ ] ⏳ **%s**\n", t.Content)
		default:
			fmt.Fprintf(&b, "- [ ] %s\n", t.Content)
		}
	}
	return b.String()
}

// PlanNote is the text of the comment showing the plan.
func (e *Env) PlanNote() string {
	if e.PlanBody == "" {
		return e.Plan()
	}
	return e.Plan() + "\n" + e.PlanBody
}

// ClosePlan removes the plan's own comment and returns the plan, for the
// answer to carry: one message at the end, which notifies, where editing
// the plan's comment would not. "" when no plan is waiting to be shown.
// The answer then calls PlanTakenIn, so that later plan updates edit it.
func (e *Env) ClosePlan(ctx context.Context) string {
	if !e.PlanOpen {
		return ""
	}
	if e.PlanNoteID != 0 && e.GitLab != nil {
		_ = e.GitLab.DeleteNote(ctx, e.Target, e.PlanNoteID)
	}
	e.PlanNoteID, e.PlanOpen = 0, false
	return e.Plan() + "\n"
}

// PlanTakenIn records the answer that now shows the plan above body.
func (e *Env) PlanTakenIn(noteID int, body string) {
	e.PlanNoteID, e.PlanBody = noteID, body
}

// Post is something a tool would have posted on GitLab during a dry run.
type Post struct {
	// comment, suggestion, reply.
	Kind string `json:"kind"`
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
	// The thread it is or went in, when known.
	Thread string `json:"thread,omitempty"`
}

// Keep records p instead of posting it, and reports whether the run is dry.
func (e *Env) Keep(p Post) bool {
	if e.DryRun {
		e.Posts = append(e.Posts, p)
	}
	return e.DryRun
}

// Again refuses a post that would repeat one already on GitLab. Models
// reword rather than copy, so it compares meaning by shared words: a remark
// close to an earlier one, a few lines apart at most in the same file, or a
// reply much like an earlier one in the same thread. A second comment on a
// line already commented goes in that thread instead.
func (e *Env) Again(p Post) error {
	words := wordsOf(p.Body)
	for _, q := range e.posted {
		switch {
		case p.Line > 0 && q.Line > 0:
			if p.Path != q.Path || abs(p.Line-q.Line) > nearbyLines {
				continue
			}
			if similar(words, wordsOf(q.Body)) {
				return fmt.Errorf("tu as déjà fait cette remarque sur %s:%d%s : « %s ». Rien n'a été posté : passe à la suite",
					q.Path, q.Line, inThread(q.Thread), clip(q.Body, 300))
			}
			if p.Line == q.Line {
				return fmt.Errorf("tu as déjà commenté %s:%d%s : « %s ». Rien n'a été posté. Pour un autre problème sur cette ligne, réponds dans ce fil avec reply_to_thread",
					q.Path, q.Line, inThread(q.Thread), clip(q.Body, 300))
			}
		case p.Line == 0 && q.Line == 0 && p.Thread == q.Thread && similar(words, wordsOf(q.Body)):
			return fmt.Errorf("tu as déjà posté une réponse presque identique%s : « %s ». Rien n'a été posté : passe à la suite",
				inThread(q.Thread), clip(q.Body, 300))
		}
	}
	return nil
}

// Sent records a post once on GitLab, for Again: one made in this job, or
// one the bot left in an earlier job.
func (e *Env) Sent(p Post) {
	e.posted = append(e.posted, p)
}

// How far apart two remarks on the same file may be and still be one.
const nearbyLines = 3

func inThread(id string) string {
	if id == "" {
		return ""
	}
	return " (fil " + id + ")"
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// wordsOf returns the distinct words of text worth comparing: lowercase,
// three letters or more.
func wordsOf(text string) map[string]bool {
	words := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}) {
		if len([]rune(w)) >= 3 {
			words[w] = true
		}
	}
	return words
}

// similar reports two texts saying much the same: half their words shared,
// or one nearly contained in the other.
func similar(a, b map[string]bool) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	shared := 0
	for w := range a {
		if b[w] {
			shared++
		}
	}
	union := len(a) + len(b) - shared
	return float64(shared)/float64(union) >= 0.5 || float64(shared)/float64(min(len(a), len(b))) >= 0.8
}

// Finish is how the model ends the job.
type Finish struct {
	Success bool
	Summary string
}
