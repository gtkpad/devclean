package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
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
