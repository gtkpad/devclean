package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
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
	Docker   EcosystemConfig `mapstructure:"docker"`
	Node     EcosystemConfig `mapstructure:"node"`
	Go       EcosystemConfig `mapstructure:"go"`
	DotNet   EcosystemConfig `mapstructure:"dotnet"`
	Python   EcosystemConfig `mapstructure:"python"`
	Rust     EcosystemConfig `mapstructure:"rust"`
	Java     EcosystemConfig `mapstructure:"java"`
	AI       EcosystemConfig `mapstructure:"ai"`
	Pnpm     EcosystemConfig `mapstructure:"pnpm"`
	Bun      EcosystemConfig `mapstructure:"bun"`
	NextJS   EcosystemConfig `mapstructure:"nextjs"`
	Xcode    EcosystemConfig `mapstructure:"xcode"`
	Homebrew  EcosystemConfig `mapstructure:"homebrew"`
	CPP       EcosystemConfig `mapstructure:"cpp"`
	Flutter   EcosystemConfig `mapstructure:"flutter"`
	Terraform EcosystemConfig `mapstructure:"terraform"`
	CocoaPods EcosystemConfig `mapstructure:"cocoapods"`
	Elixir    EcosystemConfig `mapstructure:"elixir"`
	Shell     EcosystemConfig `mapstructure:"shell"`
	MacOS     EcosystemConfig `mapstructure:"macos"`
	Browsers  EcosystemConfig `mapstructure:"browsers"`
	Leftovers EcosystemConfig `mapstructure:"leftovers"`
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
	Node      HookConfig `mapstructure:"node"`
	Go        HookConfig `mapstructure:"go"`
	Docker    HookConfig `mapstructure:"docker"`
	DotNet    HookConfig `mapstructure:"dotnet"`
	Python    HookConfig `mapstructure:"python"`
	Rust      HookConfig `mapstructure:"rust"`
	Java      HookConfig `mapstructure:"java"`
	AI        HookConfig `mapstructure:"ai"`
	Pnpm      HookConfig `mapstructure:"pnpm"`
	Bun       HookConfig `mapstructure:"bun"`
	NextJS    HookConfig `mapstructure:"nextjs"`
	Xcode     HookConfig `mapstructure:"xcode"`
	Homebrew  HookConfig `mapstructure:"homebrew"`
	CPP       HookConfig `mapstructure:"cpp"`
	Flutter   HookConfig `mapstructure:"flutter"`
	Terraform HookConfig `mapstructure:"terraform"`
	CocoaPods HookConfig `mapstructure:"cocoapods"`
	Elixir    HookConfig `mapstructure:"elixir"`
	Shell     HookConfig `mapstructure:"shell"`
	MacOS     HookConfig `mapstructure:"macos"`
	Browsers  HookConfig `mapstructure:"browsers"`
	Leftovers HookConfig `mapstructure:"leftovers"`
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

type InstallerConfig struct {
	Paths    []string `mapstructure:"paths"`
	MaxDepth int      `mapstructure:"max_depth"`
}

type SchedulerConfig struct {
	Args []string `mapstructure:"args"`
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
	Installer         InstallerConfig    `mapstructure:"installer"`
	Scheduler         SchedulerConfig    `mapstructure:"scheduler"`
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
	v.SetDefault("ecosystems.ai.enabled", defaults.Ecosystems.AI.Enabled)
	v.SetDefault("ecosystems.ai.max_age_days", defaults.Ecosystems.AI.MaxAgeDays)
	v.SetDefault("ecosystems.ai.fail_mode", defaults.Ecosystems.AI.FailMode)
	v.SetDefault("ecosystems.ai.exclude_paths", defaults.Ecosystems.AI.ExcludePaths)
	v.SetDefault("ecosystems.pnpm.enabled", defaults.Ecosystems.Pnpm.Enabled)
	v.SetDefault("ecosystems.pnpm.max_age_days", defaults.Ecosystems.Pnpm.MaxAgeDays)
	v.SetDefault("ecosystems.pnpm.fail_mode", defaults.Ecosystems.Pnpm.FailMode)
	v.SetDefault("ecosystems.pnpm.exclude_paths", defaults.Ecosystems.Pnpm.ExcludePaths)
	v.SetDefault("ecosystems.bun.enabled", defaults.Ecosystems.Bun.Enabled)
	v.SetDefault("ecosystems.bun.max_age_days", defaults.Ecosystems.Bun.MaxAgeDays)
	v.SetDefault("ecosystems.bun.fail_mode", defaults.Ecosystems.Bun.FailMode)
	v.SetDefault("ecosystems.bun.exclude_paths", defaults.Ecosystems.Bun.ExcludePaths)
	v.SetDefault("ecosystems.nextjs.enabled", defaults.Ecosystems.NextJS.Enabled)
	v.SetDefault("ecosystems.nextjs.max_age_days", defaults.Ecosystems.NextJS.MaxAgeDays)
	v.SetDefault("ecosystems.nextjs.fail_mode", defaults.Ecosystems.NextJS.FailMode)
	v.SetDefault("ecosystems.nextjs.exclude_paths", defaults.Ecosystems.NextJS.ExcludePaths)
	v.SetDefault("ecosystems.xcode.enabled", defaults.Ecosystems.Xcode.Enabled)
	v.SetDefault("ecosystems.xcode.max_age_days", defaults.Ecosystems.Xcode.MaxAgeDays)
	v.SetDefault("ecosystems.xcode.fail_mode", defaults.Ecosystems.Xcode.FailMode)
	v.SetDefault("ecosystems.xcode.exclude_paths", defaults.Ecosystems.Xcode.ExcludePaths)
	v.SetDefault("ecosystems.homebrew.enabled", defaults.Ecosystems.Homebrew.Enabled)
	v.SetDefault("ecosystems.homebrew.max_age_days", defaults.Ecosystems.Homebrew.MaxAgeDays)
	v.SetDefault("ecosystems.homebrew.fail_mode", defaults.Ecosystems.Homebrew.FailMode)
	v.SetDefault("ecosystems.homebrew.exclude_paths", defaults.Ecosystems.Homebrew.ExcludePaths)
	v.SetDefault("ecosystems.cpp.enabled", defaults.Ecosystems.CPP.Enabled)
	v.SetDefault("ecosystems.cpp.max_age_days", defaults.Ecosystems.CPP.MaxAgeDays)
	v.SetDefault("ecosystems.cpp.fail_mode", defaults.Ecosystems.CPP.FailMode)
	v.SetDefault("ecosystems.cpp.exclude_paths", defaults.Ecosystems.CPP.ExcludePaths)
	v.SetDefault("ecosystems.flutter.enabled", defaults.Ecosystems.Flutter.Enabled)
	v.SetDefault("ecosystems.flutter.max_age_days", defaults.Ecosystems.Flutter.MaxAgeDays)
	v.SetDefault("ecosystems.flutter.fail_mode", defaults.Ecosystems.Flutter.FailMode)
	v.SetDefault("ecosystems.flutter.exclude_paths", defaults.Ecosystems.Flutter.ExcludePaths)
	v.SetDefault("ecosystems.terraform.enabled", defaults.Ecosystems.Terraform.Enabled)
	v.SetDefault("ecosystems.terraform.max_age_days", defaults.Ecosystems.Terraform.MaxAgeDays)
	v.SetDefault("ecosystems.terraform.fail_mode", defaults.Ecosystems.Terraform.FailMode)
	v.SetDefault("ecosystems.terraform.exclude_paths", defaults.Ecosystems.Terraform.ExcludePaths)
	v.SetDefault("ecosystems.cocoapods.enabled", defaults.Ecosystems.CocoaPods.Enabled)
	v.SetDefault("ecosystems.cocoapods.max_age_days", defaults.Ecosystems.CocoaPods.MaxAgeDays)
	v.SetDefault("ecosystems.cocoapods.fail_mode", defaults.Ecosystems.CocoaPods.FailMode)
	v.SetDefault("ecosystems.cocoapods.exclude_paths", defaults.Ecosystems.CocoaPods.ExcludePaths)
	v.SetDefault("ecosystems.elixir.enabled", defaults.Ecosystems.Elixir.Enabled)
	v.SetDefault("ecosystems.elixir.max_age_days", defaults.Ecosystems.Elixir.MaxAgeDays)
	v.SetDefault("ecosystems.elixir.fail_mode", defaults.Ecosystems.Elixir.FailMode)
	v.SetDefault("ecosystems.elixir.exclude_paths", defaults.Ecosystems.Elixir.ExcludePaths)
	v.SetDefault("ecosystems.shell.enabled", defaults.Ecosystems.Shell.Enabled)
	v.SetDefault("ecosystems.shell.max_age_days", defaults.Ecosystems.Shell.MaxAgeDays)
	v.SetDefault("ecosystems.shell.fail_mode", defaults.Ecosystems.Shell.FailMode)
	v.SetDefault("ecosystems.shell.exclude_paths", defaults.Ecosystems.Shell.ExcludePaths)
	v.SetDefault("ecosystems.macos.enabled", defaults.Ecosystems.MacOS.Enabled)
	v.SetDefault("ecosystems.macos.max_age_days", defaults.Ecosystems.MacOS.MaxAgeDays)
	v.SetDefault("ecosystems.macos.fail_mode", defaults.Ecosystems.MacOS.FailMode)
	v.SetDefault("ecosystems.macos.exclude_paths", defaults.Ecosystems.MacOS.ExcludePaths)
	v.SetDefault("ecosystems.browsers.enabled", defaults.Ecosystems.Browsers.Enabled)
	v.SetDefault("ecosystems.browsers.max_age_days", defaults.Ecosystems.Browsers.MaxAgeDays)
	v.SetDefault("ecosystems.browsers.fail_mode", defaults.Ecosystems.Browsers.FailMode)
	v.SetDefault("ecosystems.browsers.exclude_paths", defaults.Ecosystems.Browsers.ExcludePaths)
	v.SetDefault("ecosystems.leftovers.enabled", defaults.Ecosystems.Leftovers.Enabled)
	v.SetDefault("ecosystems.leftovers.max_age_days", defaults.Ecosystems.Leftovers.MaxAgeDays)
	v.SetDefault("ecosystems.leftovers.fail_mode", defaults.Ecosystems.Leftovers.FailMode)
	v.SetDefault("ecosystems.leftovers.exclude_paths", defaults.Ecosystems.Leftovers.ExcludePaths)
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
	v.SetDefault("hooks.ai.timeout", defaults.Hooks.AI.Timeout)
	v.SetDefault("hooks.pnpm.timeout", defaults.Hooks.Pnpm.Timeout)
	v.SetDefault("hooks.bun.timeout", defaults.Hooks.Bun.Timeout)
	v.SetDefault("hooks.nextjs.timeout", defaults.Hooks.NextJS.Timeout)
	v.SetDefault("hooks.xcode.timeout", defaults.Hooks.Xcode.Timeout)
	v.SetDefault("hooks.homebrew.timeout", defaults.Hooks.Homebrew.Timeout)
	v.SetDefault("hooks.cpp.timeout", defaults.Hooks.CPP.Timeout)
	v.SetDefault("hooks.flutter.timeout", defaults.Hooks.Flutter.Timeout)
	v.SetDefault("hooks.terraform.timeout", defaults.Hooks.Terraform.Timeout)
	v.SetDefault("hooks.cocoapods.timeout", defaults.Hooks.CocoaPods.Timeout)
	v.SetDefault("hooks.elixir.timeout", defaults.Hooks.Elixir.Timeout)
	v.SetDefault("hooks.shell.timeout", defaults.Hooks.Shell.Timeout)
	v.SetDefault("hooks.macos.timeout", defaults.Hooks.MacOS.Timeout)
	v.SetDefault("hooks.browsers.timeout", defaults.Hooks.Browsers.Timeout)
	v.SetDefault("hooks.leftovers.timeout", defaults.Hooks.Leftovers.Timeout)
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
	v.SetDefault("installer.paths", defaults.Installer.Paths)
	v.SetDefault("installer.max_depth", defaults.Installer.MaxDepth)
	v.SetDefault("scheduler.args", defaults.Scheduler.Args)

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

// CanonicalDiscoveryFolders lista determinística estrita das 7 pastas de projetos sob $HOME (D-09).
var CanonicalDiscoveryFolders = []string{
	"Documents",
	"Projects",
	"Developer",
	"Code",
	"Work",
	"repos",
	"workspace",
}

// EffectiveScanPaths retorna os diretórios ativos de varredura.
// Se ScanPaths estiver preenchido no YAML ou por flag, tem precedência estrita (D-12).
// Caso contrário, descobre as pastas existentes sob $HOME em ordem determinística (D-09, D-10, D-11).
func (c *Config) EffectiveScanPaths() []string {
	if c == nil {
		return []string{}
	}
	if len(c.ScanPaths) > 0 {
		paths := make([]string, 0, len(c.ScanPaths))
		for _, p := range c.ScanPaths {
			if clean := filepath.Clean(p); clean != "." && clean != "" {
				paths = append(paths, clean)
			}
		}
		return paths
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return []string{}
	}

	discovered := make([]string, 0, len(CanonicalDiscoveryFolders))
	for _, folder := range CanonicalDiscoveryFolders {
		candidate := filepath.Join(home, folder)
		if fs.DirExists(candidate) {
			realPath, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				realPath = candidate
			}
			if !safety.IsCoreBlacklisted(realPath) {
				discovered = append(discovered, realPath)
			}
		}
	}

	return discovered
}

