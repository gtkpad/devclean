package hooks

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"text/template"
	"time"

	"github.com/gtkpad/devclean/internal/config"
)

// HookData provides template variables for hook commands.
type HookData struct {
	Path        string
	Ecosystem   string
	FreedSize   string
	TargetCount int
}

// Runner executes hooks for a given ecosystem.
type Runner struct {
	hooks config.HooksConfig
}

// New creates a hook runner with the given hooks configuration.
func New(hooks config.HooksConfig) *Runner {
	return &Runner{hooks: hooks}
}

// RunPreDelete executes the pre-delete hook for the given ecosystem.
func (r *Runner) RunPreDelete(ctx context.Context, ecosystem string, data HookData) error {
	h := r.getHookConfig(ecosystem)
	if h.PreDelete == "" {
		return nil
	}
	if !matchesPlatform(h.Platform) {
		return nil
	}
	cmd, err := renderTemplate(h.PreDelete, data)
	if err != nil {
		return fmt.Errorf("hook template error: %w", err)
	}
	return execute(ctx, cmd, h.Timeout)
}

// RunPostDelete executes the post-delete hook for the given ecosystem.
func (r *Runner) RunPostDelete(ctx context.Context, ecosystem string, data HookData) error {
	h := r.getHookConfig(ecosystem)
	if h.PostDelete == "" {
		return nil
	}
	if !matchesPlatform(h.Platform) {
		return nil
	}
	cmd, err := renderTemplate(h.PostDelete, data)
	if err != nil {
		return fmt.Errorf("hook template error: %w", err)
	}
	return execute(ctx, cmd, h.Timeout)
}

func (r *Runner) getHookConfig(ecosystem string) config.HookConfig {
	switch ecosystem {
	case "node":
		return r.hooks.Node
	case "go":
		return r.hooks.Go
	case "docker":
		return r.hooks.Docker
	case "dotnet":
		return r.hooks.DotNet
	case "python":
		return r.hooks.Python
	case "rust":
		return r.hooks.Rust
	case "java":
		return r.hooks.Java
	default:
		return config.HookConfig{}
	}
}

func matchesPlatform(platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}
	for _, p := range platforms {
		if p == runtime.GOOS {
			return true
		}
	}
	return false
}

func renderTemplate(tmpl string, data HookData) (string, error) {
	t, err := template.New("hook").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func execute(ctx context.Context, command string, timeout string) error {
	dur := 30 * time.Second
	if timeout != "" {
		parsed, err := time.ParseDuration(timeout)
		if err == nil {
			dur = parsed
		}
	}

	ctx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("hook timed out after %s: %s", dur, command)
		}
		return fmt.Errorf("hook failed: %w: %s", err, string(output))
	}
	return nil
}
