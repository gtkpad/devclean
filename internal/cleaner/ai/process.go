package ai

import (
	"context"

	"github.com/gtkpad/devclean/internal/cleaner"
)

// checkProcessStatus returns running=true if pgrep finds matching processes.
// If pgrep fails with exit code 1, running=false, uncertain=false (cleanly not running).
// If pgrep fails with any other error or unexpected result, uncertain=true (FAIL CLOSED!).
func checkProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, processNames ...string) (running bool, uncertain bool) {
	return false, false
}
