package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/spf13/cobra"
)

type cleanExecutionResult struct {
	ExitCode     int
	Output       string
	TargetsCount int
	BytesFreed   int64
	FreedHuman   string
	Status       string
	Err          error
}

var (
	reFreed   = regexp.MustCompile(`(?i)(?:Espaço Recuperado:|Total Freed:)\s*([0-9.]+\s*[A-Za-z]+)`)
	reTargets = regexp.MustCompile(`(?i)(?:Alvos Processados:|Total Targets:)\s*(\d+)`)
)

func parseCleanOutput(output string, exitCode int, runErr error) cleanExecutionResult {
	res := cleanExecutionResult{
		ExitCode:   exitCode,
		Output:     output,
		FreedHuman: "0 B",
		Err:        runErr,
	}

	if match := reFreed.FindStringSubmatch(output); len(match) > 1 {
		res.FreedHuman = strings.TrimSpace(match[1])
		res.BytesFreed = scheduler.ParseFreedBytes(res.FreedHuman)
	}

	if match := reTargets.FindStringSubmatch(output); len(match) > 1 {
		if c, err := strconv.Atoi(match[1]); err == nil {
			res.TargetsCount = c
		}
	}

	lower := strings.ToLower(output)
	switch {
	case exitCode != 0 || runErr != nil:
		res.Status = "FAIL"
	case strings.Contains(lower, "permission denied") ||
		strings.Contains(lower, "error:") ||
		strings.Contains(output, "[ERROR]"):
		res.Status = "PARTIAL"
	default:
		res.Status = "OK"
	}

	return res
}

var runCleanSubprocess = func(ctx context.Context, configDir string, args []string, logWriter io.Writer) cleanExecutionResult {
	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = "devclean"
	}

	cmdArgs := append([]string{"clean", "--auto"}, args...)
	execCmd := exec.CommandContext(ctx, binaryPath, cmdArgs...)

	var captured bytes.Buffer
	multiWriter := io.MultiWriter(logWriter, &captured)
	execCmd.Stdout = multiWriter
	execCmd.Stderr = multiWriter

	cmdErr := execCmd.Run()
	exitCode := 0
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	outStr := captured.String()
	return parseCleanOutput(outStr, exitCode, cmdErr)
}

func newScheduleRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "run",
		Hidden: true,
		Short:  "Execute automated scheduled cleanup cycle",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			resolver := platform.New()
			paths := resolver.Resolve()

			logDir := paths.LogDir
			if logDir == "" {
				home, _ := os.UserHomeDir()
				logDir = filepath.Join(home, ".local", "state", "devclean", "logs")
			}
			if err := os.MkdirAll(logDir, 0755); err != nil {
				return fmt.Errorf("creating log directory: %w", err)
			}

			logPath := filepath.Join(logDir, "schedule.log")

			// 1. Preventive rotation before run
			_ = scheduler.RotateIfNeeded(logPath, scheduler.DefaultMaxLogSizeBytes, scheduler.DefaultMaxBackups)

			// 2. Symlink protection (T-03-06)
			if fi, err := os.Lstat(logPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
				_ = os.Remove(logPath)
			}

			// 3. Open log in append mode
			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return fmt.Errorf("opening schedule log: %w", err)
			}
			defer logFile.Close()

			startTime := time.Now()

			// 4. Load config for scheduler.args
			configDir := paths.ConfigDir
			configFile := filepath.Join(configDir, "config.yaml")
			var cleanArgs []string
			if cfg, err := config.Load(configFile); err == nil && cfg != nil {
				cleanArgs = cfg.Scheduler.Args
			}

			// 5. Write START header (D-11)
			vStr := version
			if vStr == "" || vStr == "dev" {
				vStr = "v2.2"
			}
			fmt.Fprintf(logFile, "=== [START] %s | version=%s | args=[%s] ===\n",
				startTime.Format("2006-01-02 15:04:05"),
				vStr,
				strings.Join(cleanArgs, " "),
			)

			// 6. Execute cleanup subprocess
			runRes := runCleanSubprocess(ctx, configDir, cleanArgs, logFile)

			endTime := time.Now()
			duration := endTime.Sub(startTime)
			durationStr := fmt.Sprintf("%.1fs", duration.Seconds())

			// 7. Write END footer (D-11)
			fmt.Fprintf(logFile, "=== [END] %s | status=%s | targets=%d | freed=%s | duration=%s ===\n",
				endTime.Format("2006-01-02 15:04:05"),
				runRes.Status,
				runRes.TargetsCount,
				runRes.FreedHuman,
				durationStr,
			)

			// 8. Update last-run.json (D-12)
			_ = scheduler.WriteLastRun(configDir, scheduler.RunResult{
				Timestamp:    startTime,
				TargetsCount: runRes.TargetsCount,
				BytesFreed:   runRes.BytesFreed,
				Duration:     durationStr,
			})

			// 9. Post-execution rotation if limit exceeded
			_ = scheduler.RotateIfNeeded(logPath, scheduler.DefaultMaxLogSizeBytes, scheduler.DefaultMaxBackups)

			return nil
		},
	}
}
