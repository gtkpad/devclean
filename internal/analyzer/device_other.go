//go:build !darwin && !linux

package analyzer

import "io/fs"

func getDeviceID(fi fs.FileInfo) (uint64, bool) {
	return 0, false
}
