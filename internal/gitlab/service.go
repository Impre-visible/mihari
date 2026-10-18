// Package gitlab talks to the GitLab instance Mihari reviews merge requests on.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Impre-visible/mihari/internal/settings"
)

// Service is exposed to the frontend through Wails bindings.
type Service struct{}

type user struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

// TestConnection checks that token authenticates as username on the instance
// at baseURL, and returns the account's display name. An empty token falls
// back to the one already stored in the keyring.
func (s *Service) TestConnection(baseURL, username, token string) (string, error) {
	base, err := parseBaseURL(baseURL)
	if err != nil {
		return "", err
	}

	if token == "" {
		token, err = settings.GitLabToken()
		if err != nil {
			return "", err
		}
		if token == "" {
			return "", fmt.Errorf("aucun token renseigné")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var u user
	if err := get(ctx, base, token, "api/v4/user", nil, &u); err != nil {
		return "", err
	}

	if username != "" && !strings.EqualFold(u.Username, username) {
		return "", fmt.Errorf("le token appartient à @%s, pas à @%s", u.Username, username)
	}

	return u.Name, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("URL invalide : %q", raw)
	}
	return base, nil
}

// get calls the REST API at path below base and decodes the JSON answer into out.
func get(ctx context.Context, base *url.URL, token, path string, query url.Values, out any) error {
	_, err := call(ctx, http.MethodGet, base, token, path, query, out)
	return err
}

// getAll follows GitLab's pagination and returns every item of a list endpoint.
func getAll[T any](ctx context.Context, base *url.URL, token, path string, query url.Values) ([]T, error) {
	query = maps.Clone(query)
	if query == nil {
		query = url.Values{}
	}
	query.Set("per_page", "100")
	var all []T
	for page := "1"; page != ""; {
		query.Set("page", page)
		var items []T
		next, err := call(ctx, http.MethodGet, base, token, path, query, &items)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		page = next
	}
	return all, nil
}

// call sends a request without a body and decodes the JSON answer into out,
// when out is not nil. It returns GitLab's next page number, if any.
func call(ctx context.Context, method string, base *url.URL, token, path string, query url.Values, out any) (string, error) {
	return send(ctx, method, base, token, path, query, nil, out)
}

// send is call with an optional JSON body.
func send(ctx context.Context, method string, base *url.URL, token, path string, query url.Values, body, out any) (string, error) {
	endpoint := base.JoinPath(path)
	endpoint.RawQuery = query.Encode()

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("instance injoignable : %w", err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return "", fmt.Errorf("token refusé par l'instance")
	case res.StatusCode == http.StatusForbidden:
		return "", fmt.Errorf("le bot n'a pas les droits pour ça")
	case res.StatusCode == http.StatusNotFound:
		return "", fmt.Errorf("introuvable sur l'instance, ou le bot n'y a pas accès")
	case res.StatusCode < 200 || res.StatusCode > 299:
		// GitLab explains validation errors (a bad line position, say) in "message".
		var detail struct {
			Message any `json:"message"`
			Error   any `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&detail)
		if detail.Message != nil {
			return "", fmt.Errorf("GitLab a refusé (%s) : %v", res.Status, detail.Message)
		}
		if detail.Error != nil {
			return "", fmt.Errorf("GitLab a refusé (%s) : %v", res.Status, detail.Error)
		}
		return "", fmt.Errorf("réponse inattendue de l'instance : %s", res.Status)
	}

	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return "", fmt.Errorf("réponse illisible, est-ce bien une instance GitLab ? %w", err)
		}
	}
	return res.Header.Get("X-Next-Page"), nil
}
