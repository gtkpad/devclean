//go:build linux

package fs

import (
	"os"
	"syscall"
)

func getFileInfoStat(info os.FileInfo) (dev uint64, ino uint64, blocks int64, apparent int64, ok bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, 0, 0, info.Size(), false
	}
	// On Linux, Stat_t.Dev is uint64
	return stat.Dev, stat.Ino, stat.Blocks, stat.Size, true
}
