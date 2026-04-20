package store

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DealReviewLearnedPatternScopeGlobal      = "global"
	DealReviewLearnedPatternScopeTraderLocal = "trader_local"
	DealReviewLearnedPatternScopeRegimeLocal = "regime_local"
	DealReviewLearnedPatternScopeSymbol      = "symbol"

	DealReviewLearnedPatternClassPositiveEdge = "positive_edge"
	DealReviewLearnedPatternClassNegativeEdge = "negative_edge"

	DealReviewLearnedPatternValidationLabelConfirmed            = "confirmed"
	DealReviewLearnedPatternValidationLabelCandidate            = "candidate"
	DealReviewLearnedPatternValidationLabelInsufficientEvidence = "insufficient_evidence"
	DealReviewLearnedPatternValidationLabelFalsePositive        = "false_positive"
	DealReviewLearnedPatternValidationLabelReverseRisk          = "reverse_risk"
	DealReviewLearnedPatternValidationLabelDrifting             = "drifting"
	DealReviewLearnedPatternValidationLabelExpired              = "expired"

	DealReviewLearnedPatternRecommendedUseReviewHint     = "review_hint"
	DealReviewLearnedPatternRecommendedUsePromptHint     = "prompt_hint"
	DealReviewLearnedPatternRecommendedUseConfigCand     = "config_candidate"
	DealReviewLearnedPatternRecommendedUseMonitoringRule = "monitoring_rule"
	DealReviewLearnedPatternRecommendedUseMonitorOnly    = "monitor_only"
	DealReviewLearnedPatternRecommendedUseExpiredIgnore  = "expired_do_not_use"

	DealReviewPatternBacklogCategoryMissingFeature    = "missing_feature"
	DealReviewPatternBacklogCategoryValidationWindow  = "validation_window"
	DealReviewPatternBacklogCategoryDriftMonitoring   = "drift_monitoring"
	DealReviewPatternBacklogCategoryContradictionRisk = "contradiction_risk"

	dealReviewLearnedPatternFeatureLimit            = 16
	dealReviewLearnedPatternEvidenceLimit           = 6
	dealReviewLearnedPatternMaxOrder                = 3
	dealReviewLearnedPatternTripleFeatureLimit      = 7
	dealReviewLearnedPatternTripleComboLimit        = 12
	dealReviewLearnedPatternTripleMinSupportRate    = 0.72
	dealReviewLearnedPatternTripleMinConfidence     = 0.58
	dealReviewLearnedPatternTripleMinComposite      = 0.60
	dealReviewLearnedPatternTripleMinWindowScore    = 0.52
	dealReviewLearnedPatternRefreshInterval         = 30 * time.Minute
	dealReviewLearnedPatternObservedThreshold       = 3
	dealReviewLearnedPatternMinActionableLiftPnLPct = 0.45
)

type DealReviewPatternFeatureRecord struct {
	ID                 int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID             string    `gorm:"column:user_id;not null;index:idx_deal_review_pattern_features_user_trader" json:"user_id"`
	TraderID           string    `gorm:"column:trader_id;not null;index:idx_deal_review_pattern_features_user_trader;index:idx_deal_review_pattern_features_lookup" json:"trader_id"`
	CaseID             string    `gorm:"column:case_id;not null;uniqueIndex:idx_deal_review_pattern_features_case" json:"case_id"`
	PositionID         int64     `gorm:"column:position_id;default:0;index:idx_deal_review_pattern_features_lookup" json:"position_id"`
	Symbol             string    `gorm:"column:symbol;not null;index:idx_deal_review_pattern_features_lookup" json:"symbol"`
	Side               string    `gorm:"column:side;not null;index:idx_deal_review_pattern_features_lookup" json:"side"`
	Outcome            string    `gorm:"column:outcome;default:'';index:idx_deal_review_pattern_features_outcome" json:"outcome"`
	OpenConfidence     int       `gorm:"column:open_confidence;default:0" json:"open_confidence"`
	RealizedPnL        float64   `gorm:"column:realized_pnl;default:0" json:"realized_pnl"`
	RealizedPnLPct     float64   `gorm:"column:realized_pnl_pct;default:0" json:"realized_pnl_pct"`
	MaxMFEPct          float64   `gorm:"column:max_mfe_pct;default:0" json:"max_mfe_pct"`
	MaxMAEPct          float64   `gorm:"column:max_mae_pct;default:0" json:"max_mae_pct"`
	ProfitGivenBackPct float64   `gorm:"column:profit_given_back_pct;default:0" json:"profit_given_back_pct"`
	HoldDurationMs     int64     `gorm:"column:hold_duration_ms;default:0" json:"hold_duration_ms"`
	FeatureCount       int       `gorm:"column:feature_count;default:0" json:"feature_count"`
	FeaturesJSON       string    `gorm:"column:features_json;type:text;default:'[]'" json:"-"`
	QualityFlagsJSON   string    `gorm:"column:quality_flags_json;type:text;default:'[]'" json:"-"`
	ObservedAt         time.Time `gorm:"column:observed_at;index:idx_deal_review_pattern_features_observed" json:"observed_at"`
	BuiltAt            time.Time `gorm:"column:built_at;index:idx_deal_review_pattern_features_built" json:"built_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	Features           []string  `gorm:"-" json:"features,omitempty"`
	QualityFlags       []string  `gorm:"-" json:"quality_flags,omitempty"`
}

func (DealReviewPatternFeatureRecord) TableName() string { return "deal_review_pattern_features" }

type DealReviewLearnedPatternEvidence struct {
	CaseID         string  `json:"case_id"`
	PositionID     int64   `json:"position_id"`
	Symbol         string  `json:"symbol"`
	Side           string  `json:"side"`
	Outcome        string  `json:"outcome"`
	RealizedPnL    float64 `json:"realized_pnl"`
	RealizedPnLPct float64 `json:"realized_pnl_pct"`
	HoldDurationMs int64   `json:"hold_duration_ms"`
	ObservedAt     string  `json:"observed_at,omitempty"`
}

type DealReviewPatternEvidenceRecord struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         string    `gorm:"column:user_id;not null;index:idx_deal_review_pattern_evidence_user_trader" json:"user_id"`
	TraderID       string    `gorm:"column:trader_id;not null;index:idx_deal_review_pattern_evidence_user_trader;index:idx_deal_review_pattern_evidence_pattern" json:"trader_id"`
	PatternID      string    `gorm:"column:pattern_id;not null;index:idx_deal_review_pattern_evidence_pattern;index:idx_deal_review_pattern_evidence_case" json:"pattern_id"`
	EvidenceRank   int       `gorm:"column:evidence_rank;default:0" json:"evidence_rank"`
	CaseID         string    `gorm:"column:case_id;not null;index:idx_deal_review_pattern_evidence_case" json:"case_id"`
	PositionID     int64     `gorm:"column:position_id;default:0;index:idx_deal_review_pattern_evidence_case" json:"position_id"`
	Symbol         string    `gorm:"column:symbol;default:'';index:idx_deal_review_pattern_evidence_lookup" json:"symbol"`
	Side           string    `gorm:"column:side;default:'';index:idx_deal_review_pattern_evidence_lookup" json:"side"`
	Outcome        string    `gorm:"column:outcome;default:'';index:idx_deal_review_pattern_evidence_outcome" json:"outcome"`
	RealizedPnL    float64   `gorm:"column:realized_pnl;default:0" json:"realized_pnl"`
	RealizedPnLPct float64   `gorm:"column:realized_pnl_pct;default:0" json:"realized_pnl_pct"`
	HoldDurationMs int64     `gorm:"column:hold_duration_ms;default:0" json:"hold_duration_ms"`
	ObservedAt     time.Time `gorm:"column:observed_at;index:idx_deal_review_pattern_evidence_observed" json:"observed_at"`
	BuiltAt        time.Time `gorm:"column:built_at;index:idx_deal_review_pattern_evidence_built" json:"built_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (DealReviewPatternEvidenceRecord) TableName() string { return "deal_review_pattern_evidence" }

type DealReviewPatternValidationRun struct {
	ID                        string         `gorm:"primaryKey" json:"id"`
	UserID                    string         `gorm:"column:user_id;not null;index:idx_deal_review_pattern_validation_runs_user_trader" json:"user_id"`
	TraderID                  string         `gorm:"column:trader_id;not null;index:idx_deal_review_pattern_validation_runs_user_trader" json:"trader_id"`
	FeatureCount              int            `gorm:"column:feature_count;default:0" json:"feature_count"`
	PatternCount              int            `gorm:"column:pattern_count;default:0" json:"pattern_count"`
	EvidenceCount             int            `gorm:"column:evidence_count;default:0" json:"evidence_count"`
	BacklogCandidateCount     int            `gorm:"column:backlog_candidate_count;default:0" json:"backlog_candidate_count"`
	ConfirmedCount            int            `gorm:"column:confirmed_count;default:0" json:"confirmed_count"`
	CandidateCount            int            `gorm:"column:candidate_count;default:0" json:"candidate_count"`
	InsufficientEvidenceCount int            `gorm:"column:insufficient_evidence_count;default:0" json:"insufficient_evidence_count"`
	FalsePositiveCount        int            `gorm:"column:false_positive_count;default:0" json:"false_positive_count"`
	ReverseRiskCount          int            `gorm:"column:reverse_risk_count;default:0" json:"reverse_risk_count"`
	DriftingCount             int            `gorm:"column:drifting_count;default:0" json:"drifting_count"`
	ExpiredCount              int            `gorm:"column:expired_count;default:0" json:"expired_count"`
	Summary                   string         `gorm:"column:summary;type:text;default:''" json:"summary"`
	MetadataJSON              string         `gorm:"column:metadata_json;type:text;default:'{}'" json:"-"`
	StartedAt                 time.Time      `gorm:"column:started_at;index:idx_deal_review_pattern_validation_runs_started" json:"started_at"`
	CompletedAt               time.Time      `gorm:"column:completed_at;index:idx_deal_review_pattern_validation_runs_completed" json:"completed_at"`
	BuiltAt                   time.Time      `gorm:"column:built_at;index:idx_deal_review_pattern_validation_runs_built" json:"built_at"`
	CreatedAt                 time.Time      `json:"created_at"`
	UpdatedAt                 time.Time      `json:"updated_at"`
	Metadata                  map[string]any `gorm:"-" json:"metadata,omitempty"`
}

func (DealReviewPatternValidationRun) TableName() string {
	return "deal_review_pattern_validation_runs"
}

type DealReviewPatternBacklogCandidate struct {
	ID              string         `gorm:"primaryKey" json:"id"`
	UserID          string         `gorm:"column:user_id;not null;index:idx_deal_review_pattern_backlog_user_trader" json:"user_id"`
	TraderID        string         `gorm:"column:trader_id;not null;index:idx_deal_review_pattern_backlog_user_trader" json:"trader_id"`
	PatternID       string         `gorm:"column:pattern_id;default:'';index:idx_deal_review_pattern_backlog_pattern" json:"pattern_id,omitempty"`
	Category        string         `gorm:"column:category;default:'';index:idx_deal_review_pattern_backlog_category" json:"category"`
	Title           string         `gorm:"column:title;type:text;default:''" json:"title"`
	Rationale       string         `gorm:"column:rationale;type:text;default:''" json:"rationale"`
	PriorityScore   float64        `gorm:"column:priority_score;default:0;index:idx_deal_review_pattern_backlog_priority" json:"priority_score"`
	ConfidenceScore float64        `gorm:"column:confidence_score;default:0" json:"confidence_score"`
	SupportCount    int            `gorm:"column:support_count;default:0" json:"support_count"`
	MetadataJSON    string         `gorm:"column:metadata_json;type:text;default:'{}'" json:"-"`
	BuiltAt         time.Time      `gorm:"column:built_at;index:idx_deal_review_pattern_backlog_built" json:"built_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Metadata        map[string]any `gorm:"-" json:"metadata,omitempty"`
}

func (DealReviewPatternBacklogCandidate) TableName() string {
	return "deal_review_pattern_backlog_candidates"
}

type DealReviewLearnedPattern struct {
	ID                        string                                              `gorm:"primaryKey" json:"id"`
	StableKey                 string                                              `gorm:"column:stable_key;default:'';index:idx_deal_review_learned_patterns_stable" json:"stable_key,omitempty"`
	UserID                    string                                              `gorm:"column:user_id;not null;index:idx_deal_review_learned_patterns_user_trader" json:"user_id"`
	TraderID                  string                                              `gorm:"column:trader_id;not null;index:idx_deal_review_learned_patterns_user_trader;index:idx_deal_review_learned_patterns_lookup" json:"trader_id"`
	ScopeType                 string                                              `gorm:"column:scope_type;default:trader_local;index:idx_deal_review_learned_patterns_scope" json:"scope_type"`
	ScopeKey                  string                                              `gorm:"column:scope_key;default:'';index:idx_deal_review_learned_patterns_scope" json:"scope_key"`
	Symbol                    string                                              `gorm:"column:symbol;default:'';index:idx_deal_review_learned_patterns_lookup" json:"symbol"`
	Side                      string                                              `gorm:"column:side;not null;index:idx_deal_review_learned_patterns_lookup" json:"side"`
	PatternClass              string                                              `gorm:"column:pattern_class;default:'';index:idx_deal_review_learned_patterns_class" json:"pattern_class"`
	Status                    string                                              `gorm:"column:status;default:observed;index:idx_deal_review_learned_patterns_status" json:"status"`
	ValidationLabel           string                                              `gorm:"column:validation_label;default:'';index:idx_deal_review_learned_patterns_validation" json:"validation_label"`
	RecommendedUse            string                                              `gorm:"column:recommended_use;default:review_hint" json:"recommended_use"`
	PatternSignature          string                                              `gorm:"column:pattern_signature;type:text;default:'';index:idx_deal_review_learned_patterns_signature" json:"pattern_signature"`
	RegimeSignature           string                                              `gorm:"column:regime_signature;type:text;default:''" json:"regime_signature"`
	PatternOrder              int                                                 `gorm:"column:pattern_order;default:1" json:"pattern_order"`
	FeatureCount              int                                                 `gorm:"column:feature_count;default:0" json:"feature_count"`
	FeatureSetJSON            string                                              `gorm:"column:feature_set_json;type:text;default:'[]'" json:"-"`
	SampleCount               int                                                 `gorm:"column:sample_count;default:0" json:"sample_count"`
	WinningDeals              int                                                 `gorm:"column:winning_deals;default:0" json:"winning_deals"`
	LosingDeals               int                                                 `gorm:"column:losing_deals;default:0" json:"losing_deals"`
	FlatDeals                 int                                                 `gorm:"column:flat_deals;default:0" json:"flat_deals"`
	SupportCount              int                                                 `gorm:"column:support_count;default:0" json:"support_count"`
	ContradictCount           int                                                 `gorm:"column:contradict_count;default:0" json:"contradict_count"`
	WinRate                   float64                                             `gorm:"column:win_rate;default:0" json:"win_rate"`
	LossRate                  float64                                             `gorm:"column:loss_rate;default:0" json:"loss_rate"`
	NetPnL                    float64                                             `gorm:"column:net_pnl;default:0" json:"net_pnl"`
	AvgPnL                    float64                                             `gorm:"column:avg_pnl;default:0" json:"avg_pnl"`
	AvgPnLPct                 float64                                             `gorm:"column:avg_pnl_pct;default:0" json:"avg_pnl_pct"`
	Expectancy                float64                                             `gorm:"column:expectancy;default:0" json:"expectancy"`
	AvgMFEPct                 float64                                             `gorm:"column:avg_mfe_pct;default:0" json:"avg_mfe_pct"`
	AvgMAEPct                 float64                                             `gorm:"column:avg_mae_pct;default:0" json:"avg_mae_pct"`
	GiveBackRate              float64                                             `gorm:"column:give_back_rate;default:0" json:"give_back_rate"`
	AvgGiveBackPct            float64                                             `gorm:"column:avg_give_back_pct;default:0" json:"avg_give_back_pct"`
	AvgHoldMs                 int64                                               `gorm:"column:avg_hold_ms;default:0" json:"avg_hold_ms"`
	BaselineWinRate           float64                                             `gorm:"column:baseline_win_rate;default:0" json:"baseline_win_rate"`
	BaselineAvgPnLPct         float64                                             `gorm:"column:baseline_avg_pnl_pct;default:0" json:"baseline_avg_pnl_pct"`
	LiftWinRate               float64                                             `gorm:"column:lift_win_rate;default:0" json:"lift_win_rate"`
	LiftAvgPnLPct             float64                                             `gorm:"column:lift_avg_pnl_pct;default:0" json:"lift_avg_pnl_pct"`
	TrainingSampleCount       int                                                 `gorm:"column:training_sample_count;default:0" json:"training_sample_count"`
	ValidationSampleCount     int                                                 `gorm:"column:validation_sample_count;default:0" json:"validation_sample_count"`
	ValidationSupportCount    int                                                 `gorm:"column:validation_support_count;default:0" json:"validation_support_count"`
	ValidationContradictCount int                                                 `gorm:"column:validation_contradict_count;default:0" json:"validation_contradict_count"`
	ValidationAvgPnLPct       float64                                             `gorm:"column:validation_avg_pnl_pct;default:0" json:"validation_avg_pnl_pct"`
	ValidationSupportScore    float64                                             `gorm:"column:validation_support_score;default:0" json:"validation_support_score"`
	RecentSampleCount         int                                                 `gorm:"column:recent_sample_count;default:0" json:"recent_sample_count"`
	RecentSupportCount        int                                                 `gorm:"column:recent_support_count;default:0" json:"recent_support_count"`
	RecentContradictCount     int                                                 `gorm:"column:recent_contradict_count;default:0" json:"recent_contradict_count"`
	RecentAvgPnLPct           float64                                             `gorm:"column:recent_avg_pnl_pct;default:0" json:"recent_avg_pnl_pct"`
	RecentSupportScore        float64                                             `gorm:"column:recent_support_score;default:0" json:"recent_support_score"`
	ConfidenceScore           float64                                             `gorm:"column:confidence_score;default:0" json:"confidence_score"`
	StabilityScore            float64                                             `gorm:"column:stability_score;default:0" json:"stability_score"`
	DriftScore                float64                                             `gorm:"column:drift_score;default:0" json:"drift_score"`
	RecencyWeight             float64                                             `gorm:"column:recency_weight;default:0" json:"recency_weight"`
	CompositeScore            float64                                             `gorm:"column:composite_score;default:0;index:idx_deal_review_learned_patterns_score" json:"composite_score"`
	FalsePositiveScore        float64                                             `gorm:"column:false_positive_score;default:0" json:"false_positive_score"`
	ReverseRiskScore          float64                                             `gorm:"column:reverse_risk_score;default:0" json:"reverse_risk_score"`
	Summary                   string                                              `gorm:"column:summary;type:text;default:''" json:"summary"`
	ValidationAlert           string                                              `gorm:"column:validation_alert;type:text;default:''" json:"validation_alert"`
	EvidenceJSON              string                                              `gorm:"column:evidence_json;type:text;default:'[]'" json:"-"`
	FirstObservedAt           time.Time                                           `gorm:"column:first_observed_at" json:"first_observed_at"`
	LastObservedAt            time.Time                                           `gorm:"column:last_observed_at;index:idx_deal_review_learned_patterns_observed" json:"last_observed_at"`
	BuiltAt                   time.Time                                           `gorm:"column:built_at;index:idx_deal_review_learned_patterns_built" json:"built_at"`
	CreatedAt                 time.Time                                           `json:"created_at"`
	UpdatedAt                 time.Time                                           `json:"updated_at"`
	BaseRecommendedUse        string                                              `gorm:"-" json:"base_recommended_use,omitempty"`
	FeatureSet                []string                                            `gorm:"-" json:"feature_set,omitempty"`
	Evidence                  []DealReviewLearnedPatternEvidence                  `gorm:"-" json:"evidence,omitempty"`
	MatchScore                float64                                             `gorm:"-" json:"match_score,omitempty"`
	Lifecycle                 *DealReviewLearnedPatternLifecycle                  `gorm:"-" json:"lifecycle,omitempty"`
	LifecycleHistory          []DealReviewLearnedPatternLifecycleSnapshot         `gorm:"-" json:"lifecycle_history,omitempty"`
	LifecycleTrend            *DealReviewLearnedPatternLifecycleTrend             `gorm:"-" json:"lifecycle_trend,omitempty"`
	LiveGuardAttribution      *DealReviewLearnedPatternLiveGuardAttributionRollup `gorm:"-" json:"live_guard_attribution,omitempty"`
	LiveGuardAttributionDelta *DealReviewLearnedPatternLiveGuardAttributionDelta  `gorm:"-" json:"live_guard_attribution_delta,omitempty"`
	ActionHint                *DealReviewLearnedPatternActionHint                 `gorm:"-" json:"action_hint,omitempty"`
	LiveActionHint            *DealReviewLearnedPatternLiveActionHint             `gorm:"-" json:"live_action_hint,omitempty"`
	ManualControl             *DealReviewLearnedPatternManualControl              `gorm:"-" json:"manual_control,omitempty"`
	ManualControlHistory      []DealReviewLearnedPatternManualControlEvent        `gorm:"-" json:"manual_control_history,omitempty"`
	InterventionHistory       []DealReviewLearnedPatternIntervention              `gorm:"-" json:"intervention_history,omitempty"`
}

func (DealReviewLearnedPattern) TableName() string { return "deal_review_learned_patterns" }

type DealReviewLearnedPatternFilter struct {
	PatternID                 string
	StableKey                 string
	Symbol                    string
	Side                      string
	ScopeType                 string
	PatternClass              string
	ValidationLabel           string
	Feature                   string
	Regime                    string
	LiveActionKind            string
	InterventionState         string
	DirectLiveActionCandidate bool
	MinConfidence             float64
	MinDrift                  float64
	MinSampleCount            int
	Limit                     int
}

type DealReviewLearnedPatternRefreshResult struct {
	Rebuilt      bool      `json:"rebuilt"`
	FeatureCount int       `json:"feature_count"`
	PatternCount int       `json:"pattern_count"`
	GeneratedAt  time.Time `json:"generated_at"`
}

type DealReviewLearnedPatternReportingSummary struct {
	TotalCount                      int                        `json:"total_count"`
	PositiveCount                   int                        `json:"positive_count"`
	NegativeCount                   int                        `json:"negative_count"`
	ConfirmedCount                  int                        `json:"confirmed_count"`
	CandidateCount                  int                        `json:"candidate_count"`
	FalsePositiveCount              int                        `json:"false_positive_count"`
	ReverseRiskCount                int                        `json:"reverse_risk_count"`
	DriftingCount                   int                        `json:"drifting_count"`
	ExpiredCount                    int                        `json:"expired_count"`
	MonitoringRuleCount             int                        `json:"monitoring_rule_count"`
	LifecycleActiveCount            int                        `json:"lifecycle_active_count"`
	LifecycleDegradingCount         int                        `json:"lifecycle_degrading_count"`
	LifecycleRollbackWatchCount     int                        `json:"lifecycle_rollback_watch_count"`
	LifecycleExpiredCount           int                        `json:"lifecycle_expired_count"`
	LifecycleFragileCount           int                        `json:"lifecycle_fragile_count"`
	LifecycleLaggingGuardCount      int                        `json:"lifecycle_lagging_guard_count"`
	LiveGuardProtectiveCount        int                        `json:"live_guard_protective_count"`
	LiveGuardOverblockingCount      int                        `json:"live_guard_overblocking_count"`
	LiveGuardImprovingCount         int                        `json:"live_guard_improving_count"`
	LiveGuardDegradingCount         int                        `json:"live_guard_degrading_count"`
	LiveGuardNewlyOverblockingCount int                        `json:"live_guard_newly_overblocking_count"`
	DirectLiveActionCandidateCount  int                        `json:"direct_live_action_candidate_count"`
	OpenDirectLiveActionCount       int                        `json:"open_direct_live_action_count"`
	RollbackCandidateCount          int                        `json:"rollback_candidate_count"`
	SuppressionCandidateCount       int                        `json:"suppression_candidate_count"`
	ClassCounts                     map[string]int             `json:"class_counts,omitempty"`
	LabelCounts                     map[string]int             `json:"label_counts,omitempty"`
	TopPositivePatterns             []DealReviewLearnedPattern `json:"top_positive_patterns,omitempty"`
	TopNegativePatterns             []DealReviewLearnedPattern `json:"top_negative_patterns,omitempty"`
	TopSymbolOverrides              []DealReviewLearnedPattern `json:"top_symbol_overrides,omitempty"`
	TopExpiringMonitoringRules      []DealReviewLearnedPattern `json:"top_expiring_monitoring_rules,omitempty"`
	TopRollbackWatchPatterns        []DealReviewLearnedPattern `json:"top_rollback_watch_patterns,omitempty"`
	TopFragileMonitoringRules       []DealReviewLearnedPattern `json:"top_fragile_monitoring_rules,omitempty"`
	TopOverblockingMonitoringRules  []DealReviewLearnedPattern `json:"top_overblocking_monitoring_rules,omitempty"`
	TopImprovingMonitoringRules     []DealReviewLearnedPattern `json:"top_improving_monitoring_rules,omitempty"`
	TopDegradingMonitoringRules     []DealReviewLearnedPattern `json:"top_degrading_monitoring_rules,omitempty"`
	TopDirectLiveActionCandidates   []DealReviewLearnedPattern `json:"top_direct_live_action_candidates,omitempty"`
	Notes                           []string                   `json:"notes,omitempty"`
}

type dealReviewLearnedPatternAggregate struct {
	ScopeType       string
	ScopeKey        string
	Symbol          string
	Side            string
	PatternOrder    int
	FeatureSet      []string
	Signature       string
	RegimeSignature string
	Total           int
	Wins            int
	Losses          int
	Flats           int
	NetPnL          float64
	NetPnLPct       float64
	SumMFEPct       float64
	SumMAEPct       float64
	SumGiveBackPct  float64
	GiveBackCount   int
	SumHoldMs       int64
	ObservedCases   []dealReviewSymbolBehaviorObservedCase
	Evidence        []DealReviewLearnedPatternEvidence
	FirstObservedAt time.Time
	LastObservedAt  time.Time
}

type dealReviewLearnedPatternBuildOptions struct {
	OwnerUserID             string
	OwnerTraderID           string
	IncludeFeatureRows      bool
	IncludeGlobalScope      bool
	IncludeTraderLocalScope bool
	IncludeRegimeLocalScope bool
	IncludeSymbolScope      bool
	EvidenceTraderID        string
}

type dealReviewLearnedPatternBaseline struct {
	Count        int
	Wins         int
	Losses       int
	Flats        int
	NetPnLPctSum float64
}

func (b dealReviewLearnedPatternBaseline) WinRate() float64 {
	if b.Count <= 0 {
		return 0
	}
	return float64(b.Wins) / float64(b.Count)
}

func (b dealReviewLearnedPatternBaseline) AvgPnLPct() float64 {
	if b.Count <= 0 {
		return 0
	}
	return b.NetPnLPctSum / float64(b.Count)
}

func (s *DealReviewStore) RefreshLearnedPatternsIfStale(userID, traderID string) (*DealReviewLearnedPatternRefreshResult, error) {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" {
		return &DealReviewLearnedPatternRefreshResult{}, nil
	}

	var latestCaseUpdatedRaw sql.NullString
	if err := s.db.Model(&DealReviewCase{}).
		Where("user_id = ? AND trader_id = ? AND status = ?", userID, traderID, DealReviewCaseStatusClosed).
		Select("CAST(MAX(updated_at) AS TEXT)").Scan(&latestCaseUpdatedRaw).Error; err != nil {
		return nil, err
	}
	latestCaseUpdated := parseDealReviewAggregateTime(latestCaseUpdatedRaw.String)

	latestBuilt, err := s.latestLearnedPatternBuiltAt(userID, traderID)
	if err != nil {
		return nil, err
	}

	needsRefresh := latestBuilt.IsZero()
	if !needsRefresh && !latestCaseUpdated.IsZero() && latestCaseUpdated.After(latestBuilt) {
		needsRefresh = true
	}
	if !needsRefresh && time.Since(latestBuilt) >= dealReviewLearnedPatternRefreshInterval {
		needsRefresh = true
	}
	if !needsRefresh {
		if err := s.backfillLearnedPatternStableKeys(userID, traderID); err != nil {
			return nil, err
		}
		featureCount, err := s.countLearnedPatternFeatures(userID, traderID)
		if err != nil {
			return nil, err
		}
		patternCount, err := s.countLearnedPatterns(userID, traderID)
		if err != nil {
			return nil, err
		}
		return &DealReviewLearnedPatternRefreshResult{
			Rebuilt:      false,
			FeatureCount: featureCount,
			PatternCount: patternCount,
			GeneratedAt:  latestBuilt,
		}, nil
	}
	return s.RebuildLearnedPatterns(userID, traderID)
}

func (s *DealReviewStore) RebuildLearnedPatterns(userID, traderID string) (*DealReviewLearnedPatternRefreshResult, error) {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" {
		return &DealReviewLearnedPatternRefreshResult{}, nil
	}
	startedAt := time.Now().UTC()

	var cases []DealReviewCase
	if err := s.db.
		Where("user_id = ? AND trader_id = ? AND status = ?", userID, traderID, DealReviewCaseStatusClosed).
		Order("exit_time_ms DESC, updated_at DESC").
		Find(&cases).Error; err != nil {
		return nil, err
	}
	var userCases []DealReviewCase
	if err := s.db.
		Where("user_id = ? AND status = ?", userID, DealReviewCaseStatusClosed).
		Order("exit_time_ms DESC, updated_at DESC").
		Find(&userCases).Error; err != nil {
		return nil, err
	}

	featureRows := make([]DealReviewPatternFeatureRecord, 0, len(cases))
	patterns := make([]DealReviewLearnedPattern, 0)
	finishedAt := time.Now().UTC()
	if len(cases) > 0 {
		featureRows, patterns = buildDealReviewLearnedPatterns(cases, userID, traderID, finishedAt)
	}
	if len(userCases) > 0 {
		patterns = append(patterns, buildDealReviewGlobalLearnedPatterns(userCases, userID, traderID, finishedAt)...)
	}
	evidenceRows := buildDealReviewPatternEvidenceRows(patterns, finishedAt)
	backlogCandidates := buildDealReviewPatternBacklogCandidates(userID, traderID, featureRows, patterns, finishedAt)
	validationRun := buildDealReviewPatternValidationRun(userID, traderID, startedAt, finishedAt, featureRows, patterns, evidenceRows, backlogCandidates)

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockTraderForDealReviewPatternRebuild(tx, userID, traderID); err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&DealReviewPatternFeatureRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&DealReviewPatternEvidenceRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&DealReviewPatternBacklogCandidate{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&DealReviewLearnedPattern{}).Error; err != nil {
			return err
		}
		if len(featureRows) > 0 {
			if err := dealReviewCreateInBatches(tx, featureRows); err != nil {
				return err
			}
		}
		if len(patterns) > 0 {
			if err := dealReviewCreateInBatches(tx, patterns); err != nil {
				return err
			}
			if err := s.persistLearnedPatternLifecycleSnapshotsTx(
				tx,
				userID,
				traderID,
				patterns,
				finishedAt,
				dealReviewLearnedPatternLifecycleSnapshotSourceRebuild,
				"",
			); err != nil {
				return err
			}
		}
		if len(evidenceRows) > 0 {
			if err := dealReviewCreateInBatches(tx, evidenceRows); err != nil {
				return err
			}
		}
		if len(backlogCandidates) > 0 {
			if err := dealReviewCreateInBatches(tx, backlogCandidates); err != nil {
				return err
			}
		}
		if validationRun != nil {
			if err := tx.Create(validationRun).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &DealReviewLearnedPatternRefreshResult{
		Rebuilt:      true,
		FeatureCount: len(featureRows),
		PatternCount: len(patterns),
		GeneratedAt:  finishedAt,
	}, nil
}

func lockTraderForDealReviewPatternRebuild(tx *gorm.DB, userID, traderID string) error {
	if tx == nil {
		return nil
	}
	switch tx.Dialector.Name() {
	case "postgres", "mysql":
	default:
		return nil
	}

	var trader Trader
	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND id = ?", userID, traderID).
		First(&trader).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

func (s *DealReviewStore) ListLearnedPatterns(userID, traderID string, filter DealReviewLearnedPatternFilter) ([]DealReviewLearnedPattern, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 24
	}
	requiresPostHydrationFilter := shouldApplyDealReviewLearnedPatternPostHydrationFilter(filter)
	query := s.db.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID))

	patternID := strings.TrimSpace(filter.PatternID)
	stableKey := strings.TrimSpace(filter.StableKey)
	switch {
	case patternID != "" && stableKey != "":
		query = query.Where("(id = ? OR stable_key = ?)", patternID, stableKey)
	case patternID != "":
		query = query.Where("id = ?", patternID)
	case stableKey != "":
		query = query.Where("stable_key = ?", stableKey)
	}
	if symbol := strings.ToUpper(strings.TrimSpace(filter.Symbol)); symbol != "" {
		query = query.Where("symbol = ? OR symbol = ''", symbol)
	}
	if side := normalizeDealReviewSide(filter.Side); side != "" {
		query = query.Where("side = ?", side)
	}
	if scopeType := normalizeDealReviewLearnedPatternScopeType(filter.ScopeType); scopeType != "" {
		query = query.Where("scope_type = ?", scopeType)
	}
	if class := normalizeDealReviewLearnedPatternClass(filter.PatternClass); class != "" {
		query = query.Where("pattern_class = ?", class)
	}
	if label := normalizeDealReviewLearnedPatternValidationLabel(filter.ValidationLabel); label != "" {
		query = query.Where("validation_label = ?", label)
	}
	if feature := normalizeDealReviewPatternFeatureFilterToken(filter.Feature); feature != "" {
		pattern := fmt.Sprintf("%%\"%s\"%%", feature)
		query = query.Where("feature_set_json LIKE ?", pattern)
	}
	if regime := normalizeDealReviewPatternFeatureFilterToken(filter.Regime); regime != "" {
		query = query.Where("regime_signature LIKE ?", "%"+regime+"%")
	}
	if filter.MinConfidence > 0 {
		query = query.Where("confidence_score >= ?", clampDealReviewUnit(filter.MinConfidence))
	}
	if filter.MinDrift > 0 {
		query = query.Where("drift_score >= ?", clampDealReviewUnit(filter.MinDrift))
	}
	if filter.MinSampleCount > 0 {
		query = query.Where("sample_count >= ?", filter.MinSampleCount)
	}

	var items []DealReviewLearnedPattern
	query = query.Order("composite_score DESC, sample_count DESC, last_observed_at DESC")
	if !requiresPostHydrationFilter {
		query = query.Limit(filter.Limit)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	for idx := range items {
		hydrateDealReviewLearnedPattern(&items[idx])
	}
	if err := s.hydrateLearnedPatternEvidenceRows(items); err != nil {
		return nil, err
	}
	if err := s.hydrateLearnedPatternManualControls(strings.TrimSpace(userID), strings.TrimSpace(traderID), items); err != nil {
		return nil, err
	}
	if err := s.enrichLearnedPatternLifecycle(strings.TrimSpace(userID), strings.TrimSpace(traderID), items); err != nil {
		return nil, err
	}
	if err := s.hydrateLearnedPatternLifecycleHistory(
		strings.TrimSpace(userID),
		strings.TrimSpace(traderID),
		items,
		dealReviewLearnedPatternLifecycleHistoryLimit,
	); err != nil {
		return nil, err
	}
	if err := s.hydrateLearnedPatternLiveGuardAttribution(
		strings.TrimSpace(userID),
		strings.TrimSpace(traderID),
		items,
		dealReviewLearnedPatternLiveGuardRollupEventLimit,
	); err != nil {
		return nil, err
	}
	hydrateDealReviewLearnedPatternActionHints(items)
	if err := s.syncLearnedPatternSuggestedInterventions(
		strings.TrimSpace(userID),
		strings.TrimSpace(traderID),
		items,
	); err != nil {
		return nil, err
	}
	if err := s.hydrateLearnedPatternInterventions(
		strings.TrimSpace(userID),
		strings.TrimSpace(traderID),
		items,
		dealReviewLearnedPatternInterventionHistoryLimit,
	); err != nil {
		return nil, err
	}
	if requiresPostHydrationFilter {
		items = filterDealReviewLearnedPatternsPostHydration(items, filter)
		sortDealReviewLearnedPatternsForReview(items)
		if len(items) > filter.Limit {
			items = items[:filter.Limit]
		}
	}
	return items, nil
}

func (s *DealReviewStore) ListMatchingLearnedPatterns(userID, traderID string, caseRec *DealReviewCase, limit int) ([]DealReviewLearnedPattern, error) {
	if caseRec == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 6
	}
	featureSet, _ := extractDealReviewLearnedPatternFeatures(caseRec)
	if len(featureSet) == 0 {
		return nil, nil
	}
	featureLookup := make(map[string]struct{}, len(featureSet))
	for _, feature := range featureSet {
		featureLookup[feature] = struct{}{}
	}

	var items []DealReviewLearnedPattern
	if err := s.db.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ? AND side = ? AND (symbol = '' OR symbol = ?)",
			strings.TrimSpace(userID),
			strings.TrimSpace(traderID),
			normalizeDealReviewSide(caseRec.Side),
			strings.ToUpper(strings.TrimSpace(caseRec.Symbol)),
		).
		Order("composite_score DESC, sample_count DESC, last_observed_at DESC").
		Limit(200).
		Find(&items).Error; err != nil {
		return nil, err
	}

	matched := make([]DealReviewLearnedPattern, 0, limit)
	for idx := range items {
		hydrateDealReviewLearnedPattern(&items[idx])
		matchedPattern, matchScore := MatchDealReviewLearnedPattern(caseRec, &items[idx])
		if !matchedPattern {
			continue
		}
		items[idx].MatchScore = matchScore
		matched = append(matched, items[idx])
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].MatchScore == matched[j].MatchScore {
			if matched[i].CompositeScore == matched[j].CompositeScore {
				return matched[i].SampleCount > matched[j].SampleCount
			}
			return matched[i].CompositeScore > matched[j].CompositeScore
		}
		return matched[i].MatchScore > matched[j].MatchScore
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}
	if err := s.hydrateLearnedPatternEvidenceRows(matched); err != nil {
		return nil, err
	}
	if err := s.hydrateLearnedPatternManualControls(strings.TrimSpace(userID), strings.TrimSpace(traderID), matched); err != nil {
		return nil, err
	}
	if err := s.enrichLearnedPatternLifecycle(strings.TrimSpace(userID), strings.TrimSpace(traderID), matched); err != nil {
		return nil, err
	}
	if err := s.hydrateLearnedPatternLifecycleHistory(
		strings.TrimSpace(userID),
		strings.TrimSpace(traderID),
		matched,
		dealReviewLearnedPatternLifecycleHistoryLimit,
	); err != nil {
		return nil, err
	}
	if err := s.hydrateLearnedPatternLiveGuardAttribution(
		strings.TrimSpace(userID),
		strings.TrimSpace(traderID),
		matched,
		dealReviewLearnedPatternLiveGuardRollupEventLimit,
	); err != nil {
		return nil, err
	}
	hydrateDealReviewLearnedPatternActionHints(matched)
	if err := s.syncLearnedPatternSuggestedInterventions(
		strings.TrimSpace(userID),
		strings.TrimSpace(traderID),
		matched,
	); err != nil {
		return nil, err
	}
	if err := s.hydrateLearnedPatternInterventions(
		strings.TrimSpace(userID),
		strings.TrimSpace(traderID),
		matched,
		dealReviewLearnedPatternInterventionHistoryLimit,
	); err != nil {
		return nil, err
	}
	return matched, nil
}

func MatchDealReviewLearnedPattern(caseRec *DealReviewCase, pattern *DealReviewLearnedPattern) (bool, float64) {
	if caseRec == nil || pattern == nil {
		return false, 0
	}
	featureSet, _ := extractDealReviewLearnedPatternFeatures(caseRec)
	if len(featureSet) == 0 {
		return false, 0
	}
	if normalizeDealReviewSide(caseRec.Side) != normalizeDealReviewSide(pattern.Side) {
		return false, 0
	}
	scopeType := normalizeDealReviewLearnedPatternScopeType(pattern.ScopeType)
	switch scopeType {
	case DealReviewLearnedPatternScopeSymbol:
		if strings.ToUpper(strings.TrimSpace(pattern.Symbol)) != strings.ToUpper(strings.TrimSpace(caseRec.Symbol)) {
			return false, 0
		}
	case DealReviewLearnedPatternScopeRegimeLocal:
		caseRegimeKey := buildDealReviewLearnedPatternCaseRegimeScopeKey(featureSet)
		patternRegimeKey := strings.TrimSpace(pattern.ScopeKey)
		if patternRegimeKey == "" {
			patternRegimeKey = strings.TrimSpace(pattern.RegimeSignature)
		}
		if caseRegimeKey == "" || !strings.EqualFold(patternRegimeKey, caseRegimeKey) {
			return false, 0
		}
	}

	if len(pattern.FeatureSet) == 0 && strings.TrimSpace(pattern.FeatureSetJSON) != "" {
		hydrateDealReviewLearnedPattern(pattern)
	}
	if len(pattern.FeatureSet) == 0 {
		return false, 0
	}

	featureLookup := make(map[string]struct{}, len(featureSet))
	for _, feature := range featureSet {
		featureLookup[feature] = struct{}{}
	}
	if !dealReviewPatternFeatureSubset(featureLookup, pattern.FeatureSet) {
		return false, 0
	}
	return true, scoreDealReviewLearnedPatternMatch(caseRec, featureSet, pattern)
}

func BuildDealReviewLearnedPatternReportingSummary(items []DealReviewLearnedPattern) *DealReviewLearnedPatternReportingSummary {
	summary := &DealReviewLearnedPatternReportingSummary{
		TotalCount:  len(items),
		ClassCounts: map[string]int{},
		LabelCounts: map[string]int{},
	}
	if len(items) == 0 {
		return summary
	}

	positive := make([]DealReviewLearnedPattern, 0, len(items))
	negative := make([]DealReviewLearnedPattern, 0, len(items))
	symbolOverrides := make([]DealReviewLearnedPattern, 0, len(items))
	expiringRules := make([]DealReviewLearnedPattern, 0, len(items))
	rollbackWatch := make([]DealReviewLearnedPattern, 0, len(items))
	fragileRules := make([]DealReviewLearnedPattern, 0, len(items))
	overblockingRules := make([]DealReviewLearnedPattern, 0, len(items))
	improvingRules := make([]DealReviewLearnedPattern, 0, len(items))
	degradingRules := make([]DealReviewLearnedPattern, 0, len(items))
	directLiveActionCandidates := make([]DealReviewLearnedPattern, 0, len(items))
	for idx := range items {
		item := items[idx]
		hydrateDealReviewLearnedPattern(&item)
		classKey := normalizeDealReviewLearnedPatternClass(item.PatternClass)
		labelKey := normalizeDealReviewLearnedPatternValidationLabel(item.ValidationLabel)
		if classKey == "" {
			classKey = "unknown"
		}
		if labelKey == "" {
			labelKey = "unknown"
		}
		summary.ClassCounts[classKey]++
		summary.LabelCounts[labelKey]++
		switch classKey {
		case DealReviewLearnedPatternClassPositiveEdge:
			summary.PositiveCount++
			positive = append(positive, dealReviewLearnedPatternForSummaryCard(item))
		case DealReviewLearnedPatternClassNegativeEdge:
			summary.NegativeCount++
			negative = append(negative, dealReviewLearnedPatternForSummaryCard(item))
		}
		switch labelKey {
		case DealReviewLearnedPatternValidationLabelConfirmed:
			summary.ConfirmedCount++
		case DealReviewLearnedPatternValidationLabelCandidate:
			summary.CandidateCount++
		case DealReviewLearnedPatternValidationLabelFalsePositive:
			summary.FalsePositiveCount++
		case DealReviewLearnedPatternValidationLabelReverseRisk:
			summary.ReverseRiskCount++
		case DealReviewLearnedPatternValidationLabelDrifting:
			summary.DriftingCount++
		case DealReviewLearnedPatternValidationLabelExpired:
			summary.ExpiredCount++
		}
		if item.RecommendedUse == DealReviewLearnedPatternRecommendedUseMonitoringRule {
			summary.MonitoringRuleCount++
		}
		if item.Lifecycle != nil {
			switch normalizeDealReviewLearnedPatternLifecycleStatus(item.Lifecycle.Status) {
			case DealReviewLearnedPatternLifecycleStatusActive:
				summary.LifecycleActiveCount++
			case DealReviewLearnedPatternLifecycleStatusDegrading:
				summary.LifecycleDegradingCount++
				expiringRules = append(expiringRules, dealReviewLearnedPatternForSummaryCard(item))
			case DealReviewLearnedPatternLifecycleStatusRollbackWatch:
				summary.LifecycleRollbackWatchCount++
				rollbackWatch = append(rollbackWatch, dealReviewLearnedPatternForSummaryCard(item))
			case DealReviewLearnedPatternLifecycleStatusExpired:
				summary.LifecycleExpiredCount++
				expiringRules = append(expiringRules, dealReviewLearnedPatternForSummaryCard(item))
			}
		}
		if item.LifecycleTrend != nil {
			if item.LifecycleTrend.Fragile {
				summary.LifecycleFragileCount++
				fragileRules = append(fragileRules, dealReviewLearnedPatternForSummaryCard(item))
			}
			if item.LifecycleTrend.StaleGuardSnapshotCount > 0 {
				summary.LifecycleLaggingGuardCount++
			}
		}
		if item.LiveGuardAttribution != nil {
			switch normalizeDealReviewLearnedPatternLiveGuardAttributionLabel(item.LiveGuardAttribution.AttributionLabel) {
			case DealReviewLearnedPatternLiveGuardAttributionLabelProtective:
				summary.LiveGuardProtectiveCount++
			case DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking:
				summary.LiveGuardOverblockingCount++
				overblockingRules = append(overblockingRules, dealReviewLearnedPatternForSummaryCard(item))
			}
		}
		if item.LiveGuardAttributionDelta != nil {
			switch normalizeDealReviewLearnedPatternLiveGuardAttributionTrendLabel(item.LiveGuardAttributionDelta.TrendLabel) {
			case DealReviewLearnedPatternLiveGuardAttributionTrendImproving:
				summary.LiveGuardImprovingCount++
				improvingRules = append(improvingRules, dealReviewLearnedPatternForSummaryCard(item))
			case DealReviewLearnedPatternLiveGuardAttributionTrendDegrading:
				summary.LiveGuardDegradingCount++
				degradingRules = append(degradingRules, dealReviewLearnedPatternForSummaryCard(item))
			case DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking:
				summary.LiveGuardNewlyOverblockingCount++
				degradingRules = append(degradingRules, dealReviewLearnedPatternForSummaryCard(item))
			}
		}
		if dealReviewLearnedPatternHasDirectLiveActionCandidate(&item) {
			summary.DirectLiveActionCandidateCount++
			directLiveActionCandidates = append(directLiveActionCandidates, dealReviewLearnedPatternForSummaryCard(item))
			if dealReviewLearnedPatternHasOpenDirectLiveActionCandidate(&item) {
				summary.OpenDirectLiveActionCount++
			}
			switch dealReviewLearnedPatternStrongestLiveActionKind(&item, false) {
			case DealReviewLearnedPatternLiveActionKindRollback:
				summary.RollbackCandidateCount++
			case DealReviewLearnedPatternLiveActionKindSuppression:
				summary.SuppressionCandidateCount++
			}
		}
		if item.ScopeType == DealReviewLearnedPatternScopeSymbol &&
			(item.ValidationLabel == DealReviewLearnedPatternValidationLabelConfirmed ||
				item.ValidationLabel == DealReviewLearnedPatternValidationLabelFalsePositive ||
				item.ValidationLabel == DealReviewLearnedPatternValidationLabelReverseRisk) {
			symbolOverrides = append(symbolOverrides, dealReviewLearnedPatternForSummaryCard(item))
		}
	}

	sort.SliceStable(positive, func(i, j int) bool {
		if positive[i].CompositeScore == positive[j].CompositeScore {
			return positive[i].SampleCount > positive[j].SampleCount
		}
		return positive[i].CompositeScore > positive[j].CompositeScore
	})
	sort.SliceStable(negative, func(i, j int) bool {
		if negative[i].CompositeScore == negative[j].CompositeScore {
			return negative[i].SampleCount > negative[j].SampleCount
		}
		return negative[i].CompositeScore > negative[j].CompositeScore
	})
	sort.SliceStable(symbolOverrides, func(i, j int) bool {
		left := math.Abs(symbolOverrides[i].LiftAvgPnLPct)
		right := math.Abs(symbolOverrides[j].LiftAvgPnLPct)
		if left == right {
			return symbolOverrides[i].CompositeScore > symbolOverrides[j].CompositeScore
		}
		return left > right
	})
	sort.SliceStable(expiringRules, func(i, j int) bool {
		left := 0.0
		right := 0.0
		if expiringRules[i].Lifecycle != nil {
			left = expiringRules[i].Lifecycle.ExpiryScore
		}
		if expiringRules[j].Lifecycle != nil {
			right = expiringRules[j].Lifecycle.ExpiryScore
		}
		if left == right {
			return expiringRules[i].CompositeScore > expiringRules[j].CompositeScore
		}
		return left > right
	})
	sort.SliceStable(rollbackWatch, func(i, j int) bool {
		left := 0.0
		right := 0.0
		if rollbackWatch[i].Lifecycle != nil {
			left = rollbackWatch[i].Lifecycle.RollbackScore
		}
		if rollbackWatch[j].Lifecycle != nil {
			right = rollbackWatch[j].Lifecycle.RollbackScore
		}
		if left == right {
			return rollbackWatch[i].CompositeScore > rollbackWatch[j].CompositeScore
		}
		return left > right
	})
	sort.SliceStable(fragileRules, func(i, j int) bool {
		leftShare := 0.0
		rightShare := 0.0
		leftLagShare := 0.0
		rightLagShare := 0.0
		if fragileRules[i].LifecycleTrend != nil {
			leftShare = fragileRules[i].LifecycleTrend.RollbackWatchShare + fragileRules[i].LifecycleTrend.DegradingShare
			leftLagShare = fragileRules[i].LifecycleTrend.StaleGuardSnapshotShare
		}
		if fragileRules[j].LifecycleTrend != nil {
			rightShare = fragileRules[j].LifecycleTrend.RollbackWatchShare + fragileRules[j].LifecycleTrend.DegradingShare
			rightLagShare = fragileRules[j].LifecycleTrend.StaleGuardSnapshotShare
		}
		if leftShare == rightShare {
			if leftLagShare == rightLagShare {
				return fragileRules[i].CompositeScore > fragileRules[j].CompositeScore
			}
			return leftLagShare > rightLagShare
		}
		return leftShare > rightShare
	})
	sort.SliceStable(overblockingRules, func(i, j int) bool {
		leftRate := 0.0
		rightRate := 0.0
		leftConfidence := 0.0
		rightConfidence := 0.0
		leftResolved := 0
		rightResolved := 0
		if overblockingRules[i].LiveGuardAttribution != nil {
			leftRate = overblockingRules[i].LiveGuardAttribution.OverblockingRate
			leftConfidence = overblockingRules[i].LiveGuardAttribution.ConfidenceScore
			leftResolved = overblockingRules[i].LiveGuardAttribution.ResolvedEventCount
		}
		if overblockingRules[j].LiveGuardAttribution != nil {
			rightRate = overblockingRules[j].LiveGuardAttribution.OverblockingRate
			rightConfidence = overblockingRules[j].LiveGuardAttribution.ConfidenceScore
			rightResolved = overblockingRules[j].LiveGuardAttribution.ResolvedEventCount
		}
		if leftRate == rightRate {
			if leftConfidence == rightConfidence {
				if leftResolved == rightResolved {
					return overblockingRules[i].CompositeScore > overblockingRules[j].CompositeScore
				}
				return leftResolved > rightResolved
			}
			return leftConfidence > rightConfidence
		}
		return leftRate > rightRate
	})
	sort.SliceStable(improvingRules, func(i, j int) bool {
		leftDelta := 0.0
		rightDelta := 0.0
		leftConfidence := 0.0
		rightConfidence := 0.0
		leftResolved := 0
		rightResolved := 0
		if improvingRules[i].LiveGuardAttributionDelta != nil {
			leftDelta = improvingRules[i].LiveGuardAttributionDelta.ProtectiveRateDelta
			leftConfidence = improvingRules[i].LiveGuardAttributionDelta.ConfidenceScore
			leftResolved = improvingRules[i].LiveGuardAttributionDelta.RecentResolvedEventCount
		}
		if improvingRules[j].LiveGuardAttributionDelta != nil {
			rightDelta = improvingRules[j].LiveGuardAttributionDelta.ProtectiveRateDelta
			rightConfidence = improvingRules[j].LiveGuardAttributionDelta.ConfidenceScore
			rightResolved = improvingRules[j].LiveGuardAttributionDelta.RecentResolvedEventCount
		}
		if leftDelta == rightDelta {
			if leftConfidence == rightConfidence {
				if leftResolved == rightResolved {
					return improvingRules[i].CompositeScore > improvingRules[j].CompositeScore
				}
				return leftResolved > rightResolved
			}
			return leftConfidence > rightConfidence
		}
		return leftDelta > rightDelta
	})
	sort.SliceStable(degradingRules, func(i, j int) bool {
		leftTrend := ""
		rightTrend := ""
		leftDelta := 0.0
		rightDelta := 0.0
		leftConfidence := 0.0
		rightConfidence := 0.0
		leftResolved := 0
		rightResolved := 0
		if degradingRules[i].LiveGuardAttributionDelta != nil {
			leftTrend = normalizeDealReviewLearnedPatternLiveGuardAttributionTrendLabel(degradingRules[i].LiveGuardAttributionDelta.TrendLabel)
			leftDelta = degradingRules[i].LiveGuardAttributionDelta.OverblockingRateDelta
			leftConfidence = degradingRules[i].LiveGuardAttributionDelta.ConfidenceScore
			leftResolved = degradingRules[i].LiveGuardAttributionDelta.RecentResolvedEventCount
		}
		if degradingRules[j].LiveGuardAttributionDelta != nil {
			rightTrend = normalizeDealReviewLearnedPatternLiveGuardAttributionTrendLabel(degradingRules[j].LiveGuardAttributionDelta.TrendLabel)
			rightDelta = degradingRules[j].LiveGuardAttributionDelta.OverblockingRateDelta
			rightConfidence = degradingRules[j].LiveGuardAttributionDelta.ConfidenceScore
			rightResolved = degradingRules[j].LiveGuardAttributionDelta.RecentResolvedEventCount
		}
		if leftTrend != rightTrend {
			if leftTrend == DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking {
				return true
			}
			if rightTrend == DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking {
				return false
			}
		}
		if leftDelta == rightDelta {
			if leftConfidence == rightConfidence {
				if leftResolved == rightResolved {
					return degradingRules[i].CompositeScore > degradingRules[j].CompositeScore
				}
				return leftResolved > rightResolved
			}
			return leftConfidence > rightConfidence
		}
		return leftDelta > rightDelta
	})
	sortDealReviewLearnedPatternsForReview(directLiveActionCandidates)

	summary.TopPositivePatterns = limitDealReviewLearnedPatternSummaryItems(positive, 3)
	summary.TopNegativePatterns = limitDealReviewLearnedPatternSummaryItems(negative, 3)
	summary.TopSymbolOverrides = limitDealReviewLearnedPatternSummaryItems(symbolOverrides, 3)
	summary.TopExpiringMonitoringRules = limitDealReviewLearnedPatternSummaryItems(expiringRules, 3)
	summary.TopRollbackWatchPatterns = limitDealReviewLearnedPatternSummaryItems(rollbackWatch, 3)
	summary.TopFragileMonitoringRules = limitDealReviewLearnedPatternSummaryItems(fragileRules, 3)
	summary.TopOverblockingMonitoringRules = limitDealReviewLearnedPatternSummaryItems(overblockingRules, 3)
	summary.TopImprovingMonitoringRules = limitDealReviewLearnedPatternSummaryItems(improvingRules, 3)
	summary.TopDegradingMonitoringRules = limitDealReviewLearnedPatternSummaryItems(degradingRules, 3)
	summary.TopDirectLiveActionCandidates = limitDealReviewLearnedPatternSummaryItems(directLiveActionCandidates, 3)

	notes := make([]string, 0, 6)
	if summary.PositiveCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) currently lean positive under the filtered trader slice.", summary.PositiveCount))
	}
	if summary.NegativeCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) currently behave like anti-edges or caution setups.", summary.NegativeCount))
	}
	if summary.MonitoringRuleCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) are currently classified as operational monitoring rules for live guard review.", summary.MonitoringRuleCount))
	}
	if summary.FalsePositiveCount > 0 || summary.ReverseRiskCount > 0 || summary.DriftingCount > 0 {
		notes = append(notes, fmt.Sprintf("%d false-positive, %d reverse-risk, and %d drifting pattern(s) now need caution.", summary.FalsePositiveCount, summary.ReverseRiskCount, summary.DriftingCount))
	}
	if summary.LifecycleDegradingCount > 0 || summary.LifecycleExpiredCount > 0 {
		notes = append(notes, fmt.Sprintf("%d monitoring rule(s) are degrading and %d are already expired for live use.", summary.LifecycleDegradingCount, summary.LifecycleExpiredCount))
	}
	if summary.LifecycleRollbackWatchCount > 0 {
		notes = append(notes, fmt.Sprintf("%d monitoring rule(s) now sit on rollback watch because live guard activity is outrunning fresh validation support.", summary.LifecycleRollbackWatchCount))
	}
	if summary.LifecycleFragileCount > 0 {
		notes = append(notes, fmt.Sprintf("%d monitoring rule(s) now look structurally fragile across the persisted lifecycle trail.", summary.LifecycleFragileCount))
	}
	if summary.LifecycleLaggingGuardCount > 0 {
		notes = append(notes, fmt.Sprintf("%d pattern(s) showed live-guard hits after the last supporting evidence had already gone stale.", summary.LifecycleLaggingGuardCount))
	}
	if summary.LiveGuardProtectiveCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) now have recent live-guard follow-up that is mostly protective.", summary.LiveGuardProtectiveCount))
	}
	if summary.LiveGuardOverblockingCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) now look overblocking based on recent live-guard follow-up attribution.", summary.LiveGuardOverblockingCount))
	}
	if summary.LiveGuardImprovingCount > 0 {
		notes = append(notes, fmt.Sprintf("%d monitoring rule(s) improved versus their prior live-guard comparison window.", summary.LiveGuardImprovingCount))
	}
	if summary.LiveGuardDegradingCount > 0 || summary.LiveGuardNewlyOverblockingCount > 0 {
		notes = append(notes, fmt.Sprintf("%d monitoring rule(s) are degrading and %d now look newly overblocking versus the prior comparison window.", summary.LiveGuardDegradingCount, summary.LiveGuardNewlyOverblockingCount))
	}
	if summary.DirectLiveActionCandidateCount > 0 {
		notes = append(notes, fmt.Sprintf("%d monitoring rule(s) now carry direct live-action escalation, including %d open candidates and %d rollback-grade cases.", summary.DirectLiveActionCandidateCount, summary.OpenDirectLiveActionCount, summary.RollbackCandidateCount))
	}
	if len(summary.TopSymbolOverrides) > 0 {
		top := summary.TopSymbolOverrides[0]
		notes = append(notes, fmt.Sprintf("Top symbol override %q on %s %s deviates by %+.2f%% against the side baseline.", top.PatternSignature, top.Symbol, top.Side, top.LiftAvgPnLPct))
	}
	summary.Notes = notes
	return summary
}

func buildDealReviewLearnedPatterns(cases []DealReviewCase, userID, traderID string, now time.Time) ([]DealReviewPatternFeatureRecord, []DealReviewLearnedPattern) {
	return buildDealReviewLearnedPatternArtifacts(cases, now, dealReviewLearnedPatternBuildOptions{
		OwnerUserID:             userID,
		OwnerTraderID:           traderID,
		IncludeFeatureRows:      true,
		IncludeTraderLocalScope: true,
		IncludeRegimeLocalScope: true,
		IncludeSymbolScope:      true,
		EvidenceTraderID:        traderID,
	})
}

func buildDealReviewGlobalLearnedPatterns(cases []DealReviewCase, userID, traderID string, now time.Time) []DealReviewLearnedPattern {
	_, patterns := buildDealReviewLearnedPatternArtifacts(cases, now, dealReviewLearnedPatternBuildOptions{
		OwnerUserID:        userID,
		OwnerTraderID:      traderID,
		IncludeGlobalScope: true,
		EvidenceTraderID:   traderID,
	})
	return patterns
}

func buildDealReviewLearnedPatternArtifacts(cases []DealReviewCase, now time.Time, options dealReviewLearnedPatternBuildOptions) ([]DealReviewPatternFeatureRecord, []DealReviewLearnedPattern) {
	featureRows := make([]DealReviewPatternFeatureRecord, 0, len(cases))
	aggregates := map[string]*dealReviewLearnedPatternAggregate{}
	baselines := map[string]*dealReviewLearnedPatternBaseline{}

	for idx := range cases {
		caseRec := cases[idx]
		if !strings.EqualFold(caseRec.Status, DealReviewCaseStatusClosed) {
			continue
		}
		featureSet, qualityFlags := extractDealReviewLearnedPatternFeatures(&caseRec)
		observedAt := dealReviewSymbolBehaviorObservedAt(&caseRec)
		if observedAt.IsZero() {
			observedAt = now
		}
		if options.IncludeFeatureRows {
			featureRows = append(featureRows, DealReviewPatternFeatureRecord{
				UserID:             options.OwnerUserID,
				TraderID:           options.OwnerTraderID,
				CaseID:             strings.TrimSpace(caseRec.ID),
				PositionID:         caseRec.PositionID,
				Symbol:             strings.ToUpper(strings.TrimSpace(caseRec.Symbol)),
				Side:               normalizeDealReviewSide(caseRec.Side),
				Outcome:            classifyDealOutcome(caseRec.RealizedPnL),
				OpenConfidence:     caseRec.OpenConfidence,
				RealizedPnL:        caseRec.RealizedPnL,
				RealizedPnLPct:     caseRec.RealizedPnLPct,
				MaxMFEPct:          caseRec.MaxFavorableExcursionPct,
				MaxMAEPct:          caseRec.MaxAdverseExcursionPct,
				ProfitGivenBackPct: caseRec.ProfitGivenBackPct,
				HoldDurationMs:     caseRec.HoldDurationMs,
				FeatureCount:       len(featureSet),
				FeaturesJSON:       marshalDealReviewStringArray(featureSet),
				QualityFlagsJSON:   marshalDealReviewStringArray(qualityFlags),
				ObservedAt:         observedAt,
				BuiltAt:            now,
				Features:           append([]string(nil), featureSet...),
				QualityFlags:       append([]string(nil), qualityFlags...),
			})
		}

		side := normalizeDealReviewSide(caseRec.Side)
		if side == "" || len(featureSet) == 0 {
			continue
		}
		baseline := baselines[side]
		if baseline == nil {
			baseline = &dealReviewLearnedPatternBaseline{}
			baselines[side] = baseline
		}
		baseline.Count++
		baseline.NetPnLPctSum += caseRec.RealizedPnLPct
		switch classifyDealOutcome(caseRec.RealizedPnL) {
		case "profit":
			baseline.Wins++
		case "loss":
			baseline.Losses++
		default:
			baseline.Flats++
		}

		combos := buildDealReviewLearnedPatternCombos(featureSet)
		for _, scope := range buildDealReviewLearnedPatternScopes(&caseRec, featureSet, options) {
			for _, combo := range combos {
				key := strings.Join([]string{scope.scopeType, scope.scopeKey, side, strings.Join(combo, " + ")}, "|")
				agg := aggregates[key]
				if agg == nil {
					regimeSignature := buildDealReviewLearnedPatternRegimeSignature(combo)
					if strings.TrimSpace(scope.regimeSignature) != "" {
						regimeSignature = strings.TrimSpace(scope.regimeSignature)
					}
					agg = &dealReviewLearnedPatternAggregate{
						ScopeType:       scope.scopeType,
						ScopeKey:        scope.scopeKey,
						Symbol:          scope.symbol,
						Side:            side,
						PatternOrder:    len(combo),
						FeatureSet:      append([]string(nil), combo...),
						Signature:       strings.Join(combo, " + "),
						RegimeSignature: regimeSignature,
					}
					aggregates[key] = agg
				}
				agg.Total++
				agg.NetPnL += caseRec.RealizedPnL
				agg.NetPnLPct += caseRec.RealizedPnLPct
				agg.SumMFEPct += caseRec.MaxFavorableExcursionPct
				agg.SumMAEPct += caseRec.MaxAdverseExcursionPct
				agg.SumHoldMs += caseRec.HoldDurationMs
				if caseRec.ProfitGivenBackPct > 0 {
					agg.GiveBackCount++
					agg.SumGiveBackPct += caseRec.ProfitGivenBackPct
				}
				outcome := classifyDealOutcome(caseRec.RealizedPnL)
				switch outcome {
				case "profit":
					agg.Wins++
				case "loss":
					agg.Losses++
				default:
					agg.Flats++
				}
				agg.ObservedCases = append(agg.ObservedCases, dealReviewSymbolBehaviorObservedCase{
					ObservedAt:     observedAt,
					Outcome:        outcome,
					RealizedPnL:    caseRec.RealizedPnL,
					RealizedPnLPct: caseRec.RealizedPnLPct,
				})
				if agg.FirstObservedAt.IsZero() || observedAt.Before(agg.FirstObservedAt) {
					agg.FirstObservedAt = observedAt
				}
				if agg.LastObservedAt.IsZero() || observedAt.After(agg.LastObservedAt) {
					agg.LastObservedAt = observedAt
				}
				if len(agg.Evidence) < dealReviewLearnedPatternEvidenceLimit &&
					(strings.TrimSpace(options.EvidenceTraderID) == "" ||
						strings.EqualFold(strings.TrimSpace(caseRec.TraderID), strings.TrimSpace(options.EvidenceTraderID))) {
					agg.Evidence = append(agg.Evidence, DealReviewLearnedPatternEvidence{
						CaseID:         strings.TrimSpace(caseRec.ID),
						PositionID:     caseRec.PositionID,
						Symbol:         strings.ToUpper(strings.TrimSpace(caseRec.Symbol)),
						Side:           side,
						Outcome:        outcome,
						RealizedPnL:    caseRec.RealizedPnL,
						RealizedPnLPct: caseRec.RealizedPnLPct,
						HoldDurationMs: caseRec.HoldDurationMs,
						ObservedAt:     observedAt.Format(time.RFC3339),
					})
				}
			}
		}
	}

	patterns := make([]DealReviewLearnedPattern, 0, len(aggregates))
	for _, agg := range aggregates {
		if agg.Total < dealReviewLearnedPatternObservedThreshold {
			continue
		}
		pattern, ok := buildDealReviewLearnedPatternFromAggregate(agg, baselines[agg.Side], now)
		if !ok {
			continue
		}
		pattern.UserID = options.OwnerUserID
		pattern.TraderID = options.OwnerTraderID
		patterns = append(patterns, pattern)
	}

	sort.SliceStable(patterns, func(i, j int) bool {
		if patterns[i].CompositeScore == patterns[j].CompositeScore {
			return patterns[i].SampleCount > patterns[j].SampleCount
		}
		return patterns[i].CompositeScore > patterns[j].CompositeScore
	})
	return featureRows, patterns
}

func buildDealReviewLearnedPatternFromAggregate(agg *dealReviewLearnedPatternAggregate, baseline *dealReviewLearnedPatternBaseline, now time.Time) (DealReviewLearnedPattern, bool) {
	if agg == nil || agg.Total < dealReviewLearnedPatternObservedThreshold {
		return DealReviewLearnedPattern{}, false
	}
	total := float64(agg.Total)
	winRate := aggRate(agg.Wins, agg.Total)
	lossRate := aggRate(agg.Losses, agg.Total)
	avgPnL := agg.NetPnL / total
	avgPnLPct := agg.NetPnLPct / total

	bias := DealReviewSymbolBehaviorBiasMixed
	switch {
	case winRate >= 0.55 && avgPnLPct > 0.05:
		bias = DealReviewSymbolBehaviorBiasPositive
	case lossRate >= 0.55 && avgPnLPct < -0.05:
		bias = DealReviewSymbolBehaviorBiasNegative
	case avgPnLPct >= 0.30 && agg.Wins >= agg.Losses:
		bias = DealReviewSymbolBehaviorBiasPositive
	case avgPnLPct <= -0.30 && agg.Losses >= agg.Wins:
		bias = DealReviewSymbolBehaviorBiasNegative
	}
	if bias == DealReviewSymbolBehaviorBiasMixed {
		return DealReviewLearnedPattern{}, false
	}

	supportCount := agg.Wins
	contradictCount := agg.Losses
	patternClass := DealReviewLearnedPatternClassPositiveEdge
	if bias == DealReviewSymbolBehaviorBiasNegative {
		supportCount = agg.Losses
		contradictCount = agg.Wins
		patternClass = DealReviewLearnedPatternClassNegativeEdge
	}

	trainingCases, validationCases, recentCases := splitDealReviewSymbolBehaviorObservedCases(agg.ObservedCases)
	validationMetrics := summarizeDealReviewSymbolBehaviorObservedCases(validationCases, bias)
	recentMetrics := summarizeDealReviewSymbolBehaviorObservedCases(recentCases, bias)
	driftScore := computeDealReviewSymbolBehaviorDriftScore(bias, validationMetrics, recentMetrics)
	recencyWeight := dealReviewSymbolBehaviorRecencyWeight(now, agg.LastObservedAt)
	status := classifyDealReviewSymbolBehaviorPriorStatus(
		agg.Total,
		bias,
		recencyWeight,
		validationMetrics,
		recentMetrics,
		driftScore,
		agg.LastObservedAt,
		now,
	)

	sampleScore := clampDealReviewUnit(float64(agg.Total) / 18.0)
	baselineWinRate := 0.0
	baselineAvgPnLPct := 0.0
	if baseline != nil {
		baselineWinRate = baseline.WinRate()
		baselineAvgPnLPct = baseline.AvgPnLPct()
	}
	liftWinRate := winRate - baselineWinRate
	liftAvgPnLPct := avgPnLPct - baselineAvgPnLPct
	liftStrength := computeDealReviewLearnedPatternLiftStrength(bias, liftWinRate, liftAvgPnLPct)

	windowSupport := validationMetrics.SupportScore
	if recentMetrics.SupportScore > windowSupport {
		windowSupport = recentMetrics.SupportScore
	}
	directionality := clampDealReviewUnit(math.Abs(winRate - lossRate))
	confidence := clampDealReviewUnit((sampleScore * 0.35) + (windowSupport * 0.30) + (recencyWeight * 0.15) + (directionality * 0.20))
	stability := clampDealReviewUnit(((1 - driftScore) * 0.60) + (windowSupport * 0.40))
	composite := clampDealReviewUnit((confidence * 0.30) + (stability * 0.20) + (liftStrength * 0.25) + (windowSupport * 0.15) + (sampleScore * 0.10))
	supportRate := aggRate(supportCount, agg.Total)
	if agg.PatternOrder >= 3 {
		if supportRate < dealReviewLearnedPatternTripleMinSupportRate ||
			confidence < dealReviewLearnedPatternTripleMinConfidence ||
			composite < dealReviewLearnedPatternTripleMinComposite ||
			windowSupport < dealReviewLearnedPatternTripleMinWindowScore {
			return DealReviewLearnedPattern{}, false
		}
	}

	proxy := &DealReviewSymbolBehaviorPrior{
		Status:                    status,
		BehaviorBias:              bias,
		ValidationSampleCount:     validationMetrics.Count,
		ValidationSupportCount:    validationMetrics.SupportCount,
		ValidationContradictCount: validationMetrics.ContradictCount,
		ValidationAvgPnLPct:       validationMetrics.AvgPnLPct,
		ValidationSupportScore:    validationMetrics.SupportScore,
		RecentSampleCount:         recentMetrics.Count,
		RecentSupportCount:        recentMetrics.SupportCount,
		RecentContradictCount:     recentMetrics.ContradictCount,
		RecentAvgPnLPct:           recentMetrics.AvgPnLPct,
		RecentSupportScore:        recentMetrics.SupportScore,
		DriftScore:                driftScore,
	}
	applyDealReviewSymbolBehaviorPriorValidationReport(proxy)
	validationLabel := mapDealReviewLearnedPatternValidationLabel(proxy.ValidationLabel)
	validationAlert := buildDealReviewLearnedPatternValidationAlert(patternClass, validationLabel, proxy.FalsePositiveScore, proxy.FalseNegativeScore, driftScore)
	recommendedUse := buildDealReviewLearnedPatternRecommendedUse(
		patternClass,
		validationLabel,
		composite,
		confidence,
		validationMetrics.SupportScore,
		proxy.FalsePositiveScore,
		driftScore,
		agg.Total,
		math.Abs(liftAvgPnLPct),
	)

	pattern := DealReviewLearnedPattern{
		ID:                        uuid.NewString(),
		ScopeType:                 agg.ScopeType,
		ScopeKey:                  agg.ScopeKey,
		Symbol:                    agg.Symbol,
		Side:                      agg.Side,
		PatternClass:              patternClass,
		Status:                    status,
		ValidationLabel:           validationLabel,
		RecommendedUse:            recommendedUse,
		PatternSignature:          agg.Signature,
		RegimeSignature:           agg.RegimeSignature,
		PatternOrder:              agg.PatternOrder,
		FeatureCount:              len(agg.FeatureSet),
		FeatureSetJSON:            marshalDealReviewStringArray(agg.FeatureSet),
		SampleCount:               agg.Total,
		WinningDeals:              agg.Wins,
		LosingDeals:               agg.Losses,
		FlatDeals:                 agg.Flats,
		SupportCount:              supportCount,
		ContradictCount:           contradictCount,
		WinRate:                   winRate,
		LossRate:                  lossRate,
		NetPnL:                    agg.NetPnL,
		AvgPnL:                    avgPnL,
		AvgPnLPct:                 avgPnLPct,
		Expectancy:                avgPnL,
		AvgMFEPct:                 agg.SumMFEPct / total,
		AvgMAEPct:                 agg.SumMAEPct / total,
		GiveBackRate:              aggRate(agg.GiveBackCount, agg.Total),
		AvgGiveBackPct:            safeAvgDealReviewPattern(agg.SumGiveBackPct, agg.GiveBackCount),
		AvgHoldMs:                 int64(float64(agg.SumHoldMs) / total),
		BaselineWinRate:           baselineWinRate,
		BaselineAvgPnLPct:         baselineAvgPnLPct,
		LiftWinRate:               liftWinRate,
		LiftAvgPnLPct:             liftAvgPnLPct,
		TrainingSampleCount:       len(trainingCases),
		ValidationSampleCount:     validationMetrics.Count,
		ValidationSupportCount:    validationMetrics.SupportCount,
		ValidationContradictCount: validationMetrics.ContradictCount,
		ValidationAvgPnLPct:       validationMetrics.AvgPnLPct,
		ValidationSupportScore:    validationMetrics.SupportScore,
		RecentSampleCount:         recentMetrics.Count,
		RecentSupportCount:        recentMetrics.SupportCount,
		RecentContradictCount:     recentMetrics.ContradictCount,
		RecentAvgPnLPct:           recentMetrics.AvgPnLPct,
		RecentSupportScore:        recentMetrics.SupportScore,
		ConfidenceScore:           confidence,
		StabilityScore:            stability,
		DriftScore:                driftScore,
		RecencyWeight:             recencyWeight,
		CompositeScore:            composite,
		FalsePositiveScore:        proxy.FalsePositiveScore,
		ReverseRiskScore:          proxy.FalseNegativeScore,
		ValidationAlert:           validationAlert,
		EvidenceJSON:              marshalDealReviewLearnedPatternEvidence(agg.Evidence),
		FirstObservedAt:           agg.FirstObservedAt,
		LastObservedAt:            agg.LastObservedAt,
		BuiltAt:                   now,
	}
	pattern.FeatureSet = append([]string(nil), agg.FeatureSet...)
	pattern.Evidence = append([]DealReviewLearnedPatternEvidence(nil), agg.Evidence...)
	pattern.StableKey = dealReviewLearnedPatternStableKey(&pattern)
	pattern.Summary = buildDealReviewLearnedPatternSummaryText(&pattern)
	return pattern, true
}

func buildDealReviewPatternEvidenceRows(patterns []DealReviewLearnedPattern, now time.Time) []DealReviewPatternEvidenceRecord {
	if len(patterns) == 0 {
		return nil
	}
	rows := make([]DealReviewPatternEvidenceRecord, 0, len(patterns)*dealReviewLearnedPatternEvidenceLimit)
	for idx := range patterns {
		pattern := patterns[idx]
		evidence := pattern.Evidence
		if len(evidence) == 0 && strings.TrimSpace(pattern.EvidenceJSON) != "" {
			evidence = unmarshalDealReviewLearnedPatternEvidence(pattern.EvidenceJSON)
		}
		for evIdx := range evidence {
			item := evidence[evIdx]
			observedAt := parseDealReviewAggregateTime(item.ObservedAt)
			if observedAt.IsZero() {
				observedAt = pattern.LastObservedAt
			}
			if observedAt.IsZero() {
				observedAt = now
			}
			outcome := strings.TrimSpace(strings.ToLower(item.Outcome))
			if outcome == "" {
				outcome = classifyDealOutcome(item.RealizedPnL)
			}
			side := normalizeDealReviewSide(item.Side)
			if side == "" {
				side = normalizeDealReviewSide(pattern.Side)
			}
			rows = append(rows, DealReviewPatternEvidenceRecord{
				UserID:         pattern.UserID,
				TraderID:       pattern.TraderID,
				PatternID:      pattern.ID,
				EvidenceRank:   evIdx + 1,
				CaseID:         strings.TrimSpace(item.CaseID),
				PositionID:     item.PositionID,
				Symbol:         strings.ToUpper(strings.TrimSpace(item.Symbol)),
				Side:           side,
				Outcome:        outcome,
				RealizedPnL:    item.RealizedPnL,
				RealizedPnLPct: item.RealizedPnLPct,
				HoldDurationMs: item.HoldDurationMs,
				ObservedAt:     observedAt,
				BuiltAt:        now,
			})
		}
	}
	return rows
}

func buildDealReviewPatternBacklogCandidates(userID, traderID string, featureRows []DealReviewPatternFeatureRecord, patterns []DealReviewLearnedPattern, now time.Time) []DealReviewPatternBacklogCandidate {
	if len(featureRows) == 0 && len(patterns) == 0 {
		return nil
	}
	candidates := make([]DealReviewPatternBacklogCandidate, 0, 8)

	totalCases := maxInt(len(featureRows), 1)
	qualityFlagCounts := map[string]int{}
	for idx := range featureRows {
		flags := featureRows[idx].QualityFlags
		if len(flags) == 0 && strings.TrimSpace(featureRows[idx].QualityFlagsJSON) != "" {
			flags = unmarshalDealReviewStringArray(featureRows[idx].QualityFlagsJSON)
		}
		for _, flag := range flags {
			flag = normalizeDealReviewPatternFeatureToken(flag)
			if flag == "" {
				continue
			}
			qualityFlagCounts[flag]++
		}
	}

	type flagCount struct {
		flag  string
		count int
	}
	sortedFlags := make([]flagCount, 0, len(qualityFlagCounts))
	for flag, count := range qualityFlagCounts {
		sortedFlags = append(sortedFlags, flagCount{flag: flag, count: count})
	}
	sort.SliceStable(sortedFlags, func(i, j int) bool {
		if sortedFlags[i].count == sortedFlags[j].count {
			return sortedFlags[i].flag < sortedFlags[j].flag
		}
		return sortedFlags[i].count > sortedFlags[j].count
	})
	for idx := range sortedFlags {
		item := sortedFlags[idx]
		ratio := float64(item.count) / float64(totalCases)
		if ratio <= 0 {
			continue
		}
		candidates = append(candidates, DealReviewPatternBacklogCandidate{
			ID:              uuid.NewString(),
			UserID:          userID,
			TraderID:        traderID,
			Category:        DealReviewPatternBacklogCategoryMissingFeature,
			Title:           buildDealReviewPatternBacklogFlagTitle(item.flag),
			Rationale:       fmt.Sprintf("%d/%d learned-pattern cases are missing %s, which weakens regime-specific comparisons and optimizer follow-ups.", item.count, totalCases, strings.ReplaceAll(item.flag, "_", " ")),
			PriorityScore:   clampDealReviewUnit((ratio * 0.70) + 0.20),
			ConfidenceScore: clampDealReviewUnit(0.45 + (ratio * 0.55)),
			SupportCount:    item.count,
			MetadataJSON: marshalDealReviewPatternMetadata(map[string]any{
				"flag":           item.flag,
				"affected_cases": item.count,
				"total_cases":    totalCases,
				"coverage_gap":   ratio,
			}),
			BuiltAt: now,
		})
		if len(candidates) >= 4 {
			break
		}
	}

	weakValidation := make([]DealReviewLearnedPattern, 0, len(patterns))
	drifting := make([]DealReviewLearnedPattern, 0, len(patterns))
	contradictions := make([]DealReviewLearnedPattern, 0, len(patterns))
	for idx := range patterns {
		pattern := patterns[idx]
		label := normalizeDealReviewLearnedPatternValidationLabel(pattern.ValidationLabel)
		switch label {
		case DealReviewLearnedPatternValidationLabelCandidate, DealReviewLearnedPatternValidationLabelInsufficientEvidence:
			if pattern.CompositeScore >= 0.45 || math.Abs(pattern.LiftAvgPnLPct) >= 0.35 {
				weakValidation = append(weakValidation, pattern)
			}
		case DealReviewLearnedPatternValidationLabelDrifting, DealReviewLearnedPatternValidationLabelExpired:
			drifting = append(drifting, pattern)
		case DealReviewLearnedPatternValidationLabelFalsePositive, DealReviewLearnedPatternValidationLabelReverseRisk:
			contradictions = append(contradictions, pattern)
		}
	}

	appendPatternBacklog := func(category, title, rationale string, supportCount int, priority, confidence float64, topPatterns []DealReviewLearnedPattern) {
		metadata := map[string]any{
			"support_count": supportCount,
		}
		if len(topPatterns) > 0 {
			limit := len(topPatterns)
			if limit > 3 {
				limit = 3
			}
			signatures := make([]string, 0, limit)
			patternIDs := make([]string, 0, limit)
			for idx := 0; idx < limit; idx++ {
				signatures = append(signatures, topPatterns[idx].PatternSignature)
				patternIDs = append(patternIDs, topPatterns[idx].ID)
			}
			metadata["top_pattern_ids"] = patternIDs
			metadata["top_signatures"] = signatures
		}
		patternID := ""
		if len(topPatterns) > 0 {
			patternID = topPatterns[0].ID
		}
		candidates = append(candidates, DealReviewPatternBacklogCandidate{
			ID:              uuid.NewString(),
			UserID:          userID,
			TraderID:        traderID,
			PatternID:       patternID,
			Category:        category,
			Title:           title,
			Rationale:       rationale,
			PriorityScore:   clampDealReviewUnit(priority),
			ConfidenceScore: clampDealReviewUnit(confidence),
			SupportCount:    supportCount,
			MetadataJSON:    marshalDealReviewPatternMetadata(metadata),
			BuiltAt:         now,
		})
	}

	if len(weakValidation) > 0 {
		sort.SliceStable(weakValidation, func(i, j int) bool {
			if weakValidation[i].CompositeScore == weakValidation[j].CompositeScore {
				return weakValidation[i].SampleCount > weakValidation[j].SampleCount
			}
			return weakValidation[i].CompositeScore > weakValidation[j].CompositeScore
		})
		top := weakValidation[0]
		appendPatternBacklog(
			DealReviewPatternBacklogCategoryValidationWindow,
			"Collect stronger holdout evidence for promising patterns",
			fmt.Sprintf("%d learned pattern(s) still look directional but are not validated enough yet. Highest-priority example: %s on %s %s (samples %d, composite %.0f%%, lift %+.2f%%).", len(weakValidation), top.PatternSignature, dealReviewLearnedPatternScopeLabel(&top), top.Side, top.SampleCount, top.CompositeScore*100, top.LiftAvgPnLPct),
			len(weakValidation),
			0.42+(top.CompositeScore*0.35)+(clampDealReviewUnit(float64(len(weakValidation))/12.0)*0.23),
			0.55+(top.ValidationSupportScore*0.20)+(top.RecencyWeight*0.25),
			weakValidation,
		)
	}
	if len(drifting) > 0 {
		sort.SliceStable(drifting, func(i, j int) bool {
			if drifting[i].DriftScore == drifting[j].DriftScore {
				return drifting[i].CompositeScore > drifting[j].CompositeScore
			}
			return drifting[i].DriftScore > drifting[j].DriftScore
		})
		top := drifting[0]
		appendPatternBacklog(
			DealReviewPatternBacklogCategoryDriftMonitoring,
			"Re-validate drifting learned patterns before optimizer biasing",
			fmt.Sprintf("%d learned pattern(s) are drifting or expired. Highest-drift example: %s on %s %s (drift %.0f%%, recent support %d/%d).", len(drifting), top.PatternSignature, dealReviewLearnedPatternScopeLabel(&top), top.Side, top.DriftScore*100, top.RecentSupportCount, top.RecentSampleCount),
			len(drifting),
			0.40+(top.DriftScore*0.40)+(clampDealReviewUnit(float64(len(drifting))/10.0)*0.20),
			0.50+(top.DriftScore*0.50),
			drifting,
		)
	}
	if len(contradictions) > 0 {
		sort.SliceStable(contradictions, func(i, j int) bool {
			left := math.Max(contradictions[i].FalsePositiveScore, contradictions[i].ReverseRiskScore)
			right := math.Max(contradictions[j].FalsePositiveScore, contradictions[j].ReverseRiskScore)
			if left == right {
				return contradictions[i].CompositeScore > contradictions[j].CompositeScore
			}
			return left > right
		})
		top := contradictions[0]
		riskScore := math.Max(top.FalsePositiveScore, top.ReverseRiskScore)
		appendPatternBacklog(
			DealReviewPatternBacklogCategoryContradictionRisk,
			"Review contradiction-heavy setups for prompt or config gating",
			fmt.Sprintf("%d learned pattern(s) now behave like false positives or reverse-risk setups. Highest-risk example: %s on %s %s (risk %.0f%%, label %s).", len(contradictions), top.PatternSignature, dealReviewLearnedPatternScopeLabel(&top), top.Side, riskScore*100, top.ValidationLabel),
			len(contradictions),
			0.45+(riskScore*0.35)+(clampDealReviewUnit(float64(len(contradictions))/10.0)*0.20),
			0.55+(riskScore*0.45),
			contradictions,
		)
	}

	if len(candidates) > 8 {
		candidates = candidates[:8]
	}
	return candidates
}

func buildDealReviewPatternValidationRun(userID, traderID string, startedAt, completedAt time.Time, featureRows []DealReviewPatternFeatureRecord, patterns []DealReviewLearnedPattern, evidenceRows []DealReviewPatternEvidenceRecord, backlogCandidates []DealReviewPatternBacklogCandidate) *DealReviewPatternValidationRun {
	labelCounts := map[string]int{}
	classCounts := map[string]int{}
	qualityFlagCounts := map[string]int{}
	for idx := range patterns {
		labelKey := normalizeDealReviewLearnedPatternValidationLabel(patterns[idx].ValidationLabel)
		if labelKey == "" {
			labelKey = "unknown"
		}
		labelCounts[labelKey]++
		classKey := normalizeDealReviewLearnedPatternClass(patterns[idx].PatternClass)
		if classKey == "" {
			classKey = "unknown"
		}
		classCounts[classKey]++
	}
	for idx := range featureRows {
		flags := featureRows[idx].QualityFlags
		if len(flags) == 0 && strings.TrimSpace(featureRows[idx].QualityFlagsJSON) != "" {
			flags = unmarshalDealReviewStringArray(featureRows[idx].QualityFlagsJSON)
		}
		for _, flag := range flags {
			flag = normalizeDealReviewPatternFeatureToken(flag)
			if flag == "" {
				continue
			}
			qualityFlagCounts[flag]++
		}
	}
	summary := fmt.Sprintf(
		"Rebuilt %d learned pattern(s) from %d closed deal(s), materialized %d evidence rows, and surfaced %d backlog candidate(s).",
		len(patterns),
		len(featureRows),
		len(evidenceRows),
		len(backlogCandidates),
	)
	return &DealReviewPatternValidationRun{
		ID:                        uuid.NewString(),
		UserID:                    userID,
		TraderID:                  traderID,
		FeatureCount:              len(featureRows),
		PatternCount:              len(patterns),
		EvidenceCount:             len(evidenceRows),
		BacklogCandidateCount:     len(backlogCandidates),
		ConfirmedCount:            labelCounts[DealReviewLearnedPatternValidationLabelConfirmed],
		CandidateCount:            labelCounts[DealReviewLearnedPatternValidationLabelCandidate],
		InsufficientEvidenceCount: labelCounts[DealReviewLearnedPatternValidationLabelInsufficientEvidence],
		FalsePositiveCount:        labelCounts[DealReviewLearnedPatternValidationLabelFalsePositive],
		ReverseRiskCount:          labelCounts[DealReviewLearnedPatternValidationLabelReverseRisk],
		DriftingCount:             labelCounts[DealReviewLearnedPatternValidationLabelDrifting],
		ExpiredCount:              labelCounts[DealReviewLearnedPatternValidationLabelExpired],
		Summary:                   summary,
		MetadataJSON: marshalDealReviewPatternMetadata(map[string]any{
			"class_counts":        classCounts,
			"label_counts":        labelCounts,
			"quality_flag_counts": qualityFlagCounts,
		}),
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		BuiltAt:     completedAt,
	}
}

func (s *DealReviewStore) countLearnedPatternFeatures(userID, traderID string) (int, error) {
	var count int64
	if err := s.db.Model(&DealReviewPatternFeatureRecord{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID)).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *DealReviewStore) countLearnedPatterns(userID, traderID string) (int, error) {
	var count int64
	if err := s.db.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID)).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *DealReviewStore) latestLearnedPatternBuiltAt(userID, traderID string) (time.Time, error) {
	var latestBuiltRaw sql.NullString
	if err := s.db.Model(&DealReviewPatternValidationRun{}).
		Where("user_id = ? AND trader_id = ?", userID, traderID).
		Select("CAST(MAX(built_at) AS TEXT)").Scan(&latestBuiltRaw).Error; err != nil {
		return time.Time{}, err
	}
	latestBuilt := parseDealReviewAggregateTime(latestBuiltRaw.String)
	if !latestBuilt.IsZero() {
		return latestBuilt, nil
	}
	if err := s.db.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ?", userID, traderID).
		Select("CAST(MAX(built_at) AS TEXT)").Scan(&latestBuiltRaw).Error; err != nil {
		return time.Time{}, err
	}
	return parseDealReviewAggregateTime(latestBuiltRaw.String), nil
}

func (s *DealReviewStore) hydrateLearnedPatternEvidenceRows(items []DealReviewLearnedPattern) error {
	if len(items) == 0 {
		return nil
	}
	patternIDs := make([]string, 0, len(items))
	for idx := range items {
		if strings.TrimSpace(items[idx].ID) == "" {
			continue
		}
		patternIDs = append(patternIDs, items[idx].ID)
	}
	if len(patternIDs) == 0 {
		return nil
	}

	var rows []DealReviewPatternEvidenceRecord
	if err := s.db.Model(&DealReviewPatternEvidenceRecord{}).
		Where("pattern_id IN ?", patternIDs).
		Order("pattern_id ASC, evidence_rank ASC, observed_at DESC").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	byPatternID := make(map[string][]DealReviewLearnedPatternEvidence, len(patternIDs))
	for idx := range rows {
		row := rows[idx]
		byPatternID[row.PatternID] = append(byPatternID[row.PatternID], row.toLearnedPatternEvidence())
	}
	for idx := range items {
		if evidence, ok := byPatternID[items[idx].ID]; ok && len(evidence) > 0 {
			items[idx].Evidence = append([]DealReviewLearnedPatternEvidence(nil), evidence...)
		}
	}
	return nil
}

func extractDealReviewLearnedPatternFeatures(caseRec *DealReviewCase) ([]string, []string) {
	if caseRec == nil {
		return nil, nil
	}
	features := make([]string, 0, dealReviewLearnedPatternFeatureLimit)
	qualityFlags := make([]string, 0, 6)
	seen := map[string]struct{}{}
	appendFeature := func(prefix, raw string) {
		value := normalizeDealReviewPatternFeatureToken(raw)
		if value == "" || value == "unknown" {
			return
		}
		feature := prefix + ":" + value
		if _, ok := seen[feature]; ok {
			return
		}
		seen[feature] = struct{}{}
		features = append(features, feature)
	}
	appendFlag := func(flag string) {
		flag = normalizeDealReviewPatternFeatureToken(flag)
		if flag == "" {
			return
		}
		for _, current := range qualityFlags {
			if current == flag {
				return
			}
		}
		qualityFlags = append(qualityFlags, flag)
	}

	appendFeature("bucket", caseRec.OpenSelectionBucket)
	appendFeature("trend", caseRec.OpenTrendRegime)
	appendFeature("vol", caseRec.OpenVolatilityRegime)
	appendFeature("btc", caseRec.OpenBTCStrengthRegime)
	appendFeature("funding", caseRec.OpenFundingRegime)
	appendFeature("oi", caseRec.OpenOIRegime)
	appendFeature("session", caseRec.OpenSessionBucket)
	appendFeature("weekday", caseRec.OpenWeekdayBucket)
	appendFeature("venue", caseRec.OpenVenueTier)
	appendFeature("liq", caseRec.OpenLiquidityTier)
	appendFeature("spread", caseRec.OpenSpreadBucket)
	appendFeature("slip", caseRec.OpenSlippageBucket)

	switch {
	case caseRec.OpenConfidence >= 80:
		appendFeature("conf", "high")
	case caseRec.OpenConfidence >= 65:
		appendFeature("conf", "mid")
	case caseRec.OpenConfidence > 0:
		appendFeature("conf", "low")
	default:
		appendFlag("missing_confidence")
	}

	if riskBucket := bucketDealReviewPlannedRisk(caseRec.PlannedRiskPct); riskBucket != "" {
		appendFeature("risk", riskBucket)
	}
	for _, source := range unmarshalDealReviewStringArray(caseRec.OpenCandidateSourcesJSON) {
		appendFeature("src", source)
	}

	if normalizeDealReviewPatternFeatureToken(caseRec.OpenTrendRegime) == "" {
		appendFlag("missing_trend_regime")
	}
	if normalizeDealReviewPatternFeatureToken(caseRec.OpenVolatilityRegime) == "" {
		appendFlag("missing_volatility_regime")
	}
	if normalizeDealReviewPatternFeatureToken(caseRec.OpenOIRegime) == "" {
		appendFlag("missing_oi_regime")
	}
	if normalizeDealReviewPatternFeatureToken(caseRec.OpenSessionBucket) == "" {
		appendFlag("missing_session_bucket")
	}

	sort.Strings(features)
	if len(features) > dealReviewLearnedPatternFeatureLimit {
		features = features[:dealReviewLearnedPatternFeatureLimit]
	}
	sort.Strings(qualityFlags)
	return features, qualityFlags
}

func buildDealReviewLearnedPatternCombos(features []string) [][]string {
	if len(features) == 0 {
		return nil
	}
	limit := len(features)
	if limit > dealReviewLearnedPatternFeatureLimit {
		limit = dealReviewLearnedPatternFeatureLimit
	}
	trimmed := append([]string(nil), features[:limit]...)
	combos := make([][]string, 0, len(trimmed)+(len(trimmed)*(len(trimmed)-1))/2)
	for i := range trimmed {
		combos = append(combos, []string{trimmed[i]})
	}
	if dealReviewLearnedPatternMaxOrder >= 2 {
		for i := 0; i < len(trimmed); i++ {
			for j := i + 1; j < len(trimmed); j++ {
				combos = append(combos, []string{trimmed[i], trimmed[j]})
			}
		}
	}
	if dealReviewLearnedPatternMaxOrder >= 3 {
		triples := buildDealReviewLearnedPatternTripleCombos(trimmed)
		if len(triples) > 0 {
			combos = append(combos, triples...)
		}
	}
	return combos
}

func buildDealReviewLearnedPatternTripleCombos(features []string) [][]string {
	eligible := selectDealReviewLearnedPatternTripleFeatures(features)
	if len(eligible) < 3 {
		return nil
	}
	combos := make([][]string, 0, dealReviewLearnedPatternTripleComboLimit)
	for i := 0; i < len(eligible); i++ {
		for j := i + 1; j < len(eligible); j++ {
			for k := j + 1; k < len(eligible); k++ {
				combos = append(combos, []string{eligible[i], eligible[j], eligible[k]})
				if len(combos) >= dealReviewLearnedPatternTripleComboLimit {
					return combos
				}
			}
		}
	}
	return combos
}

func selectDealReviewLearnedPatternTripleFeatures(features []string) []string {
	if len(features) == 0 {
		return nil
	}
	trimmed := append([]string(nil), features...)
	sort.SliceStable(trimmed, func(i, j int) bool {
		leftPriority := dealReviewLearnedPatternTripleFeaturePriority(trimmed[i])
		rightPriority := dealReviewLearnedPatternTripleFeaturePriority(trimmed[j])
		if leftPriority == rightPriority {
			return trimmed[i] < trimmed[j]
		}
		return leftPriority < rightPriority
	})
	if len(trimmed) > dealReviewLearnedPatternTripleFeatureLimit {
		trimmed = trimmed[:dealReviewLearnedPatternTripleFeatureLimit]
	}
	sort.Strings(trimmed)
	return trimmed
}

func dealReviewLearnedPatternTripleFeaturePriority(feature string) int {
	switch {
	case strings.HasPrefix(feature, "bucket:"):
		return 0
	case strings.HasPrefix(feature, "trend:"):
		return 1
	case strings.HasPrefix(feature, "vol:"):
		return 2
	case strings.HasPrefix(feature, "oi:"):
		return 3
	case strings.HasPrefix(feature, "session:"):
		return 4
	case strings.HasPrefix(feature, "conf:"):
		return 5
	case strings.HasPrefix(feature, "risk:"):
		return 6
	case strings.HasPrefix(feature, "btc:"):
		return 7
	case strings.HasPrefix(feature, "funding:"):
		return 8
	case strings.HasPrefix(feature, "venue:"):
		return 9
	case strings.HasPrefix(feature, "liq:"):
		return 10
	case strings.HasPrefix(feature, "spread:"):
		return 11
	case strings.HasPrefix(feature, "slip:"):
		return 12
	case strings.HasPrefix(feature, "weekday:"):
		return 13
	case strings.HasPrefix(feature, "src:"):
		return 14
	default:
		return 15
	}
}

type dealReviewLearnedPatternScope struct {
	scopeType       string
	scopeKey        string
	symbol          string
	regimeSignature string
}

func buildDealReviewLearnedPatternScopes(caseRec *DealReviewCase, featureSet []string, options dealReviewLearnedPatternBuildOptions) []dealReviewLearnedPatternScope {
	if caseRec == nil {
		return nil
	}
	symbol := strings.ToUpper(strings.TrimSpace(caseRec.Symbol))
	out := make([]dealReviewLearnedPatternScope, 0, 4)
	if options.IncludeGlobalScope {
		out = append(out, dealReviewLearnedPatternScope{
			scopeType: DealReviewLearnedPatternScopeGlobal,
			scopeKey:  DealReviewLearnedPatternScopeGlobal,
			symbol:    "",
		})
	}
	if options.IncludeTraderLocalScope {
		out = append(out, dealReviewLearnedPatternScope{
			scopeType: DealReviewLearnedPatternScopeTraderLocal,
			scopeKey:  DealReviewLearnedPatternScopeTraderLocal,
			symbol:    "",
		})
	}
	regimeScopeKey := buildDealReviewLearnedPatternCaseRegimeScopeKey(featureSet)
	if options.IncludeRegimeLocalScope && regimeScopeKey != "" {
		out = append(out, dealReviewLearnedPatternScope{
			scopeType:       DealReviewLearnedPatternScopeRegimeLocal,
			scopeKey:        regimeScopeKey,
			symbol:          "",
			regimeSignature: regimeScopeKey,
		})
	}
	if options.IncludeSymbolScope && symbol != "" {
		out = append(out, dealReviewLearnedPatternScope{
			scopeType: DealReviewLearnedPatternScopeSymbol,
			scopeKey:  symbol,
			symbol:    symbol,
		})
	}
	return out
}

func buildDealReviewLearnedPatternRegimeSignature(features []string) string {
	if len(features) == 0 {
		return ""
	}
	selected := make([]string, 0, 4)
	for _, feature := range features {
		switch {
		case strings.HasPrefix(feature, "trend:"),
			strings.HasPrefix(feature, "vol:"),
			strings.HasPrefix(feature, "oi:"),
			strings.HasPrefix(feature, "funding:"),
			strings.HasPrefix(feature, "btc:"),
			strings.HasPrefix(feature, "session:"):
			selected = append(selected, feature)
		}
		if len(selected) >= 4 {
			break
		}
	}
	return strings.Join(selected, " | ")
}

func buildDealReviewLearnedPatternCaseRegimeScopeKey(features []string) string {
	if len(features) == 0 {
		return ""
	}
	preferredPrefixes := []string{
		"bucket:",
		"trend:",
		"vol:",
		"btc:",
		"funding:",
		"oi:",
		"session:",
		"weekday:",
		"venue:",
		"liq:",
		"spread:",
		"slip:",
		"risk:",
	}
	featureByPrefix := make(map[string]string, len(preferredPrefixes))
	for _, feature := range features {
		for _, prefix := range preferredPrefixes {
			if strings.HasPrefix(feature, prefix) {
				featureByPrefix[prefix] = feature
				break
			}
		}
	}
	selected := make([]string, 0, len(preferredPrefixes))
	for _, prefix := range preferredPrefixes {
		if feature := strings.TrimSpace(featureByPrefix[prefix]); feature != "" {
			selected = append(selected, feature)
		}
	}
	return strings.Join(selected, " | ")
}

func bucketDealReviewPlannedRisk(value float64) string {
	switch {
	case value <= 0:
		return ""
	case value <= 0.50:
		return "tight"
	case value <= 1.25:
		return "normal"
	case value <= 2.50:
		return "wide"
	default:
		return "aggressive"
	}
}

func normalizeDealReviewPatternFeatureToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return ""
	}
	return out
}

func normalizeDealReviewPatternFeatureFilterToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		prefix := normalizeDealReviewPatternFeatureToken(parts[0])
		suffix := normalizeDealReviewPatternFeatureToken(parts[1])
		if prefix == "" || suffix == "" {
			return ""
		}
		return prefix + ":" + suffix
	}
	normalized := normalizeDealReviewPatternFeatureToken(value)
	if normalized == "" {
		return ""
	}
	if idx := strings.Index(normalized, "_"); idx > 0 && idx < len(normalized)-1 {
		return normalized[:idx] + ":" + normalized[idx+1:]
	}
	return normalized
}

func normalizeDealReviewLearnedPatternScopeType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternScopeGlobal:
		return DealReviewLearnedPatternScopeGlobal
	case DealReviewLearnedPatternScopeTraderLocal:
		return DealReviewLearnedPatternScopeTraderLocal
	case DealReviewLearnedPatternScopeRegimeLocal:
		return DealReviewLearnedPatternScopeRegimeLocal
	case DealReviewLearnedPatternScopeSymbol:
		return DealReviewLearnedPatternScopeSymbol
	default:
		return ""
	}
}

func normalizeDealReviewLearnedPatternClass(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternClassPositiveEdge:
		return DealReviewLearnedPatternClassPositiveEdge
	case DealReviewLearnedPatternClassNegativeEdge:
		return DealReviewLearnedPatternClassNegativeEdge
	default:
		return ""
	}
}

func normalizeDealReviewLearnedPatternValidationLabel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternValidationLabelConfirmed:
		return DealReviewLearnedPatternValidationLabelConfirmed
	case DealReviewLearnedPatternValidationLabelCandidate:
		return DealReviewLearnedPatternValidationLabelCandidate
	case DealReviewLearnedPatternValidationLabelInsufficientEvidence:
		return DealReviewLearnedPatternValidationLabelInsufficientEvidence
	case DealReviewLearnedPatternValidationLabelFalsePositive:
		return DealReviewLearnedPatternValidationLabelFalsePositive
	case DealReviewLearnedPatternValidationLabelReverseRisk:
		return DealReviewLearnedPatternValidationLabelReverseRisk
	case DealReviewLearnedPatternValidationLabelDrifting:
		return DealReviewLearnedPatternValidationLabelDrifting
	case DealReviewLearnedPatternValidationLabelExpired:
		return DealReviewLearnedPatternValidationLabelExpired
	default:
		return ""
	}
}

func computeDealReviewLearnedPatternLiftStrength(bias string, liftWinRate, liftAvgPnLPct float64) float64 {
	switch strings.TrimSpace(bias) {
	case DealReviewSymbolBehaviorBiasPositive:
		return clampDealReviewUnit((clampDealReviewUnit(liftWinRate/0.25) * 0.45) + (clampDealReviewUnit(liftAvgPnLPct/1.5) * 0.55))
	case DealReviewSymbolBehaviorBiasNegative:
		return clampDealReviewUnit((clampDealReviewUnit((-liftWinRate)/0.25) * 0.45) + (clampDealReviewUnit(math.Abs(minFloat(liftAvgPnLPct, 0))/1.5) * 0.55))
	default:
		return 0
	}
}

func mapDealReviewLearnedPatternValidationLabel(value string) string {
	switch strings.TrimSpace(value) {
	case DealReviewSymbolBehaviorValidationLabelConfirmed:
		return DealReviewLearnedPatternValidationLabelConfirmed
	case DealReviewSymbolBehaviorValidationLabelCandidate:
		return DealReviewLearnedPatternValidationLabelCandidate
	case DealReviewSymbolBehaviorValidationLabelFalsePositive:
		return DealReviewLearnedPatternValidationLabelFalsePositive
	case DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk:
		return DealReviewLearnedPatternValidationLabelReverseRisk
	case DealReviewSymbolBehaviorValidationLabelDrifting:
		return DealReviewLearnedPatternValidationLabelDrifting
	case DealReviewSymbolBehaviorValidationLabelInsufficientEvidence:
		return DealReviewLearnedPatternValidationLabelInsufficientEvidence
	default:
		if strings.EqualFold(strings.TrimSpace(value), DealReviewSymbolBehaviorPriorStatusExpired) {
			return DealReviewLearnedPatternValidationLabelExpired
		}
		return DealReviewLearnedPatternValidationLabelCandidate
	}
}

func buildDealReviewLearnedPatternValidationAlert(patternClass, label string, falsePositiveScore, reverseRiskScore, driftScore float64) string {
	switch normalizeDealReviewLearnedPatternValidationLabel(label) {
	case DealReviewLearnedPatternValidationLabelConfirmed:
		return "Holdout and recent windows still support this learned pattern."
	case DealReviewLearnedPatternValidationLabelFalsePositive:
		return fmt.Sprintf("Holdout or recent windows contradicted this %s strongly enough that it now looks like a likely false positive (score %.0f%%).", patternClass, falsePositiveScore*100)
	case DealReviewLearnedPatternValidationLabelReverseRisk:
		return fmt.Sprintf("Recent evidence is leaning toward the opposite direction instead (score %.0f%%), so this pattern now behaves like a reverse-risk setup.", reverseRiskScore*100)
	case DealReviewLearnedPatternValidationLabelDrifting:
		return fmt.Sprintf("Recent evidence is drifting away from the earlier learned edge (drift %.0f%%).", driftScore*100)
	case DealReviewLearnedPatternValidationLabelExpired:
		return "This pattern is too stale to trust without fresh confirmation."
	case DealReviewLearnedPatternValidationLabelInsufficientEvidence:
		return "Not enough holdout evidence yet to judge whether this pattern is real."
	default:
		return "This pattern is directional, but its validation windows are still mixed."
	}
}

func buildDealReviewLearnedPatternRecommendedUse(patternClass, label string, composite, confidenceScore, validationSupportScore, falsePositiveScore, driftScore float64, sampleCount int, absLiftPnLPct float64) string {
	switch normalizeDealReviewLearnedPatternValidationLabel(label) {
	case DealReviewLearnedPatternValidationLabelExpired:
		return DealReviewLearnedPatternRecommendedUseExpiredIgnore
	case DealReviewLearnedPatternValidationLabelFalsePositive, DealReviewLearnedPatternValidationLabelDrifting:
		return DealReviewLearnedPatternRecommendedUseMonitorOnly
	case DealReviewLearnedPatternValidationLabelReverseRisk:
		return DealReviewLearnedPatternRecommendedUseReviewHint
	case DealReviewLearnedPatternValidationLabelConfirmed:
		if patternClass == DealReviewLearnedPatternClassPositiveEdge &&
			composite >= 0.78 &&
			sampleCount >= 8 &&
			absLiftPnLPct >= 0.45 {
			return DealReviewLearnedPatternRecommendedUseConfigCand
		}
		if isDealReviewLearnedPatternMonitoringRuleCandidate(
			patternClass,
			label,
			composite,
			confidenceScore,
			validationSupportScore,
			falsePositiveScore,
			driftScore,
			sampleCount,
			absLiftPnLPct,
		) {
			return DealReviewLearnedPatternRecommendedUseMonitoringRule
		}
		if patternClass == DealReviewLearnedPatternClassNegativeEdge {
			return DealReviewLearnedPatternRecommendedUsePromptHint
		}
		return DealReviewLearnedPatternRecommendedUseReviewHint
	default:
		return DealReviewLearnedPatternRecommendedUseReviewHint
	}
}

func normalizeDealReviewLearnedPatternRecommendedUse(value string) string {
	switch strings.TrimSpace(value) {
	case DealReviewLearnedPatternRecommendedUseReviewHint:
		return DealReviewLearnedPatternRecommendedUseReviewHint
	case DealReviewLearnedPatternRecommendedUsePromptHint:
		return DealReviewLearnedPatternRecommendedUsePromptHint
	case DealReviewLearnedPatternRecommendedUseConfigCand:
		return DealReviewLearnedPatternRecommendedUseConfigCand
	case DealReviewLearnedPatternRecommendedUseMonitoringRule:
		return DealReviewLearnedPatternRecommendedUseMonitoringRule
	case DealReviewLearnedPatternRecommendedUseMonitorOnly:
		return DealReviewLearnedPatternRecommendedUseMonitorOnly
	case DealReviewLearnedPatternRecommendedUseExpiredIgnore:
		return DealReviewLearnedPatternRecommendedUseExpiredIgnore
	default:
		return ""
	}
}

func isDealReviewLearnedPatternMonitoringRuleCandidate(patternClass, label string, composite, confidenceScore, validationSupportScore, falsePositiveScore, driftScore float64, sampleCount int, absLiftPnLPct float64) bool {
	if normalizeDealReviewLearnedPatternClass(patternClass) != DealReviewLearnedPatternClassNegativeEdge {
		return false
	}
	if normalizeDealReviewLearnedPatternValidationLabel(label) != DealReviewLearnedPatternValidationLabelConfirmed {
		return false
	}
	if composite < DefaultLearnedPatternLiveGuardMinCompositeScore {
		return false
	}
	if confidenceScore < DefaultLearnedPatternLiveGuardMinConfidenceScore {
		return false
	}
	if validationSupportScore < DefaultLearnedPatternLiveGuardMinValidationSupportScore {
		return false
	}
	if sampleCount < DefaultLearnedPatternLiveGuardMinSampleCount {
		return false
	}
	if absLiftPnLPct < dealReviewLearnedPatternMinActionableLiftPnLPct {
		return false
	}
	if falsePositiveScore > DefaultLearnedPatternLiveGuardMaxFalsePositiveScore {
		return false
	}
	if driftScore > DefaultLearnedPatternLiveGuardMaxDriftScore {
		return false
	}
	return true
}

func buildDealReviewLearnedPatternSummaryText(pattern *DealReviewLearnedPattern) string {
	if pattern == nil {
		return ""
	}
	baselineDelta := pattern.LiftAvgPnLPct
	scopeLabel := "trader-local"
	switch normalizeDealReviewLearnedPatternScopeType(pattern.ScopeType) {
	case DealReviewLearnedPatternScopeGlobal:
		scopeLabel = "global"
	case DealReviewLearnedPatternScopeRegimeLocal:
		scopeLabel = blankToValue(strings.TrimSpace(pattern.RegimeSignature), "regime-local")
	case DealReviewLearnedPatternScopeSymbol:
		if strings.TrimSpace(pattern.Symbol) != "" {
			scopeLabel = pattern.Symbol
		}
	}
	return fmt.Sprintf(
		"%s on %s %s matched %d/%d supporting cases (avg %+.2f%%, lift %+.2f%% vs baseline). Holdout %d/%d, recent %d/%d, drift %.0f%%.",
		pattern.PatternSignature,
		scopeLabel,
		pattern.Side,
		pattern.SupportCount,
		pattern.SampleCount,
		pattern.AvgPnLPct,
		baselineDelta,
		pattern.ValidationSupportCount,
		pattern.ValidationSampleCount,
		pattern.RecentSupportCount,
		pattern.RecentSampleCount,
		pattern.DriftScore*100,
	)
}

func dealReviewPatternFeatureSubset(lookup map[string]struct{}, features []string) bool {
	if len(features) == 0 {
		return false
	}
	for _, feature := range features {
		if _, ok := lookup[feature]; !ok {
			return false
		}
	}
	return true
}

func scoreDealReviewLearnedPatternMatch(caseRec *DealReviewCase, caseFeatures []string, pattern *DealReviewLearnedPattern) float64 {
	if caseRec == nil || pattern == nil || len(caseFeatures) == 0 || len(pattern.FeatureSet) == 0 {
		return 0
	}
	score := 0.35
	score += clampDealReviewUnit(float64(len(pattern.FeatureSet))/float64(maxInt(len(caseFeatures), 1))) * 0.20
	score += clampDealReviewUnit(pattern.CompositeScore) * 0.20
	score += clampDealReviewUnit(pattern.ConfidenceScore) * 0.15
	score += clampDealReviewUnit(pattern.ValidationSupportScore) * 0.10
	switch normalizeDealReviewLearnedPatternScopeType(pattern.ScopeType) {
	case DealReviewLearnedPatternScopeSymbol:
		if strings.EqualFold(strings.TrimSpace(caseRec.Symbol), strings.TrimSpace(pattern.Symbol)) {
			score += 0.10
		}
	case DealReviewLearnedPatternScopeRegimeLocal:
		score += 0.08
	case DealReviewLearnedPatternScopeTraderLocal:
		score += 0.04
	case DealReviewLearnedPatternScopeGlobal:
		score += 0.02
	}
	if pattern.PatternOrder >= 3 {
		score += 0.08
	} else if pattern.PatternOrder >= 2 {
		score += 0.05
	}
	return clampDealReviewUnit(score)
}

func dealReviewLearnedPatternForSummaryCard(item DealReviewLearnedPattern) DealReviewLearnedPattern {
	item.Evidence = nil
	item.FeatureSet = append([]string(nil), item.FeatureSet...)
	return item
}

func limitDealReviewLearnedPatternSummaryItems(items []DealReviewLearnedPattern, limit int) []DealReviewLearnedPattern {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]DealReviewLearnedPattern, 0, len(items))
	for _, item := range items {
		out = append(out, dealReviewLearnedPatternForSummaryCard(item))
	}
	return out
}

func hydrateDealReviewLearnedPattern(item *DealReviewLearnedPattern) {
	if item == nil {
		return
	}
	item.FeatureSet = unmarshalDealReviewStringArray(item.FeatureSetJSON)
	item.Evidence = unmarshalDealReviewLearnedPatternEvidence(item.EvidenceJSON)
	if strings.TrimSpace(item.StableKey) == "" {
		item.StableKey = dealReviewLearnedPatternStableKey(item)
	}
	hydrateDealReviewLearnedPatternManualControls(item)
}

func dealReviewLearnedPatternStableKey(item *DealReviewLearnedPattern) string {
	if item == nil {
		return ""
	}
	features := append([]string(nil), item.FeatureSet...)
	if len(features) == 0 && strings.TrimSpace(item.FeatureSetJSON) != "" {
		features = unmarshalDealReviewStringArray(item.FeatureSetJSON)
	}
	normalizedFeatures := make([]string, 0, len(features))
	for _, feature := range features {
		token := normalizeDealReviewPatternFeatureToken(feature)
		if token == "" {
			continue
		}
		normalizedFeatures = append(normalizedFeatures, token)
	}
	sort.Strings(normalizedFeatures)
	canonical := strings.Join([]string{
		normalizeDealReviewLearnedPatternScopeType(item.ScopeType),
		strings.ToLower(strings.TrimSpace(item.ScopeKey)),
		strings.ToUpper(strings.TrimSpace(item.Symbol)),
		normalizeDealReviewSide(item.Side),
		normalizeDealReviewLearnedPatternClass(item.PatternClass),
		strings.ToLower(strings.TrimSpace(item.PatternSignature)),
		strings.ToLower(strings.TrimSpace(item.RegimeSignature)),
		strconv.Itoa(item.PatternOrder),
		strings.Join(normalizedFeatures, "|"),
	}, "||")
	if strings.Trim(canonical, "|") == "" {
		return ""
	}
	sum := sha1.Sum([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func (s *DealReviewStore) backfillLearnedPatternStableKeys(userID, traderID string) error {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" {
		return nil
	}

	var items []DealReviewLearnedPattern
	if err := s.db.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ? AND (stable_key = '' OR stable_key IS NULL)", userID, traderID).
		Find(&items).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for idx := range items {
			hydrateDealReviewLearnedPattern(&items[idx])
			if strings.TrimSpace(items[idx].StableKey) == "" {
				continue
			}
			if err := tx.Model(&DealReviewLearnedPattern{}).
				Where("id = ?", strings.TrimSpace(items[idx].ID)).
				Update("stable_key", items[idx].StableKey).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func unmarshalDealReviewLearnedPatternEvidence(raw string) []DealReviewLearnedPatternEvidence {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var items []DealReviewLearnedPatternEvidence
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}
	return items
}

func marshalDealReviewLearnedPatternEvidence(items []DealReviewLearnedPatternEvidence) string {
	body, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(body)
}

func marshalDealReviewPatternMetadata(payload map[string]any) string {
	if len(payload) == 0 {
		return "{}"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(body)
}

func (r DealReviewPatternEvidenceRecord) toLearnedPatternEvidence() DealReviewLearnedPatternEvidence {
	item := DealReviewLearnedPatternEvidence{
		CaseID:         strings.TrimSpace(r.CaseID),
		PositionID:     r.PositionID,
		Symbol:         strings.ToUpper(strings.TrimSpace(r.Symbol)),
		Side:           normalizeDealReviewSide(r.Side),
		Outcome:        strings.TrimSpace(strings.ToLower(r.Outcome)),
		RealizedPnL:    r.RealizedPnL,
		RealizedPnLPct: r.RealizedPnLPct,
		HoldDurationMs: r.HoldDurationMs,
	}
	if !r.ObservedAt.IsZero() {
		item.ObservedAt = r.ObservedAt.UTC().Format(time.RFC3339)
	}
	return item
}

func buildDealReviewPatternBacklogFlagTitle(flag string) string {
	switch normalizeDealReviewPatternFeatureToken(flag) {
	case "missing_trend_regime":
		return "Persist trend regime more consistently"
	case "missing_volatility_regime":
		return "Persist volatility regime more consistently"
	case "missing_oi_regime":
		return "Persist open-interest regime more consistently"
	case "missing_session_bucket":
		return "Persist session bucket more consistently"
	case "missing_confidence":
		return "Persist open-confidence snapshots more consistently"
	default:
		return fmt.Sprintf("Improve %s coverage in learned-pattern inputs", strings.ReplaceAll(strings.TrimSpace(flag), "_", " "))
	}
}

func dealReviewLearnedPatternScopeLabel(pattern *DealReviewLearnedPattern) string {
	if pattern == nil {
		return "trader_local"
	}
	switch normalizeDealReviewLearnedPatternScopeType(pattern.ScopeType) {
	case DealReviewLearnedPatternScopeGlobal:
		return "global"
	case DealReviewLearnedPatternScopeRegimeLocal:
		return blankToValue(strings.TrimSpace(pattern.RegimeSignature), "regime_local")
	case DealReviewLearnedPatternScopeSymbol:
		return strings.ToUpper(strings.TrimSpace(pattern.Symbol))
	}
	return "trader_local"
}

func safeAvgDealReviewPattern(sum float64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return sum / float64(count)
}
