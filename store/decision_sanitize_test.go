package store

import "testing"

func TestSanitizeDecisionCoTTrace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "fence marker only",
			input:    "```json",
			expected: "",
		},
		{
			name:     "fence line only with newline",
			input:    "```json\n",
			expected: "",
		},
		{
			name:     "fenced json only",
			input:    "```json\n{\"ts_utc\":\"2026-02-23T00:00:00Z\",\"decisions\":[]}\n```",
			expected: "",
		},
		{
			name:     "raw json only",
			input:    "{\"ts_utc\":\"2026-02-23T00:00:00Z\",\"decisions\":[]}",
			expected: "",
		},
		{
			name:     "human-readable reasoning",
			input:    "trend aligns, stale check passed",
			expected: "trend aligns, stale check passed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeDecisionCoTTrace(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestIsFenceLinesOnly(t *testing.T) {
	if !isFenceLinesOnly("```json\n```") {
		t.Fatal("expected fence-only lines to be treated as fence-only")
	}
	if isFenceLinesOnly("```json\nsome text") {
		t.Fatal("expected mixed fence and text to not be treated as fence-only")
	}
}
