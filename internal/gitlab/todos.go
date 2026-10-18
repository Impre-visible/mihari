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
	// Text of the comment that mentioned the bot; empty for a review request.
	Body string `json:"body"`
	// Made on the Dev page rather than read from GitLab, so never marked done there.
	Simulated bool `json:"simulated"`
	// The comment that mentioned the bot; 0 for a review request.
	NoteID int `json:"noteId"`
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

// PendingTodos returns the bot's pending merge request todos, oldest first,
// leaving out the ones the bot raised itself, using the URL and token saved
// in the settings.
func PendingTodos(ctx context.Context) ([]Todo, error) {
	base, token, bot, err := connection()
	if err != nil {
		return nil, err
	}

	// Every page: todos left pending on switched-off projects must not hide new ones.
	query := url.Values{"state": {"pending"}, "type": {"MergeRequest"}}
	items, err := getAll[apiTodo](ctx, base, token, "api/v4/todos", query)
	if err != nil {
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
			Body:      t.Body,
			NoteID:    NoteIDFromURL(t.TargetURL),
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

// MergeRequestTodo builds the todo author mentioning the bot with body would
// create on the merge request at webURL, without anything happening on GitLab.
// An empty author stands for the bot itself.
func MergeRequestTodo(ctx context.Context, webURL, author, body string) (Todo, error) {
	base, token, bot, err := connection()
	if err != nil {
		return Todo{}, err
	}
	u, err := url.Parse(strings.TrimSpace(webURL))
	if err != nil || u.Host != base.Host {
		return Todo{}, fmt.Errorf("l'URL doit pointer vers une MR de %s", base.Host)
	}
	project, iid, ok := strings.Cut(strings.Trim(strings.TrimPrefix(u.Path, base.Path), "/"), "/-/merge_requests/")
	iid, _, _ = strings.Cut(iid, "/")
	if !ok || project == "" || iid == "" {
		return Todo{}, fmt.Errorf("pas une URL de merge request : %s", webURL)
	}

	var mr struct {
		IID       int    `json:"iid"`
		Title     string `json:"title"`
		WebURL    string `json:"web_url"`
		ProjectID int    `json:"project_id"`
	}
	if err := get(ctx, base, token, "api/v4/projects/"+url.PathEscape(project)+"/merge_requests/"+iid, nil, &mr); err != nil {
		return Todo{}, err
	}

	action := "mentioned"
	if strings.TrimSpace(body) == "" {
		action = "review_requested"
	}
	return Todo{
		Action:    action,
		Project:   project,
		ProjectID: mr.ProjectID,
		MRIID:     mr.IID,
		Title:     mr.Title,
		URL:       mr.WebURL,
		Author:    cmp.Or(strings.TrimPrefix(strings.TrimSpace(author), "@"), bot),
		CreatedAt: time.Now(),
		Body:      body,
		Simulated: true,
	}, nil
}
