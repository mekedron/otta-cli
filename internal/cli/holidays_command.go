package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

func newHolidaysCommand() *cobra.Command {
	var (
		dateFrom      string
		dateTo        string
		worktimeGroup int64
		outputFormat  string
	)

	cmd := &cobra.Command{
		Use:   "holidays",
		Short: "Retrieve holidays/workday calendar data.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}
			durationFormat, err := resolveDurationFormat(cmd)
			if err != nil {
				return err
			}

			fromDate, err := parseISODate(dateFrom)
			if err != nil {
				return fmt.Errorf("--from must be YYYY-MM-DD")
			}
			toDate, err := parseISODate(dateTo)
			if err != nil {
				return fmt.Errorf("--to must be YYYY-MM-DD")
			}
			if toDate.Before(fromDate) {
				return fmt.Errorf("--to must be greater than or equal to --from")
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
			if worktimeGroup <= 0 {
				if value, ok := config.EnvInt64(config.EnvWorktimeGroupID); ok {
					worktimeGroup = value
				}
			}
			if worktimeGroup <= 0 {
				worktimeGroup = cache.User.WorktimeGroupID
			}
			if worktimeGroup <= 0 {
				return fmt.Errorf("--worktimegroup is required (or set OTTA_CLI_WORKTIMEGROUP_ID / run `otta status` to refresh cache)")
			}

			client := newAPIClient(cfg, configPath)
			query := map[string]string{
				"date":          fmt.Sprintf("%s_%s", dateFrom, dateTo),
				"worktimegroup": fmt.Sprintf("%d", worktimeGroup),
			}

			var raw any
			if err := client.Request(cmd.Context(), http.MethodGet, "/ttapi/workdayCalendar/workdayDays", query, nil, &raw); err != nil {
				return err
			}

			rows := extractHolidayRows(raw)
			count := len(rows)
			holidayMinutes := sumHolidayDurationField(rows, "minutes")
			absenceMinutes := sumHolidayDurationField(rows, "absence_minutes")
			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "holidays",
					Data: map[string]any{
						"from":              dateFrom,
						"to":                dateTo,
						"worktimegroup":     worktimeGroup,
						"worktime_group_id": worktimeGroup,
						"count":             count,
						"holiday_minutes":   holidayMinutes,
						"absence_minutes":   absenceMinutes,
						"duration_format":   durationFormat,
						"holiday_duration":  durationSummary(holidayMinutes, durationFormat),
						"absence_duration":  durationSummary(absenceMinutes, durationFormat),
						"raw":               raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "from: %s\n", dateFrom)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "to: %s\n", dateTo)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktime_group_id: %d\n", worktimeGroup)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "days: %d\n", count)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "holiday_minutes: %d\n", holidayMinutes)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "holiday_duration: %s\n", formatDurationForText(holidayMinutes, durationFormat))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "absence_minutes: %d\n", absenceMinutes)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "absence_duration: %s\n", formatDurationForText(absenceMinutes, durationFormat))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	today := formatISODate(time.Now().UTC())
	cmd.Flags().StringVar(&dateFrom, "from", today, "Start date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&dateTo, "to", today, "End date in YYYY-MM-DD.")
	cmd.Flags().Int64Var(&worktimeGroup, "worktimegroup", 0, "Worktimegroup id.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func sumHolidayDurationField(rows []map[string]any, key string) int {
	total := 0
	for _, row := range rows {
		value := int(toInt64(row[key]))
		if value <= 0 {
			continue
		}
		total += value
	}
	return total
}
