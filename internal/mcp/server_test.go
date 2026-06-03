package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/mark3labs/mcp-go/mcp"
)

type fakeCleaner struct {
	name      string
	installed bool
	targets   []domain.Target
	cleaned   []domain.Target
}

func (f *fakeCleaner) Name() string        { return f.name }
func (f *fakeCleaner) IsInstalled() bool   { return f.installed }
func (f *fakeCleaner) Scan(_ context.Context, _ domain.EcosystemRules) ([]domain.Target, error) {
	return f.targets, nil
}
func (f *fakeCleaner) Clean(_ context.Context, targets []domain.Target) error {
	f.cleaned = append(f.cleaned, targets...)
	return nil
}

type fakeResolver struct{}

func (r *fakeResolver) Resolve() platform.Paths      { return platform.Paths{} }
func (r *fakeResolver) IsNetworkMount(_ string) bool { return false }

func testServer() (*Server, *fakeCleaner) {
	cfg := &config.Config{
		Ecosystems: config.EcosystemsConfig{
			Docker: config.EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue"},
		},
	}
	eng := engine.New(cfg, &fakeResolver{})

	fc := &fakeCleaner{
		name:      "docker",
		installed: true,
		targets: []domain.Target{
			{ID: "img-1", Name: "dangling", Path: "docker://img-1", SizeBytes: 1024, Category: "dangling-image"},
			{ID: "img-2", Name: "dangling2", Path: "docker://img-2", SizeBytes: 2048, Category: "dangling-image"},
		},
	}
	eng.Register(fc)

	srv := New(eng, cfg)
	return srv, fc
}

type fakeHistoryReader struct {
	entries []history.Entry
	err     error
}

func (f *fakeHistoryReader) Read() ([]history.Entry, error) {
	return f.entries, f.err
}

func (f *fakeHistoryReader) ReadFiltered(opts history.FilterOpts) ([]history.Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	var filtered []history.Entry
	for _, e := range f.entries {
		if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
			continue
		}
		if opts.Ecosystem != "" {
			found := false
			for _, eco := range e.Ecosystems {
				if eco == opts.Ecosystem {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
}

type fakeConfigProvider struct {
	cfg    *config.Config
	cfgDir string
}

func (f *fakeConfigProvider) Config() *config.Config { return f.cfg }
func (f *fakeConfigProvider) ConfigDir() string      { return f.cfgDir }

func TestHandleScan_ReturnsTargets(t *testing.T) {
	srv, _ := testServer()

	result, err := srv.handleScan(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)

	var output []struct {
		Ecosystem string          `json:"ecosystem"`
		Targets   []domain.Target `json:"targets"`
	}
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(output) != 1 || output[0].Ecosystem != "docker" {
		t.Fatalf("expected 1 docker ecosystem, got %d", len(output))
	}
	if len(output[0].Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(output[0].Targets))
	}
}

func TestHandlePropose_BlocksBlacklisted(t *testing.T) {
	cfg := &config.Config{
		ProtectedPatterns: []string{"*.env"},
		Ecosystems: config.EcosystemsConfig{
			Docker: config.EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue"},
		},
	}
	eng := engine.New(cfg, &fakeResolver{})
	eng.Register(&fakeCleaner{
		name:      "docker",
		installed: true,
		targets: []domain.Target{
			{ID: "safe-1", Name: "cache", Path: "/tmp/cache", SizeBytes: 100},
			{ID: "blocked-1", Name: "env", Path: "/home/user/.env", SizeBytes: 50},
		},
	})

	srv := New(eng, cfg)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"target_ids": `["safe-1", "blocked-1", "missing-1"]`,
	}

	result, err := srv.handlePropose(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)

	var entries []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	statuses := make(map[string]string)
	for _, e := range entries {
		statuses[e.ID] = e.Status
	}

	if statuses["safe-1"] != "approved" {
		t.Errorf("expected safe-1 approved, got %s", statuses["safe-1"])
	}
	if statuses["blocked-1"] != "blocked" {
		t.Errorf("expected blocked-1 blocked, got %s", statuses["blocked-1"])
	}
	if statuses["missing-1"] != "not_found" {
		t.Errorf("expected missing-1 not_found, got %s", statuses["missing-1"])
	}
}

func TestHandleExecute_DeletesApproved(t *testing.T) {
	srv, fc := testServer()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"target_ids": `["img-1"]`,
	}

	result, err := srv.handleExecute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if int(out["deleted_count"].(float64)) != 1 {
		t.Errorf("expected 1 deleted, got %v", out["deleted_count"])
	}
	if int(out["freed_bytes"].(float64)) != 1024 {
		t.Errorf("expected 1024 freed, got %v", out["freed_bytes"])
	}
	if len(fc.cleaned) != 1 || fc.cleaned[0].ID != "img-1" {
		t.Errorf("expected img-1 to be cleaned, got %v", fc.cleaned)
	}
}

func extractText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("empty result content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func TestHandleGetStats_ReturnsAggregates(t *testing.T) {
	entries := []history.Entry{
		{
			Timestamp:  time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Targets:    3,
			FreedBytes: 1000,
			Ecosystems: []string{"node"},
			Mode:       "interactive",
		},
		{
			Timestamp:  time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC),
			Targets:    2,
			FreedBytes: 2000,
			Ecosystems: []string{"docker"},
			Mode:       "auto",
		},
	}

	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithHistory(&fakeHistoryReader{entries: entries}))

	req := mcp.CallToolRequest{}
	result, err := srv.handleGetStats(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)

	var resp struct {
		TotalFreedBytes int64 `json:"total_freed_bytes"`
		TotalRuns       int   `json:"total_runs"`
		PerEcosystem    []struct {
			Name       string `json:"name"`
			FreedBytes int64  `json:"freed_bytes"`
		} `json:"per_ecosystem"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.TotalFreedBytes != 3000 {
		t.Errorf("expected 3000 freed bytes, got %d", resp.TotalFreedBytes)
	}
	if resp.TotalRuns != 2 {
		t.Errorf("expected 2 runs, got %d", resp.TotalRuns)
	}
	if len(resp.PerEcosystem) != 2 {
		t.Errorf("expected 2 ecosystems, got %d", len(resp.PerEcosystem))
	}
}

func TestHandleGetStats_WithFilters(t *testing.T) {
	entries := []history.Entry{
		{
			Timestamp:  time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
			Targets:    1,
			FreedBytes: 500,
			Ecosystems: []string{"node"},
		},
		{
			Timestamp:  time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC),
			Targets:    2,
			FreedBytes: 2000,
			Ecosystems: []string{"docker"},
		},
	}

	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithHistory(&fakeHistoryReader{entries: entries}))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"since":     "2026-05-01",
		"ecosystem": "docker",
	}

	result, err := srv.handleGetStats(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var resp struct {
		TotalRuns int `json:"total_runs"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TotalRuns != 1 {
		t.Errorf("expected 1 run after filter, got %d", resp.TotalRuns)
	}
}

func TestHandleGetStats_IncludeHistory(t *testing.T) {
	entries := []history.Entry{
		{Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC), Targets: 1, FreedBytes: 100, Ecosystems: []string{"go"}},
		{Timestamp: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC), Targets: 2, FreedBytes: 200, Ecosystems: []string{"go"}},
		{Timestamp: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC), Targets: 3, FreedBytes: 300, Ecosystems: []string{"go"}},
	}

	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithHistory(&fakeHistoryReader{entries: entries}))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"include_history": true,
		"limit":           float64(2),
	}

	result, err := srv.handleGetStats(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var resp struct {
		History []history.Entry `json:"history"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.History) != 2 {
		t.Errorf("expected 2 history entries (limit), got %d", len(resp.History))
	}
}

func TestHandleGetStats_NilHistory(t *testing.T) {
	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg)

	req := mcp.CallToolRequest{}
	result, err := srv.handleGetStats(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	if text != "history not available" {
		t.Errorf("expected error message, got %s", text)
	}
}

func TestHandleGetConfig_ReturnsFullConfig(t *testing.T) {
	cfg := &config.Config{
		ScanPaths: []string{"/home/user/dev"},
		Ecosystems: config.EcosystemsConfig{
			Node: config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
		},
	}

	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithConfigProvider(&fakeConfigProvider{cfg: cfg, cfgDir: "/tmp/config"}))

	req := mcp.CallToolRequest{}
	result, err := srv.handleGetConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var out config.Config
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(out.ScanPaths) != 1 || out.ScanPaths[0] != "/home/user/dev" {
		t.Errorf("unexpected scan_paths: %v", out.ScanPaths)
	}
	if !out.Ecosystems.Node.Enabled {
		t.Error("expected node ecosystem enabled")
	}
}

func TestHandleGetConfig_NilProvider(t *testing.T) {
	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg)

	req := mcp.CallToolRequest{}
	result, err := srv.handleGetConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	if text != "config not available" {
		t.Errorf("expected error message, got %s", text)
	}
}
