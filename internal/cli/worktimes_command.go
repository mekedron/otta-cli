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

func newWorktimesCommand() *cobra.Command {
	worktimesCmd := &cobra.Command{
		Use:   "worktimes",
		Short: "Collect and manage worktime entries.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	worktimesCmd.AddCommand(newWorktimesListCommand())
	worktimesCmd.AddCommand(newWorktimesBrowseCommand())
	worktimesCmd.AddCommand(newWorktimesReportCommand())
	worktimesCmd.AddCommand(newWorktimesOptionsCommand())
	worktimesCmd.AddCommand(newWorktimesAddCommand())
	worktimesCmd.AddCommand(newWorktimesUpdateCommand())
	worktimesCmd.AddCommand(newWorktimesDeleteCommand())

	return worktimesCmd
}

func newWorktimesListCommand() *cobra.Command {
	var (
		date         string
		user         string
		order        string
		sideload     bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List worktime entries for a date.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}

			if _, err := parseISODate(date); err != nil {
				return fmt.Errorf("--date must be YYYY-MM-DD")
			}

			configPath := config.ResolvePath()
			cfg, err := loadRuntimeConfig(configPath)
			if err != nil {
				return err
			}
			if err := requireAccessToken(cfg); err != nil {
				return err
			}

			query := map[string]string{
				"date":     date,
				"order":    strings.TrimSpace(order),
				"sideload": fmt.Sprintf("%t", sideload),
			}
			if strings.TrimSpace(user) != "" {
				query["user"] = strings.TrimSpace(user)
			}

			client := newAPIClient(cfg, configPath)
			var raw any
			if err := client.Request(cmd.Context(), http.MethodGet, "/worktimes", query, nil, &raw); err != nil {
				return err
			}

			count := countList(raw, "worktimes", "items", "results")
			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "worktimes list",
					Data: map[string]any{
						"date":  date,
						"count": count,
						"raw":   raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "date: %s\n", date)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "entries: %d\n", count)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "use --format json for full payload\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", formatISODate(time.Now().UTC()), "Date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&user, "user", "self", "User filter, use `self` for logged-in user.")
	cmd.Flags().StringVar(&order, "order", "starttime,endtime", "Sort order.")
	cmd.Flags().BoolVar(&sideload, "sideload", true, "Request sideload data.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func newWorktimesAddCommand() *cobra.Command {
	var (
		date         string
		start        string
		end          string
		pause        string
		projectID    int64
		userID       int64
		worktypeID   int64
		taskID       int64
		subtaskID    int64
		superiorID   int64
		description  string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a worktime entry.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}

			if _, err := parseISODate(date); err != nil {
				return fmt.Errorf("--date must be YYYY-MM-DD")
			}
			if err := validateHHMM(start, "--start"); err != nil {
				return err
			}
			if err := validateHHMM(end, "--end"); err != nil {
				return err
			}
			if strings.TrimSpace(pause) == "" {
				return fmt.Errorf("--pause is required")
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
			if userID <= 0 {
				if value, ok := config.EnvInt64(config.EnvUserID); ok {
					userID = value
				}
			}
			if userID <= 0 {
				userID = cache.User.ID
			}
			if userID <= 0 {
				return fmt.Errorf("--user is required (or set OTTA_CLI_USER_ID / run `otta status` to refresh cache)")
			}
			if projectID <= 0 {
				return fmt.Errorf("--project is required")
			}
			if worktypeID <= 0 {
				return fmt.Errorf("--worktype is required")
			}
			if cmd.Flags().Changed("task") && taskID <= 0 {
				return fmt.Errorf("--task must be > 0")
			}
			if cmd.Flags().Changed("subtask") && subtaskID <= 0 {
				return fmt.Errorf("--subtask must be > 0")
			}
			if cmd.Flags().Changed("superior") && superiorID <= 0 {
				return fmt.Errorf("--superior must be > 0")
			}

			var taskValue any
			if taskID > 0 {
				taskValue = taskID
			}
			var subtaskValue any
			if subtaskID > 0 {
				subtaskValue = subtaskID
			}
			var superiorValue any
			if superiorID > 0 {
				superiorValue = superiorID
			}

			body := map[string]any{
				"worktime": map[string]any{
					"date":        date,
					"starttime":   start,
					"endtime":     end,
					"pause":       pause,
					"project":     projectID,
					"user":        userID,
					"worktype":    worktypeID,
					"description": description,
					"row_info":    nil,
					"subtask":     subtaskValue,
					"superior":    superiorValue,
					"task":        taskValue,
				},
			}

			client := newAPIClient(cfg, configPath)
			var raw any
			if err := client.Request(cmd.Context(), http.MethodPost, "/worktimes", nil, body, &raw); err != nil {
				return err
			}

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "worktimes add",
					Data: map[string]any{
						"raw": raw,
					},
				})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "worktime created")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", formatISODate(time.Now().UTC()), "Date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&start, "start", "09:00", "Start time in HH:MM.")
	cmd.Flags().StringVar(&end, "end", "17:00", "End time in HH:MM.")
	cmd.Flags().StringVar(&pause, "pause", "30", "Break minutes.")
	cmd.Flags().Int64Var(&projectID, "project", 0, "Project id.")
	cmd.Flags().Int64Var(&userID, "user", 0, "User id.")
	cmd.Flags().Int64Var(&worktypeID, "worktype", 0, "Worktype id.")
	cmd.Flags().Int64Var(&taskID, "task", 0, "Task id.")
	cmd.Flags().Int64Var(&subtaskID, "subtask", 0, "Sub-task id.")
	cmd.Flags().Int64Var(&superiorID, "superior", 0, "Superior id.")
	cmd.Flags().StringVar(&description, "description", "- usual IT tasks as ERP development and Magento support", "Entry description.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func newWorktimesUpdateCommand() *cobra.Command {
	var (
		id           int64
		date         string
		start        string
		end          string
		pause        string
		projectID    int64
		userID       int64
		worktypeID   int64
		description  string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a worktime entry.",
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

			changedAny := cmd.Flags().Changed("date") ||
				cmd.Flags().Changed("start") ||
				cmd.Flags().Changed("end") ||
				cmd.Flags().Changed("pause") ||
				cmd.Flags().Changed("project") ||
				cmd.Flags().Changed("user") ||
				cmd.Flags().Changed("worktype") ||
				cmd.Flags().Changed("description")
			if !changedAny {
				return fmt.Errorf("no fields to update")
			}

			client := newAPIClient(cfg, configPath)
			existing, err := fetchExistingWorktime(cmd, client, id)
			if err != nil {
				return err
			}

			// Otta expects a full payload for updates; start from current row and apply overrides.
			worktime := map[string]any{
				"date":        getString(existing, "date"),
				"starttime":   getString(existing, "starttime"),
				"endtime":     getString(existing, "endtime"),
				"pause":       existing["pause"],
				"project":     toInt64(existing["project"]),
				"user":        toInt64(existing["user"]),
				"worktype":    toInt64(existing["worktype"]),
				"description": getString(existing, "description"),
				"status":      getString(existing, "status"),
				"subtask":     existing["subtask"],
				"superior":    existing["superior"],
				"task":        existing["task"],
			}

			if cmd.Flags().Changed("date") {
				if _, err := parseISODate(date); err != nil {
					return fmt.Errorf("--date must be YYYY-MM-DD")
				}
				worktime["date"] = date
			}
			if cmd.Flags().Changed("start") {
				if err := validateHHMM(start, "--start"); err != nil {
					return err
				}
				worktime["starttime"] = start
			}
			if cmd.Flags().Changed("end") {
				if err := validateHHMM(end, "--end"); err != nil {
					return err
				}
				worktime["endtime"] = end
			}
			if cmd.Flags().Changed("pause") {
				if strings.TrimSpace(pause) == "" {
					return fmt.Errorf("--pause must not be empty")
				}
				worktime["pause"] = pause
			}
			if cmd.Flags().Changed("project") {
				if projectID <= 0 {
					return fmt.Errorf("--project must be > 0")
				}
				worktime["project"] = projectID
			}
			if cmd.Flags().Changed("user") {
				if userID <= 0 {
					return fmt.Errorf("--user must be > 0")
				}
				worktime["user"] = userID
			}
			if cmd.Flags().Changed("worktype") {
				if worktypeID <= 0 {
					return fmt.Errorf("--worktype must be > 0")
				}
				worktime["worktype"] = worktypeID
			}
			if cmd.Flags().Changed("description") {
				worktime["description"] = description
			}

			body := map[string]any{"worktime": worktime}
			var raw any
			if err := client.Request(cmd.Context(), http.MethodPut, fmt.Sprintf("/worktimes/%d", id), nil, body, &raw); err != nil {
				return err
			}

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "worktimes update",
					Data: map[string]any{
						"id":  id,
						"raw": raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktime updated: %d\n", id)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	cmd.Flags().Int64Var(&id, "id", 0, "Worktime id.")
	cmd.Flags().StringVar(&date, "date", "", "Date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&start, "start", "", "Start time in HH:MM.")
	cmd.Flags().StringVar(&end, "end", "", "End time in HH:MM.")
	cmd.Flags().StringVar(&pause, "pause", "", "Break minutes.")
	cmd.Flags().Int64Var(&projectID, "project", 0, "Project id.")
	cmd.Flags().Int64Var(&userID, "user", 0, "User id.")
	cmd.Flags().Int64Var(&worktypeID, "worktype", 0, "Worktype id.")
	cmd.Flags().StringVar(&description, "description", "", "Entry description.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func fetchExistingWorktime(cmd *cobra.Command, client *otta.Client, id int64) (map[string]any, error) {
	var raw any
	err := client.Request(cmd.Context(), http.MethodGet, "/worktimes", map[string]string{
		"id":       fmt.Sprintf("%d", id),
		"sideload": "true",
	}, nil, &raw)
	if err != nil {
		return nil, err
	}

	root, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected API response while fetching worktime %d", id)
	}

	items, ok := root["worktimes"].([]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("worktime %d not found", id)
	}

	first, ok := items[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected worktime payload for %d", id)
	}

	return first, nil
}

func newWorktimesDeleteCommand() *cobra.Command {
	var (
		id           int64
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a worktime entry.",
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
			if err := client.Request(cmd.Context(), http.MethodDelete, fmt.Sprintf("/worktimes/%d", id), nil, nil, &raw); err != nil {
				return err
			}

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "worktimes delete",
					Data: map[string]any{
						"id":  id,
						"raw": raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktime deleted: %d\n", id)
			return nil
		},
	}

	cmd.Flags().Int64Var(&id, "id", 0, "Worktime id.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}
