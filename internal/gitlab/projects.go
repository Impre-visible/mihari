package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// GitLab access levels, as the API returns them.
const (
	AccessGuest      = 10
	AccessReporter   = 20
	AccessDeveloper  = 30
	AccessMaintainer = 40
	AccessOwner      = 50
)

type Project struct {
	ID            int    `json:"id"`
	Path          string `json:"path"`
	Name          string `json:"name"`
	WebURL        string `json:"webUrl"`
	DefaultBranch string `json:"defaultBranch"`
}

type Member struct {
	Username    string `json:"username"`
	Name        string `json:"name"`
	AccessLevel int    `json:"accessLevel"`
}

// BotProjects lists the active projects the bot is a member of.
func BotProjects(ctx context.Context) ([]Project, error) {
	base, token, _, err := connection()
	if err != nil {
		return nil, err
	}
	type apiProject struct {
		ID                int    `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
		NameWithNamespace string `json:"name_with_namespace"`
		WebURL            string `json:"web_url"`
		DefaultBranch     string `json:"default_branch"`
	}
	query := url.Values{"membership": {"true"}, "simple": {"true"}, "archived": {"false"}, "order_by": {"path"}, "sort": {"asc"}}
	items, err := getAll[apiProject](ctx, base, token, "api/v4/projects", query)
	if err != nil {
		return nil, err
	}
	projects := make([]Project, len(items))
	for i, p := range items {
		projects[i] = Project{ID: p.ID, Path: p.PathWithNamespace, Name: p.NameWithNamespace, WebURL: p.WebURL, DefaultBranch: p.DefaultBranch}
	}
	return projects, nil
}

// ProjectMembers lists everyone with access to the project, including members
// inherited from its groups, with their highest access level.
func ProjectMembers(ctx context.Context, projectID int) ([]Member, error) {
	base, token, bot, err := connection()
	if err != nil {
		return nil, err
	}
	type apiMember struct {
		Username    string `json:"username"`
		Name        string `json:"name"`
		AccessLevel int    `json:"access_level"`
		State       string `json:"state"`
	}
	items, err := getAll[apiMember](ctx, base, token, "api/v4/projects/"+strconv.Itoa(projectID)+"/members/all", nil)
	if err != nil {
		return nil, err
	}
	members := make([]Member, 0, len(items))
	for _, m := range items {
		// The bot never authorizes itself, and blocked accounts cannot ping.
		if m.Username == bot || m.State == "blocked" {
			continue
		}
		members = append(members, Member{Username: m.Username, Name: m.Name, AccessLevel: m.AccessLevel})
	}
	return members, nil
}

// MarkTodoDone removes a todo from the bot's list on GitLab.
func MarkTodoDone(ctx context.Context, id int) error {
	base, token, _, err := connection()
	if err != nil {
		return err
	}
	_, err = call(ctx, http.MethodPost, base, token, "api/v4/todos/"+strconv.Itoa(id)+"/mark_as_done", nil, nil)
	return err
}
