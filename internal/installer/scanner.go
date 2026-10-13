package installer

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/safety"
)

// Scanner scans specified filesystem locations for installer candidates.
type Scanner struct {
	opts      Options
	blacklist *safety.Blacklist
	detector  *Detector
}

// NewScanner creates a new Scanner with safety configurations and options.
func NewScanner(opts Options, bl *safety.Blacklist) *Scanner {
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 2
	}
	if opts.MinAge < 0 {
		opts.MinAge = 24 * time.Hour
	}
	return &Scanner{
		opts:      opts,
		blacklist: bl,
		detector:  NewDetector(),
	}
}

// Scan performs depth-limited scanning on configured directories.
func (s *Scanner) Scan(ctx context.Context) ([]Candidate, error) {
	var candidates []Candidate
	now := time.Now()

	for _, rawBase := range s.opts.Paths {
		baseDir, err := expandTilde(rawBase)
		if err != nil {
			continue
		}
		if _, err := os.Stat(baseDir); os.IsNotExist(err) {
			continue
		}

		err = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // Tolerate restricted directory permissions
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if path == baseDir {
				return nil
			}

			// Blacklist enforcement (NFR-03)
			if s.blacklist != nil && s.blacklist.IsBlacklisted(path) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Ignore hidden files and directories (dotfiles)
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Relative depth limiter (D-02)
			rel, err := filepath.Rel(baseDir, path)
			if err != nil {
				return nil
			}
			depth := strings.Count(rel, string(filepath.Separator)) + 1
			if depth > s.opts.MaxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Symlink isolation (T-02-02): ignore symlinks to prevent loops and escapes
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}

			// Special handling for macOS .app bundles (D-04)
			if d.IsDir() {
				if strings.HasSuffix(strings.ToLower(name), ".app") {
					info, err := d.Info()
					if err == nil {
						if now.Sub(info.ModTime()) >= s.opts.MinAge {
							size := calculateDirSize(path)
							if size >= s.opts.MinSize {
								candidates = append(candidates, Candidate{
									Path:      path,
									SizeBytes: size,
									ModTime:   info.ModTime(),
									Type:      TypeDirectApp,
									Detail:    "aplicativo macOS (.app)",
								})
							}
						}
					}
					return filepath.SkipDir
				}
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			// MinAge and MinSize filters (D-03)
			if now.Sub(info.ModTime()) < s.opts.MinAge {
				return nil
			}
			if info.Size() < s.opts.MinSize {
				return nil
			}

			cand, matched, err := s.detector.Detect(path, info)
			if err == nil && matched && cand != nil {
				candidates = append(candidates, *cand)
			}

			return nil
		})

		if err != nil && err != context.Canceled {
			return nil, err
		}
		if err == context.Canceled {
			return nil, err
		}
	}

	return candidates, nil
}

func expandTilde(path string) (string, error) {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func calculateDirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
