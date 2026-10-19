package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	// Keeps a note about the project for later jobs.
	Remember func(note string) error
	// Set for an incremental review: the commit the last review covered.
	ReviewedSHA string
	// Dry run: what would be posted on GitLab is kept in Posts instead.
	DryRun bool
	Posts  []Post
	// Messages from the thread not yet handed to the model; Run sets it.
	Inbox <-chan Event

	// Set by tools.
	Replied  bool
	Finished *Finish
	Pushes   []string
	OpenedMR *gitlab.MergeRequest
	// The model's plan, as todo_write last left it.
	Todos []Todo
	// The comment showing the plan on GitLab, 0 when there is none to update.
	PlanNoteID int
	// Whether the plan changed since a reply last carried it.
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

// ClosePlan removes the plan's own comment and returns the plan, for the
// answer to carry: one message at the end, which notifies, where editing
// the plan's comment would not. "" when no plan is waiting to be shown.
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

// Post is something a tool would have posted on GitLab during a dry run.
type Post struct {
	// comment, suggestion, reply.
	Kind string `json:"kind"`
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
}

// Keep records p instead of posting it, and reports whether the run is dry.
func (e *Env) Keep(p Post) bool {
	if e.DryRun {
		e.Posts = append(e.Posts, p)
	}
	return e.DryRun
}

// Finish is how the model ends the job.
type Finish struct {
	Success bool
	Summary string
}
