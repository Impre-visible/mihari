package gitlab

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/settings"
)

// ErrNotConfigured means the URL or the token has not been saved yet.
var ErrNotConfigured = errors.New("GitLab n'est pas configuré")

// Todo is a pending item of the bot's GitLab to-do list, on a merge request or an issue.
type Todo struct {
	ID int `json:"id"`
	// Why the bot got the todo: review_requested, assigned, mentioned or directly_addressed.
	Action     string     `json:"action"`
	TargetType TargetType `json:"targetType"`
	Project    string     `json:"project"`
	ProjectID  int        `json:"projectId"`
	// Of the merge request or the issue.
	IID       int       `json:"iid"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	// Text of the comment that mentioned the bot; empty for a review request.
	Body string `json:"body"`
	// Made on the Dev page rather than read from GitLab, so never marked done there.
	Simulated bool `json:"simulated"`
	// The comment that mentioned the bot; 0 for a review request.
	NoteID int `json:"noteId"`
}

// Target is what the todo is about.
func (t Todo) Target() Target {
	return Target{Type: t.TargetType, IID: t.IID}
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
	TargetType string    `json:"target_type"`
	TargetURL  string    `json:"target_url"`
	Body       string    `json:"body"`
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

// PendingTodos returns the bot's pending todos on merge requests and issues,
// oldest first, leaving out the ones the bot raised itself.
func PendingTodos(ctx context.Context) ([]Todo, error) {
	base, token, bot, err := connection()
	if err != nil {
		return nil, err
	}

	// Every page: todos left pending on switched-off projects must not hide new ones.
	items, err := getAll[apiTodo](ctx, base, token, "api/v4/todos", url.Values{"state": {"pending"}})
	if err != nil {
		return nil, err
	}

	todos := make([]Todo, 0, len(items))
	// GitLab lists the newest first; work is done in arrival order.
	for i := len(items) - 1; i >= 0; i-- {
		t := items[i]
		target := TargetType(t.TargetType)
		// The bot mentioning itself in its own comments must not create work.
		if !actionable[t.ActionName] || strings.EqualFold(t.Author.Username, bot) ||
			(target != MergeRequestTarget && target != IssueTarget) {
			continue
		}
		todos = append(todos, Todo{
			ID:         t.ID,
			Action:     t.ActionName,
			TargetType: target,
			Project:    t.Project.PathWithNamespace,
			ProjectID:  t.Project.ID,
			IID:        t.Target.IID,
			Title:      t.Target.Title,
			URL:        t.TargetURL,
			Author:     t.Author.Username,
			CreatedAt:  t.CreatedAt,
			Body:       t.Body,
			NoteID:     NoteIDFromURL(t.TargetURL),
		})
	}
	return todos, nil
}

// connection reads the instance URL, token and bot username from the settings.
func connection() (base *url.URL, token, bot string, err error) {
	config, err := (&settings.Service{}).GetConfig()
	if err != nil {
		return nil, "", "", err
	}
	rawURL, _ := config["gitlab_url"].(string)
	bot, _ = config["gitlab_username"].(string)
	token, err = settings.GitLabToken()
	if err != nil {
		return nil, "", "", err
	}
	if rawURL == "" || token == "" {
		return nil, "", "", ErrNotConfigured
	}
	base, err = parseBaseURL(rawURL)
	return base, token, bot, err
}

// SimulatedTodo builds the todo author mentioning the bot with body would
// create on the merge request or issue at webURL, without anything happening
// on GitLab. An empty author stands for the bot itself.
func SimulatedTodo(ctx context.Context, webURL, author, body string) (Todo, error) {
	base, token, bot, err := connection()
	if err != nil {
		return Todo{}, err
	}
	u, err := url.Parse(strings.TrimSpace(webURL))
	if err != nil || u.Host != base.Host {
		return Todo{}, fmt.Errorf("l'URL doit pointer vers une MR ou une issue de %s", base.Host)
	}
	path := strings.Trim(strings.TrimPrefix(u.Path, base.Path), "/")
	var project, iid string
	target := MergeRequestTarget
	for marker, t := range map[string]TargetType{
		"/-/merge_requests/": MergeRequestTarget, "/-/issues/": IssueTarget, "/-/work_items/": IssueTarget,
	} {
		if p, rest, ok := strings.Cut(path, marker); ok {
			project, target = p, t
			iid, _, _ = strings.Cut(rest, "/")
		}
	}
	if project == "" || iid == "" {
		return Todo{}, fmt.Errorf("pas une URL de merge request ou d'issue : %s", webURL)
	}

	var item struct {
		IID       int    `json:"iid"`
		Title     string `json:"title"`
		WebURL    string `json:"web_url"`
		ProjectID int    `json:"project_id"`
	}
	endpoint := "api/v4/projects/" + url.PathEscape(project) + "/" + target.collection() + "/" + iid
	if err := get(ctx, base, token, endpoint, nil, &item); err != nil {
		return Todo{}, err
	}

	action := "mentioned"
	if strings.TrimSpace(body) == "" && target == MergeRequestTarget {
		action = "review_requested"
	}
	return Todo{
		Action:     action,
		TargetType: target,
		Project:    project,
		ProjectID:  item.ProjectID,
		IID:        item.IID,
		Title:      item.Title,
		URL:        item.WebURL,
		Author:     cmp.Or(strings.TrimPrefix(strings.TrimSpace(author), "@"), bot),
		CreatedAt:  time.Now(),
		Body:       body,
		Simulated:  true,
	}, nil
}
