//go:build linux

package scheduler

// New creates a platform-specific Scheduler using crontab on Linux.
func New(configDir string) Scheduler {
	return NewCronSchedulerWithExecutor(configDir, &osExecutor{})
}
