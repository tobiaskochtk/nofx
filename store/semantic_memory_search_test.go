package store

import (
	"strings"
	"testing"
	"time"
)

func TestSemanticMemorySearchFilterClauses(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	clauses, args := semanticMemorySearchFilterClauses("d", SemanticMemorySearchFilter{
		Symbol:                "raveusdt",
		Side:                  "long",
		Outcome:               "loss",
		CloseReason:           "stop_loss",
		ExitOrigin:            "trailing_engine",
		ExitReasonQuality:     "high_confidence",
		OpenTrendRegime:       "uptrend",
		OpenVolatilityRegime:  "medium",
		OpenOIRegime:          "rising",
		CloseTrendRegime:      "flat",
		CloseVolatilityRegime: "high",
		CloseOIRegime:         "falling",
		FromTime:              &from,
		ToTime:                &to,
	})
	if len(clauses) != 14 {
		t.Fatalf("len(clauses) = %d, want 14", len(clauses))
	}
	if len(args) != len(clauses) {
		t.Fatalf("len(args) = %d, want %d", len(args), len(clauses))
	}
	if got := args[0]; got != "RAVEUSDT" {
		t.Fatalf("symbol arg = %#v, want RAVEUSDT", got)
	}
	if got := args[1]; got != "LONG" {
		t.Fatalf("side arg = %#v, want LONG", got)
	}
	if !strings.Contains(clauses[0], "metadata_json::jsonb ->> 'symbol'") {
		t.Fatalf("expected symbol filter clause, got %q", clauses[0])
	}
	if !strings.Contains(clauses[len(clauses)-2], "source_updated_at >= ?") {
		t.Fatalf("expected from_time clause near end, got %q", clauses[len(clauses)-2])
	}
	if !strings.Contains(clauses[len(clauses)-1], "source_updated_at <= ?") {
		t.Fatalf("expected to_time clause at end, got %q", clauses[len(clauses)-1])
	}
}

func TestSemanticMemorySourceLink(t *testing.T) {
	tests := []struct {
		docType  string
		sourceID string
		want     string
	}{
		{docType: SemanticMemoryDocTypeDealReviewCase, sourceID: "case-1", want: "/deal-review?case_id=case-1"},
		{docType: SemanticMemoryDocTypeAutonomousOptimizerRun, sourceID: "run-1", want: "/optimizer?run_id=run-1"},
		{docType: SemanticMemoryDocTypeOptimizerBacklogItem, sourceID: "backlog-1", want: "/optimizer?backlog_id=backlog-1"},
		{docType: SemanticMemoryDocTypeStrategyVersion, sourceID: "ver-1", want: "/deal-review?strategy_version_id=ver-1"},
	}
	for _, tc := range tests {
		doc := &SemanticMemoryDocument{DocType: tc.docType, SourceID: tc.sourceID}
		if got := semanticMemorySourceLink(doc); got != tc.want {
			t.Fatalf("semanticMemorySourceLink(%s) = %q, want %q", tc.docType, got, tc.want)
		}
	}
}
