package fs

import (
	"context"
	"time"

	"github.com/gtkpad/devclean/internal/platform"
)

const networkMountTimeout = 2 * time.Second

// IsNetworkMount checks if the given path is on a network mount.
// It first delegates to the platform resolver. If the resolver says no,
// it performs a stat with a timeout as a fallback — if the stat times out,
// the path is treated as a network mount.
func IsNetworkMount(ctx context.Context, resolver platform.Resolver, path string) bool {
	if resolver.IsNetworkMount(path) {
		return true
	}
	return isSlowPath(ctx, path)
}

// isSlowPath attempts to stat the path with a timeout.
// If the operation doesn't complete within networkMountTimeout, returns true.
func isSlowPath(ctx context.Context, path string) bool {
	ctx, cancel := context.WithTimeout(ctx, networkMountTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		// Any filesystem operation — if it hangs, it's likely a network mount
		_ = statPath(path)
		close(done)
	}()

	select {
	case <-done:
		return false
	case <-ctx.Done():
		return true
	}
}
