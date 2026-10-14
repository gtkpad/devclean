package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/spf13/cobra"
)

func newTrashCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trash",
		Short: "Manage trashed items (undo, list, clean, purge)",
		Long:  "View and manage items moved to trash by devclean clean and installer.",
	}
	cmd.AddCommand(newTrashUndoCmd())
	cmd.AddCommand(newTrashListCmd())
	cmd.AddCommand(newTrashCleanCmd()) // D-13: Canônico
	cmd.AddCommand(newTrashPurgeCmd()) // D-13: Alias transparente
	return cmd
}

func newTrashUndoCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "undo",
		Short: "Restore trashed items to original paths",
		Long:  "Without --id, restores the last batch. With --id, restores by batch or entry ID.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			ctx := context.Background()
			trashMgr := trash.New(cfg.Safety)
			out := cmd.OutOrStdout()
			if id == "" {
				if err := trashMgr.UndoLast(ctx); err != nil {
					return err
				}
				fmt.Fprintln(out, "Last batch restored successfully")
				return nil
			}
			if err := trashMgr.Undo(ctx, id); err != nil {
				return err
			}
			fmt.Fprintf(out, "Restored: %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "batch or entry ID to restore")
	return cmd
}

func newTrashListCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show all restorable items with age and size",
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unknown format: %q (supported: table, json)", format)
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			ctx := context.Background()
			trashMgr := trash.New(cfg.Safety)
			batches, err := trashMgr.List(ctx)
			if err != nil {
				return err
			}

			quota := trashMgr.QuotaStatus()
			out := cmd.OutOrStdout()
			retention := time.Duration(cfg.Safety.RetentionDays) * 24 * time.Hour
			now := time.Now()

			if format == "json" {
				type listEntryJSON struct {
					ID           string `json:"id"`
					SizeBytes    int64  `json:"size_bytes"`
					OriginalPath string `json:"original_path"`
				}
				type listBatchJSON struct {
					ID                 string          `json:"id"`
					CreatedAt          time.Time       `json:"created_at"`
					Age                string          `json:"age"`
					TotalSize          int64           `json:"total_size"`
					ItemCount          int             `json:"item_count"`
					Expired            bool            `json:"expired"`
					RemainingRetention string          `json:"remaining_retention,omitempty"`
					Entries            []listEntryJSON `json:"entries"`
				}
				type listOutputJSON struct {
					Quota        trash.QuotaInfo `json:"quota"`
					TotalBatches int             `json:"total_batches"`
					Batches      []listBatchJSON `json:"batches"`
				}

				batchesJSON := make([]listBatchJSON, len(batches))
				for i, b := range batches {
					age := now.Sub(b.CreatedAt)
					isExpired := retention > 0 && age > retention
					var remainingStr string
					if !isExpired && retention > 0 {
						remainingStr = formatAge(retention - age)
					}

					entriesJSON := make([]listEntryJSON, len(b.Entries))
					for j, e := range b.Entries {
						entriesJSON[j] = listEntryJSON{
							ID:           e.ID,
							SizeBytes:    e.SizeBytes,
							OriginalPath: e.OriginalPath,
						}
					}

					batchesJSON[i] = listBatchJSON{
						ID:                 b.ID,
						CreatedAt:          b.CreatedAt,
						Age:                formatAge(age.Round(time.Minute)),
						TotalSize:          b.TotalSize,
						ItemCount:          len(b.Entries),
						Expired:            isExpired,
						RemainingRetention: remainingStr,
						Entries:            entriesJSON,
					}
				}

				resp := listOutputJSON{
					Quota:        quota,
					TotalBatches: len(batches),
					Batches:      batchesJSON,
				}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}

			// Table format
			fmt.Fprintf(out, "Lixeira: %s / %s (%.1f%% ocupado)\n\n",
				humanSize(quota.CurrentBytes), humanSize(quota.MaxBytes), quota.Percent)

			if len(batches) == 0 {
				fmt.Fprintln(out, "Trash is empty")
				return nil
			}

			for _, b := range batches {
				age := now.Sub(b.CreatedAt)
				isExpired := retention > 0 && age > retention
				switch {
				case isExpired:
					fmt.Fprintf(out, "Batch %s [EXPIRADO] (%s ago, %s total, %d items)\n",
						b.ID[:8], formatAge(age.Round(time.Minute)), humanSize(b.TotalSize), len(b.Entries))
				case retention > 0:
					fmt.Fprintf(out, "Batch %s (%s ago, tempo restante: %s, %s total, %d items)\n",
						b.ID[:8], formatAge(age.Round(time.Minute)), formatAge(retention-age), humanSize(b.TotalSize), len(b.Entries))
				default:
					fmt.Fprintf(out, "Batch %s (%s ago, %s total, %d items)\n",
						b.ID[:8], formatAge(age.Round(time.Minute)), humanSize(b.TotalSize), len(b.Entries))
				}
				for _, e := range b.Entries {
					fmt.Fprintf(out, "  %s  %s  %s\n",
						e.ID[:8], humanSize(e.SizeBytes), e.OriginalPath)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "output format (table, json)")
	return cmd
}

func newTrashCleanCmd() *cobra.Command {
	var expired bool
	var all bool
	var format string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean items from trash (expired items by default, or all with --all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unknown format: %q (supported: table, json)", format)
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			ctx := context.Background()
			trashMgr := trash.New(cfg.Safety)
			out := cmd.OutOrStdout()

			if all {
				if err := trashMgr.Purge(ctx, true); err != nil {
					return err
				}
				if format == "json" {
					enc := json.NewEncoder(out)
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]any{
						"status": "all_purged",
						"all":    true,
					})
				}
				fmt.Fprintln(out, "Todos os lotes da lixeira foram removidos.")
				return nil
			}

			stats, err := trashMgr.CleanExpired(ctx)
			if err != nil {
				return err
			}

			if format == "json" {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(stats)
			}

			if stats.BatchesPurged == 0 {
				fmt.Fprintln(out, "Nenhum item expirado na lixeira.")
				return nil
			}

			if stats.BatchesPurged == 1 {
				fmt.Fprintf(out, "1 lote expirado removido (%s liberados).\n", humanSize(stats.BytesFreed))
			} else {
				fmt.Fprintf(out, "%d lotes expirados removidos (%s liberados).\n", stats.BatchesPurged, humanSize(stats.BytesFreed))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&expired, "expired", true, "clean only expired entries (default behavior)")
	cmd.Flags().BoolVar(&all, "all", false, "clean ALL trash entries, bypassing retention period")
	cmd.Flags().StringVar(&format, "format", "table", "output format (table, json)")
	return cmd
}

func newTrashPurgeCmd() *cobra.Command {
	cleanCmd := newTrashCleanCmd()
	cleanCmd.Use = "purge"
	cleanCmd.Short = "Alias for 'trash clean'"
	return cleanCmd
}

func formatAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours()) / 24
	if days < 7 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dw", days/7)
}

