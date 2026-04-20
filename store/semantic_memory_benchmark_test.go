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

func TestSemanticMemoryBenchmarkOptimizerRunRelevance(t *testing.T) {
	query := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeAutonomousOptimizerRun,
		Title:   "Optimizer run blocked_by_gate run-1",
		Summary: "Trigger schedule, proposal config_patch, critic_action block_apply, config_validation passed.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"status":                    "blocked_by_gate",
			"trigger":                   "schedule",
			"primary_model_name":        "gpt-5.4",
			"critic_model_name":         "gpt-5.4",
			"proposal_type":             "config_patch",
			"critic_recommended_action": "block_apply",
			"config_validation_status":  "passed",
			"gate_reasons": []string{
				"critic_requested_narrower_patch",
				"mixed evidence across repeated symbols",
			},
			"deferred_reasons": []string{
				"cooldown active",
			},
			"config_patch_paths": []string{
				"risk_control.adaptive_reentry_guard.enabled",
				"risk_control.adaptive_reentry_guard.same_symbol_loss_cooldown_minutes",
			},
			"config_patch_keys": []string{
				"risk_control.adaptive_reentry_guard.enabled",
				"risk_control.adaptive_reentry_guard.same_symbol_loss_cooldown_minutes",
			},
			"prompt_patch_keys": []string{
				"strategy.prompt_sections.entry_standards",
			},
		}),
	}
	similar := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeAutonomousOptimizerRun,
		Title:   "Optimizer run blocked_by_gate run-2",
		Summary: "Trigger schedule, proposal config_patch, critic_action block_apply, config_validation passed.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"status":                    "blocked_by_gate",
			"trigger":                   "schedule",
			"primary_model_name":        "gpt-5.4",
			"critic_model_name":         "gpt-5.4",
			"proposal_type":             "config_patch",
			"critic_recommended_action": "block_apply",
			"config_validation_status":  "passed",
			"gate_reasons": []string{
				"critic_requested_narrower_patch",
			},
			"deferred_reasons": []string{
				"cooldown active",
			},
			"config_patch_paths": []string{
				"risk_control.adaptive_reentry_guard.enabled",
			},
			"config_patch_keys": []string{
				"risk_control.adaptive_reentry_guard.enabled",
			},
			"prompt_patch_keys": []string{
				"strategy.prompt_sections.entry_standards",
			},
		}),
	}
	different := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeAutonomousOptimizerRun,
		Title:   "Optimizer run backlog_only run-3",
		Summary: "Trigger manual, proposal backlog_only, critic_action approve_backlog_only, config_validation skipped.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"status":                    "backlog_only",
			"trigger":                   "manual",
			"primary_model_name":        "gpt-5.4-mini",
			"critic_model_name":         "gpt-5.4-mini",
			"proposal_type":             "backlog_only",
			"critic_recommended_action": "approve_backlog_only",
			"config_validation_status":  "skipped",
			"gate_reasons": []string{
				"insufficient evidence",
			},
			"deferred_reasons": []string{
				"none",
			},
			"config_patch_paths": []string{
				"risk.trailing_stop.tiers[].trigger_profit_pct",
			},
			"config_patch_keys": []string{
				"risk.trailing_stop.tiers[].trigger_profit_pct",
			},
			"prompt_patch_keys": []string{
				"optimizer.proposal_instructions",
			},
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

func TestSemanticMemoryBenchmarkDecisionRecordSummaryRelevance(t *testing.T) {
	query := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeDecisionRecordSummary,
		Title:   "Decision cycle 77 open_long hold",
		Summary: "RAVEUSDT and ENAUSDT with primary/exploration buckets and same_symbol_cooldown.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"symbols":            []string{"RAVEUSDT", "ENAUSDT"},
			"action_types":       []string{"open_long", "hold"},
			"selection_buckets":  []string{"primary", "exploration"},
			"reject_reasons":     []string{"same_symbol_cooldown", "late_breakout"},
			"candidate_sources":  []string{"ai500"},
			"sessions":           []string{"asia"},
			"trend_regimes":      []string{"uptrend"},
			"volatility_regimes": []string{"high_vol"},
			"oi_regimes":         []string{"oi_flat"},
		}),
	}
	similar := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeDecisionRecordSummary,
		Title:   "Decision cycle 88 open_long hold",
		Summary: "RAVEUSDT and EDGEUSDT with primary/exploration buckets and same_symbol_cooldown.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"symbols":            []string{"RAVEUSDT", "EDGEUSDT"},
			"action_types":       []string{"open_long", "hold"},
			"selection_buckets":  []string{"primary", "exploration"},
			"reject_reasons":     []string{"same_symbol_cooldown"},
			"candidate_sources":  []string{"ai500"},
			"sessions":           []string{"asia"},
			"trend_regimes":      []string{"uptrend"},
			"volatility_regimes": []string{"high_vol"},
			"oi_regimes":         []string{"oi_flat"},
		}),
	}
	different := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeDecisionRecordSummary,
		Title:   "Decision cycle 99 wait",
		Summary: "BTCUSDT short wait in mean_reversion with trend_conflict.",
		MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
			"symbols":            []string{"BTCUSDT"},
			"action_types":       []string{"wait"},
			"selection_buckets":  []string{"mean_reversion"},
			"reject_reasons":     []string{"trend_conflict"},
			"candidate_sources":  []string{"manual"},
			"sessions":           []string{"us"},
			"trend_regimes":      []string{"downtrend"},
			"volatility_regimes": []string{"low_vol"},
			"oi_regimes":         []string{"oi_rising"},
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

func TestSemanticMemoryBenchmarkSymbolBehaviorPriorRelevance(t *testing.T) {
	query := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeSymbolBehaviorPrior,
		Title:   "Symbol prior RAVEUSDT long confirmed",
		Summary: "Bias negative, validated, breakout chase under flat OI keeps failing.",
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
	similar := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeSymbolBehaviorPrior,
		Title:   "Symbol prior RAVEUSDT long confirmed",
		Summary: "Bias negative, validated, breakout chase under flat OI keeps failing.",
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
	different := &SemanticMemoryDocument{
		DocType: SemanticMemoryDocTypeSymbolBehaviorPrior,
		Title:   "Symbol prior BTCUSDT short false positive",
		Summary: "Bias positive, rejected, oversold fade recovered.",
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

	goodScore := semanticMemoryBenchmarkRelevance(query, similar)
	badScore := semanticMemoryBenchmarkRelevance(query, different)
	if goodScore <= badScore {
		t.Fatalf("goodScore = %f, badScore = %f; want goodScore > badScore", goodScore, badScore)
	}
	if goodScore < semanticMemoryBenchmarkRelevantThreshold {
		t.Fatalf("goodScore = %f, want >= %f", goodScore, semanticMemoryBenchmarkRelevantThreshold)
	}
}
