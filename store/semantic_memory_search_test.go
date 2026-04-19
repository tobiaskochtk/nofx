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
		{
			docType:  SemanticMemoryDocTypeSymbolBehaviorPrior,
			sourceID: "RAVEUSDT::LONG::momentum::uptrend::high_vol::oi_flat",
			want:     "/deal-review?symbol_prior_id=prior-1&symbol=RAVEUSDT&side=LONG",
		},
		{
			docType:  SemanticMemoryDocTypeLearnedPattern,
			sourceID: "symbol::RAVEUSDT::LONG::negative_edge::bucket:breakout+oi:flat",
			want:     "/pattern-lab?pattern_class=negative_edge&pattern_id=pattern-1&scope_type=symbol&side=LONG&symbol=RAVEUSDT&validation_label=false_positive",
		},
	}
	for _, tc := range tests {
		doc := &SemanticMemoryDocument{DocType: tc.docType, SourceID: tc.sourceID}
		if tc.docType == SemanticMemoryDocTypeSymbolBehaviorPrior {
			doc.MetadataJSON = semanticMemoryMarshalJSON(map[string]any{
				"prior_id": "prior-1",
				"symbol":   "RAVEUSDT",
				"side":     "LONG",
			})
		}
		if tc.docType == SemanticMemoryDocTypeLearnedPattern {
			doc.MetadataJSON = semanticMemoryMarshalJSON(map[string]any{
				"pattern_id":       "pattern-1",
				"scope_type":       "symbol",
				"pattern_class":    "negative_edge",
				"validation_label": "false_positive",
				"symbol":           "RAVEUSDT",
				"side":             "LONG",
			})
		}
		if got := semanticMemorySourceLink(doc); got != tc.want {
			t.Fatalf("semanticMemorySourceLink(%s) = %q, want %q", tc.docType, got, tc.want)
		}
	}
}

func TestSemanticMemorySearchCompositeScorePrefersOptimizerRunSignalOverlap(t *testing.T) {
	query := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeAutonomousOptimizerRun,
		Title:   "Optimizer run blocked_by_gate",
		Summary: "Proposal config_patch, critic_action block_apply, config_validation passed.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"status":                    "blocked_by_gate",
			"trigger":                   "schedule",
			"primary_model_name":        "gpt-5.4",
			"critic_model_name":         "gpt-5.4",
			"proposal_type":             "config_patch",
			"critic_recommended_action": "block_apply",
			"config_validation_status":  "passed",
			"gate_reasons":              []string{"critic_requested_narrower_patch"},
			"config_patch_paths":        []string{"risk_control.adaptive_reentry_guard.enabled"},
			"prompt_patch_keys":         []string{"strategy.prompt_sections.entry_standards"},
		}),
	}
	good := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeAutonomousOptimizerRun,
		Title:   "Optimizer run blocked_by_gate",
		Summary: "Proposal config_patch, critic_action block_apply, config_validation passed.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"status":                    "blocked_by_gate",
			"trigger":                   "schedule",
			"primary_model_name":        "gpt-5.4",
			"critic_model_name":         "gpt-5.4",
			"proposal_type":             "config_patch",
			"critic_recommended_action": "block_apply",
			"config_validation_status":  "passed",
			"gate_reasons":              []string{"critic_requested_narrower_patch"},
			"config_patch_paths":        []string{"risk_control.adaptive_reentry_guard.enabled"},
			"prompt_patch_keys":         []string{"strategy.prompt_sections.entry_standards"},
		}),
	}
	bad := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeAutonomousOptimizerRun,
		Title:   "Optimizer run backlog_only",
		Summary: "Proposal backlog_only, critic_action approve_backlog_only.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"status":                    "backlog_only",
			"trigger":                   "manual",
			"primary_model_name":        "gpt-5.4-mini",
			"critic_model_name":         "gpt-5.4-mini",
			"proposal_type":             "backlog_only",
			"critic_recommended_action": "approve_backlog_only",
			"config_validation_status":  "skipped",
			"gate_reasons":              []string{"insufficient evidence"},
			"config_patch_paths":        []string{"risk.trailing_stop.tiers[].trigger_profit_pct"},
			"prompt_patch_keys":         []string{"optimizer.proposal_instructions"},
		}),
	}

	goodScore := semanticMemorySearchCompositeScore(query, good, 0.70)
	badScore := semanticMemorySearchCompositeScore(query, bad, 0.72)
	if goodScore <= badScore {
		t.Fatalf("goodScore = %f, badScore = %f; want goodScore > badScore", goodScore, badScore)
	}
}

func TestSemanticMemorySearchCompositeScorePrefersSymbolPriorSignalOverlap(t *testing.T) {
	query := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeSymbolBehaviorPrior,
		Title:   "Symbol prior RAVEUSDT long confirmed",
		Summary: "Bias negative, status validated, validation confirmed.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"symbol":                 "RAVEUSDT",
			"side":                   "LONG",
			"status":                 "validated",
			"validation_label":       "confirmed",
			"behavior_bias":          "negative",
			"recommended_action":     "tighten_entry",
			"open_selection_bucket":  "momentum",
			"open_trend_regime":      "uptrend",
			"open_volatility_regime": "high_vol",
			"open_oi_regime":         "oi_flat",
			"signal_tags":            []string{"breakout_chase", "crowded_longs"},
		}),
	}
	good := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeSymbolBehaviorPrior,
		Title:   "Symbol prior RAVEUSDT long confirmed",
		Summary: "Bias negative, status validated, validation confirmed.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"symbol":                 "RAVEUSDT",
			"side":                   "LONG",
			"status":                 "validated",
			"validation_label":       "confirmed",
			"behavior_bias":          "negative",
			"recommended_action":     "tighten_entry",
			"open_selection_bucket":  "momentum",
			"open_trend_regime":      "uptrend",
			"open_volatility_regime": "high_vol",
			"open_oi_regime":         "oi_flat",
			"signal_tags":            []string{"breakout_chase"},
		}),
	}
	bad := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeSymbolBehaviorPrior,
		Title:   "Symbol prior BTCUSDT short false positive",
		Summary: "Bias positive, status rejected, validation false positive.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"symbol":                 "BTCUSDT",
			"side":                   "SHORT",
			"status":                 "rejected",
			"validation_label":       "false_positive",
			"behavior_bias":          "positive",
			"recommended_action":     "monitor_only",
			"open_selection_bucket":  "mean_reversion",
			"open_trend_regime":      "downtrend",
			"open_volatility_regime": "low_vol",
			"open_oi_regime":         "oi_rising",
			"signal_tags":            []string{"fade", "oversold"},
		}),
	}

	goodScore := semanticMemorySearchCompositeScore(query, good, 0.69)
	badScore := semanticMemorySearchCompositeScore(query, bad, 0.72)
	if goodScore <= badScore {
		t.Fatalf("goodScore = %f, badScore = %f; want goodScore > badScore", goodScore, badScore)
	}
}

func TestSemanticMemorySearchCompositeScorePrefersLearnedPatternSignalOverlap(t *testing.T) {
	query := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeLearnedPattern,
		Title:   "Learned pattern RAVEUSDT long anti-edge",
		Summary: "Breakout plus flat OI repeatedly failed.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"pattern_id":        "pattern-rave-1",
			"scope_type":        "symbol",
			"symbol":            "RAVEUSDT",
			"side":              "LONG",
			"pattern_class":     "negative_edge",
			"validation_label":  "false_positive",
			"recommended_use":   "monitor_only",
			"pattern_signature": "bucket:breakout + oi:flat",
			"regime_signature":  "trend:uptrend|vol:high",
			"feature_set":       []string{"bucket:breakout", "oi:flat", "spread:wide"},
			"evidence_case_ids": []string{"case-1", "case-2"},
		}),
	}
	good := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeLearnedPattern,
		Title:   "Learned pattern RAVEUSDT long anti-edge",
		Summary: "Breakout plus flat OI repeatedly failed.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"pattern_id":        "pattern-rave-2",
			"scope_type":        "symbol",
			"symbol":            "RAVEUSDT",
			"side":              "LONG",
			"pattern_class":     "negative_edge",
			"validation_label":  "false_positive",
			"recommended_use":   "monitor_only",
			"pattern_signature": "bucket:breakout + oi:flat",
			"regime_signature":  "trend:uptrend|vol:high",
			"feature_set":       []string{"bucket:breakout", "oi:flat"},
			"evidence_case_ids": []string{"case-2", "case-3"},
		}),
	}
	bad := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeLearnedPattern,
		Title:   "Learned pattern BTCUSDT short positive edge",
		Summary: "Mean reversion shorts performed well in quiet sessions.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"pattern_id":        "pattern-btc-1",
			"scope_type":        "trader_local",
			"symbol":            "BTCUSDT",
			"side":              "SHORT",
			"pattern_class":     "positive_edge",
			"validation_label":  "confirmed",
			"recommended_use":   "config_candidate",
			"pattern_signature": "bucket:mean_revert + session:asia",
			"regime_signature":  "trend:downtrend|vol:low",
			"feature_set":       []string{"bucket:mean_revert", "session:asia"},
			"evidence_case_ids": []string{"case-9"},
		}),
	}

	goodScore := semanticMemorySearchCompositeScore(query, good, 0.68)
	badScore := semanticMemorySearchCompositeScore(query, bad, 0.74)
	if goodScore <= badScore {
		t.Fatalf("goodScore = %f, badScore = %f; want goodScore > badScore", goodScore, badScore)
	}
}
