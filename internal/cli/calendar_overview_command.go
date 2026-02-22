package cli

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/mekedron/otta-cli/internal/otta"
	"github.com/spf13/cobra"
)

type calendarDaySection struct {
	Count        int              `json:"count"`
	TotalMinutes int              `json:"total_minutes"`
	TotalHours   float64          `json:"total_hours"`
	Items        []map[string]any `json:"items"`
}

type calendarHolidaySection struct {
	Count int              `json:"count"`
	Items []map[string]any `json:"items"`
}

type calendarOverviewDay struct {
	Date      string                 `json:"date"`
	Weekday   string                 `json:"weekday"`
	Weekend   bool                   `json:"weekend"`
	Worktimes calendarDaySection     `json:"worktimes"`
	Absences  calendarDaySection     `json:"absences"`
	Holidays  calendarHolidaySection `json:"holidays"`
}

type calendarOverviewTotals struct {
	WorktimeCount     int     `json:"worktime_count"`
	WorktimeMinutes   int     `json:"worktime_minutes"`
	WorktimeHours     float64 `json:"worktime_hours"`
	AbsenceCount      int     `json:"absence_count"`
	AbsenceMinutes    int     `json:"absence_minutes"`
	AbsenceHours      float64 `json:"absence_hours"`
	HolidayCount      int     `json:"holiday_count"`
	DaysWithWorktimes int     `json:"days_with_worktimes"`
	DaysWithAbsences  int     `json:"days_with_absences"`
	DaysWithHolidays  int     `json:"days_with_holidays"`
	WeekendDays       int     `json:"weekend_days"`
}

type calendarOverviewResult struct {
	From          string                 `json:"from"`
	To            string                 `json:"to"`
	Days          int                    `json:"days"`
	WorktimeGroup int64                  `json:"worktimegroup"`
	Totals        calendarOverviewTotals `json:"totals"`
	Items         []calendarOverviewDay  `json:"items"`
	Raw           map[string]any         `json:"raw"`
}

func newCalendarOverviewCommand() *cobra.Command {
	var (
		dateFrom      string
		dateTo        string
		user          string
		order         string
		sideload      bool
		worktimeGroup int64
		outputFormat  string
	)

	today := formatISODate(time.Now().UTC())

	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Generate combined calendar overview for worktimes, absences, and holidays.",
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
			report, err := collectCalendarOverview(cmd.Context(), client, fromDate, toDate, user, order, sideload, worktimeGroup)
			if err != nil {
				return err
			}

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "calendar overview",
					Data: map[string]any{
						"from":          report.From,
						"to":            report.To,
						"days":          report.Days,
						"worktimegroup": report.WorktimeGroup,
						"filters": map[string]any{
							"user":     strings.TrimSpace(user),
							"order":    strings.TrimSpace(order),
							"sideload": sideload,
						},
						"totals": report.Totals,
						"items":  report.Items,
						"raw":    report.Raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "from: %s\n", report.From)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "to: %s\n", report.To)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "days: %d\n", report.Days)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktimegroup: %d\n", report.WorktimeGroup)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktime_minutes: %d\n", report.Totals.WorktimeMinutes)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "absence_minutes: %d\n", report.Totals.AbsenceMinutes)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "holiday_rows: %d\n", report.Totals.HolidayCount)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for full payload")
			return nil
		},
	}

	cmd.Flags().StringVar(&dateFrom, "from", today, "Start date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&dateTo, "to", today, "End date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&user, "user", "self", "User filter, use `self` for logged-in user.")
	cmd.Flags().StringVar(&order, "order", "starttime,endtime", "Sort order for worktimes/absences.")
	cmd.Flags().BoolVar(&sideload, "sideload", true, "Request sideload data for worktimes/absences.")
	cmd.Flags().Int64Var(&worktimeGroup, "worktimegroup", 0, "Worktimegroup id.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func collectCalendarOverview(
	ctx context.Context,
	client *otta.Client,
	fromDate time.Time,
	toDate time.Time,
	user string,
	order string,
	sideload bool,
	worktimeGroup int64,
) (calendarOverviewResult, error) {
	result := calendarOverviewResult{
		From:          formatISODate(fromDate),
		To:            formatISODate(toDate),
		WorktimeGroup: worktimeGroup,
		Items:         make([]calendarOverviewDay, 0),
		Raw:           map[string]any{},
	}

	trimmedUser := strings.TrimSpace(user)
	trimmedOrder := strings.TrimSpace(order)

	worktimesQuery := map[string]string{
		"date":     fmt.Sprintf("%s_%s", result.From, result.To),
		"order":    trimmedOrder,
		"limit":    "500",
		"sideload": strconv.FormatBool(sideload),
	}
	if trimmedUser != "" {
		worktimesQuery["user"] = trimmedUser
	}

	var worktimesRaw any
	if err := client.Request(ctx, http.MethodGet, "/worktimes", worktimesQuery, nil, &worktimesRaw); err != nil {
		return calendarOverviewResult{}, err
	}

	absencesQuery := map[string]string{
		"startdate": result.From,
		"enddate":   result.To,
		"order":     trimmedOrder,
	}
	if trimmedUser != "" {
		absencesQuery["user"] = trimmedUser
	}
	if sideload {
		absencesQuery["sideload[]"] = "abcensetype.name"
	}

	var absencesRaw any
	if err := client.Request(ctx, http.MethodGet, "/ttapi/absence/split", absencesQuery, nil, &absencesRaw); err != nil {
		return calendarOverviewResult{}, err
	}

	holidaysQuery := map[string]string{
		"date":          fmt.Sprintf("%s_%s", result.From, result.To),
		"worktimegroup": fmt.Sprintf("%d", worktimeGroup),
	}

	var holidaysRaw any
	if err := client.Request(ctx, http.MethodGet, "/ttapi/workdayCalendar/workdayDays", holidaysQuery, nil, &holidaysRaw); err != nil {
		return calendarOverviewResult{}, err
	}

	worktimeItems := extractWorktimeRows(worktimesRaw, "")
	sort.SliceStable(worktimeItems, func(i int, j int) bool {
		return worktimeRowLess(worktimeItems[i], worktimeItems[j])
	})

	absenceItems := extractAbsenceRows(absencesRaw)
	sort.SliceStable(absenceItems, func(i int, j int) bool {
		return absenceRowLess(absenceItems[i], absenceItems[j])
	})

	holidayItems := extractHolidayRows(holidaysRaw)
	sort.SliceStable(holidayItems, func(i int, j int) bool {
		leftDate := holidayDate(holidayItems[i])
		rightDate := holidayDate(holidayItems[j])
		if leftDate != rightDate {
			return leftDate < rightDate
		}
		leftID := toInt64(holidayItems[i]["id"])
		rightID := toInt64(holidayItems[j]["id"])
		if leftID != rightID {
			return leftID < rightID
		}
		return false
	})

	worktimeByDate := map[string][]map[string]any{}
	for _, item := range worktimeItems {
		dateValue := normalizedISODate(firstNonEmpty(getString(item, "date"), getString(item, "startdate")))
		if dateValue == "" {
			continue
		}
		item["date"] = dateValue
		worktimeByDate[dateValue] = append(worktimeByDate[dateValue], item)
	}

	absenceByDate := map[string][]map[string]any{}
	for _, item := range absenceItems {
		dateValue := absenceDate(item)
		if dateValue == "" {
			continue
		}
		item["date"] = dateValue
		absenceByDate[dateValue] = append(absenceByDate[dateValue], item)
	}

	holidayByDate := map[string][]map[string]any{}
	for _, item := range holidayItems {
		dateValue := holidayDate(item)
		if dateValue == "" {
			continue
		}
		item["date"] = dateValue
		holidayByDate[dateValue] = append(holidayByDate[dateValue], item)
	}

	for current := fromDate; !current.After(toDate); current = current.AddDate(0, 0, 1) {
		dateValue := formatISODate(current)
		weekend := current.Weekday() == time.Saturday || current.Weekday() == time.Sunday
		if weekend {
			result.Totals.WeekendDays++
		}

		dayWorktimes := normalizeRows(worktimeByDate[dateValue])
		dayAbsences := normalizeRows(absenceByDate[dateValue])
		dayHolidays := normalizeRows(holidayByDate[dateValue])

		worktimeMinutes := sumWorktimeMinutes(dayWorktimes)
		absenceMinutesTotal := sumAbsenceMinutes(dayAbsences)

		if len(dayWorktimes) > 0 {
			result.Totals.DaysWithWorktimes++
		}
		if len(dayAbsences) > 0 {
			result.Totals.DaysWithAbsences++
		}
		if len(dayHolidays) > 0 {
			result.Totals.DaysWithHolidays++
		}

		result.Totals.WorktimeCount += len(dayWorktimes)
		result.Totals.WorktimeMinutes += worktimeMinutes
		result.Totals.AbsenceCount += len(dayAbsences)
		result.Totals.AbsenceMinutes += absenceMinutesTotal
		result.Totals.HolidayCount += len(dayHolidays)

		result.Items = append(result.Items, calendarOverviewDay{
			Date:    dateValue,
			Weekday: strings.ToLower(current.Weekday().String()),
			Weekend: weekend,
			Worktimes: calendarDaySection{
				Count:        len(dayWorktimes),
				TotalMinutes: worktimeMinutes,
				TotalHours:   minutesToHours(worktimeMinutes),
				Items:        dayWorktimes,
			},
			Absences: calendarDaySection{
				Count:        len(dayAbsences),
				TotalMinutes: absenceMinutesTotal,
				TotalHours:   minutesToHours(absenceMinutesTotal),
				Items:        dayAbsences,
			},
			Holidays: calendarHolidaySection{
				Count: len(dayHolidays),
				Items: dayHolidays,
			},
		})
	}

	result.Days = len(result.Items)
	result.Totals.WorktimeHours = minutesToHours(result.Totals.WorktimeMinutes)
	result.Totals.AbsenceHours = minutesToHours(result.Totals.AbsenceMinutes)
	result.Raw = map[string]any{
		"worktimes": worktimesRaw,
		"absences":  absencesRaw,
		"holidays":  holidaysRaw,
	}

	return result, nil
}

func extractHolidayRows(raw any) []map[string]any {
	var list []any

	switch typed := raw.(type) {
	case []any:
		list = typed
	case map[string]any:
		for _, key := range []string{"workdayDays", "workdaydays", "days", "items", "results", "data"} {
			value, ok := typed[key]
			if !ok {
				continue
			}
			parsed, ok := value.([]any)
			if !ok {
				continue
			}
			list = parsed
			break
		}
	}

	if len(list) == 0 {
		return nil
	}

	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}

		normalized := cloneAnyMap(row)
		if dateValue := holidayDate(normalized); dateValue != "" {
			normalized["date"] = dateValue
		}
		rows = append(rows, normalized)
	}

	return rows
}

func holidayDate(item map[string]any) string {
	return normalizedISODate(
		firstNonEmpty(
			getString(item, "date"),
			getString(item, "day"),
			getString(item, "workday"),
			getString(item, "workdaydate"),
			getString(item, "workdayDate"),
			getString(item, "holidaydate"),
			getString(item, "holidayDate"),
			getString(item, "calendar_date"),
			getString(item, "startdate"),
		),
	)
}

func normalizeRows(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return []map[string]any{}
	}
	return values
}
