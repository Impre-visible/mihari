package projects

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Impre-visible/mihari/internal/settings"
)

// What the agent learned about a project lives in one Markdown file per
// project next to the config, so it is kept across jobs, readable, and
// editable on the Projects page. The agent edits it with its memory tool.

var memoryMu sync.Mutex

func memoryPath(projectID int) (string, error) {
	config, err := settings.GetConfigPath()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(config), "memory")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, strconv.Itoa(projectID)+".md"), nil
}

// Memory returns the project's notes, "" when there are none.
func Memory(projectID int) string {
	path, err := memoryPath(projectID)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SetMemory replaces the project's notes.
func SetMemory(projectID int, text string) error {
	memoryMu.Lock()
	defer memoryMu.Unlock()
	return writeMemory(projectID, strings.TrimSpace(text))
}

func writeMemory(projectID int, text string) error {
	path, err := memoryPath(projectID)
	if err != nil {
		return err
	}
	if text == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, []byte(text+"\n"), 0o600)
}

// Memory returns a project's notes for the Projects page.
func (s *Service) Memory(projectID int) string {
	return Memory(projectID)
}

// SetMemory saves notes edited on the Projects page.
func (s *Service) SetMemory(projectID int, text string) error {
	return SetMemory(projectID, text)
}
