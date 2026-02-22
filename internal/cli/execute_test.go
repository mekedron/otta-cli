package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mekedron/otta-cli/internal/config"
)

func TestExecuteShowsVersion(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Execute(context.Background(), []string{"--version"}, "v1.2.3", &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(out.String()) != "v1.2.3" {
		t.Fatalf("expected version output, got %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", errOut.String())
	}
}

func TestExecuteVersionFlagIsRootOnly(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Execute(context.Background(), []string{"status", "--version"}, "v1.2.3", &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "unknown flag: --version") {
		t.Fatalf("expected unknown flag error, got %q", errOut.String())
	}
}

func TestExecuteStatusRequiresAccessToken(t *testing.T) {
	t.Setenv(config.EnvConfigPath, filepath.Join(t.TempDir(), "missing-config.json"))

	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Execute(context.Background(), []string{"status"}, "dev", &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "no access token configured") {
		t.Fatalf("expected token-missing error, got %q", errOut.String())
	}
}

func TestExecuteConfigPathUsesEnvOverride(t *testing.T) {
	expected := filepath.Join(t.TempDir(), "custom.json")
	t.Setenv(config.EnvConfigPath, expected)

	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Execute(context.Background(), []string{"config", "path"}, "dev", &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(out.String()) != expected {
		t.Fatalf("expected %q, got %q", expected, out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", errOut.String())
	}
}

func TestExecuteConfigCachePathUsesEnvOverride(t *testing.T) {
	expected := filepath.Join(t.TempDir(), "cache.json")
	t.Setenv(config.EnvCachePath, expected)

	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Execute(context.Background(), []string{"config", "cache-path"}, "dev", &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(out.String()) != expected {
		t.Fatalf("expected %q, got %q", expected, out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", errOut.String())
	}
}

func TestExecuteRootHelpUsesCustomReferenceLayout(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Execute(context.Background(), []string{"--help"}, "dev", &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	helpText := out.String()
	for _, expected := range []string{
		"otta: CLI for interacting with otta.fi time tracking services.",
		"usage: otta <command> [options]",
		"global options (all optional unless marked required):",
		"commands:",
		"full reference:",
		"- otta auth login",
	} {
		if !strings.Contains(helpText, expected) {
			t.Fatalf("expected help to contain %q, got:\n%s", expected, helpText)
		}
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", errOut.String())
	}
}
