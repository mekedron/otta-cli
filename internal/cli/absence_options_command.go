package cli

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mekedron/otta-cli/internal/config"
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

			client := newAPIClient(cfg, configPath)
			request := func(endpoint string, query map[string]string) (map[string]any, error) {
				var raw map[string]any
				if err := client.Request(cmd.Context(), http.MethodGet, endpoint, query, nil, &raw); err != nil {
					return nil, err
				}
				if raw == nil {
					return map[string]any{}, nil
				}
				return raw, nil
			}

			typesQuery := map[string]string{"limit": "100", "offset": "0"}
			if strings.TrimSpace(typeFilter) != "" {
				typesQuery["type"] = strings.TrimSpace(typeFilter)
			}
			typesRaw, err := request("/abcense/abcensetypes", typesQuery)
			if err != nil {
				return err
			}

			usersRaw, err := request("/abcense/users", map[string]string{"limit": "100", "offset": "0"})
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
							"type": strings.TrimSpace(typeFilter),
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
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
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
