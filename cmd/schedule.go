package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newScheduleCmd() *cobra.Command {
	var daily bool
	var cron string
	var status bool
	var watchMode bool

	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Register native OS scheduler for automated cleanup",
		Long:  "Registers a recurring devclean execution using launchd (macOS) or crontab (Linux).",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolver := platform.New()
			paths := resolver.Resolve()
			configDir := paths.ConfigDir

			if status {
				return showStatus(configDir, watchMode)
			}

			if watchMode {
				return installWatch(configDir)
			}

			if daily && cron != "" {
				return fmt.Errorf("cannot use both --daily and --cron flags")
			}
			if !daily && cron == "" {
				return fmt.Errorf("specify --daily or --cron <expression>")
			}

			cronExpr := cron
			if daily {
				cronExpr = "0 12 * * *"
			}

			if err := scheduler.ValidateCron(cronExpr); err != nil {
				return fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
			}

			// Pre-registration dry-run validation
			self, err := os.Executable()
			if err == nil {
				dryRun := exec.Command(self, "clean", "--auto", "--dry-run")
				if out, runErr := dryRun.CombinedOutput(); runErr != nil {
					return fmt.Errorf("config validation failed (dry-run): %s", strings.TrimSpace(string(out)))
				}
			}

			sched := scheduler.New(configDir)

			// Conflict detection
			if sched.IsInstalled() {
				if term.IsTerminal(int(os.Stdin.Fd())) {
					fmt.Fprint(os.Stdout, "Schedule already exists. Overwrite? [y/N] ")
					reader := bufio.NewReader(os.Stdin)
					answer, _ := reader.ReadString('\n')
					answer = strings.TrimSpace(strings.ToLower(answer))
					if answer != "y" && answer != "yes" {
						return nil
					}
				} else {
					fmt.Fprintln(os.Stderr, "Warning: overwriting existing schedule (non-interactive mode)")
				}
				if err := sched.Remove(); err != nil {
					return fmt.Errorf("removing existing schedule: %w", err)
				}
			}

			s := scheduler.Schedule{CronExpr: cronExpr}
			if err := sched.Install(s); err != nil {
				return fmt.Errorf("installing schedule: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Schedule registered: %s\n", cronExpr)
			return nil
		},
	}

	cmd.Flags().BoolVar(&daily, "daily", false, "schedule daily cleanup at noon (alias for --cron '0 12 * * *')")
	cmd.Flags().StringVar(&cron, "cron", "", "custom cron expression (5-field format)")
	cmd.Flags().BoolVar(&status, "status", false, "show current schedule status")
	cmd.Flags().BoolVar(&watchMode, "watch", false, "register watch daemon for disk monitoring")

	return cmd
}

func showStatus(configDir string, watchMode bool) error {
	sched := scheduler.New(configDir)

	if watchMode {
		if sched.IsWatchInstalled() {
			fmt.Fprintln(os.Stdout, "Watch daemon: registered")
		} else {
			fmt.Fprintln(os.Stdout, "Watch daemon: not registered. Use `devclean schedule --watch` to set up.")
		}
		return nil
	}

	st, err := sched.Status()
	if err != nil {
		return fmt.Errorf("reading status: %w", err)
	}

	if !st.Active {
		fmt.Fprintln(os.Stdout, "No schedule registered. Use `devclean schedule --daily` to set up.")
		return nil
	}

	fmt.Fprintln(os.Stdout, "Schedule Status:")
	fmt.Fprintf(os.Stdout, "  Active:     yes\n")
	if st.CronExpr != "" {
		fmt.Fprintf(os.Stdout, "  Frequency:  %s\n", st.CronExpr)
	}
	if st.BinaryPath != "" {
		fmt.Fprintf(os.Stdout, "  Binary:     %s\n", st.BinaryPath)
	}
	if st.LastResult != nil {
		fmt.Fprintf(os.Stdout, "  Last run:   %s (%d targets, %s freed, %s)\n",
			st.LastResult.Timestamp.Format("2006-01-02 15:04:05"),
			st.LastResult.TargetsCount,
			humanSize(st.LastResult.BytesFreed),
			st.LastResult.Duration,
		)
	}
	if st.NextEstimate != nil {
		fmt.Fprintf(os.Stdout, "  Next run:   %s\n", st.NextEstimate.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func installWatch(configDir string) error {
	cfg, err := loadConfigWithProfile()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	interval, err := time.ParseDuration(cfg.Watch.Interval)
	if err != nil {
		interval = 6 * time.Hour
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}

	sched := scheduler.New(configDir)

	if sched.IsWatchInstalled() {
		fmt.Fprintln(os.Stderr, "Watch daemon already registered. Remove first with `devclean unschedule --watch`.")
		return nil
	}

	ws := scheduler.WatchSchedule{
		BinaryPath: self,
		Interval:   interval,
	}

	if err := sched.InstallWatch(ws); err != nil {
		return fmt.Errorf("installing watch daemon: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Watch daemon registered (interval: %s, threshold: %s)\n",
		interval, humanSize(cfg.Watch.ThresholdMB*1024*1024))
	return nil
}

func newUnscheduleCmd() *cobra.Command {
	var watchMode bool

	cmd := &cobra.Command{
		Use:   "unschedule",
		Short: "Remove existing devclean schedule",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolver := platform.New()
			paths := resolver.Resolve()
			sched := scheduler.New(paths.ConfigDir)

			if watchMode {
				if err := sched.RemoveWatch(); err != nil {
					fmt.Fprintln(os.Stdout, "Watch daemon not registered")
					return nil
				}
				fmt.Fprintln(os.Stdout, "Watch daemon removed")
				return nil
			}

			if err := sched.Remove(); err != nil {
				if err == scheduler.ErrNotInstalled {
					fmt.Fprintln(os.Stdout, "No schedule found")
					return nil
				}
				return fmt.Errorf("removing schedule: %w", err)
			}

			fmt.Fprintln(os.Stdout, "Schedule removed")
			return nil
		},
	}

	cmd.Flags().BoolVar(&watchMode, "watch", false, "remove watch daemon registration")
	return cmd
}
