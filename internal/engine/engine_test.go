package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/platform"
)

type fakeCleaner struct {
	name      string
	installed bool
	scanFn    func(domain.EcosystemRules) ([]domain.Target, error)
}

func (f *fakeCleaner) Name() string                                    { return f.name }
func (f *fakeCleaner) IsInstalled() bool                               { return f.installed }
func (f *fakeCleaner) Clean(_ context.Context, _ []domain.Target) error { return nil }
func (f *fakeCleaner) Scan(_ context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if f.scanFn != nil {
		return f.scanFn(rules)
	}
	return nil, nil
}

type fakeResolver struct{}

func (r *fakeResolver) Resolve() platform.Paths      { return platform.Paths{} }
func (r *fakeResolver) IsNetworkMount(_ string) bool { return false }

func defaultConfig() *config.Config {
	return &config.Config{
		Ecosystems: config.EcosystemsConfig{
			Docker: config.EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue"},
			Go:     config.EcosystemConfig{Enabled: true, MaxAgeDays: 60, FailMode: "continue"},
			DotNet: config.EcosystemConfig{Enabled: true, MaxAgeDays: 45, FailMode: "continue"},
			Python: config.EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue"},
			Node:   config.EcosystemConfig{Enabled: true, MaxAgeDays: 14, FailMode: "continue"},
		},
	}
}

func TestScan_ParallelExecution(t *testing.T) {
	cfg := defaultConfig()
	e := New(cfg, &fakeResolver{})

	var count atomic.Int32
	makeScanner := func(name string) *fakeCleaner {
		return &fakeCleaner{
			name:      name,
			installed: true,
			scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
				count.Add(1)
				time.Sleep(50 * time.Millisecond)
				return []domain.Target{{ID: name, Category: name}}, nil
			},
		}
	}

	e.Register(makeScanner("docker"))
	e.Register(makeScanner("go"))
	e.Register(makeScanner("python"))

	start := time.Now()
	ctx := context.Background()
	results := Collect(ctx, e.Scan(ctx))
	elapsed := time.Since(start)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// If truly parallel, 3 x 50ms cleaners should take ~50ms, not ~150ms
	if elapsed > 200*time.Millisecond {
		t.Errorf("execution too slow (%v), likely sequential", elapsed)
	}
}

func TestScan_GracefulDegradation_Continue(t *testing.T) {
	cfg := defaultConfig()
	cfg.Ecosystems.Docker.FailMode = "continue"

	e := New(cfg, &fakeResolver{})
	e.Register(&fakeCleaner{
		name:      "docker",
		installed: true,
		scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
			return nil, errors.New("daemon offline")
		},
	})
	e.Register(&fakeCleaner{
		name:      "go",
		installed: true,
		scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
			return []domain.Target{{ID: "go-cache", Category: "go-build-cache"}}, nil
		},
	})

	ctx := context.Background()
	results := Collect(ctx, e.Scan(ctx))

	var dockerResult, goResult *domain.ScanResult
	for i := range results {
		switch results[i].Ecosystem {
		case "docker":
			dockerResult = &results[i]
		case "go":
			goResult = &results[i]
		}
	}

	if dockerResult == nil || dockerResult.Error == nil {
		t.Error("expected docker to have error")
	}
	if goResult == nil || len(goResult.Targets) != 1 {
		t.Error("expected go to have 1 target")
	}
}

func TestScan_GracefulDegradation_Abort(t *testing.T) {
	cfg := defaultConfig()
	cfg.Ecosystems.Docker.FailMode = "abort"

	e := New(cfg, &fakeResolver{})
	e.Register(&fakeCleaner{
		name:      "docker",
		installed: true,
		scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
			return nil, errors.New("critical failure")
		},
	})
	e.Register(&fakeCleaner{
		name:      "go",
		installed: true,
		scanFn: func(rules domain.EcosystemRules) ([]domain.Target, error) {
			// Simulate slow scan that should be cancelled
			time.Sleep(200 * time.Millisecond)
			return []domain.Target{{ID: "go"}}, nil
		},
	})

	ctx := context.Background()
	results := Collect(ctx, e.Scan(ctx))

	var hasDockerError bool
	for _, r := range results {
		if r.Ecosystem == "docker" && r.Error != nil {
			hasDockerError = true
		}
	}
	if !hasDockerError {
		t.Error("expected docker error result")
	}
}

func TestScan_DisabledEcosystem(t *testing.T) {
	cfg := defaultConfig()
	cfg.Ecosystems.Docker.Enabled = false

	e := New(cfg, &fakeResolver{})
	called := false
	e.Register(&fakeCleaner{
		name:      "docker",
		installed: true,
		scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
			called = true
			return nil, nil
		},
	})

	ctx := context.Background()
	_ = Collect(ctx, e.Scan(ctx))

	if called {
		t.Error("disabled cleaner should not be scanned")
	}
}

func TestScan_NotInstalled(t *testing.T) {
	cfg := defaultConfig()
	e := New(cfg, &fakeResolver{})
	e.Register(&fakeCleaner{
		name:      "docker",
		installed: false,
	})

	ctx := context.Background()
	results := Collect(ctx, e.Scan(ctx))

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !errors.Is(results[0].Error, ErrNotInstalled) {
		t.Errorf("expected ErrNotInstalled, got %v", results[0].Error)
	}
}

func TestCollect_DrainsChannel(t *testing.T) {
	ch := make(chan domain.ScanResult, 3)
	ch <- domain.ScanResult{Ecosystem: "a"}
	ch <- domain.ScanResult{Ecosystem: "b"}
	ch <- domain.ScanResult{Ecosystem: "c"}
	close(ch)

	results := Collect(context.Background(), ch)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestScan_ContextCancellation(t *testing.T) {
	cfg := defaultConfig()
	e := New(cfg, &fakeResolver{})
	e.Register(&fakeCleaner{
		name:      "docker",
		installed: true,
		scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
			time.Sleep(500 * time.Millisecond)
			return nil, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	ch := e.Scan(ctx)
	cancel() // cancel immediately

	// Collect should return without blocking indefinitely
	done := make(chan struct{})
	go func() {
		_ = Collect(context.Background(), ch)
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("Collect blocked after context cancellation")
	}
}

func TestScan_AbortPreservesCompletedResults(t *testing.T) {
	cfg := defaultConfig()
	cfg.Ecosystems.Go.FailMode = "abort"

	e := New(cfg, &fakeResolver{})

	// Fast cleaner completes before abort
	e.Register(&fakeCleaner{
		name:      "docker",
		installed: true,
		scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
			return []domain.Target{{ID: "docker-img", Category: "dangling-image"}}, nil
		},
	})

	// Slow cleaner triggers abort
	e.Register(&fakeCleaner{
		name:      "go",
		installed: true,
		scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
			time.Sleep(50 * time.Millisecond)
			return nil, errors.New("go cache corrupt")
		},
	})

	ctx := context.Background()
	results := Collect(ctx, e.Scan(ctx))

	var hasDocker, hasGoErr bool
	for _, r := range results {
		if r.Ecosystem == "docker" && len(r.Targets) == 1 {
			hasDocker = true
		}
		if r.Ecosystem == "go" && r.Error != nil {
			hasGoErr = true
		}
	}

	if !hasDocker {
		t.Error("expected docker result to be preserved despite go abort")
	}
	if !hasGoErr {
		t.Error("expected go error result")
	}
}
