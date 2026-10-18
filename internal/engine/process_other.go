//go:build !linux && !windows

package engine

import "syscall"

// Only Linux has a parent-death signal; elsewhere ReapOrphans does the job.
func setDeathSignal(*syscall.SysProcAttr) {}
