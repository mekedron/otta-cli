package cli

import (
	"fmt"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

func newWorktimesBrowseCommand() *cobra.Command {
	var (
		dateFrom     string
		dateTo       string
		user         string
		order        string
		sideload     bool
		outputFormat string
	)

	today := formatISODate(time.Now().UTC())

	cmd := &cobra.Command{
		Use:   "browse",
		Short: "Browse worktime entries across a date range.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}

			fromDate, toDate, err := parseWorktimesDateRange(dateFrom, dateTo)
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

			client := newAPIClient(cfg, configPath)
			report, err := collectWorktimesRange(cmd.Context(), client, fromDate, toDate, user, order, sideload)
			if err != nil {
				return err
			}

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "worktimes browse",
					Data: map[string]any{
						"from":          report.From,
						"to":            report.To,
						"days":          report.Days,
						"count":         report.Count,
						"total_minutes": report.TotalMinutes,
						"items":         report.Items,
						"responses":     report.Responses,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "from: %s\n", report.From)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "to: %s\n", report.To)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "days: %d\n", report.Days)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "entries: %d\n", report.Count)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "total_minutes: %d\n", report.TotalMinutes)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for full payload")
			return nil
		},
	}

	cmd.Flags().StringVar(&dateFrom, "from", today, "Start date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&dateTo, "to", today, "End date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&user, "user", "self", "User filter, use `self` for logged-in user.")
	cmd.Flags().StringVar(&order, "order", "starttime,endtime", "Sort order.")
	cmd.Flags().BoolVar(&sideload, "sideload", true, "Request sideload data.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}
