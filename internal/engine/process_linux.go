package engine

import "syscall"

// Linux kills the engine when Mihari dies, however it dies. Its own children
// share its group and are reaped on the next start.
func setDeathSignal(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGKILL
}
