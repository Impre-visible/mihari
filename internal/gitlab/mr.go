package gitlab

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// MergeRequest is what a job needs to know about the merge request it works on.
type MergeRequest struct {
	ProjectID       int    `json:"projectId"`
	IID             int    `json:"iid"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Author          string `json:"author"`
	SourceBranch    string `json:"sourceBranch"`
	TargetBranch    string `json:"targetBranch"`
	SourceProjectID int    `json:"sourceProjectId"`
	WebURL          string `json:"webUrl"`
	State           string `json:"state"`
	// Commits the diff is between, which line comments must refer to.
	BaseSHA  string `json:"baseSha"`
	HeadSHA  string `json:"headSha"`
	StartSHA string `json:"startSha"`
}

// FromFork reports whether the source branch lives in another project.
func (mr MergeRequest) FromFork() bool {
	return mr.SourceProjectID != mr.ProjectID
}

// Client acts on one project with the saved credentials.
type Client struct {
	base      *url.URL
	token     string
	Bot       string
	ProjectID int
}

func NewClient(projectID int) (*Client, error) {
	base, token, bot, err := connection()
	if err != nil {
		return nil, err
	}
	return &Client{base: base, token: token, Bot: bot, ProjectID: projectID}, nil
}

func (c *Client) mrPath(iid int, rest string) string {
	return c.path(Target{Type: MergeRequestTarget, IID: iid}, rest)
}

func (c *Client) MergeRequest(ctx context.Context, iid int) (MergeRequest, error) {
	var mr struct {
		IID             int    `json:"iid"`
		Title           string `json:"title"`
		Description     string `json:"description"`
		SourceBranch    string `json:"source_branch"`
		TargetBranch    string `json:"target_branch"`
		SourceProjectID int    `json:"source_project_id"`
		ProjectID       int    `json:"project_id"`
		WebURL          string `json:"web_url"`
		State           string `json:"state"`
		Author          struct {
			Username string `json:"username"`
		} `json:"author"`
		DiffRefs struct {
			BaseSHA  string `json:"base_sha"`
			HeadSHA  string `json:"head_sha"`
			StartSHA string `json:"start_sha"`
		} `json:"diff_refs"`
	}
	if err := get(ctx, c.base, c.token, c.mrPath(iid, ""), nil, &mr); err != nil {
		return MergeRequest{}, err
	}
	return MergeRequest{
		ProjectID: mr.ProjectID, IID: mr.IID, Title: mr.Title, Description: mr.Description,
		Author: mr.Author.Username, SourceBranch: mr.SourceBranch, TargetBranch: mr.TargetBranch,
		SourceProjectID: mr.SourceProjectID, WebURL: mr.WebURL, State: mr.State,
		BaseSHA: mr.DiffRefs.BaseSHA, HeadSHA: mr.DiffRefs.HeadSHA, StartSHA: mr.DiffRefs.StartSHA,
	}, nil
}

type FileDiff struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
	Diff    string `json:"diff"`
	New     bool   `json:"new_file"`
	Deleted bool   `json:"deleted_file"`
	Renamed bool   `json:"renamed_file"`
}

func (c *Client) Diffs(ctx context.Context, iid int) ([]FileDiff, error) {
	return getAll[FileDiff](ctx, c.base, c.token, c.mrPath(iid, "/diffs"), nil)
}

// CommentLine starts a thread on a line of the diff: a line of the new file,
// or of the old one for a removed line, and returns the thread's id. GitLab
// rejects lines outside the diff.
func (c *Client) CommentLine(ctx context.Context, mr MergeRequest, oldPath, newPath string, newLine, oldLine int, body string) (string, error) {
	position := map[string]any{
		"position_type": "text",
		"base_sha":      mr.BaseSHA,
		"head_sha":      mr.HeadSHA,
		"start_sha":     mr.StartSHA,
		"old_path":      oldPath,
		"new_path":      newPath,
	}
	if newLine > 0 {
		position["new_line"] = newLine
	}
	if oldLine > 0 {
		position["old_line"] = oldLine
	}
	var d struct {
		ID string `json:"id"`
	}
	_, err := send(ctx, http.MethodPost, c.base, c.token, c.mrPath(mr.IID, "/discussions"), nil,
		map[string]any{"body": body, "position": position}, &d)
	return d.ID, err
}

// Identity is a name and email for git commits.
type Identity struct {
	Name  string
	Email string
}

// Identity returns the commit identity of a user: their public or commit
// email, else GitLab's no-reply address for them.
func (c *Client) Identity(ctx context.Context, username string) (Identity, error) {
	var users []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Username    string `json:"username"`
		PublicEmail string `json:"public_email"`
		CommitEmail string `json:"commit_email"`
	}
	if err := get(ctx, c.base, c.token, "api/v4/users", url.Values{"username": {username}}, &users); err != nil {
		return Identity{}, err
	}
	if len(users) == 0 {
		return Identity{}, fmt.Errorf("utilisateur @%s introuvable", username)
	}
	u := users[0]
	email := cmp.Or(u.CommitEmail, u.PublicEmail,
		fmt.Sprintf("%d-%s@users.noreply.%s", u.ID, u.Username, strings.TrimPrefix(c.base.Hostname(), "www.")))
	return Identity{Name: cmp.Or(u.Name, u.Username), Email: email}, nil
}

// ProjectPath returns the namespaced path of the client's project.
func (c *Client) ProjectPath(ctx context.Context) (string, error) {
	var p struct {
		PathWithNamespace string `json:"path_with_namespace"`
	}
	err := get(ctx, c.base, c.token, "api/v4/projects/"+strconv.Itoa(c.ProjectID), nil, &p)
	return p.PathWithNamespace, err
}
