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
			Rust:   EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue", ExcludePaths: []string{}},
			Java:   EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue", ExcludePaths: []string{}},
		},
		Safety: SafetyConfig{
			TrashEnabled:  false,
			RetentionDays: 7,
			MaxSizeMB:     5120,
			TrashDir:      "",
		},
		TUI: TUIConfig{
			MassSelectThresholdMB: 100,
		},
		Hooks: HooksConfig{
			Node:   HookConfig{Timeout: "30s"},
			Go:     HookConfig{Timeout: "30s"},
			Docker: HookConfig{Timeout: "30s"},
			DotNet: HookConfig{Timeout: "30s"},
			Python: HookConfig{Timeout: "30s"},
			Rust:   HookConfig{Timeout: "30s"},
			Java:   HookConfig{Timeout: "30s"},
		},
		History: HistoryConfig{
			RetentionDays: 90,
		},
		Intelligence: IntelligenceConfig{
			Threshold:  0.7,
			MaxAgeDays: 180,
			RefSizeMB:  5120,
			Weights: WeightsConfig{
				Age:         0.35,
				Size:        0.30,
				GitActivity: 0.20,
				Risk:        0.15,
			},
		},
		Watch: WatchConfig{
			Interval:    "6h",
			ThresholdMB: 2048,
		},
	}
}
