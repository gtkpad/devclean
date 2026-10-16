package macos

import (
	"context"

	"github.com/gtkpad/devclean/internal/domain"
)

// scanTrash inventories $HOME/.Trash as ReviewOnly by default (MAC-01, D-01, D-02).
func scanTrash(homeDir string) domain.Target {
	return domain.Target{}
}

// cleanTrash purges items inside $HOME/.Trash directly without internal quarantine (MAC-01, D-03, D-04).
func cleanTrash(ctx context.Context, trashDir string) error {
	return nil
}
