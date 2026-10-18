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
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return "", fmt.Errorf("URL invalide : %q", baseURL)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.JoinPath("api/v4/user").String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("instance injoignable : %w", err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return "", fmt.Errorf("token refusé par l'instance")
	case res.StatusCode != http.StatusOK:
		return "", fmt.Errorf("réponse inattendue de l'instance : %s", res.Status)
	}

	var u user
	if err := json.NewDecoder(res.Body).Decode(&u); err != nil {
		return "", fmt.Errorf("réponse illisible, est-ce bien une instance GitLab ? %w", err)
	}

	if username != "" && !strings.EqualFold(u.Username, username) {
		return "", fmt.Errorf("le token appartient à @%s, pas à @%s", u.Username, username)
	}

	return u.Name, nil
}
