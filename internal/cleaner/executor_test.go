package cleaner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
)

func TestMockCommandExecutor(t *testing.T) {
	ctx := context.Background()
	mock := cleaner.NewMockExecutor()

	// Verify initial state
	if len(mock.Calls) != 0 {
		t.Fatalf("expected 0 calls, got %d", len(mock.Calls))
	}

	// Setup mock outputs and errors
	mock.Outputs["git status"] = []byte("clean")
	mock.Outputs["pnpm store path"] = []byte("/Users/test/.pnpm-store")
	mock.Errors["git commit -m test"] = errors.New("nothing to commit")
	mock.Outputs["lookpath:custom"] = []byte("/opt/bin/custom")
	mock.Errors["lookpath:missing"] = errors.New("executable file not found in $PATH")

	// Test successful Output call
	out, err := mock.Output(ctx, "git", "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "clean" {
		t.Fatalf("expected 'clean', got '%s'", string(out))
	}

	// Test error Output call
	out, err = mock.Output(ctx, "git", "commit", "-m", "test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "nothing to commit" {
		t.Fatalf("expected 'nothing to commit', got '%v'", err)
	}

	// Test unconfigured Output call (returns nil, nil)
	out, err = mock.Output(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error for unconfigured call: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil output, got %v", out)
	}

	// Verify Calls recording
	if len(mock.Calls) != 3 {
		t.Fatalf("expected 3 recorded calls, got %d", len(mock.Calls))
	}
	if mock.Calls[0][0] != "git" || mock.Calls[0][1] != "status" {
		t.Fatalf("unexpected call 0: %v", mock.Calls[0])
	}
	if mock.Calls[1][0] != "git" || mock.Calls[1][1] != "commit" || mock.Calls[1][2] != "-m" || mock.Calls[1][3] != "test" {
		t.Fatalf("unexpected call 1: %v", mock.Calls[1])
	}

	// Test LookPath with configured output
	path, err := mock.LookPath("custom")
	if err != nil {
		t.Fatalf("unexpected LookPath error: %v", err)
	}
	if path != "/opt/bin/custom" {
		t.Fatalf("expected '/opt/bin/custom', got '%s'", path)
	}

	// Test LookPath with configured error
	_, err = mock.LookPath("missing")
	if err == nil {
		t.Fatal("expected LookPath error for missing binary, got nil")
	}

	// Test LookPath fallback
	path, err = mock.LookPath("pnpm")
	if err != nil {
		t.Fatalf("unexpected LookPath fallback error: %v", err)
	}
	if path != "/usr/local/bin/pnpm" {
		t.Fatalf("expected fallback '/usr/local/bin/pnpm', got '%s'", path)
	}
}

func TestDefaultExecutorInterface(t *testing.T) {
	// Verify DefaultExecutor implements CommandExecutor interface
	var _ cleaner.CommandExecutor = (*cleaner.DefaultExecutor)(nil)
	var _ cleaner.CommandExecutor = (*cleaner.MockCommandExecutor)(nil)
}
