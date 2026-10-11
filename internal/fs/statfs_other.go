//go:build !darwin && !linux

package fs

import "fmt"

func getPlatformDiskSpace(path string) (DiskSpace, error) {
	return DiskSpace{}, fmt.Errorf("statfs not supported on this platform")
}
