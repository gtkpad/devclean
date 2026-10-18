package leftovers

import (
	"fmt"
	"os"

	"howett.net/plist"
)

// parseLaunchAgentFile reads and decodes a LaunchAgent plist file (either XML or binary bplist00).
func parseLaunchAgentFile(path string) (*LaunchAgentPlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading launchagent plist %s: %w", path, err)
	}

	var p LaunchAgentPlist
	if _, err := plist.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decoding launchagent plist %s: %w", path, err)
	}

	return &p, nil
}

// parseAppBundleInfoFile reads and decodes an application bundle's Info.plist file.
func parseAppBundleInfoFile(path string) (*AppBundleInfoPlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading app bundle plist %s: %w", path, err)
	}

	var info AppBundleInfoPlist
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("decoding app bundle plist %s: %w", path, err)
	}

	return &info, nil
}
