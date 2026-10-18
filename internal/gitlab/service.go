// Package gitlab talks to the GitLab instance Mihari reviews merge requests on.
package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
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
	endpoint := base.JoinPath(path)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("instance injoignable : %w", err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("token refusé par l'instance")
	case res.StatusCode == http.StatusNotFound:
		return fmt.Errorf("introuvable sur l'instance, ou le bot n'y a pas accès")
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("réponse inattendue de l'instance : %s", res.Status)
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("réponse illisible, est-ce bien une instance GitLab ? %w", err)
	}
	return nil
}
