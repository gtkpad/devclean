package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/mark3labs/mcp-go/mcp"
)

// PathExplainer provides path explanation logic.
type PathExplainer interface {
	Explain(ctx context.Context, path string) (*ExplainResult, error)
}

// ExplainResult is the structured response for explain_target.
type ExplainResult struct {
	Path      string   `json:"path"`
	Status    string   `json:"status"`
	Ecosystem string   `json:"ecosystem,omitempty"`
	Reason    string   `json:"reason"`
	SizeBytes int64    `json:"size_bytes,omitempty"`
	AgeDays   int      `json:"age_days,omitempty"`
	RiskLevel string   `json:"risk_level,omitempty"`
	Score     *float64 `json:"score,omitempty"`
}

// WithExplainer injects a PathExplainer into the server.
func WithExplainer(e PathExplainer) Option {
	return func(s *Server) { s.explainer = e }
}

// targetPatterns maps directory basenames to their ecosystem.
var targetPatterns = map[string]string{
	"node_modules": "node",
	"target":       "rust",
	"build":        "java",
	"__pycache__":  "python",
	".venv":        "python",
	"bin":          "dotnet",
	"obj":          "dotnet",
}

// manifestForEcosystem maps ecosystem to the manifest file that must be a sibling.
var manifestForEcosystem = map[string][]string{
	"node":   {"package.json"},
	"rust":   {"Cargo.toml"},
	"java":   {"pom.xml", "build.gradle"},
	"python": {"requirements.txt", "pyproject.toml", "setup.py"},
	"dotnet": {"*.csproj", "*.sln"},
}

// DefaultExplainer implements PathExplainer using config and blacklist.
type DefaultExplainer struct {
	bl  *safety.Blacklist
	cfg *config.Config
}

// NewDefaultExplainer creates a DefaultExplainer.
func NewDefaultExplainer(cfg *config.Config, bl *safety.Blacklist) *DefaultExplainer {
	return &DefaultExplainer{cfg: cfg, bl: bl}
}

func (e *DefaultExplainer) Explain(ctx context.Context, path string) (*ExplainResult, error) {
	result := &ExplainResult{Path: path}

	// Check blacklist
	if e.bl != nil && e.bl.IsBlacklisted(path) {
		result.Status = "protected"
		result.Reason = "Protected by pattern: " + filepath.Base(path)
		return result, nil
	}

	// Check exclude paths
	for _, pattern := range e.cfg.ExcludePaths {
		if matched, _ := filepath.Match(pattern, path); matched {
			result.Status = "excluded"
			result.Reason = "Excluded by pattern: " + pattern
			return result, nil
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			result.Status = "excluded"
			result.Reason = "Excluded by pattern: " + pattern
			return result, nil
		}
	}

	// Check target patterns
	base := filepath.Base(path)
	if eco, ok := targetPatterns[base]; ok {
		if hasSiblingManifest(path, eco) {
			result.Status = "target"
			result.Ecosystem = eco
			result.Reason = base + "/ with sibling manifest"
			result.SizeBytes, _ = fs.CalcDirSize(path)
			result.AgeDays = calcAgeDays(path)
			return result, nil
		}
	}

	// Check Go module cache
	if strings.Contains(path, "go/pkg/mod") {
		result.Status = "target"
		result.Ecosystem = "go"
		result.Reason = "Go module cache"
		result.SizeBytes, _ = fs.CalcDirSize(path)
		return result, nil
	}

	// Check scan_paths
	inScanPath := len(e.cfg.ScanPaths) == 0
	for _, sp := range e.cfg.ScanPaths {
		if strings.HasPrefix(path, sp) {
			inScanPath = true
			break
		}
	}

	if !inScanPath {
		result.Status = "not_target"
		result.Reason = "Path not in scan_paths"
		return result, nil
	}

	result.Status = "not_target"
	result.Reason = "Not recognized as a cleanup target by any ecosystem"
	return result, nil
}

func hasSiblingManifest(path, ecosystem string) bool {
	parent := filepath.Dir(path)
	manifests, ok := manifestForEcosystem[ecosystem]
	if !ok {
		return false
	}
	for _, m := range manifests {
		if strings.Contains(m, "*") {
			matches, _ := filepath.Glob(filepath.Join(parent, m))
			if len(matches) > 0 {
				return true
			}
		} else {
			if _, err := os.Stat(filepath.Join(parent, m)); err == nil {
				return true
			}
		}
	}
	return false
}

func calcAgeDays(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return int(time.Since(info.ModTime()).Hours() / 24)
}

func (s *Server) handleExplainTarget(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.explainer == nil {
		return mcp.NewToolResultError("explain not available"), nil
	}

	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := s.explainer.Explain(ctx, path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("explain error: %v", err)), nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
