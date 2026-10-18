// Package settings stores Mihari's local state in the user's config directory.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// Service is exposed to the frontend through Wails bindings.
type Service struct{}

func GetConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	folder := filepath.Join(dir, "mihari")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return "", err
	}
	file := filepath.Join(folder, "config.json")
	return file, nil
}

// IsOnboarded reports whether the user has already gone past the welcome screen.
func (s *Service) IsOnboarded() (bool, error) {
	value, err := s.Get("onboarded")
	if err != nil {
		return false, err
	}
	if value == nil {
		return false, nil
	}
	onboarded, ok := value.(bool)
	if !ok {
		return false, nil
	}
	return onboarded, nil
}

// CompleteOnboarding records that the welcome screen has been seen.
func (s *Service) CompleteOnboarding() error {
	return s.Set("onboarded", true)
}

func (s *Service) GetConfig() (map[string]interface{}, error) {
	filepath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]interface{}{}, nil
	} else if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

func (s *Service) Get(field string) (interface{}, error) {
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}

	value, ok := config[field]
	if !ok {
		return nil, nil
	}

	return value, nil
}

func (s *Service) Set(field string, value interface{}) error {
	filepath, err := GetConfigPath()
	if err != nil {
		return err
	}

	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	config[field] = value

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath, data, 0o600); err != nil {
		return err
	}

	return nil
}

// GitLabToken returns the token stored in the keyring, or "" when none is.
func GitLabToken() (string, error) {
	value, err := keyring.Get("mihari", "gitlab_token")
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return value, err
}

func (s *Service) HasGitLabToken() (bool, error) {
	value, err := GitLabToken()
	if err != nil {
		return false, err
	}
	return value != "", nil
}

func (s *Service) SetGitLabToken(token string) error {
	return keyring.Set("mihari", "gitlab_token", token)
}
