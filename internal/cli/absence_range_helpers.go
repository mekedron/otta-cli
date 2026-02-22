package cli

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mekedron/otta-cli/internal/otta"
)

type absenceDailyResponse struct {
	Date         string  `json:"date"`
	Count        int     `json:"count"`
	TotalMinutes int     `json:"total_minutes"`
	TotalHours   float64 `json:"total_hours"`
}

type absenceRangeResult struct {
	From         string                 `json:"from"`
	To           string                 `json:"to"`
	Days         int                    `json:"days"`
	Count        int                    `json:"count"`
	TotalMinutes int                    `json:"total_minutes"`
	TotalHours   float64                `json:"total_hours"`
	Items        []map[string]any       `json:"items"`
	Responses    []absenceDailyResponse `json:"responses"`
	Raw          any                    `json:"raw,omitempty"`
}

func collectAbsenceRange(
	ctx context.Context,
	client *otta.Client,
	fromDate time.Time,
	toDate time.Time,
	user string,
	order string,
	sideload bool,
) (absenceRangeResult, error) {
	result := absenceRangeResult{
		From:      formatISODate(fromDate),
		To:        formatISODate(toDate),
		Items:     make([]map[string]any, 0),
		Responses: make([]absenceDailyResponse, 0),
	}

	trimmedUser := strings.TrimSpace(user)
	trimmedOrder := strings.TrimSpace(order)
	query := map[string]string{
		"startdate": result.From,
		"enddate":   result.To,
		"order":     trimmedOrder,
	}
	if trimmedUser != "" {
		query["user"] = trimmedUser
	}
	if sideload {
		query["sideload[]"] = "abcensetype.name"
	}

	var raw any
	if err := client.Request(ctx, http.MethodGet, "/ttapi/absence/split", query, nil, &raw); err != nil {
		return absenceRangeResult{}, err
	}

	rows := extractAbsenceRows(raw)
	sort.SliceStable(rows, func(i int, j int) bool {
		return absenceRowLess(rows[i], rows[j])
	})

	result.Raw = raw
	result.Items = rows
	grouped := map[string][]map[string]any{}
	for _, item := range rows {
		dateValue := absenceDate(item)
		if dateValue == "" {
			continue
		}
		grouped[dateValue] = append(grouped[dateValue], item)
	}

	for current := fromDate; !current.After(toDate); current = current.AddDate(0, 0, 1) {
		dateValue := formatISODate(current)
		dayItems := grouped[dateValue]
		minutes := sumAbsenceMinutes(dayItems)

		result.Responses = append(result.Responses, absenceDailyResponse{
			Date:         dateValue,
			Count:        len(dayItems),
			TotalMinutes: minutes,
			TotalHours:   minutesToHours(minutes),
		})

		result.Count += len(dayItems)
		result.TotalMinutes += minutes
	}

	result.Days = len(result.Responses)
	result.TotalHours = minutesToHours(result.TotalMinutes)
	return result, nil
}

func extractAbsenceRows(raw any) []map[string]any {
	var list []any

	switch typed := raw.(type) {
	case []any:
		list = typed
	case map[string]any:
		for _, key := range []string{"abcenses", "absences", "items", "results", "data"} {
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
		normalizedDate := normalizedISODate(
			firstNonEmpty(
				getString(normalized, "date"),
				getString(normalized, "startdate"),
			),
		)
		if normalizedDate != "" {
			normalized["date"] = normalizedDate
		}
		rows = append(rows, normalized)
	}

	return rows
}

func absenceRowLess(left map[string]any, right map[string]any) bool {
	leftDate := absenceDate(left)
	rightDate := absenceDate(right)
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

func absenceDate(item map[string]any) string {
	return normalizedISODate(
		firstNonEmpty(
			getString(item, "date"),
			getString(item, "startdate"),
		),
	)
}

func sumAbsenceMinutes(items []map[string]any) int {
	total := 0
	for _, item := range items {
		minutes, ok := absenceMinutes(item)
		if !ok {
			continue
		}
		total += minutes
	}
	return total
}

func absenceMinutes(item map[string]any) (int, bool) {
	if hours, ok := toFloat64(item["absence_hours"]); ok && hours > 0 {
		return int(math.Round(hours * 60)), true
	}
	if hours, ok := toFloat64(item["hours"]); ok && hours > 0 {
		return int(math.Round(hours * 60)), true
	}
	if rule, ok := item["rule"].(map[string]any); ok {
		if value := toInt64(rule["minutes"]); value > 0 {
			return int(value), true
		}
		if value := toInt64(rule["absence_minutes"]); value > 0 {
			return int(value), true
		}
	}

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
	if totalMinutes <= 0 {
		return 0, false
	}

	return totalMinutes, true
}

func normalizedISODate(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if _, err := parseISODate(trimmed); err == nil {
		return trimmed
	}
	if len(trimmed) >= len("2006-01-02") {
		candidate := trimmed[:10]
		if _, err := parseISODate(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func minutesToHours(minutes int) float64 {
	if minutes <= 0 {
		return 0
	}
	return float64(minutes) / 60
}

func toFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
