package hooks

import (
	"context"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
)

func TestRunPreDelete_Success(t *testing.T) {
	runner := New(config.HooksConfig{
		Node: config.HookConfig{
			PreDelete: "echo pre-delete {{.Path}}",
			Timeout:   "5s",
		},
	})

	err := runner.RunPreDelete(context.Background(), "node", HookData{
		Path:      "/tmp/test",
		Ecosystem: "node",
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestRunPreDelete_Timeout(t *testing.T) {
	runner := New(config.HooksConfig{
		Node: config.HookConfig{
			PreDelete: "sleep 5",
			Timeout:   "50ms",
		},
	})

	err := runner.RunPreDelete(context.Background(), "node", HookData{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestRunPreDelete_PlatformMismatch(t *testing.T) {
	runner := New(config.HooksConfig{
		Node: config.HookConfig{
			PreDelete: "echo should-not-run",
			Platform:  []string{"nonexistent-os"},
			Timeout:   "5s",
		},
	})

	err := runner.RunPreDelete(context.Background(), "node", HookData{})
	if err != nil {
		t.Fatalf("expected nil for platform mismatch, got: %v", err)
	}
}

func TestRunPreDelete_NoHookConfigured(t *testing.T) {
	runner := New(config.HooksConfig{})

	err := runner.RunPreDelete(context.Background(), "node", HookData{})
	if err != nil {
		t.Fatalf("expected nil for no hook, got: %v", err)
	}
}

func TestRunPostDelete_FailureReturnsError(t *testing.T) {
	runner := New(config.HooksConfig{
		Go: config.HookConfig{
			PostDelete: "exit 1",
			Timeout:    "5s",
		},
	})

	err := runner.RunPostDelete(context.Background(), "go", HookData{})
	if err == nil {
		t.Fatal("expected error for exit 1")
	}
}

func TestRenderTemplate_AllVariables(t *testing.T) {
	tmpl := "{{.Path}} {{.Ecosystem}} {{.FreedSize}} {{.TargetCount}}"
	data := HookData{
		Path:        "/tmp/test",
		Ecosystem:   "node",
		FreedSize:   "340MB",
		TargetCount: 5,
	}
	result, err := renderTemplate(tmpl, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	expected := "/tmp/test node 340MB 5"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRenderTemplate_InvalidTemplate(t *testing.T) {
	_, err := renderTemplate("{{.Invalid", HookData{})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}
