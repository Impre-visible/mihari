package gitlab

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/settings"
)

// ErrNotConfigured means the URL or the token has not been saved yet.
var ErrNotConfigured = errors.New("GitLab n'est pas configuré")

// Todo is a pending item of the bot's GitLab to-do list on a merge request.
type Todo struct {
	ID int `json:"id"`
	// Why the bot got the todo: review_requested, assigned, mentioned or directly_addressed.
	Action    string    `json:"action"`
	Project   string    `json:"project"`
	ProjectID int       `json:"projectId"`
	MRIID     int       `json:"mrIid"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}

// Actions that ask the bot to do something; the rest (build_failed, unmergeable…) is noise.
var actionable = map[string]bool{
	"review_requested":   true,
	"assigned":           true,
	"mentioned":          true,
	"directly_addressed": true,
}

type apiTodo struct {
	ID         int       `json:"id"`
	ActionName string    `json:"action_name"`
	TargetURL  string    `json:"target_url"`
	CreatedAt  time.Time `json:"created_at"`
	Project    struct {
		ID                int    `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	Target struct {
		IID   int    `json:"iid"`
		Title string `json:"title"`
	} `json:"target"`
}

// PendingTodos returns the bot's pending merge request todos, oldest first,
// leaving out the ones the bot raised itself, using the URL and token saved
// in the settings.
func PendingTodos(ctx context.Context) ([]Todo, error) {
	config, err := (&settings.Service{}).GetConfig()
	if err != nil {
		return nil, err
	}
	rawURL, _ := config["gitlab_url"].(string)
	bot, _ := config["gitlab_username"].(string)
	token, err := settings.GitLabToken()
	if err != nil {
		return nil, err
	}
	if rawURL == "" || token == "" {
		return nil, ErrNotConfigured
	}
	base, err := parseBaseURL(rawURL)
	if err != nil {
		return nil, err
	}

	query := url.Values{
		"state":    {"pending"},
		"type":     {"MergeRequest"},
		"per_page": {"100"},
	}
	var items []apiTodo
	if err := get(ctx, base, token, "api/v4/todos", query, &items); err != nil {
		return nil, err
	}

	todos := make([]Todo, 0, len(items))
	// GitLab lists the newest first; work is done in arrival order.
	for i := len(items) - 1; i >= 0; i-- {
		t := items[i]
		// The bot mentioning itself in its own comments must not create work.
		if !actionable[t.ActionName] || strings.EqualFold(t.Author.Username, bot) {
			continue
		}
		todos = append(todos, Todo{
			ID:        t.ID,
			Action:    t.ActionName,
			Project:   t.Project.PathWithNamespace,
			ProjectID: t.Project.ID,
			MRIID:     t.Target.IID,
			Title:     t.Target.Title,
			URL:       t.TargetURL,
			Author:    t.Author.Username,
			CreatedAt: t.CreatedAt,
		})
	}
	return todos, nil
}
