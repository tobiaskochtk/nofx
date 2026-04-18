package store

import "testing"

func TestSemanticMemoryBenchmarkDealCaseRelevance(t *testing.T) {
	query := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeDealReviewCase,
		Title:   "Deal review case RAVEUSDT long loss",
		Summary: "Outcome loss with close_reason stop_loss and exit_origin synced_trigger_order.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"symbol":                 "RAVEUSDT",
			"side":                   "LONG",
			"outcome":                "loss",
			"close_reason":           "stop_loss",
			"exit_origin":            "synced_trigger_order",
			"open_selection_bucket":  "momentum",
			"open_trend_regime":      "uptrend",
			"open_volatility_regime": "high",
			"open_oi_regime":         "flat",
			"labels":                 []string{"reentry", "late_entry"},
		}),
	}
	similar := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeDealReviewCase,
		Title:   "Deal review case RAVEUSDT long loss",
		Summary: "Outcome loss with close_reason stop_loss and exit_origin synced_trigger_order.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"symbol":                 "RAVEUSDT",
			"side":                   "LONG",
			"outcome":                "loss",
			"close_reason":           "stop_loss",
			"exit_origin":            "synced_trigger_order",
			"open_selection_bucket":  "momentum",
			"open_trend_regime":      "uptrend",
			"open_volatility_regime": "high",
			"open_oi_regime":         "flat",
			"labels":                 []string{"reentry"},
		}),
	}
	different := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeDealReviewCase,
		Title:   "Deal review case BTCUSDT short profit",
		Summary: "Outcome profit with close_reason take_profit and exit_origin ai_decision.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"symbol":                 "BTCUSDT",
			"side":                   "SHORT",
			"outcome":                "profit",
			"close_reason":           "take_profit",
			"exit_origin":            "ai_decision",
			"open_selection_bucket":  "mean_reversion",
			"open_trend_regime":      "downtrend",
			"open_volatility_regime": "low",
			"open_oi_regime":         "rising",
			"labels":                 []string{"clean_exit"},
		}),
	}

	goodScore := semanticMemoryBenchmarkRelevance(query, similar)
	badScore := semanticMemoryBenchmarkRelevance(query, different)
	if goodScore <= badScore {
		t.Fatalf("goodScore = %f, badScore = %f; want goodScore > badScore", goodScore, badScore)
	}
	if goodScore < semanticMemoryBenchmarkRelevantThreshold {
		t.Fatalf("goodScore = %f, want >= %f", goodScore, semanticMemoryBenchmarkRelevantThreshold)
	}
}

func TestSemanticMemoryBenchmarkConfigNormalization(t *testing.T) {
	cfg := normalizeSemanticMemoryBenchmarkConfig(SemanticMemoryBenchmarkConfig{})
	if cfg.SamplePerDocType != semanticMemoryBenchmarkDefaultSamplePerDocType {
		t.Fatalf("SamplePerDocType = %d, want %d", cfg.SamplePerDocType, semanticMemoryBenchmarkDefaultSamplePerDocType)
	}
	if cfg.TopK != semanticMemoryBenchmarkDefaultTopK {
		t.Fatalf("TopK = %d, want %d", cfg.TopK, semanticMemoryBenchmarkDefaultTopK)
	}
	if len(cfg.DocTypes) == 0 {
		t.Fatal("DocTypes should default to the normalized semantic-memory corpus set")
	}
}
