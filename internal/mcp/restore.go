package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gtkpad/devclean/internal/trash"
	"github.com/mark3labs/mcp-go/mcp"
)

// TrashRestorer abstracts trash restore operations for testability.
type TrashRestorer interface {
	UndoEntry(ctx context.Context, entryID string) error
	ManifestPath() string
}

// WithTrash injects a TrashRestorer into the server.
func WithTrash(t TrashRestorer) Option {
	return func(s *Server) { s.trash = t }
}

func (s *Server) handleRestoreTarget(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.trash == nil {
		return mcp.NewToolResultError("trash not available"), nil
	}

	targetID, err := request.RequireString("target_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Load manifest to get the entry's original path before restoring
	manifest, err := trash.LoadManifest(s.trash.ManifestPath())
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("loading manifest: %v", err)), nil
	}

	entry, _ := manifest.FindEntry(targetID)
	if entry == nil {
		data, _ := json.Marshal(map[string]interface{}{
			"success":      false,
			"error_code":   "not_found",
			"error_detail": fmt.Sprintf("entry %s not found in trash", targetID),
		})
		return mcp.NewToolResultText(string(data)), nil
	}

	originalPath := entry.OriginalPath

	if err := s.trash.UndoEntry(ctx, targetID); err != nil {
		errCode := "unknown"
		errDetail := err.Error()

		switch {
		case strings.Contains(errDetail, "path conflict") || strings.Contains(errDetail, "already exists"):
			errCode = "path_conflict"
		case strings.Contains(errDetail, "symlink"):
			errCode = "symlink_escape"
		case strings.Contains(errDetail, "not found"):
			errCode = "not_found"
		case strings.Contains(errDetail, "expired") || strings.Contains(errDetail, "purged"):
			errCode = "expired"
		}

		data, _ := json.Marshal(map[string]interface{}{
			"success":      false,
			"error_code":   errCode,
			"error_detail": errDetail,
		})
		return mcp.NewToolResultText(string(data)), nil
	}

	data, _ := json.Marshal(map[string]interface{}{
		"success":       true,
		"restored_path": originalPath,
	})
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleListTrash(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.trash == nil {
		return mcp.NewToolResultError("trash not available"), nil
	}

	manifest, err := trash.LoadManifest(s.trash.ManifestPath())
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("loading manifest: %v", err)), nil
	}

	type trashEntry struct {
		ID           string `json:"id"`
		BatchID      string `json:"batch_id"`
		OriginalPath string `json:"original_path"`
		SizeBytes    int64  `json:"size_bytes"`
		Ecosystem    string `json:"ecosystem"`
		MovedAt      string `json:"moved_at"`
	}

	var entries []trashEntry
	for _, batch := range manifest.Batches {
		for _, e := range batch.Entries {
			entries = append(entries, trashEntry{
				ID:           e.ID,
				BatchID:      e.BatchID,
				OriginalPath: e.OriginalPath,
				SizeBytes:    e.SizeBytes,
				Ecosystem:    e.Ecosystem,
				MovedAt:      e.MovedAt.Format("2006-01-02T15:04:05Z"),
			})
		}
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
