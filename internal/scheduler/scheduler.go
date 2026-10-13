package scheduler

import (
	"errors"
	"time"
)

// Provider constants for scheduler backends.
const (
	ProviderSystemd = "systemd"
	ProviderCron    = "cron"
	ProviderLaunchd = "launchd"
	ProviderAuto    = "auto"

	LingerHint = "Dica: Para garantir que o timer execute mesmo quando você não estiver logado, habilite o linger com: loginctl enable-linger $USER"
)

// ErrAlreadyInstalled is returned when a schedule is already registered.
var ErrAlreadyInstalled = errors.New("schedule already installed")

// ErrNotInstalled is returned when no schedule exists to remove.
var ErrNotInstalled = errors.New("schedule not installed")

// Schedule holds the parameters for registering a scheduled cleanup.
type Schedule struct {
	CronExpr   string
	BinaryPath string
}

// Status represents the current state of the scheduled task.
type Status struct {
	Active       bool
	CronExpr     string
	BinaryPath   string
	LastRun      *time.Time
	LastResult   *RunResult
	NextEstimate *time.Time
}

// RunResult records the outcome of an automated cleanup execution.
type RunResult struct {
	Timestamp    time.Time `json:"timestamp"`
	TargetsCount int       `json:"targets_count"`
	BytesFreed   int64     `json:"bytes_freed"`
	Duration     string    `json:"duration"`
}

// Scheduler manages platform-specific scheduled task registration.
type Scheduler interface {
	Install(Schedule) error
	Remove() error
	Status() (*Status, error)
	IsInstalled() bool
	InstallWatch(WatchSchedule) error
	RemoveWatch() error
	IsWatchInstalled() bool
}

// WatchSchedule holds the parameters for registering a watch daemon.
type WatchSchedule struct {
	BinaryPath string
	Interval   time.Duration
}
