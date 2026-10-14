//go:build darwin

package scheduler

import (
	"strings"
	"testing"
)

func TestSchedulerDarwin_NewWithProvider(t *testing.T) {
	configDir := t.TempDir()

	// Supported providers: "", "launchd", "auto"
	for _, prov := range []string{"", ProviderLaunchd, ProviderAuto} {
		sched, err := NewWithProvider(configDir, prov)
		if err != nil {
			t.Fatalf("NewWithProvider(%q) unexpected error: %v", prov, err)
		}
		if _, ok := sched.(*launchdScheduler); !ok {
			t.Errorf("NewWithProvider(%q) expected *launchdScheduler, got %T", prov, sched)
		}
	}

	// Incompatible providers: "systemd", "cron"
	for _, prov := range []string{ProviderSystemd, ProviderCron} {
		_, err := NewWithProvider(configDir, prov)
		if err == nil {
			t.Fatalf("NewWithProvider(%q) expected error, got nil", prov)
		}
		wantSub := "launchd is the only supported scheduler"
		if !strings.Contains(err.Error(), wantSub) {
			t.Errorf("NewWithProvider(%q) error = %v, want substring %q", prov, err, wantSub)
		}
	}

	// Invalid provider
	_, err := NewWithProvider(configDir, "windows-tasks")
	if err == nil {
		t.Fatal("NewWithProvider(invalid) expected error, got nil")
	}
	wantSub := "expected launchd or auto"
	if !strings.Contains(err.Error(), wantSub) {
		t.Errorf("NewWithProvider(invalid) error = %v, want substring %q", err, wantSub)
	}
}
