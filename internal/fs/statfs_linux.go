//go:build linux

package fs

import "golang.org/x/sys/unix"

func getPlatformDiskSpace(path string) (DiskSpace, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return DiskSpace{}, err
	}
	bsize := uint64(stat.Bsize)
	total := uint64(stat.Blocks) * bsize
	free := uint64(stat.Bavail) * bsize
	var used uint64
	if total > free {
		used = total - free
	}
	return DiskSpace{
		TotalBytes: total,
		FreeBytes:  free,
		UsedBytes:  used,
	}, nil
}
