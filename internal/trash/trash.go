package trash

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
)

// Manager handles trash operations: move, retention, and size enforcement.
type Manager struct {
	baseDir   string
	retention time.Duration
	maxSize   int64
}

// New creates a Manager from SafetyConfig.
// If TrashDir is empty, defaults to ~/.local/share/devclean/trash/.
func New(cfg config.SafetyConfig) *Manager {
	dir := cfg.TrashDir
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share", "devclean", "trash")
	}
	// Validate: reject paths with ".." segments
	if containsDotDot(dir) {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share", "devclean", "trash")
	}
	return &Manager{
		baseDir:   dir,
		retention: time.Duration(cfg.RetentionDays) * 24 * time.Hour,
		maxSize:   int64(cfg.MaxSizeMB) * 1024 * 1024,
	}
}

// BaseDir returns the trash root directory.
func (m *Manager) BaseDir() string {
	return m.baseDir
}

// ManifestPath returns the path to the manifest file.
func (m *Manager) ManifestPath() string {
	return filepath.Join(m.baseDir, "manifest.json")
}

// Move moves the given targets to the trash directory and records them in the manifest.
func (m *Manager) Move(ctx context.Context, targets []domain.Target, ecosystem string) error {
	if len(targets) == 0 {
		return nil
	}

	if err := os.MkdirAll(m.baseDir, 0o700); err != nil {
		return fmt.Errorf("creating trash dir: %w", err)
	}

	batchID := uuid.New().String()
	now := time.Now()

	batch := Batch{
		ID:        batchID,
		CreatedAt: now,
	}

	for _, t := range targets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if t.IsVirtual() {
			continue
		}

		entryID := uuid.New().String()
		entryDir := filepath.Join(m.baseDir, "batches", batchID, entryID)
		dataDir := filepath.Join(entryDir, "data")

		if err := os.MkdirAll(dataDir, 0o700); err != nil {
			return fmt.Errorf("creating entry dir: %w", err)
		}

		dest := filepath.Join(dataDir, filepath.Base(t.Path))
		if err := moveItem(t.Path, dest); err != nil {
			return fmt.Errorf("moving %s to trash: %w", t.Path, err)
		}

		entry := Entry{
			ID:           entryID,
			BatchID:      batchID,
			OriginalPath: t.Path,
			SizeBytes:    t.SizeBytes,
			Ecosystem:    ecosystem,
			MovedAt:      now,
		}
		batch.Entries = append(batch.Entries, entry)
		batch.TotalSize += t.SizeBytes
	}

	if len(batch.Entries) == 0 {
		return nil
	}

	manifest, err := LoadManifest(m.ManifestPath())
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}
	manifest.Batches = append(manifest.Batches, batch)
	if err := SaveManifest(m.ManifestPath(), manifest); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// CleanStats holds metrics about a purge or retention operation.
type CleanStats struct {
	BatchesPurged int   `json:"batches_purged"`
	EntriesPurged int   `json:"entries_purged"`
	BytesFreed    int64 `json:"bytes_freed"`
}

// CleanExpired removes entries older than the configured retention period and returns structured stats (D-09, D-11).
func (m *Manager) CleanExpired(ctx context.Context) (CleanStats, error) {
	manifest, err := LoadManifest(m.ManifestPath())
	if err != nil {
		return CleanStats{}, fmt.Errorf("loading manifest: %w", err)
	}

	var stats CleanStats
	now := time.Now()

	type expiredItem struct {
		batchID   string
		entryID   string
		sizeBytes int64
	}
	var expired []expiredItem

	for i := range manifest.Batches {
		b := &manifest.Batches[i]
		for j := range b.Entries {
			e := &b.Entries[j]
			if now.Sub(e.MovedAt) > m.retention {
				expired = append(expired, expiredItem{
					batchID:   b.ID,
					entryID:   e.ID,
					sizeBytes: e.SizeBytes,
				})
			}
		}
	}

	if len(expired) == 0 {
		return stats, nil
	}

	batchesTouched := make(map[string]bool)
	for _, item := range expired {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		entryDir := filepath.Join(m.baseDir, "batches", item.batchID, item.entryID)
		_ = os.RemoveAll(entryDir)
		manifest.RemoveEntry(item.entryID)

		stats.EntriesPurged++
		stats.BytesFreed += item.sizeBytes
		batchesTouched[item.batchID] = true
	}

	// Remove physical directory for any batch whose entries were all purged
	for bID := range batchesTouched {
		if manifest.FindBatch(bID) == nil {
			batchDir := filepath.Join(m.baseDir, "batches", bID)
			_ = os.RemoveAll(batchDir)
			stats.BatchesPurged++
		}
	}

	if err := SaveManifest(m.ManifestPath(), manifest); err != nil {
		return stats, fmt.Errorf("saving manifest: %w", err)
	}

	return stats, nil
}

// EnforceRetention removes entries older than the configured retention period.
func (m *Manager) EnforceRetention(ctx context.Context) error {
	_, err := m.CleanExpired(ctx)
	return err
}

// EnforceMaxSize purges oldest entries until total size is within limits.
func (m *Manager) EnforceMaxSize(ctx context.Context) error {
	manifest, err := LoadManifest(m.ManifestPath())
	if err != nil {
		return err
	}

	for manifest.TotalSize() > m.maxSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		oldest := m.findOldestEntry(manifest)
		if oldest == nil {
			break
		}
		m.removeEntryData(manifest, oldest.ID)
	}

	return SaveManifest(m.ManifestPath(), manifest)
}

// List returns all batches in the manifest, newest first.
func (m *Manager) List(_ context.Context) ([]Batch, error) {
	manifest, err := LoadManifest(m.ManifestPath())
	if err != nil {
		return nil, err
	}
	// Return in reverse order (newest first)
	batches := make([]Batch, len(manifest.Batches))
	for i, b := range manifest.Batches {
		batches[len(manifest.Batches)-1-i] = b
	}
	return batches, nil
}

// Purge removes expired entries, or all entries if all is true.
func (m *Manager) Purge(ctx context.Context, all bool) error {
	if all {
		manifest, err := LoadManifest(m.ManifestPath())
		if err != nil {
			return err
		}
		// Remove all batch data
		for _, b := range manifest.Batches {
			batchDir := filepath.Join(m.baseDir, "batches", b.ID)
			os.RemoveAll(batchDir)
		}
		manifest.Batches = nil
		return SaveManifest(m.ManifestPath(), manifest)
	}
	return m.EnforceRetention(ctx)
}

// SanitizeOrphans removes unlisted physical batch directories from batches/ (D-11).
func (m *Manager) SanitizeOrphans(ctx context.Context) error {
	manifest, err := LoadManifest(m.ManifestPath())
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	knownBatches := make(map[string]bool, len(manifest.Batches))
	for _, b := range manifest.Batches {
		knownBatches[b.ID] = true
	}

	batchesDir := filepath.Join(m.baseDir, "batches")
	entries, err := os.ReadDir(batchesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading batches directory: %w", err)
	}

	for _, e := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if e.IsDir() && !knownBatches[e.Name()] {
			_ = os.RemoveAll(filepath.Join(batchesDir, e.Name()))
		}
	}
	return nil
}

func (m *Manager) findOldestEntry(manifest *Manifest) *Entry {
	var oldest *Entry
	for i := range manifest.Batches {
		for j := range manifest.Batches[i].Entries {
			e := &manifest.Batches[i].Entries[j]
			if oldest == nil || e.MovedAt.Before(oldest.MovedAt) {
				oldest = e
			}
		}
	}
	return oldest
}

func (m *Manager) removeEntryData(manifest *Manifest, entryID string) {
	entry, batch := manifest.FindEntry(entryID)
	if entry == nil {
		return
	}
	batchID := batch.ID
	entryDir := filepath.Join(m.baseDir, "batches", batchID, entryID)
	_ = os.RemoveAll(entryDir)
	manifest.RemoveEntry(entryID)
	if manifest.FindBatch(batchID) == nil {
		batchDir := filepath.Join(m.baseDir, "batches", batchID)
		_ = os.RemoveAll(batchDir)
	}
}

// moveItem moves src to dest, falling back to copy+remove on cross-device errors.
func moveItem(src, dest string) error {
	err := os.Rename(src, dest)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	// Cross-device: copy then remove
	if err := copyDir(src, dest); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyDir(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}

func containsDotDot(path string) bool {
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		if part == ".." {
			return true
		}
	}
	return false
}
