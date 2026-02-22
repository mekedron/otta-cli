package cli

import (
	"fmt"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

func newWorktimesReadCommand() *cobra.Command {
	var (
		id           int64
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read a worktime entry by id.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}
			durationFormat, err := resolveDurationFormat(cmd)
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
			item, err := fetchExistingWorktime(cmd, client, id)
			if err != nil {
				return err
			}

			totalMinutes, _ := worktimeMinutes(item)
			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "worktimes read",
					Data: map[string]any{
						"id":              id,
						"item":            item,
						"total_minutes":   totalMinutes,
						"duration_format": durationFormat,
						"total_duration":  durationSummary(totalMinutes, durationFormat),
						"raw":             item,
					},
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "id: %d\n", id)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "date: %s\n", getString(item, "date"))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "start: %s\n", firstNonEmpty(getString(item, "starttime"), getString(item, "start_time")))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "end: %s\n", firstNonEmpty(getString(item, "endtime"), getString(item, "end_time")))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "pause: %v\n", item["pause"])
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "project: %d\n", toInt64(item["project"]))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "user: %d\n", toInt64(item["user"]))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktype: %d\n", toInt64(item["worktype"]))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "description: %s\n", getString(item, "description"))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "total_duration: %s\n", formatDurationForText(totalMinutes, durationFormat))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "note: this command returns worktimes only (no absences)")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use --format json for response payload")
			return nil
		},
	}

	cmd.Flags().Int64Var(&id, "id", 0, "Worktime id.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}
