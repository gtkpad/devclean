package engine

import (
	"context"
	"errors"
	"fmt"
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

func (f *fakeCleaner) Name() string                                     { return f.name }
func (f *fakeCleaner) IsInstalled() bool                                { return f.installed }
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
			// Simulate slow scan that should be canceled
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

func TestEcosystems_ReturnsSortedNames(t *testing.T) {
	cfg := defaultConfig()
	e := New(cfg, &fakeResolver{})

	e.Register(&fakeCleaner{name: "python", installed: true})
	e.Register(&fakeCleaner{name: "docker", installed: true})
	e.Register(&fakeCleaner{name: "go", installed: true})

	got := e.Ecosystems()
	want := []string{"docker", "go", "python"}

	if len(got) != len(want) {
		t.Fatalf("Ecosystems() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Ecosystems()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestScan_NoDroppedResultsUnderConcurrency(t *testing.T) {
	cfg := defaultConfig()
	e := New(cfg, &fakeResolver{})

	const numCleaners = 20
	expectedCleaners := make(map[string]int)

	for i := 0; i < numCleaners; i++ {
		name := fmt.Sprintf("cleaner-%02d", i)
		targetCount := (i % 3) + 1
		expectedCleaners[name] = targetCount
		delay := time.Duration(10+(i*2)) * time.Millisecond

		tc := targetCount
		d := delay
		n := name
		e.Register(&fakeCleaner{
			name:      n,
			installed: true,
			scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
				time.Sleep(d)
				targets := make([]domain.Target, tc)
				for j := 0; j < tc; j++ {
					targets[j] = domain.Target{
						ID:       fmt.Sprintf("%s-target-%d", n, j),
						Category: "test-category",
					}
				}
				return targets, nil
			},
		})
	}

	ctx := context.Background()
	results := Collect(ctx, e.Scan(ctx))

	if len(results) != numCleaners {
		t.Fatalf("expected %d results, got %d", numCleaners, len(results))
	}

	received := make(map[string]int)
	for _, res := range results {
		if res.Error != nil {
			t.Errorf("cleaner %s returned unexpected error: %v", res.Ecosystem, res.Error)
		}
		received[res.Ecosystem] = len(res.Targets)
	}

	for name, expectedCount := range expectedCleaners {
		gotCount, exists := received[name]
		if !exists {
			t.Errorf("missing result for cleaner %s", name)
		} else if gotCount != expectedCount {
			t.Errorf("cleaner %s: expected %d targets, got %d", name, expectedCount, gotCount)
		}
	}
}

type networkMountResolver struct {
	mounts map[string]bool
}

func (r *networkMountResolver) Resolve() platform.Paths { return platform.Paths{} }
func (r *networkMountResolver) IsNetworkMount(path string) bool {
	return r.mounts[path]
}

func TestEngine_NetworkMountsAndCleanerLookup(t *testing.T) {
	cfg := defaultConfig()
	res := &networkMountResolver{
		mounts: map[string]bool{
			"/mnt/nfs/shared": true,
			"/Volumes/smb":    true,
		},
	}
	eng := New(cfg, res)

	// 1. Cleaner Lookup
	cleaner := &fakeCleaner{name: "node", installed: true}
	eng.Register(cleaner)

	if got := eng.GetCleaner("node"); got != cleaner {
		t.Errorf("GetCleaner(node) = %v, want %v", got, cleaner)
	}
	if got := eng.GetCleaner("nonexistent"); got != nil {
		t.Errorf("GetCleaner(nonexistent) = %v, want nil", got)
	}

	// 2. FilterNetworkMounts
	ctx := context.Background()
	paths := []string{
		"/Users/dev/project",
		"/mnt/nfs/shared",
		"/Volumes/smb",
		"/var/cache/local",
	}
	filtered := eng.FilterNetworkMounts(ctx, paths)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 local paths, got %d: %v", len(filtered), filtered)
	}
	if filtered[0] != "/Users/dev/project" || filtered[1] != "/var/cache/local" {
		t.Errorf("unexpected filtered paths: %v", filtered)
	}
}

func TestEngine_ScanContextCancellation_NoDeadlock(t *testing.T) {
	cfg := defaultConfig()
	eng := New(cfg, &fakeResolver{})

	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("slow-cleaner-%d", i)
		eng.Register(&fakeCleaner{
			name:      name,
			installed: true,
			scanFn: func(_ domain.EcosystemRules) ([]domain.Target, error) {
				time.Sleep(100 * time.Millisecond)
				return []domain.Target{{ID: "slow"}}, nil
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()
	results := Collect(ctx, eng.Scan(ctx))
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("scan with cancellation took too long: %v (possible channel deadlock)", elapsed)
	}

	// Se houver erros reportados, devem ser de cancelamento de contexto
	for _, r := range results {
		if r.Error != nil && !errors.Is(r.Error, context.Canceled) && !errors.Is(r.Error, context.DeadlineExceeded) {
			t.Errorf("unexpected error on canceled scan: %v", r.Error)
		}
	}
}




