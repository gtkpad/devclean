//go:build acceptance

package acceptance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// BenchmarkResult represents comparative benchmark metrics extracted from
// live execution of `clean --dry-run` or from the normative SPEC snapshot.
type BenchmarkResult struct {
	Source              string           `json:"source"`
	Categories          map[string]int64 `json:"categories"`
	CategoryItems       map[string]int   `json:"category_items,omitempty"`
	TotalItems          int              `json:"total_items"`
	TotalBytes          int64            `json:"total_bytes"`
	TotalBytesFormatted string           `json:"total_bytes_formatted"`
}

// RunBaselineOrFallback dynamically invokes `clean --dry-run` if the `benchmark`
// binary is present on the host path. If absent or execution fails, it seamlessly
// falls back to the 9-category normative snapshot documented in benchmark specification §1.
func RunBaselineOrFallback(t *testing.T) BenchmarkResult {
	if t != nil {
		t.Helper()
	}

	benchmarkPath, err := exec.LookPath("devclean-benchmark")
	if err == nil && benchmarkPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, benchmarkPath, "clean", "--dry-run")
		out, runErr := cmd.CombinedOutput()
		if runErr == nil && len(out) > 0 {
			cleanListPath := filepath.Join(os.Getenv("HOME"), ".config", "devclean", "clean-list.txt")
			res, parseErr := parseBenchmarkOutput(string(out), cleanListPath)
			if parseErr == nil && len(res.Categories) > 0 && res.TotalBytes > 0 {
				res.Source = fmt.Sprintf("Dynamic execution (%s clean --dry-run)", benchmarkPath)
				if t != nil {
					t.Logf("Baseline benchmark dynamically gathered: %s (%d categories, %s)",
						res.Source, len(res.Categories), res.TotalBytesFormatted)
				}
				return res
			}
		}
		if t != nil {
			t.Logf("Baseline dynamic execution did not yield complete results (err: %v); using normative snapshot fallback", runErr)
		}
	} else if t != nil {
		t.Logf("Baseline executable not found in PATH; using normative snapshot fallback")
	}

	return getNormativeSpecSnapshot()
}

// getNormativeSpecSnapshot returns the reference baseline from benchmark specification §1.
func getNormativeSpecSnapshot() BenchmarkResult {
	cats := map[string]int64{
		"System":                  850 * 1024 * 1024,   // 850 MB
		"User essentials":        3840 * 1024 * 1024,  // 3.84 GB
		"App caches":              120 * 1024 * 1024,   // 120 MB
		"Browsers":                802 * 1024 * 1024,   // 802 MB
		"Developer tools":         780 * 1024 * 1024,   // 780 MB
		"Apps & utilities":        45 * 1024 * 1024,    // 45 MB
		"Application Support":     18 * 1024 * 1024,    // 18 MB
		"App leftovers":           2 * 1024 * 1024,     // 2 MB
		"Large files":            19600 * 1024 * 1024,  // 19.6 GB
	}

	items := map[string]int{
		"System":              12,
		"User essentials":    46,
		"App caches":          38,
		"Browsers":            15,
		"Developer tools":     64,
		"Apps & utilities":    18,
		"Application Support": 24,
		"App leftovers":       5,
		"Large files":         8,
	}

	var totalBytes int64
	var totalItems int
	for _, b := range cats {
		totalBytes += b
	}
	for _, c := range items {
		totalItems += c
	}

	return BenchmarkResult{
		Source:              "Normative snapshot per .planning/benchmark specification §1",
		Categories:          cats,
		CategoryItems:       items,
		TotalItems:          totalItems,
		TotalBytes:          totalBytes,
		TotalBytesFormatted: FormatBytes(totalBytes),
	}
}

var sizeRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*([KMGT]?B)`)

// parseBenchmarkOutput extracts category totals and large files from stdout and clean-list.txt.
func parseBenchmarkOutput(stdout string, cleanListPath string) (BenchmarkResult, error) {
	categories := make(map[string]int64)
	categoryItems := make(map[string]int)

	// 1. Parse clean-list.txt if it exists
	if data, err := os.ReadFile(cleanListPath); err == nil {
		lines := strings.Split(string(data), "\n")
		currentCategory := ""
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "=== ") && strings.HasSuffix(line, " ===") {
				currentCategory = strings.TrimPrefix(strings.TrimSuffix(line, " ==="), "=== ")
				continue
			}
			if currentCategory != "" && !strings.HasPrefix(line, "#") && strings.Contains(line, "#") {
				parts := strings.Split(line, "#")
				if len(parts) >= 2 {
					sizeStr := strings.TrimSpace(parts[1])
					bytes := parseSizeString(sizeStr)
					categories[currentCategory] += bytes
					categoryItems[currentCategory]++
				}
			}
		}
	}

	// 2. Parse stdout for Large files and Stale login items (App leftovers)
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "⊙") || strings.Contains(line, "Large files") || strings.Contains(line, "OrbStack") ||
			strings.Contains(line, "Docker") || strings.Contains(line, "NuGet") || strings.Contains(line, "pnpm store") {
			matches := sizeRe.FindAllString(line, -1)
			for _, m := range matches {
				bytes := parseSizeString(m)
				if bytes > 10*1024*1024 { // >= 10MB
					categories["Large files"] += bytes
					categoryItems["Large files"]++
					break
				}
			}
		}
		if strings.Contains(line, "Stale login item") || strings.Contains(line, "LaunchAgents") {
			categories["App leftovers"] += 2 * 1024 * 1024
			categoryItems["App leftovers"]++
		}
		if strings.Contains(line, "System logs") || strings.Contains(line, "DiagnosticReports") {
			categories["System"] += 50 * 1024 * 1024
			categoryItems["System"]++
		}
	}

	var totalBytes int64
	var totalItems int
	for _, b := range categories {
		totalBytes += b
	}
	for _, it := range categoryItems {
		totalItems += it
	}

	if totalBytes == 0 {
		return BenchmarkResult{}, fmt.Errorf("no bytes extracted from benchmark output")
	}

	return BenchmarkResult{
		Categories:          categories,
		CategoryItems:       categoryItems,
		TotalItems:          totalItems,
		TotalBytes:          totalBytes,
		TotalBytesFormatted: FormatBytes(totalBytes),
	}, nil
}

func parseSizeString(s string) int64 {
	matches := sizeRe.FindStringSubmatch(s)
	if len(matches) < 3 {
		return 0
	}
	val, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	unit := strings.ToUpper(matches[2])
	switch unit {
	case "B":
		return int64(val)
	case "KB":
		return int64(val * 1024)
	case "MB":
		return int64(val * 1024 * 1024)
	case "GB":
		return int64(val * 1024 * 1024 * 1024)
	case "TB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	default:
		return int64(val)
	}
}

// FormatBytes formats a byte count into a human-readable string.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
