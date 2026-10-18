package jobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Impre-visible/mihari/internal/agent"
)

// Each job is one JSON file, so the history survives restarts without a database.

// Job files kept on disk; the Status page shows the latest historySize.
const storeSize = 200

type record struct {
	Summary Summary       `json:"summary"`
	Events  []agent.Event `json:"events"`
}

func storeDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "mihari", "jobs")
	return dir, os.MkdirAll(dir, 0o700)
}

func save(r record) {
	dir, err := storeDir()
	if err != nil {
		return
	}
	data, err := json.Marshal(r)
	if err != nil {
		return
	}
	path := filepath.Join(dir, strconv.Itoa(r.Summary.ID)+".json")
	// Written aside then renamed, so a crash never leaves half a file.
	if err := os.WriteFile(path+".tmp", data, 0o600); err == nil {
		_ = os.Rename(path+".tmp", path)
	}
}

func loadRecord(id int) (record, bool) {
	dir, err := storeDir()
	if err != nil {
		return record{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(id)+".json"))
	if err != nil {
		return record{}, false
	}
	var r record
	return r, json.Unmarshal(data, &r) == nil
}

// loadAll returns the stored jobs oldest first, after deleting all but the
// latest storeSize.
func loadAll() []record {
	dir, err := storeDir()
	if err != nil {
		return nil
	}
	entries, _ := os.ReadDir(dir)
	var ids []int
	for _, e := range entries {
		if id, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".json")); err == nil && strings.HasSuffix(e.Name(), ".json") {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	for len(ids) > storeSize {
		_ = os.Remove(filepath.Join(dir, strconv.Itoa(ids[0])+".json"))
		ids = ids[1:]
	}
	var out []record
	for _, id := range ids {
		if r, ok := loadRecord(id); ok {
			out = append(out, r)
		}
	}
	return out
}
