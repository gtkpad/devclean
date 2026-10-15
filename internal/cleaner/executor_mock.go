package cleaner

import (
	"context"
	"fmt"
	"strings"
)

// MockExitError implements interface{ ExitCode() int } and error.
type MockExitError struct {
	Code int
	Msg  string
}

func (e *MockExitError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *MockExitError) ExitCode() int {
	return e.Code
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

// Output records the call and returns the configured output or error.
func (m *MockCommandExecutor) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	cmdKey := name + " " + strings.Join(args, " ")
	m.Calls = append(m.Calls, append([]string{name}, args...))
	if err, ok := m.Errors[cmdKey]; ok {
		return m.Outputs[cmdKey], err
	}
	return m.Outputs[cmdKey], nil
}

// LookPath returns the configured path or error, falling back to /usr/local/bin/<file>.
func (m *MockCommandExecutor) LookPath(file string) (string, error) {
	if err, ok := m.Errors["lookpath:"+file]; ok {
		return "", err
	}
	if out, ok := m.Outputs["lookpath:"+file]; ok {
		return string(out), nil
	}
	return "/usr/local/bin/" + file, nil
}
