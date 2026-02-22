package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

func newStatusCommand() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check credential validity and print basic user info.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}
			durationFormat, err := resolveDurationFormat(cmd)
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
			_ = enrichUserFromAPI(cmd.Context(), client, cache, cfg.Username)

			var raw any
			err = client.Request(cmd.Context(), http.MethodGet, "/worktimes", map[string]string{
				"date":     formatISODate(time.Now().UTC()),
				"order":    "starttime,endtime",
				"sideload": "true",
				"user":     "self",
			}, nil, &raw)
			if err != nil {
				return err
			}

			extracted := extractBestUser(raw)
			if hasUserData(extracted) {
				mergeUser(cache, extracted)
			}
			_ = config.SaveCache(cachePath, cache)

			count := countList(raw, "worktimes", "items", "results")
			if selectedFormat == outputFormatJSON {
				payload := commandResult{
					OK:      true,
					Command: "status",
					Data: map[string]any{
						"config_path":     configPath,
						"cache_path":      cachePath,
						"user":            cache.User,
						"token_expires":   cfg.Token.ExpiresAt,
						"entries_today":   count,
						"duration_format": durationFormat,
						"raw":             raw,
					},
				}
				return writeJSON(cmd, payload)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "config: %s\n", configPath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cache: %s\n", cachePath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "auth: valid\n")
			if cfg.Token.ExpiresAt != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "token_expires_at: %s\n", cfg.Token.ExpiresAt.Format(time.RFC3339))
			}
			if name := userDisplayName(cache.User); name != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "user: %s\n", name)
			}
			if cache.User.ID > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "user_id: %d\n", cache.User.ID)
			}
			if cache.User.WorktimeGroupID > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktimegroup_id: %d\n", cache.User.WorktimeGroupID)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worktimes_today: %d\n", count)
			return nil
		},
	}

	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}
