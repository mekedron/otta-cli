package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

type calendarCelebration struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Source string `json:"source"`
	Raw    any    `json:"raw,omitempty"`
}

type calendarDetailedEvent struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Minutes int    `json:"minutes,omitempty"`
	Raw     any    `json:"raw,omitempty"`
}

type calendarDetailedDay struct {
	Date          string                  `json:"date"`
	Weekday       string                  `json:"weekday"`
	IsWeekend     bool                    `json:"is_weekend"`
	IsDayOff      bool                    `json:"is_day_off"`
	DayOffReasons []string                `json:"day_off_reasons"`
	Worktimes     calendarDaySection      `json:"worktimes"`
	Absences      calendarDaySection      `json:"absences"`
	Holidays      calendarHolidaySection  `json:"holidays"`
	Celebrations  []calendarCelebration   `json:"celebrations"`
	Events        []calendarDetailedEvent `json:"events"`
}

type calendarDetailedTotals struct {
	WorktimeCount    int     `json:"worktime_count"`
	WorktimeMinutes  int     `json:"worktime_minutes"`
	WorktimeHours    float64 `json:"worktime_hours"`
	AbsenceCount     int     `json:"absence_count"`
	AbsenceMinutes   int     `json:"absence_minutes"`
	AbsenceHours     float64 `json:"absence_hours"`
	HolidayCount     int     `json:"holiday_count"`
	DayOffDays       int     `json:"day_off_days"`
	CelebrationDays  int     `json:"celebration_days"`
	CelebrationCount int     `json:"celebration_count"`
}

type calendarDetailedResult struct {
	From            string                 `json:"from"`
	To              string                 `json:"to"`
	Days            int                    `json:"days"`
	WorktimeGroupID int64                  `json:"worktime_group_id"`
	Totals          calendarDetailedTotals `json:"totals"`
	Items           []calendarDetailedDay  `json:"items"`
	Raw             map[string]any         `json:"raw"`
}

func newCalendarDetailedCommand() *cobra.Command {
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
		Use:   "detailed",
		Short: "Generate detailed day-by-day calendar report.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
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
			overview, err := collectCalendarOverview(cmd.Context(), client, fromDate, toDate, user, order, sideload, worktimeGroup)
			if err != nil {
				return err
			}
			detailed := buildCalendarDetailedResult(overview)

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "calendar detailed",
					Data: map[string]any{
						"from":              detailed.From,
						"to":                detailed.To,
						"days":              detailed.Days,
						"worktime_group_id": detailed.WorktimeGroupID,
						"filters": map[string]any{
							"user":     strings.TrimSpace(user),
							"order":    strings.TrimSpace(order),
							"sideload": sideload,
						},
						"duration_format": durationFormat,
						"durations": map[string]any{
							"worktime": durationSummary(detailed.Totals.WorktimeMinutes, durationFormat),
							"absence":  durationSummary(detailed.Totals.AbsenceMinutes, durationFormat),
						},
						"totals": detailed.Totals,
						"items":  detailed.Items,
						"raw":    detailed.Raw,
					},
				})
			}

			renderCalendarDetailedText(cmd.OutOrStdout(), detailed, durationFormat)
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

func buildCalendarDetailedResult(overview calendarOverviewResult) calendarDetailedResult {
	result := calendarDetailedResult{
		From:            overview.From,
		To:              overview.To,
		Days:            overview.Days,
		WorktimeGroupID: overview.WorktimeGroup,
		Items:           make([]calendarDetailedDay, 0, len(overview.Items)),
		Raw:             overview.Raw,
	}

	for _, day := range overview.Items {
		dayReasons := dayOffReasons(day)
		celebrations := collectCelebrations(day)
		events := buildCalendarDayEvents(day, celebrations)

		detailedDay := calendarDetailedDay{
			Date:          day.Date,
			Weekday:       day.Weekday,
			IsWeekend:     day.Weekend,
			IsDayOff:      len(dayReasons) > 0,
			DayOffReasons: dayReasons,
			Worktimes:     day.Worktimes,
			Absences:      day.Absences,
			Holidays:      day.Holidays,
			Celebrations:  celebrations,
			Events:        events,
		}

		if detailedDay.IsDayOff {
			result.Totals.DayOffDays++
		}
		if len(celebrations) > 0 {
			result.Totals.CelebrationDays++
		}
		result.Totals.CelebrationCount += len(celebrations)
		result.Totals.WorktimeCount += day.Worktimes.Count
		result.Totals.WorktimeMinutes += day.Worktimes.TotalMinutes
		result.Totals.AbsenceCount += day.Absences.Count
		result.Totals.AbsenceMinutes += day.Absences.TotalMinutes
		result.Totals.HolidayCount += day.Holidays.Count
		result.Items = append(result.Items, detailedDay)
	}

	result.Totals.WorktimeHours = minutesToHours(result.Totals.WorktimeMinutes)
	result.Totals.AbsenceHours = minutesToHours(result.Totals.AbsenceMinutes)
	return result
}

func dayOffReasons(day calendarOverviewDay) []string {
	reasons := make([]string, 0, 3)
	if day.Weekend {
		reasons = append(reasons, "weekend")
	}
	if day.Holidays.Count > 0 {
		reasons = append(reasons, "public_holiday")
	}
	if day.Absences.TotalMinutes > 0 && day.Worktimes.TotalMinutes == 0 {
		reasons = append(reasons, "absence")
	}
	return reasons
}

func collectCelebrations(day calendarOverviewDay) []calendarCelebration {
	celebrations := make([]calendarCelebration, 0)
	seen := map[string]struct{}{}

	for _, holiday := range day.Holidays.Items {
		name := firstNonEmpty(
			getString(holiday, "desc"),
			getString(holiday, "name"),
			getString(holiday, "title"),
			getString(holiday, "text"),
			getString(holiday, "label"),
		)
		if strings.TrimSpace(name) == "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(name))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		celebrations = append(celebrations, calendarCelebration{
			Name:   name,
			Type:   "public_holiday",
			Source: "workday_calendar",
			Raw:    holiday,
		})
	}

	for _, absence := range day.Absences.Items {
		label := absenceTypeName(absence)
		if !containsCelebrationKeyword(label) {
			continue
		}
		name := firstNonEmpty(label, getString(absence, "description"), "Celebration")
		key := strings.ToLower(strings.TrimSpace(name))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		celebrations = append(celebrations, calendarCelebration{
			Name:   name,
			Type:   "absence_celebration",
			Source: "absence",
			Raw:    absence,
		})
	}

	sort.SliceStable(celebrations, func(i int, j int) bool {
		return celebrations[i].Name < celebrations[j].Name
	})
	return celebrations
}

func containsCelebrationKeyword(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return false
	}
	for _, keyword := range []string{"birthday", "syntym", "anniversary", "celebration", "juhla"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func absenceTypeName(item map[string]any) string {
	if nested, ok := item["abcensetype"].(map[string]any); ok {
		return firstNonEmpty(
			getString(nested, "name"),
			getString(nested, "text"),
		)
	}
	return firstNonEmpty(getString(item, "abcensetype_name"), getString(item, "type"))
}

func buildCalendarDayEvents(day calendarOverviewDay, celebrations []calendarCelebration) []calendarDetailedEvent {
	events := make([]calendarDetailedEvent, 0)

	for _, item := range day.Worktimes.Items {
		label := firstNonEmpty(
			getString(item, "description"),
			formatWorktimeRange(item),
			"Worktime",
		)
		label = normalizeSingleLine(label)
		minutes, _ := worktimeMinutes(item)
		events = append(events, calendarDetailedEvent{
			Type:    "worktime",
			Title:   label,
			Minutes: minutes,
			Raw:     item,
		})
	}

	for _, item := range day.Absences.Items {
		label := firstNonEmpty(
			absenceTypeName(item),
			getString(item, "description"),
			"Absence",
		)
		label = normalizeSingleLine(label)
		minutes, _ := absenceMinutes(item)
		events = append(events, calendarDetailedEvent{
			Type:    "absence",
			Title:   label,
			Minutes: minutes,
			Raw:     item,
		})
	}

	for _, item := range day.Holidays.Items {
		label := firstNonEmpty(
			getString(item, "desc"),
			getString(item, "name"),
			"Public holiday",
		)
		label = normalizeSingleLine(label)
		events = append(events, calendarDetailedEvent{
			Type:  "holiday",
			Title: label,
			Raw:   item,
		})
	}

	for _, item := range celebrations {
		events = append(events, calendarDetailedEvent{
			Type:  "celebration",
			Title: item.Name,
			Raw:   item.Raw,
		})
	}

	sort.SliceStable(events, func(i int, j int) bool {
		if events[i].Type != events[j].Type {
			return events[i].Type < events[j].Type
		}
		return events[i].Title < events[j].Title
	})
	return events
}

func formatWorktimeRange(item map[string]any) string {
	start := firstNonEmpty(getString(item, "starttime"), getString(item, "start_time"))
	end := firstNonEmpty(getString(item, "endtime"), getString(item, "end_time"))
	if start == "" || end == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s", start, end)
}

func renderCalendarDetailedText(out io.Writer, report calendarDetailedResult, durationFormat string) {
	_, _ = fmt.Fprintf(out, "from: %s\n", report.From)
	_, _ = fmt.Fprintf(out, "to: %s\n", report.To)
	_, _ = fmt.Fprintf(out, "days: %d\n", report.Days)
	_, _ = fmt.Fprintf(out, "worktime_group_id: %d\n", report.WorktimeGroupID)
	_, _ = fmt.Fprintf(out, "worktime_minutes: %d\n", report.Totals.WorktimeMinutes)
	_, _ = fmt.Fprintf(out, "worktime_duration: %s\n", formatDurationForText(report.Totals.WorktimeMinutes, durationFormat))
	_, _ = fmt.Fprintf(out, "absence_minutes: %d\n", report.Totals.AbsenceMinutes)
	_, _ = fmt.Fprintf(out, "absence_duration: %s\n", formatDurationForText(report.Totals.AbsenceMinutes, durationFormat))
	_, _ = fmt.Fprintf(out, "holiday_rows: %d\n", report.Totals.HolidayCount)
	_, _ = fmt.Fprintf(out, "day_off_days: %d\n", report.Totals.DayOffDays)
	_, _ = fmt.Fprintf(out, "celebration_days: %d\n", report.Totals.CelebrationDays)
	_, _ = fmt.Fprintln(out)

	for _, day := range report.Items {
		reasons := "working_day"
		if len(day.DayOffReasons) > 0 {
			reasons = strings.Join(day.DayOffReasons, ",")
		}
		_, _ = fmt.Fprintf(out, "%s (%s) day_off=%t reasons=%s\n", day.Date, day.Weekday, day.IsDayOff, reasons)
		_, _ = fmt.Fprintf(out, "  worktimes: %d (%s)\n", day.Worktimes.Count, formatDurationForText(day.Worktimes.TotalMinutes, durationFormat))
		_, _ = fmt.Fprintf(out, "  absences: %d (%s)\n", day.Absences.Count, formatDurationForText(day.Absences.TotalMinutes, durationFormat))
		_, _ = fmt.Fprintf(out, "  holidays: %d\n", day.Holidays.Count)
		_, _ = fmt.Fprintf(out, "  celebrations: %d\n", len(day.Celebrations))
		for _, event := range day.Events {
			if event.Minutes > 0 {
				_, _ = fmt.Fprintf(out, "    - [%s] %s (%s)\n", event.Type, event.Title, formatDurationForText(event.Minutes, durationFormat))
				continue
			}
			_, _ = fmt.Fprintf(out, "    - [%s] %s\n", event.Type, event.Title)
		}
	}
}

func normalizeSingleLine(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}
