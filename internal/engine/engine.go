package engine

import (
	"context"
	"errors"
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
				results <- domain.ScanResult{
					Ecosystem: name,
					Error:     ErrNotInstalled,
				}
				continue
			}

			rules := ecoCfg.ToRules()
			c := cleaner
			n := name

			g.Go(func() error {
				targets, err := c.Scan(gctx, rules)
				if err != nil {
					results <- domain.ScanResult{
						Ecosystem: n,
						Error:     err,
					}
					if rules.FailMode == "abort" {
						mu.Lock()
						abortErr = err
						mu.Unlock()
						return err // cancels errgroup context
					}
					return nil // continue mode
				}
				// Use non-blocking send with fallback: even if context is
				// cancelled (abort from another cleaner), deliver completed results.
				select {
				case results <- domain.ScanResult{
					Ecosystem: n,
					Targets:   targets,
				}:
				default:
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
	default:
		return config.EcosystemConfig{}
	}
}
