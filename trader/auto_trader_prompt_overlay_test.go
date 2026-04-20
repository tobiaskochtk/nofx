package trader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nofx/decision"
	"nofx/store"
)

func TestBuildEffectiveStrategyConfigPrependsPromptTemplateOverlay(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	promptsDir := filepath.Join(wd, "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(prompts) error = %v", err)
	}
	templateName := "test_overlay_prompt"
	templateText := "# TEST OVERLAY\n\nUse memory-aware execution discipline."
	templatePath := filepath.Join(promptsDir, templateName+".txt")
	if err := os.WriteFile(templatePath, []byte(templateText), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", templatePath, err)
	}
	t.Cleanup(func() {
		_ = os.Remove(templatePath)
		_ = os.Remove(promptsDir)
	})
	if err := decision.ReloadPromptTemplates(); err != nil {
		t.Fatalf("ReloadPromptTemplates() error = %v", err)
	}

	base := store.GetDefaultStrategyConfig("en")

	effective, err := buildEffectiveStrategyConfig(&base, "Trader-specific note", false, templateName)
	if err != nil {
		t.Fatalf("buildEffectiveStrategyConfig() error = %v", err)
	}
	if effective == nil {
		t.Fatal("buildEffectiveStrategyConfig() = nil, want effective config")
	}

	loadedTemplateText := strings.TrimSpace(loadPromptTemplateOverlay(templateName))
	if loadedTemplateText == "" {
		t.Fatal("loadPromptTemplateOverlay() returned empty content")
	}
	if !strings.HasPrefix(effective.PromptSections.RoleDefinition, loadedTemplateText) {
		t.Fatalf("RoleDefinition does not start with overlay.\nwant prefix: %q\n got: %q", loadedTemplateText[:min(64, len(loadedTemplateText))], effective.PromptSections.RoleDefinition[:min(64, len(effective.PromptSections.RoleDefinition))])
	}
	if !strings.Contains(effective.PromptSections.RoleDefinition, base.PromptSections.RoleDefinition) {
		t.Fatal("base role definition missing after overlay prepend")
	}
	if !strings.Contains(effective.CustomPrompt, "Trader-specific note") {
		t.Fatalf("CustomPrompt = %q, want trader prompt to remain appended", effective.CustomPrompt)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
