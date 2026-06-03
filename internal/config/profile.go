package config

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/spf13/viper"
)

var validProfileName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// LoadProfile reads a profile YAML file from the profiles directory.
// Profile path: {configDir}/profiles/{name}.yaml
func LoadProfile(configDir, name string) (*Config, error) {
	if !validProfileName.MatchString(name) {
		return nil, fmt.Errorf("invalid profile name %q: must match [a-zA-Z0-9_-]", name)
	}

	profilePath := filepath.Join(configDir, "profiles", name+".yaml")

	v := viper.New()
	v.SetConfigFile(profilePath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("loading profile %q: %w", name, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing profile %q: %w", name, err)
	}
	return &cfg, nil
}

// MergeProfile merges a profile config on top of a root config.
// Array fields (ScanPaths, ExcludePaths, ProtectedPatterns): APPEND.
// Scalar and object fields: REPLACE (profile value wins if non-zero).
func MergeProfile(root *Config, profile *Config) *Config {
	merged := *root

	if len(profile.ScanPaths) > 0 {
		merged.ScanPaths = dedup(append(merged.ScanPaths, profile.ScanPaths...))
	}
	if len(profile.ExcludePaths) > 0 {
		merged.ExcludePaths = dedup(append(merged.ExcludePaths, profile.ExcludePaths...))
	}
	if len(profile.ProtectedPatterns) > 0 {
		merged.ProtectedPatterns = dedup(append(merged.ProtectedPatterns, profile.ProtectedPatterns...))
	}

	mergeEcosystem(&merged.Ecosystems.Node, &profile.Ecosystems.Node)
	mergeEcosystem(&merged.Ecosystems.Go, &profile.Ecosystems.Go)
	mergeEcosystem(&merged.Ecosystems.Docker, &profile.Ecosystems.Docker)
	mergeEcosystem(&merged.Ecosystems.DotNet, &profile.Ecosystems.DotNet)
	mergeEcosystem(&merged.Ecosystems.Python, &profile.Ecosystems.Python)
	mergeEcosystem(&merged.Ecosystems.Rust, &profile.Ecosystems.Rust)
	mergeEcosystem(&merged.Ecosystems.Java, &profile.Ecosystems.Java)

	if profile.Safety != (SafetyConfig{}) {
		merged.Safety = profile.Safety
	}
	if profile.TUI != (TUIConfig{}) {
		merged.TUI = profile.TUI
	}
	if isHooksSet(&profile.Hooks) {
		merged.Hooks = profile.Hooks
	}
	if profile.History.RetentionDays != 0 {
		merged.History = profile.History
	}

	return &merged
}

func isHooksSet(h *HooksConfig) bool {
	return h.Node.PreDelete != "" || h.Node.PostDelete != "" || h.Node.Timeout != "" ||
		h.Go.PreDelete != "" || h.Go.PostDelete != "" || h.Go.Timeout != "" ||
		h.Docker.PreDelete != "" || h.Docker.PostDelete != "" || h.Docker.Timeout != "" ||
		h.DotNet.PreDelete != "" || h.DotNet.PostDelete != "" || h.DotNet.Timeout != "" ||
		h.Python.PreDelete != "" || h.Python.PostDelete != "" || h.Python.Timeout != "" ||
		h.Rust.PreDelete != "" || h.Rust.PostDelete != "" || h.Rust.Timeout != "" ||
		h.Java.PreDelete != "" || h.Java.PostDelete != "" || h.Java.Timeout != ""
}

func mergeEcosystem(dst *EcosystemConfig, src *EcosystemConfig) {
	if src.MaxAgeDays != 0 {
		dst.MaxAgeDays = src.MaxAgeDays
	}
	if src.FailMode != "" {
		dst.FailMode = src.FailMode
	}
	if len(src.ExcludePaths) > 0 {
		dst.ExcludePaths = append(dst.ExcludePaths, src.ExcludePaths...)
	}
	// Enabled is a bool — we check if it's explicitly set by checking if the whole src is non-zero
	// Since we can't distinguish unset from false for bools in this pattern,
	// ecosystem-level override only triggers if any other field is set
	if src.MaxAgeDays != 0 || src.FailMode != "" || len(src.ExcludePaths) > 0 {
		dst.Enabled = src.Enabled
	}
}

func dedup(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}
