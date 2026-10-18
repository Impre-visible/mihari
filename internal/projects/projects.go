// Package projects holds the per-project settings: whether the bot works
// there, who may call it, and the environment its jobs run in.
package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Impre-visible/mihari/internal/gitlab"
	"github.com/Impre-visible/mihari/internal/settings"
)

// DefaultImage is the container image of projects that set none, until the
// user picks another one.
const DefaultImage = "debian:stable-slim"

type Settings struct {
	// GitLab project id, which survives renames and transfers.
	ID int `json:"id"`
	// Last known path, to show a project the bot can no longer see.
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
	// Members at this GitLab access level or above may call the bot.
	MinRole int `json:"minRole"`
	// Members below MinRole who may call the bot anyway.
	ExtraUsers []string `json:"extraUsers"`
	// Members who may not call the bot, whatever their role.
	BlockedUsers []string `json:"blockedUsers"`
	// Overrides Config.DefaultImage when set.
	Image string `json:"image"`
	// Added to the profile's prompt for jobs on this project.
	Instructions string `json:"instructions"`
}

type Config struct {
	Projects     []Settings
	DefaultImage string
}

func Load() (Config, error) {
	raw, err := (&settings.Service{}).GetConfig()
	if err != nil {
		return Config{}, err
	}
	var c Config
	if value, ok := raw["projects"]; ok && value != nil {
		data, err := json.Marshal(value)
		if err != nil {
			return Config{}, err
		}
		if err := json.Unmarshal(data, &c.Projects); err != nil {
			return Config{}, fmt.Errorf("projets illisibles : %w", err)
		}
	}
	c.DefaultImage, _ = raw["default_image"].(string)
	if strings.TrimSpace(c.DefaultImage) == "" {
		c.DefaultImage = DefaultImage
	}
	for i := range c.Projects {
		if c.Projects[i].MinRole == 0 {
			c.Projects[i].MinRole = gitlab.AccessDeveloper
		}
	}
	return c, nil
}

// Project returns the settings of a project, or defaults for one never set up,
// which is off.
func (c Config) Project(id int) Settings {
	for _, p := range c.Projects {
		if p.ID == id {
			return p
		}
	}
	return Settings{ID: id, MinRole: gitlab.AccessDeveloper}
}

// Image returns the container image jobs on the project run in.
func (c Config) Image(id int) string {
	if image := strings.TrimSpace(c.Project(id).Image); image != "" {
		return image
	}
	return c.DefaultImage
}

// Authorizes reports whether username may call the bot, given the project's
// current members.
func (s Settings) Authorizes(username string, members []gitlab.Member) bool {
	if slices.ContainsFunc(s.BlockedUsers, func(b string) bool { return strings.EqualFold(b, username) }) {
		return false
	}
	for _, m := range members {
		if !strings.EqualFold(m.Username, username) {
			continue
		}
		if m.AccessLevel >= s.MinRole {
			return true
		}
		return slices.ContainsFunc(s.ExtraUsers, func(e string) bool { return strings.EqualFold(e, username) })
	}
	// Not a member at all: an extra user who left the project loses access.
	return false
}

// View is a project as the Projects page shows it.
type View struct {
	gitlab.Project
	Settings Settings `json:"settings"`
	// False for a project set up earlier that the bot can no longer see.
	Accessible bool `json:"accessible"`
}

// Service is exposed to the frontend through Wails bindings.
type Service struct{}

type Overview struct {
	Projects     []View `json:"projects"`
	DefaultImage string `json:"defaultImage"`
}

// List merges the bot's GitLab projects with their saved settings.
func (s *Service) List() (Overview, error) {
	config, err := Load()
	if err != nil {
		return Overview{}, err
	}
	found, err := gitlab.BotProjects(context.Background())
	if err != nil {
		return Overview{}, err
	}
	seen := map[int]bool{}
	views := make([]View, 0, len(found))
	for _, p := range found {
		seen[p.ID] = true
		settings := config.Project(p.ID)
		settings.Path = p.Path
		views = append(views, View{Project: p, Settings: settings, Accessible: true})
	}
	for _, p := range config.Projects {
		if !seen[p.ID] {
			views = append(views, View{Project: gitlab.Project{ID: p.ID, Path: p.Path, Name: p.Path}, Settings: p})
		}
	}
	return Overview{Projects: views, DefaultImage: config.DefaultImage}, nil
}

// Members lists the people with access to a project and their role.
func (s *Service) Members(projectID int) ([]gitlab.Member, error) {
	return gitlab.ProjectMembers(context.Background(), projectID)
}
