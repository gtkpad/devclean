package config

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

var validPlatforms = map[string]bool{
	"linux":   true,
	"darwin":  true,
	"windows": true,
}

// Validate checks the config for invalid values and returns a clear error message.
func (c *Config) Validate() error {
	var errs []string

	for i, p := range c.ScanPaths {
		if strings.TrimSpace(p) == "" {
			errs = append(errs, fmt.Sprintf("scan_paths[%d]: empty path", i))
		}
	}

	hooks := map[string]HookConfig{
		"hooks.node":   c.Hooks.Node,
		"hooks.go":     c.Hooks.Go,
		"hooks.docker": c.Hooks.Docker,
		"hooks.dotnet": c.Hooks.DotNet,
		"hooks.python": c.Hooks.Python,
		"hooks.rust":   c.Hooks.Rust,
		"hooks.java":   c.Hooks.Java,
	}

	for name, h := range hooks {
		if h.Timeout != "" {
			if _, err := time.ParseDuration(h.Timeout); err != nil {
				errs = append(errs, fmt.Sprintf("%s.timeout: invalid duration %q", name, h.Timeout))
			}
		}
		for _, p := range h.Platform {
			if !validPlatforms[p] {
				errs = append(errs, fmt.Sprintf("%s.platform: unsupported platform %q (valid: linux, darwin, windows)", name, p))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// ShouldRunHook checks whether a hook should run on the current platform.
func (h HookConfig) ShouldRunHook() bool {
	if len(h.Platform) == 0 {
		return true
	}
	for _, p := range h.Platform {
		if p == runtime.GOOS {
			return true
		}
	}
	return false
}
