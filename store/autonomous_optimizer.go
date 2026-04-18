package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	AutonomousOptimizerStatusScheduled             = "scheduled"
	AutonomousOptimizerStatusRunning               = "running"
	AutonomousOptimizerStatusInsufficientEvidence  = "insufficient_evidence"
	AutonomousOptimizerStatusNoChange              = "no_change"
	AutonomousOptimizerStatusBacklogOnly           = "backlog_only"
	AutonomousOptimizerStatusBlockedByGate         = "blocked_by_gate"
	AutonomousOptimizerStatusDeferredForNextWindow = "deferred_for_next_window"
	AutonomousOptimizerStatusAutoApplied           = "auto_applied"
	AutonomousOptimizerStatusMonitoring            = "monitoring"
	AutonomousOptimizerStatusRollbackPending       = "rollback_pending"
	AutonomousOptimizerStatusRolledBack            = "rolled_back"
	AutonomousOptimizerStatusKept                  = "kept"
	AutonomousOptimizerStatusPaused                = "paused"
	AutonomousOptimizerStatusFailed                = "failed"

	AutonomousOptimizerRunTriggerManual    = "manual"
	AutonomousOptimizerRunTriggerBootstrap = "bootstrap"
	AutonomousOptimizerRunTriggerSchedule  = "schedule"

	AutonomousOptimizerBacklogStatusNew        = "new"
	AutonomousOptimizerBacklogStatusConfirmed  = "confirmed"
	AutonomousOptimizerBacklogStatusPlanned    = "planned"
	AutonomousOptimizerBacklogStatusInProgress = "in_progress"
	AutonomousOptimizerBacklogStatusDone       = "done"
	AutonomousOptimizerBacklogStatusRejected   = "rejected"

	AutonomousOptimizerDefaultReviewIntervalHours   = 12
	AutonomousOptimizerDefaultCooldownHours         = 12
	AutonomousOptimizerDefaultMaxConsecutiveApplies = 2
	AutonomousOptimizerDefaultModelName             = "gpt-5.4"
)

type AutonomousOptimizerStore struct {
	db *gorm.DB
}

type AutonomousOptimizerConfig struct {
	ID                         string    `gorm:"primaryKey" json:"id"`
	UserID                     string    `gorm:"column:user_id;not null;index:idx_autonomous_optimizer_configs_user_trader,unique" json:"user_id"`
	TraderID                   string    `gorm:"column:trader_id;not null;index:idx_autonomous_optimizer_configs_user_trader,unique" json:"trader_id"`
	Enabled                    bool      `gorm:"column:enabled;default:false" json:"enabled"`
	Status                     string    `gorm:"column:status;default:paused;index:idx_autonomous_optimizer_configs_status" json:"status"`
	ReviewIntervalHours        int       `gorm:"column:review_interval_hours;default:12" json:"review_interval_hours"`
	AutoApplyCooldownHours     int       `gorm:"column:auto_apply_cooldown_hours;default:12" json:"auto_apply_cooldown_hours"`
	MaxConsecutiveAutoApplies  int       `gorm:"column:max_consecutive_auto_applies;default:2" json:"max_consecutive_auto_applies"`
	AutoApplyConfigPatch       bool      `gorm:"column:auto_apply_config_patch;default:true" json:"auto_apply_config_patch"`
	AutoApplyPromptPatch       bool      `gorm:"column:auto_apply_prompt_patch;default:true" json:"auto_apply_prompt_patch"`
	AutoRollbackEnabled        bool      `gorm:"column:auto_rollback_enabled;default:true" json:"auto_rollback_enabled"`
	SelfPauseEnabled           bool      `gorm:"column:self_pause_enabled;default:true" json:"self_pause_enabled"`
	ProposalPromptInstructions string    `gorm:"column:proposal_prompt_instructions;type:text;default:''" json:"proposal_prompt_instructions"`
	CriticPromptInstructions   string    `gorm:"column:critic_prompt_instructions;type:text;default:''" json:"critic_prompt_instructions"`
	PrimaryModelConfigID       string    `gorm:"column:primary_model_config_id;default:''" json:"primary_model_config_id"`
	PrimaryModelName           string    `gorm:"column:primary_model_name;default:'gpt-5.4'" json:"primary_model_name"`
	CriticModelConfigID        string    `gorm:"column:critic_model_config_id;default:''" json:"critic_model_config_id"`
	CriticModelName            string    `gorm:"column:critic_model_name;default:'gpt-5.4'" json:"critic_model_name"`
	SeedSourceTraderID         string    `gorm:"column:seed_source_trader_id;default:''" json:"seed_source_trader_id"`
	SeedSourceStrategyID       string    `gorm:"column:seed_source_strategy_id;default:''" json:"seed_source_strategy_id"`
	BaselineStrategyID         string    `gorm:"column:baseline_strategy_id;default:''" json:"baseline_strategy_id"`
	BaselineStrategyJSON       string    `gorm:"column:baseline_strategy_json;type:text;default:'{}'" json:"-"`
	BaselineTraderJSON         string    `gorm:"column:baseline_trader_json;type:text;default:'{}'" json:"-"`
	CurrentSeedStrategyID      string    `gorm:"column:current_seed_strategy_id;default:''" json:"current_seed_strategy_id"`
	LastRunID                  string    `gorm:"column:last_run_id;default:''" json:"last_run_id"`
	LastRunAt                  time.Time `gorm:"column:last_run_at" json:"last_run_at"`
	NextRunAt                  time.Time `gorm:"column:next_run_at;index:idx_autonomous_optimizer_configs_next_run" json:"next_run_at"`
	SeededAt                   time.Time `gorm:"column:seeded_at" json:"seeded_at"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

func (AutonomousOptimizerConfig) TableName() string { return "autonomous_optimizer_configs" }

type AutonomousOptimizerRun struct {
	ID                       string    `gorm:"primaryKey" json:"id"`
	UserID                   string    `gorm:"column:user_id;not null;index:idx_autonomous_optimizer_runs_user_trader_time" json:"user_id"`
	TraderID                 string    `gorm:"column:trader_id;not null;index:idx_autonomous_optimizer_runs_user_trader_time" json:"trader_id"`
	ConfigID                 string    `gorm:"column:config_id;default:'';index:idx_autonomous_optimizer_runs_config" json:"config_id"`
	Trigger                  string    `gorm:"column:trigger;default:manual" json:"trigger"`
	Status                   string    `gorm:"column:status;default:scheduled;index:idx_autonomous_optimizer_runs_status" json:"status"`
	Summary                  string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	PrimaryModelConfigID     string    `gorm:"column:primary_model_config_id;default:''" json:"primary_model_config_id"`
	PrimaryModelName         string    `gorm:"column:primary_model_name;default:'gpt-5.4'" json:"primary_model_name"`
	CriticModelConfigID      string    `gorm:"column:critic_model_config_id;default:''" json:"critic_model_config_id"`
	CriticModelName          string    `gorm:"column:critic_model_name;default:'gpt-5.4'" json:"critic_model_name"`
	SourceTraderID           string    `gorm:"column:source_trader_id;default:''" json:"source_trader_id"`
	SourceStrategyID         string    `gorm:"column:source_strategy_id;default:''" json:"source_strategy_id"`
	AppliedStrategyVersionID string    `gorm:"column:applied_strategy_version_id;default:''" json:"applied_strategy_version_id"`
	ConfigPatchJSON          string    `gorm:"column:config_patch_json;type:text;default:'{}'" json:"-"`
	PromptPatchJSON          string    `gorm:"column:prompt_patch_json;type:text;default:'{}'" json:"-"`
	ValidationJSON           string    `gorm:"column:validation_json;type:text;default:'{}'" json:"-"`
	MetadataJSON             string    `gorm:"column:metadata_json;type:text;default:'{}'" json:"-"`
	StartedAt                time.Time `gorm:"column:started_at;index:idx_autonomous_optimizer_runs_started" json:"started_at"`
	CompletedAt              time.Time `gorm:"column:completed_at" json:"completed_at"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

func (AutonomousOptimizerRun) TableName() string { return "autonomous_optimizer_runs" }

type AutonomousOptimizerBacklogItem struct {
	ID                 string    `gorm:"primaryKey" json:"id"`
	UserID             string    `gorm:"column:user_id;not null;index:idx_autonomous_optimizer_backlog_user_trader" json:"user_id"`
	TraderID           string    `gorm:"column:trader_id;not null;index:idx_autonomous_optimizer_backlog_user_trader" json:"trader_id"`
	RunID              string    `gorm:"column:run_id;default:'';index:idx_autonomous_optimizer_backlog_run" json:"run_id"`
	Title              string    `gorm:"column:title;not null" json:"title"`
	Category           string    `gorm:"column:category;default:'';index:idx_autonomous_optimizer_backlog_category" json:"category"`
	Description        string    `gorm:"column:description;type:text;default:''" json:"description"`
	ExpectedImpact     string    `gorm:"column:expected_impact;type:text;default:''" json:"expected_impact"`
	Confidence         float64   `gorm:"column:confidence;default:0" json:"confidence"`
	ImplementationCost float64   `gorm:"column:implementation_cost;default:0" json:"implementation_cost"`
	Urgency            float64   `gorm:"column:urgency;default:0" json:"urgency"`
	RecurrenceCount    int       `gorm:"column:recurrence_count;default:1" json:"recurrence_count"`
	CompositeScore     float64   `gorm:"column:composite_score;default:0;index:idx_autonomous_optimizer_backlog_score" json:"composite_score"`
	Status             string    `gorm:"column:status;default:new;index:idx_autonomous_optimizer_backlog_status" json:"status"`
	AIGenerated        bool      `gorm:"column:ai_generated;default:true" json:"ai_generated"`
	UserEdited         bool      `gorm:"column:user_edited;default:false" json:"user_edited"`
	MergedFindingCount int       `gorm:"column:merged_finding_count;default:1" json:"merged_finding_count"`
	EvidenceJSON       string    `gorm:"column:evidence_json;type:text;default:'[]'" json:"-"`
	MetadataJSON       string    `gorm:"column:metadata_json;type:text;default:'{}'" json:"-"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (AutonomousOptimizerBacklogItem) TableName() string { return "autonomous_optimizer_backlog_items" }

func NewAutonomousOptimizerStore(db *gorm.DB) *AutonomousOptimizerStore {
	return &AutonomousOptimizerStore{db: db}
}

func (s *AutonomousOptimizerStore) initTables() error {
	return s.db.AutoMigrate(&AutonomousOptimizerConfig{}, &AutonomousOptimizerRun{}, &AutonomousOptimizerBacklogItem{})
}

func (s *AutonomousOptimizerStore) GetConfig(userID, traderID string) (*AutonomousOptimizerConfig, error) {
	var cfg AutonomousOptimizerConfig
	if err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *AutonomousOptimizerStore) GetOrCreateConfig(userID, traderID string) (*AutonomousOptimizerConfig, error) {
	cfg, err := s.GetConfig(userID, traderID)
	if err == nil {
		normalizedCooldown := normalizeAutonomousOptimizerCooldownHours(cfg.AutoApplyCooldownHours)
		normalizedMaxApplies := normalizeAutonomousOptimizerMaxConsecutiveApplies(cfg.MaxConsecutiveAutoApplies)
		expectedNextRunAt := cfg.NextRunAt
		if cfg.Enabled && cfg.Status != AutonomousOptimizerStatusPaused && expectedNextRunAt.IsZero() {
			expectedNextRunAt = nextAutonomousOptimizerRunAt(cfg.LastRunAt, cfg.ReviewIntervalHours)
		}
		if cfg.AutoApplyCooldownHours != normalizedCooldown || cfg.MaxConsecutiveAutoApplies != normalizedMaxApplies || !cfg.NextRunAt.Equal(expectedNextRunAt) {
			cfg.AutoApplyCooldownHours = normalizedCooldown
			cfg.MaxConsecutiveAutoApplies = normalizedMaxApplies
			cfg.NextRunAt = expectedNextRunAt
			if saveErr := s.db.Save(cfg).Error; saveErr != nil {
				return nil, saveErr
			}
		}
		return cfg, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	now := time.Now().UTC()
	cfg = &AutonomousOptimizerConfig{
		ID:                        uuid.NewString(),
		UserID:                    userID,
		TraderID:                  traderID,
		Enabled:                   false,
		Status:                    AutonomousOptimizerStatusPaused,
		ReviewIntervalHours:       AutonomousOptimizerDefaultReviewIntervalHours,
		AutoApplyCooldownHours:    AutonomousOptimizerDefaultCooldownHours,
		MaxConsecutiveAutoApplies: AutonomousOptimizerDefaultMaxConsecutiveApplies,
		AutoApplyConfigPatch:      true,
		AutoApplyPromptPatch:      true,
		AutoRollbackEnabled:       true,
		SelfPauseEnabled:          true,
		PrimaryModelName:          AutonomousOptimizerDefaultModelName,
		CriticModelName:           AutonomousOptimizerDefaultModelName,
		BaselineStrategyJSON:      "{}",
		BaselineTraderJSON:        "{}",
		NextRunAt:                 now.Add(AutonomousOptimizerDefaultReviewIntervalHours * time.Hour),
	}
	if err := s.db.Create(cfg).Error; err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *AutonomousOptimizerStore) SaveConfig(cfg *AutonomousOptimizerConfig) error {
	if cfg == nil {
		return fmt.Errorf("autonomous optimizer config cannot be nil")
	}
	cfg.ReviewIntervalHours = normalizeAutonomousOptimizerInterval(cfg.ReviewIntervalHours)
	cfg.AutoApplyCooldownHours = normalizeAutonomousOptimizerCooldownHours(cfg.AutoApplyCooldownHours)
	cfg.MaxConsecutiveAutoApplies = normalizeAutonomousOptimizerMaxConsecutiveApplies(cfg.MaxConsecutiveAutoApplies)
	cfg.Status = normalizeAutonomousOptimizerStatus(cfg.Status)
	cfg.PrimaryModelName = normalizeAutonomousOptimizerModelName(cfg.PrimaryModelName)
	cfg.CriticModelName = normalizeAutonomousOptimizerModelName(cfg.CriticModelName)
	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}
	if cfg.Enabled && cfg.Status != AutonomousOptimizerStatusPaused && cfg.NextRunAt.IsZero() {
		cfg.NextRunAt = nextAutonomousOptimizerRunAt(cfg.LastRunAt, cfg.ReviewIntervalHours)
	}
	return s.db.Save(cfg).Error
}

func (s *AutonomousOptimizerStore) SaveRun(run *AutonomousOptimizerRun) error {
	if run == nil {
		return fmt.Errorf("autonomous optimizer run cannot be nil")
	}
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	run.Status = normalizeAutonomousOptimizerStatus(run.Status)
	run.Trigger = normalizeAutonomousOptimizerTrigger(run.Trigger)
	run.PrimaryModelName = normalizeAutonomousOptimizerModelName(run.PrimaryModelName)
	run.CriticModelName = normalizeAutonomousOptimizerModelName(run.CriticModelName)
	return s.db.Save(run).Error
}

func (s *AutonomousOptimizerStore) ListDueConfigs(now time.Time, limit int) ([]AutonomousOptimizerConfig, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var items []AutonomousOptimizerConfig
	err := s.db.
		Where("enabled = ? AND status IN ? AND next_run_at <= ?", true, []string{
			AutonomousOptimizerStatusScheduled,
			AutonomousOptimizerStatusNoChange,
			AutonomousOptimizerStatusInsufficientEvidence,
			AutonomousOptimizerStatusBacklogOnly,
			AutonomousOptimizerStatusBlockedByGate,
			AutonomousOptimizerStatusDeferredForNextWindow,
			AutonomousOptimizerStatusMonitoring,
			AutonomousOptimizerStatusRollbackPending,
			AutonomousOptimizerStatusKept,
			AutonomousOptimizerStatusRolledBack,
			AutonomousOptimizerStatusFailed,
		}, now).
		Order("next_run_at ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (s *AutonomousOptimizerStore) ListRuns(userID, traderID string, limit int) ([]*AutonomousOptimizerRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var items []*AutonomousOptimizerRun
	err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).
		Order("created_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (s *AutonomousOptimizerStore) GetRun(userID, traderID, runID string) (*AutonomousOptimizerRun, error) {
	var item AutonomousOptimizerRun
	if err := s.db.Where("user_id = ? AND trader_id = ? AND id = ?", userID, traderID, runID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AutonomousOptimizerStore) ListStaleRunningRuns(cutoff time.Time, limit int) ([]*AutonomousOptimizerRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var items []*AutonomousOptimizerRun
	err := s.db.
		Where("status = ? AND started_at <= ?", AutonomousOptimizerStatusRunning, cutoff).
		Order("started_at ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (s *AutonomousOptimizerStore) ListConfigsByStatus(status string, limit int) ([]AutonomousOptimizerConfig, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var items []AutonomousOptimizerConfig
	err := s.db.
		Where("status = ?", normalizeAutonomousOptimizerStatus(status)).
		Order("updated_at ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (s *AutonomousOptimizerStore) GetLatestRunByStatus(userID, traderID, status string) (*AutonomousOptimizerRun, error) {
	var item AutonomousOptimizerRun
	if err := s.db.
		Where("user_id = ? AND trader_id = ? AND status = ?", userID, traderID, normalizeAutonomousOptimizerStatus(status)).
		Order("started_at DESC, created_at DESC").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AutonomousOptimizerStore) GetBacklogItem(userID, traderID, itemID string) (*AutonomousOptimizerBacklogItem, error) {
	var item AutonomousOptimizerBacklogItem
	if err := s.db.Where("user_id = ? AND trader_id = ? AND id = ?", userID, traderID, itemID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AutonomousOptimizerStore) SaveBacklogItem(item *AutonomousOptimizerBacklogItem) error {
	if item == nil {
		return fmt.Errorf("autonomous optimizer backlog item cannot be nil")
	}
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.RecurrenceCount <= 0 {
		item.RecurrenceCount = 1
	}
	if item.MergedFindingCount <= 0 {
		item.MergedFindingCount = 1
	}
	item.Status = normalizeAutonomousOptimizerBacklogStatus(item.Status)
	return s.db.Save(item).Error
}

func (s *AutonomousOptimizerStore) ListBacklog(userID, traderID string, limit int) ([]*AutonomousOptimizerBacklogItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var items []*AutonomousOptimizerBacklogItem
	err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).
		Order("composite_score DESC, updated_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func normalizeAutonomousOptimizerInterval(hours int) int {
	if hours <= 0 {
		return AutonomousOptimizerDefaultReviewIntervalHours
	}
	if hours > 168 {
		return 168
	}
	return hours
}

func normalizeAutonomousOptimizerCooldownHours(hours int) int {
	if hours <= 0 {
		return AutonomousOptimizerDefaultCooldownHours
	}
	if hours > 168 {
		return 168
	}
	return hours
}

func normalizeAutonomousOptimizerMaxConsecutiveApplies(value int) int {
	if value <= 0 {
		return AutonomousOptimizerDefaultMaxConsecutiveApplies
	}
	if value > 10 {
		return 10
	}
	return value
}

func normalizeAutonomousOptimizerStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AutonomousOptimizerStatusScheduled,
		AutonomousOptimizerStatusRunning,
		AutonomousOptimizerStatusInsufficientEvidence,
		AutonomousOptimizerStatusNoChange,
		AutonomousOptimizerStatusBacklogOnly,
		AutonomousOptimizerStatusBlockedByGate,
		AutonomousOptimizerStatusDeferredForNextWindow,
		AutonomousOptimizerStatusAutoApplied,
		AutonomousOptimizerStatusMonitoring,
		AutonomousOptimizerStatusRollbackPending,
		AutonomousOptimizerStatusRolledBack,
		AutonomousOptimizerStatusKept,
		AutonomousOptimizerStatusPaused,
		AutonomousOptimizerStatusFailed:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return AutonomousOptimizerStatusPaused
	}
}

func normalizeAutonomousOptimizerTrigger(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AutonomousOptimizerRunTriggerBootstrap,
		AutonomousOptimizerRunTriggerSchedule:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return AutonomousOptimizerRunTriggerManual
	}
}

func normalizeAutonomousOptimizerBacklogStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AutonomousOptimizerBacklogStatusConfirmed,
		AutonomousOptimizerBacklogStatusPlanned,
		AutonomousOptimizerBacklogStatusInProgress,
		AutonomousOptimizerBacklogStatusDone,
		AutonomousOptimizerBacklogStatusRejected:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return AutonomousOptimizerBacklogStatusNew
	}
}

func normalizeAutonomousOptimizerModelName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return AutonomousOptimizerDefaultModelName
	}
	return value
}

func nextAutonomousOptimizerRunAt(base time.Time, intervalHours int) time.Time {
	base = base.UTC()
	if base.IsZero() {
		base = time.Now().UTC()
	}
	return base.Add(time.Duration(normalizeAutonomousOptimizerInterval(intervalHours)) * time.Hour)
}
