package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// HistoryReader abstracts history access for testability.
type HistoryReader interface {
	Read() ([]history.Entry, error)
	ReadFiltered(opts history.FilterOpts) ([]history.Entry, error)
}

// ConfigProvider abstracts config loading for testability.
type ConfigProvider interface {
	Config() *config.Config
	ConfigDir() string
}

// Option configures the Server.
type Option func(*Server)

// WithHistory injects a HistoryReader into the server.
func WithHistory(h HistoryReader) Option {
	return func(s *Server) { s.history = h }
}

// WithConfigProvider injects a ConfigProvider into the server.
func WithConfigProvider(cp ConfigProvider) Option {
	return func(s *Server) { s.configProvider = cp }
}

// WithCoordinator injects a deleter.Coordinator into the server.
func WithCoordinator(c *deleter.Coordinator) Option {
	return func(s *Server) { s.coordinator = c }
}

// Server wraps the MCP server with devclean engine access.
type Server struct {
	mcp            *server.MCPServer
	engine         *engine.Engine
	bl             *safety.Blacklist
	coordinator    *deleter.Coordinator
	history        HistoryReader
	configProvider ConfigProvider
	explainer      PathExplainer
	trash          TrashRestorer
}

// New creates a devclean MCP server with all tools registered.
func New(eng *engine.Engine, cfg *config.Config, opts ...Option) *Server {
	var bl *safety.Blacklist
	if cfg != nil {
		bl = safety.NewBlacklist(cfg.ProtectedPatterns)
	} else {
		bl = safety.NewBlacklist(nil)
	}

	s := &Server{
		engine: eng,
		bl:     bl,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.coordinator == nil {
		var trashMgr *trash.Manager
		var hookRunner *hooks.Runner
		if cfg != nil {
			trashMgr = trash.New(cfg.Safety)
			hookRunner = hooks.New(cfg.Hooks)
		}
		var cfgDir string
		if s.configProvider != nil {
			cfgDir = s.configProvider.ConfigDir()
		}
		s.coordinator = deleter.New(eng, cfg, s.bl, trashMgr, hookRunner, cfgDir, "")
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

	statsTool := mcp.NewTool("get_stats",
		mcp.WithDescription("Retrieve cleanup history and statistics with optional filters"),
		mcp.WithString("since", mcp.Description("ISO date string to filter entries after")),
		mcp.WithString("ecosystem", mcp.Description("Filter by ecosystem name")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of history entries to return")),
		mcp.WithBoolean("include_history", mcp.Description("Include individual history entries in response")),
	)
	s.mcp.AddTool(statsTool, s.handleGetStats)

	configTool := mcp.NewTool("get_config",
		mcp.WithDescription("Retrieve current configuration, optionally merged with a named profile"),
		mcp.WithString("profile", mcp.Description("Profile name to merge with root config")),
	)
	s.mcp.AddTool(configTool, s.handleGetConfig)

	explainTool := mcp.NewTool("explain_target",
		mcp.WithDescription("Explain whether a path is a cleanup target, excluded, or protected"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to analyze")),
	)
	s.mcp.AddTool(explainTool, s.handleExplainTarget)

	restoreTool := mcp.NewTool("restore_target",
		mcp.WithDescription("Restore a target from trash by entry ID"),
		mcp.WithString("target_id", mcp.Required(), mcp.Description("Trash entry ID to restore")),
	)
	s.mcp.AddTool(restoreTool, s.handleRestoreTarget)

	listTrashTool := mcp.NewTool("list_trash",
		mcp.WithDescription("List all restorable items currently in trash"),
	)
	s.mcp.AddTool(listTrashTool, s.handleListTrash)
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

	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	byEco := make(map[string][]domain.Target)
	for _, r := range results {
		for _, t := range r.Targets {
			if _, ok := idSet[t.ID]; ok {
				if !s.bl.IsBlacklisted(t.Path) {
					byEco[r.Ecosystem] = append(byEco[r.Ecosystem], t)
				}
			}
		}
	}

	res, err := s.coordinator.Delete(ctx, byEco, deleter.Options{
		Mode: "mcp",
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("deletion failed: %v", err)), nil
	}

	result := map[string]interface{}{
		"deleted_count": res.TotalCount,
		"freed_bytes":   res.TotalFreed,
	}
	if len(res.Errors) > 0 {
		var errStrs []string
		for _, e := range res.Errors {
			errStrs = append(errStrs, e.Error())
		}
		result["errors"] = errStrs
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleGetStats(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.history == nil {
		return mcp.NewToolResultError("history not available"), nil
	}

	var opts history.FilterOpts

	if since := request.GetString("since", ""); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			t, err = time.Parse("2006-01-02", since)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid since date: %v", err)), nil
			}
		}
		opts.Since = t
	}

	if eco := request.GetString("ecosystem", ""); eco != "" {
		opts.Ecosystem = eco
	}

	limit := request.GetInt("limit", 0)
	includeHistory := request.GetBool("include_history", false)

	var entries []history.Entry
	var err error
	if opts.Since.IsZero() && opts.Ecosystem == "" {
		entries, err = s.history.Read()
	} else {
		entries, err = s.history.ReadFiltered(opts)
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reading history: %v", err)), nil
	}

	type ecoStats struct {
		Name            string `json:"name"`
		FreedBytes      int64  `json:"freed_bytes"`
		Runs            int    `json:"runs"`
		TargetsCleaned  int    `json:"targets_cleaned"`
	}

	var totalFreed int64
	perEco := make(map[string]*ecoStats)

	for _, e := range entries {
		totalFreed += e.FreedBytes
		for _, eco := range e.Ecosystems {
			st, ok := perEco[eco]
			if !ok {
				st = &ecoStats{Name: eco}
				perEco[eco] = st
			}
			st.FreedBytes += e.FreedBytes / int64(len(e.Ecosystems))
			st.Runs++
			st.TargetsCleaned += e.Targets / len(e.Ecosystems)
		}
	}

	ecoList := make([]ecoStats, 0, len(perEco))
	for _, st := range perEco {
		ecoList = append(ecoList, *st)
	}

	type statsResponse struct {
		TotalFreedBytes int64          `json:"total_freed_bytes"`
		TotalRuns       int            `json:"total_runs"`
		PerEcosystem    []ecoStats     `json:"per_ecosystem"`
		History         []history.Entry `json:"history,omitempty"`
	}

	resp := statsResponse{
		TotalFreedBytes: totalFreed,
		TotalRuns:       len(entries),
		PerEcosystem:    ecoList,
	}

	if includeHistory {
		h := entries
		if limit > 0 && limit < len(h) {
			h = h[len(h)-limit:]
		}
		resp.History = h
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleGetConfig(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.configProvider == nil {
		return mcp.NewToolResultError("config not available"), nil
	}

	cfg := s.configProvider.Config()

	if profile := request.GetString("profile", ""); profile != "" {
		profileCfg, err := config.LoadProfile(s.configProvider.ConfigDir(), profile)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("loading profile: %v", err)), nil
		}
		cfg = config.MergeProfile(cfg, profileCfg)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
