//go:build darwin

package scheduler

import "fmt"

// New creates a platform-specific Scheduler using Launchd on macOS.
func New(configDir string) Scheduler {
	sched, _ := NewWithProvider(configDir, ProviderAuto)
	return sched
}

// NewWithProvider creates a Scheduler on macOS according to the requested provider.
func NewWithProvider(configDir string, provider string) (Scheduler, error) {
	switch provider {
	case "", ProviderLaunchd, ProviderAuto:
		return NewLaunchdSchedulerWithExecutor(configDir, &osExecutor{}, ""), nil
	case ProviderSystemd, ProviderCron:
		return nil, fmt.Errorf("unsupported provider %q on macOS: launchd is the only supported scheduler", provider)
	default:
		return nil, fmt.Errorf("invalid provider %q: expected launchd or auto", provider)
	}
}
