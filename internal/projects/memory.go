package projects

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/settings"
)

// What the agent learned about a project lives in one Markdown file per
// project next to the config, so it is kept across jobs, readable, and
// editable on the Projects page.

// Notes beyond this many lines drop their oldest ones.
const memoryLines = 200

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

// Remember adds a dated note, dropping the oldest lines past memoryLines.
func Remember(projectID int, note string) error {
	note = strings.Join(strings.Fields(note), " ")
	if note == "" {
		return errors.New("note vide")
	}
	memoryMu.Lock()
	defer memoryMu.Unlock()
	lines := strings.Split(Memory(projectID), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	for _, l := range lines {
		// The same lesson twice is noise.
		if strings.HasSuffix(l, " "+note) || l == "- "+note {
			return nil
		}
	}
	lines = append(lines, fmt.Sprintf("- %s %s", time.Now().Format("2006-01-02"), note))
	if len(lines) > memoryLines {
		lines = lines[len(lines)-memoryLines:]
	}
	return writeMemory(projectID, strings.Join(lines, "\n"))
}

// Memory returns a project's notes for the Projects page.
func (s *Service) Memory(projectID int) string {
	return Memory(projectID)
}

// SetMemory saves notes edited on the Projects page.
func (s *Service) SetMemory(projectID int, text string) error {
	return SetMemory(projectID, text)
}
