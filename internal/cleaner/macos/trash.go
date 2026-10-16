package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

// scanTrash inventories $HOME/.Trash as ReviewOnly by default (MAC-01, D-01, D-02).
// Deliberately ignores external volumes /Volumes/*/.Trashes.
func scanTrash(homeDir string) domain.Target {
	trashDir := filepath.Join(homeDir, ".Trash")
	info, err := os.Stat(trashDir)
	if err != nil && os.IsNotExist(err) {
		return domain.Target{}
	}
	if info != nil && !info.IsDir() {
		return domain.Target{}
	}

	usage, err := fs.CalcUsage(trashDir)
	if err != nil && os.IsNotExist(err) {
		return domain.Target{}
	}

	size := usage.AllocatedBytes
	if size == 0 {
		size = usage.ApparentBytes
	}

	return domain.Target{
		ID:            "macos-trash",
		Name:          "macOS Trash (~/.Trash)",
		Path:          trashDir,
		SizeBytes:     size,
		EligibleBytes: 0, // INVARIANT: Always 0 on scan (D-01)
		TTLEligible:   false,
		ReviewOnly:    true, // INVARIANT: Always ReviewOnly on scan (D-01)
		ReviewReason:  "trash_requires_flag",
		Risk:          domain.RiskLow,
		Category:      "macos-trash",
	}
}

// cleanTrash purges items inside $HOME/.Trash directly without internal quarantine (MAC-01, D-03, D-04).
func cleanTrash(ctx context.Context, trashDir string) error {
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var remainingBytes int64
	var errs []error

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		itemPath := filepath.Join(trashDir, entry.Name())

		// Attempt chmod u+w in case item was marked read-only (D-04)
		_ = fs.ChmodRecursive(itemPath, 0755, 0644)

		if err := os.RemoveAll(itemPath); err != nil {
			usage, _ := fs.CalcUsage(itemPath)
			remaining := usage.AllocatedBytes
			if remaining == 0 {
				remaining = usage.ApparentBytes
			}
			remainingBytes += remaining
			errs = append(errs, fmt.Errorf("could not remove %s: %w", itemPath, err))
		}
	}

	// Guarantee root preservation: never call os.RemoveAll(trashDir) (D-03)
	if len(errs) > 0 {
		return fmt.Errorf("trash cleanup completed with %d remaining bytes: %v", remainingBytes, errs[0])
	}
	return nil
}
