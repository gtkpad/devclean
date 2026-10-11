package scheduler

import (
	"context"
	"os/exec"
	"strings"
)

// CommandExecutor abstracts operating system command execution.
type CommandExecutor interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)
	CombinedOutputWithStdin(ctx context.Context, stdin string, name string, args ...string) ([]byte, error)
	LookPath(file string) (string, error)
}

// osExecutor is the default implementation for production use.
type osExecutor struct{}

func (e *osExecutor) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (e *osExecutor) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (e *osExecutor) CombinedOutputWithStdin(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.CombinedOutput()
}

func (e *osExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// MockCommandExecutor records calls and returns controlled outputs/errors for testing.
type MockCommandExecutor struct {
	Outputs map[string][]byte
	Errors  map[string]error
	Calls   [][]string
}

// NewMockExecutor constructs an empty MockCommandExecutor.
func NewMockExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		Outputs: make(map[string][]byte),
		Errors:  make(map[string]error),
		Calls:   make([][]string, 0),
	}
}

func (m *MockCommandExecutor) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return m.CombinedOutput(ctx, name, args...)
}

func (m *MockCommandExecutor) CombinedOutput(_ context.Context, name string, args ...string) ([]byte, error) {
	cmdKey := name + " " + strings.Join(args, " ")
	m.Calls = append(m.Calls, append([]string{name}, args...))
	if err, ok := m.Errors[cmdKey]; ok {
		return m.Outputs[cmdKey], err
	}
	return m.Outputs[cmdKey], nil
}

func (m *MockCommandExecutor) CombinedOutputWithStdin(_ context.Context, stdin string, name string, args ...string) ([]byte, error) {
	cmdKey := name + " " + strings.Join(args, " ")
	m.Calls = append(m.Calls, append([]string{name}, args...))
	if err, ok := m.Errors[cmdKey]; ok {
		return m.Outputs[cmdKey], err
	}
	return m.Outputs[cmdKey], nil
}

func (m *MockCommandExecutor) LookPath(file string) (string, error) {
	if err, ok := m.Errors["lookpath:"+file]; ok {
		return "", err
	}
	if out, ok := m.Outputs["lookpath:"+file]; ok {
		return string(out), nil
	}
	return "/usr/local/bin/" + file, nil
}
