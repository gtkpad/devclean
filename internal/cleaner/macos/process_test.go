package macos

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
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

type mockProcessExecutor struct {
	runningProcesses   map[string]bool
	uncertainProcesses map[string]int // maps process name to unexpected exit code (e.g. 2)
	err                error
	invokedCommands    [][]string
}

func newMockProcessExecutor() *mockProcessExecutor {
	return &mockProcessExecutor{
		runningProcesses:   make(map[string]bool),
		uncertainProcesses: make(map[string]int),
		invokedCommands:    make([][]string, 0),
	}
}

func (m *mockProcessExecutor) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := append([]string{name}, args...)
	m.invokedCommands = append(m.invokedCommands, cmd)

	if m.err != nil {
		return nil, m.err
	}

	if name == "pgrep" && len(args) == 2 && args[0] == "-x" {
		proc := args[1]
		if exitCode, ok := m.uncertainProcesses[proc]; ok {
			return nil, &mockExitError{exitCode: exitCode}
		}
		if m.runningProcesses[proc] {
			return []byte("12345\n"), nil
		}
		return nil, &mockExitError{exitCode: 1}
	}
	return nil, errors.New("unsupported command")
}

func (m *mockProcessExecutor) LookPath(file string) (string, error) {
	return "/usr/bin/" + file, nil
}

// Ensure mockProcessExecutor implements cleaner.CommandExecutor
var _ cleaner.CommandExecutor = (*mockProcessExecutor)(nil)

func TestProcessGuard_Running(t *testing.T) {
	ctx := context.Background()
	mock := newMockProcessExecutor()
	mock.runningProcesses["Spotify"] = true

	running, uncertain := CheckProcessStatus(ctx, mock, "Spotify")
	if !running || uncertain {
		t.Errorf("expected running=true, uncertain=false, got running=%v, uncertain=%v", running, uncertain)
	}
}

func TestProcessGuard_Stopped(t *testing.T) {
	ctx := context.Background()
	mock := newMockProcessExecutor()

	running, uncertain := CheckProcessStatus(ctx, mock, "Spotify")
	if running || uncertain {
		t.Errorf("expected running=false, uncertain=false, got running=%v, uncertain=%v", running, uncertain)
	}
}

func TestProcessGuard_FailClosedOnUnexpectedExit(t *testing.T) {
	ctx := context.Background()
	mock := newMockProcessExecutor()
	mock.uncertainProcesses["Spotify"] = 2 // exit code 2 (syntax/system error)

	running, uncertain := CheckProcessStatus(ctx, mock, "Spotify")
	if running || !uncertain {
		t.Errorf("expected running=false, uncertain=true for exit code 2, got running=%v, uncertain=%v", running, uncertain)
	}

	// Also test generic non-exit error
	mockErr := newMockProcessExecutor()
	mockErr.err = errors.New("cannot execute pgrep: binary missing")
	running, uncertain = CheckProcessStatus(ctx, mockErr, "Spotify")
	if running || !uncertain {
		t.Errorf("expected running=false, uncertain=true for binary error, got running=%v, uncertain=%v", running, uncertain)
	}
}

func TestProcessGuard_FailClosedOnNilExecutor(t *testing.T) {
	ctx := context.Background()

	running, uncertain := CheckProcessStatus(ctx, nil, "Spotify")
	if running || !uncertain {
		t.Errorf("expected running=false, uncertain=true for nil executor, got running=%v, uncertain=%v", running, uncertain)
	}

	running, uncertain = IsBundleProcessRunning(ctx, nil, "com.spotify.client")
	if running || !uncertain {
		t.Errorf("expected running=false, uncertain=true for nil executor on bundle check, got running=%v, uncertain=%v", running, uncertain)
	}
}

func TestProcessGuard_KnownBundleMappings(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		bundleID         string
		runningProc      string
		expectedCommands [][]string
	}{
		{"com.spotify.client", "Spotify", [][]string{{"pgrep", "-x", "Spotify"}}},
		{"com.apple.mediaanalysisd", "mediaanalysisd", [][]string{{"pgrep", "-x", "mediaanalysisd"}}},
		{"com.apple.geod", "geod", [][]string{{"pgrep", "-x", "geod"}}},
		{"com.apple.helpd", "helpd", [][]string{{"pgrep", "-x", "helpd"}}},
		{"com.apple.WallpaperAgent", "wallpaper", [][]string{{"pgrep", "-x", "WallpaperAgent"}, {"pgrep", "-x", "wallpaper"}}},
		{"com.apple.parsecd", "parsecd", [][]string{{"pgrep", "-x", "parsecd"}}},
		{"com.apple.AppleMediaServices", "appstoreagent", [][]string{{"pgrep", "-x", "amsaccountsd"}, {"pgrep", "-x", "appstoreagent"}}},
		{"com.apple.coreservices.useractivityd", "useractivityd", [][]string{{"pgrep", "-x", "useractivityd"}}},
		{"com.google.Chrome.code_sign_clone", "Google Chrome", [][]string{{"pgrep", "-x", "Google Chrome"}}},
		{"org.chromium.Chromium.code_sign_clone", "Chromium", [][]string{{"pgrep", "-x", "Chromium"}}},
	}

	for _, tc := range cases {
		t.Run(tc.bundleID, func(t *testing.T) {
			mock := newMockProcessExecutor()
			mock.runningProcesses[tc.runningProc] = true

			running, uncertain := IsBundleProcessRunning(ctx, mock, tc.bundleID)
			if !running || uncertain {
				t.Fatalf("expected bundle %s with active proc %s to return running=true, uncertain=false, got running=%v, uncertain=%v",
					tc.bundleID, tc.runningProc, running, uncertain)
			}

			// Verify tokenized arguments passed without shell wrapper
			if len(mock.invokedCommands) == 0 {
				t.Fatalf("expected commands to be executed for bundle %s", tc.bundleID)
			}
			lastCmd := mock.invokedCommands[len(mock.invokedCommands)-1]
			if lastCmd[0] != "pgrep" || lastCmd[1] != "-x" || lastCmd[2] != tc.runningProc {
				t.Errorf("expected command [pgrep -x %s], got %v", tc.runningProc, lastCmd)
			}
		})
	}
}

func TestProcessGuard_FallbackSanitizedName(t *testing.T) {
	ctx := context.Background()

	// An unmapped apple bundle like "com.apple.Safari" should trim "com.apple." to "Safari"
	mock := newMockProcessExecutor()
	mock.runningProcesses["Safari"] = true

	running, uncertain := IsBundleProcessRunning(ctx, mock, "com.apple.Safari")
	if !running || uncertain {
		t.Errorf("expected unmapped bundle com.apple.Safari to query Safari and return running=true, got running=%v, uncertain=%v", running, uncertain)
	}

	// An unmapped third party bundle like "com.slack.Slack" should trim "com." to "slack.Slack"
	mock2 := newMockProcessExecutor()
	mock2.runningProcesses["slack.Slack"] = true

	running2, uncertain2 := IsBundleProcessRunning(ctx, mock2, "com.slack.Slack")
	if !running2 || uncertain2 {
		t.Errorf("expected unmapped bundle com.slack.Slack to query slack.Slack and return running=true, got running=%v, uncertain=%v", running2, uncertain2)
	}
}
