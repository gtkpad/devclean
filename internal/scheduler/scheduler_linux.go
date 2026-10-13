//go:build linux

package scheduler

// New creates a platform-specific Scheduler on Linux using systemd user timers
// if available, or falls back to crontab.
func New(configDir string) Scheduler {
	sched, _ := NewWithProvider(configDir, ProviderAuto)
	return sched
}

// NewWithProvider creates a Scheduler on Linux according to the requested provider.
func NewWithProvider(configDir string, provider string) (Scheduler, error) {
	return newLinuxSchedulerWithExecutor(configDir, provider, &osExecutor{})
}
