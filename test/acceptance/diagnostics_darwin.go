//go:build acceptance && darwin

package acceptance

import (
	"golang.org/x/sys/unix"
)

func probeOSVersion() string {
	ver, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return "unknown"
	}
	return ver
}

func probeAPFS(path string) (bool, string) {
	if path == "" {
		return false, "empty path"
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return false, "statfs_error"
	}
	var buf []byte
	for _, b := range stat.Fstypename {
		if b == 0 {
			break
		}
		buf = append(buf, byte(b))
	}
	fsType := string(buf)
	return fsType == "apfs", fsType
}
