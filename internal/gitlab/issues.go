package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Issue struct {
	ProjectID   int      `json:"projectId"`
	IID         int      `json:"iid"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Labels      []string `json:"labels"`
	WebURL      string   `json:"webUrl"`
	State       string   `json:"state"`
}

type apiIssue struct {
	IID         int      `json:"iid"`
	ProjectID   int      `json:"project_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	WebURL      string   `json:"web_url"`
	State       string   `json:"state"`
	Author      struct {
		Username string `json:"username"`
	} `json:"author"`
}

func (i apiIssue) convert() Issue {
	return Issue{ProjectID: i.ProjectID, IID: i.IID, Title: i.Title, Description: i.Description,
		Author: i.Author.Username, Labels: i.Labels, WebURL: i.WebURL, State: i.State}
}

func (c *Client) Issue(ctx context.Context, iid int) (Issue, error) {
	var i apiIssue
	err := get(ctx, c.base, c.token, c.path(Target{Type: IssueTarget, IID: iid}, ""), nil, &i)
	return i.convert(), err
}

// OpenIssues lists the project's open issues carrying any of labels.
func (c *Client) OpenIssues(ctx context.Context, labels ...string) ([]Issue, error) {
	var all []Issue
	seen := map[int]bool{}
	// GitLab ANDs the labels of one query; each label gets its own.
	for _, label := range labels {
		query := url.Values{"state": {"opened"}, "labels": {label}}
		items, err := getAll[apiIssue](ctx, c.base, c.token, "api/v4/projects/"+strconv.Itoa(c.ProjectID)+"/issues", query)
		if err != nil {
			return nil, err
		}
		for _, i := range items {
			if !seen[i.IID] {
				seen[i.IID] = true
				all = append(all, i.convert())
			}
		}
	}
	return all, nil
}

// LabelEvent is someone adding or removing a label.
type LabelEvent struct {
	User  string
	Label string
	Added bool
	At    time.Time
}

func (c *Client) LabelEvents(ctx context.Context, iid int) ([]LabelEvent, error) {
	type apiEvent struct {
		Action    string    `json:"action"`
		CreatedAt time.Time `json:"created_at"`
		User      struct {
			Username string `json:"username"`
		} `json:"user"`
		Label *struct {
			Name string `json:"name"`
		} `json:"label"`
	}
	items, err := getAll[apiEvent](ctx, c.base, c.token, c.path(Target{Type: IssueTarget, IID: iid}, "/resource_label_events"), nil)
	if err != nil {
		return nil, err
	}
	var out []LabelEvent
	for _, e := range items {
		// A label deleted from the project leaves events without one.
		if e.Label == nil {
			continue
		}
		out = append(out, LabelEvent{User: e.User.Username, Label: e.Label.Name, Added: e.Action == "add", At: e.CreatedAt})
	}
	return out, nil
}

// LastAdded returns who added label most recently, and when.
func LastAdded(events []LabelEvent, label string) (LabelEvent, bool) {
	var last LabelEvent
	found := false
	for _, e := range events {
		if e.Added && strings.EqualFold(e.Label, label) && (!found || e.At.After(last.At)) {
			last, found = e, true
		}
	}
	return last, found
}

// Relabel adds and removes labels on an issue in one call; GitLab creates
// labels that do not exist yet.
func (c *Client) Relabel(ctx context.Context, iid int, add, remove []string) error {
	body := map[string]string{}
	if len(add) > 0 {
		body["add_labels"] = strings.Join(add, ",")
	}
	if len(remove) > 0 {
		body["remove_labels"] = strings.Join(remove, ",")
	}
	_, err := send(ctx, http.MethodPut, c.base, c.token, c.path(Target{Type: IssueTarget, IID: iid}, ""), nil, body, nil)
	return err
}

// UserID looks up a user's numeric id, which reviewer lists need.
func (c *Client) UserID(ctx context.Context, username string) (int, error) {
	var users []struct {
		ID int `json:"id"`
	}
	if err := get(ctx, c.base, c.token, "api/v4/users", url.Values{"username": {username}}, &users); err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, fmt.Errorf("utilisateur @%s introuvable", username)
	}
	return users[0].ID, nil
}

// OpenMergeRequest opens a draft merge request from source into target.
func (c *Client) OpenMergeRequest(ctx context.Context, source, target, title, description string, reviewers []int) (MergeRequest, error) {
	if !strings.HasPrefix(strings.ToLower(title), "draft:") {
		title = "Draft: " + title
	}
	body := map[string]any{
		"source_branch":        source,
		"target_branch":        target,
		"title":                title,
		"description":          description,
		"remove_source_branch": true,
	}
	if len(reviewers) > 0 {
		body["reviewer_ids"] = reviewers
	}
	var mr struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
	}
	path := "api/v4/projects/" + strconv.Itoa(c.ProjectID) + "/merge_requests"
	if _, err := send(ctx, http.MethodPost, c.base, c.token, path, nil, body, &mr); err != nil {
		return MergeRequest{}, err
	}
	return c.MergeRequest(ctx, mr.IID)
}

// DefaultBranch returns the project's default branch.
func (c *Client) DefaultBranch(ctx context.Context) (string, error) {
	var p struct {
		DefaultBranch string `json:"default_branch"`
	}
	err := get(ctx, c.base, c.token, "api/v4/projects/"+strconv.Itoa(c.ProjectID), nil, &p)
	return p.DefaultBranch, err
}

// BranchExists reports whether the project has a branch of that name.
func (c *Client) BranchExists(ctx context.Context, name string) (bool, error) {
	_, err := call(ctx, http.MethodGet, c.base, c.token,
		"api/v4/projects/"+strconv.Itoa(c.ProjectID)+"/repository/branches/"+url.PathEscape(name), nil, nil)
	if err != nil && strings.Contains(err.Error(), "introuvable") {
		return false, nil
	}
	return err == nil, err
}

// Found is an issue or a merge request a search returned.
type Found struct {
	IID    int    `json:"iid"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Author string `json:"author"`
	WebURL string `json:"webUrl"`
}

// Search finds the project's issues or merge requests whose title or
// description holds text, newest first; state is opened, closed, merged or
// "" for any.
func (c *Client) Search(ctx context.Context, kind TargetType, text, state string, limit int) ([]Found, error) {
	var items []struct {
		IID    int    `json:"iid"`
		Title  string `json:"title"`
		State  string `json:"state"`
		WebURL string `json:"web_url"`
		Author struct {
			Username string `json:"username"`
		} `json:"author"`
	}
	query := url.Values{"search": {text}, "per_page": {strconv.Itoa(limit)}}
	if state != "" {
		query.Set("state", state)
	}
	path := "api/v4/projects/" + strconv.Itoa(c.ProjectID) + "/" + kind.collection()
	if err := get(ctx, c.base, c.token, path, query, &items); err != nil {
		return nil, err
	}
	out := make([]Found, len(items))
	for i, it := range items {
		out[i] = Found{IID: it.IID, Title: it.Title, State: it.State, Author: it.Author.Username, WebURL: it.WebURL}
	}
	return out, nil
}
