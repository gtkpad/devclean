package config

// Defaults returns the compiled default configuration.
// Used when no config file exists or for fields not specified in YAML.
func Defaults() Config {
	return Config{
		ScanPaths:         []string{},
		ProtectedPatterns: []string{},
		Ecosystems: EcosystemsConfig{
			Docker: EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue", ExcludePaths: []string{}},
			Node:   EcosystemConfig{Enabled: true, MaxAgeDays: 14, FailMode: "continue", ExcludePaths: []string{}},
			Go:     EcosystemConfig{Enabled: true, MaxAgeDays: 60, FailMode: "continue", ExcludePaths: []string{}},
			DotNet: EcosystemConfig{Enabled: true, MaxAgeDays: 45, FailMode: "continue", ExcludePaths: []string{}},
			Python: EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue", ExcludePaths: []string{}},
		},
		Safety: SafetyConfig{
			TrashEnabled:  false,
			RetentionDays: 7,
			MaxSizeMB:     5120,
			TrashDir:      "",
		},
	}
}
