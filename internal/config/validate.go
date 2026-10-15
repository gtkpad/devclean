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

var allowedCleanFlags = map[string]bool{
	"--auto":        true,
	"--force":       true,
	"--dry-run":     true,
	"--trash":       true,
	"--permanent":   true,
	"--smart":       true,
	"--all":         true,
	"--preset":      true,
	"--format":      true,
	"--ecosystem":   true,
	"-e":            true,
	"--keep-builds": true,
	"--quiet":       true,
	"-q":            true,
	"--verbose":     true,
	"-v":            true,
}

var allowedCleanValues = map[string]bool{
	"safe":        true,
	"standard":    true,
	"aggressive":  true,
	"table":       true,
	"json":        true,
	"docker":      true,
	"node":        true,
	"go":          true,
	"dotnet":      true,
	"python":      true,
	"rust":        true,
	"java":        true,
	"ai":          true,
	"pnpm":        true,
	"bun":         true,
	"nextjs":      true,
	"xcode":       true,
	"homebrew":    true,
	"cpp":         true,
	"flutter":     true,
	"terraform":   true,
	"cocoapods":   true,
	"elixir":      true,
	"shell":       true,
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
		"hooks.node":      c.Hooks.Node,
		"hooks.go":        c.Hooks.Go,
		"hooks.docker":    c.Hooks.Docker,
		"hooks.dotnet":    c.Hooks.DotNet,
		"hooks.python":    c.Hooks.Python,
		"hooks.rust":      c.Hooks.Rust,
		"hooks.java":      c.Hooks.Java,
		"hooks.ai":        c.Hooks.AI,
		"hooks.pnpm":      c.Hooks.Pnpm,
		"hooks.bun":       c.Hooks.Bun,
		"hooks.nextjs":    c.Hooks.NextJS,
		"hooks.xcode":     c.Hooks.Xcode,
		"hooks.homebrew":  c.Hooks.Homebrew,
		"hooks.cpp":       c.Hooks.CPP,
		"hooks.flutter":   c.Hooks.Flutter,
		"hooks.terraform": c.Hooks.Terraform,
		"hooks.cocoapods": c.Hooks.CocoaPods,
		"hooks.elixir":    c.Hooks.Elixir,
		"hooks.shell":     c.Hooks.Shell,
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

	// Installer validation
	if c.Installer.MaxDepth < 0 || c.Installer.MaxDepth > 5 {
		errs = append(errs, "installer.max_depth: must be between 0 and 5")
	}
	for i, p := range c.Installer.Paths {
		if strings.TrimSpace(p) == "" {
			errs = append(errs, fmt.Sprintf("installer.paths[%d]: empty path", i))
		}
	}

	// Safety validation
	if c.Safety.RetentionDays < 1 {
		errs = append(errs, "safety.retention_days: must be >= 1")
	}
	if c.Safety.MaxSizeMB < 100 {
		errs = append(errs, "safety.max_size_mb: must be >= 100")
	}

	// Scheduler.Args validation
	dangerousChars := []string{";", "|", "&", "$", "`", "<", ">", "\n", "\r"}
	for i, arg := range c.Scheduler.Args {
		hasDanger := false
		for _, ch := range dangerousChars {
			if strings.Contains(arg, ch) {
				errs = append(errs, fmt.Sprintf("scheduler.args[%d]: dangerous shell character in %q", i, arg))
				hasDanger = true
				break
			}
		}
		if hasDanger {
			continue
		}

		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			flag, val := parts[0], parts[1]
			if !allowedCleanFlags[flag] {
				errs = append(errs, fmt.Sprintf("scheduler.args[%d]: unsupported flag %q", i, flag))
			} else if !allowedCleanValues[val] {
				errs = append(errs, fmt.Sprintf("scheduler.args[%d]: unsupported value %q for flag %q", i, val, flag))
			}
			continue
		}

		if strings.HasPrefix(arg, "-") {
			if !allowedCleanFlags[arg] {
				errs = append(errs, fmt.Sprintf("scheduler.args[%d]: unsupported flag %q", i, arg))
			}
		} else {
			if !allowedCleanValues[arg] {
				errs = append(errs, fmt.Sprintf("scheduler.args[%d]: unsupported argument %q", i, arg))
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
