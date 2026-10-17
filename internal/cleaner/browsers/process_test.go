package browsers

import (
	"context"
	"errors"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
)

func TestBrowsers_ProcessGuard_Running(t *testing.T) {
	ctx := context.Background()
	chromeDef, ok := FindBrowserDef("chrome")
	if !ok {
		t.Fatalf("chrome definition not found")
	}

	mockExec := cleaner.NewMockExecutor()
	mockExec.Outputs["pgrep -x Google Chrome"] = []byte("54321\n")

	running, uncertain := CheckBrowserProcessStatus(ctx, mockExec, chromeDef, "darwin")
	if !running {
		t.Errorf("expected running=true for active browser, got false")
	}
	if uncertain {
		t.Errorf("expected uncertain=false for cleanly detected active browser, got true")
	}

	// Verify exact tokenized call without shell interpreter
	if len(mockExec.Calls) == 0 {
		t.Fatalf("expected at least 1 command call, got 0")
	}
	firstCall := mockExec.Calls[0]
	if len(firstCall) != 3 || firstCall[0] != "pgrep" || firstCall[1] != "-x" || firstCall[2] != "Google Chrome" {
		t.Errorf("unexpected command call format: %v", firstCall)
	}
}

func TestBrowsers_ProcessGuard_Stopped(t *testing.T) {
	ctx := context.Background()
	chromeDef, ok := FindBrowserDef("chrome")
	if !ok {
		t.Fatalf("chrome definition not found")
	}

	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["pgrep -x Google Chrome"] = &cleaner.MockExitError{Code: 1}
	mockExec.Errors["pgrep -x Google Chrome Helper"] = &cleaner.MockExitError{Code: 1}

	running, uncertain := CheckBrowserProcessStatus(ctx, mockExec, chromeDef, "darwin")
	if running {
		t.Errorf("expected running=false for stopped browser, got true")
	}
	if uncertain {
		t.Errorf("expected uncertain=false for cleanly stopped browser with exit 1, got true")
	}
}

func TestBrowsers_ProcessGuard_FailClosed(t *testing.T) {
	ctx := context.Background()
	chromeDef, ok := FindBrowserDef("chrome")
	if !ok {
		t.Fatalf("chrome definition not found")
	}

	t.Run("pgrep exits with code 2", func(t *testing.T) {
		mockExec := cleaner.NewMockExecutor()
		mockExec.Errors["pgrep -x Google Chrome"] = &cleaner.MockExitError{Code: 2, Msg: "syntax error"}

		running, uncertain := CheckBrowserProcessStatus(ctx, mockExec, chromeDef, "darwin")
		if running {
			t.Errorf("expected running=false on error, got true")
		}
		if !uncertain {
			t.Errorf("FAIL-CLOSED VIOLATION: expected uncertain=true on unexpected exit code 2, got false")
		}
	})

	t.Run("pgrep binary missing or execution error", func(t *testing.T) {
		mockExec := cleaner.NewMockExecutor()
		mockExec.Errors["pgrep -x Google Chrome"] = errors.New("exec: pgrep not found in $PATH")

		running, uncertain := CheckBrowserProcessStatus(ctx, mockExec, chromeDef, "darwin")
		if running {
			t.Errorf("expected running=false on missing tool, got true")
		}
		if !uncertain {
			t.Errorf("FAIL-CLOSED VIOLATION: expected uncertain=true on missing pgrep, got false")
		}
	})
}

func TestBrowsers_ProcessGuard_NilExecutor(t *testing.T) {
	ctx := context.Background()
	chromeDef, _ := FindBrowserDef("chrome")

	running, uncertain := CheckBrowserProcessStatus(ctx, nil, chromeDef, "darwin")
	if running {
		t.Errorf("expected running=false for nil executor, got true")
	}
	if !uncertain {
		t.Errorf("FAIL-CLOSED VIOLATION: expected uncertain=true for nil executor, got false")
	}
}

func TestBrowsers_ProcessGuard_HelperDetection(t *testing.T) {
	ctx := context.Background()
	chromeDef, ok := FindBrowserDef("chrome")
	if !ok {
		t.Fatalf("chrome definition not found")
	}

	// Main binary is stopped (exit 1), but helper process is still running (exit 0)
	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["pgrep -x Google Chrome"] = &cleaner.MockExitError{Code: 1}
	mockExec.Outputs["pgrep -x Google Chrome Helper"] = []byte("99999\n")

	running, uncertain := CheckBrowserProcessStatus(ctx, mockExec, chromeDef, "darwin")
	if !running {
		t.Errorf("expected running=true when helper process is running, got false")
	}
	if uncertain {
		t.Errorf("expected uncertain=false, got true")
	}
}

func TestBrowsers_ProcessGuard_PerBrowserIsolation(t *testing.T) {
	ctx := context.Background()
	chromeDef, _ := FindBrowserDef("chrome")
	firefoxDef, _ := FindBrowserDef("firefox")

	mockExec := cleaner.NewMockExecutor()
	// Chrome is running
	mockExec.Outputs["pgrep -x Google Chrome"] = []byte("11111\n")
	// Firefox is stopped
	mockExec.Errors["pgrep -x firefox"] = &cleaner.MockExitError{Code: 1}
	mockExec.Errors["pgrep -x firefox-bin"] = &cleaner.MockExitError{Code: 1}

	// Verify Chrome is running
	chromeRunning, chromeUncertain := CheckBrowserProcessStatus(ctx, mockExec, chromeDef, "darwin")
	if !chromeRunning || chromeUncertain {
		t.Errorf("expected Chrome running=true, uncertain=false, got running=%v, uncertain=%v", chromeRunning, chromeUncertain)
	}

	// Verify Firefox is stopped (not blocked by Chrome!)
	firefoxRunning, firefoxUncertain := CheckBrowserProcessStatus(ctx, mockExec, firefoxDef, "darwin")
	if firefoxRunning || firefoxUncertain {
		t.Errorf("ISOLATION FAILURE: expected Firefox running=false, uncertain=false, got running=%v, uncertain=%v", firefoxRunning, firefoxUncertain)
	}
}
