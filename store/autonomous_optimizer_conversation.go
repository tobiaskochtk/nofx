package store

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	AutonomousOptimizerConversationPurposeProposal = "proposal"
	AutonomousOptimizerConversationPurposeCritic   = "critic"

	AutonomousOptimizerConversationRoleSystem    = "system"
	AutonomousOptimizerConversationRoleUser      = "user"
	AutonomousOptimizerConversationRoleAssistant = "assistant"

	AutonomousOptimizerDefaultConversationReplayMessages = 8
)

type AutonomousOptimizerConversation struct {
	ID                 string    `gorm:"primaryKey" json:"id"`
	UserID             string    `gorm:"column:user_id;not null;index:idx_autonomous_optimizer_conversations_scope,unique" json:"user_id"`
	TraderID           string    `gorm:"column:trader_id;not null;index:idx_autonomous_optimizer_conversations_scope,unique" json:"trader_id"`
	Purpose            string    `gorm:"column:purpose;not null;index:idx_autonomous_optimizer_conversations_scope,unique" json:"purpose"`
	ModelConfigID      string    `gorm:"column:model_config_id;default:''" json:"model_config_id"`
	ModelName          string    `gorm:"column:model_name;default:''" json:"model_name"`
	LastRunID          string    `gorm:"column:last_run_id;default:''" json:"last_run_id"`
	LastMessageAt      time.Time `gorm:"column:last_message_at;index:idx_autonomous_optimizer_conversations_last_message" json:"last_message_at"`
	ReplayMessageLimit int       `gorm:"column:replay_message_limit;default:8" json:"replay_message_limit"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (AutonomousOptimizerConversation) TableName() string {
	return "autonomous_optimizer_conversations"
}

type AutonomousOptimizerConversationMessage struct {
	ID             string    `gorm:"primaryKey" json:"id"`
	ConversationID string    `gorm:"column:conversation_id;not null;index:idx_autonomous_optimizer_conversation_messages_conversation_seq" json:"conversation_id"`
	UserID         string    `gorm:"column:user_id;not null;index:idx_autonomous_optimizer_conversation_messages_scope" json:"user_id"`
	TraderID       string    `gorm:"column:trader_id;not null;index:idx_autonomous_optimizer_conversation_messages_scope" json:"trader_id"`
	RunID          string    `gorm:"column:run_id;default:'';index:idx_autonomous_optimizer_conversation_messages_run" json:"run_id"`
	Purpose        string    `gorm:"column:purpose;not null;index:idx_autonomous_optimizer_conversation_messages_scope" json:"purpose"`
	Role           string    `gorm:"column:role;not null" json:"role"`
	Sequence       int       `gorm:"column:sequence;not null;index:idx_autonomous_optimizer_conversation_messages_conversation_seq" json:"sequence"`
	Content        string    `gorm:"column:content;type:text;default:''" json:"content"`
	ReplayContent  string    `gorm:"column:replay_content;type:text;default:''" json:"replay_content"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (AutonomousOptimizerConversationMessage) TableName() string {
	return "autonomous_optimizer_conversation_messages"
}

func normalizeAutonomousOptimizerConversationPurpose(value string) string {
	switch value {
	case AutonomousOptimizerConversationPurposeCritic:
		return AutonomousOptimizerConversationPurposeCritic
	default:
		return AutonomousOptimizerConversationPurposeProposal
	}
}

func normalizeAutonomousOptimizerConversationRole(value string) string {
	switch value {
	case AutonomousOptimizerConversationRoleSystem,
		AutonomousOptimizerConversationRoleAssistant:
		return value
	default:
		return AutonomousOptimizerConversationRoleUser
	}
}

func normalizeAutonomousOptimizerConversationReplayLimit(limit int) int {
	if limit <= 0 {
		return AutonomousOptimizerDefaultConversationReplayMessages
	}
	if limit > 24 {
		return 24
	}
	return limit
}

func (s *AutonomousOptimizerStore) GetOrCreateConversation(userID, traderID, purpose, modelConfigID, modelName string) (*AutonomousOptimizerConversation, error) {
	purpose = normalizeAutonomousOptimizerConversationPurpose(purpose)
	var convo AutonomousOptimizerConversation
	err := s.db.Where("user_id = ? AND trader_id = ? AND purpose = ?", userID, traderID, purpose).First(&convo).Error
	if err == nil {
		changed := false
		if convo.ModelConfigID != modelConfigID {
			convo.ModelConfigID = modelConfigID
			changed = true
		}
		if convo.ModelName != modelName {
			convo.ModelName = modelName
			changed = true
		}
		normalizedLimit := normalizeAutonomousOptimizerConversationReplayLimit(convo.ReplayMessageLimit)
		if convo.ReplayMessageLimit != normalizedLimit {
			convo.ReplayMessageLimit = normalizedLimit
			changed = true
		}
		if changed {
			if saveErr := s.db.Save(&convo).Error; saveErr != nil {
				return nil, saveErr
			}
		}
		return &convo, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	convo = AutonomousOptimizerConversation{
		ID:                 uuid.NewString(),
		UserID:             userID,
		TraderID:           traderID,
		Purpose:            purpose,
		ModelConfigID:      modelConfigID,
		ModelName:          modelName,
		ReplayMessageLimit: AutonomousOptimizerDefaultConversationReplayMessages,
	}
	if err := s.db.Create(&convo).Error; err != nil {
		return nil, err
	}
	return &convo, nil
}

func (s *AutonomousOptimizerStore) ListConversationMessages(conversationID string, limit int) ([]*AutonomousOptimizerConversationMessage, error) {
	if conversationID == "" {
		return nil, nil
	}
	limit = normalizeAutonomousOptimizerConversationReplayLimit(limit)
	var items []*AutonomousOptimizerConversationMessage
	if err := s.db.
		Where("conversation_id = ?", conversationID).
		Order("sequence DESC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, nil
}

func (s *AutonomousOptimizerStore) AppendConversationMessages(conversation *AutonomousOptimizerConversation, runID string, messages []*AutonomousOptimizerConversationMessage) error {
	if conversation == nil {
		return fmt.Errorf("autonomous optimizer conversation cannot be nil")
	}
	if len(messages) == 0 {
		return nil
	}

	now := time.Now().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		var maxSequence int
		if err := tx.Model(&AutonomousOptimizerConversationMessage{}).
			Where("conversation_id = ?", conversation.ID).
			Select("COALESCE(MAX(sequence), 0)").
			Scan(&maxSequence).Error; err != nil {
			return err
		}

		nextSequence := maxSequence
		for _, item := range messages {
			if item == nil {
				continue
			}
			nextSequence++
			item.ID = uuid.NewString()
			item.ConversationID = conversation.ID
			item.UserID = conversation.UserID
			item.TraderID = conversation.TraderID
			item.Purpose = conversation.Purpose
			item.RunID = runID
			item.Role = normalizeAutonomousOptimizerConversationRole(item.Role)
			item.Sequence = nextSequence
			if err := tx.Create(item).Error; err != nil {
				return err
			}
		}

		conversation.LastRunID = runID
		conversation.LastMessageAt = now
		conversation.ReplayMessageLimit = normalizeAutonomousOptimizerConversationReplayLimit(conversation.ReplayMessageLimit)
		return tx.Save(conversation).Error
	})
}
