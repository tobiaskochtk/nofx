package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SemanticMemorySearchPreset struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	UserID     string    `gorm:"column:user_id;not null;index:idx_semantic_memory_search_presets_user_trader" json:"user_id"`
	TraderID   string    `gorm:"column:trader_id;not null;index:idx_semantic_memory_search_presets_user_trader" json:"trader_id"`
	Name       string    `gorm:"column:name;not null;index:idx_semantic_memory_search_presets_name" json:"name"`
	ConfigJSON string    `gorm:"column:config_json;type:text;default:'{}'" json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (SemanticMemorySearchPreset) TableName() string {
	return "semantic_memory_search_presets"
}

type SemanticMemorySearchPresetDetail struct {
	Preset SemanticMemorySearchPreset `json:"preset"`
	Config map[string]any             `json:"config,omitempty"`
}

func (s *SemanticMemoryStore) SaveSearchPreset(preset *SemanticMemorySearchPreset) error {
	if preset == nil {
		return fmt.Errorf("semantic-memory preset cannot be nil")
	}
	if strings.TrimSpace(preset.ID) == "" {
		preset.ID = uuid.NewString()
	}
	preset.UserID = strings.TrimSpace(preset.UserID)
	preset.TraderID = strings.TrimSpace(preset.TraderID)
	preset.Name = strings.TrimSpace(preset.Name)
	if preset.UserID == "" {
		return fmt.Errorf("semantic-memory preset user_id cannot be empty")
	}
	if preset.TraderID == "" {
		return fmt.Errorf("semantic-memory preset trader_id cannot be empty")
	}
	if preset.Name == "" {
		return fmt.Errorf("semantic-memory preset name cannot be empty")
	}
	if strings.TrimSpace(preset.ConfigJSON) == "" {
		preset.ConfigJSON = "{}"
	}
	return s.db.Save(preset).Error
}

func (s *SemanticMemoryStore) ListSearchPresets(userID, traderID string) ([]SemanticMemorySearchPresetDetail, error) {
	var presets []SemanticMemorySearchPreset
	if err := s.db.Model(&SemanticMemorySearchPreset{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID)).
		Order("updated_at DESC, created_at DESC").
		Find(&presets).Error; err != nil {
		return nil, err
	}
	result := make([]SemanticMemorySearchPresetDetail, 0, len(presets))
	for _, preset := range presets {
		result = append(result, buildSemanticMemorySearchPresetDetail(preset))
	}
	return result, nil
}

func (s *SemanticMemoryStore) DeleteSearchPreset(userID, traderID, presetID string) error {
	return s.db.
		Where("id = ? AND user_id = ? AND trader_id = ?", strings.TrimSpace(presetID), strings.TrimSpace(userID), strings.TrimSpace(traderID)).
		Delete(&SemanticMemorySearchPreset{}).Error
}

func buildSemanticMemorySearchPresetDetail(preset SemanticMemorySearchPreset) SemanticMemorySearchPresetDetail {
	detail := SemanticMemorySearchPresetDetail{
		Preset: preset,
		Config: map[string]any{},
	}
	if strings.TrimSpace(preset.ConfigJSON) == "" || strings.TrimSpace(preset.ConfigJSON) == "{}" {
		return detail
	}
	if err := json.Unmarshal([]byte(preset.ConfigJSON), &detail.Config); err != nil {
		detail.Config = map[string]any{}
	}
	return detail
}
