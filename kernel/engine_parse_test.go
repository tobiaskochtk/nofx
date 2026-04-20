package kernel

import (
	"errors"
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

func TestExtractDecisions_FenceOnlyArtifactReturnsEmptyDecisions(t *testing.T) {
	response := "```json"

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions for fence-only artifact, got: %d", len(decisions))
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

func TestExtractDecisions_JSONWrapperObject(t *testing.T) {
	response := `{
  "ts_utc": "2026-02-22T17:38:26Z",
  "cycle": 5,
  "decisions": [
    {"sym":"BTCUSDT","action":"ENTER","side":"long","size_pct":0.25,"leverage":3,"sl_px":96000,"tp_px":101000,"confidence":0.82,"reason_codes":["trend_up"]}
  ]
}`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got: %d", len(decisions))
	}
	if decisions[0].Action != "open_long" {
		t.Fatalf("expected mapped action open_long, got: %s", decisions[0].Action)
	}
	if decisions[0].PositionSizeUSD >= 0 {
		t.Fatalf("expected size_pct marker (negative) before validation, got %.4f", decisions[0].PositionSizeUSD)
	}
	if decisions[0].Confidence != 82 {
		t.Fatalf("expected confidence converted to 82, got %d", decisions[0].Confidence)
	}
}

func TestExtractCoTTrace_FencedJSONOnlyReturnsEmpty(t *testing.T) {
	response := "```json\n{\"ts_utc\":\"2026-02-22T17:38:26Z\",\"cycle\":5,\"decisions\":[]}\n```"

	trace := extractCoTTrace(response)
	if trace != "" {
		t.Fatalf("expected empty CoT trace for fenced JSON-only response, got: %q", trace)
	}
}

func TestExtractCoTTrace_ObjectJSONOnlyReturnsEmpty(t *testing.T) {
	response := "{\"ts_utc\":\"2026-02-22T17:38:26Z\",\"cycle\":5,\"decisions\":[]}"

	trace := extractCoTTrace(response)
	if trace != "" {
		t.Fatalf("expected empty CoT trace for object JSON-only response, got: %q", trace)
	}
}

func TestExtractCoTTrace_ReasoningTagStillExtracts(t *testing.T) {
	response := "<reasoning>trend aligns and stale check passed</reasoning>\n<decision>\n```json\n{\"ts_utc\":\"2026-02-22T17:38:26Z\",\"cycle\":5,\"decisions\":[]}\n```\n</decision>"

	trace := extractCoTTrace(response)
	if trace != "trend aligns and stale check passed" {
		t.Fatalf("expected reasoning text to be extracted, got: %q", trace)
	}
}

func TestIsEmptyAIContentError(t *testing.T) {
	if !isEmptyAIContentError(errors.New("AI API call failed: fail to parse AI server response: API returned empty content")) {
		t.Fatal("expected empty-content error to be detected")
	}
	if isEmptyAIContentError(errors.New("AI API call failed: timeout exceeded")) {
		t.Fatal("expected non-empty-content error to be ignored")
	}
}

func TestBuildNoTradeDecisionEnvelope_ProducesValidEmptyDecisions(t *testing.T) {
	envelope := buildNoTradeDecisionEnvelope(957)

	decisions, err := extractDecisions(envelope)
	if err != nil {
		t.Fatalf("expected no error parsing fallback envelope, got: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions, got: %d", len(decisions))
	}
}
