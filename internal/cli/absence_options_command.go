package cli

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/mekedron/otta-cli/internal/otta"
	"github.com/spf13/cobra"
)

type absenceOptionsSet struct {
	Types []absenceOption `json:"types"`
	Users []absenceOption `json:"users"`
}

type absenceOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name,omitempty"`
}

func newAbsenceOptionsCommand() *cobra.Command {
	var (
		typeFilter   string
		mode         string
		userID       int64
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "options",
		Short: "List selectable values for absence creation.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
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

			client := newAPIClient(cfg, configPath)
			resolvedUserID := resolveAbsenceUserID(userID, cache)
			resolvedMode := ""
			if cmd.Flags().Changed("mode") {
				resolvedMode, err = parseAbsenceMode(mode, false)
				if err != nil {
					return err
				}
			}

			typesQuery, err := buildAbsenceTypeOptionsQuery(strings.TrimSpace(typeFilter), resolvedMode, resolvedUserID)
			if err != nil {
				return err
			}
			typesRaw, err := requestAbsenceOptions(cmd.Context(), client, "/abcense/abcensetypes", typesQuery)
			if err != nil {
				return err
			}

			usersRaw, err := requestAbsenceOptions(cmd.Context(), client, "/abcense/users", map[string]string{"limit": "100", "offset": "0"})
			if err != nil {
				return err
			}

			options := absenceOptionsSet{
				Types: parseAbsenceOptions(typesRaw["abcensetypes"]),
				Users: parseAbsenceOptions(usersRaw["users"]),
			}

			raw := map[string]any{
				"types": typesRaw,
				"users": usersRaw,
			}

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "absence options",
					Data: map[string]any{
						"filters": map[string]any{
							"type":       strings.TrimSpace(typeFilter),
							"mode":       resolvedMode,
							"user":       resolvedUserID,
							"type_query": typesQuery["type"],
						},
						"options": options,
						"raw":     raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "types: %d\n", len(options.Types))
			for _, item := range options.Types {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d  %s\n", item.ID, displayOptionName(item.Name))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "users: %d\n", len(options.Users))
			for _, item := range options.Users {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d  %s\n", item.ID, displayOptionName(item.Name))
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	cmd.Flags().StringVar(&typeFilter, "type", "", "Optional absence type filter passed to API (e.g. days, both).")
	cmd.Flags().StringVar(&mode, "mode", "", "Optional absence mode for type options: days or hours.")
	cmd.Flags().Int64Var(&userID, "user", 0, "Optional user id (required when --mode is used if no fallback user is configured).")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func requestAbsenceOptions(ctx context.Context, client *otta.Client, endpoint string, query map[string]string) (map[string]any, error) {
	var raw map[string]any
	if err := client.Request(ctx, http.MethodGet, endpoint, query, nil, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return map[string]any{}, nil
	}
	return raw, nil
}

func buildAbsenceTypeOptionsQuery(rawTypeFilter string, mode string, userID int64) (map[string]string, error) {
	query := map[string]string{
		"limit":  "100",
		"offset": "0",
	}
	if rawTypeFilter != "" && mode != "" {
		return nil, fmt.Errorf("--type and --mode cannot be used together")
	}
	if mode != "" {
		if userID <= 0 {
			return nil, fmt.Errorf("--user is required when --mode is set (or set OTTA_CLI_USER_ID / run `otta status` to refresh cache)")
		}
		query["type"] = absenceTypeFilterForMode(mode)
		query["user"] = strconv.FormatInt(userID, 10)
		return query, nil
	}
	if rawTypeFilter != "" {
		query["type"] = rawTypeFilter
	}
	return query, nil
}

func fetchModeAbsenceTypeOptions(ctx context.Context, client *otta.Client, mode string, userID int64) ([]absenceOption, map[string]any, error) {
	query, err := buildAbsenceTypeOptionsQuery("", mode, userID)
	if err != nil {
		return nil, nil, err
	}
	raw, err := requestAbsenceOptions(ctx, client, "/abcense/abcensetypes", query)
	if err != nil {
		return nil, nil, err
	}
	return parseAbsenceOptions(raw["abcensetypes"]), raw, nil
}

func containsAbsenceOptionID(options []absenceOption, optionID int64) bool {
	for _, item := range options {
		if item.ID == optionID {
			return true
		}
	}
	return false
}

func formatAbsenceOptionIDs(options []absenceOption, limit int) string {
	if len(options) == 0 {
		return "(none)"
	}
	if limit <= 0 || limit > len(options) {
		limit = len(options)
	}

	parts := make([]string, 0, limit+1)
	for idx := 0; idx < limit; idx++ {
		item := options[idx]
		parts = append(parts, fmt.Sprintf("%d=%s", item.ID, displayOptionName(item.Name)))
	}
	if limit < len(options) {
		parts = append(parts, fmt.Sprintf("... (%d more)", len(options)-limit))
	}
	return strings.Join(parts, ", ")
}

func parseAbsenceOptions(raw any) []absenceOption {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}

	result := make([]absenceOption, 0, len(list))
	for _, item := range list {
		option, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := toInt64(option["id"])
		if id <= 0 {
			continue
		}
		name := firstNonEmpty(
			getString(option, "text"),
			getString(option, "name"),
		)
		if values, ok := option["values"].(map[string]any); ok {
			name = firstNonEmpty(name, getString(values, "name"), strings.TrimSpace(getString(values, "lastname")+" "+getString(values, "firstname")))
		}
		result = append(result, absenceOption{ID: id, Name: strings.TrimSpace(name)})
	}

	return result
}
