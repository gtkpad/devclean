package config

import (
	"os"

	"github.com/gabrielvp/devclean/internal/domain"
	"github.com/gabrielvp/devclean/internal/platform"
	"github.com/spf13/viper"
)

type EcosystemConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	MaxAgeDays   int      `mapstructure:"max_age_days"`
	ExcludePaths []string `mapstructure:"exclude_paths"`
}

// ToRules converts an EcosystemConfig to a domain.EcosystemRules.
func (ec EcosystemConfig) ToRules() domain.EcosystemRules {
	return domain.EcosystemRules{
		Enabled:      ec.Enabled,
		MaxAgeDays:   ec.MaxAgeDays,
		ExcludePaths: ec.ExcludePaths,
	}
}

type EcosystemsConfig struct {
	Docker EcosystemConfig `mapstructure:"docker"`
	Node   EcosystemConfig `mapstructure:"node"`
	Go     EcosystemConfig `mapstructure:"go"`
	DotNet EcosystemConfig `mapstructure:"dotnet"`
	Python EcosystemConfig `mapstructure:"python"`
}

type Config struct {
	ScanPaths         []string         `mapstructure:"scan_paths"`
	ProtectedPatterns []string         `mapstructure:"protected_patterns"`
	Ecosystems        EcosystemsConfig `mapstructure:"ecosystems"`
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
	v.SetDefault("ecosystems.docker.exclude_paths", defaults.Ecosystems.Docker.ExcludePaths)
	v.SetDefault("ecosystems.node.enabled", defaults.Ecosystems.Node.Enabled)
	v.SetDefault("ecosystems.node.max_age_days", defaults.Ecosystems.Node.MaxAgeDays)
	v.SetDefault("ecosystems.node.exclude_paths", defaults.Ecosystems.Node.ExcludePaths)
	v.SetDefault("ecosystems.go.enabled", defaults.Ecosystems.Go.Enabled)
	v.SetDefault("ecosystems.go.max_age_days", defaults.Ecosystems.Go.MaxAgeDays)
	v.SetDefault("ecosystems.go.exclude_paths", defaults.Ecosystems.Go.ExcludePaths)
	v.SetDefault("ecosystems.dotnet.enabled", defaults.Ecosystems.DotNet.Enabled)
	v.SetDefault("ecosystems.dotnet.max_age_days", defaults.Ecosystems.DotNet.MaxAgeDays)
	v.SetDefault("ecosystems.dotnet.exclude_paths", defaults.Ecosystems.DotNet.ExcludePaths)
	v.SetDefault("ecosystems.python.enabled", defaults.Ecosystems.Python.Enabled)
	v.SetDefault("ecosystems.python.max_age_days", defaults.Ecosystems.Python.MaxAgeDays)
	v.SetDefault("ecosystems.python.exclude_paths", defaults.Ecosystems.Python.ExcludePaths)

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		resolver := platform.New()
		paths := resolver.Resolve()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(paths.ConfigDir)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
