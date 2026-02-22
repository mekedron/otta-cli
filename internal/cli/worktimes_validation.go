package cli

import (
	"fmt"
	"regexp"
	"strings"
)

var hhmmPattern = regexp.MustCompile(`^\d{2}:\d{2}$`)

func validateHHMM(value string, flagName string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s is required", flagName)
	}
	if !hhmmPattern.MatchString(trimmed) {
		return fmt.Errorf("%s must be HH:MM", flagName)
	}
	return nil
}
