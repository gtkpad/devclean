package browsers

import (
	"context"
	"errors"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
)

// CheckBrowserProcessStatus verifies whether any main or helper process of a browser is running.
// Returns:
//   - running=true, uncertain=false: At least one process was found running (exit code 0 with output).
//   - running=false, uncertain=false: All configured processes are cleanly stopped (exit code 1).
//   - running=false, uncertain=true: pgrep failed unexpectedly (exit code > 1, command missing, timeout) -> FAIL CLOSED!
func CheckBrowserProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, bDef BrowserDef, goos string) (running bool, uncertain bool) {
	if exec == nil {
		return false, true
	}
	names := bDef.ProcessNames[goos]
	if len(names) == 0 {
		return false, false
	}

	for _, name := range names {
		out, err := exec.Output(ctx, "pgrep", "-x", name)
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return true, false
		}
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			continue // Cleanly stopped for this process name
		}
		// Unexpected error: missing binary, exit code != 1, permission denial -> FAIL CLOSED!
		return false, true
	}
	return false, false
}
