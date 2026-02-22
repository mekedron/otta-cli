package cli

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/mekedron/otta-cli/internal/otta"
	"github.com/spf13/cobra"
)

func fetchAbsenceByID(cmd *cobra.Command, client *otta.Client, id int64) (map[string]any, any, error) {
	var raw any
	if err := client.Request(cmd.Context(), http.MethodGet, fmt.Sprintf("/abcenses/%d", id), nil, nil, &raw); err != nil {
		return nil, nil, err
	}

	item := extractAbsenceItem(raw)
	if item == nil {
		return nil, nil, fmt.Errorf("absence %d not found", id)
	}

	return item, raw, nil
}

func extractAbsenceItem(raw any) map[string]any {
	if typed, ok := raw.(map[string]any); ok {
		for _, key := range []string{"abcense", "absence", "item", "data"} {
			value, ok := typed[key]
			if !ok {
				continue
			}
			item, ok := value.(map[string]any)
			if !ok {
				continue
			}
			return cloneAnyMap(item)
		}
		if toInt64(typed["id"]) > 0 {
			return cloneAnyMap(typed)
		}
	}

	return nil
}

func absenceTypeIDValue(value any) int64 {
	if item, ok := value.(map[string]any); ok {
		return toInt64(item["id"])
	}
	return toInt64(value)
}

func resolveAbsenceUserID(userID int64, cache *config.Cache) int64 {
	if userID > 0 {
		return userID
	}
	if value, ok := config.EnvInt64(config.EnvUserID); ok {
		return value
	}
	if cache != nil && cache.User.ID > 0 {
		return cache.User.ID
	}
	return 0
}

func validateOptionalHHMM(value string, flagName string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return validateHHMM(value, flagName)
}
