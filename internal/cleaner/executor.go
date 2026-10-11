package cleaner

import (
	"context"
	"os/exec"
)

// CommandExecutor abstracts operating system command execution.
type CommandExecutor interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	LookPath(file string) (string, error)
}

// DefaultExecutor is the default production implementation.
type DefaultExecutor struct{}

// Output runs the command with context and returns standard output.
func (e *DefaultExecutor) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// LookPath searches for an executable in directories named by the PATH environment variable.
func (e *DefaultExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}
