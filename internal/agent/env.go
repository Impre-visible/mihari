package agent

import (
	"context"
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

	// Set by tools.
	Replied  bool
	Finished *Finish
	Pushes   []string
	OpenedMR *gitlab.MergeRequest
}

// Finish is how the model ends the job.
type Finish struct {
	Success bool
	Summary string
}

type gitlabNote = gitlab.Note
