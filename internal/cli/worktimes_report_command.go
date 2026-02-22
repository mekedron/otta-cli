package cli

import (
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

func newWorktimesReportCommand() *cobra.Command {
	var (
		dateFrom string
		dateTo   string
		user     string
		order    string
		sideload bool
		format   string
	)

	today := formatISODate(time.Now().UTC())

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate worktime-only report in JSON or CSV (no absences).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, format, outputFormatJSON, outputFormatJSON, outputFormatCSV)
			if err != nil {
				return err
			}
			durationFormat, err := resolveDurationFormat(cmd)
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

			if selectedFormat == outputFormatCSV {
				return writeWorktimesCSV(cmd.OutOrStdout(), report.Items)
			}

			return writeJSON(cmd, commandResult{
				OK:      true,
				Command: "worktimes report",
				Data: map[string]any{
					"from":            report.From,
					"to":              report.To,
					"days":            report.Days,
					"count":           report.Count,
					"total_minutes":   report.TotalMinutes,
					"duration_format": durationFormat,
					"total_duration":  durationSummary(report.TotalMinutes, durationFormat),
					"items":           report.Items,
					"responses":       report.Responses,
				},
			})
		},
	}

	cmd.Flags().StringVar(&dateFrom, "from", today, "Start date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&dateTo, "to", today, "End date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&user, "user", "self", "User filter, use `self` for logged-in user.")
	cmd.Flags().StringVar(&order, "order", "starttime,endtime", "Sort order.")
	cmd.Flags().BoolVar(&sideload, "sideload", true, "Request sideload data.")
	addOutputFormatFlags(cmd, &format, outputFormatJSON, outputFormatJSON, outputFormatCSV)

	return cmd
}
