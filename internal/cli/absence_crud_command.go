package cli

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/mekedron/otta-cli/internal/otta"
	"github.com/spf13/cobra"
)

func newAbsenceAddCommand() *cobra.Command {
	var (
		dateFrom      string
		dateTo        string
		startTime     string
		endTime       string
		absenceTypeID int64
		userID        int64
		dayAmount     float64
		absenceHours  float64
		description   string
		outputFormat  string
	)

	today := formatISODate(time.Now().UTC())

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create an absence entry.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}

			if _, _, err := parseWorktimesDateRange(dateFrom, dateTo); err != nil {
				return err
			}
			if err := validateOptionalHHMM(startTime, "--start"); err != nil {
				return err
			}
			if err := validateOptionalHHMM(endTime, "--end"); err != nil {
				return err
			}
			if absenceTypeID <= 0 {
				return fmt.Errorf("--type is required")
			}
			if cmd.Flags().Changed("dayamount") && dayAmount <= 0 {
				return fmt.Errorf("--dayamount must be > 0")
			}
			if cmd.Flags().Changed("hours") && absenceHours < 0 {
				return fmt.Errorf("--hours must be >= 0")
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
			resolvedUserID := resolveAbsenceUserID(userID, cache)
			if resolvedUserID <= 0 {
				return fmt.Errorf("--user is required (or set OTTA_CLI_USER_ID / run `otta status` to refresh cache)")
			}

			absence := map[string]any{
				"user":        resolvedUserID,
				"abcensetype": absenceTypeID,
				"startdate":   strings.TrimSpace(dateFrom),
				"starttime":   strings.TrimSpace(startTime),
				"enddate":     strings.TrimSpace(dateTo),
				"endtime":     strings.TrimSpace(endTime),
				"description": description,
			}
			if cmd.Flags().Changed("dayamount") {
				absence["dayamount"] = dayAmount
			}
			if cmd.Flags().Changed("hours") {
				absence["absence_hours"] = absenceHours
			}

			body := map[string]any{"abcense": absence}

			client := newAPIClient(cfg, configPath)
			var raw any
			if err := client.Request(cmd.Context(), http.MethodPost, "/abcenses", nil, body, &raw); err != nil {
				return err
			}

			created := extractAbsenceItem(raw)
			createdID := toInt64(created["id"])
			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "absence add",
					Data: map[string]any{
						"id":  createdID,
						"raw": raw,
					},
				})
			}

			if createdID > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "absence created: %d\n", createdID)
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "absence created")
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	cmd.Flags().StringVar(&dateFrom, "from", today, "Start date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&dateTo, "to", today, "End date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&startTime, "start", "", "Optional start time in HH:MM (empty for whole-day absence).")
	cmd.Flags().StringVar(&endTime, "end", "", "Optional end time in HH:MM (empty for whole-day absence).")
	cmd.Flags().Int64Var(&absenceTypeID, "type", 0, "Absence type id (see `otta absence options`).")
	cmd.Flags().Int64Var(&userID, "user", 0, "User id.")
	cmd.Flags().Float64Var(&dayAmount, "dayamount", 0, "Optional day amount (for example 1 or 0.5).")
	cmd.Flags().Float64Var(&absenceHours, "hours", 0, "Optional absence duration in hours.")
	cmd.Flags().StringVar(&description, "description", "", "Optional absence description.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func newAbsenceReadCommand() *cobra.Command {
	var (
		id           int64
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read an absence entry by id.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}
			if id <= 0 {
				return fmt.Errorf("--id is required")
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
			item, raw, err := fetchAbsenceByID(cmd, client, id)
			if err != nil {
				return err
			}

			minutes, _ := absenceMinutes(item)
			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "absence read",
					Data: map[string]any{
						"id":            id,
						"item":          item,
						"total_minutes": minutes,
						"total_hours":   minutesToHours(minutes),
						"raw":           raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "id: %d\n", id)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "type: %d\n", absenceTypeIDValue(item["abcensetype"]))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "user: %d\n", toInt64(item["user"]))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "from: %s\n", getString(item, "startdate"))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "to: %s\n", getString(item, "enddate"))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "start: %s\n", getString(item, "starttime"))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "end: %s\n", getString(item, "endtime"))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "description: %s\n", getString(item, "description"))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "total_duration: %s\n", formatDurationForText(minutes, durationFormatMinutes))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	cmd.Flags().Int64Var(&id, "id", 0, "Absence id.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func newAbsenceUpdateCommand() *cobra.Command {
	var (
		id           int64
		dateFrom     string
		dateTo       string
		startTime    string
		endTime      string
		absenceType  int64
		userID       int64
		dayAmount    float64
		absenceHours float64
		description  string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an absence entry.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}
			if id <= 0 {
				return fmt.Errorf("--id is required")
			}

			changedAny := cmd.Flags().Changed("from") ||
				cmd.Flags().Changed("to") ||
				cmd.Flags().Changed("start") ||
				cmd.Flags().Changed("end") ||
				cmd.Flags().Changed("type") ||
				cmd.Flags().Changed("user") ||
				cmd.Flags().Changed("dayamount") ||
				cmd.Flags().Changed("hours") ||
				cmd.Flags().Changed("description")
			if !changedAny {
				return fmt.Errorf("no fields to update")
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
			existing, _, err := fetchAbsenceByID(cmd, client, id)
			if err != nil {
				return err
			}

			absence := cloneAnyMap(existing)

			currentFrom := strings.TrimSpace(getString(absence, "startdate"))
			if currentFrom == "" {
				currentFrom = strings.TrimSpace(getString(absence, "date"))
			}
			currentTo := strings.TrimSpace(getString(absence, "enddate"))
			if currentTo == "" {
				currentTo = currentFrom
			}

			if cmd.Flags().Changed("from") {
				currentFrom = strings.TrimSpace(dateFrom)
			}
			if cmd.Flags().Changed("to") {
				currentTo = strings.TrimSpace(dateTo)
			}
			if _, _, err := parseWorktimesDateRange(currentFrom, currentTo); err != nil {
				return err
			}
			absence["startdate"] = currentFrom
			absence["enddate"] = currentTo

			if cmd.Flags().Changed("start") {
				if err := validateOptionalHHMM(startTime, "--start"); err != nil {
					return err
				}
				absence["starttime"] = strings.TrimSpace(startTime)
			}
			if cmd.Flags().Changed("end") {
				if err := validateOptionalHHMM(endTime, "--end"); err != nil {
					return err
				}
				absence["endtime"] = strings.TrimSpace(endTime)
			}
			if cmd.Flags().Changed("type") {
				if absenceType <= 0 {
					return fmt.Errorf("--type must be > 0")
				}
				absence["abcensetype"] = absenceType
			}
			if cmd.Flags().Changed("user") {
				if userID <= 0 {
					return fmt.Errorf("--user must be > 0")
				}
				absence["user"] = userID
			}
			if cmd.Flags().Changed("dayamount") {
				if dayAmount <= 0 {
					return fmt.Errorf("--dayamount must be > 0")
				}
				absence["dayamount"] = dayAmount
			}
			if cmd.Flags().Changed("hours") {
				if absenceHours < 0 {
					return fmt.Errorf("--hours must be >= 0")
				}
				absence["absence_hours"] = absenceHours
			}
			if cmd.Flags().Changed("description") {
				absence["description"] = description
			}

			body := map[string]any{"abcense": absence}
			var raw any
			if err := client.Request(cmd.Context(), http.MethodPut, fmt.Sprintf("/abcenses/%d", id), nil, body, &raw); err != nil {
				return err
			}

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "absence update",
					Data: map[string]any{
						"id":  id,
						"raw": raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "absence updated: %d\n", id)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	cmd.Flags().Int64Var(&id, "id", 0, "Absence id.")
	cmd.Flags().StringVar(&dateFrom, "from", "", "Start date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&dateTo, "to", "", "End date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&startTime, "start", "", "Optional start time in HH:MM (empty for whole-day absence).")
	cmd.Flags().StringVar(&endTime, "end", "", "Optional end time in HH:MM (empty for whole-day absence).")
	cmd.Flags().Int64Var(&absenceType, "type", 0, "Absence type id.")
	cmd.Flags().Int64Var(&userID, "user", 0, "User id.")
	cmd.Flags().Float64Var(&dayAmount, "dayamount", 0, "Optional day amount (for example 1 or 0.5).")
	cmd.Flags().Float64Var(&absenceHours, "hours", 0, "Optional absence duration in hours.")
	cmd.Flags().StringVar(&description, "description", "", "Absence description.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func newAbsenceDeleteCommand() *cobra.Command {
	var (
		id           int64
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an absence entry.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}
			if id <= 0 {
				return fmt.Errorf("--id is required")
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
			var raw any
			if err := client.Request(cmd.Context(), http.MethodDelete, fmt.Sprintf("/abcenses/%d", id), nil, nil, &raw); err != nil {
				return err
			}

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "absence delete",
					Data: map[string]any{
						"id":  id,
						"raw": raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "absence deleted: %d\n", id)
			return nil
		},
	}

	cmd.Flags().Int64Var(&id, "id", 0, "Absence id.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

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
	if cache != nil {
		if cache.User.ID > 0 {
			return cache.User.ID
		}
	}
	return 0
}

func validateOptionalHHMM(value string, flagName string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return validateHHMM(value, flagName)
}
