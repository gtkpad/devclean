package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gabrielvp/devclean/internal/domain"
)

func formatOutput(w io.Writer, results []domain.ScanResult, format string, verbose bool) error {
	switch format {
	case "table":
		formatTable(w, results, verbose)
		return nil
	case "json":
		return formatJSON(w, results)
	default:
		return fmt.Errorf("unknown format: %q (supported: table, json)", format)
	}
}

func formatTable(w io.Writer, results []domain.ScanResult, verbose bool) {
	if totalTargets(results) == 0 {
		fmt.Fprintln(w, "No targets found")
		return
	}

	fmt.Fprintf(w, "%-12s %-50s %10s %8s %12s\n", "ECOSYSTEM", "PATH", "SIZE", "RISK", "LAST ACCESSED")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 96))

	for i, r := range results {
		if r.Error != nil || len(r.Targets) == 0 {
			continue
		}

		if i > 0 {
			fmt.Fprintln(w)
		}

		for _, t := range r.Targets {
			path := t.Path
			if len(path) > 50 {
				path = "..." + path[len(path)-47:]
			}

			lastAccessed := t.LastAccessed.Format("2006-01-02")

			if verbose {
				fmt.Fprintf(w, "%-12s %-50s %10s %8s %12s  [%s] %s\n",
					r.Ecosystem, path, humanSize(t.SizeBytes), string(t.Risk), lastAccessed, t.ID, t.Name)
			} else {
				fmt.Fprintf(w, "%-12s %-50s %10s %8s %12s\n",
					r.Ecosystem, path, humanSize(t.SizeBytes), string(t.Risk), lastAccessed)
			}
		}
	}
}

type jsonEcosystem struct {
	Ecosystem string          `json:"ecosystem"`
	Targets   []domain.Target `json:"targets"`
}

func formatJSON(w io.Writer, results []domain.ScanResult) error {
	var output []jsonEcosystem
	for _, r := range results {
		if r.Error != nil {
			continue
		}
		output = append(output, jsonEcosystem{
			Ecosystem: r.Ecosystem,
			Targets:   r.Targets,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func humanSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}

	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(bytes) / 1024
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}
