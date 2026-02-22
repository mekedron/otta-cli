package cli

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

type worktimeOptionsSet struct {
	Projects  []worktimeOption `json:"projects"`
	Worktypes []worktimeOption `json:"worktypes"`
	Tasks     []worktimeOption `json:"tasks"`
	Subtasks  []worktimeOption `json:"subtasks"`
	Superiors []worktimeOption `json:"superiors"`
}

type worktimeOption struct {
	ID         int64  `json:"id"`
	Name       string `json:"name,omitempty"`
	ProjectID  int64  `json:"project,omitempty"`
	WorktypeID int64  `json:"worktype,omitempty"`
	TaskID     int64  `json:"task,omitempty"`
}

func newWorktimesOptionsCommand() *cobra.Command {
	var (
		date           string
		user           string
		projectFilter  int64
		worktypeFilter int64
		taskFilter     int64
		outputFormat   string
	)

	cmd := &cobra.Command{
		Use:   "options",
		Short: "List selectable IDs for worktimes add.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}

			if _, err := parseISODate(date); err != nil {
				return fmt.Errorf("--date must be YYYY-MM-DD")
			}
			if cmd.Flags().Changed("project") && projectFilter <= 0 {
				return fmt.Errorf("--project must be > 0")
			}
			if cmd.Flags().Changed("worktype") && worktypeFilter <= 0 {
				return fmt.Errorf("--worktype must be > 0")
			}
			if cmd.Flags().Changed("task") && taskFilter <= 0 {
				return fmt.Errorf("--task must be > 0")
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

			userID, err := resolveWorktimeOptionsUserID(user, cache)
			if err != nil {
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

			projectsRaw, err := request("/worktime/projects", newWorktimeOptionsQuery())
			if err != nil {
				return err
			}
			worktypesQuery := newWorktimeOptionsQuery()
			if userID > 0 {
				worktypesQuery["user"] = fmt.Sprintf("%d", userID)
			}
			worktypesRaw, err := request("/worktime/worktypes", worktypesQuery)
			if err != nil {
				return err
			}
			tasksQuery := newWorktimeOptionsQuery()
			if projectFilter > 0 {
				tasksQuery["project"] = fmt.Sprintf("%d", projectFilter)
			}
			tasksRaw, err := request("/worktime/tasks", tasksQuery)
			if err != nil {
				return err
			}
			subtasksQuery := newWorktimeOptionsQuery()
			if taskFilter > 0 {
				subtasksQuery["task"] = fmt.Sprintf("%d", taskFilter)
			}
			subtasksRaw, err := request("/worktime/subtasks", subtasksQuery)
			if err != nil {
				return err
			}

			raw := map[string]any{
				"projects":  projectsRaw,
				"worktypes": worktypesRaw,
				"tasks":     tasksRaw,
				"subtasks":  subtasksRaw,
			}
			normalized := map[string]any{
				"projects":  projectsRaw["projects"],
				"worktypes": worktypesRaw["worktypes"],
				"tasks":     tasksRaw["tasks"],
				"subtasks":  subtasksRaw["subtasks"],
			}

			options := resolveWorktimeOptions(normalized)
			options = filterWorktimeOptions(options, projectFilter, worktypeFilter, taskFilter)

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "worktimes options",
					Data: map[string]any{
						"date":    date,
						"filters": map[string]any{"project": projectFilter, "worktype": worktypeFilter, "task": taskFilter},
						"options": options,
						"raw":     raw,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "date: %s\n", date)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "projects: %d\n", len(options.Projects))
			for _, item := range options.Projects {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d  %s\n", item.ID, displayOptionName(item.Name))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktypes: %d\n", len(options.Worktypes))
			for _, item := range options.Worktypes {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d  %s\n", item.ID, displayOptionName(item.Name))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "tasks: %d\n", len(options.Tasks))
			for _, item := range options.Tasks {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d  %s\n", item.ID, displayOptionName(item.Name))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "subtasks: %d\n", len(options.Subtasks))
			for _, item := range options.Subtasks {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d  %s\n", item.ID, displayOptionName(item.Name))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "superiors: %d\n", len(options.Superiors))
			for _, item := range options.Superiors {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d  %s\n", item.ID, displayOptionName(item.Name))
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", formatISODate(time.Now().UTC()), "Date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&user, "user", "self", "User filter, use `self` for logged-in user.")
	cmd.Flags().Int64Var(&projectFilter, "project", 0, "Filter task/subtask options by project id.")
	cmd.Flags().Int64Var(&worktypeFilter, "worktype", 0, "Filter task/subtask options by worktype id.")
	cmd.Flags().Int64Var(&taskFilter, "task", 0, "Filter subtask options by task id.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func resolveWorktimeOptionsUserID(user string, cache *config.Cache) (int64, error) {
	userValue := strings.TrimSpace(user)
	if userValue != "" && !strings.EqualFold(userValue, "self") {
		userID := toInt64(userValue)
		if userID <= 0 {
			return 0, fmt.Errorf("--user must be numeric id or `self`")
		}
		return userID, nil
	}

	if userID, ok := config.EnvInt64(config.EnvUserID); ok && userID > 0 {
		return userID, nil
	}
	if cache != nil && cache.User.ID > 0 {
		return cache.User.ID, nil
	}

	return 0, nil
}

func newWorktimeOptionsQuery() map[string]string {
	return map[string]string{
		"limit":  "100",
		"offset": "0",
	}
}

func resolveWorktimeOptions(raw any) worktimeOptionsSet {
	options := worktimeOptionsSet{
		Projects:  collectNamedOptions(raw, "projects", "project"),
		Worktypes: collectNamedOptions(raw, "worktypes", "worktype"),
		Tasks:     collectNamedOptions(raw, "tasks", "task"),
		Subtasks:  collectNamedOptions(raw, "subtasks", "subtask"),
		Superiors: collectNamedOptions(raw, "superiors", "superior"),
	}

	// Fallback to ids already present in listed rows if sideload payload is sparse.
	fallback := optionsFromWorktimeRows(raw)
	if len(options.Projects) == 0 {
		options.Projects = fallback.Projects
	}
	if len(options.Worktypes) == 0 {
		options.Worktypes = fallback.Worktypes
	}
	if len(options.Tasks) == 0 {
		options.Tasks = fallback.Tasks
	}
	if len(options.Subtasks) == 0 {
		options.Subtasks = fallback.Subtasks
	}
	if len(options.Superiors) == 0 {
		options.Superiors = fallback.Superiors
	}

	return options
}

func collectNamedOptions(raw any, keys ...string) []worktimeOption {
	targets := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		targets[normalizeOptionKey(key)] = struct{}{}
	}

	merged := map[int64]worktimeOption{}
	var visit func(value any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, nested := range typed {
				if _, ok := targets[normalizeOptionKey(key)]; ok {
					for _, option := range toWorktimeOptions(nested) {
						merged[option.ID] = mergeWorktimeOption(merged[option.ID], option)
					}
				}
				visit(nested)
			}
		case []any:
			for _, nested := range typed {
				visit(nested)
			}
		}
	}

	visit(raw)
	return sortWorktimeOptions(merged)
}

func toWorktimeOptions(value any) []worktimeOption {
	options := make([]worktimeOption, 0)
	appendOption := func(item any) {
		option, ok := toWorktimeOption(item)
		if !ok {
			return
		}
		options = append(options, option)
	}

	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			appendOption(item)
		}
	default:
		appendOption(typed)
	}

	return options
}

func toWorktimeOption(value any) (worktimeOption, bool) {
	switch typed := value.(type) {
	case map[string]any:
		id := toInt64(typed["id"])
		if id <= 0 {
			id = toInt64(typed["value"])
		}
		if id <= 0 {
			return worktimeOption{}, false
		}
		return worktimeOption{
			ID:         id,
			Name:       optionName(typed),
			ProjectID:  firstPositiveInt64(toInt64(typed["project"]), toInt64(typed["project_id"]), toInt64(typed["projectid"])),
			WorktypeID: firstPositiveInt64(toInt64(typed["worktype"]), toInt64(typed["worktype_id"]), toInt64(typed["worktypeid"])),
			TaskID:     firstPositiveInt64(toInt64(typed["task"]), toInt64(typed["task_id"]), toInt64(typed["parent"]), toInt64(typed["parent_id"]), toInt64(typed["superior"])),
		}, true
	default:
		id := toInt64(typed)
		if id <= 0 {
			return worktimeOption{}, false
		}
		return worktimeOption{ID: id}, true
	}
}

func optionName(values map[string]any) string {
	return firstNonEmpty(
		getString(values, "name"),
		getString(values, "title"),
		getString(values, "label"),
		getString(values, "code"),
		getString(values, "description"),
		getString(values, "text"),
		getString(values, "shortname"),
		getString(values, "short_name"),
	)
}

func optionsFromWorktimeRows(raw any) worktimeOptionsSet {
	root, ok := raw.(map[string]any)
	if !ok {
		return worktimeOptionsSet{}
	}

	items, ok := root["worktimes"].([]any)
	if !ok {
		return worktimeOptionsSet{}
	}

	projects := map[int64]worktimeOption{}
	worktypes := map[int64]worktimeOption{}
	tasks := map[int64]worktimeOption{}
	subtasks := map[int64]worktimeOption{}
	superiors := map[int64]worktimeOption{}

	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if id := toInt64(item["project"]); id > 0 {
			projects[id] = worktimeOption{ID: id}
		}
		if id := toInt64(item["worktype"]); id > 0 {
			worktypes[id] = worktimeOption{ID: id}
		}
		if option, ok := toWorktimeOption(item["task"]); ok {
			tasks[option.ID] = option
		}
		if option, ok := toWorktimeOption(item["subtask"]); ok {
			subtasks[option.ID] = option
		}
		if option, ok := toWorktimeOption(item["superior"]); ok {
			superiors[option.ID] = option
		}
	}

	return worktimeOptionsSet{
		Projects:  sortWorktimeOptions(projects),
		Worktypes: sortWorktimeOptions(worktypes),
		Tasks:     sortWorktimeOptions(tasks),
		Subtasks:  sortWorktimeOptions(subtasks),
		Superiors: sortWorktimeOptions(superiors),
	}
}

func filterWorktimeOptions(options worktimeOptionsSet, projectID int64, worktypeID int64, taskID int64) worktimeOptionsSet {
	if projectID > 0 {
		options.Projects = filterOptionList(options.Projects, func(item worktimeOption) bool {
			return item.ID == projectID
		})
	}
	if worktypeID > 0 {
		options.Worktypes = filterOptionList(options.Worktypes, func(item worktimeOption) bool {
			return item.ID == worktypeID
		})
	}

	options.Tasks = filterOptionList(options.Tasks, func(item worktimeOption) bool {
		if projectID > 0 && item.ProjectID > 0 && item.ProjectID != projectID {
			return false
		}
		if worktypeID > 0 && item.WorktypeID > 0 && item.WorktypeID != worktypeID {
			return false
		}
		if taskID > 0 {
			return item.ID == taskID
		}
		return true
	})

	options.Subtasks = filterOptionList(options.Subtasks, func(item worktimeOption) bool {
		if projectID > 0 && item.ProjectID > 0 && item.ProjectID != projectID {
			return false
		}
		if worktypeID > 0 && item.WorktypeID > 0 && item.WorktypeID != worktypeID {
			return false
		}
		if taskID > 0 && item.TaskID > 0 && item.TaskID != taskID {
			return false
		}
		return true
	})

	options.Superiors = filterOptionList(options.Superiors, func(item worktimeOption) bool {
		if projectID > 0 && item.ProjectID > 0 && item.ProjectID != projectID {
			return false
		}
		if worktypeID > 0 && item.WorktypeID > 0 && item.WorktypeID != worktypeID {
			return false
		}
		if taskID > 0 && item.TaskID > 0 && item.TaskID != taskID {
			return false
		}
		return true
	})

	return options
}

func filterOptionList(values []worktimeOption, keep func(item worktimeOption) bool) []worktimeOption {
	filtered := make([]worktimeOption, 0, len(values))
	for _, value := range values {
		if keep(value) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func mergeWorktimeOption(existing worktimeOption, incoming worktimeOption) worktimeOption {
	if existing.ID <= 0 {
		return incoming
	}
	if existing.Name == "" {
		existing.Name = incoming.Name
	}
	if existing.ProjectID <= 0 {
		existing.ProjectID = incoming.ProjectID
	}
	if existing.WorktypeID <= 0 {
		existing.WorktypeID = incoming.WorktypeID
	}
	if existing.TaskID <= 0 {
		existing.TaskID = incoming.TaskID
	}
	return existing
}

func sortWorktimeOptions(values map[int64]worktimeOption) []worktimeOption {
	options := make([]worktimeOption, 0, len(values))
	for _, value := range values {
		options = append(options, value)
	}

	sort.Slice(options, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(options[i].Name))
		rightName := strings.ToLower(strings.TrimSpace(options[j].Name))
		if leftName != rightName {
			if leftName == "" {
				return false
			}
			if rightName == "" {
				return true
			}
			return leftName < rightName
		}
		return options[i].ID < options[j].ID
	})

	return options
}

func normalizeOptionKey(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, " ", "")
	return key
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func displayOptionName(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return strings.TrimSpace(value)
}
