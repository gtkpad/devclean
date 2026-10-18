//go:build acceptance

package acceptance

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// HostDiagnostics encapsulates environmental security, filesystem, and OS attributes
// probed on the live host without CGO (VAL-03, D-03).
type HostDiagnostics struct {
	OSVersion           string   `json:"os_version"`
	Architecture        string   `json:"architecture"`
	APFSSupported       bool     `json:"apfs_supported"`
	FilesystemType      string   `json:"filesystem_type"`
	FullDiskAccess      bool     `json:"full_disk_access"`
	FDANote             string   `json:"fda_note"`
	ActiveProtectedApps []string `json:"active_protected_apps"`
}

// ProbeHost inspects the runtime environment in pure Go:
// 1. macOS kernel product version via sysctl kern.osproductversion
// 2. Hardware architecture via runtime.GOARCH
// 3. APFS filesystem capabilities via statfs
// 4. TCC / Full Disk Access permission state by inspecting ~/Library/Safari
// 5. Active protected processes via Process Guard / pgrep check
func ProbeHost(t *testing.T) HostDiagnostics {
	t.Helper()
	homeDir, _ := os.UserHomeDir()

	osVer := probeOSVersion()
	apfs, fsType := probeAPFS(homeDir)
	fda, fdaNote := probeFullDiskAccess(homeDir)
	activeApps := checkActiveProtectedApps()

	return HostDiagnostics{
		OSVersion:           osVer,
		Architecture:        runtime.GOARCH,
		APFSSupported:       apfs,
		FilesystemType:      fsType,
		FullDiskAccess:      fda,
		FDANote:             fdaNote,
		ActiveProtectedApps: activeApps,
	}
}

// probeFullDiskAccess checks whether DevClean holds Full Disk Access permissions
// by attempting to read ~/Library/Safari without prompting user dialogs.
func probeFullDiskAccess(homeDir string) (bool, string) {
	if homeDir == "" {
		return false, "empty home directory"
	}
	safariPath := filepath.Join(homeDir, "Library", "Safari")
	f, err := os.Open(safariPath)
	if err != nil {
		if errors.Is(err, syscall.EPERM) || os.IsPermission(err) {
			return false, "TCC restricted (EPERM / operation not permitted)"
		}
		return false, fmt.Sprintf("inaccessible: %v", err)
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err != nil && (errors.Is(err, syscall.EPERM) || os.IsPermission(err)) {
		return false, "TCC restricted on readdirnames"
	}
	return true, "Full Disk Access granted"
}

// checkActiveProtectedApps queries currently running processes for known applications
// that must be guarded against live cache deletion.
func checkActiveProtectedApps() []string {
	var active []string
	knownApps := []string{"Google Chrome", "Xcode", "OrbStack", "Spotify", "Simulator"}
	for _, app := range knownApps {
		cmd := exec.Command("pgrep", "-x", app)
		if err := cmd.Run(); err == nil {
			active = append(active, app)
		}
	}
	return active
}
