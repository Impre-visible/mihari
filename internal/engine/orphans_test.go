//go:build !windows

package engine

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestReapOrphans(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip(err)
	}
	// An engine a crashed Mihari left: its own group, with a child.
	cmd := exec.Command("sh", "-c", sleep+" 300 & exec "+sleep+" 300")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	time.Sleep(200 * time.Millisecond)
	track(cmd.Process.Pid, sleep)
	// An entry whose pid now runs something else must be left alone.
	track(1, sleep)

	ReapOrphans()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("orphan still running")
	}
	if syscall.Kill(-cmd.Process.Pid, 0) == nil {
		t.Error("its child still running")
	}
	if len(readTracked()) != 0 {
		t.Error("tracking file not cleared")
	}
}
