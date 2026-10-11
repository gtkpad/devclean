package cmd

import (
	"fmt"

	"github.com/gtkpad/devclean/internal/safety"
	"github.com/spf13/cobra"
)

func newWhitelistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whitelist",
		Short: "Manage excluded paths and protected patterns",
		Long:  "View, add, or remove paths and glob patterns protected from deletion.",
	}
	cmd.AddCommand(newWhitelistListCmd())
	return cmd
}

func newWhitelistListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all excluded paths, protected patterns, and immutable blacklist",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigWithProfile()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			fmt.Fprintln(w, "Caminhos Excluídos (exclude_paths):")
			if len(cfg.ExcludePaths) == 0 {
				fmt.Fprintln(w, "  (nenhum)")
			} else {
				for _, p := range cfg.ExcludePaths {
					fmt.Fprintf(w, "  • %s\n", p)
				}
			}
			fmt.Fprintln(w)

			fmt.Fprintln(w, "Padrões Protegidos (protected_patterns):")
			if len(cfg.ProtectedPatterns) == 0 {
				fmt.Fprintln(w, "  (nenhum)")
			} else {
				for _, pat := range cfg.ProtectedPatterns {
					fmt.Fprintf(w, "  • %s\n", pat)
				}
			}
			fmt.Fprintln(w)

			fmt.Fprintln(w, "Blacklist Imutável do Sistema:")
			for _, item := range safety.CoreBlacklist() {
				fmt.Fprintf(w, "  • %s [imutável, somente-leitura]\n", item)
			}

			return nil
		},
	}
}
