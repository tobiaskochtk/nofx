package store

import (
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
		ConfigPatchJSON:  `{"risk":{"same_symbol_cooldown_minutes":180}}`,
		PromptPatchJSON:  `{"strategy.prompt_sections.entry_standards":"avoid instant re-entry after loss"}`,
		ValidationJSON:   `{"status":"passed"}`,
		MetadataJSON:     `{"gate_reasons":["critic_requested_narrower_patch"]}`,
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
