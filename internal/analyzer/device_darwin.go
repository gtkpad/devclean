//go:build darwin

package analyzer

import (
	"io/fs"
	"syscall"
)

func getDeviceID(fi fs.FileInfo) (uint64, bool) {
	if fi == nil {
		return 0, false
	}
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(stat.Dev), true
	}
	return 0, false
}
