package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/gtkpad/devclean/internal/domain"
)

// dockerAPI abstracts Docker SDK operations for testability.
type dockerAPI interface {
	listImages(ctx context.Context, dangling bool) ([]imageInfo, error)
	listVolumes(ctx context.Context) ([]volumeInfo, error)
	diskUsage(ctx context.Context) (buildCacheBytes int64, err error)
	contextInfo(ctx context.Context) (name, endpoint string)
	ping(ctx context.Context) error
}

type imageInfo struct {
	ID      string
	Size    int64
	Created time.Time
	Tags    []string
}

type volumeInfo struct {
	Name    string
	Created time.Time
}

type dockerCleaner struct {
	api         dockerAPI
	contextName string
	endpoint    string
}

// New creates a Docker cleaner using the Docker SDK.
func New() domain.Cleaner {
	return &dockerCleaner{
		api: &sdkAPI{},
	}
}

func (c *dockerCleaner) Name() string { return "docker" }

func (c *dockerCleaner) IsInstalled() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.api.ping(ctx) == nil
}

func (c *dockerCleaner) Scan(rules domain.EcosystemRules) ([]domain.Target, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.contextName, c.endpoint = c.api.contextInfo(ctx)

	var targets []domain.Target

	// Dangling images
	images, err := c.api.listImages(ctx, true)
	if err != nil {
		// Fallback to CLI
		return c.scanCLI(rules)
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	for _, img := range images {
		if rules.MaxAgeDays > 0 && img.Created.After(cutoff) {
			continue
		}
		targets = append(targets, domain.Target{
			ID:           img.ID,
			Name:         formatImageName(img.Tags),
			Path:         "docker://" + img.ID,
			SizeBytes:    img.Size,
			Risk:         classifyImageRisk(img.Created, cutoff),
			LastAccessed: img.Created,
			Category:     "dangling-image",
		})
	}

	// Orphan volumes
	volumes, err := c.api.listVolumes(ctx)
	if err == nil {
		for _, vol := range volumes {
			if rules.MaxAgeDays > 0 && vol.Created.After(cutoff) {
				continue
			}
			targets = append(targets, domain.Target{
				ID:           vol.Name,
				Name:         vol.Name,
				Path:         "docker://volume/" + vol.Name,
				SizeBytes:    0, // volume size not easily obtainable without inspection
				Risk:         classifyVolumeRisk(vol.Created, cutoff),
				LastAccessed: vol.Created,
				Category:     "orphan-volume",
			})
		}
	}

	// Build cache
	cacheSize, err := c.api.diskUsage(ctx)
	if err == nil && cacheSize > 0 {
		targets = append(targets, domain.Target{
			ID:           "build-cache",
			Name:         "Docker Build Cache",
			Path:         "docker://build-cache",
			SizeBytes:    cacheSize,
			Risk:         domain.RiskLow,
			LastAccessed: time.Now(),
			Category:     "build-cache",
		})
	}

	return targets, nil
}

func (c *dockerCleaner) Clean(targets []domain.Target) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return c.cleanCLI(ctx, targets)
	}
	defer cli.Close()

	for _, t := range targets {
		switch t.Category {
		case "dangling-image":
			_, _ = cli.ImageRemove(ctx, t.ID, image.RemoveOptions{Force: true})
		case "orphan-volume":
			_ = cli.VolumeRemove(ctx, t.ID, true)
		case "build-cache":
			_, _ = cli.BuildCachePrune(ctx, types.BuildCachePruneOptions{All: true})
		}
	}
	return nil
}

func (c *dockerCleaner) cleanCLI(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		switch t.Category {
		case "dangling-image":
			_, _ = execCommand(ctx, "docker", "image", "rm", "-f", t.ID)
		case "orphan-volume":
			_, _ = execCommand(ctx, "docker", "volume", "rm", "-f", t.ID)
		case "build-cache":
			_, _ = execCommand(ctx, "docker", "builder", "prune", "-af")
		}
	}
	return nil
}

// scanCLI is the fallback when the SDK fails.
func (c *dockerCleaner) scanCLI(rules domain.EcosystemRules) ([]domain.Target, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// Dangling images via CLI
	out, err := execCommand(ctx, "docker", "image", "ls", "--filter", "dangling=true",
		"--format", "{{.ID}}\t{{.Size}}\t{{.CreatedAt}}")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) < 3 {
				continue
			}
			size := parseDockerSize(parts[1])
			created := parseDockerTime(parts[2])
			if rules.MaxAgeDays > 0 && created.After(cutoff) {
				continue
			}
			targets = append(targets, domain.Target{
				ID:           parts[0],
				Name:         parts[0][:12],
				Path:         "docker://" + parts[0],
				SizeBytes:    size,
				Risk:         classifyImageRisk(created, cutoff),
				LastAccessed: created,
				Category:     "dangling-image",
			})
		}
	}

	// Dangling volumes via CLI
	out, err = execCommand(ctx, "docker", "volume", "ls", "--filter", "dangling=true",
		"--format", "{{.Name}}")
	if err == nil {
		for _, name := range strings.Split(strings.TrimSpace(out), "\n") {
			if name == "" {
				continue
			}
			targets = append(targets, domain.Target{
				ID:           name,
				Name:         name,
				Path:         "docker://volume/" + name,
				SizeBytes:    0,
				Risk:         domain.RiskMedium,
				LastAccessed: time.Now(),
				Category:     "orphan-volume",
			})
		}
	}

	// Build cache via system df
	out, err = execCommand(ctx, "docker", "system", "df", "--format", "{{json .}}")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			var entry struct {
				Type        string `json:"Type"`
				TotalCount  int    `json:"TotalCount"`
				Size        string `json:"Size"`
				Reclaimable string `json:"Reclaimable"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Type == "Build Cache" {
				size := parseDockerSize(entry.Size)
				if size > 0 {
					targets = append(targets, domain.Target{
						ID:           "build-cache",
						Name:         "Docker Build Cache",
						Path:         "docker://build-cache",
						SizeBytes:    size,
						Risk:         domain.RiskLow,
						LastAccessed: time.Now(),
						Category:     "build-cache",
					})
				}
			}
		}
	}

	if len(targets) == 0 && err != nil {
		return nil, fmt.Errorf("docker CLI fallback failed: %w", err)
	}
	return targets, nil
}

func classifyImageRisk(created, cutoff time.Time) domain.RiskLevel {
	if created.Before(cutoff) {
		return domain.RiskLow
	}
	return domain.RiskMedium
}

func classifyVolumeRisk(created, cutoff time.Time) domain.RiskLevel {
	if created.Before(cutoff) {
		return domain.RiskMedium
	}
	return domain.RiskHigh
}

func formatImageName(tags []string) string {
	if len(tags) == 0 || (len(tags) == 1 && tags[0] == "<none>:<none>") {
		return "<dangling>"
	}
	return tags[0]
}

func parseDockerSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	multipliers := map[string]float64{
		"B": 1, "KB": 1000, "MB": 1e6, "GB": 1e9, "TB": 1e12,
		"kB": 1000, "KiB": 1024, "MiB": 1 << 20, "GiB": 1 << 30,
	}

	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSpace(strings.TrimSuffix(s, suffix))
			if val, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int64(val * mult)
			}
		}
	}
	return 0
}

func parseDockerTime(s string) time.Time {
	// Docker format: "2024-01-15 10:30:00 +0000 UTC"
	layouts := []string{
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t
		}
	}
	return time.Time{}
}

func execCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// --- SDK implementation ---

type sdkAPI struct{}

func (s *sdkAPI) ping(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.Ping(ctx)
	return err
}

func (s *sdkAPI) listImages(ctx context.Context, dangling bool) ([]imageInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	opts := image.ListOptions{}
	if dangling {
		opts.Filters = filters.NewArgs(filters.Arg("dangling", "true"))
	}

	images, err := cli.ImageList(ctx, opts)
	if err != nil {
		return nil, err
	}

	var result []imageInfo
	for _, img := range images {
		result = append(result, imageInfo{
			ID:      img.ID,
			Size:    img.Size,
			Created: time.Unix(img.Created, 0),
			Tags:    img.RepoTags,
		})
	}
	return result, nil
}

func (s *sdkAPI) listVolumes(ctx context.Context) ([]volumeInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	// List all volumes
	vols, err := cli.VolumeList(ctx, volume.ListOptions{
		Filters: filters.NewArgs(filters.Arg("dangling", "true")),
	})
	if err != nil {
		return nil, err
	}

	var result []volumeInfo
	for _, vol := range vols.Volumes {
		created, _ := time.Parse(time.RFC3339, vol.CreatedAt)
		result = append(result, volumeInfo{
			Name:    vol.Name,
			Created: created,
		})
	}
	return result, nil
}

func (s *sdkAPI) diskUsage(ctx context.Context) (int64, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return 0, err
	}
	defer cli.Close()

	du, err := cli.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		return 0, err
	}

	var total int64
	if du.BuildCache != nil {
		for _, bc := range du.BuildCache {
			total += bc.Size
		}
	}
	return total, nil
}

func (s *sdkAPI) contextInfo(ctx context.Context) (string, string) {
	// Get active docker context via CLI — SDK doesn't directly expose context info
	out, err := execCommand(ctx, "docker", "context", "inspect", "--format", "{{.Name}} {{.Endpoints.docker.Host}}")
	if err != nil {
		return "default", ""
	}
	parts := strings.SplitN(strings.TrimSpace(out), " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return "default", ""
}
