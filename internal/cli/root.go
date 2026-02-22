package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var errVersionShown = errors.New("version shown")

// NewRootCommand builds the command tree for otta-cli.
func NewRootCommand(version string) *cobra.Command {
	resolved := resolvedVersion(version)

	root := &cobra.Command{
		Use:           "otta",
		Short:         "CLI for interacting with otta.fi time tracking services.",
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			showVersion, _ := cmd.Flags().GetBool("version")
			if showVersion {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), resolved)
				return errVersionShown
			}
			return cmd.Help()
		},
	}

	root.Flags().BoolP("version", "v", false, "Show CLI version and exit.")
	root.SetHelpCommand(&cobra.Command{Hidden: true})
	defaultHelpFunc := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == root {
			renderRootHelp(cmd.OutOrStdout(), root)
			return
		}
		defaultHelpFunc(cmd, args)
	})

	root.AddCommand(newStatusCommand())
	root.AddCommand(newConfigCommand())
	root.AddCommand(newAuthCommand())
	root.AddCommand(newWorktimesCommand())
	root.AddCommand(newCalendarCommand())
	root.AddCommand(newHolidaysCommand())
	root.AddCommand(newAbsenceCommand())

	return root
}

func renderRootHelp(out io.Writer, root *cobra.Command) {
	_, _ = fmt.Fprintf(out, "%s: %s\n\n", root.Name(), root.Short)
	_, _ = fmt.Fprintf(out, "usage: %s <command> [options]\n", root.Name())
	_, _ = fmt.Fprintln(out, "global options (all optional unless marked required):")
	for _, option := range commandOptions(root) {
		_, _ = fmt.Fprintf(out, "  %s%s: %s\n", option.token, optionLabels(option), option.usage)
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "commands:")
	for _, cmd := range visibleCommands(root) {
		_, _ = fmt.Fprintf(out, "  %s\n", cmd.Name())
		_, _ = fmt.Fprintf(out, "    %s\n", cmd.Short)
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "notes:")
	_, _ = fmt.Fprintln(out, "  - options are optional unless marked [required].")
	_, _ = fmt.Fprintln(out, "  - use --format json on data commands for machine-readable output.")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "full reference:")
	emitReference(out, root, root.Name())
}

func visibleCommands(parent *cobra.Command) []*cobra.Command {
	commands := make([]*cobra.Command, 0)
	for _, cmd := range parent.Commands() {
		if cmd.Hidden {
			continue
		}
		commands = append(commands, cmd)
	}
	return commands
}

func emitReference(out io.Writer, parent *cobra.Command, path string) {
	for _, cmd := range visibleCommands(parent) {
		signature := strings.TrimSpace(path + " " + cmd.Use)
		_, _ = fmt.Fprintf(out, "- %s\n", signature)
		_, _ = fmt.Fprintf(out, "  %s\n", cmd.Short)
		options := commandOptions(cmd)
		if len(options) > 0 {
			_, _ = fmt.Fprintln(out, "  options:")
			for _, option := range options {
				_, _ = fmt.Fprintf(out, "    %s%s: %s\n", option.token, optionLabels(option), option.usage)
			}
		}
		_, _ = fmt.Fprintln(out)
		emitReference(out, cmd, strings.TrimSpace(path+" "+cmd.Name()))
	}
}

type optionDoc struct {
	name      string
	token     string
	usage     string
	required  bool
	inherited bool
}

func commandOptions(cmd *cobra.Command) []optionDoc {
	seen := map[string]struct{}{}
	options := make([]optionDoc, 0)
	for _, option := range collectOptionDocs(cmd.NonInheritedFlags(), false) {
		seen[option.name] = struct{}{}
		options = append(options, option)
	}
	for _, option := range collectOptionDocs(cmd.InheritedFlags(), true) {
		if _, ok := seen[option.name]; ok {
			continue
		}
		options = append(options, option)
	}
	return options
}

func collectOptionDocs(flags *pflag.FlagSet, inherited bool) []optionDoc {
	options := make([]optionDoc, 0)
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden || flag.Name == "help" {
			return
		}
		options = append(options, optionDoc{
			name:      flag.Name,
			token:     flagToken(flag),
			usage:     strings.TrimSpace(flag.Usage),
			required:  isFlagRequired(flag),
			inherited: inherited,
		})
	})
	sort.Slice(options, func(i, j int) bool {
		return options[i].name < options[j].name
	})
	return options
}

func flagToken(flag *pflag.Flag) string {
	token := "--" + flag.Name
	if flag.Shorthand != "" {
		token += "/-" + flag.Shorthand
	}
	return token
}

func isFlagRequired(flag *pflag.Flag) bool {
	values, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]
	if !ok || len(values) == 0 {
		return false
	}
	return strings.EqualFold(values[0], "true") || values[0] == "1"
}

func optionLabels(option optionDoc) string {
	labels := make([]string, 0, 2)
	if option.required {
		labels = append(labels, "required")
	}
	if option.inherited {
		labels = append(labels, "global")
	}
	if len(labels) == 0 {
		return ""
	}
	return " [" + strings.Join(labels, ", ") + "]"
}
