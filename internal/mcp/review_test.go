package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCP_ReviewOnly_Propose(t *testing.T) {
	cfg := &config.Config{
		Ecosystems: config.EcosystemsConfig{
			Docker: config.EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue"},
		},
	}
	eng := engine.New(cfg, &fakeResolver{})

	cleaner := &fakeCleaner{
		name:      "docker",
		installed: true,
		targets: []domain.Target{
			{
				ID:            "docker-shared-store",
				Name:          "Docker Storage",
				Path:          "docker://storage",
				SizeBytes:     4096000,
				EligibleBytes: 0,
				ReviewOnly:    true,
				ReviewReason:  domain.ReviewReasonSharedStore,
			},
			{
				ID:            "docker-dangling-img",
				Name:          "Dangling Image",
				Path:          "docker://sha256:dangling",
				SizeBytes:     2048000,
				EligibleBytes: 2048000,
				ReviewOnly:    false,
			},
		},
	}
	eng.Register(cleaner)
	srv := New(eng, cfg)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"target_ids": `["docker-shared-store", "docker-dangling-img"]`,
	}

	result, err := srv.handlePropose(context.Background(), req)
	if err != nil {
		t.Fatalf("handlePropose failed: %v", err)
	}

	text := extractText(t, result)
	var entries []struct {
		ID            string `json:"id"`
		Path          string `json:"path"`
		Size          int64  `json:"size_bytes"`
		EligibleBytes int64  `json:"eligible_bytes"`
		Status        string `json:"status"`
		ReviewReason  string `json:"review_reason"`
		Reason        string `json:"reason"`
	}

	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		t.Fatalf("failed to unmarshal propose response: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	entryMap := make(map[string]struct {
		ID            string `json:"id"`
		Path          string `json:"path"`
		Size          int64  `json:"size_bytes"`
		EligibleBytes int64  `json:"eligible_bytes"`
		Status        string `json:"status"`
		ReviewReason  string `json:"review_reason"`
		Reason        string `json:"reason"`
	})
	for _, e := range entries {
		entryMap[e.ID] = e
	}

	// 1. Assert ReviewOnly target
	storeEntry, ok := entryMap["docker-shared-store"]
	if !ok {
		t.Fatal("missing docker-shared-store in propose output")
	}
	if storeEntry.Status != "review_only" {
		t.Errorf("docker-shared-store Status = %q, want %q", storeEntry.Status, "review_only")
	}
	if storeEntry.EligibleBytes != 0 {
		t.Errorf("docker-shared-store EligibleBytes = %d, want 0", storeEntry.EligibleBytes)
	}
	if storeEntry.ReviewReason != domain.ReviewReasonSharedStore {
		t.Errorf("docker-shared-store ReviewReason = %q, want %q", storeEntry.ReviewReason, domain.ReviewReasonSharedStore)
	}
	if storeEntry.Size != 4096000 {
		t.Errorf("docker-shared-store Size = %d, want 4096000", storeEntry.Size)
	}
	if !strings.Contains(storeEntry.Reason, "manual") && !strings.Contains(storeEntry.Reason, "dedicada") {
		t.Errorf("docker-shared-store Reason = %q, want explanation noting manual/dedicated tool action", storeEntry.Reason)
	}

	// 2. Assert normal approved target
	imgEntry, ok := entryMap["docker-dangling-img"]
	if !ok {
		t.Fatal("missing docker-dangling-img in propose output")
	}
	if imgEntry.Status != "approved" {
		t.Errorf("docker-dangling-img Status = %q, want %q", imgEntry.Status, "approved")
	}
	if imgEntry.EligibleBytes != 2048000 {
		t.Errorf("docker-dangling-img EligibleBytes = %d, want 2048000", imgEntry.EligibleBytes)
	}
}

func TestMCP_ReviewOnly_Scan(t *testing.T) {
	cfg := &config.Config{
		Ecosystems: config.EcosystemsConfig{
			Docker: config.EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue"},
		},
	}
	eng := engine.New(cfg, &fakeResolver{})

	cleaner := &fakeCleaner{
		name:      "docker",
		installed: true,
		targets: []domain.Target{
			{
				ID:            "docker-shared-store",
				Name:          "Docker Storage",
				Path:          "docker://storage",
				SizeBytes:     8192000,
				EligibleBytes: 0,
				ReviewOnly:    true,
				ReviewReason:  domain.ReviewReasonSharedStore,
			},
		},
	}
	eng.Register(cleaner)
	srv := New(eng, cfg)

	result, err := srv.handleScan(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleScan failed: %v", err)
	}

	text := extractText(t, result)
	var output []struct {
		Ecosystem string `json:"ecosystem"`
		Targets   []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			Path          string `json:"path"`
			SizeBytes     int64  `json:"size_bytes"`
			EligibleBytes int64  `json:"eligible_bytes"`
			ReviewOnly    bool   `json:"review_only"`
			ReviewReason  string `json:"review_reason"`
		} `json:"targets"`
	}

	if err := json.Unmarshal([]byte(text), &output); err != nil {
		t.Fatalf("failed to unmarshal scan response: %v", err)
	}

	if len(output) != 1 || len(output[0].Targets) != 1 {
		t.Fatalf("unexpected scan results: %+v", output)
	}

	tgt := output[0].Targets[0]
	if !tgt.ReviewOnly {
		t.Errorf("expected review_only = true in scan output, got false")
	}
	if tgt.ReviewReason != domain.ReviewReasonSharedStore {
		t.Errorf("expected review_reason = %q in scan output, got %q", domain.ReviewReasonSharedStore, tgt.ReviewReason)
	}
	if tgt.EligibleBytes != 0 {
		t.Errorf("expected eligible_bytes = 0, got %d", tgt.EligibleBytes)
	}
}

func TestMCP_ReviewOnly_Explain(t *testing.T) {
	cfg := &config.Config{
		ScanPaths: []string{"/tmp/workspace"},
	}
	eng := engine.New(cfg, &fakeResolver{})

	cleaner := &fakeCleaner{
		name:      "pnpm",
		installed: true,
		targets: []domain.Target{
			{
				ID:            "pnpm-store-1",
				Name:          "pnpm store",
				Path:          "/tmp/workspace/.pnpm-store",
				SizeBytes:     5000000,
				EligibleBytes: 0,
				ReviewOnly:    true,
				ReviewReason:  domain.ReviewReasonSharedStore,
			},
		},
	}
	eng.Register(cleaner)

	bl := safety.NewBlacklist(nil)
	explainer := NewDefaultExplainer(cfg, bl)
	srv := New(eng, cfg, WithExplainer(explainer))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"path": "/tmp/workspace/.pnpm-store",
	}

	result, err := srv.handleExplainTarget(context.Background(), req)
	if err != nil {
		t.Fatalf("handleExplainTarget failed: %v", err)
	}

	text := extractText(t, result)
	var resp struct {
		Path         string `json:"path"`
		Status       string `json:"status"`
		Reason       string `json:"reason"`
		ReviewOnly   bool   `json:"review_only"`
		ReviewReason string `json:"review_reason"`
	}

	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to unmarshal explain response: %v", err)
	}

	if resp.Status != "review_only" {
		t.Errorf("Status = %q, want %q", resp.Status, "review_only")
	}
	if !resp.ReviewOnly {
		t.Errorf("ReviewOnly = false, want true")
	}
	if resp.ReviewReason != domain.ReviewReasonSharedStore {
		t.Errorf("ReviewReason = %q, want %q", resp.ReviewReason, domain.ReviewReasonSharedStore)
	}
	if !strings.Contains(resp.Reason, "manual") && !strings.Contains(resp.Reason, "dedicada") {
		t.Errorf("Reason = %q, want explanation referencing manual or dedicated native tool action", resp.Reason)
	}
}

func TestMCP_ReviewOnly_ExecuteRejection(t *testing.T) {
	cfg := &config.Config{
		Ecosystems: config.EcosystemsConfig{
			Docker: config.EcosystemConfig{Enabled: true, MaxAgeDays: 30, FailMode: "continue"},
		},
	}
	eng := engine.New(cfg, &fakeResolver{})

	cleaner := &fakeCleaner{
		name:      "docker",
		installed: true,
		targets: []domain.Target{
			{
				ID:            "docker-shared-1",
				Name:          "Docker System Store",
				Path:          "docker://system-store",
				SizeBytes:     10000000,
				EligibleBytes: 0,
				ReviewOnly:    true,
				ReviewReason:  domain.ReviewReasonSharedStore,
			},
			{
				ID:            "docker-dangling-1",
				Name:          "Dangling Image",
				Path:          "docker://dangling-img",
				SizeBytes:     3000000,
				EligibleBytes: 3000000,
				ReviewOnly:    false,
			},
		},
	}
	eng.Register(cleaner)

	bl := safety.NewBlacklist(nil)
	coord := deleter.New(eng, cfg, bl, nil, nil, "", "")
	srv := New(eng, cfg, WithCoordinator(coord))

	// 1. Mixed batch: request both ReviewOnly and normal target
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"target_ids": `["docker-shared-1", "docker-dangling-1"]`,
	}

	result, err := srv.handleExecute(context.Background(), req)
	if err != nil {
		t.Fatalf("handleExecute failed: %v", err)
	}

	text := extractText(t, result)
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if int(out["deleted_count"].(float64)) != 1 {
		t.Errorf("expected 1 deleted, got %v", out["deleted_count"])
	}
	if int(out["freed_bytes"].(float64)) != 3000000 {
		t.Errorf("expected 3000000 freed, got %v", out["freed_bytes"])
	}

	// Verify cleaner was called ONLY for dangling target and NEVER for shared store
	if len(cleaner.cleaned) != 1 || cleaner.cleaned[0].ID != "docker-dangling-1" {
		t.Errorf("cleaner executed wrong targets: %+v", cleaner.cleaned)
	}

	// 2. 100% ReviewOnly batch: request only ReviewOnly target
	cleaner.cleaned = nil
	reqOnlyReview := mcp.CallToolRequest{}
	reqOnlyReview.Params.Arguments = map[string]interface{}{
		"target_ids": `["docker-shared-1"]`,
	}

	resReview, err := srv.handleExecute(context.Background(), reqOnlyReview)
	if err != nil {
		t.Fatalf("handleExecute on 100%% review batch failed: %v", err)
	}

	textReview := extractText(t, resReview)
	var outReview map[string]interface{}
	if err := json.Unmarshal([]byte(textReview), &outReview); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if int(outReview["deleted_count"].(float64)) != 0 {
		t.Errorf("expected 0 deleted, got %v", outReview["deleted_count"])
	}
	if int(outReview["freed_bytes"].(float64)) != 0 {
		t.Errorf("expected 0 freed, got %v", outReview["freed_bytes"])
	}
	if len(cleaner.cleaned) != 0 {
		t.Errorf("expected 0 cleaned targets, got %d", len(cleaner.cleaned))
	}
}
