package cli

import (
	"fmt"
	"strings"
)

const (
	absenceModeAuto  = "auto"
	absenceModeDays  = "days"
	absenceModeHours = "hours"
)

func parseAbsenceMode(value string, allowAuto bool) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "day", "days":
		return absenceModeDays, nil
	case "hour", "hours":
		return absenceModeHours, nil
	case "auto":
		if allowAuto {
			return absenceModeAuto, nil
		}
	}

	if allowAuto {
		return "", fmt.Errorf("--mode must be one of: auto,days,hours")
	}
	return "", fmt.Errorf("--mode must be one of: days,hours")
}

func resolveAbsenceAddMode(requested string, startChanged bool, endChanged bool, hoursChanged bool) (string, error) {
	mode, err := parseAbsenceMode(requested, true)
	if err != nil {
		return "", err
	}
	if mode != absenceModeAuto {
		return mode, nil
	}
	if startChanged || endChanged || hoursChanged {
		return absenceModeHours, nil
	}
	return absenceModeDays, nil
}

func absenceTypeFilterForMode(mode string) string {
	switch mode {
	case absenceModeHours:
		return "both||hours||(empty)"
	default:
		return "both||days||(empty)"
	}
}
