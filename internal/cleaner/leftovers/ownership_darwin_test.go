//go:build darwin

package leftovers

import (
	"os"
	"syscall"
	"testing"
	"time"
)

type mockStatFileInfo struct {
	name string
	uid  uint32
}

func (m mockStatFileInfo) Name() string       { return m.name }
func (m mockStatFileInfo) Size() int64        { return 0 }
func (m mockStatFileInfo) Mode() os.FileMode  { return 0644 }
func (m mockStatFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockStatFileInfo) IsDir() bool        { return false }
func (m mockStatFileInfo) Sys() any {
	return &syscall.Stat_t{Uid: m.uid}
}

func TestLeftovers_LaunchAgents_LocationAwareOwnership(t *testing.T) {
	currentUID := uint32(os.Getuid())
	mismatchedUID := currentUID + 9999

	// 1. Current user is permitted in both system and user directories
	userFile := mockStatFileInfo{name: "user.plist", uid: currentUID}
	if !isAllowedOwnership(userFile, false) {
		t.Errorf("expected isAllowedOwnership=true for current UID in user dir")
	}
	if !isAllowedOwnership(userFile, true) {
		t.Errorf("expected isAllowedOwnership=true for current UID in system dir")
	}

	// 2. Root (UID 0) is permitted in system directory (/Library/LaunchAgents) (D-09)
	rootFile := mockStatFileInfo{name: "root.plist", uid: 0}
	if !isAllowedOwnership(rootFile, true) {
		t.Errorf("expected isAllowedOwnership=true for root UID in system dir (/Library/LaunchAgents)")
	}
	if isAllowedOwnership(rootFile, false) {
		t.Errorf("expected isAllowedOwnership=false for root UID in user dir (~/Library/LaunchAgents)")
	}

	// 3. Foreign non-root UID is rejected in both
	foreignFile := mockStatFileInfo{name: "foreign.plist", uid: mismatchedUID}
	if isAllowedOwnership(foreignFile, false) {
		t.Errorf("expected isAllowedOwnership=false for mismatched UID in user dir")
	}
	if isAllowedOwnership(foreignFile, true) {
		t.Errorf("expected isAllowedOwnership=false for mismatched UID in system dir")
	}

	// 4. nil FileInfo
	if isAllowedOwnership(nil, true) || isAllowedOwnership(nil, false) {
		t.Errorf("expected isAllowedOwnership=false for nil info")
	}
}
