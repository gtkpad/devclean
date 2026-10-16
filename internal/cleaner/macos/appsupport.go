package macos

import (
	"context"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

// Allowed regenerable leaf subdirectories under ~/Library/Application Support/* (MAC-03, D-07).
var allowedAppSupportLeaves = map[string]bool{
	"cache":              true,
	"caches":             true,
	"logs":               true,
	"crashpad/completed": true,
}

// scanApplicationSupport scans strictly regenerable leaf directories in ~/Library/Application Support
// and isolates the dedicated Spotify PersistentCache target with active process protection (MAC-03, D-07).
func scanApplicationSupport(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	// Stub for TDD RED phase
	return nil, nil
}
