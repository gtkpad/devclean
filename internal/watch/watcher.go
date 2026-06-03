package watch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

// ScanFunc abstracts the scan operation for testability.
type ScanFunc func(ctx context.Context) <-chan domain.ScanResult

// Watcher runs periodic scans and notifies when space exceeds threshold.
type Watcher struct {
	ScanFn         ScanFunc
	Interval       time.Duration
	ThresholdBytes int64
	Notifier       Notifier
	Logger         *slog.Logger
}

// Run starts the watch loop. Blocks until ctx is cancelled.
// Runs one immediate scan, then repeats at Interval.
func (w *Watcher) Run(ctx context.Context) error {
	w.scanAndNotify(ctx)

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.scanAndNotify(ctx)
		}
	}
}

// RunOnce executes a single scan cycle (for foreground `devclean watch`).
func (w *Watcher) RunOnce(ctx context.Context) (totalBytes int64, targetCount int, err error) {
	return w.doScan(ctx)
}

func (w *Watcher) scanAndNotify(ctx context.Context) {
	totalBytes, targetCount, err := w.doScan(ctx)
	if err != nil {
		w.Logger.Warn("watch scan failed", "error", err)
		return
	}

	if totalBytes >= w.ThresholdBytes {
		msg := fmt.Sprintf("%.1fGB reclaimable found in %d targets. Run devclean to clean up.",
			float64(totalBytes)/(1024*1024*1024), targetCount)
		if err := w.Notifier.Notify("DevClean", msg); err != nil {
			w.Logger.Warn("notification failed", "error", err)
		}
	}
}

func (w *Watcher) doScan(ctx context.Context) (int64, int, error) {
	results := w.ScanFn(ctx)
	var totalBytes int64
	var count int
	for result := range results {
		if result.Error != nil {
			continue
		}
		for _, t := range result.Targets {
			totalBytes += t.SizeBytes
			count++
		}
	}
	return totalBytes, count, nil
}
