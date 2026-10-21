//go:build !darwin

package leftovers

import (
	"testing"
)

func TestLeftovers_LaunchAgents_LocationAwareOwnership_Other(t *testing.T) {
	if !isAllowedOwnership(nil, true) {
		t.Errorf("expected isAllowedOwnership=true on non-darwin")
	}
	if !isAllowedOwnership(nil, false) {
		t.Errorf("expected isAllowedOwnership=true on non-darwin")
	}
}
