//go:build acceptance

package acceptance

import (
	"runtime"
	"testing"
)

func TestHostDiagnostics(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Acceptance suite requires a live macOS host (runtime.GOOS == 'darwin')")
	}

	diag := ProbeHost(t)
	t.Logf("Host Diagnostics: Darwin %s (%s), APFS=%v (type=%s), FDA=%v (%s), ActiveProtectedApps=%v",
		diag.OSVersion, diag.Architecture, diag.APFSSupported, diag.FilesystemType,
		diag.FullDiskAccess, diag.FDANote, diag.ActiveProtectedApps)

	if diag.OSVersion == "" || diag.OSVersion == "unknown" {
		t.Errorf("expected valid non-empty OS version, got %q", diag.OSVersion)
	}
	if diag.Architecture == "" {
		t.Errorf("expected non-empty Architecture")
	}
	if !diag.APFSSupported {
		t.Errorf("expected APFSSupported=true on live modern macOS host, got false (type: %s)", diag.FilesystemType)
	}
}
