package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAbsenceCommand() *cobra.Command {
	absenceCmd := &cobra.Command{
		Use:   "absence",
		Short: "Absence utilities.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	absenceCmd.AddCommand(newAbsenceOptionsCommand())
	absenceCmd.AddCommand(newAbsenceBrowseCommand())
	absenceCmd.AddCommand(newAbsenceReadCommand())
	absenceCmd.AddCommand(newAbsenceAddCommand())
	absenceCmd.AddCommand(newAbsenceUpdateCommand())
	absenceCmd.AddCommand(newAbsenceDeleteCommand())
	absenceCmd.AddCommand(newAbsenceCommentCommand())

	return absenceCmd
}

func newAbsenceCommentCommand() *cobra.Command {
	var (
		absenceType  string
		dateFrom     string
		dateTo       string
		details      string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Generate a comment text for absence submission.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selectedFormat, err := resolveOutputFormat(cmd, outputFormat, outputFormatText, outputFormatJSON)
			if err != nil {
				return err
			}

			if strings.TrimSpace(absenceType) == "" {
				return fmt.Errorf("--type is required")
			}
			fromDate, err := parseISODate(dateFrom)
			if err != nil {
				return fmt.Errorf("--from must be YYYY-MM-DD")
			}
			toDate, err := parseISODate(dateTo)
			if err != nil {
				return fmt.Errorf("--to must be YYYY-MM-DD")
			}
			if toDate.Before(fromDate) {
				return fmt.Errorf("--to must be greater than or equal to --from")
			}

			normalizedType := strings.ToLower(strings.TrimSpace(absenceType))
			comment := fmt.Sprintf(
				"Absence: %s (%s - %s)%s",
				normalizedType,
				dateFrom,
				dateTo,
				formatDetails(details),
			)

			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd, commandResult{
					OK:      true,
					Command: "absence comment",
					Data: map[string]any{
						"type":    normalizedType,
						"from":    dateFrom,
						"to":      dateTo,
						"details": strings.TrimSpace(details),
						"comment": comment,
					},
				})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), comment)
			return nil
		},
	}

	today := formatISODate(time.Now().UTC())
	cmd.Flags().StringVar(&absenceType, "type", "", "Absence type, e.g. sick, vacation, other.")
	cmd.Flags().StringVar(&dateFrom, "from", today, "Start date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&dateTo, "to", today, "End date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&details, "details", "", "Optional additional details.")
	addOutputFormatFlags(cmd, &outputFormat, outputFormatText, outputFormatText, outputFormatJSON)

	return cmd
}

func formatDetails(details string) string {
	trimmed := strings.TrimSpace(details)
	if trimmed == "" {
		return ""
	}
	return ": " + trimmed
}
