//go:build !windows

package sandbox

import (
	"os"
	"syscall"
)

func fileOwner(info os.FileInfo) (int, bool) {
	if info == nil {
		return 0, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return int(st.Uid), ok
}
