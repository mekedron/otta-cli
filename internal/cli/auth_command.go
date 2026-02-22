package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/mekedron/otta-cli/internal/otta"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var isTerminalFD = term.IsTerminal

var readPasswordFD = term.ReadPassword

func newAuthCommand() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate and manage API token.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	var (
		username      string
		password      string
		clientID      string
		passwordStdin bool
		outputFormat  string
	)

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login and store token in local config.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}

			if passwordStdin && strings.TrimSpace(password) != "" {
				return fmt.Errorf("use either --password or --password-stdin")
			}

			configPath := config.ResolvePath()
			cfg, err := loadRuntimeConfig(configPath)
			if err != nil {
				return err
			}
			cachePath := config.ResolveCachePath()
			cache, err := loadRuntimeCache(cachePath)
			if err != nil {
				return err
			}

			username = firstNonEmpty(username, cfg.Username)
			if strings.TrimSpace(username) == "" {
				return fmt.Errorf("username is required (use --username)")
			}

			clientID = firstNonEmpty(clientID, cfg.ClientID, config.DefaultClientID)
			resolvedPassword, err := resolvePassword(cmd, password, passwordStdin)
			if err != nil {
				return err
			}

			client := otta.NewClient(cfg.APIBaseURL, nil)
			loginResponse, err := client.Login(cmd.Context(), username, resolvedPassword, clientID)
			if err != nil {
				return err
			}

			cfg.Username = username
			cfg.ClientID = clientID
			applyLoginToken(cfg, loginResponse)

			if err := config.Save(configPath, cfg); err != nil {
				return err
			}

			client.SetAccessToken(loginResponse.AccessToken)
			_ = enrichUserFromAPI(cmd.Context(), client, cache, username)

			var raw any
			healthErr := client.Request(cmd.Context(), http.MethodGet, "/worktimes", map[string]string{
				"date":     formatISODate(time.Now().UTC()),
				"order":    "starttime,endtime",
				"sideload": "true",
				"user":     "self",
			}, nil, &raw)
			if healthErr == nil {
				enrichedUser := extractBestUser(raw)
				if hasUserData(enrichedUser) {
					mergeUser(cache, enrichedUser)
				}
			}
			if err := config.SaveCache(cachePath, cache); err != nil {
				return err
			}

			if selectedFormat == outputFormatJSON {
				payload := commandResult{
					OK:      true,
					Command: "auth login",
					Data: map[string]any{
						"config_path":   configPath,
						"cache_path":    cachePath,
						"username":      cfg.Username,
						"user":          cache.User,
						"token_expires": cfg.Token.ExpiresAt,
						"verified":      healthErr == nil,
						"verification_error": func() string {
							if healthErr == nil {
								return ""
							}
							return healthErr.Error()
						}(),
					},
				}
				return writeJSON(cmd, payload)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "login: success\n")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "config: %s\n", configPath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cache: %s\n", cachePath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "username: %s\n", cfg.Username)
			if name := userDisplayName(cache.User); name != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "user: %s\n", name)
			}
			if cache.User.ID > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "user_id: %d\n", cache.User.ID)
			}
			if cfg.Token.ExpiresAt != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "token_expires_at: %s\n", cfg.Token.ExpiresAt.Format(time.RFC3339))
			}
			if healthErr != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "verification: skipped (%s)\n", healthErr.Error())
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "verification: ok\n")
			}
			return nil
		},
	}
	loginCmd.Flags().StringVarP(&username, "username", "u", "", "Otta username.")
	loginCmd.Flags().StringVar(&password, "password", "", "Otta password (avoid shell history, prefer --password-stdin).")
	loginCmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read password from stdin.")
	loginCmd.Flags().StringVar(&clientID, "client-id", "", "OAuth client_id (default: ember_app).")
	addOutputFormatFlags(loginCmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	authCmd.AddCommand(loginCmd)

	return authCmd
}

func resolvePassword(cmd *cobra.Command, password string, fromStdin bool) (string, error) {
	if fromStdin {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(string(data))
		if value == "" {
			return "", fmt.Errorf("password from stdin is empty")
		}
		return value, nil
	}

	if strings.TrimSpace(password) != "" {
		return strings.TrimSpace(password), nil
	}
	if envPassword := config.EnvString(config.EnvPassword); envPassword != "" {
		return envPassword, nil
	}

	_, _ = fmt.Fprint(cmd.ErrOrStderr(), "Password: ")
	input := cmd.InOrStdin()
	if fd, ok := fileDescriptor(input); ok && isTerminalFD(fd) {
		data, err := readPasswordFD(fd)
		_, _ = fmt.Fprintln(cmd.ErrOrStderr())
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(string(data))
		if value == "" {
			return "", fmt.Errorf("password is required")
		}
		return value, nil
	}

	reader := bufio.NewReader(input)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("password is required")
	}
	// Best effort cleanup for local shells where prompt may echo.
	_, _ = fmt.Fprintln(cmd.ErrOrStderr())
	return value, nil
}

func fileDescriptor(reader io.Reader) (int, bool) {
	file, ok := reader.(*os.File)
	if !ok || file == nil {
		return 0, false
	}
	return int(file.Fd()), true
}
