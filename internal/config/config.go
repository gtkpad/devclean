package config

import (
	"fmt"
	"os"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/spf13/viper"
)

type EcosystemConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	MaxAgeDays   int      `mapstructure:"max_age_days"`
	FailMode     string   `mapstructure:"fail_mode"`
	ExcludePaths []string `mapstructure:"exclude_paths"`
}

// ToRules converts an EcosystemConfig to a domain.EcosystemRules.
func (ec EcosystemConfig) ToRules() domain.EcosystemRules {
	fm := ec.FailMode
	if fm == "" {
		fm = "continue"
	}
	return domain.EcosystemRules{
		Enabled:      ec.Enabled,
		MaxAgeDays:   ec.MaxAgeDays,
		FailMode:     fm,
		ExcludePaths: ec.ExcludePaths,
	}
}

type EcosystemsConfig struct {
	Docker EcosystemConfig `mapstructure:"docker"`
	Node   EcosystemConfig `mapstructure:"node"`
	Go     EcosystemConfig `mapstructure:"go"`
	DotNet EcosystemConfig `mapstructure:"dotnet"`
	Python EcosystemConfig `mapstructure:"python"`
	Rust   EcosystemConfig `mapstructure:"rust"`
	Java   EcosystemConfig `mapstructure:"java"`
}

type SafetyConfig struct {
	TrashEnabled  bool   `mapstructure:"trash_enabled"`
	RetentionDays int    `mapstructure:"retention_days"`
	MaxSizeMB     int    `mapstructure:"max_size_mb"`
	TrashDir      string `mapstructure:"trash_dir"`
}

type TUIConfig struct {
	MassSelectThresholdMB int64 `mapstructure:"mass_select_threshold_mb"`
}

type HookConfig struct {
	PreDelete  string   `mapstructure:"pre_delete"`
	PostDelete string   `mapstructure:"post_delete"`
	Timeout    string   `mapstructure:"timeout"`
	Platform   []string `mapstructure:"platform"`
}

type HooksConfig struct {
	Node   HookConfig `mapstructure:"node"`
	Go     HookConfig `mapstructure:"go"`
	Docker HookConfig `mapstructure:"docker"`
	DotNet HookConfig `mapstructure:"dotnet"`
	Python HookConfig `mapstructure:"python"`
	Rust   HookConfig `mapstructure:"rust"`
	Java   HookConfig `mapstructure:"java"`
}

type HistoryConfig struct {
	RetentionDays int `mapstructure:"retention_days"`
}

type IntelligenceConfig struct {
	Threshold  float64       `mapstructure:"threshold"`
	MaxAgeDays int           `mapstructure:"max_age_days"`
	RefSizeMB  int64         `mapstructure:"ref_size_mb"`
	Weights    WeightsConfig `mapstructure:"weights"`
}

type WeightsConfig struct {
	Age         float64 `mapstructure:"age"`
	Size        float64 `mapstructure:"size"`
	GitActivity float64 `mapstructure:"git_activity"`
	Risk        float64 `mapstructure:"risk"`
}

type WatchConfig struct {
	Interval    string `mapstructure:"interval"`
	ThresholdMB int64  `mapstructure:"threshold_mb"`
}

type Config struct {
	ScanPaths         []string           `mapstructure:"scan_paths"`
	ExcludePaths      []string           `mapstructure:"exclude_paths"`
	ProtectedPatterns []string           `mapstructure:"protected_patterns"`
	Ecosystems        EcosystemsConfig   `mapstructure:"ecosystems"`
	Safety            SafetyConfig       `mapstructure:"safety"`
	TUI               TUIConfig          `mapstructure:"tui"`
	Hooks             HooksConfig        `mapstructure:"hooks"`
	History           HistoryConfig      `mapstructure:"history"`
	Intelligence      IntelligenceConfig `mapstructure:"intelligence"`
	Watch             WatchConfig        `mapstructure:"watch"`
}

// Load reads configuration with precedence: defaults < file < flags.
// configPath overrides the default config location (for testing).
func Load(configPath string) (*Config, error) {
	v := viper.New()

	defaults := Defaults()
	v.SetDefault("scan_paths", defaults.ScanPaths)
	v.SetDefault("protected_patterns", defaults.ProtectedPatterns)
	v.SetDefault("ecosystems.docker.enabled", defaults.Ecosystems.Docker.Enabled)
	v.SetDefault("ecosystems.docker.max_age_days", defaults.Ecosystems.Docker.MaxAgeDays)
	v.SetDefault("ecosystems.docker.fail_mode", defaults.Ecosystems.Docker.FailMode)
	v.SetDefault("ecosystems.docker.exclude_paths", defaults.Ecosystems.Docker.ExcludePaths)
	v.SetDefault("ecosystems.node.enabled", defaults.Ecosystems.Node.Enabled)
	v.SetDefault("ecosystems.node.max_age_days", defaults.Ecosystems.Node.MaxAgeDays)
	v.SetDefault("ecosystems.node.fail_mode", defaults.Ecosystems.Node.FailMode)
	v.SetDefault("ecosystems.node.exclude_paths", defaults.Ecosystems.Node.ExcludePaths)
	v.SetDefault("ecosystems.go.enabled", defaults.Ecosystems.Go.Enabled)
	v.SetDefault("ecosystems.go.max_age_days", defaults.Ecosystems.Go.MaxAgeDays)
	v.SetDefault("ecosystems.go.fail_mode", defaults.Ecosystems.Go.FailMode)
	v.SetDefault("ecosystems.go.exclude_paths", defaults.Ecosystems.Go.ExcludePaths)
	v.SetDefault("ecosystems.dotnet.enabled", defaults.Ecosystems.DotNet.Enabled)
	v.SetDefault("ecosystems.dotnet.max_age_days", defaults.Ecosystems.DotNet.MaxAgeDays)
	v.SetDefault("ecosystems.dotnet.fail_mode", defaults.Ecosystems.DotNet.FailMode)
	v.SetDefault("ecosystems.dotnet.exclude_paths", defaults.Ecosystems.DotNet.ExcludePaths)
	v.SetDefault("ecosystems.python.enabled", defaults.Ecosystems.Python.Enabled)
	v.SetDefault("ecosystems.python.max_age_days", defaults.Ecosystems.Python.MaxAgeDays)
	v.SetDefault("ecosystems.python.fail_mode", defaults.Ecosystems.Python.FailMode)
	v.SetDefault("ecosystems.python.exclude_paths", defaults.Ecosystems.Python.ExcludePaths)
	v.SetDefault("ecosystems.rust.enabled", defaults.Ecosystems.Rust.Enabled)
	v.SetDefault("ecosystems.rust.max_age_days", defaults.Ecosystems.Rust.MaxAgeDays)
	v.SetDefault("ecosystems.rust.fail_mode", defaults.Ecosystems.Rust.FailMode)
	v.SetDefault("ecosystems.rust.exclude_paths", defaults.Ecosystems.Rust.ExcludePaths)
	v.SetDefault("ecosystems.java.enabled", defaults.Ecosystems.Java.Enabled)
	v.SetDefault("ecosystems.java.max_age_days", defaults.Ecosystems.Java.MaxAgeDays)
	v.SetDefault("ecosystems.java.fail_mode", defaults.Ecosystems.Java.FailMode)
	v.SetDefault("ecosystems.java.exclude_paths", defaults.Ecosystems.Java.ExcludePaths)
	v.SetDefault("safety.trash_enabled", defaults.Safety.TrashEnabled)
	v.SetDefault("safety.retention_days", defaults.Safety.RetentionDays)
	v.SetDefault("safety.max_size_mb", defaults.Safety.MaxSizeMB)
	v.SetDefault("safety.trash_dir", defaults.Safety.TrashDir)
	v.SetDefault("tui.mass_select_threshold_mb", defaults.TUI.MassSelectThresholdMB)
	v.SetDefault("hooks.node.timeout", defaults.Hooks.Node.Timeout)
	v.SetDefault("hooks.go.timeout", defaults.Hooks.Go.Timeout)
	v.SetDefault("hooks.docker.timeout", defaults.Hooks.Docker.Timeout)
	v.SetDefault("hooks.dotnet.timeout", defaults.Hooks.DotNet.Timeout)
	v.SetDefault("hooks.python.timeout", defaults.Hooks.Python.Timeout)
	v.SetDefault("hooks.rust.timeout", defaults.Hooks.Rust.Timeout)
	v.SetDefault("hooks.java.timeout", defaults.Hooks.Java.Timeout)
	v.SetDefault("history.retention_days", defaults.History.RetentionDays)
	v.SetDefault("intelligence.threshold", defaults.Intelligence.Threshold)
	v.SetDefault("intelligence.max_age_days", defaults.Intelligence.MaxAgeDays)
	v.SetDefault("intelligence.ref_size_mb", defaults.Intelligence.RefSizeMB)
	v.SetDefault("intelligence.weights.age", defaults.Intelligence.Weights.Age)
	v.SetDefault("intelligence.weights.size", defaults.Intelligence.Weights.Size)
	v.SetDefault("intelligence.weights.git_activity", defaults.Intelligence.Weights.GitActivity)
	v.SetDefault("intelligence.weights.risk", defaults.Intelligence.Weights.Risk)
	v.SetDefault("watch.interval", defaults.Watch.Interval)
	v.SetDefault("watch.threshold_mb", defaults.Watch.ThresholdMB)

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config %s: %w", configPath, err)
		}
	} else {
		resolver := platform.New()
		paths := resolver.Resolve()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(paths.ConfigDir)
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
				return nil, err
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
