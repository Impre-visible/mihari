package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Impre-visible/mihari/internal/devmode"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/projects"
)

// Service is exposed to the frontend through Wails bindings.
type Service struct{}

// Info reports the container runtime jobs would use.
func (s *Service) Info() Info {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return Detect(ctx)
}

// NewWorkspace makes an empty directory for one job's clone, under the user cache.
func NewWorkspace() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "mihari", "workspaces")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return os.MkdirTemp(dir, "job-")
}

type TryResult struct {
	Image  string `json:"image"`
	Branch string `json:"branch"`
	Result Result `json:"result"`
	// How long cloning and starting the container took, before the command.
	SetupMs int64 `json:"setupMs"`
}

// Try clones a project, runs command in its container as a job would, then
// removes both: a dry run of a job's environment for the Dev page.
func (s *Service) Try(projectID int, ref, command string) (TryResult, error) {
	if !devmode.Enabled {
		return TryResult{}, devmode.ErrDisabled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	config, err := projects.Load()
	if err != nil {
		return TryResult{}, err
	}
	var project gitlab.Project
	found, err := gitlab.BotProjects(ctx)
	if err != nil {
		return TryResult{}, err
	}
	for _, p := range found {
		if p.ID == projectID {
			project = p
		}
	}
	if project.ID == 0 {
		return TryResult{}, errors.New("le bot ne voit pas ce projet")
	}
	if ref == "" {
		ref = project.DefaultBranch
	}

	begin := time.Now()
	dir, err := NewWorkspace()
	if err != nil {
		return TryResult{}, err
	}
	defer os.RemoveAll(dir)
	repo := filepath.Join(dir, "repo")
	if err := gitlab.Clone(ctx, project.Path, ref, repo); err != nil {
		return TryResult{}, err
	}

	image := config.Image(projectID)
	c, err := Start(ctx, Spec{Image: image, Workspace: repo})
	if err != nil {
		return TryResult{}, err
	}
	defer c.Close()
	setup := time.Since(begin).Milliseconds()

	res, err := c.Exec(ctx, command, nil, 5*time.Minute)
	if err != nil {
		return TryResult{}, err
	}
	return TryResult{Image: image, Branch: ref, Result: res, SetupMs: setup}, nil
}
