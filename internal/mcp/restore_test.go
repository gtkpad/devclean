package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/mark3labs/mcp-go/mcp"
)

type fakeTrashRestorer struct {
	manifestPath string
	undoErr      error
}

func (f *fakeTrashRestorer) UndoEntry(ctx context.Context, entryID string) error {
	return f.undoErr
}

func (f *fakeTrashRestorer) ManifestPath() string {
	return f.manifestPath
}

func setupTrashManifest(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	manifestPath := filepath.Join(tmp, "manifest.json")

	manifest := &trash.Manifest{
		Batches: []trash.Batch{
			{
				ID:        "batch-1",
				CreatedAt: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
				TotalSize: 1024,
				Entries: []trash.Entry{
					{
						ID:           "entry-1",
						BatchID:      "batch-1",
						OriginalPath: "/home/user/project/node_modules",
						SizeBytes:    1024,
						Ecosystem:    "node",
						MovedAt:      time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return manifestPath
}

func TestHandleRestoreTarget_Success(t *testing.T) {
	manifestPath := setupTrashManifest(t)

	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithTrash(&fakeTrashRestorer{
		manifestPath: manifestPath,
		undoErr:      nil,
	}))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"target_id": "entry-1"}

	result, err := srv.handleRestoreTarget(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}
	if resp["restored_path"] != "/home/user/project/node_modules" {
		t.Errorf("unexpected restored_path: %v", resp["restored_path"])
	}
}

func TestHandleRestoreTarget_NotFound(t *testing.T) {
	manifestPath := setupTrashManifest(t)

	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithTrash(&fakeTrashRestorer{
		manifestPath: manifestPath,
	}))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"target_id": "nonexistent"}

	result, err := srv.handleRestoreTarget(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["success"] != false {
		t.Errorf("expected success=false, got %v", resp["success"])
	}
	if resp["error_code"] != "not_found" {
		t.Errorf("expected not_found error code, got %v", resp["error_code"])
	}
}

func TestHandleRestoreTarget_PathConflict(t *testing.T) {
	manifestPath := setupTrashManifest(t)

	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithTrash(&fakeTrashRestorer{
		manifestPath: manifestPath,
		undoErr:      errors.New("path conflict: file already exists"),
	}))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"target_id": "entry-1"}

	result, err := srv.handleRestoreTarget(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["success"] != false {
		t.Errorf("expected success=false, got %v", resp["success"])
	}
	if resp["error_code"] != "path_conflict" {
		t.Errorf("expected path_conflict, got %v", resp["error_code"])
	}
}

func TestHandleListTrash_ReturnsEntries(t *testing.T) {
	manifestPath := setupTrashManifest(t)

	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg, WithTrash(&fakeTrashRestorer{
		manifestPath: manifestPath,
	}))

	req := mcp.CallToolRequest{}
	result, err := srv.handleListTrash(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var entries []struct {
		ID           string `json:"id"`
		OriginalPath string `json:"original_path"`
		Ecosystem    string `json:"ecosystem"`
	}
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != "entry-1" {
		t.Errorf("expected entry-1, got %s", entries[0].ID)
	}
	if entries[0].Ecosystem != "node" {
		t.Errorf("expected node ecosystem, got %s", entries[0].Ecosystem)
	}
}

func TestHandleListTrash_NilTrash(t *testing.T) {
	cfg := &config.Config{}
	eng := engine.New(cfg, &fakeResolver{})
	srv := New(eng, cfg)

	req := mcp.CallToolRequest{}
	result, err := srv.handleListTrash(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	if text != "trash not available" {
		t.Errorf("expected error message, got %s", text)
	}
}
