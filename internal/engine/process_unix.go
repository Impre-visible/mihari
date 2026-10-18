//go:build !windows

package engine

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// The engine gets its own process group so that stopping it also stops the
// workers it spawns (vllm-mlx and Ollama run child processes).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	setDeathSignal(cmd.SysProcAttr)
}

func interrupt(cmd *exec.Cmd) {
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

func kill(cmd *exec.Cmd) {
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func killGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

// isProcess reports whether pid is alive and runs binary.
func isProcess(pid int, binary string) bool {
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && strings.HasPrefix(strings.TrimSpace(string(out)), binary)
}
