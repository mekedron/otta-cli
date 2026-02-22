package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

type holidaysCommandOptions struct {
	dateFrom      string
	dateTo        string
	worktimeGroup int64
	outputFormat  string
}

func newHolidaysCommand() *cobra.Command {
	options := holidaysCommandOptions{}

	cmd := &cobra.Command{
		Use:   "holidays",
		Short: "Retrieve holidays/workday calendar data.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHolidaysQuery(cmd, "holidays", options)
		},
	}

	addHolidaysFlags(cmd, &options)
	cmd.AddCommand(newHolidaysReadCommand())

	return cmd
}

func newHolidaysReadCommand() *cobra.Command {
	options := holidaysCommandOptions{}
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read holidays/workday calendar data.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHolidaysQuery(cmd, "holidays read", options)
		},
	}

	addHolidaysFlags(cmd, &options)
	return cmd
}

func addHolidaysFlags(cmd *cobra.Command, options *holidaysCommandOptions) {
	today := formatISODate(time.Now().UTC())
	cmd.Flags().StringVar(&options.dateFrom, "from", today, "Start date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&options.dateTo, "to", today, "End date in YYYY-MM-DD.")
	cmd.Flags().Int64Var(&options.worktimeGroup, "worktimegroup", 0, "Worktimegroup id.")
	addOutputFormatFlags(cmd, &options.outputFormat, outputFormatText, outputFormatText, outputFormatJSON)
}

func runHolidaysQuery(cmd *cobra.Command, commandName string, options holidaysCommandOptions) error {
	selectedFormat, err := resolveOutputFormat(cmd, options.outputFormat, outputFormatText, outputFormatJSON)
	if err != nil {
		return err
	}
	durationFormat, err := resolveDurationFormat(cmd)
	if err != nil {
		return err
	}
	if _, _, err := parseWorktimesDateRange(options.dateFrom, options.dateTo); err != nil {
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

	worktimeGroup := options.worktimeGroup
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
		"date":          fmt.Sprintf("%s_%s", options.dateFrom, options.dateTo),
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
			Command: commandName,
			Data: map[string]any{
				"from":              options.dateFrom,
				"to":                options.dateTo,
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

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "from: %s\n", options.dateFrom)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "to: %s\n", options.dateTo)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktime_group_id: %d\n", worktimeGroup)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "days: %d\n", count)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "holiday_minutes: %d\n", holidayMinutes)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "holiday_duration: %s\n", formatDurationForText(holidayMinutes, durationFormat))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "absence_minutes: %d\n", absenceMinutes)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "absence_duration: %s\n", formatDurationForText(absenceMinutes, durationFormat))
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
	return nil
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
