package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
)

type dockerCleaner struct {
	exec cleaner.CommandExecutor
}

// New creates a Docker cleaner using the default command executor.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a Docker cleaner using the provided CommandExecutor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	return &dockerCleaner{exec: exec}
}

func (c *dockerCleaner) Name() string {
	return "docker"
}

func (c *dockerCleaner) IsInstalled() bool {
	if _, err := c.exec.LookPath("docker"); err == nil {
		return true
	}
	if _, err := c.exec.LookPath("orb"); err == nil {
		return true
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}

	candidates := []string{
		filepath.Join(home, ".docker"),
		filepath.Join(home, "Library", "Containers", "com.docker.docker"),
		filepath.Join(home, "Library", "Group Containers", "group.dev.orbstack"),
		filepath.Join(home, "Library", "Caches", "dev.orbstack.OrbStack"),
		filepath.Join(home, ".orbstack"),
		filepath.Join(home, ".cache", "orbstack"),
	}
	for _, p := range candidates {
		if fs.DirExists(p) {
			return true
		}
	}
	return false
}

type rawSystemDFEntry struct {
	Type        string          `json:"Type"`
	TotalCount  json.RawMessage `json:"TotalCount"`
	Active      json.RawMessage `json:"Active"`
	Size        string          `json:"Size"`
	Reclaimable string          `json:"Reclaimable"`
}

func (c *dockerCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target

	out, err := c.exec.Output(ctx, "docker", "system", "df", "--format", "{{json .}}")
	if err != nil {
		if c.IsInstalled() {
			targets = append(targets, domain.Target{
				ID:            "docker-daemon-status",
				Name:          "Docker daemon (unavailable / stopped)",
				Path:          "docker://daemon/status",
				SizeBytes:     0,
				EligibleBytes: 0,
				TTLEligible:   false,
				ReviewOnly:    true,
				ReviewReason:  domain.ReviewReasonAmbiguous,
				Risk:          domain.RiskLow,
				Category:      "docker-daemon",
				ProviderMetrics: &domain.ProviderMetrics{
					Partial:    true,
					StatusNote: fmt.Sprintf("docker system df failed: %v", err),
				},
			})
		}
	} else {
		parsedTargets, hasMalformed := parseSystemDFOutput(out)
		targets = append(targets, parsedTargets...)

		if hasMalformed {
			targets = append(targets, domain.Target{
				ID:            "docker-daemon-parse-error",
				Name:          "Docker daemon (parse warning)",
				Path:          "docker://daemon/status",
				SizeBytes:     0,
				EligibleBytes: 0,
				TTLEligible:   false,
				ReviewOnly:    true,
				ReviewReason:  domain.ReviewReasonAmbiguous,
				Risk:          domain.RiskLow,
				Category:      "docker-daemon",
				ProviderMetrics: &domain.ProviderMetrics{
					Partial:    true,
					StatusNote: "docker system df returned malformed or unrecognized JSON lines",
				},
			})
		}
	}

	if vmTarget, ok := scanDockerVMDisk(); ok {
		targets = append(targets, vmTarget)
	}

	return targets, nil
}

func scanDockerVMDisk() (domain.Target, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return domain.Target{}, false
	}

	candidates := []string{
		filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data", "vms", "0", "data", "Docker.raw"),
		filepath.Join(home, ".docker", "desktop", "vms", "0", "data", "Docker.raw"),
	}

	for _, p := range candidates {
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			usage, _ := fs.CalcUsage(p)
			size := usage.AllocatedBytes
			if size == 0 {
				size = info.Size()
			}
			apparent := usage.ApparentBytes
			if apparent == 0 {
				apparent = info.Size()
			}

			target := domain.Target{
				ID:            "docker-vm-disk",
				Name:          fmt.Sprintf("Docker Desktop VM Disk (allocated: %s, apparent: %s)", formatBytes(size), formatBytes(apparent)),
				Path:          p,
				SizeBytes:     size,
				EligibleBytes: 0, // D-02: 100% ReviewOnly
				TTLEligible:   false,
				ReviewOnly:    true,
				ReviewReason:  domain.ReviewReasonSharedStore,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				Category:      "docker-vm-disk",
				ProviderMetrics: &domain.ProviderMetrics{
					SizeBytes:      size,
					AllocatedBytes: usage.AllocatedBytes,
					ApparentBytes:  apparent,
					StatusNote:     "physical host VM disk image",
				},
			}
			return target, true
		}
	}

	return domain.Target{}, false
}

func (c *dockerCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.ReviewOnly || t.Category == "docker-vm-disk" || strings.HasPrefix(t.Path, "docker://") {
			continue
		}
	}
	return nil
}

func parseSystemDFOutput(out []byte) ([]domain.Target, bool) {
	var targets []domain.Target
	var hasMalformed bool

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry rawSystemDFEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			hasMalformed = true
			continue
		}

		var id, path, category string
		switch entry.Type {
		case "Images":
			id = "docker-images"
			path = "docker://system-df/images"
			category = "docker-images"
		case "Containers":
			id = "docker-containers"
			path = "docker://system-df/containers"
			category = "docker-containers"
		case "Local Volumes":
			id = "docker-volumes"
			path = "docker://system-df/volumes"
			category = "docker-volumes"
		case "Build Cache":
			id = "docker-build-cache"
			path = "docker://system-df/build-cache"
			category = "docker-build-cache"
		default:
			hasMalformed = true
			continue
		}

		total := parseFlexibleInt(entry.TotalCount)
		active := parseFlexibleInt(entry.Active)
		sizeBytes := parseDockerSize(entry.Size)
		reclaimableBytes := parseDockerReclaimable(entry.Reclaimable)

		name := formatDockerTargetName(entry.Type, total, active, sizeBytes, reclaimableBytes)

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          name,
			Path:          path,
			SizeBytes:     sizeBytes,
			EligibleBytes: 0, // D-02: 100% ReviewOnly
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSharedStore,
			Risk:          domain.RiskLow,
			Category:      category,
			ProviderMetrics: &domain.ProviderMetrics{
				TotalCount:       total,
				ActiveCount:      active,
				SizeBytes:        sizeBytes,
				ReclaimableBytes: reclaimableBytes,
			},
		})
	}

	return targets, hasMalformed
}

func parseFlexibleInt(raw json.RawMessage) int {
	s := strings.Trim(string(raw), "\" \t\r\n")
	if s == "" || s == "null" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		if f, err2 := strconv.ParseFloat(s, 64); err2 == nil {
			return int(f)
		}
		return 0
	}
	return v
}

func parseDockerSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	type unitMult struct {
		suffix string
		mult   float64
	}
	units := []unitMult{
		{"TiB", float64(1 << 40)},
		{"GiB", float64(1 << 30)},
		{"MiB", float64(1 << 20)},
		{"KiB", 1024},
		{"TB", 1e12},
		{"GB", 1e9},
		{"MB", 1e6},
		{"KB", 1000},
		{"kB", 1000},
		{"B", 1},
	}

	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			numStr := strings.TrimSpace(strings.TrimSuffix(s, u.suffix))
			if val, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int64(val * u.mult)
			}
		}
	}
	return 0
}

func parseDockerReclaimable(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if idx := strings.Index(s, "("); idx != -1 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	return parseDockerSize(s)
}

func formatDockerTargetName(typeName string, total, active int, size, reclaimable int64) string {
	return fmt.Sprintf("Docker %s (%d total, %d active, %s, %s reclaimable)",
		typeName, total, active, formatBytes(size), formatBytes(reclaimable))
}

func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.1f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
