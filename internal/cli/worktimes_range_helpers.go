package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mekedron/otta-cli/internal/otta"
)

var worktimeCSVHeaders = []string{
	"date",
	"id",
	"starttime",
	"endtime",
	"pause",
	"minutes",
	"project",
	"worktype",
	"task",
	"subtask",
	"superior",
	"user",
	"status",
	"description",
}

type worktimeDailyResponse struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
	Raw   any    `json:"raw,omitempty"`
}

type worktimeRangeResult struct {
	From         string                  `json:"from"`
	To           string                  `json:"to"`
	Days         int                     `json:"days"`
	Count        int                     `json:"count"`
	TotalMinutes int                     `json:"total_minutes"`
	Items        []map[string]any        `json:"items"`
	Responses    []worktimeDailyResponse `json:"responses"`
}

func parseWorktimesDateRange(dateFrom string, dateTo string) (time.Time, time.Time, error) {
	fromDate, err := parseISODate(dateFrom)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--from must be YYYY-MM-DD")
	}
	toDate, err := parseISODate(dateTo)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--to must be YYYY-MM-DD")
	}
	if toDate.Before(fromDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("--to must be greater than or equal to --from")
	}
	return fromDate, toDate, nil
}

func collectWorktimesRange(
	ctx context.Context,
	client *otta.Client,
	fromDate time.Time,
	toDate time.Time,
	user string,
	order string,
	sideload bool,
) (worktimeRangeResult, error) {
	result := worktimeRangeResult{
		From:      formatISODate(fromDate),
		To:        formatISODate(toDate),
		Items:     make([]map[string]any, 0),
		Responses: make([]worktimeDailyResponse, 0),
	}

	trimmedUser := strings.TrimSpace(user)
	trimmedOrder := strings.TrimSpace(order)

	for current := fromDate; !current.After(toDate); current = current.AddDate(0, 0, 1) {
		dateValue := formatISODate(current)
		query := map[string]string{
			"date":     dateValue,
			"order":    trimmedOrder,
			"sideload": strconv.FormatBool(sideload),
		}
		if trimmedUser != "" {
			query["user"] = trimmedUser
		}

		var raw any
		if err := client.Request(ctx, http.MethodGet, "/worktimes", query, nil, &raw); err != nil {
			return worktimeRangeResult{}, err
		}

		rows := extractWorktimeRows(raw, dateValue)
		result.Items = append(result.Items, rows...)
		result.Responses = append(result.Responses, worktimeDailyResponse{
			Date:  dateValue,
			Count: len(rows),
			Raw:   raw,
		})
	}

	result.Days = len(result.Responses)
	result.Count = len(result.Items)
	result.TotalMinutes = sumWorktimeMinutes(result.Items)

	return result, nil
}

func extractWorktimeRows(raw any, fallbackDate string) []map[string]any {
	root, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	for _, key := range []string{"worktimes", "items", "results", "data"} {
		list, ok := root[key].([]any)
		if !ok {
			continue
		}

		rows := make([]map[string]any, 0, len(list))
		for _, item := range list {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}

			normalized := cloneAnyMap(row)
			if getString(normalized, "date") == "" {
				normalized["date"] = fallbackDate
			}
			rows = append(rows, normalized)
		}
		return rows
	}

	return nil
}

func cloneAnyMap(input map[string]any) map[string]any {
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func writeWorktimesCSV(out io.Writer, items []map[string]any) error {
	writer := csv.NewWriter(out)
	if err := writer.Write(worktimeCSVHeaders); err != nil {
		return err
	}

	sorted := append([]map[string]any(nil), items...)
	sort.SliceStable(sorted, func(i int, j int) bool {
		return worktimeRowLess(sorted[i], sorted[j])
	})

	for _, item := range sorted {
		minutesValue := ""
		if minutes, ok := worktimeMinutes(item); ok {
			minutesValue = strconv.Itoa(minutes)
		}

		row := []string{
			getString(item, "date"),
			worktimeCSVValue(item["id"]),
			firstNonEmpty(getString(item, "starttime"), getString(item, "start_time")),
			firstNonEmpty(getString(item, "endtime"), getString(item, "end_time")),
			worktimeCSVValue(item["pause"]),
			minutesValue,
			worktimeCSVValue(item["project"]),
			worktimeCSVValue(item["worktype"]),
			worktimeCSVValue(item["task"]),
			worktimeCSVValue(item["subtask"]),
			worktimeCSVValue(item["superior"]),
			worktimeCSVValue(item["user"]),
			getString(item, "status"),
			getString(item, "description"),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

func worktimeRowLess(left map[string]any, right map[string]any) bool {
	leftDate := getString(left, "date")
	rightDate := getString(right, "date")
	if leftDate != rightDate {
		return leftDate < rightDate
	}

	leftStart := firstNonEmpty(getString(left, "starttime"), getString(left, "start_time"))
	rightStart := firstNonEmpty(getString(right, "starttime"), getString(right, "start_time"))
	if leftStart != rightStart {
		return leftStart < rightStart
	}

	leftEnd := firstNonEmpty(getString(left, "endtime"), getString(left, "end_time"))
	rightEnd := firstNonEmpty(getString(right, "endtime"), getString(right, "end_time"))
	if leftEnd != rightEnd {
		return leftEnd < rightEnd
	}

	leftID := toInt64(left["id"])
	rightID := toInt64(right["id"])
	if leftID != rightID {
		return leftID < rightID
	}

	return false
}

func sumWorktimeMinutes(items []map[string]any) int {
	total := 0
	for _, item := range items {
		minutes, ok := worktimeMinutes(item)
		if !ok {
			continue
		}
		total += minutes
	}
	return total
}

func worktimeMinutes(item map[string]any) (int, bool) {
	startValue := firstNonEmpty(getString(item, "starttime"), getString(item, "start_time"))
	endValue := firstNonEmpty(getString(item, "endtime"), getString(item, "end_time"))
	if startValue == "" || endValue == "" {
		return 0, false
	}

	startTime, err := time.Parse("15:04", startValue)
	if err != nil {
		return 0, false
	}
	endTime, err := time.Parse("15:04", endValue)
	if err != nil {
		return 0, false
	}

	totalMinutes := int(endTime.Sub(startTime).Minutes())
	if totalMinutes < 0 {
		totalMinutes += 24 * 60
	}

	pauseMinutes := int(toInt64(item["pause"]))
	if pauseMinutes < 0 {
		pauseMinutes = 0
	}

	netMinutes := totalMinutes - pauseMinutes
	if netMinutes < 0 {
		netMinutes = 0
	}

	return netMinutes, true
}

func worktimeCSVValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	case map[string]any:
		if id := toInt64(typed["id"]); id > 0 {
			return strconv.FormatInt(id, 10)
		}
		label := firstNonEmpty(
			getString(typed, "text"),
			getString(typed, "name"),
			getString(typed, "label"),
			getString(typed, "title"),
		)
		if label != "" {
			return label
		}
		encoded, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(encoded)
	case []any:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(encoded)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}
