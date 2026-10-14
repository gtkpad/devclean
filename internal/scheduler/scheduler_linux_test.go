//go:build linux

package scheduler

import (
	"strings"
	"testing"
)

func TestSchedulerLinux_NewWithProviderLive(t *testing.T) {
	configDir := t.TempDir()

	sched := New(configDir)
	if sched == nil {
		t.Fatal("New(configDir) returned nil on Linux")
	}

	for _, prov := range []string{"", ProviderAuto, ProviderCron, ProviderSystemd} {
		s, err := NewWithProvider(configDir, prov)
		if err != nil {
			t.Fatalf("NewWithProvider(%q) unexpected error: %v", prov, err)
		}
		if s == nil {
			t.Fatalf("NewWithProvider(%q) returned nil", prov)
		}
	}

	// Unsupported provider=launchd on Linux
	_, err := NewWithProvider(configDir, ProviderLaunchd)
	if err == nil || !strings.Contains(err.Error(), "unsupported provider \"launchd\" on Linux") {
		t.Fatalf("expected unsupported provider error for launchd on Linux, got %v", err)
	}

	// Invalid provider on Linux
	_, err = NewWithProvider(configDir, "invalid-provider")
	if err == nil || !strings.Contains(err.Error(), "expected systemd, cron or auto") {
		t.Fatalf("expected invalid provider error on Linux, got %v", err)
	}
}

