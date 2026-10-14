package scheduler

import (
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultMaxLogSizeBytes is the maximum log size (10 MB) before rotation is triggered.
	DefaultMaxLogSizeBytes int64 = 10 * 1024 * 1024

	// DefaultMaxBackups is the maximum number of historical rotated logs kept.
	DefaultMaxBackups int = 3
)

// RotateIfNeeded checks if the log file at logPath exceeds maxBytes.
// If it does, it performs a circular rotation maintaining up to maxBackups
// historical backups (.1.log, .2.log, ..., .<maxBackups>.log).
//
// If maxBytes <= 0, DefaultMaxLogSizeBytes is used.
// If maxBackups <= 0, DefaultMaxBackups is used.
// If logPath is a symlink, it is safely removed to prevent symlink attacks (T-03-06).
func RotateIfNeeded(logPath string, maxBytes int64, maxBackups int) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxLogSizeBytes
	}
	if maxBackups <= 0 {
		maxBackups = DefaultMaxBackups
	}

	fi, err := os.Lstat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Symlink check: never follow or append to a symlinked log file (T-03-06)
	if fi.Mode()&os.ModeSymlink != 0 {
		return os.Remove(logPath)
	}

	if fi.Size() < maxBytes {
		return nil
	}

	base := strings.TrimSuffix(logPath, ".log")

	// 1. Remove oldest backup if it exists (e.g. schedule.3.log)
	oldest := fmt.Sprintf("%s.%d.log", base, maxBackups)
	_ = os.Remove(oldest)

	// 2. Shift older backups in descending order (2 -> 3, 1 -> 2)
	for i := maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d.log", base, i)
		dst := fmt.Sprintf("%s.%d.log", base, i+1)
		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}

	// 3. Rename active log to .1.log
	first := fmt.Sprintf("%s.1.log", base)
	return os.Rename(logPath, first)
}
