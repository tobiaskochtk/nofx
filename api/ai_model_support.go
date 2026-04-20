package api

import (
	"strings"

	"nofx/store"
)

func providerUsesAmbientCredentials(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codex":
		return true
	default:
		return false
	}
}

func providerUsesLocalModelCatalog(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codex":
		return true
	default:
		return false
	}
}

func modelHasUsableCredentials(model *store.AIModel) bool {
	if model == nil {
		return false
	}
	if providerUsesAmbientCredentials(model.Provider) {
		return true
	}
	return strings.TrimSpace(string(model.APIKey)) != ""
}

func buildAmbientModelConfig(modelID string) *store.AIModel {
	provider := strings.ToLower(strings.TrimSpace(modelID))
	if !providerUsesAmbientCredentials(provider) {
		return nil
	}

	name := strings.Title(provider)
	switch provider {
	case "codex":
		name = "Codex"
	}

	return &store.AIModel{
		ID:       modelID,
		Name:     name,
		Provider: provider,
		Enabled:  true,
	}
}
