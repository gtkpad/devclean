package ai

import (
	"context"
	"errors"
	"testing"
)

func TestProcessGuard_CheckProcessStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("nil executor fails closed", func(t *testing.T) {
		running, uncertain := checkProcessStatus(ctx, nil, "Code")
		if running || !uncertain {
			t.Errorf("expected running=false, uncertain=true for nil executor, got running=%v, uncertain=%v", running, uncertain)
		}
	})

	t.Run("running process detected", func(t *testing.T) {
		mockExec := newMockExecutor()
		mockExec.runningProcesses["Code"] = true

		running, uncertain := checkProcessStatus(ctx, mockExec, "Code")
		if !running || uncertain {
			t.Errorf("expected running=true, uncertain=false, got running=%v, uncertain=%v", running, uncertain)
		}
	})

	t.Run("stopped process cleanly detected with exit code 1", func(t *testing.T) {
		mockExec := newMockExecutor()

		running, uncertain := checkProcessStatus(ctx, mockExec, "Code")
		if running || uncertain {
			t.Errorf("expected running=false, uncertain=false, got running=%v, uncertain=%v", running, uncertain)
		}
	})

	t.Run("uncertain process fails closed on unexpected error", func(t *testing.T) {
		mockExec := newMockExecutor()
		mockExec.uncertainProcesses["Code"] = true

		running, uncertain := checkProcessStatus(ctx, mockExec, "Code")
		if running || !uncertain {
			t.Errorf("expected running=false, uncertain=true, got running=%v, uncertain=%v", running, uncertain)
		}
	})

	t.Run("secondary process name detected when primary stopped", func(t *testing.T) {
		mockExec := newMockExecutor()
		mockExec.runningProcesses["code"] = true // secondary name running

		running, uncertain := checkProcessStatus(ctx, mockExec, "Code", "code")
		if !running || uncertain {
			t.Errorf("expected running=true, uncertain=false, got running=%v, uncertain=%v", running, uncertain)
		}
	})

	t.Run("general executor error fails closed", func(t *testing.T) {
		mockExec := &mockExecutor{err: errors.New("exec binary not found")}

		running, uncertain := checkProcessStatus(ctx, mockExec, "Code")
		if running || !uncertain {
			t.Errorf("expected running=false, uncertain=true, got running=%v, uncertain=%v", running, uncertain)
		}
	})
}
