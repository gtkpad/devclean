package fs

import (
	"context"
	"os"
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

	return Usage{}, nil
}
