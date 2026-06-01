package fs

import (
	"context"
	"testing"

	"github.com/gabrielvp/devclean/internal/platform"
)

// mountTestResolver implements platform.Resolver for tests.
type mountTestResolver struct {
	networkPaths map[string]bool
}

func (r *mountTestResolver) Resolve() platform.Paths { return platform.Paths{} }
func (r *mountTestResolver) IsNetworkMount(path string) bool {
	return r.networkPaths[path]
}

func TestIsNetworkMount_ResolverDetects(t *testing.T) {
	resolver := &mountTestResolver{networkPaths: map[string]bool{"/mnt/nfs": true}}
	ctx := context.Background()

	if !IsNetworkMount(ctx, resolver, "/mnt/nfs") {
		t.Error("expected /mnt/nfs to be detected as network mount")
	}
}

func TestIsNetworkMount_LocalPath(t *testing.T) {
	resolver := &mountTestResolver{networkPaths: map[string]bool{}}
	ctx := context.Background()

	dir := t.TempDir()
	if IsNetworkMount(ctx, resolver, dir) {
		t.Error("expected local temp dir not to be a network mount")
	}
}
