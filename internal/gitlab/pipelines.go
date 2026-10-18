package gitlab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Commit struct {
	SHA     string    `json:"sha"`
	Title   string    `json:"title"`
	Author  string    `json:"author"`
	Created time.Time `json:"created"`
}

// Commits lists the merge request's commits, oldest first.
func (c *Client) Commits(ctx context.Context, iid int) ([]Commit, error) {
	type apiCommit struct {
		ShortID    string    `json:"short_id"`
		ID         string    `json:"id"`
		Title      string    `json:"title"`
		AuthorName string    `json:"author_name"`
		CreatedAt  time.Time `json:"created_at"`
	}
	items, err := getAll[apiCommit](ctx, c.base, c.token, c.mrPath(iid, "/commits"), nil)
	if err != nil {
		return nil, err
	}
	out := make([]Commit, 0, len(items))
	// GitLab lists the newest first.
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		out = append(out, Commit{SHA: it.ID, Title: it.Title, Author: it.AuthorName, Created: it.CreatedAt})
	}
	return out, nil
}

type PipelineJob struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
}

type Pipeline struct {
	ID     int           `json:"id"`
	Status string        `json:"status"`
	SHA    string        `json:"sha"`
	WebURL string        `json:"webUrl"`
	Jobs   []PipelineJob `json:"jobs"`
}

// LatestPipeline returns the merge request's most recent pipeline and its
// jobs; ok is false when it has none.
func (c *Client) LatestPipeline(ctx context.Context, iid int) (Pipeline, bool, error) {
	var pipelines []struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
		SHA    string `json:"sha"`
		WebURL string `json:"web_url"`
	}
	if err := get(ctx, c.base, c.token, c.mrPath(iid, "/pipelines"), nil, &pipelines); err != nil {
		return Pipeline{}, false, err
	}
	if len(pipelines) == 0 {
		return Pipeline{}, false, nil
	}
	p := Pipeline{ID: pipelines[0].ID, Status: pipelines[0].Status, SHA: pipelines[0].SHA, WebURL: pipelines[0].WebURL}
	jobs, err := getAll[PipelineJob](ctx, c.base, c.token,
		"api/v4/projects/"+strconv.Itoa(c.ProjectID)+"/pipelines/"+strconv.Itoa(p.ID)+"/jobs", nil)
	if err != nil {
		return p, true, err
	}
	p.Jobs = jobs
	return p, true, nil
}

// JobLog returns the last bytes of a CI job's log, where failures show.
func (c *Client) JobLog(ctx context.Context, jobID, lastBytes int) (string, error) {
	endpoint := c.base.JoinPath("api/v4/projects/" + strconv.Itoa(c.ProjectID) + "/jobs/" + strconv.Itoa(jobID) + "/trace")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("instance injoignable : %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("log du job %d : %s", jobID, res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if len(data) > lastBytes {
		data = data[len(data)-lastBytes:]
	}
	return string(data), nil
}

// Compare returns the commits and file changes from one commit to another,
// as GitLab computes them, whatever the depth of the local clone.
func (c *Client) Compare(ctx context.Context, from, to string) ([]Commit, []FileDiff, error) {
	var out struct {
		Commits []struct {
			ID         string    `json:"id"`
			Title      string    `json:"title"`
			AuthorName string    `json:"author_name"`
			CreatedAt  time.Time `json:"created_at"`
		} `json:"commits"`
		Diffs []FileDiff `json:"diffs"`
	}
	query := url.Values{"from": {from}, "to": {to}, "straight": {"false"}}
	if err := get(ctx, c.base, c.token, "api/v4/projects/"+strconv.Itoa(c.ProjectID)+"/repository/compare", query, &out); err != nil {
		return nil, nil, err
	}
	commits := make([]Commit, len(out.Commits))
	for i, it := range out.Commits {
		commits[i] = Commit{SHA: it.ID, Title: it.Title, Author: it.AuthorName, Created: it.CreatedAt}
	}
	return commits, out.Diffs, nil
}
