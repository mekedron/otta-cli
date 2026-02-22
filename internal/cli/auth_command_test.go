package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mekedron/otta-cli/internal/config"
	"github.com/spf13/cobra"
)

func TestResolvePasswordUsesEnvWhenPromptInputMissing(t *testing.T) {
	t.Setenv(config.EnvPassword, "env-secret")
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))

	got, err := resolvePassword(cmd, "", false)
	if err != nil {
		t.Fatalf("resolvePassword returned error: %v", err)
	}
	if got != "env-secret" {
		t.Fatalf("expected env-secret, got %q", got)
	}
}

func TestResolvePasswordFallsBackToLineReaderForNonTTY(t *testing.T) {
	var errOut bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("typed-secret\n"))
	cmd.SetErr(&errOut)

	got, err := resolvePassword(cmd, "", false)
	if err != nil {
		t.Fatalf("resolvePassword returned error: %v", err)
	}
	if got != "typed-secret" {
		t.Fatalf("expected typed-secret, got %q", got)
	}
	if !strings.Contains(errOut.String(), "Password: ") {
		t.Fatalf("expected prompt in stderr, got %q", errOut.String())
	}
}

func TestResolvePasswordUsesHiddenReaderForTTY(t *testing.T) {
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe failed: %v", err)
	}
	t.Cleanup(func() {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
	})

	originalIsTerminalFD := isTerminalFD
	originalReadPasswordFD := readPasswordFD
	t.Cleanup(func() {
		isTerminalFD = originalIsTerminalFD
		readPasswordFD = originalReadPasswordFD
	})

	var terminalCheckCount int
	var readCount int
	isTerminalFD = func(_ int) bool {
		terminalCheckCount++
		return true
	}
	readPasswordFD = func(_ int) ([]byte, error) {
		readCount++
		return []byte("hidden-secret"), nil
	}

	var errOut bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetIn(pipeReader)
	cmd.SetErr(&errOut)

	got, err := resolvePassword(cmd, "", false)
	if err != nil {
		t.Fatalf("resolvePassword returned error: %v", err)
	}
	if got != "hidden-secret" {
		t.Fatalf("expected hidden-secret, got %q", got)
	}
	if terminalCheckCount != 1 {
		t.Fatalf("expected terminal check once, got %d", terminalCheckCount)
	}
	if readCount != 1 {
		t.Fatalf("expected hidden read once, got %d", readCount)
	}
}

func TestResolvePasswordReturnsReadPasswordError(t *testing.T) {
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe failed: %v", err)
	}
	t.Cleanup(func() {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
	})

	originalIsTerminalFD := isTerminalFD
	originalReadPasswordFD := readPasswordFD
	t.Cleanup(func() {
		isTerminalFD = originalIsTerminalFD
		readPasswordFD = originalReadPasswordFD
	})

	wantErr := errors.New("read password failed")
	isTerminalFD = func(_ int) bool { return true }
	readPasswordFD = func(_ int) ([]byte, error) { return nil, wantErr }

	cmd := &cobra.Command{}
	cmd.SetIn(pipeReader)

	_, gotErr := resolvePassword(cmd, "", false)
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, gotErr)
	}
}
