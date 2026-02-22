package kernel

import (
	"strings"
	"testing"
)

func TestExtractDecisions_EmptyJSONArrayInDecisionBlock(t *testing.T) {
	response := "<reasoning>analysis</reasoning>\n<decision>\n```json\n[]\n```\n</decision>"

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions for explicit empty array, got: %d", len(decisions))
	}
}

func TestExtractDecisions_EmptyJSONArrayWithoutClosingDecisionTag(t *testing.T) {
	response := "<reasoning>analysis</reasoning>\n<decision>\n```json\n[]\n```"

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions for explicit empty array, got: %d", len(decisions))
	}
}

func TestExtractDecisions_FallbackStillWorksForNoJSON(t *testing.T) {
	response := `<reasoning>only analysis, no decision JSON</reasoning>`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 fallback decision, got: %d", len(decisions))
	}
	if decisions[0].Action != "wait" {
		t.Fatalf("expected fallback action wait, got: %s", decisions[0].Action)
	}
	if !strings.Contains(decisions[0].Reasoning, "Model didn't output structured JSON decision") {
		t.Fatalf("unexpected fallback reasoning: %s", decisions[0].Reasoning)
	}
}

func TestValidateJSONFormat_EmptyArrayAllowed(t *testing.T) {
	cases := []string{"[]", "[ ]", "[\n]"}
	for _, tc := range cases {
		if err := validateJSONFormat(tc); err != nil {
			t.Fatalf("expected empty array to be valid (%q), got: %v", tc, err)
		}
	}
}
