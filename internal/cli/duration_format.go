package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
)

const (
	durationFormatMinutes = "minutes"
	durationFormatHours   = "hours"
	durationFormatDays    = "days"
	durationFormatHHMM    = "hhmm"
)

func resolveDurationFormat(cmd *cobra.Command) (string, error) {
	if cmd == nil {
		return durationFormatMinutes, nil
	}

	value, err := cmd.Flags().GetString("duration-format")
	if err != nil {
		return "", err
	}

	resolved := normalizeDurationFormat(value)
	if resolved == "" {
		return "", fmt.Errorf("--duration-format must be one of: minutes,hours,days,hhmm")
	}
	return resolved, nil
}

func normalizeDurationFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "minutes", "minute", "min", "mins", "m":
		return durationFormatMinutes
	case "hours", "hour", "hr", "hrs", "h":
		return durationFormatHours
	case "days", "day", "d":
		return durationFormatDays
	case "hhmm", "hh:mm", "h:mm", "clock":
		return durationFormatHHMM
	default:
		return ""
	}
}

func durationSummary(minutes int, format string) map[string]any {
	value, text := durationValueAndText(minutes, format)
	return map[string]any{
		"format":  format,
		"minutes": minutes,
		"value":   value,
		"text":    text,
	}
}

func durationValueAndText(minutes int, format string) (any, string) {
	resolved := normalizeDurationFormat(format)
	if resolved == "" {
		resolved = durationFormatMinutes
	}

	switch resolved {
	case durationFormatHours:
		hours := roundDuration(float64(minutes)/60, 4)
		return hours, fmt.Sprintf("%.2f hours", hours)
	case durationFormatDays:
		days := roundDuration(float64(minutes)/(24*60), 6)
		return days, fmt.Sprintf("%.4f days", days)
	case durationFormatHHMM:
		value := formatMinutesHHMM(minutes)
		return value, value + " hh:mm"
	case durationFormatMinutes:
		fallthrough
	default:
		return minutes, fmt.Sprintf("%d minutes", minutes)
	}
}

func formatDurationForText(minutes int, format string) string {
	_, text := durationValueAndText(minutes, format)
	return text
}

func roundDuration(value float64, digits int) float64 {
	if digits <= 0 {
		return math.Round(value)
	}
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}

func formatMinutesHHMM(minutes int) string {
	if minutes <= 0 {
		return "0:00"
	}
	hours := minutes / 60
	rest := minutes % 60
	return fmt.Sprintf("%d:%02d", hours, rest)
}
