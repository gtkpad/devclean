package config

import (
	"fmt"
	"math"
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

	// Intelligence validation
	if c.Intelligence.Threshold < 0.0 || c.Intelligence.Threshold > 1.0 {
		errs = append(errs, "intelligence.threshold: must be between 0.0 and 1.0")
	}
	if c.Intelligence.MaxAgeDays <= 0 {
		errs = append(errs, "intelligence.max_age_days: must be > 0")
	}
	if c.Intelligence.RefSizeMB <= 0 {
		errs = append(errs, "intelligence.ref_size_mb: must be > 0")
	}
	weightsSum := c.Intelligence.Weights.Age + c.Intelligence.Weights.Size +
		c.Intelligence.Weights.GitActivity + c.Intelligence.Weights.Risk
	if math.Abs(weightsSum-1.0) > 0.01 {
		errs = append(errs, fmt.Sprintf("intelligence.weights: must sum to 1.0 (got %.4f)", weightsSum))
	}

	// Watch validation
	if c.Watch.Interval != "" {
		d, err := time.ParseDuration(c.Watch.Interval)
		if err != nil {
			errs = append(errs, fmt.Sprintf("watch.interval: invalid duration %q", c.Watch.Interval))
		} else if d < 30*time.Minute {
			errs = append(errs, "watch.interval: must be >= 30m")
		}
	}
	if c.Watch.ThresholdMB <= 0 {
		errs = append(errs, "watch.threshold_mb: must be > 0")
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
