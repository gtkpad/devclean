package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/spf13/cobra"
)

func newTrashCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trash",
		Short: "Manage trashed items (undo, list, purge)",
		Long:  "View and manage items moved to trash by devclean clean --trash.",
	}
	cmd.AddCommand(newTrashUndoCmd())
	cmd.AddCommand(newTrashListCmd())
	cmd.AddCommand(newTrashPurgeCmd())
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
			if id == "" {
				if err := trashMgr.UndoLast(ctx); err != nil {
					return err
				}
				fmt.Fprintln(os.Stdout, "Last batch restored successfully")
				return nil
			}
			if err := trashMgr.Undo(ctx, id); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Restored: %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "batch or entry ID to restore")
	return cmd
}

func newTrashListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show all restorable items with age and size",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if len(batches) == 0 {
				fmt.Fprintln(os.Stdout, "Trash is empty")
				return nil
			}
			for _, b := range batches {
				age := time.Since(b.CreatedAt).Round(time.Minute)
				fmt.Fprintf(os.Stdout, "Batch %s (%s ago, %s total, %d items)\n",
					b.ID[:8], formatAge(age), humanSize(b.TotalSize), len(b.Entries))
				for _, e := range b.Entries {
					fmt.Fprintf(os.Stdout, "  %s  %s  %s\n",
						e.ID[:8], humanSize(e.SizeBytes), e.OriginalPath)
				}
			}
			return nil
		},
	}
}

func newTrashPurgeCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Remove expired items from trash (or all with --all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			ctx := context.Background()
			trashMgr := trash.New(cfg.Safety)
			if err := trashMgr.Purge(ctx, all); err != nil {
				return err
			}
			if all {
				fmt.Fprintln(os.Stdout, "All trash entries purged")
			} else {
				fmt.Fprintln(os.Stdout, "Expired entries purged")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "purge ALL trash entries, not just expired")
	return cmd
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
