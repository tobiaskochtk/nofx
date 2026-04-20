package store

import "testing"

func TestSemanticMemorySearchPresetRoundTrip(t *testing.T) {
	root := newPositionHistoryTestStore(t, "semantic-memory-presets.db")

	preset := &SemanticMemorySearchPreset{
		UserID:   "user-a",
		TraderID: "trader-a",
		Name:     "Re-entry losses",
		ConfigJSON: `{
			"query":"same-symbol re-entry losses after failed breakout",
			"doc_scope":"deal_review_case",
			"doc_types":["deal_review_case"],
			"outcome":"loss",
			"limit":12
		}`,
	}
	if err := root.SemanticMemory().SaveSearchPreset(preset); err != nil {
		t.Fatalf("SaveSearchPreset() error = %v", err)
	}

	items, err := root.SemanticMemory().ListSearchPresets("user-a", "trader-a")
	if err != nil {
		t.Fatalf("ListSearchPresets() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Preset.Name != "Re-entry losses" {
		t.Fatalf("preset name = %q, want Re-entry losses", items[0].Preset.Name)
	}
	if items[0].Config["query"] != "same-symbol re-entry losses after failed breakout" {
		t.Fatalf("query = %#v, want saved query", items[0].Config["query"])
	}

	preset.Name = "Updated preset"
	preset.ConfigJSON = `{"query":"narrower query","doc_scope":"strategy_version"}`
	if err := root.SemanticMemory().SaveSearchPreset(preset); err != nil {
		t.Fatalf("SaveSearchPreset(update) error = %v", err)
	}

	items, err = root.SemanticMemory().ListSearchPresets("user-a", "trader-a")
	if err != nil {
		t.Fatalf("ListSearchPresets(after update) error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) after update = %d, want 1", len(items))
	}
	if items[0].Preset.Name != "Updated preset" {
		t.Fatalf("updated preset name = %q, want Updated preset", items[0].Preset.Name)
	}
	if items[0].Config["doc_scope"] != "strategy_version" {
		t.Fatalf("doc_scope = %#v, want strategy_version", items[0].Config["doc_scope"])
	}

	if err := root.SemanticMemory().DeleteSearchPreset("user-a", "trader-a", preset.ID); err != nil {
		t.Fatalf("DeleteSearchPreset() error = %v", err)
	}
	items, err = root.SemanticMemory().ListSearchPresets("user-a", "trader-a")
	if err != nil {
		t.Fatalf("ListSearchPresets(after delete) error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("len(items) after delete = %d, want 0", len(items))
	}
}
