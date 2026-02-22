package cli

import (
	"fmt"
	"net/http"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

func newSaldoCommand() *cobra.Command {
	var (
		user         string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "saldo",
		Short: "Get current cumulative saldo.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}
			durationFormat, err := resolveDurationFormat(cmd)
			if err != nil {
				return err
			}

			configPath := config.ResolvePath()
			cfg, err := loadRuntimeConfig(configPath)
			if err != nil {
				return err
			}
			if err := requireAccessToken(cfg); err != nil {
				return err
			}
			cachePath := config.ResolveCachePath()
			cache, err := loadRuntimeCache(cachePath)
			if err != nil {
				return err
			}

			userID, err := resolveWorktimeOptionsUserID(user, cache)
			if err != nil {
				return err
			}
			if userID <= 0 {
				return fmt.Errorf("--user is required (or set OTTA_CLI_USER_ID / run `otta status` to refresh cache)")
			}

			client := newAPIClient(cfg, configPath)
			query := map[string]string{
				"userid": fmt.Sprintf("%d", userID),
			}

			raw := map[string]any{}
			if err := client.Request(cmd.Context(), http.MethodGet, "/ttapi/saldo/get_current_saldo", query, nil, &raw); err != nil {
				return err
			}

			cumulativeSaldo := extractCurrentSaldoMinutes(raw)
			fromDate := firstNonEmpty(getString(raw, "from"), getString(raw, "startdate"), getString(raw, "start"))
			toDate := firstNonEmpty(getString(raw, "to"), getString(raw, "enddate"), getString(raw, "end"))

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "saldo",
					Data: map[string]any{
						"user":                      userID,
						"user_id":                   userID,
						"from":                      fromDate,
						"to":                        toDate,
						"cumulative_saldo":          cumulativeSaldo,
						"cumulative_saldo_minutes":  cumulativeSaldo,
						"duration_format":           durationFormat,
						"cumulative_saldo_duration": durationSummary(cumulativeSaldo, durationFormat),
						"raw":                       raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "user_id: %d\n", userID)
			if fromDate != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "from: %s\n", fromDate)
			}
			if toDate != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "to: %s\n", toDate)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cumulative_saldo_minutes: %d\n", cumulativeSaldo)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cumulative_saldo_duration: %s\n", formatDurationForText(cumulativeSaldo, durationFormat))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	cmd.Flags().StringVar(&user, "user", "self", "User filter, use `self` for logged-in user.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func extractCurrentSaldoMinutes(raw map[string]any) int {
	for _, key := range []string{"saldo", "cumulative_saldo", "minutes"} {
		if value, ok := raw[key]; ok {
			return int(toInt64(value))
		}
	}
	return 0
}
