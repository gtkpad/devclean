package watch

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func fakeScanFunc(results []domain.ScanResult) ScanFunc {
	return func(ctx context.Context) <-chan domain.ScanResult {
		ch := make(chan domain.ScanResult, len(results))
		for _, r := range results {
			ch <- r
		}
		close(ch)
		return ch
	}
}

func TestRunOnce_AccumulatesBytes(t *testing.T) {
	results := []domain.ScanResult{
		{Ecosystem: "node", Targets: []domain.Target{
			{SizeBytes: 500 * 1024 * 1024},
			{SizeBytes: 300 * 1024 * 1024},
		}},
		{Ecosystem: "go", Targets: []domain.Target{
			{SizeBytes: 200 * 1024 * 1024},
		}},
	}

	w := &Watcher{
		ScanFn:         fakeScanFunc(results),
		Interval:       time.Hour,
		ThresholdBytes: 1024 * 1024 * 1024,
		Notifier:       &NoopNotifier{},
		Logger:         slog.Default(),
	}

	totalBytes, count, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	expectedBytes := int64(1000 * 1024 * 1024)
	if totalBytes != expectedBytes {
		t.Errorf("totalBytes = %d, want %d", totalBytes, expectedBytes)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestRunOnce_SkipsErrors(t *testing.T) {
	results := []domain.ScanResult{
		{Ecosystem: "node", Targets: []domain.Target{
			{SizeBytes: 500 * 1024 * 1024},
		}},
		{Ecosystem: "docker", Error: context.DeadlineExceeded},
	}

	w := &Watcher{
		ScanFn:         fakeScanFunc(results),
		Interval:       time.Hour,
		ThresholdBytes: 1024 * 1024 * 1024,
		Notifier:       &NoopNotifier{},
		Logger:         slog.Default(),
	}

	totalBytes, count, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if totalBytes != 500*1024*1024 {
		t.Errorf("totalBytes = %d, want %d", totalBytes, int64(500*1024*1024))
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestScanAndNotify_NotifiesAboveThreshold(t *testing.T) {
	results := []domain.ScanResult{
		{Ecosystem: "node", Targets: []domain.Target{
			{SizeBytes: 3 * 1024 * 1024 * 1024}, // 3GB
		}},
	}

	notifier := &NoopNotifier{}
	w := &Watcher{
		ScanFn:         fakeScanFunc(results),
		Interval:       time.Hour,
		ThresholdBytes: 2 * 1024 * 1024 * 1024, // 2GB threshold
		Notifier:       notifier,
		Logger:         slog.Default(),
	}

	w.scanAndNotify(context.Background())

	if notifier.Called != 1 {
		t.Errorf("Notifier.Called = %d, want 1", notifier.Called)
	}
	if notifier.LastTitle != "DevClean" {
		t.Errorf("LastTitle = %q, want DevClean", notifier.LastTitle)
	}
}

func TestScanAndNotify_NoNotificationBelowThreshold(t *testing.T) {
	results := []domain.ScanResult{
		{Ecosystem: "node", Targets: []domain.Target{
			{SizeBytes: 500 * 1024 * 1024}, // 500MB
		}},
	}

	notifier := &NoopNotifier{}
	w := &Watcher{
		ScanFn:         fakeScanFunc(results),
		Interval:       time.Hour,
		ThresholdBytes: 2 * 1024 * 1024 * 1024, // 2GB threshold
		Notifier:       notifier,
		Logger:         slog.Default(),
	}

	w.scanAndNotify(context.Background())

	if notifier.Called != 0 {
		t.Errorf("Notifier.Called = %d, want 0 (below threshold)", notifier.Called)
	}
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	results := []domain.ScanResult{
		{Ecosystem: "node", Targets: []domain.Target{
			{SizeBytes: 100 * 1024 * 1024},
		}},
	}

	w := &Watcher{
		ScanFn:         fakeScanFunc(results),
		Interval:       50 * time.Millisecond,
		ThresholdBytes: 1024 * 1024 * 1024,
		Notifier:       &NoopNotifier{},
		Logger:         slog.Default(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err := w.Run(ctx)
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestNoopNotifier(t *testing.T) {
	n := &NoopNotifier{}
	err := n.Notify("Title", "Message")
	if err != nil {
		t.Fatal(err)
	}
	if n.Called != 1 {
		t.Errorf("Called = %d, want 1", n.Called)
	}
	if n.LastTitle != "Title" {
		t.Errorf("LastTitle = %q, want Title", n.LastTitle)
	}
	if n.LastMessage != "Message" {
		t.Errorf("LastMessage = %q, want Message", n.LastMessage)
	}
}
