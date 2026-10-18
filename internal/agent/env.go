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
	KindPing   Kind = "ping"
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
	MR     gitlab.MergeRequest
	// Thread of the comment that started the job; "" for a review request,
	// whose answer goes in a new comment.
	ThreadID  string
	Requester string
	// Commits the workspace and pushes it to the merge request's branch.
	Push func(ctx context.Context, message string) (string, error)

	// Set by tools.
	Replied  bool
	Finished *Finish
	Pushes   []string
}

// Finish is how the model ends the job.
type Finish struct {
	Success bool
	Summary string
}

type gitlabNote = gitlab.Note
