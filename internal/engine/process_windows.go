//go:build windows

package engine

import (
	"os/exec"
	"strconv"
)

func setProcessGroup(*exec.Cmd) {}

// Windows has no SIGTERM; taskkill /T also stops the engine's children.
func interrupt(cmd *exec.Cmd) {
	_ = exec.Command("taskkill", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}

func kill(cmd *exec.Cmd) {
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}
