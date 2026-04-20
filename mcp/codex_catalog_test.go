package mcp

import "testing"

func TestParseCodexVisibleModelsFiltersHiddenEntries(t *testing.T) {
	body := []byte(`{
		"models": [
			{"slug":"gpt-5.4","display_name":"gpt-5.4","visibility":"list"},
			{"slug":"gpt-5.4-mini","display_name":"gpt-5.4-mini","visibility":"list"},
			{"slug":"codex-auto-review","display_name":"codex-auto-review","visibility":"hidden"}
		]
	}`)

	models, err := ParseCodexVisibleModels(body)
	if err != nil {
		t.Fatalf("ParseCodexVisibleModels() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2 visible models", len(models))
	}
	if models[0].ID != "gpt-5.4" {
		t.Fatalf("models[0].ID = %q, want gpt-5.4", models[0].ID)
	}
	if models[1].ID != "gpt-5.4-mini" {
		t.Fatalf("models[1].ID = %q, want gpt-5.4-mini", models[1].ID)
	}
}

func TestParseCodexVisibleModelsRequiresVisibleEntries(t *testing.T) {
	body := []byte(`{
		"models": [
			{"slug":"codex-auto-review","display_name":"codex-auto-review","visibility":"hidden"}
		]
	}`)

	_, err := ParseCodexVisibleModels(body)
	if err == nil {
		t.Fatal("expected error when no visible Codex models are present")
	}
}
