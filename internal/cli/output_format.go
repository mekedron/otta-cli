package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	outputFormatText = "text"
	outputFormatJSON = "json"
	outputFormatCSV  = "csv"
)

func addOutputFormatFlags(cmd *cobra.Command, format *string, defaultFormat string, allowed ...string) {
	normalizedAllowed := normalizeOutputFormats(allowed)
	cmd.Flags().StringVar(format, "format", defaultFormat, fmt.Sprintf("Output format: %s.", strings.Join(normalizedAllowed, ", ")))
	cmd.Flags().Bool("json", false, "Deprecated alias for --format json.")
	_ = cmd.Flags().MarkHidden("json")
}

func resolveOutputFormat(cmd *cobra.Command, format string, allowed ...string) (string, error) {
	normalizedAllowed := normalizeOutputFormats(allowed)
	if len(normalizedAllowed) == 0 {
		return "", fmt.Errorf("no output formats configured")
	}

	allowedMap := make(map[string]struct{}, len(normalizedAllowed))
	for _, option := range normalizedAllowed {
		allowedMap[option] = struct{}{}
	}

	selected := strings.ToLower(strings.TrimSpace(format))
	jsonFlag, _ := cmd.Flags().GetBool("json")
	jsonAliasEnabled := cmd.Flags().Changed("json") && jsonFlag
	if jsonAliasEnabled {
		if cmd.Flags().Changed("format") && selected != outputFormatJSON {
			return "", fmt.Errorf("--json conflicts with --format=%s (use only --format json)", selected)
		}
		selected = outputFormatJSON
	}
	if selected == "" {
		selected = normalizedAllowed[0]
	}

	if _, ok := allowedMap[selected]; !ok {
		return "", fmt.Errorf("--format must be one of: %s", strings.Join(normalizedAllowed, ","))
	}

	return selected, nil
}

func normalizeOutputFormats(formats []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(formats))
	for _, value := range formats {
		format := strings.ToLower(strings.TrimSpace(value))
		if format == "" {
			continue
		}
		if _, exists := seen[format]; exists {
			continue
		}
		seen[format] = struct{}{}
		normalized = append(normalized, format)
	}
	return normalized
}
