package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/mark3labs/mcp-go/mcp"
)

type fakeExplainer struct {
	result *ExplainResult
	err    error
}

func (f *fakeExplainer) Explain(ctx context.Context, path string) (*ExplainResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func TestHandleExplainTarget_Target(t *testing.T) {
	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithExplainer(&fakeExplainer{
		result: &ExplainResult{
			Path:      "/home/user/project/node_modules",
			Status:    "target",
			Ecosystem: "node",
			Reason:    "node_modules/ with sibling manifest",
			SizeBytes: 500000,
			AgeDays:   10,
		},
	}))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"path": "/home/user/project/node_modules"}

	result, err := srv.handleExplainTarget(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var resp ExplainResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != "target" {
		t.Errorf("expected target status, got %s", resp.Status)
	}
	if resp.Ecosystem != "node" {
		t.Errorf("expected node ecosystem, got %s", resp.Ecosystem)
	}
}

func TestHandleExplainTarget_Protected(t *testing.T) {
	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithExplainer(&fakeExplainer{
		result: &ExplainResult{
			Path:   "/home/user/.git",
			Status: "protected",
			Reason: "Protected by pattern: .git",
		},
	}))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"path": "/home/user/.git"}

	result, err := srv.handleExplainTarget(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var resp ExplainResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != "protected" {
		t.Errorf("expected protected status, got %s", resp.Status)
	}
}

func TestHandleExplainTarget_NilExplainer(t *testing.T) {
	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"path": "/tmp/test"}

	result, err := srv.handleExplainTarget(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	if text != "explain not available" {
		t.Errorf("expected error message, got %s", text)
	}
}

func TestDefaultExplainer_Target(t *testing.T) {
	// Create temp dir with node_modules and package.json
	tmp := t.TempDir()
	nmDir := filepath.Join(tmp, "node_modules")
	if err := os.MkdirAll(nmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{ScanPaths: []string{tmp}}
	bl := safety.NewBlacklist(nil)
	explainer := NewDefaultExplainer(cfg, bl)

	result, err := explainer.Explain(context.Background(), nmDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "target" {
		t.Errorf("expected target, got %s", result.Status)
	}
	if result.Ecosystem != "node" {
		t.Errorf("expected node, got %s", result.Ecosystem)
	}
}

func TestDefaultExplainer_Protected(t *testing.T) {
	cfg := &config.Config{}
	bl := safety.NewBlacklist([]string{".git"})
	explainer := NewDefaultExplainer(cfg, bl)

	result, err := explainer.Explain(context.Background(), "/home/user/project/.git")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "protected" {
		t.Errorf("expected protected, got %s", result.Status)
	}
}

func TestDefaultExplainer_NotTarget(t *testing.T) {
	cfg := &config.Config{ScanPaths: []string{"/home"}}
	bl := safety.NewBlacklist(nil)
	explainer := NewDefaultExplainer(cfg, bl)

	result, err := explainer.Explain(context.Background(), "/home/user/random-dir")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "not_target" {
		t.Errorf("expected not_target, got %s", result.Status)
	}
}
