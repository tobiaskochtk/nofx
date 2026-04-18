package store

import (
	"testing"
	"time"
)

func TestResolveOpenAIEmbeddingsURL(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "default", input: "", expect: "https://api.openai.com/v1/embeddings"},
		{name: "base v1", input: "https://api.openai.com/v1", expect: "https://api.openai.com/v1/embeddings"},
		{name: "chat completions", input: "https://api.openai.com/v1/chat/completions", expect: "https://api.openai.com/v1/embeddings"},
		{name: "custom compatible", input: "https://example.com/openai/v1", expect: "https://example.com/openai/v1/embeddings"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveOpenAIEmbeddingsURL(tc.input); got != tc.expect {
				t.Fatalf("resolveOpenAIEmbeddingsURL(%q) = %q, want %q", tc.input, got, tc.expect)
			}
		})
	}
}

func TestSemanticMemorySplitEmbeddingBatchesRespectsInputAndTokenLimits(t *testing.T) {
	makeDoc := func(id string, tokens int) *SemanticMemoryDocument {
		return &SemanticMemoryDocument{
			ID:            id,
			TokenEstimate: tokens,
			Title:         id,
			Summary:       "summary",
			Body:          "body",
		}
	}
	docs := []*SemanticMemoryDocument{
		makeDoc("a", 150000),
		makeDoc("b", 60000),
		makeDoc("c", 60000),
	}
	batches := semanticMemorySplitEmbeddingBatches(docs)
	if len(batches) != 2 {
		t.Fatalf("len(batches) = %d, want 2", len(batches))
	}
	if len(batches[0]) != 1 {
		t.Fatalf("len(batches[0]) = %d, want 1", len(batches[0]))
	}
	if len(batches[1]) != 2 {
		t.Fatalf("len(batches[1]) = %d, want 2", len(batches[1]))
	}
}

func TestSemanticMemoryListDocumentsForEmbeddingAndRequeueHonorsDocTypes(t *testing.T) {
	root := newPositionHistoryTestStore(t, "semantic-memory-embed-filter.db")
	now := time.Now().UTC()

	docs := []*SemanticMemoryDocument{
		{
			ID:                "case-doc",
			UserID:            "user-a",
			TraderID:          "trader-a",
			DocType:           SemanticMemoryDocTypeDealReviewCase,
			SourceID:          "case-1",
			SourceUpdatedAt:   now,
			Title:             "Case",
			Summary:           "Pending case",
			Body:              "Body",
			EmbeddingStatus:   SemanticMemoryEmbeddingStatusPending,
			EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
			EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		},
		{
			ID:                "version-doc",
			UserID:            "user-a",
			TraderID:          "trader-a",
			DocType:           SemanticMemoryDocTypeStrategyVersion,
			SourceID:          "version-1",
			SourceUpdatedAt:   now,
			Title:             "Version",
			Summary:           "Embedded version",
			Body:              "Body",
			EmbeddingStatus:   SemanticMemoryEmbeddingStatusEmbedded,
			EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
			EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		},
	}
	for _, doc := range docs {
		if _, err := root.SemanticMemory().UpsertDocument(doc); err != nil {
			t.Fatalf("UpsertDocument(%s) error = %v", doc.ID, err)
		}
	}

	filtered, err := root.SemanticMemory().ListDocumentsForEmbedding(
		"user-a",
		"trader-a",
		[]string{SemanticMemoryDocTypeStrategyVersion},
		[]string{SemanticMemoryEmbeddingStatusPending},
		10,
	)
	if err != nil {
		t.Fatalf("ListDocumentsForEmbedding() error = %v", err)
	}
	if len(filtered) != 0 {
		t.Fatalf("len(filtered) = %d, want 0", len(filtered))
	}

	queued, err := root.SemanticMemory().MarkDocumentsPendingForEmbedding(
		"user-a",
		"trader-a",
		[]string{SemanticMemoryDocTypeStrategyVersion},
	)
	if err != nil {
		t.Fatalf("MarkDocumentsPendingForEmbedding() error = %v", err)
	}
	if queued != 1 {
		t.Fatalf("queued = %d, want 1", queued)
	}

	filtered, err = root.SemanticMemory().ListDocumentsForEmbedding(
		"user-a",
		"trader-a",
		[]string{SemanticMemoryDocTypeStrategyVersion},
		[]string{SemanticMemoryEmbeddingStatusPending},
		10,
	)
	if err != nil {
		t.Fatalf("ListDocumentsForEmbedding(after requeue) error = %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("len(filtered) after requeue = %d, want 1", len(filtered))
	}
	if filtered[0].DocType != SemanticMemoryDocTypeStrategyVersion {
		t.Fatalf("DocType = %q, want %q", filtered[0].DocType, SemanticMemoryDocTypeStrategyVersion)
	}
}
