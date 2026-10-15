package cleaner_test

import (
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
)

func TestProviders_NoShellInjection(t *testing.T) {
	// Audit that all virtual and ecosystem cleaners execute subprocesses
	// strictly via exec.CommandContext direct token slices, never through
	// sh -c, /bin/sh, or bash -c interpolation (POL-04, D-16).
	cleanerDir := "."
	violations, err := cleaner.AuditNoShellInjection(cleanerDir)
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("shell injection risk detected: %s", v)
		}
	}
}
