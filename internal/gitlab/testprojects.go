package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// TestPrefix starts the path of every test project the Dev page makes. Only
// projects with it, in the bot's own namespace, are ever deleted.
const TestPrefix = "mihari-test-"

// TestProject is a test project with its open merge requests and issues.
type TestProject struct {
	ID            int        `json:"id"`
	Path          string     `json:"path"`
	WebURL        string     `json:"webUrl"`
	MergeRequests []TestItem `json:"mergeRequests"`
	Issues        []TestItem `json:"issues"`
}

type TestItem struct {
	IID    int    `json:"iid"`
	Title  string `json:"title"`
	WebURL string `json:"webUrl"`
}

// TestSpec is what a test project starts with.
type TestSpec struct {
	// Path within the bot's namespace, starting with TestPrefix.
	Path        string
	Description string
	// Files of the default branch, by path.
	Files         map[string]string
	MergeRequests []TestChange
	Issues        []TestIssue
	// GitLab username added as a developer, so a real person can ping the bot.
	Invite string
}

// TestChange is a merge request: a branch off main that writes Files, full
// contents, and removes Deleted.
type TestChange struct {
	Branch, Title, Description string
	Files                      map[string]string
	Deleted                    []string
}

type TestIssue struct {
	Title, Description string
	Labels             []string
}

type apiTestProject struct {
	ID                int    `json:"id"`
	Path              string `json:"path"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
	// Set once a deletion is scheduled rather than done.
	MarkedForDeletionOn *string `json:"marked_for_deletion_on"`
	Namespace           struct {
		Kind string `json:"kind"`
		Path string `json:"path"`
	} `json:"namespace"`
}

// owner reads the bot's account from the token, rather than trusting the
// username typed in the settings.
func owner(ctx context.Context) (base *url.URL, token string, bot user, id int, err error) {
	base, token, _, err = connection()
	if err != nil {
		return nil, "", user{}, 0, err
	}
	var u struct {
		user
		ID int `json:"id"`
	}
	if err := get(ctx, base, token, "api/v4/user", nil, &u); err != nil {
		return nil, "", user{}, 0, err
	}
	return base, token, u.user, u.ID, nil
}

// isTestProject guards every deletion: a project of the bot's own namespace
// whose path starts with TestPrefix, and not already on its way out.
func isTestProject(p apiTestProject, bot string) bool {
	return p.Namespace.Kind == "user" && strings.EqualFold(p.Namespace.Path, bot) &&
		strings.HasPrefix(p.Path, TestPrefix) && p.MarkedForDeletionOn == nil
}

// TestProjects lists the test projects with their open merge requests and issues.
func TestProjects(ctx context.Context) ([]TestProject, error) {
	base, token, bot, _, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	found, err := testProjects(ctx, base, token, bot.Username)
	if err != nil {
		return nil, err
	}
	out := make([]TestProject, 0, len(found))
	for _, p := range found {
		tp := TestProject{ID: p.ID, Path: p.PathWithNamespace, WebURL: p.WebURL}
		type item struct {
			IID    int    `json:"iid"`
			Title  string `json:"title"`
			WebURL string `json:"web_url"`
		}
		open := url.Values{"state": {"opened"}, "order_by": {"created_at"}, "sort": {"asc"}}
		for _, list := range []struct {
			path string
			into *[]TestItem
		}{{"merge_requests", &tp.MergeRequests}, {"issues", &tp.Issues}} {
			items, err := getAll[item](ctx, base, token, fmt.Sprintf("api/v4/projects/%d/%s", p.ID, list.path), open)
			if err != nil {
				return nil, err
			}
			*list.into = []TestItem{}
			for _, it := range items {
				*list.into = append(*list.into, TestItem{IID: it.IID, Title: it.Title, WebURL: it.WebURL})
			}
		}
		out = append(out, tp)
	}
	return out, nil
}

func testProjects(ctx context.Context, base *url.URL, token, bot string) ([]apiTestProject, error) {
	query := url.Values{"owned": {"true"}, "search": {TestPrefix}, "order_by": {"path"}, "sort": {"asc"}}
	items, err := getAll[apiTestProject](ctx, base, token, "api/v4/projects", query)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(items, func(p apiTestProject) bool { return !isTestProject(p, bot) }), nil
}

// CreateTestProject makes a private project in the bot's namespace and fills
// it through the commits API: no git needed. The first commit, on main, makes
// main the default branch. A path still held by a project being deleted gets
// a suffix.
func CreateTestProject(ctx context.Context, spec TestSpec) (TestProject, error) {
	if !strings.HasPrefix(spec.Path, TestPrefix) {
		return TestProject{}, fmt.Errorf("un projet de test commence par %s", TestPrefix)
	}
	base, token, _, _, err := owner(ctx)
	if err != nil {
		return TestProject{}, err
	}
	var p apiTestProject
	create := map[string]any{
		"name": spec.Path, "path": spec.Path, "description": spec.Description, "visibility": "private",
	}
	_, err = send(ctx, http.MethodPost, base, token, "api/v4/projects", nil, create, &p)
	if err != nil && strings.Contains(err.Error(), "has already been taken") {
		suffix := "-" + strconv.FormatInt(time.Now().Unix(), 36)
		create["name"], create["path"] = spec.Path+suffix, spec.Path+suffix
		_, err = send(ctx, http.MethodPost, base, token, "api/v4/projects", nil, create, &p)
	}
	if err != nil {
		return TestProject{}, fmt.Errorf("création de %s : %w", spec.Path, err)
	}
	project := fmt.Sprintf("api/v4/projects/%d", p.ID)
	commit := func(branch, from, message string, files map[string]string, deleted []string, existing map[string]string) error {
		var actions []map[string]string
		for _, path := range sortedKeys(files) {
			action := "create"
			if _, ok := existing[path]; ok {
				action = "update"
			}
			actions = append(actions, map[string]string{"action": action, "file_path": path, "content": files[path]})
		}
		for _, path := range deleted {
			actions = append(actions, map[string]string{"action": "delete", "file_path": path})
		}
		body := map[string]any{"branch": branch, "commit_message": message, "actions": actions}
		if from != "" {
			body["start_branch"] = from
		}
		_, err := send(ctx, http.MethodPost, base, token, project+"/repository/commits", nil, body, nil)
		return err
	}

	if err := commit("main", "", "Code de départ", spec.Files, nil, nil); err != nil {
		return TestProject{}, fmt.Errorf("%s, code de départ : %w", spec.Path, err)
	}
	for _, mr := range spec.MergeRequests {
		if err := commit(mr.Branch, "main", mr.Title, mr.Files, mr.Deleted, spec.Files); err != nil {
			return TestProject{}, fmt.Errorf("%s, branche %s : %w", spec.Path, mr.Branch, err)
		}
		body := map[string]any{"source_branch": mr.Branch, "target_branch": "main", "title": mr.Title, "description": mr.Description}
		if _, err := send(ctx, http.MethodPost, base, token, project+"/merge_requests", nil, body, nil); err != nil {
			return TestProject{}, fmt.Errorf("%s, MR %q : %w", spec.Path, mr.Title, err)
		}
	}
	for _, is := range spec.Issues {
		body := map[string]any{"title": is.Title, "description": is.Description, "labels": strings.Join(is.Labels, ",")}
		if _, err := send(ctx, http.MethodPost, base, token, project+"/issues", nil, body, nil); err != nil {
			return TestProject{}, fmt.Errorf("%s, issue %q : %w", spec.Path, is.Title, err)
		}
	}
	if invite := strings.TrimPrefix(strings.TrimSpace(spec.Invite), "@"); invite != "" {
		var users []struct {
			ID int `json:"id"`
		}
		if err := get(ctx, base, token, "api/v4/users", url.Values{"username": {invite}}, &users); err != nil {
			return TestProject{}, err
		}
		if len(users) == 0 {
			return TestProject{}, fmt.Errorf("aucun compte @%s sur l'instance", invite)
		}
		body := map[string]any{"user_id": users[0].ID, "access_level": AccessDeveloper}
		if _, err := send(ctx, http.MethodPost, base, token, project+"/members", nil, body, nil); err != nil {
			return TestProject{}, fmt.Errorf("invitation de @%s : %w", invite, err)
		}
	}
	return TestProject{ID: p.ID, Path: p.PathWithNamespace, WebURL: p.WebURL}, nil
}

// DeleteTestProjects deletes every test project and returns their ids. Where
// the instance only schedules the deletion, as GitLab.com does, the project
// is then removed for good so its path is free again.
func DeleteTestProjects(ctx context.Context) ([]int, error) {
	base, token, bot, _, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	found, err := testProjects(ctx, base, token, bot.Username)
	if err != nil {
		return nil, err
	}
	var deleted []int
	var errs []error
	for _, p := range found {
		path := fmt.Sprintf("api/v4/projects/%d", p.ID)
		if _, err := send(ctx, http.MethodDelete, base, token, path, nil, nil, nil); err != nil {
			errs = append(errs, fmt.Errorf("%s : %w", p.PathWithNamespace, err))
			continue
		}
		deleted = append(deleted, p.ID)
		var after apiTestProject
		if get(ctx, base, token, path, nil, &after) != nil || after.MarkedForDeletionOn == nil {
			continue
		}
		now := url.Values{"permanently_remove": {"true"}, "full_path": {after.PathWithNamespace}}
		if _, err := send(ctx, http.MethodDelete, base, token, path, now, nil, nil); err != nil {
			errs = append(errs, fmt.Errorf("%s, suppression définitive : %w", p.PathWithNamespace, err))
		}
	}
	return deleted, errors.Join(errs...)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
