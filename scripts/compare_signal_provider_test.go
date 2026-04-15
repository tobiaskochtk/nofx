package main

import "testing"

func TestCompareRankedSymbolSetsCalculatesOverlapAndMissing(t *testing.T) {
	report := compareRankedSymbolSets(
		[]string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT"},
		[]string{"ETHUSDT", "BTCUSDT", "DOGEUSDT", "SOLUSDT"},
	)

	if report.SharedCount != 3 {
		t.Fatalf("shared_count = %d, want 3", report.SharedCount)
	}
	if report.ReferenceCount != 4 {
		t.Fatalf("reference_count = %d, want 4", report.ReferenceCount)
	}
	if report.CandidateCount != 4 {
		t.Fatalf("candidate_count = %d, want 4", report.CandidateCount)
	}
	if report.OverlapRate != 0.75 {
		t.Fatalf("overlap_rate = %v, want 0.75", report.OverlapRate)
	}
	if report.MissingRate != 0.25 {
		t.Fatalf("missing_rate = %v, want 0.25", report.MissingRate)
	}
	if len(report.ReferenceOnly) != 1 || report.ReferenceOnly[0] != "XRPUSDT" {
		t.Fatalf("unexpected reference_only: %#v", report.ReferenceOnly)
	}
	if len(report.CandidateOnly) != 1 || report.CandidateOnly[0] != "DOGEUSDT" {
		t.Fatalf("unexpected candidate_only: %#v", report.CandidateOnly)
	}
}

func TestSpearmanRankCorrelationReturnsExpectedValue(t *testing.T) {
	shared := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	reference := map[string]int{"BTCUSDT": 1, "ETHUSDT": 2, "SOLUSDT": 3}
	candidate := map[string]int{"BTCUSDT": 1, "ETHUSDT": 3, "SOLUSDT": 2}

	value := spearmanRankCorrelation(shared, reference, candidate)
	if value == nil {
		t.Fatal("expected correlation")
	}

	if round(*value, 4) != 0.5 {
		t.Fatalf("correlation = %v, want 0.5", round(*value, 4))
	}
}
