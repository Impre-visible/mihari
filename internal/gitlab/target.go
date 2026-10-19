package gitlab

import (
	"cmp"
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

// TargetType is GitLab's name for what a comment thread hangs on.
type TargetType string

const (
	MergeRequestTarget TargetType = "MergeRequest"
	IssueTarget        TargetType = "Issue"
)

func (t TargetType) collection() string {
	if t == IssueTarget {
		return "issues"
	}
	return "merge_requests"
}

// Target is a merge request or an issue of the client's project.
type Target struct {
	Type TargetType `json:"type"`
	IID  int        `json:"iid"`
}

// Ref is how GitLab writes it in text: !12 or #42.
func (t Target) Ref() string {
	if t.Type == IssueTarget {
		return "#" + strconv.Itoa(t.IID)
	}
	return "!" + strconv.Itoa(t.IID)
}

func (c *Client) path(t Target, rest string) string {
	return "api/v4/projects/" + strconv.Itoa(c.ProjectID) + "/" + t.Type.collection() + "/" + strconv.Itoa(t.IID) + rest
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
	// A thread on a diff line that nobody has resolved yet.
	Open bool `json:"open"`
}

type apiNote struct {
	ID         int       `json:"id"`
	Resolvable bool      `json:"resolvable"`
	Resolved   bool      `json:"resolved"`
	Body       string    `json:"body"`
	System     bool      `json:"system"`
	CreatedAt  time.Time `json:"created_at"`
	Author     struct {
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
		if n.Resolvable && !n.Resolved {
			disc.Open = true
		}
		note := Note{ID: n.ID, Body: n.Body, Author: n.Author.Username, CreatedAt: n.CreatedAt}
		if n.Position != nil {
			note.Path, note.Line = n.Position.NewPath, cmp.Or(n.Position.NewLine, n.Position.OldLine)
		}
		disc.Notes = append(disc.Notes, note)
	}
	return disc
}

// Discussions returns every thread with at least one comment.
func (c *Client) Discussions(ctx context.Context, t Target) ([]Discussion, error) {
	items, err := getAll[apiDiscussion](ctx, c.base, c.token, c.path(t, "/discussions"), nil)
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
func (c *Client) Discussion(ctx context.Context, t Target, id string) (Discussion, error) {
	var d apiDiscussion
	err := get(ctx, c.base, c.token, c.path(t, "/discussions/"+url.PathEscape(id)), nil, &d)
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
func (c *Client) Comment(ctx context.Context, t Target, body string) (Note, error) {
	var n struct {
		ID int `json:"id"`
	}
	_, err := send(ctx, http.MethodPost, c.base, c.token, c.path(t, "/notes"), nil, map[string]string{"body": body}, &n)
	return Note{ID: n.ID, Body: body, Author: c.Bot}, err
}

// Reply answers inside an existing thread.
func (c *Client) Reply(ctx context.Context, t Target, discussionID, body string) (Note, error) {
	var n struct {
		ID int `json:"id"`
	}
	path := c.path(t, "/discussions/"+url.PathEscape(discussionID)+"/notes")
	_, err := send(ctx, http.MethodPost, c.base, c.token, path, nil, map[string]string{"body": body}, &n)
	return Note{ID: n.ID, Body: body, Author: c.Bot}, err
}

// React adds an emoji to the comment noteID, or to the merge request or issue
// itself when noteID is 0, and returns the award to remove it later.
func (c *Client) React(ctx context.Context, t Target, noteID int, emoji string) (int, error) {
	var award struct {
		ID int `json:"id"`
	}
	_, err := send(ctx, http.MethodPost, c.base, c.token, c.awards(t, noteID, ""), url.Values{"name": {emoji}}, nil, &award)
	return award.ID, err
}

func (c *Client) Unreact(ctx context.Context, t Target, noteID, awardID int) error {
	_, err := call(ctx, http.MethodDelete, c.base, c.token, c.awards(t, noteID, "/"+strconv.Itoa(awardID)), nil, nil)
	return err
}

func (c *Client) awards(t Target, noteID int, rest string) string {
	if noteID != 0 {
		return c.path(t, "/notes/"+strconv.Itoa(noteID)+"/award_emoji"+rest)
	}
	return c.path(t, "/award_emoji"+rest)
}
