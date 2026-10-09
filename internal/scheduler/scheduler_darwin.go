//go:build darwin

package scheduler

// New creates a platform-specific Scheduler using Launchd on macOS.
func New(configDir string) Scheduler {
	return NewLaunchdSchedulerWithExecutor(configDir, &osExecutor{}, "")
}
