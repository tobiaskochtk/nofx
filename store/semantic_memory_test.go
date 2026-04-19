package store

import (
	"strings"
	"testing"
	"time"
)

func TestSemanticMemoryBackfillCoreDocuments(t *testing.T) {
	root := newPositionHistoryTestStore(t, "semantic-memory.db")

	now := time.Now().UTC()

	deal := &DealReviewCase{
		ID:                   "case-1",
		UserID:               "user-a",
		TraderID:             "trader-a",
		PositionID:           101,
		Symbol:               "RAVEUSDT",
		Side:                 "LONG",
		Status:               DealReviewCaseStatusClosed,
		Outcome:              "profit",
		EntryPrice:           9.84,
		ExitPrice:            10.06,
		EntryQuantity:        1.5,
		ExitQuantity:         1.5,
		Leverage:             5,
		CloseReason:          "take_profit",
		ExitOrigin:           DealReviewExitOriginSyncedTriggerOrder,
		ExitReasonQuality:    DealReviewExitReasonQualityExplicit,
		ExitEvidenceSummary:  "Trigger order matched exchange take profit.",
		RealizedPnL:          0.24,
		RealizedPnLPct:       11.97,
		OpenSelectionBucket:  "momentum",
		OpenTrendRegime:      "uptrend",
		OpenVolatilityRegime: "medium",
		OpenOIRegime:         "rising",
		LabelsJSON:           `["good_exit","momentum"]`,
		AnalystNote:          "Held through first pullback and exited cleanly.",
		UpdatedAt:            now,
	}
	if err := root.GormDB().Create(deal).Error; err != nil {
		t.Fatalf("Create(deal) error = %v", err)
	}

	run := &AutonomousOptimizerRun{
		ID:               "run-1",
		UserID:           "user-a",
		TraderID:         "trader-a",
		Trigger:          AutonomousOptimizerRunTriggerManual,
		Status:           AutonomousOptimizerStatusBlockedByGate,
		Summary:          "Suggested re-entry guard was too broad for the window.",
		PrimaryModelName: "gpt-5.4",
		CriticModelName:  "gpt-5.4",
		ConfigPatchJSON:  `{"risk_control":{"adaptive_reentry_guard":{"enabled":true,"same_symbol_loss_cooldown_minutes":180}}}`,
		PromptPatchJSON:  `{"strategy":{"prompt_sections":{"entry_standards":"avoid instant re-entry after loss"}}}`,
		ValidationJSON:   `{"status":"passed","config_patch_paths":["risk_control.adaptive_reentry_guard.enabled","risk_control.adaptive_reentry_guard.same_symbol_loss_cooldown_minutes"],"gate_reasons":["critic_requested_narrower_patch"],"deferred_reasons":["cooldown_active"],"critic":{"recommended_action":"block_apply","blocking_issues":["mixed evidence"]}}`,
		MetadataJSON:     `{"backlog_items_created":2,"proposal":{"proposal_type":"config_patch","rationale":["same-symbol churn after losses"],"expected_effect":"Reduce revenge re-entries."}}`,
		UpdatedAt:        now,
	}
	if err := root.GormDB().Create(run).Error; err != nil {
		t.Fatalf("Create(run) error = %v", err)
	}

	backlog := &AutonomousOptimizerBacklogItem{
		ID:                 "backlog-1",
		UserID:             "user-a",
		TraderID:           "trader-a",
		RunID:              "run-1",
		Title:              "Add symbol-level re-entry memory",
		Category:           "risk_controls",
		Description:        "Persist recent same-symbol losses for short-term cooldown logic.",
		ExpectedImpact:     "Reduce revenge re-entries after avoidable losses.",
		Confidence:         0.78,
		ImplementationCost: 0.45,
		Urgency:            0.8,
		RecurrenceCount:    3,
		CompositeScore:     0.76,
		Status:             AutonomousOptimizerBacklogStatusNew,
		EvidenceJSON:       `["ENA repeat re-entry","RAVE repeat re-entry"]`,
		MetadataJSON:       `{"target":"same_symbol_loss_cooldown"}`,
		UpdatedAt:          now,
	}
	if err := root.GormDB().Create(backlog).Error; err != nil {
		t.Fatalf("Create(backlog) error = %v", err)
	}

	version := &DealReviewStrategyVersion{
		ID:                 "version-1",
		UserID:             "user-a",
		TraderID:           "trader-a",
		StrategyID:         "strategy-a",
		SourceType:         "ai_apply",
		Summary:            "Tightened same-symbol re-entry logic after repeated losses.",
		ExpectedEffect:     "Reduce revenge re-entries while preserving fresh breakouts.",
		TargetCohortJSON:   `{"symbol":"RAVEUSDT","side":"LONG"}`,
		PreviousConfigJSON: `{"risk":{"same_symbol_cooldown_minutes":0}}`,
		NextConfigJSON:     `{"risk":{"same_symbol_cooldown_minutes":180}}`,
		AppliedAt:          now,
		UpdatedAt:          now,
	}
	if err := root.DealReview().SaveStrategyVersion(version); err != nil {
		t.Fatalf("SaveStrategyVersion(version) error = %v", err)
	}

	result, err := root.SemanticMemory().BackfillCoreDocuments("user-a", "trader-a", 0)
	if err != nil {
		t.Fatalf("BackfillCoreDocuments() error = %v", err)
	}
	if result.Run == nil {
		t.Fatal("BackfillCoreDocuments() returned nil run")
	}
	if result.Run.TotalDocuments != 4 {
		t.Fatalf("TotalDocuments = %d, want 4", result.Run.TotalDocuments)
	}
	if result.Run.InsertedDocuments != 4 {
		t.Fatalf("InsertedDocuments = %d, want 4", result.Run.InsertedDocuments)
	}
	if result.Run.Status != SemanticMemorySyncStatusCompleted {
		t.Fatalf("run.Status = %q, want %q", result.Run.Status, SemanticMemorySyncStatusCompleted)
	}

	docs, err := root.SemanticMemory().ListDocuments("user-a", "trader-a", "", 10)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 4 {
		t.Fatalf("len(docs) = %d, want 4", len(docs))
	}
	for _, doc := range docs {
		if doc.TokenEstimate <= 0 {
			t.Fatalf("doc %s token_estimate = %d, want > 0", doc.ID, doc.TokenEstimate)
		}
		if doc.EmbeddingStatus != SemanticMemoryEmbeddingStatusPending {
			t.Fatalf("doc %s embedding_status = %q, want %q", doc.ID, doc.EmbeddingStatus, SemanticMemoryEmbeddingStatusPending)
		}
	}

	var optimizerDoc *SemanticMemoryDocument
	for _, doc := range docs {
		if doc.DocType == SemanticMemoryDocTypeAutonomousOptimizerRun {
			optimizerDoc = doc
			break
		}
	}
	if optimizerDoc == nil {
		t.Fatal("expected optimizer semantic-memory document to exist")
	}
	optimizerMeta := semanticMemoryParseJSONObject(optimizerDoc.MetadataJSON)
	if got := semanticMemoryMetadataString(optimizerMeta, "proposal_type"); got != "config_patch" {
		t.Fatalf("proposal_type = %q, want config_patch", got)
	}
	if got := semanticMemoryMetadataString(optimizerMeta, "critic_recommended_action"); got != "block_apply" {
		t.Fatalf("critic_recommended_action = %q, want block_apply", got)
	}
	if got := semanticMemoryMetadataString(optimizerMeta, "config_validation_status"); got != "passed" {
		t.Fatalf("config_validation_status = %q, want passed", got)
	}
	if got := len(semanticMemoryMetadataStringSlice(optimizerMeta, "config_patch_paths")); got < 2 {
		t.Fatalf("config_patch_paths length = %d, want >= 2", got)
	}
	if got := len(semanticMemoryMetadataStringSlice(optimizerMeta, "prompt_patch_keys")); got < 1 {
		t.Fatalf("prompt_patch_keys length = %d, want >= 1", got)
	}
	if got := semanticMemoryMetadataInt(optimizerMeta, "backlog_items_created"); got != 2 {
		t.Fatalf("backlog_items_created = %d, want 2", got)
	}
	if !strings.Contains(optimizerDoc.Body, "Critic recommended action: block_apply") {
		t.Fatalf("optimizer body should include critic action, got %q", optimizerDoc.Body)
	}
}

func TestSemanticMemoryBackfillDocumentsHonorsDocTypeFilter(t *testing.T) {
	root := newPositionHistoryTestStore(t, "semantic-memory-doc-type.db")

	now := time.Now().UTC()
	version := &DealReviewStrategyVersion{
		ID:                 "version-only",
		UserID:             "user-a",
		TraderID:           "trader-a",
		StrategyID:         "strategy-a",
		SourceType:         "rollback",
		Summary:            "Rolled back an overfit patch.",
		ExpectedEffect:     "Restore previous baseline behavior.",
		PreviousConfigJSON: `{"risk":{"same_symbol_cooldown_minutes":180}}`,
		NextConfigJSON:     `{"risk":{"same_symbol_cooldown_minutes":0}}`,
		AppliedAt:          now,
		UpdatedAt:          now,
	}
	if err := root.DealReview().SaveStrategyVersion(version); err != nil {
		t.Fatalf("SaveStrategyVersion(version) error = %v", err)
	}

	result, err := root.SemanticMemory().BackfillDocuments("user-a", "trader-a", []string{SemanticMemoryDocTypeStrategyVersion}, 0)
	if err != nil {
		t.Fatalf("BackfillDocuments() error = %v", err)
	}
	if result.Run.TotalDocuments != 1 {
		t.Fatalf("TotalDocuments = %d, want 1", result.Run.TotalDocuments)
	}
	docs, err := root.SemanticMemory().ListDocuments("user-a", "trader-a", "", 10)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d, want 1", len(docs))
	}
	if docs[0].DocType != SemanticMemoryDocTypeStrategyVersion {
		t.Fatalf("DocType = %q, want %q", docs[0].DocType, SemanticMemoryDocTypeStrategyVersion)
	}
}

func TestSemanticMemoryBackfillLearnedPatternDocuments(t *testing.T) {
	root := newPositionHistoryTestStore(t, "semantic-memory-learned-pattern.db")

	now := time.Now().UTC()
	cases := []DealReviewCase{
		{
			ID:                   "pattern-case-1",
			UserID:               "user-a",
			TraderID:             "trader-a",
			PositionID:           1001,
			Symbol:               "RAVEUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "profit",
			OpenSelectionBucket:  "momentum",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "medium",
			OpenOIRegime:         "rising",
			OpenSessionBucket:    "asia",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_1",
			OpenLiquidityTier:    "deep",
			OpenSpreadBucket:     "tight",
			OpenSlippageBucket:   "low",
			OpenConfidence:       82,
			PlannedRiskPct:       0.80,
			RealizedPnL:          0.9,
			RealizedPnLPct:       1.2,
			HoldDurationMs:       int64((18 * time.Minute) / time.Millisecond),
			ExitTimeMs:           now.Add(-3 * time.Hour).UnixMilli(),
		},
		{
			ID:                   "pattern-case-2",
			UserID:               "user-a",
			TraderID:             "trader-a",
			PositionID:           1002,
			Symbol:               "RAVEUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "profit",
			OpenSelectionBucket:  "momentum",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "medium",
			OpenOIRegime:         "rising",
			OpenSessionBucket:    "asia",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_1",
			OpenLiquidityTier:    "deep",
			OpenSpreadBucket:     "tight",
			OpenSlippageBucket:   "low",
			OpenConfidence:       84,
			PlannedRiskPct:       0.85,
			RealizedPnL:          1.1,
			RealizedPnLPct:       1.4,
			HoldDurationMs:       int64((16 * time.Minute) / time.Millisecond),
			ExitTimeMs:           now.Add(-2 * time.Hour).UnixMilli(),
		},
		{
			ID:                   "pattern-case-3",
			UserID:               "user-a",
			TraderID:             "trader-a",
			PositionID:           1003,
			Symbol:               "RAVEUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "profit",
			OpenSelectionBucket:  "momentum",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "medium",
			OpenOIRegime:         "rising",
			OpenSessionBucket:    "asia",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_1",
			OpenLiquidityTier:    "deep",
			OpenSpreadBucket:     "tight",
			OpenSlippageBucket:   "low",
			OpenConfidence:       86,
			PlannedRiskPct:       0.90,
			RealizedPnL:          1.0,
			RealizedPnLPct:       1.3,
			HoldDurationMs:       int64((14 * time.Minute) / time.Millisecond),
			ExitTimeMs:           now.Add(-1 * time.Hour).UnixMilli(),
		},
	}
	for idx := range cases {
		if err := root.GormDB().Create(&cases[idx]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", idx, err)
		}
	}

	result, err := root.SemanticMemory().BackfillDocuments(
		"user-a",
		"trader-a",
		[]string{SemanticMemoryDocTypeLearnedPattern},
		0,
	)
	if err != nil {
		t.Fatalf("BackfillDocuments(learned_pattern) error = %v", err)
	}
	if result.Run == nil || result.Run.TotalDocuments <= 0 {
		t.Fatalf("BackfillDocuments(learned_pattern) run = %#v, want non-zero documents", result.Run)
	}

	docs, err := root.SemanticMemory().ListDocuments("user-a", "trader-a", SemanticMemoryDocTypeLearnedPattern, 20)
	if err != nil {
		t.Fatalf("ListDocuments(learned_pattern) error = %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one learned-pattern semantic-memory document")
	}

	doc := docs[0]
	if doc.SourceID == "" {
		t.Fatal("learned-pattern doc SourceID = empty, want stable source id")
	}
	meta := semanticMemoryParseJSONObject(doc.MetadataJSON)
	if got := semanticMemoryMetadataString(meta, "pattern_id"); got == "" {
		t.Fatalf("pattern_id metadata = %q, want non-empty", got)
	}
	if got := semanticMemoryMetadataString(meta, "side"); got != "LONG" {
		t.Fatalf("side metadata = %q, want LONG", got)
	}
	if !strings.Contains(doc.Body, "Feature set:") {
		t.Fatalf("learned-pattern doc body should include feature set, got %q", doc.Body)
	}
}

func TestSemanticMemoryUpsertMarksChangedDocumentPendingAgain(t *testing.T) {
	root := newPositionHistoryTestStore(t, "semantic-memory-update.db")

	doc := &SemanticMemoryDocument{
		UserID:            "user-a",
		TraderID:          "trader-a",
		DocType:           SemanticMemoryDocTypeDealReviewCase,
		SourceID:          "case-1",
		SourceUpdatedAt:   time.Now().UTC(),
		Title:             "First title",
		Summary:           "Initial summary",
		Body:              "Initial body",
		MetadataJSON:      `{"symbol":"BTCUSDT"}`,
		EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
		EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		EmbeddingStatus:   SemanticMemoryEmbeddingStatusEmbedded,
		LastEmbeddedAt:    time.Now().UTC(),
	}
	outcome, err := root.SemanticMemory().UpsertDocument(doc)
	if err != nil {
		t.Fatalf("UpsertDocument(first) error = %v", err)
	}
	if outcome != semanticMemoryUpsertInserted {
		t.Fatalf("first outcome = %q, want %q", outcome, semanticMemoryUpsertInserted)
	}

	doc.Title = "Updated title"
	outcome, err = root.SemanticMemory().UpsertDocument(doc)
	if err != nil {
		t.Fatalf("UpsertDocument(update) error = %v", err)
	}
	if outcome != semanticMemoryUpsertUpdated {
		t.Fatalf("update outcome = %q, want %q", outcome, semanticMemoryUpsertUpdated)
	}

	items, err := root.SemanticMemory().ListDocuments("user-a", "trader-a", SemanticMemoryDocTypeDealReviewCase, 10)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].EmbeddingStatus != SemanticMemoryEmbeddingStatusPending {
		t.Fatalf("EmbeddingStatus = %q, want %q", items[0].EmbeddingStatus, SemanticMemoryEmbeddingStatusPending)
	}
	if items[0].Title != "Updated title" {
		t.Fatalf("Title = %q, want Updated title", items[0].Title)
	}
}

func TestSemanticMemoryDecisionRecordSummaryDocuments(t *testing.T) {
	root := newPositionHistoryTestStore(t, "semantic-memory-decision-summary.db")

	record := &DecisionRecord{
		TraderID:         "trader-a",
		CycleNumber:      77,
		Timestamp:        time.Now().UTC().Add(-10 * time.Minute),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		AccountState: AccountSnapshot{
			TotalBalance:     1200,
			AvailableBalance: 930,
			PositionCount:    2,
			MarginUsedPct:    18.5,
		},
		CandidateDetails: []CandidateDetail{
			{
				Symbol:          "RAVEUSDT",
				SelectionBucket: "primary",
				Sources:         []string{"ai500"},
				RejectReasons:   []string{"late_breakout"},
				MarketContext: &DealReviewMarketContextSnapshot{
					SessionBucket:    "asia",
					TrendRegime:      "uptrend",
					VolatilityRegime: "high_vol",
					OIRegime:         "oi_flat",
				},
			},
			{
				Symbol:          "ENAUSDT",
				SelectionBucket: "exploration",
				Sources:         []string{"ai500"},
				RejectReasons:   []string{"same_symbol_cooldown"},
			},
		},
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "RAVEUSDT",
				Confidence: 82,
				Reasoning:  "fresh breakout with OI support",
				Execution: &DecisionExecutionTelemetry{
					TerminalStatus: "submitted",
				},
			},
			{
				Action:     "hold",
				Symbol:     "ENAUSDT",
				Confidence: 55,
				Reasoning:  "same-symbol cooldown after recent loss",
				RejectReasons: []string{
					"same_symbol_cooldown",
				},
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(record); err != nil {
		t.Fatalf("LogDecision() error = %v", err)
	}

	result, err := root.SemanticMemory().BackfillDocuments("user-a", "trader-a", []string{SemanticMemoryDocTypeDecisionRecordSummary}, 0)
	if err != nil {
		t.Fatalf("BackfillDocuments() error = %v", err)
	}
	if result.Run.TotalDocuments != 1 {
		t.Fatalf("TotalDocuments = %d, want 1", result.Run.TotalDocuments)
	}

	docs, err := root.SemanticMemory().ListDocuments("user-a", "trader-a", SemanticMemoryDocTypeDecisionRecordSummary, 10)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d, want 1", len(docs))
	}
	meta := semanticMemoryParseJSONObject(docs[0].MetadataJSON)
	if got := semanticMemoryMetadataInt(meta, "cycle_number"); got != 77 {
		t.Fatalf("cycle_number = %d, want 77", got)
	}
	if got := len(semanticMemoryMetadataStringSlice(meta, "action_types")); got != 2 {
		t.Fatalf("action_types length = %d, want 2", got)
	}
	if got := len(semanticMemoryMetadataStringSlice(meta, "symbols")); got != 2 {
		t.Fatalf("symbols length = %d, want 2", got)
	}
	if got := len(semanticMemoryMetadataStringSlice(meta, "reject_reasons")); got < 2 {
		t.Fatalf("reject_reasons length = %d, want >= 2", got)
	}
	if !strings.Contains(docs[0].Body, "Cycle number: 77") {
		t.Fatalf("body = %q, want cycle number", docs[0].Body)
	}
}

func TestSemanticMemorySymbolBehaviorPriorDocuments(t *testing.T) {
	root := newPositionHistoryTestStore(t, "semantic-memory-symbol-prior.db")

	now := time.Now().UTC()
	cases := []DealReviewCase{
		{
			ID:                       "case-rave-1",
			UserID:                   "user-a",
			TraderID:                 "trader-a",
			PositionID:               101,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "loss",
			OpenSelectionBucket:      "momentum",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high_vol",
			OpenOIRegime:             "oi_flat",
			RealizedPnL:              -0.44,
			RealizedPnLPct:           -2.1,
			MaxFavorableExcursionPct: 0.7,
			MaxAdverseExcursionPct:   -2.7,
			HoldDurationMs:           int64((14 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-4 * time.Hour).UnixMilli(),
			CloseReason:              "stop_loss",
			ExitOrigin:               DealReviewExitOriginTrailingEngine,
		},
		{
			ID:                       "case-rave-2",
			UserID:                   "user-a",
			TraderID:                 "trader-a",
			PositionID:               102,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "loss",
			OpenSelectionBucket:      "momentum",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high_vol",
			OpenOIRegime:             "oi_flat",
			RealizedPnL:              -0.31,
			RealizedPnLPct:           -1.4,
			MaxFavorableExcursionPct: 0.5,
			MaxAdverseExcursionPct:   -1.9,
			HoldDurationMs:           int64((11 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-3 * time.Hour).UnixMilli(),
			CloseReason:              "stop_loss",
			ExitOrigin:               DealReviewExitOriginTrailingEngine,
		},
		{
			ID:                       "case-rave-3",
			UserID:                   "user-a",
			TraderID:                 "trader-a",
			PositionID:               103,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "loss",
			OpenSelectionBucket:      "momentum",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high_vol",
			OpenOIRegime:             "oi_flat",
			RealizedPnL:              -0.52,
			RealizedPnLPct:           -2.3,
			MaxFavorableExcursionPct: 0.9,
			MaxAdverseExcursionPct:   -2.8,
			HoldDurationMs:           int64((18 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-2 * time.Hour).UnixMilli(),
			CloseReason:              "stop_loss",
			ExitOrigin:               DealReviewExitOriginTrailingEngine,
		},
		{
			ID:                       "case-rave-4",
			UserID:                   "user-a",
			TraderID:                 "trader-a",
			PositionID:               104,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "profit",
			OpenSelectionBucket:      "momentum",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high_vol",
			OpenOIRegime:             "oi_flat",
			RealizedPnL:              0.08,
			RealizedPnLPct:           0.4,
			MaxFavorableExcursionPct: 1.4,
			MaxAdverseExcursionPct:   -0.6,
			ProfitGivenBackPct:       35,
			HoldDurationMs:           int64((9 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-90 * time.Minute).UnixMilli(),
			CloseReason:              "manual_exit",
			ExitOrigin:               DealReviewExitOriginAIDecision,
		},
		{
			ID:                       "case-rave-5",
			UserID:                   "user-a",
			TraderID:                 "trader-a",
			PositionID:               105,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "loss",
			OpenSelectionBucket:      "momentum",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high_vol",
			OpenOIRegime:             "oi_flat",
			RealizedPnL:              -0.27,
			RealizedPnLPct:           -1.1,
			MaxFavorableExcursionPct: 0.4,
			MaxAdverseExcursionPct:   -1.4,
			HoldDurationMs:           int64((8 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-35 * time.Minute).UnixMilli(),
			CloseReason:              "stop_loss",
			ExitOrigin:               DealReviewExitOriginTrailingEngine,
		},
	}
	for idx := range cases {
		if err := root.GormDB().Create(&cases[idx]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", idx, err)
		}
	}
	if _, err := root.DealReview().RebuildSymbolBehaviorPriors("user-a", "trader-a"); err != nil {
		t.Fatalf("RebuildSymbolBehaviorPriors() error = %v", err)
	}

	result, err := root.SemanticMemory().BackfillDocuments("user-a", "trader-a", []string{SemanticMemoryDocTypeSymbolBehaviorPrior}, 0)
	if err != nil {
		t.Fatalf("BackfillDocuments() error = %v", err)
	}
	if result.Run.TotalDocuments != 1 {
		t.Fatalf("TotalDocuments = %d, want 1", result.Run.TotalDocuments)
	}

	docs, err := root.SemanticMemory().ListDocuments("user-a", "trader-a", SemanticMemoryDocTypeSymbolBehaviorPrior, 10)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d, want 1", len(docs))
	}
	if docs[0].SourceID != "RAVEUSDT::LONG::momentum::uptrend::high_vol::oi_flat" {
		t.Fatalf("SourceID = %q, want stable symbol-prior key", docs[0].SourceID)
	}
	meta := semanticMemoryParseJSONObject(docs[0].MetadataJSON)
	if got := semanticMemoryMetadataString(meta, "prior_id"); got == "" {
		t.Fatal("prior_id should be present in metadata")
	}
	if got := semanticMemoryMetadataString(meta, "validation_label"); got == "" {
		t.Fatal("validation_label should be present in metadata")
	}
	if got := semanticMemoryMetadataString(meta, "symbol"); got != "RAVEUSDT" {
		t.Fatalf("symbol = %q, want RAVEUSDT", got)
	}
	if !strings.Contains(docs[0].Body, "Symbol: RAVEUSDT") {
		t.Fatalf("body = %q, want symbol details", docs[0].Body)
	}
}
