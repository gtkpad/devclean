package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// IsSystemdAvailable checks whether the systemd user daemon is available and operational.
func IsSystemdAvailable(exec CommandExecutor) bool {
	if exec == nil {
		exec = &osExecutor{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CombinedOutput(ctx, "systemctl", "--user", "is-system-running")
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "not found in PATH") {
			return false
		}
	}

	state := strings.TrimSpace(string(out))
	switch state {
	case "running", "degraded", "starting", "initializing":
		return true
	default:
		return false
	}
}

// newLinuxSchedulerWithExecutor constructs a Scheduler for Linux based on the provider string and executor.
func newLinuxSchedulerWithExecutor(configDir string, provider string, exec CommandExecutor) (Scheduler, error) {
	switch provider {
	case ProviderSystemd:
		return NewSystemdSchedulerWithExecutor(configDir, exec, ""), nil
	case ProviderCron:
		return NewCronSchedulerWithExecutor(configDir, exec), nil
	case "", ProviderAuto:
		if IsSystemdAvailable(exec) {
			return NewSystemdSchedulerWithExecutor(configDir, exec, ""), nil
		}
		return NewCronSchedulerWithExecutor(configDir, exec), nil
	case ProviderLaunchd:
		return nil, fmt.Errorf("unsupported provider %q on Linux: use systemd or cron", provider)
	default:
		return nil, fmt.Errorf("invalid provider %q: expected systemd, cron or auto", provider)
	}
}
