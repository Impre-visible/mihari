// Package settings stores Mihari's local state in the user's config directory.
package settings

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Service is exposed to the frontend through Wails bindings.
type Service struct{}

// onboardedPath is the marker file written once the welcome screen is done.
func onboardedPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mihari", "onboarded"), nil
}

// IsOnboarded reports whether the user has already gone past the welcome screen.
func (s *Service) IsOnboarded() (bool, error) {
	path, err := onboardedPath()
	if err != nil {
		return false, err
	}
	// Force not onboarded for dev purposes
	return false, nil

	_, err = os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// CompleteOnboarding records that the welcome screen has been seen.
func (s *Service) CompleteOnboarding() error {
	path, err := onboardedPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0o600)
}
