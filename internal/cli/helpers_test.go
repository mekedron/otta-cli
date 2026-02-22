package cli

import "testing"

func TestFirstNonEmptySkipsNilPlaceholder(t *testing.T) {
	got := firstNonEmpty("  <nil>  ", "", "nikita")
	if got != "nikita" {
		t.Fatalf("expected nikita, got %q", got)
	}
}

func TestGetStringHandlesNilAndNilString(t *testing.T) {
	values := map[string]any{
		"a": nil,
		"b": "<nil>",
		"c": " value ",
	}

	if got := getString(values, "a"); got != "" {
		t.Fatalf("expected empty for nil, got %q", got)
	}
	if got := getString(values, "b"); got != "" {
		t.Fatalf("expected empty for <nil>, got %q", got)
	}
	if got := getString(values, "c"); got != "value" {
		t.Fatalf("expected value, got %q", got)
	}
}
