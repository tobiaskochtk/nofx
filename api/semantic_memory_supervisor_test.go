package api

import (
	"testing"

	"nofx/store"
)

func TestCollectSemanticMemoryRefreshScopesDedupesAndSkipsInvalid(t *testing.T) {
	traders := []*store.Trader{
		nil,
		{UserID: "user-a", ID: "trader-a", Name: "Alpha"},
		{UserID: "user-a", ID: "trader-a", Name: "Alpha duplicate"},
		{UserID: "user-a", ID: "trader-b", Name: "Beta"},
		{UserID: "", ID: "missing-user"},
		{UserID: "user-c", ID: ""},
	}

	scopes := collectSemanticMemoryRefreshScopes(traders)
	if len(scopes) != 2 {
		t.Fatalf("len(scopes) = %d, want 2", len(scopes))
	}
	if scopes[0].UserID != "user-a" || scopes[0].TraderID != "trader-a" {
		t.Fatalf("scope[0] = %#v, want user-a/trader-a", scopes[0])
	}
	if scopes[1].UserID != "user-a" || scopes[1].TraderID != "trader-b" {
		t.Fatalf("scope[1] = %#v, want user-a/trader-b", scopes[1])
	}
}

func TestShouldLogSemanticMemoryRefresh(t *testing.T) {
	if !shouldLogSemanticMemoryRefresh(nil, 1, 120) {
		t.Fatal("expected embedded docs to trigger logging")
	}

	if shouldLogSemanticMemoryRefresh(nil, 0, 0) {
		t.Fatal("nil result without embedding activity should not log")
	}

	result := &store.SemanticMemoryBackfillResult{
		Run: &store.SemanticMemorySyncRun{
			InsertedDocuments: 0,
			UpdatedDocuments:  0,
			FailedDocuments:   0,
		},
	}
	if shouldLogSemanticMemoryRefresh(result, 0, 0) {
		t.Fatal("no-op backfill should not log")
	}

	result.Run.UpdatedDocuments = 2
	if !shouldLogSemanticMemoryRefresh(result, 0, 0) {
		t.Fatal("updated documents should trigger logging")
	}
}
