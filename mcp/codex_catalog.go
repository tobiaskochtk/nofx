package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const codexConfigDirEnv = "CODEX_CONFIG_DIR"

// CodexCachedModel describes a visible model exposed by the local Codex CLI.
type CodexCachedModel struct {
	ID    string
	Label string
}

type codexModelsCache struct {
	Models []struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"display_name"`
		Visibility  string `json:"visibility"`
	} `json:"models"`
}

func resolveCodexConfigDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(codexConfigDirEnv)); dir != "" {
		return filepath.Clean(dir), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve Codex config directory: %w", err)
	}
	if strings.TrimSpace(homeDir) == "" {
		return "", fmt.Errorf("failed to resolve Codex config directory: user home is empty")
	}

	return filepath.Join(homeDir, ".codex"), nil
}

// CodexConfigDir returns the directory that contains the local Codex CLI auth
// and model-cache files.
func CodexConfigDir() (string, error) {
	return resolveCodexConfigDir()
}

// LoadCodexVisibleModels returns the user-visible models from the local Codex
// CLI cache. This lets the UI mirror the actual client-side model selector.
func LoadCodexVisibleModels() ([]CodexCachedModel, error) {
	configDir, err := CodexConfigDir()
	if err != nil {
		return nil, err
	}

	cachePath := filepath.Join(configDir, "models_cache.json")
	body, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Codex models cache at %s: %w", cachePath, err)
	}

	return ParseCodexVisibleModels(body)
}

// ParseCodexVisibleModels parses a Codex models cache document and returns the
// models that should appear in end-user selectors.
func ParseCodexVisibleModels(body []byte) ([]CodexCachedModel, error) {
	var parsed codexModelsCache
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse Codex models cache: %w", err)
	}

	models := make([]CodexCachedModel, 0, len(parsed.Models))
	seen := make(map[string]struct{}, len(parsed.Models))
	for _, item := range parsed.Models {
		id := strings.TrimSpace(item.Slug)
		if id == "" {
			continue
		}

		visibility := strings.TrimSpace(item.Visibility)
		if visibility != "" && !strings.EqualFold(visibility, "list") {
			continue
		}

		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}

		label := strings.TrimSpace(item.DisplayName)
		if label == "" {
			label = id
		}

		models = append(models, CodexCachedModel{
			ID:    id,
			Label: label,
		})
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("Codex models cache does not contain any visible models")
	}

	return models, nil
}
