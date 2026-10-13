package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/installer"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	installerStdinReader io.Reader = os.Stdin
	installerIsTerminal            = term.IsTerminal
)

type installerJSONOutput struct {
	Candidates     []installer.Candidate `json:"candidates"`
	TotalFiles     int                   `json:"total_files"`
	TotalSizeBytes int64                 `json:"total_size_bytes"`
}

type installerCleanerProvider struct {
	cleaner domain.Cleaner
}

func (p *installerCleanerProvider) GetCleaner(name string) domain.Cleaner {
	if name == "installer" {
		return p.cleaner
	}
	return nil
}

func newInstallerCmd() *cobra.Command {
	var minAgeStr string
	var minSizeStr string
	var customPath string
	var dryRun bool
	var yes bool
	var useTrash bool
	var permanent bool
	var format string

	cmd := &cobra.Command{
		Use:          "installer",
		Short:        "Detect and clean orphan installer packages (.dmg, .pkg, .iso, .exe, .zip, etc.)",
		Long:         "Scans ~/Downloads, ~/Desktop and custom paths for old installers and disk images.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if useTrash && permanent {
				return fmt.Errorf("--trash and --permanent are mutually exclusive")
			}
			if format != "table" && format != "json" {
				return fmt.Errorf("unknown format: %q (supported: table, json)", format)
			}

			var minAge time.Duration
			if minAgeStr != "" {
				var err error
				minAge, err = parseDuration(minAgeStr)
				if err != nil {
					return fmt.Errorf("invalid --min-age %q: %w", minAgeStr, err)
				}
			}

			var minSize int64
			if minSizeStr != "" {
				var err error
				minSize, err = parseBytes(minSizeStr)
				if err != nil {
					return fmt.Errorf("invalid --min-size %q: %w", minSizeStr, err)
				}
			}

			cfg, err := loadConfigWithProfile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			scanPaths := cfg.Installer.Paths
			if customPath != "" {
				scanPaths = []string{customPath}
			}
			if len(scanPaths) == 0 {
				scanPaths = []string{"~/Downloads", "~/Desktop"}
			}

			maxDepth := cfg.Installer.MaxDepth
			if maxDepth <= 0 {
				maxDepth = 2
			}

			bl := safety.NewBlacklist(cfg.ProtectedPatterns)

			scannerOpts := installer.Options{
				Paths:    scanPaths,
				MaxDepth: maxDepth,
				MinAge:   minAge,
				MinSize:  minSize,
			}
			scanner := installer.NewScanner(scannerOpts, bl)

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			candidates, err := scanner.Scan(ctx)
			if err != nil {
				return fmt.Errorf("scanning installers: %w", err)
			}

			var totalBytes int64
			for _, c := range candidates {
				totalBytes += c.SizeBytes
			}

			if len(candidates) == 0 {
				if format == "json" {
					out := installerJSONOutput{
						Candidates:     []installer.Candidate{},
						TotalFiles:     0,
						TotalSizeBytes: 0,
					}
					data, err := json.MarshalIndent(out, "", "  ")
					if err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
					return nil
				}
				if !quiet {
					fmt.Fprintln(cmd.OutOrStdout(), "Nenhum instalador órfão encontrado.")
				}
				return nil
			}

			// Render Preview
			if format == "json" {
				out := installerJSONOutput{
					Candidates:     candidates,
					TotalFiles:     len(candidates),
					TotalSizeBytes: totalBytes,
				}
				data, err := json.MarshalIndent(out, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else if !quiet {
				renderInstallerTable(cmd.OutOrStdout(), candidates, totalBytes)
			}

			if dryRun {
				return nil
			}

			// Interactive confirmation
			if !yes && !dryRun {
				isTTY := installerIsTerminal(int(os.Stdin.Fd()))
				if !isTTY {
					return fmt.Errorf("confirmação interativa indisponível sem TTY; use --yes para prosseguir")
				}

				fmt.Fprint(cmd.OutOrStdout(), "Deseja mover os instaladores selecionados para a lixeira? [y/N]: ")
				reader := bufio.NewReader(installerStdinReader)
				ans, err := reader.ReadString('\n')
				if err != nil && err != io.EOF {
					return fmt.Errorf("lendo confirmação: %w", err)
				}
				ans = strings.TrimSpace(strings.ToLower(ans))
				if ans != "y" && ans != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "Operação cancelada pelo usuário.")
					return nil
				}
			}

			// Coordinator Deletion Execution
			dataDir := getDataDir()
			trashActive := cfg.Safety.TrashEnabled
			if useTrash {
				trashActive = true
			}
			if permanent {
				trashActive = false
			}

			trashMgr := trash.New(cfg.Safety)
			if trashActive {
				_ = trashMgr.EnforceRetention(ctx)
				_ = trashMgr.EnforceMaxSize(ctx)
			}

			cleanerProvider := &installerCleanerProvider{
				cleaner: installer.NewCleaner(scannerOpts),
			}

			hookRunner := hooks.New(cfg.Hooks)
			paths := platform.New().Resolve()
			coord := deleter.New(cleanerProvider, cfg, bl, trashMgr, hookRunner, paths.ConfigDir, dataDir)

			var useTrashOpt *bool
			if useTrash {
				t := true
				useTrashOpt = &t
			} else if permanent {
				f := false
				useTrashOpt = &f
			}

			targets := make([]domain.Target, len(candidates))
			for i, cand := range candidates {
				targets[i] = domain.Target{
					ID:           "installer:" + cand.Path,
					Name:         cand.Detail,
					Path:         cand.Path,
					SizeBytes:    cand.SizeBytes,
					Risk:         domain.RiskLow,
					LastAccessed: cand.ModTime,
					Category:     "installer",
				}
			}

			opts := deleter.Options{
				DryRun:   dryRun,
				UseTrash: useTrashOpt,
				Mode:     "installer",
				Auto:     false,
				OnProgress: func(ev deleter.ProgressEvent) {
					if quiet || format == "json" {
						return
					}
					switch ev.Action {
					case deleter.ActionWouldDelete:
						fmt.Fprintf(cmd.OutOrStdout(), "Would delete: %s (%s)\n", ev.Target.Path, humanSize(ev.Target.SizeBytes))
					case deleter.ActionMovingTrash:
						fmt.Fprintf(cmd.OutOrStdout(), "Moving to trash: %s\n", ev.Target.Path)
					case deleter.ActionDeletingDirect, deleter.ActionVirtualDirect:
						fmt.Fprintf(cmd.OutOrStdout(), "Deleting: %s\n", ev.Target.Path)
					}
				},
				OnWarning: func(msg string) {
					if !quiet {
						fmt.Fprintln(os.Stderr, "Warning:", msg)
					}
				},
			}

			res, err := coord.Delete(ctx, map[string][]domain.Target{"installer": targets}, opts)
			if err != nil {
				return fmt.Errorf("installer cleanup failed: %w", err)
			}

			for _, err := range res.Errors {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}

			if !quiet && format == "table" {
				if trashActive {
					fmt.Fprintf(cmd.OutOrStdout(), "\nLimpeza concluída: %d instalador(es) movido(s) para a lixeira (%s liberados).\n", res.TotalCount, humanSize(res.TotalFreed))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "\nLimpeza concluída: %d instalador(es) removido(s) permanentemente (%s liberados).\n", res.TotalCount, humanSize(res.TotalFreed))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&minAgeStr, "min-age", "24h", "minimum file age threshold before selection (e.g. 24h, 7d, 0 to disable)")
	cmd.Flags().StringVar(&minSizeStr, "min-size", "", "minimum file size threshold (e.g. 50MB, 1GB)")
	cmd.Flags().StringVar(&customPath, "path", "", "custom single directory to scan")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview installer files without deleting")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip interactive confirmation prompt")
	cmd.Flags().BoolVar(&useTrash, "trash", false, "force moving installer files to trash")
	cmd.Flags().BoolVar(&permanent, "permanent", false, "permanently delete installer files without trash")
	cmd.Flags().StringVar(&format, "format", "table", "output format (table, json)")

	return cmd
}

func renderInstallerTable(w io.Writer, candidates []installer.Candidate, totalBytes int64) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		Headers("CAMINHO", "TAMANHO", "IDADE", "TIPO/DETALHE").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return lipgloss.NewStyle()
		})

	for _, c := range candidates {
		ageStr := formatAge(time.Since(c.ModTime))
		t.Row(c.Path, humanSize(c.SizeBytes), ageStr, c.Detail)
	}

	fmt.Fprintln(w, t.Render())
	fmt.Fprintf(w, "Total: %d arquivo(s) (%s recuperáveis)\n\n", len(candidates), humanSize(totalBytes))
}
