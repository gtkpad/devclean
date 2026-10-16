package fs

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
)

// Usage captures apparent vs physically allocated byte usage, file counts,
// and partial degradation states for sparse files and hardlinked stores.
type Usage struct {
	ApparentBytes  int64
	AllocatedBytes int64
	FileCount      int
	Partial        bool
	StatusNote     string
}

// CalcUsage calculates disk usage for a single file or directory tree.
func CalcUsage(path string) (Usage, error) {
	return CalcUsageWithContext(context.Background(), path)
}

// CalcUsageWithContext traverses the path with context cancellation support.
func CalcUsageWithContext(ctx context.Context, path string) (Usage, error) {
	if err := ctx.Err(); err != nil {
		return Usage{}, err
	}

	info, err := os.Lstat(path)
	if err != nil {
		return Usage{Partial: true, StatusNote: err.Error()}, err
	}

	// Single file case (e.g. Docker.raw or data.img.raw)
	if !info.IsDir() {
		_, _, blocks, apparent, ok := getFileInfoStat(info)
		if !ok {
			return Usage{ApparentBytes: info.Size(), AllocatedBytes: info.Size(), FileCount: 1}, nil
		}
		alloc := blocks * 512
		return Usage{ApparentBytes: apparent, AllocatedBytes: alloc, FileCount: 1}, nil
	}

	// Directory tree traversal with inode deduplication
	seenInodes := make(map[[2]uint64]bool)
	var apparentTotal, allocatedTotal int64
	var fileCount int
	var partial bool
	var statusNote string

	walkErr := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if p == path {
				return err
			}
			partial = true
			if statusNote == "" {
				statusNote = "permission or access error on subpath: " + err.Error()
			}
			return nil // Skip inaccessible branch but preserve already scanned items (D-16)
		}

		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil // Skip symlinks to prevent duplicate accounting or directory escape (D-16)
		}

		fi, err := d.Info()
		if err != nil {
			partial = true
			if statusNote == "" {
				statusNote = "stat error on entry: " + err.Error()
			}
			return nil
		}

		dev, ino, blocks, apparent, ok := getFileInfoStat(fi)
		if ok && dev != 0 && ino != 0 {
			key := [2]uint64{dev, ino}
			if seenInodes[key] {
				return nil // Hardlink duplicate: do not count twice (D-07, D-11)
			}
			seenInodes[key] = true
			allocatedTotal += blocks * 512
			apparentTotal += apparent
		} else {
			allocatedTotal += fi.Size()
			apparentTotal += fi.Size()
		}
		fileCount++
		return nil
	})

	if walkErr != nil {
		return Usage{
			ApparentBytes:  apparentTotal,
			AllocatedBytes: allocatedTotal,
			FileCount:      fileCount,
			Partial:        partial,
			StatusNote:     statusNote,
		}, walkErr
	}

	return Usage{
		ApparentBytes:  apparentTotal,
		AllocatedBytes: allocatedTotal,
		FileCount:      fileCount,
		Partial:        partial,
		StatusNote:     statusNote,
	}, nil
}
