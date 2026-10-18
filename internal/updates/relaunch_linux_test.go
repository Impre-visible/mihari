package updates

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRelaunchWhenGone(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	// Stands for the new Mihari: it records that it ran.
	app := filepath.Join(dir, "mihari")
	if err := os.WriteFile(app, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Stands for the old Mihari, still running.
	old := exec.Command("sleep", "1")
	if err := old.Start(); err != nil {
		t.Fatal(err)
	}

	if err := relaunchWhenGone(old.Process.Pid, app); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("relaunched while the old process was still running")
	}
	_ = old.Wait()
	for range 30 {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("not relaunched after the old process exited")
}
