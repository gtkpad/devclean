package fs

import (
	"context"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
)

// CalcDirSize returns the total size in bytes of all regular files in the directory tree.
func CalcDirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == path {
				return err // root path inaccessible — propagate
			}
			return nil // skip inaccessible entries within the tree
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// CalcDirSizes calculates sizes of multiple directories in parallel with bounded concurrency.
func CalcDirSizes(ctx context.Context, paths []string) (map[string]int64, error) {
	results := make(map[string]int64, len(paths))
	var mu sync.Mutex

	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	var firstErr error

	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return results, err
		}

		select {
		case <-ctx.Done():
			return results, ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			defer func() { <-sem }()

			size, err := CalcDirSize(dir)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			results[dir] = size
		}(p)
	}

	wg.Wait()
	return results, firstErr
}
