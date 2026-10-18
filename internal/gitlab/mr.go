package gitlab

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
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
	return "api/v4/projects/" + strconv.Itoa(c.ProjectID) + "/merge_requests/" + strconv.Itoa(iid) + rest
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

type Note struct {
	ID        int       `json:"id"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	// Set for a comment on a diff line.
	Path string `json:"path"`
	Line int    `json:"line"`
}

type Discussion struct {
	ID    string `json:"id"`
	Notes []Note `json:"notes"`
}

type apiNote struct {
	ID        int       `json:"id"`
	Body      string    `json:"body"`
	System    bool      `json:"system"`
	CreatedAt time.Time `json:"created_at"`
	Author    struct {
		Username string `json:"username"`
	} `json:"author"`
	Position *struct {
		NewPath string `json:"new_path"`
		NewLine int    `json:"new_line"`
		OldLine int    `json:"old_line"`
	} `json:"position"`
}

type apiDiscussion struct {
	ID    string    `json:"id"`
	Notes []apiNote `json:"notes"`
}

// convert drops GitLab's system notes ("added 2 commits"…).
func (d apiDiscussion) convert() Discussion {
	disc := Discussion{ID: d.ID}
	for _, n := range d.Notes {
		if n.System {
			continue
		}
		note := Note{ID: n.ID, Body: n.Body, Author: n.Author.Username, CreatedAt: n.CreatedAt}
		if n.Position != nil {
			note.Path, note.Line = n.Position.NewPath, cmp.Or(n.Position.NewLine, n.Position.OldLine)
		}
		disc.Notes = append(disc.Notes, note)
	}
	return disc
}

// Discussions returns every thread of the merge request with at least one comment.
func (c *Client) Discussions(ctx context.Context, iid int) ([]Discussion, error) {
	items, err := getAll[apiDiscussion](ctx, c.base, c.token, c.mrPath(iid, "/discussions"), nil)
	if err != nil {
		return nil, err
	}
	var out []Discussion
	for _, d := range items {
		if disc := d.convert(); len(disc.Notes) > 0 {
			out = append(out, disc)
		}
	}
	return out, nil
}

// Discussion reads a single thread, cheap enough to watch while a job runs.
func (c *Client) Discussion(ctx context.Context, iid int, id string) (Discussion, error) {
	var d apiDiscussion
	err := get(ctx, c.base, c.token, c.mrPath(iid, "/discussions/"+url.PathEscape(id)), nil, &d)
	return d.convert(), err
}

// DiscussionOf finds the thread a note belongs to.
func DiscussionOf(discussions []Discussion, noteID int) (Discussion, bool) {
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.ID == noteID {
				return d, true
			}
		}
	}
	return Discussion{}, false
}

var noteAnchor = regexp.MustCompile(`#note_(\d+)$`)

// NoteIDFromURL reads the note a todo points to, 0 for none.
func NoteIDFromURL(u string) int {
	if m := noteAnchor.FindStringSubmatch(u); m != nil {
		id, _ := strconv.Atoi(m[1])
		return id
	}
	return 0
}

// Comment posts a new top-level comment.
func (c *Client) Comment(ctx context.Context, iid int, body string) (Note, error) {
	var n struct {
		ID int `json:"id"`
	}
	_, err := send(ctx, http.MethodPost, c.base, c.token, c.mrPath(iid, "/notes"), nil, map[string]string{"body": body}, &n)
	return Note{ID: n.ID, Body: body, Author: c.Bot}, err
}

// Reply answers inside an existing thread.
func (c *Client) Reply(ctx context.Context, iid int, discussionID, body string) (Note, error) {
	var n struct {
		ID int `json:"id"`
	}
	path := c.mrPath(iid, "/discussions/"+url.PathEscape(discussionID)+"/notes")
	_, err := send(ctx, http.MethodPost, c.base, c.token, path, nil, map[string]string{"body": body}, &n)
	return Note{ID: n.ID, Body: body, Author: c.Bot}, err
}

// CommentLine starts a thread on a line of the diff: a line of the new file,
// or of the old one for a removed line. GitLab rejects lines outside the diff.
func (c *Client) CommentLine(ctx context.Context, mr MergeRequest, oldPath, newPath string, newLine, oldLine int, body string) error {
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
	_, err := send(ctx, http.MethodPost, c.base, c.token, c.mrPath(mr.IID, "/discussions"), nil,
		map[string]any{"body": body, "position": position}, nil)
	return err
}

// React adds an emoji to the merge request and returns the award to remove it later.
func (c *Client) React(ctx context.Context, iid int, emoji string) (int, error) {
	var award struct {
		ID int `json:"id"`
	}
	_, err := send(ctx, http.MethodPost, c.base, c.token, c.mrPath(iid, "/award_emoji"), url.Values{"name": {emoji}}, nil, &award)
	return award.ID, err
}

func (c *Client) Unreact(ctx context.Context, iid, awardID int) error {
	_, err := call(ctx, http.MethodDelete, c.base, c.token, c.mrPath(iid, "/award_emoji/"+strconv.Itoa(awardID)), nil, nil)
	return err
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
