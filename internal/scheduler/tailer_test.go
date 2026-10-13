package scheduler

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestStreamLog_StreamingLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")

	// Create initial file
	if err := os.WriteFile(logPath, []byte("line 0\n"), 0644); err != nil {
		t.Fatalf("writing log: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var out safeBuffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- StreamLog(ctx, logPath, &out)
	}()

	// Wait for stream to start, then append lines
	time.Sleep(150 * time.Millisecond)

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("opening log for append: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString("line 1\nline 2\n"); err != nil {
		t.Fatalf("appending to log: %v", err)
	}

	// Give poll loop time to read
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got := out.String()
		if strings.Contains(got, "line 1") && strings.Contains(got, "line 2") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	err = <-errCh
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected StreamLog exit error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "line 1") || !strings.Contains(got, "line 2") {
		t.Errorf("expected lines 1 and 2 in output, got: %q", got)
	}
}

func TestStreamLog_RotationResilience(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")

	if err := os.WriteFile(logPath, []byte("pre-rotation 1\n"), 0644); err != nil {
		t.Fatalf("writing log: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var out safeBuffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- StreamLog(ctx, logPath, &out)
	}()

	time.Sleep(150 * time.Millisecond)

	// Append more pre-rotation data
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("opening log: %v", err)
	}
	_, _ = f.WriteString("pre-rotation 2\n")
	f.Close()

	// Wait until pre-rotation 2 is read
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), "pre-rotation 2") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Trigger rotation: rotate schedule.log to schedule.1.log
	if err := RotateIfNeeded(logPath, 1, 3); err != nil {
		t.Fatalf("rotating log: %v", err)
	}

	// Create new active log file with post-rotation content
	if err := os.WriteFile(logPath, []byte("post-rotation new stream\n"), 0644); err != nil {
		t.Fatalf("creating new log file: %v", err)
	}

	// Give poll loop time to detect rotation and read new file
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), "post-rotation new stream") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	err = <-errCh
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected StreamLog exit error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "post-rotation new stream") {
		t.Errorf("expected post-rotation content in output, got: %q", got)
	}
}

func TestStreamLog_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")

	if err := os.WriteFile(logPath, []byte("data\n"), 0644); err != nil {
		t.Fatalf("writing log: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var out safeBuffer
	err := StreamLog(ctx, logPath, &out)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestStreamLog_WaitForCreation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "schedule.log")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var out safeBuffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- StreamLog(ctx, logPath, &out)
	}()

	// Wait a moment while file does not exist
	time.Sleep(100 * time.Millisecond)

	// Now create file with content
	if err := os.WriteFile(logPath, []byte("created later\n"), 0644); err != nil {
		t.Fatalf("creating log later: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), "created later") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	err := <-errCh
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected exit error: %v", err)
	}

	if !strings.Contains(out.String(), "created later") {
		t.Errorf("expected 'created later' in output, got: %q", out.String())
	}
}
