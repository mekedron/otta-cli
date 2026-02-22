package cli

import (
	"fmt"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect local configuration.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	configCmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print resolved config path.",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), config.ResolvePath())
		},
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "cache-path",
		Short: "Print resolved cache path.",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), config.ResolveCachePath())
		},
	})

	return configCmd
}
