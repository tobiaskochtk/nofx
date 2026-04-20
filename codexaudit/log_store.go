package codexaudit

import (
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

const Retention = 100

// LogEntry records one real Codex CLI invocation at the provider boundary.
type LogEntry struct {
	ID                 int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt          time.Time  `gorm:"autoCreateTime" json:"created_at"`
	RequestStartedAt   time.Time  `gorm:"column:request_started_at;not null;index:idx_codex_call_logs_started_at" json:"request_started_at"`
	ResponseFinishedAt *time.Time `gorm:"column:response_finished_at" json:"response_finished_at,omitempty"`
	CallerType         string     `gorm:"column:caller_type;default:'';index:idx_codex_call_logs_caller_type" json:"caller_type"`
	CallerID           string     `gorm:"column:caller_id;default:'';index:idx_codex_call_logs_caller_id" json:"caller_id"`
	CallerName         string     `gorm:"column:caller_name;default:''" json:"caller_name"`
	UserID             string     `gorm:"column:user_id;default:'';index:idx_codex_call_logs_user_id" json:"user_id"`
	TraderID           string     `gorm:"column:trader_id;default:'';index:idx_codex_call_logs_trader_id" json:"trader_id"`
	TraderName         string     `gorm:"column:trader_name;default:''" json:"trader_name"`
	Component          string     `gorm:"column:component;default:''" json:"component"`
	CycleNumber        int        `gorm:"column:cycle_number;default:0" json:"cycle_number"`
	Provider           string     `gorm:"column:provider;not null;index:idx_codex_call_logs_provider_model" json:"provider"`
	Model              string     `gorm:"column:model;not null;index:idx_codex_call_logs_provider_model" json:"model"`
	Transport          string     `gorm:"column:transport;not null;default:'codex_cli'" json:"transport"`
	CodexBinary        string     `gorm:"column:codex_binary;default:''" json:"codex_binary"`
	CommandArgsJSON    string     `gorm:"column:command_args_json;type:text;default:''" json:"command_args_json"`
	RequestPrompt      string     `gorm:"column:request_prompt;type:text;default:''" json:"request_prompt"`
	ResponseText       string     `gorm:"column:response_text;type:text;default:''" json:"response_text"`
	CombinedOutput     string     `gorm:"column:combined_output;type:text;default:''" json:"combined_output"`
	Success            bool       `gorm:"column:success;not null;default:false" json:"success"`
	ExitCode           int        `gorm:"column:exit_code;default:0" json:"exit_code"`
	DurationMs         int64      `gorm:"column:duration_ms;default:0" json:"duration_ms"`
	ErrorMessage       string     `gorm:"column:error_message;type:text;default:''" json:"error_message"`
}

func (LogEntry) TableName() string { return "codex_call_logs" }

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) InitTables() error {
	return s.db.AutoMigrate(&LogEntry{})
}

func (s *Store) Record(entry *LogEntry) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("codex audit store is not initialized")
	}
	if entry == nil {
		return fmt.Errorf("codex audit entry cannot be nil")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entry).Error; err != nil {
			return err
		}

		var staleIDs []int64
		if err := tx.Model(&LogEntry{}).
			Order("created_at DESC, id DESC").
			Offset(Retention).
			Pluck("id", &staleIDs).Error; err != nil {
			return err
		}
		if len(staleIDs) == 0 {
			return nil
		}
		return tx.Where("id IN ?", staleIDs).Delete(&LogEntry{}).Error
	})
}

func (s *Store) ListRecentForUser(userID, traderID string, limit int) ([]LogEntry, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("codex audit store is not initialized")
	}
	if limit <= 0 || limit > Retention {
		limit = Retention
	}

	query := s.db.Model(&LogEntry{}).Order("created_at DESC, id DESC").Limit(limit)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if traderID != "" {
		query = query.Where("trader_id = ?", traderID)
	}

	var items []LogEntry
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

var (
	globalDB       *gorm.DB
	globalDBMu     sync.RWMutex
	globalInitOnce sync.Once
	globalInitErr  error
)

func SetDB(db *gorm.DB) {
	globalDBMu.Lock()
	defer globalDBMu.Unlock()
	globalDB = db
}

func globalStore() (*Store, error) {
	globalDBMu.RLock()
	db := globalDB
	globalDBMu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("codex audit database is not initialized")
	}

	store := NewStore(db)
	globalInitOnce.Do(func() {
		globalInitErr = store.InitTables()
	})
	if globalInitErr != nil {
		return nil, globalInitErr
	}
	return store, nil
}

func RecordGlobal(entry *LogEntry) error {
	store, err := globalStore()
	if err != nil {
		return err
	}
	return store.Record(entry)
}
