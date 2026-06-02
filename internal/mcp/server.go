package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with devclean engine access.
type Server struct {
	mcp    *server.MCPServer
	engine *engine.Engine
	bl     *safety.Blacklist
}

// New creates a devclean MCP server with all tools registered.
func New(eng *engine.Engine, cfg *config.Config) *Server {
	s := &Server{
		engine: eng,
		bl:     safety.NewBlacklist(cfg.ProtectedPatterns),
	}

	s.mcp = server.NewMCPServer(
		"devclean",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	s.registerTools()
	return s
}

// ServeStdio starts the MCP server on stdin/stdout.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcp)
}

func (s *Server) registerTools() {
	scanTool := mcp.NewTool("scan_environment",
		mcp.WithDescription("Scan the development environment for cleanable targets across all ecosystems (Docker, Go, Node, .NET, Python)"),
	)
	s.mcp.AddTool(scanTool, s.handleScan)

	proposeTool := mcp.NewTool("propose_deletion",
		mcp.WithDescription("Validate target IDs against safety rules and return a deletion preview"),
		mcp.WithString("target_ids",
			mcp.Required(),
			mcp.Description("JSON array of target IDs to propose for deletion"),
		),
	)
	s.mcp.AddTool(proposeTool, s.handlePropose)

	executeTool := mcp.NewTool("execute_approved_deletion",
		mcp.WithDescription("Execute deletion of previously proposed and approved targets"),
		mcp.WithString("target_ids",
			mcp.Required(),
			mcp.Description("JSON array of approved target IDs to delete"),
		),
	)
	s.mcp.AddTool(executeTool, s.handleExecute)
}

func (s *Server) handleScan(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ch := s.engine.Scan(ctx)
	results := engine.Collect(ctx, ch)

	type scanOutput struct {
		Ecosystem string          `json:"ecosystem"`
		Targets   []domain.Target `json:"targets,omitempty"`
		Error     string          `json:"error,omitempty"`
	}

	var output []scanOutput
	for _, r := range results {
		entry := scanOutput{Ecosystem: r.Ecosystem}
		if r.Error != nil {
			entry.Error = r.Error.Error()
		} else {
			entry.Targets = r.Targets
		}
		output = append(output, entry)
	}

	data, err := json.Marshal(output)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handlePropose(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idsJSON, err := request.RequireString("target_ids")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var ids []string
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid target_ids JSON: %v", err)), nil
	}

	// Scan to get current targets
	ch := s.engine.Scan(ctx)
	results := engine.Collect(ctx, ch)

	// Build lookup
	targetMap := make(map[string]domain.Target)
	for _, r := range results {
		for _, t := range r.Targets {
			targetMap[t.ID] = t
		}
	}

	type proposalEntry struct {
		ID       string `json:"id"`
		Path     string `json:"path"`
		Size     int64  `json:"size_bytes"`
		Status   string `json:"status"` // "approved", "blocked", "not_found"
		Reason   string `json:"reason,omitempty"`
	}

	var entries []proposalEntry
	for _, id := range ids {
		t, found := targetMap[id]
		if !found {
			entries = append(entries, proposalEntry{ID: id, Status: "not_found", Reason: "target not found in current scan"})
			continue
		}
		if s.bl.IsBlacklisted(t.Path) {
			entries = append(entries, proposalEntry{ID: id, Path: t.Path, Size: t.SizeBytes, Status: "blocked", Reason: "path is protected by safety blacklist"})
			continue
		}
		entries = append(entries, proposalEntry{ID: id, Path: t.Path, Size: t.SizeBytes, Status: "approved"})
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleExecute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idsJSON, err := request.RequireString("target_ids")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var ids []string
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid target_ids JSON: %v", err)), nil
	}

	// Scan to find targets and their ecosystems
	ch := s.engine.Scan(ctx)
	results := engine.Collect(ctx, ch)

	type ecoTarget struct {
		ecosystem string
		target    domain.Target
	}

	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	var toDelete []ecoTarget
	for _, r := range results {
		for _, t := range r.Targets {
			if _, ok := idSet[t.ID]; ok {
				if !s.bl.IsBlacklisted(t.Path) {
					toDelete = append(toDelete, ecoTarget{ecosystem: r.Ecosystem, target: t})
				}
			}
		}
	}

	// Group by ecosystem and clean
	byEco := make(map[string][]domain.Target)
	for _, et := range toDelete {
		byEco[et.ecosystem] = append(byEco[et.ecosystem], et.target)
	}

	var totalFreed int64
	var totalCount int
	var errors []string

	for eco, targets := range byEco {
		cleaner := s.engine.GetCleaner(eco)
		if cleaner == nil {
			errors = append(errors, fmt.Sprintf("no cleaner for %s", eco))
			continue
		}
		if err := cleaner.Clean(ctx, targets); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", eco, err))
			continue
		}
		for _, t := range targets {
			totalFreed += t.SizeBytes
			totalCount++
		}
	}

	result := map[string]interface{}{
		"deleted_count": totalCount,
		"freed_bytes":   totalFreed,
	}
	if len(errors) > 0 {
		result["errors"] = errors
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
