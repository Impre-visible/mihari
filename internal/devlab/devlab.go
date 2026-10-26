// Package devlab makes test projects on the bot's own GitLab account, for
// development builds: real code, merge requests with planted bugs to review,
// issues for the night profile. And deletes them, to start again from scratch.
package devlab

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/devmode"
	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/projects"
)

// One directory per project: project.json, the default branch in main/, and
// each merge request's files, full contents, in a directory named after its
// branch. The underscore keeps the Go tool from building their code, and a
// file is stored with a .fixture suffix where its real name would get in the
// way: go embed skips any directory holding a go.mod.
//
//go:embed all:_fixtures
var fixtures embed.FS

type manifest struct {
	Description   string `json:"description"`
	MergeRequests []struct {
		Branch      string   `json:"branch"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Deleted     []string `json:"deleted"`
	} `json:"mergeRequests"`
	Issues []struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Labels      []string `json:"labels"`
	} `json:"issues"`
}

// Specs reads the test projects embedded in the binary.
func Specs() ([]gitlab.TestSpec, error) {
	dirs, err := fs.ReadDir(fixtures, "_fixtures")
	if err != nil {
		return nil, err
	}
	var specs []gitlab.TestSpec
	for _, d := range dirs {
		root := path.Join("_fixtures", d.Name())
		data, err := fs.ReadFile(fixtures, path.Join(root, "project.json"))
		if err != nil {
			return nil, err
		}
		var m manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		spec := gitlab.TestSpec{Path: gitlab.TestPrefix + d.Name(), Description: m.Description}
		if spec.Files, err = files(path.Join(root, "main")); err != nil {
			return nil, err
		}
		for _, mr := range m.MergeRequests {
			changed, err := files(path.Join(root, mr.Branch))
			if err != nil {
				return nil, err
			}
			spec.MergeRequests = append(spec.MergeRequests, gitlab.TestChange{
				Branch: mr.Branch, Title: mr.Title, Description: mr.Description, Files: changed, Deleted: mr.Deleted,
			})
		}
		for _, is := range m.Issues {
			spec.Issues = append(spec.Issues, gitlab.TestIssue{Title: is.Title, Description: is.Description, Labels: is.Labels})
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// files reads every file under dir, by path relative to it.
func files(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := fs.WalkDir(fixtures, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fixtures, p)
		out[strings.TrimSuffix(strings.TrimPrefix(p, dir+"/"), ".fixture")] = string(data)
		return err
	})
	return out, err
}

// Service is exposed to the frontend through Wails bindings.
type Service struct{}

// List returns the test projects on the bot's account.
func (s *Service) List() ([]gitlab.TestProject, error) {
	if !devmode.Enabled {
		return nil, devmode.ErrDisabled
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return gitlab.TestProjects(ctx)
}

// Create makes the test projects the bot does not have yet, switches them on
// in Mihari, and adds invite to each as a developer when it is not "".
func (s *Service) Create(invite string) ([]gitlab.TestProject, error) {
	if !devmode.Enabled {
		return nil, devmode.ErrDisabled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	specs, err := Specs()
	if err != nil {
		return nil, err
	}
	existing, err := gitlab.TestProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, spec := range specs {
		if has(existing, spec.Path) {
			continue
		}
		spec.Invite = invite
		p, err := gitlab.CreateTestProject(ctx, spec)
		if err != nil {
			return nil, err
		}
		if err := projects.Enable(p.ID, p.Path); err != nil {
			return nil, err
		}
	}
	return gitlab.TestProjects(ctx)
}

// Reset deletes the test projects and makes them again, as new.
func (s *Service) Reset(invite string) ([]gitlab.TestProject, error) {
	if err := s.Delete(); err != nil {
		return nil, err
	}
	return s.Create(invite)
}

// Delete deletes the test projects and forgets their settings in Mihari.
func (s *Service) Delete() error {
	if !devmode.Enabled {
		return devmode.ErrDisabled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	deleted, err := gitlab.DeleteTestProjects(ctx)
	return errors.Join(err, projects.Forget(deleted))
}

// has reports whether one of the projects was made from the spec at path,
// suffixed or not.
func has(list []gitlab.TestProject, specPath string) bool {
	for _, p := range list {
		name := path.Base(p.Path)
		if name == specPath || strings.HasPrefix(name, specPath+"-") {
			return true
		}
	}
	return false
}
