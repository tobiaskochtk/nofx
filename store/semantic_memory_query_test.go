package store

import (
	"testing"
	"time"
)

func TestSemanticMemoryGetCorpusStatusIncludesObservability(t *testing.T) {
	root := newPositionHistoryTestStore(t, "semantic-memory-status.db")
	now := time.Now().UTC()

	docs := []*SemanticMemoryDocument{
		{
			ID:              "doc-embedded",
			UserID:          "user-a",
			TraderID:        "trader-a",
			DocType:         SemanticMemoryDocTypeDealReviewCase,
			SourceID:        "case-1",
			SourceUpdatedAt: now,
			Title:           "Embedded case",
			ContentHash:     "hash-1",
			TokenEstimate:   120,
			EmbeddingStatus: SemanticMemoryEmbeddingStatusEmbedded,
			EmbeddingModel:  SemanticMemoryDefaultEmbeddingModel,
			LastEmbeddedAt:  now,
			LastBuiltAt:     now,
		},
		{
			ID:              "doc-pending",
			UserID:          "user-a",
			TraderID:        "trader-a",
			DocType:         SemanticMemoryDocTypeAutonomousOptimizerRun,
			SourceID:        "run-1",
			SourceUpdatedAt: now,
			Title:           "Pending run",
			ContentHash:     "hash-2",
			TokenEstimate:   80,
			EmbeddingStatus: SemanticMemoryEmbeddingStatusPending,
			LastBuiltAt:     now,
		},
		{
			ID:                 "doc-failed",
			UserID:             "user-a",
			TraderID:           "trader-a",
			DocType:            SemanticMemoryDocTypeOptimizerBacklogItem,
			SourceID:           "backlog-1",
			SourceUpdatedAt:    now,
			Title:              "Failed backlog",
			ContentHash:        "hash-3",
			TokenEstimate:      45,
			EmbeddingStatus:    SemanticMemoryEmbeddingStatusFailed,
			LastEmbeddingError: "upstream timeout",
			LastBuiltAt:        now,
		},
	}
	for _, doc := range docs {
		if err := root.GormDB().Create(doc).Error; err != nil {
			t.Fatalf("Create(doc %s) error = %v", doc.ID, err)
		}
	}

	runs := []*SemanticMemorySyncRun{
		{
			ID:               "run-embed-1",
			UserID:           "user-a",
			TraderID:         "trader-a",
			Scope:            SemanticMemorySyncScopeEmbedPending,
			Status:           SemanticMemorySyncStatusCompleted,
			Summary:          "Embedded 3 semantic documents.",
			TotalDocuments:   3,
			UpdatedDocuments: 3,
			MetadataJSON: semanticMemoryMarshalJSON(map[string]any{
				"embedding_provider": "openai",
				"embedding_model":    SemanticMemoryDefaultEmbeddingModel,
				"prompt_tokens":      345,
				"estimated_cost_usd": semanticMemoryEstimatedCostUSD(SemanticMemoryDefaultEmbeddingModel, 345),
				"doc_types": []string{
					SemanticMemoryDocTypeDealReviewCase,
					SemanticMemoryDocTypeAutonomousOptimizerRun,
				},
			}),
			StartedAt:   now.Add(-10 * time.Minute),
			CompletedAt: now.Add(-9 * time.Minute),
			CreatedAt:   now.Add(-10 * time.Minute),
			UpdatedAt:   now.Add(-9 * time.Minute),
		},
		{
			ID:              "run-core-failed",
			UserID:          "user-a",
			TraderID:        "trader-a",
			Scope:           SemanticMemorySyncScopeCoreKnowledge,
			Status:          SemanticMemorySyncStatusFailed,
			Summary:         "Backfill failed.",
			FailedDocuments: 2,
			ErrorMessage:    "vector runtime unavailable",
			StartedAt:       now.Add(-5 * time.Minute),
			CompletedAt:     now.Add(-4 * time.Minute),
			CreatedAt:       now.Add(-5 * time.Minute),
			UpdatedAt:       now.Add(-4 * time.Minute),
		},
	}
	for _, run := range runs {
		if err := root.GormDB().Create(run).Error; err != nil {
			t.Fatalf("Create(run %s) error = %v", run.ID, err)
		}
	}

	status, err := root.SemanticMemory().GetCorpusStatus("user-a", "trader-a", 8)
	if err != nil {
		t.Fatalf("GetCorpusStatus() error = %v", err)
	}
	if status.TotalDocuments != 3 {
		t.Fatalf("TotalDocuments = %d, want 3", status.TotalDocuments)
	}
	if status.EmbeddedDocuments != 1 || status.PendingDocuments != 1 || status.FailedDocuments != 1 {
		t.Fatalf("unexpected document counts: embedded=%d pending=%d failed=%d", status.EmbeddedDocuments, status.PendingDocuments, status.FailedDocuments)
	}
	if status.TotalTokenEstimate != 245 {
		t.Fatalf("TotalTokenEstimate = %d, want 245", status.TotalTokenEstimate)
	}
	if status.EmbeddedTokenEstimate != 120 || status.PendingTokenEstimate != 80 || status.FailedTokenEstimate != 45 {
		t.Fatalf("unexpected token counts: embedded=%d pending=%d failed=%d", status.EmbeddedTokenEstimate, status.PendingTokenEstimate, status.FailedTokenEstimate)
	}
	if status.CostEstimate.EmbeddingModel != SemanticMemoryDefaultEmbeddingModel {
		t.Fatalf("CostEstimate.EmbeddingModel = %q, want %q", status.CostEstimate.EmbeddingModel, SemanticMemoryDefaultEmbeddingModel)
	}
	if status.CostEstimate.PricePer1MTokensUSD != semanticMemoryTextEmbedding3SmallCostPer1MTokens {
		t.Fatalf("PricePer1MTokensUSD = %f, want %f", status.CostEstimate.PricePer1MTokensUSD, semanticMemoryTextEmbedding3SmallCostPer1MTokens)
	}
	expectedPendingCost := semanticMemoryEstimatedCostUSD(SemanticMemoryDefaultEmbeddingModel, 80)
	if status.CostEstimate.EstimatedPendingCostUSD != expectedPendingCost {
		t.Fatalf("EstimatedPendingCostUSD = %f, want %f", status.CostEstimate.EstimatedPendingCostUSD, expectedPendingCost)
	}
	if status.EmbeddingUsage.RecentRuns != 1 {
		t.Fatalf("EmbeddingUsage.RecentRuns = %d, want 1", status.EmbeddingUsage.RecentRuns)
	}
	if status.EmbeddingUsage.RecentEmbeddedDocuments != 3 || status.EmbeddingUsage.RecentPromptTokens != 345 {
		t.Fatalf("unexpected embedding usage summary: docs=%d tokens=%d", status.EmbeddingUsage.RecentEmbeddedDocuments, status.EmbeddingUsage.RecentPromptTokens)
	}
	if status.FailureSummary.RecentFailedRuns != 1 {
		t.Fatalf("FailureSummary.RecentFailedRuns = %d, want 1", status.FailureSummary.RecentFailedRuns)
	}
	if status.FailureSummary.LastFailureScope != SemanticMemorySyncScopeCoreKnowledge {
		t.Fatalf("LastFailureScope = %q, want %q", status.FailureSummary.LastFailureScope, SemanticMemorySyncScopeCoreKnowledge)
	}
	if status.FailureSummary.LastFailureMessage != "vector runtime unavailable" {
		t.Fatalf("LastFailureMessage = %q, want %q", status.FailureSummary.LastFailureMessage, "vector runtime unavailable")
	}
	if len(status.DocTypes) != 3 {
		t.Fatalf("len(DocTypes) = %d, want 3", len(status.DocTypes))
	}
}
