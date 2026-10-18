package engine

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Engines run in their own process group, so a Mihari killed without a clean
// shutdown (wails3 dev restarting it, a crash) leaves them running, holding
// the GPU. Mihari writes down every group it starts and kills the leftovers
// on its next start.

type tracked struct {
	PID int `json:"pid"`
	// The engine binary, to tell our process from an unrelated one that got the same pid.
	Binary string `json:"binary"`
}

var trackMu sync.Mutex

func trackFile() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "mihari"), 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "mihari", "engines.json"), nil
}

func readTracked() []tracked {
	path, err := trackFile()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var list []tracked
	_ = json.Unmarshal(data, &list)
	return list
}

func writeTracked(list []tracked) {
	path, err := trackFile()
	if err != nil {
		return
	}
	data, _ := json.Marshal(list)
	_ = os.WriteFile(path, data, 0o600)
}

func track(pid int, binary string) {
	trackMu.Lock()
	defer trackMu.Unlock()
	writeTracked(append(readTracked(), tracked{PID: pid, Binary: binary}))
}

func untrack(pid int) {
	trackMu.Lock()
	defer trackMu.Unlock()
	list := readTracked()
	kept := list[:0]
	for _, t := range list {
		if t.PID != pid {
			kept = append(kept, t)
		}
	}
	writeTracked(kept)
}

// ReapOrphans kills the engines a previous run of Mihari left behind. Call it
// once at startup, before starting any engine.
func ReapOrphans() {
	trackMu.Lock()
	defer trackMu.Unlock()
	for _, t := range readTracked() {
		if isProcess(t.PID, t.Binary) {
			log.Printf("stopping engine left by a previous run: pid %d (%s)", t.PID, t.Binary)
			killGroup(t.PID)
		}
	}
	writeTracked(nil)
}
