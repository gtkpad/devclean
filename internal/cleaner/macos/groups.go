package macos

import (
	"context"

	"github.com/gtkpad/devclean/internal/domain"
)

// Allowed subfolder names for group containers (D-08).
var allowedGroupContainerSubdirs = []string{
	"Library/Caches",
	"Library/Logs",
	"Caches",
	"Logs",
}

// scanGroupContainers scans allowlisted cache and log subfolders in ~/Library/Group Containers (D-08).
func scanGroupContainers(ctx context.Context, homeDir string) ([]domain.Target, error) {
	// Stub for TDD RED phase
	return nil, nil
}
