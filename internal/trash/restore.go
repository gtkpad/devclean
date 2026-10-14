package trash

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gtkpad/devclean/internal/safety"
)

// UndoLast restores the entire last batch to their original paths.
func (m *Manager) UndoLast(ctx context.Context) error {
	manifest, err := LoadManifest(m.ManifestPath())
	if err != nil {
		return err
	}

	batch := manifest.LastBatch()
	if batch == nil {
		return errors.New("no batches to restore")
	}

	if err := m.restoreBatch(ctx, batch); err != nil {
		return err
	}

	manifest.RemoveBatch(batch.ID)
	return SaveManifest(m.ManifestPath(), manifest)
}

// Undo restores by batch ID or entry ID.
func (m *Manager) Undo(ctx context.Context, id string) error {
	manifest, err := LoadManifest(m.ManifestPath())
	if err != nil {
		return err
	}

	if batch := manifest.FindBatch(id); batch != nil {
		if err := m.restoreBatch(ctx, batch); err != nil {
			return err
		}
		manifest.RemoveBatch(id)
		return SaveManifest(m.ManifestPath(), manifest)
	}

	entry, batch := manifest.FindEntry(id)
	if entry != nil {
		if err := m.restoreEntry(ctx, entry, batch.ID); err != nil {
			return err
		}
		manifest.RemoveEntry(id)
		return SaveManifest(m.ManifestPath(), manifest)
	}

	return fmt.Errorf("no batch or entry found with ID %s", id)
}

// UndoEntry restores a single entry by its ID.
func (m *Manager) UndoEntry(ctx context.Context, entryID string) error {
	manifest, err := LoadManifest(m.ManifestPath())
	if err != nil {
		return err
	}

	entry, batch := manifest.FindEntry(entryID)
	if entry == nil {
		return fmt.Errorf("entry not found: %s", entryID)
	}

	if err := m.restoreEntry(ctx, entry, batch.ID); err != nil {
		return err
	}

	manifest.RemoveEntry(entryID)
	return SaveManifest(m.ManifestPath(), manifest)
}

// restoreBatch validates all entries first, then restores them. Aborts entirely on any validation failure.
func (m *Manager) restoreBatch(ctx context.Context, batch *Batch) error {
	// Step 1: validate all entries before restoring any
	for i := range batch.Entries {
		if err := m.validateRestore(batch.Entries[i].OriginalPath); err != nil {
			return fmt.Errorf("restore aborted: entry %s: %w", batch.Entries[i].ID, err)
		}
	}

	// Step 2: restore all entries
	for i := range batch.Entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := m.restoreEntry(ctx, &batch.Entries[i], batch.ID); err != nil {
			return err
		}
	}
	return nil
}

// restoreEntry moves a single entry's data back to its original path.
func (m *Manager) restoreEntry(_ context.Context, entry *Entry, batchID string) error {
	if err := m.validateRestore(entry.OriginalPath); err != nil {
		return err
	}

	entryDir := filepath.Join(m.baseDir, "batches", batchID, entry.ID, "data")
	items, err := os.ReadDir(entryDir)
	if err != nil {
		return fmt.Errorf("reading trash entry data: %w", err)
	}
	if len(items) == 0 {
		return fmt.Errorf("trash entry data is empty: %s", entry.ID)
	}

	// The stored item is the first entry in data/
	src := filepath.Join(entryDir, items[0].Name())
	dest := entry.OriginalPath

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating parent dir: %w", err)
	}

	if err := restoreMove(src, dest); err != nil {
		return fmt.Errorf("restoring %s: %w", entry.OriginalPath, err)
	}

	// Clean up the entry directory from trash
	batchEntryDir := filepath.Join(m.baseDir, "batches", batchID, entry.ID)
	os.RemoveAll(batchEntryDir)

	return nil
}

// validateRestore checks that restoring to the given path is safe.
func (m *Manager) validateRestore(originalPath string) error {
	// Path conflict: destination already exists
	if _, err := os.Stat(originalPath); err == nil {
		return fmt.Errorf("path conflict: %s already exists", originalPath)
	}

	parentDir := filepath.Dir(originalPath)

	// Parent is a symlink — prevents symlink escape attacks where the parent
	// directory was replaced with a symlink after the item was trashed.
	isLink, err := safety.IsSymlink(parentDir)
	if err == nil && isLink {
		return fmt.Errorf("restore destination parent is a symlink: %s", parentDir)
	}

	return nil
}

func restoreMove(src, dest string) error {
	err := os.Rename(src, dest)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyDir(src, dest); err != nil {
		return err
	}
	return os.RemoveAll(src)
}
