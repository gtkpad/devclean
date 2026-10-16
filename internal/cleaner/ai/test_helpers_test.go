package ai

import (
	"context"
	"errors"
	"fmt"
)

type mockExitError struct {
	exitCode int
}

func (e *mockExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.exitCode)
}

func (e *mockExitError) ExitCode() int {
	return e.exitCode
}

type mockExecutor struct {
	runningProcesses   map[string]bool
	uncertainProcesses map[string]bool
	err                error
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		runningProcesses:   make(map[string]bool),
		uncertainProcesses: make(map[string]bool),
	}
}

func (m *mockExecutor) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	if name == "pgrep" && len(args) == 2 && args[0] == "-x" {
		proc := args[1]
		if m.uncertainProcesses != nil && m.uncertainProcesses[proc] {
			return nil, errors.New("pgrep fatal internal error")
		}
		if m.runningProcesses != nil && m.runningProcesses[proc] {
			return []byte("12345\n"), nil
		}
		return nil, &mockExitError{exitCode: 1}
	}
	return nil, errors.New("command not supported in mockExecutor")
}

func (m *mockExecutor) LookPath(file string) (string, error) {
	return "/usr/bin/" + file, nil
}
