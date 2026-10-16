//go:build !darwin && !linux

package fs

import "os"

func getFileInfoStat(info os.FileInfo) (dev uint64, ino uint64, blocks int64, apparent int64, ok bool) {
	return 0, 0, 0, info.Size(), false
}
