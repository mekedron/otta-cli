package cli

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

func newAbsenceAddCommand() *cobra.Command {
	var (
		dateFrom      string
		dateTo        string
		mode          string
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

			if absenceTypeID <= 0 {
				return fmt.Errorf("--type is required")
			}
			startChanged := cmd.Flags().Changed("start")
			endChanged := cmd.Flags().Changed("end")
			dayAmountChanged := cmd.Flags().Changed("dayamount")
			hoursChanged := cmd.Flags().Changed("hours")

			resolvedMode, err := resolveAbsenceAddMode(mode, startChanged, endChanged, hoursChanged)
			if err != nil {
				return err
			}

			resolvedDateFrom := strings.TrimSpace(dateFrom)
			resolvedDateTo := strings.TrimSpace(dateTo)
			switch resolvedMode {
			case absenceModeDays:
				if startChanged || endChanged {
					return fmt.Errorf("--start and --end are only supported when --mode=hours")
				}
				if hoursChanged {
					return fmt.Errorf("--hours is only supported when --mode=hours")
				}
			case absenceModeHours:
				if err := validateHHMM(startTime, "--start"); err != nil {
					return err
				}
				if err := validateHHMM(endTime, "--end"); err != nil {
					return err
				}
				if dayAmountChanged {
					return fmt.Errorf("--dayamount is only supported when --mode=days")
				}
				if cmd.Flags().Changed("to") && strings.TrimSpace(dateTo) != resolvedDateFrom {
					return fmt.Errorf("--to must match --from when --mode=hours")
				}
				resolvedDateTo = resolvedDateFrom
			}

			if _, _, err := parseWorktimesDateRange(resolvedDateFrom, resolvedDateTo); err != nil {
				return err
			}
			if dayAmountChanged && dayAmount <= 0 {
				return fmt.Errorf("--dayamount must be > 0")
			}
			if hoursChanged && absenceHours < 0 {
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
			client := newAPIClient(cfg, configPath)
			availableTypes, _, err := fetchModeAbsenceTypeOptions(cmd.Context(), client, resolvedMode, resolvedUserID)
			if err != nil {
				return fmt.Errorf("load absence types for --mode=%s: %w", resolvedMode, err)
			}
			if !containsAbsenceOptionID(availableTypes, absenceTypeID) {
				return fmt.Errorf("--type %d is not available for --mode=%s; available: %s", absenceTypeID, resolvedMode, formatAbsenceOptionIDs(availableTypes, 8))
			}

			absence := map[string]any{
				"user":        resolvedUserID,
				"abcensetype": absenceTypeID,
				"startdate":   resolvedDateFrom,
				"enddate":     resolvedDateTo,
				"description": description,
			}
			if resolvedMode == absenceModeHours {
				absence["starttime"] = strings.TrimSpace(startTime)
				absence["endtime"] = strings.TrimSpace(endTime)
			}
			if dayAmountChanged {
				absence["dayamount"] = dayAmount
			}
			if hoursChanged {
				absence["absence_hours"] = absenceHours
			}

			body := map[string]any{"abcense": absence}
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
						"id":   createdID,
						"mode": resolvedMode,
						"raw":  raw,
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
	cmd.Flags().StringVar(&dateTo, "to", today, "End date in YYYY-MM-DD (days mode; must equal --from for hours mode).")
	cmd.Flags().StringVar(&mode, "mode", absenceModeAuto, "Absence mode: auto, days, hours.")
	cmd.Flags().StringVar(&startTime, "start", "", "Start time in HH:MM (required for hours mode).")
	cmd.Flags().StringVar(&endTime, "end", "", "End time in HH:MM (required for hours mode).")
	cmd.Flags().Int64Var(&absenceTypeID, "type", 0, "Absence type id (see otta absence options).")
	cmd.Flags().Int64Var(&userID, "user", 0, "User id.")
	cmd.Flags().Float64Var(&dayAmount, "dayamount", 0, "Optional day amount (days mode, for example 1 or 0.5).")
	cmd.Flags().Float64Var(&absenceHours, "hours", 0, "Optional absence duration in hours (hours mode).")
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
