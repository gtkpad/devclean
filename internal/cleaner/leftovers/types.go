package leftovers

// LaunchAgentPlist represents the launchd job configuration parsed from a .plist file.
type LaunchAgentPlist struct {
	Label            string   `plist:"Label"`
	Program          string   `plist:"Program"`
	ProgramArguments []string `plist:"ProgramArguments"`
	Disabled         bool     `plist:"Disabled"`
}

// AppBundleInfoPlist represents metadata extracted from an application bundle's Info.plist.
type AppBundleInfoPlist struct {
	CFBundleIdentifier string `plist:"CFBundleIdentifier"`
	CFBundleName       string `plist:"CFBundleName"`
	CFBundleExecutable string `plist:"CFBundleExecutable"`
}
