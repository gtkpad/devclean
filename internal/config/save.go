package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultConfigYAML contains the documented default configuration template
// used when initializing devclean for the first time.
const DefaultConfigYAML = `# DevClean Configuration
# Generated automatically by devclean.

# Directories to scan for cleanable development artifacts
scan_paths: []

# Directories to exclude from scanning
exclude_paths: []

# Protected file glob patterns (never deleted)
protected_patterns: []

# Safety & Trash configuration
safety:
  trash_enabled: true      # Move files to devclean trash instead of permanent deletion
  retention_days: 30       # Days to retain items in trash before auto-purge
  max_size_mb: 10240       # Max trash size in megabytes (10 GB)
  trash_dir: ""            # Custom trash directory (empty = platform default)

# Ecosystem-specific cleanup rules (18 ecosystems)
ecosystems:
  # Docker containers, images, build cache, volumes
  docker:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  # Node.js node_modules and package manager caches
  node:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  # Go build cache and test cache
  go:
    enabled: true
    max_age_days: 60
    fail_mode: continue
    exclude_paths: []

  # .NET bin, obj, and NuGet packages
  dotnet:
    enabled: true
    max_age_days: 45
    fail_mode: continue
    exclude_paths: []

  # Python virtual environments, __pycache__, and uv cache
  python:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  # Rust cargo target directories and cache
  rust:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  # Java Maven target and Gradle cache
  java:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  # Next.js .next cache and build output
  nextjs:
    enabled: true
    max_age_days: 7
    fail_mode: continue
    exclude_paths: []

  # Bun runtime cache and installed dependencies
  bun:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  # Xcode DerivedData and iOS simulator caches
  xcode:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  # AI models & local LLM caches (Claude Code, Ollama, HuggingFace)
  ai:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  # PNPM store and local virtual store
  pnpm:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  # Homebrew download cache
  homebrew:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  # C/C++ build artifacts (CMake, vcpkg, Conan)
  cpp:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  # Flutter / Dart build artifacts and pub cache
  flutter:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  # Terraform / OpenTofu plugin cache and .terraform
  terraform:
    enabled: true
    max_age_days: 30
    fail_mode: continue
    exclude_paths: []

  # CocoaPods cache and Pods directories
  cocoapods:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

  # Elixir / Mix _build and deps directories
  elixir:
    enabled: true
    max_age_days: 14
    fail_mode: continue
    exclude_paths: []

# Background filesystem watcher daemon
watch:
  interval: 6h             # Check interval (e.g. 1h, 6h, 24h)
  threshold_mb: 2048       # Disk reclaimed alert threshold in MB

# Automated cleanup scheduler arguments
scheduler:
  args: []

# Orphan installer detection
installer:
  paths:
    - "~/Downloads"
    - "~/Desktop"
  max_depth: 2
`

// ConfigSyntaxError represents a YAML syntax failure with location and backup hint.
type ConfigSyntaxError struct {
	Path      string
	Err       error
	HasBackup bool
}

func (e *ConfigSyntaxError) Error() string {
	msg := fmt.Sprintf("Erro de sintaxe no arquivo de configuração: %s\n  %v", e.Path, e.Err)
	if e.HasBackup {
		bak := e.Path + ".bak"
		msg += fmt.Sprintf("\n  Dica: Foi detectado um arquivo de backup em %s.\n  Para restaurá-lo, execute: cp %s %s", bak, bak, e.Path)
	}
	return msg
}

// LoadWithAST loads the YAML AST root node and unmarshals into a Config struct.
// Handles missing files transparently and reports detailed syntax errors (D-11, D-12).
func LoadWithAST(configFile string) (*yaml.Node, *Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			defaults := Defaults()
			var root yaml.Node
			if err := yaml.Unmarshal([]byte(DefaultConfigYAML), &root); err != nil {
				return nil, nil, fmt.Errorf("parsing default template: %w", err)
			}
			return &root, &defaults, nil
		}
		return nil, nil, fmt.Errorf("reading config file %s: %w", configFile, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		bakPath := configFile + ".bak"
		hasBak := false
		if _, statErr := os.Stat(bakPath); statErr == nil {
			hasBak = true
		}
		return nil, nil, &ConfigSyntaxError{
			Path:      configFile,
			Err:       err,
			HasBackup: hasBak,
		}
	}

	if len(root.Content) == 0 {
		root = yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{
					Kind: yaml.MappingNode,
					Tag:  "!!map",
				},
			},
		}
	}

	cfg, err := Load(configFile)
	if err != nil {
		return nil, nil, err
	}

	return &root, cfg, nil
}

// SaveConfigNode atomically writes root to configFile with backup creation (D-09, D-10).
func SaveConfigNode(configFile string, root *yaml.Node) error {
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory %s: %w", dir, err)
	}

	// 1. Generate backup if file currently exists
	if _, err := os.Stat(configFile); err == nil {
		bakPath := configFile + ".bak"
		src, err := os.Open(configFile)
		if err == nil {
			dst, err := os.OpenFile(bakPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err == nil {
				_, _ = io.Copy(dst, src)
				_ = dst.Close()
			}
			_ = src.Close()
		}
	}

	// 2. Encode AST with 2-space indentation
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return fmt.Errorf("encoding yaml AST: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing yaml encoder: %w", err)
	}

	// 3. Write to temporary file with Sync()
	tmpPath := configFile + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("creating tmp config file: %w", err)
	}

	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing tmp config: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("syncing tmp config: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing tmp config: %w", err)
	}

	// 4. Atomic POSIX replacement
	if err := os.Rename(tmpPath, configFile); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming config file: %w", err)
	}

	return nil
}
