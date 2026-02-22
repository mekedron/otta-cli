package cli

import "github.com/spf13/cobra"

func newCalendarCommand() *cobra.Command {
	calendarCmd := &cobra.Command{
		Use:   "calendar",
		Short: "Calendar reporting utilities.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	calendarCmd.AddCommand(newCalendarOverviewCommand())
	return calendarCmd
}
