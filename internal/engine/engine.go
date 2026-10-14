package engine

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/platform"
	"golang.org/x/sync/errgroup"
)

// ErrNotInstalled is returned when a cleaner's tool is not available.
var ErrNotInstalled = errors.New("not installed")

// Engine orchestrates parallel cleaner execution.
type Engine struct {
	cleaners map[string]domain.Cleaner
	config   *config.Config
	resolver platform.Resolver
}

// New creates a scan engine.
func New(cfg *config.Config, resolver platform.Resolver) *Engine {
	if cfg == nil {
		def := config.Defaults()
		cfg = &def
	}
	return &Engine{
		cleaners: make(map[string]domain.Cleaner),
		config:   cfg,
		resolver: resolver,
	}
}

// Register adds a cleaner to the engine.
func (e *Engine) Register(c domain.Cleaner) {
	e.cleaners[c.Name()] = c
}

// GetCleaner returns a registered cleaner by ecosystem name.
func (e *Engine) GetCleaner(name string) domain.Cleaner {
	return e.cleaners[name]
}

// Ecosystems returns a sorted list of registered ecosystem names.
func (e *Engine) Ecosystems() []string {
	names := make([]string, 0, len(e.cleaners))
	for name := range e.cleaners {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Scan runs all registered cleaners in parallel and streams results.
// Channel is closed when all cleaners finish.
func (e *Engine) Scan(ctx context.Context) <-chan domain.ScanResult {
	results := make(chan domain.ScanResult, len(e.cleaners))

	go func() {
		defer close(results)

		g, gctx := errgroup.WithContext(ctx)
		var mu sync.Mutex
		var abortErr error

		for name, cleaner := range e.cleaners {
			ecoCfg := e.ecosystemConfig(name)
			if !ecoCfg.Enabled {
				continue
			}

			if !cleaner.IsInstalled() {
				select {
				case results <- domain.ScanResult{
					Ecosystem: name,
					Error:     ErrNotInstalled,
				}:
				case <-ctx.Done():
					return
				}
				continue
			}

			rules := ecoCfg.ToRules()
			c := cleaner
			n := name

			g.Go(func() error {
				targets, err := c.Scan(gctx, rules)
				if err != nil {
					select {
					case results <- domain.ScanResult{
						Ecosystem: n,
						Error:     err,
					}:
					case <-gctx.Done():
						return gctx.Err()
					}
					if rules.FailMode == "abort" {
						mu.Lock()
						abortErr = err
						mu.Unlock()
						return err // cancels errgroup context
					}
					return nil // continue mode
				}
				select {
				case results <- domain.ScanResult{
					Ecosystem: n,
					Targets:   targets,
				}:
				case <-gctx.Done():
					return gctx.Err()
				}
				return nil
			})
		}

		_ = g.Wait()
		_ = abortErr
	}()

	return results
}

// FilterNetworkMounts filters out paths that are on network mounts.
func (e *Engine) FilterNetworkMounts(ctx context.Context, paths []string) []string {
	var local []string
	for _, p := range paths {
		if !fs.IsNetworkMount(ctx, e.resolver, p) {
			local = append(local, p)
		}
	}
	return local
}

// Collect drains the results channel into a slice.
func Collect(ctx context.Context, ch <-chan domain.ScanResult) []domain.ScanResult {
	var results []domain.ScanResult
	for {
		select {
		case r, ok := <-ch:
			if !ok {
				return results
			}
			results = append(results, r)
		case <-ctx.Done():
			return results
		}
	}
}

func (e *Engine) ecosystemConfig(name string) config.EcosystemConfig {
	switch name {
	case "docker":
		return e.config.Ecosystems.Docker
	case "go":
		return e.config.Ecosystems.Go
	case "dotnet":
		return e.config.Ecosystems.DotNet
	case "python":
		return e.config.Ecosystems.Python
	case "node":
		return e.config.Ecosystems.Node
	case "rust":
		return e.config.Ecosystems.Rust
	case "java":
		return e.config.Ecosystems.Java
	case "ai":
		return e.config.Ecosystems.AI
	case "pnpm":
		return e.config.Ecosystems.Pnpm
	case "bun":
		return e.config.Ecosystems.Bun
	case "nextjs":
		return e.config.Ecosystems.NextJS
	case "xcode":
		return e.config.Ecosystems.Xcode
	case "homebrew":
		return e.config.Ecosystems.Homebrew
	case "cpp":
		return e.config.Ecosystems.CPP
	case "flutter":
		return e.config.Ecosystems.Flutter
	case "terraform":
		return e.config.Ecosystems.Terraform
	case "cocoapods":
		return e.config.Ecosystems.CocoaPods
	case "elixir":
		return e.config.Ecosystems.Elixir
	default:
		return config.EcosystemConfig{Enabled: true, FailMode: "continue"}
	}
}
