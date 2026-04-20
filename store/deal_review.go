package store

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DealReviewStageOpen  = "open"
	DealReviewStageClose = "close"

	DealReviewEventSourceAIDecision = "ai_decision"
	DealReviewEventSourceSync       = "sync_position"

	DealReviewEventStatusPending  = "pending"
	DealReviewEventStatusLinked   = "linked"
	DealReviewEventStatusCanceled = "canceled"

	DealReviewCaseStatusOpen   = "OPEN"
	DealReviewCaseStatusClosed = "CLOSED"

	DealReviewAIScanValidationPending = "pending"
	DealReviewAIScanValidationPassed  = "passed"
	DealReviewAIScanValidationFailed  = "failed"

	DealReviewChallengerModePaper        = "paper"
	DealReviewChallengerModeIsolatedLive = "isolated_live"
	DealReviewChallengerModeSharedLive   = "shared_live"

	DealReviewChallengerStatusStarting  = "starting"
	DealReviewChallengerStatusRunning   = "running"
	DealReviewChallengerStatusCompleted = "completed"
	DealReviewChallengerStatusStopped   = "stopped"
	DealReviewChallengerStatusFailed    = "failed"

	DealReviewClassifierHeuristic       = "heuristic_label_memory"
	DealReviewClassifierAIAssist        = "ai_review_assist"
	DealReviewClassifierVerdictAccepted = "accepted"
	DealReviewClassifierVerdictRejected = "rejected"

	DealReviewExitOriginAIDecision           = "ai_decision"
	DealReviewExitOriginSyncedTriggerOrder   = "synced_trigger_order"
	DealReviewExitOriginSyncedMarketOrder    = "synced_market_order"
	DealReviewExitOriginSyncedCloseFill      = "synced_close_fill"
	DealReviewExitOriginTargetProximity      = "target_proximity"
	DealReviewExitOriginTrailingEngine       = "trailing_engine"
	DealReviewExitOriginManualUIClose        = "manual_ui_close"
	DealReviewExitOriginRiskGuard            = "risk_guard"
	DealReviewExitOriginInternalExitIntent   = "internal_exit_intent"
	DealReviewExitOriginStoredPositionReason = "stored_position_reason"
	DealReviewExitOriginExchangeSyncUnknown  = "exchange_sync_unknown"

	DealReviewExitReasonQualityExplicit       = "explicit"
	DealReviewExitReasonQualityHighConfidence = "high_confidence_inferred"
	DealReviewExitReasonQualityLowConfidence  = "low_confidence_inferred"

	DealReviewExitIntentStatusSubmitted = "submitted"
	DealReviewExitIntentStatusLinked    = "linked"
	DealReviewExitIntentStatusCanceled  = "canceled"

	DealReviewExitIntentTypeAICloseDecision = "ai_close_decision"
	DealReviewExitIntentTypeManualUIClose   = "manual_ui_close"
	DealReviewExitIntentTypeDrawdownGuard   = "drawdown_guard_exit"
	DealReviewExitIntentTypeGridEmergency   = "grid_emergency_exit"
	DealReviewExitIntentTypeGridDecision    = "grid_decision_close"
	DealReviewExitIntentTypeGridBreakout    = "grid_breakout_close_all"
	DealReviewExitIntentTypeGridLevelStop   = "grid_level_stop_loss"
)

type DealReviewStore struct {
	db *gorm.DB
}

type DealReviewCase struct {
	ID                       string                  `gorm:"primaryKey" json:"id"`
	UserID                   string                  `gorm:"column:user_id;not null;index:idx_deal_review_cases_user_time" json:"user_id"`
	TraderID                 string                  `gorm:"column:trader_id;not null;index:idx_deal_review_cases_trader_time" json:"trader_id"`
	PositionID               int64                   `gorm:"column:position_id;not null;uniqueIndex:idx_deal_review_cases_position" json:"position_id"`
	ExchangeID               string                  `gorm:"column:exchange_id;default:''" json:"exchange_id"`
	ExchangeType             string                  `gorm:"column:exchange_type;default:''" json:"exchange_type"`
	AIModelID                string                  `gorm:"column:ai_model_id;default:''" json:"ai_model_id"`
	StrategyID               string                  `gorm:"column:strategy_id;default:''" json:"strategy_id"`
	Symbol                   string                  `gorm:"column:symbol;not null;index:idx_deal_review_cases_symbol" json:"symbol"`
	Side                     string                  `gorm:"column:side;not null;index:idx_deal_review_cases_side" json:"side"`
	Status                   string                  `gorm:"column:status;default:OPEN;index:idx_deal_review_cases_status" json:"status"`
	Outcome                  string                  `gorm:"column:outcome;default:open;index:idx_deal_review_cases_outcome" json:"outcome"`
	OpenEventID              string                  `gorm:"column:open_event_id;default:''" json:"open_event_id"`
	CloseEventID             string                  `gorm:"column:close_event_id;default:''" json:"close_event_id"`
	OpenCycleNumber          int                     `gorm:"column:open_cycle_number;default:0" json:"open_cycle_number"`
	CloseCycleNumber         int                     `gorm:"column:close_cycle_number;default:0" json:"close_cycle_number"`
	EntryOrderID             string                  `gorm:"column:entry_order_id;default:''" json:"entry_order_id"`
	ExitOrderID              string                  `gorm:"column:exit_order_id;default:''" json:"exit_order_id"`
	EntryTimeMs              int64                   `gorm:"column:entry_time_ms;default:0" json:"entry_time_ms"`
	ExitTimeMs               int64                   `gorm:"column:exit_time_ms;default:0" json:"exit_time_ms"`
	EntryPrice               float64                 `gorm:"column:entry_price;default:0" json:"entry_price"`
	ExitPrice                float64                 `gorm:"column:exit_price;default:0" json:"exit_price"`
	EntryQuantity            float64                 `gorm:"column:entry_quantity;default:0" json:"entry_quantity"`
	ExitQuantity             float64                 `gorm:"column:exit_quantity;default:0" json:"exit_quantity"`
	Leverage                 int                     `gorm:"column:leverage;default:1" json:"leverage"`
	OpenStopLoss             float64                 `gorm:"column:open_stop_loss;default:0" json:"open_stop_loss"`
	OpenTakeProfit           float64                 `gorm:"column:open_take_profit;default:0" json:"open_take_profit"`
	OpenConfidence           int                     `gorm:"column:open_confidence;default:0" json:"open_confidence"`
	CloseConfidence          int                     `gorm:"column:close_confidence;default:0" json:"close_confidence"`
	OpenSelectionBucket      string                  `gorm:"column:open_selection_bucket;default:''" json:"open_selection_bucket"`
	OpenTrendRegime          string                  `gorm:"column:open_trend_regime;default:''" json:"open_trend_regime"`
	OpenVolatilityRegime     string                  `gorm:"column:open_volatility_regime;default:''" json:"open_volatility_regime"`
	OpenBTCStrengthRegime    string                  `gorm:"column:open_btc_strength_regime;default:''" json:"open_btc_strength_regime"`
	OpenFundingRegime        string                  `gorm:"column:open_funding_regime;default:''" json:"open_funding_regime"`
	OpenOIRegime             string                  `gorm:"column:open_oi_regime;default:''" json:"open_oi_regime"`
	OpenSessionBucket        string                  `gorm:"column:open_session_bucket;default:''" json:"open_session_bucket"`
	OpenWeekdayBucket        string                  `gorm:"column:open_weekday_bucket;default:''" json:"open_weekday_bucket"`
	OpenVenueTier            string                  `gorm:"column:open_venue_tier;default:''" json:"open_venue_tier"`
	OpenLiquidityTier        string                  `gorm:"column:open_liquidity_tier;default:''" json:"open_liquidity_tier"`
	OpenSpreadBucket         string                  `gorm:"column:open_spread_bucket;default:''" json:"open_spread_bucket"`
	OpenSlippageBucket       string                  `gorm:"column:open_slippage_bucket;default:''" json:"open_slippage_bucket"`
	CloseTrendRegime         string                  `gorm:"column:close_trend_regime;default:''" json:"close_trend_regime"`
	CloseVolatilityRegime    string                  `gorm:"column:close_volatility_regime;default:''" json:"close_volatility_regime"`
	CloseBTCStrengthRegime   string                  `gorm:"column:close_btc_strength_regime;default:''" json:"close_btc_strength_regime"`
	CloseFundingRegime       string                  `gorm:"column:close_funding_regime;default:''" json:"close_funding_regime"`
	CloseOIRegime            string                  `gorm:"column:close_oi_regime;default:''" json:"close_oi_regime"`
	CloseSessionBucket       string                  `gorm:"column:close_session_bucket;default:''" json:"close_session_bucket"`
	CloseWeekdayBucket       string                  `gorm:"column:close_weekday_bucket;default:''" json:"close_weekday_bucket"`
	CloseVenueTier           string                  `gorm:"column:close_venue_tier;default:''" json:"close_venue_tier"`
	CloseLiquidityTier       string                  `gorm:"column:close_liquidity_tier;default:''" json:"close_liquidity_tier"`
	CloseSpreadBucket        string                  `gorm:"column:close_spread_bucket;default:''" json:"close_spread_bucket"`
	CloseSlippageBucket      string                  `gorm:"column:close_slippage_bucket;default:''" json:"close_slippage_bucket"`
	OpenCandidateSourcesJSON string                  `gorm:"column:open_candidate_sources_json;type:text;default:'[]'" json:"-"`
	LabelsJSON               string                  `gorm:"column:labels_json;type:text;default:'[]'" json:"-"`
	AnalystNote              string                  `gorm:"column:analyst_note;type:text;default:''" json:"analyst_note"`
	MaxFavorableExcursion    float64                 `gorm:"column:max_favorable_excursion;default:0" json:"max_favorable_excursion"`
	MaxFavorableExcursionPct float64                 `gorm:"column:max_favorable_excursion_pct;default:0" json:"max_favorable_excursion_pct"`
	MaxAdverseExcursion      float64                 `gorm:"column:max_adverse_excursion;default:0" json:"max_adverse_excursion"`
	MaxAdverseExcursionPct   float64                 `gorm:"column:max_adverse_excursion_pct;default:0" json:"max_adverse_excursion_pct"`
	MFECapturedPct           float64                 `gorm:"column:mfe_captured_pct;default:0" json:"mfe_captured_pct"`
	ProfitGivenBack          float64                 `gorm:"column:profit_given_back;default:0" json:"profit_given_back"`
	ProfitGivenBackPct       float64                 `gorm:"column:profit_given_back_pct;default:0" json:"profit_given_back_pct"`
	TimeToFirstProfitMs      int64                   `gorm:"column:time_to_first_profit_ms;default:0" json:"time_to_first_profit_ms"`
	TimeToMaxDrawdownMs      int64                   `gorm:"column:time_to_max_drawdown_ms;default:0" json:"time_to_max_drawdown_ms"`
	PlannedRiskPct           float64                 `gorm:"column:planned_risk_pct;default:0" json:"planned_risk_pct"`
	ExitEfficiencyScore      float64                 `gorm:"column:exit_efficiency_score;default:0" json:"exit_efficiency_score"`
	EntryTimingScore         float64                 `gorm:"column:entry_timing_score;default:0" json:"entry_timing_score"`
	RiskSizingScore          float64                 `gorm:"column:risk_sizing_score;default:0" json:"risk_sizing_score"`
	RealizedPnL              float64                 `gorm:"column:realized_pnl;default:0" json:"realized_pnl"`
	RealizedPnLPct           float64                 `gorm:"column:realized_pnl_pct;default:0" json:"realized_pnl_pct"`
	Fee                      float64                 `gorm:"column:fee;default:0" json:"fee"`
	HoldDurationMs           int64                   `gorm:"column:hold_duration_ms;default:0" json:"hold_duration_ms"`
	CloseReason              string                  `gorm:"column:close_reason;default:''" json:"close_reason"`
	ExitOrigin               string                  `gorm:"column:exit_origin;default:''" json:"exit_origin"`
	ExitReasonQuality        string                  `gorm:"column:exit_reason_quality;default:''" json:"exit_reason_quality"`
	ExitEvidenceSummary      string                  `gorm:"column:exit_evidence_summary;type:text;default:''" json:"exit_evidence_summary"`
	ExitEvidenceJSON         string                  `gorm:"column:exit_evidence_json;type:text;default:'{}'" json:"-"`
	ExitEvidence             *DealReviewExitEvidence `gorm:"-" json:"exit_evidence,omitempty"`
	CreatedAt                time.Time               `json:"created_at"`
	UpdatedAt                time.Time               `json:"updated_at"`
}

func (DealReviewCase) TableName() string { return "deal_review_cases" }

type DealReviewEvent struct {
	ID                   string                  `gorm:"primaryKey" json:"id"`
	UserID               string                  `gorm:"column:user_id;not null;index:idx_deal_review_events_user_time" json:"user_id"`
	TraderID             string                  `gorm:"column:trader_id;not null;index:idx_deal_review_events_trader_time" json:"trader_id"`
	DealID               string                  `gorm:"column:deal_id;default:'';index:idx_deal_review_events_deal" json:"deal_id"`
	PositionID           int64                   `gorm:"column:position_id;default:0;index:idx_deal_review_events_position" json:"position_id"`
	Stage                string                  `gorm:"column:stage;not null;index:idx_deal_review_events_stage" json:"stage"`
	Source               string                  `gorm:"column:source;default:ai_decision" json:"source"`
	Status               string                  `gorm:"column:status;default:pending;index:idx_deal_review_events_status" json:"status"`
	DecisionCycleNumber  int                     `gorm:"column:decision_cycle_number;default:0" json:"decision_cycle_number"`
	DecisionTimestamp    time.Time               `gorm:"column:decision_timestamp" json:"decision_timestamp"`
	ExchangeID           string                  `gorm:"column:exchange_id;default:''" json:"exchange_id"`
	ExchangeOrderID      string                  `gorm:"column:exchange_order_id;default:'';index:idx_deal_review_events_order" json:"exchange_order_id"`
	Symbol               string                  `gorm:"column:symbol;not null;index:idx_deal_review_events_symbol" json:"symbol"`
	Side                 string                  `gorm:"column:side;not null" json:"side"`
	Action               string                  `gorm:"column:action;default:''" json:"action"`
	Quantity             float64                 `gorm:"column:quantity;default:0" json:"quantity"`
	PositionSizeUSD      float64                 `gorm:"column:position_size_usd;default:0" json:"position_size_usd"`
	Price                float64                 `gorm:"column:price;default:0" json:"price"`
	Leverage             int                     `gorm:"column:leverage;default:0" json:"leverage"`
	StopLoss             float64                 `gorm:"column:stop_loss;default:0" json:"stop_loss"`
	TakeProfit           float64                 `gorm:"column:take_profit;default:0" json:"take_profit"`
	Confidence           int                     `gorm:"column:confidence;default:0" json:"confidence"`
	Reasoning            string                  `gorm:"column:reasoning;type:text;default:''" json:"reasoning"`
	SelectionBucket      string                  `gorm:"column:selection_bucket;default:''" json:"selection_bucket"`
	CandidateSourcesJSON string                  `gorm:"column:candidate_sources_json;type:text;default:'[]'" json:"-"`
	SnapshotJSON         string                  `gorm:"column:snapshot_json;type:text;default:'{}'" json:"-"`
	OutcomePnL           float64                 `gorm:"column:outcome_pnl;default:0" json:"outcome_pnl"`
	OutcomePnLPct        float64                 `gorm:"column:outcome_pnl_pct;default:0" json:"outcome_pnl_pct"`
	CloseReason          string                  `gorm:"column:close_reason;default:''" json:"close_reason"`
	ExitOrigin           string                  `gorm:"column:exit_origin;default:''" json:"exit_origin"`
	ExitReasonQuality    string                  `gorm:"column:exit_reason_quality;default:''" json:"exit_reason_quality"`
	ExitEvidenceSummary  string                  `gorm:"column:exit_evidence_summary;type:text;default:''" json:"exit_evidence_summary"`
	ExitEvidenceJSON     string                  `gorm:"column:exit_evidence_json;type:text;default:'{}'" json:"-"`
	ExitEvidence         *DealReviewExitEvidence `gorm:"-" json:"exit_evidence,omitempty"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

func (DealReviewEvent) TableName() string { return "deal_review_events" }

type DealReviewAIScan struct {
	ID                string    `gorm:"primaryKey" json:"id"`
	UserID            string    `gorm:"column:user_id;not null;index:idx_deal_review_ai_scans_user_time" json:"user_id"`
	TraderID          string    `gorm:"column:trader_id;not null;index:idx_deal_review_ai_scans_trader_time" json:"trader_id"`
	StrategyID        string    `gorm:"column:strategy_id;default:''" json:"strategy_id"`
	ModelConfigID     string    `gorm:"column:model_config_id;default:''" json:"model_config_id"`
	Provider          string    `gorm:"column:provider;default:''" json:"provider"`
	ModelName         string    `gorm:"column:model_name;default:''" json:"model_name"`
	DatasetCount      int       `gorm:"column:dataset_count;default:0" json:"dataset_count"`
	FilterJSON        string    `gorm:"column:filter_json;type:text;default:'{}'" json:"-"`
	ResultJSON        string    `gorm:"column:result_json;type:text;default:'{}'" json:"-"`
	StrategyPatchJSON string    `gorm:"column:strategy_patch_json;type:text;default:'{}'" json:"-"`
	Status            string    `gorm:"column:status;default:completed" json:"status"`
	Summary           string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	ErrorMessage      string    `gorm:"column:error_message;type:text;default:''" json:"error_message"`
	ValidationStatus  string    `gorm:"column:validation_status;default:pending;index:idx_deal_review_ai_scans_validation" json:"validation_status"`
	ValidationSummary string    `gorm:"column:validation_summary;type:text;default:''" json:"validation_summary"`
	ValidationJSON    string    `gorm:"column:validation_json;type:text;default:'{}'" json:"-"`
	ValidatedAt       time.Time `gorm:"column:validated_at" json:"validated_at"`
	AppliedAt         time.Time `gorm:"column:applied_at" json:"applied_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (DealReviewAIScan) TableName() string { return "deal_review_ai_scans" }

type DealReviewStrategyVersion struct {
	ID                 string    `gorm:"primaryKey" json:"id"`
	UserID             string    `gorm:"column:user_id;not null;index:idx_deal_review_strategy_versions_user_time" json:"user_id"`
	TraderID           string    `gorm:"column:trader_id;not null;index:idx_deal_review_strategy_versions_trader_time" json:"trader_id"`
	StrategyID         string    `gorm:"column:strategy_id;not null;index:idx_deal_review_strategy_versions_strategy" json:"strategy_id"`
	SourceScanID       string    `gorm:"column:source_scan_id;default:'';index:idx_deal_review_strategy_versions_scan" json:"source_scan_id"`
	SourceCompareID    string    `gorm:"column:source_compare_id;default:'';index:idx_deal_review_strategy_versions_compare" json:"source_compare_id"`
	SourceType         string    `gorm:"column:source_type;default:'ai_apply'" json:"source_type"`
	Summary            string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	ExpectedEffect     string    `gorm:"column:expected_effect;type:text;default:''" json:"expected_effect"`
	TargetCohortJSON   string    `gorm:"column:target_cohort_json;type:text;default:'{}'" json:"-"`
	PreviousConfigJSON string    `gorm:"column:previous_config_json;type:text;default:'{}'" json:"-"`
	NextConfigJSON     string    `gorm:"column:next_config_json;type:text;default:'{}'" json:"-"`
	AppliedAt          time.Time `gorm:"column:applied_at;index:idx_deal_review_strategy_versions_applied" json:"applied_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (DealReviewStrategyVersion) TableName() string { return "deal_review_strategy_versions" }

type DealReviewChallengerCompare struct {
	ID                      string    `gorm:"primaryKey" json:"id"`
	UserID                  string    `gorm:"column:user_id;not null;index:idx_deal_review_challenger_compares_user_time" json:"user_id"`
	TraderID                string    `gorm:"column:trader_id;not null;index:idx_deal_review_challenger_compares_trader_time" json:"trader_id"`
	IncumbentTraderID       string    `gorm:"column:incumbent_trader_id;not null;index:idx_deal_review_challenger_compares_incumbent" json:"incumbent_trader_id"`
	IncumbentStrategyID     string    `gorm:"column:incumbent_strategy_id;default:''" json:"incumbent_strategy_id"`
	ChallengerTraderID      string    `gorm:"column:challenger_trader_id;default:'';index:idx_deal_review_challenger_compares_challenger" json:"challenger_trader_id"`
	ChallengerStrategyID    string    `gorm:"column:challenger_strategy_id;default:''" json:"challenger_strategy_id"`
	ChallengerExchangeID    string    `gorm:"column:challenger_exchange_id;default:''" json:"challenger_exchange_id"`
	Mode                    string    `gorm:"column:mode;default:shared_live" json:"mode"`
	WindowHours             int       `gorm:"column:window_hours;default:24" json:"window_hours"`
	ExtensionCount          int       `gorm:"column:extension_count;default:0" json:"extension_count"`
	SourceScanID            string    `gorm:"column:source_scan_id;default:'';index:idx_deal_review_challenger_compares_scan" json:"source_scan_id"`
	SourceStrategyVersionID string    `gorm:"column:source_strategy_version_id;default:''" json:"source_strategy_version_id"`
	Status                  string    `gorm:"column:status;default:starting;index:idx_deal_review_challenger_compares_status" json:"status"`
	WinnerTraderID          string    `gorm:"column:winner_trader_id;default:''" json:"winner_trader_id"`
	LoserTraderID           string    `gorm:"column:loser_trader_id;default:''" json:"loser_trader_id"`
	Summary                 string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	ErrorMessage            string    `gorm:"column:error_message;type:text;default:''" json:"error_message"`
	MetricsJSON             string    `gorm:"column:metrics_json;type:text;default:'{}'" json:"-"`
	ProtocolJSON            string    `gorm:"column:protocol_json;type:text;default:'[]'" json:"-"`
	StartedAt               time.Time `gorm:"column:started_at;index:idx_deal_review_challenger_compares_started" json:"started_at"`
	EndsAt                  time.Time `gorm:"column:ends_at;index:idx_deal_review_challenger_compares_ends" json:"ends_at"`
	LastEvaluatedAt         time.Time `gorm:"column:last_evaluated_at" json:"last_evaluated_at"`
	ResolvedAt              time.Time `gorm:"column:resolved_at" json:"resolved_at"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func (DealReviewChallengerCompare) TableName() string { return "deal_review_challenger_compares" }

type DealReviewCyclePointRecord struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID              string    `gorm:"column:user_id;not null;index:idx_deal_review_cycle_points_user_time" json:"user_id"`
	TraderID            string    `gorm:"column:trader_id;not null;index:idx_deal_review_cycle_points_trader_time" json:"trader_id"`
	DealID              string    `gorm:"column:deal_id;default:'';index:idx_deal_review_cycle_points_deal" json:"deal_id"`
	PositionID          int64     `gorm:"column:position_id;not null;index:idx_deal_review_cycle_points_position;uniqueIndex:idx_deal_review_cycle_points_position_cycle" json:"position_id"`
	Symbol              string    `gorm:"column:symbol;not null;index:idx_deal_review_cycle_points_symbol" json:"symbol"`
	Side                string    `gorm:"column:side;not null" json:"side"`
	TimestampMs         int64     `gorm:"column:timestamp_ms;not null;index:idx_deal_review_cycle_points_time" json:"timestamp_ms"`
	DecisionCycleNumber int       `gorm:"column:decision_cycle_number;not null;uniqueIndex:idx_deal_review_cycle_points_position_cycle" json:"decision_cycle_number"`
	MarkPrice           float64   `gorm:"column:mark_price;default:0" json:"mark_price"`
	EntryPrice          float64   `gorm:"column:entry_price;default:0" json:"entry_price"`
	Quantity            float64   `gorm:"column:quantity;default:0" json:"quantity"`
	UnrealizedPnL       float64   `gorm:"column:unrealized_pnl;default:0" json:"unrealized_pnl"`
	UnrealizedPnLPct    float64   `gorm:"column:unrealized_pnl_pct;default:0" json:"unrealized_pnl_pct"`
	InProfit            bool      `gorm:"column:in_profit;default:false" json:"in_profit"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (DealReviewCyclePointRecord) TableName() string { return "deal_review_cycle_points" }

type DealReviewTrailingUpdateRecord struct {
	ID                   int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID               string    `gorm:"column:user_id;not null;index:idx_deal_review_trailing_updates_user_time" json:"user_id"`
	TraderID             string    `gorm:"column:trader_id;not null;index:idx_deal_review_trailing_updates_trader_time" json:"trader_id"`
	DealID               string    `gorm:"column:deal_id;default:'';index:idx_deal_review_trailing_updates_deal" json:"deal_id"`
	PositionID           int64     `gorm:"column:position_id;default:0;index:idx_deal_review_trailing_updates_position" json:"position_id"`
	ExchangeID           string    `gorm:"column:exchange_id;default:''" json:"exchange_id"`
	Symbol               string    `gorm:"column:symbol;not null;index:idx_deal_review_trailing_updates_symbol" json:"symbol"`
	Side                 string    `gorm:"column:side;not null" json:"side"`
	TimestampMs          int64     `gorm:"column:timestamp_ms;not null;index:idx_deal_review_trailing_updates_time" json:"timestamp_ms"`
	PreviousStopPrice    float64   `gorm:"column:previous_stop_price;default:0" json:"previous_stop_price"`
	NewStopPrice         float64   `gorm:"column:new_stop_price;default:0" json:"new_stop_price"`
	TakeProfitPrice      float64   `gorm:"column:take_profit_price;default:0" json:"take_profit_price"`
	Quantity             float64   `gorm:"column:quantity;default:0" json:"quantity"`
	EntryPrice           float64   `gorm:"column:entry_price;default:0" json:"entry_price"`
	MarkPrice            float64   `gorm:"column:mark_price;default:0" json:"mark_price"`
	Leverage             int       `gorm:"column:leverage;default:0" json:"leverage"`
	ProfitPct            float64   `gorm:"column:profit_pct;default:0" json:"profit_pct"`
	UnrealizedPnL        float64   `gorm:"column:unrealized_pnl;default:0" json:"unrealized_pnl"`
	UnrealizedPnLPct     float64   `gorm:"column:unrealized_pnl_pct;default:0" json:"unrealized_pnl_pct"`
	StopProfitPct        float64   `gorm:"column:stop_profit_pct;default:0" json:"stop_profit_pct"`
	ProtectsBreakeven    bool      `gorm:"column:protects_breakeven;default:false" json:"protects_breakeven"`
	TierIndex            int       `gorm:"column:tier_index;default:-1" json:"tier_index"`
	TierTriggerProfitPct float64   `gorm:"column:tier_trigger_profit_pct;default:0" json:"tier_trigger_profit_pct"`
	TrailingMode         string    `gorm:"column:trailing_mode;default:''" json:"trailing_mode"`
	LockProfitPct        float64   `gorm:"column:lock_profit_pct;default:0" json:"lock_profit_pct"`
	TrailOffsetPct       float64   `gorm:"column:trail_offset_pct;default:0" json:"trail_offset_pct"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (DealReviewTrailingUpdateRecord) TableName() string { return "deal_review_trailing_updates" }

type DealReviewExitIntentRecord struct {
	ID                  string    `gorm:"primaryKey" json:"id"`
	UserID              string    `gorm:"column:user_id;not null;index:idx_deal_review_exit_intents_user_time" json:"user_id"`
	TraderID            string    `gorm:"column:trader_id;not null;index:idx_deal_review_exit_intents_trader_time" json:"trader_id"`
	DealID              string    `gorm:"column:deal_id;default:'';index:idx_deal_review_exit_intents_deal" json:"deal_id"`
	PositionID          int64     `gorm:"column:position_id;default:0;index:idx_deal_review_exit_intents_position" json:"position_id"`
	ExchangeID          string    `gorm:"column:exchange_id;default:''" json:"exchange_id"`
	ExchangeOrderID     string    `gorm:"column:exchange_order_id;default:'';index:idx_deal_review_exit_intents_order" json:"exchange_order_id"`
	Symbol              string    `gorm:"column:symbol;not null;index:idx_deal_review_exit_intents_symbol" json:"symbol"`
	Side                string    `gorm:"column:side;not null" json:"side"`
	Action              string    `gorm:"column:action;default:''" json:"action"`
	IntentType          string    `gorm:"column:intent_type;not null;index:idx_deal_review_exit_intents_type" json:"intent_type"`
	SourceModule        string    `gorm:"column:source_module;default:''" json:"source_module"`
	Status              string    `gorm:"column:status;default:submitted;index:idx_deal_review_exit_intents_status" json:"status"`
	Summary             string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	Reasoning           string    `gorm:"column:reasoning;type:text;default:''" json:"reasoning"`
	DecisionCycleNumber int       `gorm:"column:decision_cycle_number;default:0" json:"decision_cycle_number"`
	Confidence          int       `gorm:"column:confidence;default:0" json:"confidence"`
	Quantity            float64   `gorm:"column:quantity;default:0" json:"quantity"`
	Price               float64   `gorm:"column:price;default:0" json:"price"`
	EntryPrice          float64   `gorm:"column:entry_price;default:0" json:"entry_price"`
	TimestampMs         int64     `gorm:"column:timestamp_ms;not null;index:idx_deal_review_exit_intents_time" json:"timestamp_ms"`
	LinkedDealID        string    `gorm:"column:linked_deal_id;default:''" json:"linked_deal_id"`
	LinkedEventID       string    `gorm:"column:linked_event_id;default:''" json:"linked_event_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (DealReviewExitIntentRecord) TableName() string { return "deal_review_exit_intents" }

type DealReviewMarketPointRecord struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           string    `gorm:"column:user_id;not null;index:idx_deal_review_market_points_user_time" json:"user_id"`
	TraderID         string    `gorm:"column:trader_id;not null;index:idx_deal_review_market_points_trader_time" json:"trader_id"`
	DealID           string    `gorm:"column:deal_id;default:'';index:idx_deal_review_market_points_deal" json:"deal_id"`
	PositionID       int64     `gorm:"column:position_id;not null;index:idx_deal_review_market_points_position;uniqueIndex:idx_deal_review_market_points_position_source_time" json:"position_id"`
	Symbol           string    `gorm:"column:symbol;not null;index:idx_deal_review_market_points_symbol" json:"symbol"`
	Side             string    `gorm:"column:side;not null" json:"side"`
	Source           string    `gorm:"column:source;not null;default:platform;index:idx_deal_review_market_points_source;uniqueIndex:idx_deal_review_market_points_position_source_time" json:"source"`
	TimestampMs      int64     `gorm:"column:timestamp_ms;not null;index:idx_deal_review_market_points_time;uniqueIndex:idx_deal_review_market_points_position_source_time" json:"timestamp_ms"`
	MarkPrice        float64   `gorm:"column:mark_price;default:0" json:"mark_price"`
	EntryPrice       float64   `gorm:"column:entry_price;default:0" json:"entry_price"`
	Quantity         float64   `gorm:"column:quantity;default:0" json:"quantity"`
	UnrealizedPnL    float64   `gorm:"column:unrealized_pnl;default:0" json:"unrealized_pnl"`
	UnrealizedPnLPct float64   `gorm:"column:unrealized_pnl_pct;default:0" json:"unrealized_pnl_pct"`
	InProfit         bool      `gorm:"column:in_profit;default:false" json:"in_profit"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (DealReviewMarketPointRecord) TableName() string { return "deal_review_market_points" }

type DealReviewClassifierFeedback struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	UserID        string    `gorm:"column:user_id;not null;index:idx_deal_review_classifier_feedback_user_case" json:"user_id"`
	TraderID      string    `gorm:"column:trader_id;not null;index:idx_deal_review_classifier_feedback_trader_case" json:"trader_id"`
	CaseID        string    `gorm:"column:case_id;not null;index:idx_deal_review_classifier_feedback_user_case;uniqueIndex:idx_deal_review_classifier_feedback_case_suggestion" json:"case_id"`
	ClassifierID  string    `gorm:"column:classifier_id;not null;index:idx_deal_review_classifier_feedback_case_suggestion" json:"classifier_id"`
	SuggestionKey string    `gorm:"column:suggestion_key;not null;index:idx_deal_review_classifier_feedback_case_suggestion" json:"suggestion_key"`
	Label         string    `gorm:"column:label;not null;index:idx_deal_review_classifier_feedback_label" json:"label"`
	IssueType     string    `gorm:"column:issue_type;default:'';index:idx_deal_review_classifier_feedback_issue" json:"issue_type"`
	Verdict       string    `gorm:"column:verdict;not null;index:idx_deal_review_classifier_feedback_verdict" json:"verdict"`
	Rationale     string    `gorm:"column:rationale;type:text;default:''" json:"rationale"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (DealReviewClassifierFeedback) TableName() string {
	return "deal_review_classifier_feedback"
}

type DealReviewClassifierSuggestion struct {
	ClassifierID    string  `json:"classifier_id"`
	SuggestionKey   string  `json:"suggestion_key"`
	Label           string  `json:"label"`
	IssueType       string  `json:"issue_type,omitempty"`
	Score           float64 `json:"score"`
	HighlightLevel  string  `json:"highlight_level,omitempty"`
	EvidenceCount   int     `json:"evidence_count"`
	AcceptedCount   int     `json:"accepted_count"`
	RejectedCount   int     `json:"rejected_count"`
	FeedbackVerdict string  `json:"feedback_verdict,omitempty"`
	Rationale       string  `json:"rationale,omitempty"`
}

type DealReviewClassifierAssist struct {
	Source         string                           `json:"source"`
	Summary        string                           `json:"summary"`
	HighlightScore float64                          `json:"highlight_score"`
	HighlightLevel string                           `json:"highlight_level,omitempty"`
	Suggestions    []DealReviewClassifierSuggestion `json:"suggestions,omitempty"`
}

type DealReviewEventSnapshot struct {
	AccountState     AccountSnapshot                  `json:"account_state"`
	Positions        []PositionSnapshot               `json:"positions,omitempty"`
	CandidateCoins   []string                         `json:"candidate_coins,omitempty"`
	CandidateDetails []CandidateDetail                `json:"candidate_details,omitempty"`
	MarketContext    *DealReviewMarketContextSnapshot `json:"market_context,omitempty"`
	ExecutionLog     []string                         `json:"execution_log,omitempty"`
	SystemPrompt     string                           `json:"system_prompt,omitempty"`
	UserPrompt       string                           `json:"user_prompt,omitempty"`
	DecisionJSON     string                           `json:"decision_json,omitempty"`
	RawResponse      string                           `json:"raw_response,omitempty"`
	CoTTrace         string                           `json:"cot_trace,omitempty"`
	AIRequestMs      int64                            `json:"ai_request_duration_ms,omitempty"`
}

type DealReviewDecisionEventInput struct {
	UserID           string
	TraderID         string
	ExchangeID       string
	ExchangeOrderID  string
	Stage            string
	CycleNumber      int
	DecisionTime     time.Time
	Symbol           string
	Side             string
	Action           string
	Quantity         float64
	PositionSizeUSD  float64
	Price            float64
	Leverage         int
	StopLoss         float64
	TakeProfit       float64
	Confidence       int
	Reasoning        string
	SelectionBucket  string
	CandidateSources []string
	Snapshot         DealReviewEventSnapshot
}

type matchedDealReviewDecision struct {
	Record           *DecisionRecord
	Action           *DecisionAction
	CandidateSources []string
	SelectionBucket  string
}

type dealReviewExitExecutionContext struct {
	Order *TraderOrder
	Fill  *TraderFill
}

type DealReviewExitEvidence struct {
	MatchedBy                 string  `json:"matched_by,omitempty"`
	DecisionCycleNumber       int     `json:"decision_cycle_number,omitempty"`
	DecisionAction            string  `json:"decision_action,omitempty"`
	OrderType                 string  `json:"order_type,omitempty"`
	VenueOrderType            string  `json:"venue_order_type,omitempty"`
	TriggerSubtype            string  `json:"trigger_subtype,omitempty"`
	TriggerSource             string  `json:"trigger_source,omitempty"`
	OrderAction               string  `json:"order_action,omitempty"`
	OrderStatus               string  `json:"order_status,omitempty"`
	ClientOrderID             string  `json:"client_order_id,omitempty"`
	ExchangeOrderID           string  `json:"exchange_order_id,omitempty"`
	TriggerPrice              float64 `json:"trigger_price,omitempty"`
	OrderPrice                float64 `json:"order_price,omitempty"`
	FillPrice                 float64 `json:"fill_price,omitempty"`
	FillQuantity              float64 `json:"fill_quantity,omitempty"`
	ReduceOnly                bool    `json:"reduce_only,omitempty"`
	ClosePosition             bool    `json:"close_position,omitempty"`
	TargetStopLoss            float64 `json:"target_stop_loss,omitempty"`
	TargetTakeProfit          float64 `json:"target_take_profit,omitempty"`
	StopLossDistanceBps       float64 `json:"stop_loss_distance_bps,omitempty"`
	TakeProfitDistanceBps     float64 `json:"take_profit_distance_bps,omitempty"`
	TrailingUpdateID          int64   `json:"trailing_update_id,omitempty"`
	PreviousStopPrice         float64 `json:"previous_stop_price,omitempty"`
	NewStopPrice              float64 `json:"new_stop_price,omitempty"`
	TrailingMode              string  `json:"trailing_mode,omitempty"`
	TrailingTierIndex         int     `json:"trailing_tier_index,omitempty"`
	TrailingTriggerProfitPct  float64 `json:"trailing_trigger_profit_pct,omitempty"`
	TrailingLockProfitPct     float64 `json:"trailing_lock_profit_pct,omitempty"`
	TrailingOffsetPct         float64 `json:"trailing_offset_pct,omitempty"`
	TrailingUpdatedAtMs       int64   `json:"trailing_updated_at_ms,omitempty"`
	TrailingUnrealizedPnL     float64 `json:"trailing_unrealized_pnl,omitempty"`
	TrailingUnrealizedPnLPct  float64 `json:"trailing_unrealized_pnl_pct,omitempty"`
	TrailingStopProfitPct     float64 `json:"trailing_stop_profit_pct,omitempty"`
	TrailingProtectsBreakeven bool    `json:"trailing_protects_breakeven,omitempty"`
	IntentID                  string  `json:"intent_id,omitempty"`
	IntentType                string  `json:"intent_type,omitempty"`
	IntentSourceModule        string  `json:"intent_source_module,omitempty"`
	IntentSummary             string  `json:"intent_summary,omitempty"`
	IntentReasoning           string  `json:"intent_reasoning,omitempty"`
	IntentConfidence          int     `json:"intent_confidence,omitempty"`
	IntentCreatedAtMs         int64   `json:"intent_created_at_ms,omitempty"`
}

type DealReviewTrailingUpdateInput struct {
	UserID               string
	TraderID             string
	PositionID           int64
	DealID               string
	ExchangeID           string
	Symbol               string
	Side                 string
	Timestamp            time.Time
	PreviousStopPrice    float64
	NewStopPrice         float64
	TakeProfitPrice      float64
	Quantity             float64
	EntryPrice           float64
	MarkPrice            float64
	Leverage             int
	ProfitPct            float64
	UnrealizedPnL        float64
	UnrealizedPnLPct     float64
	StopProfitPct        float64
	ProtectsBreakeven    bool
	TierIndex            int
	TierTriggerProfitPct float64
	TrailingMode         string
	LockProfitPct        float64
	TrailOffsetPct       float64
}

type DealReviewExitIntentInput struct {
	UserID              string
	TraderID            string
	PositionID          int64
	DealID              string
	ExchangeID          string
	ExchangeOrderID     string
	Symbol              string
	Side                string
	Action              string
	IntentType          string
	SourceModule        string
	Summary             string
	Reasoning           string
	DecisionCycleNumber int
	Confidence          int
	Quantity            float64
	Price               float64
	EntryPrice          float64
	Timestamp           time.Time
}

type dealReviewCloseInferenceResult struct {
	Reason              string
	InferredBy          string
	ExitOrigin          string
	ExitReasonQuality   string
	ExitEvidenceSummary string
	ExitEvidence        *DealReviewExitEvidence
}

type DealReviewListFilter struct {
	TraderID               string
	Symbol                 string
	Side                   string
	Status                 string
	Outcome                string
	OpenSelectionBucket    string
	CloseReason            string
	ExitReasonQuality      string
	OpenTrendRegime        string
	OpenVolatilityRegime   string
	OpenBTCStrengthRegime  string
	OpenFundingRegime      string
	OpenOIRegime           string
	OpenSessionBucket      string
	OpenWeekdayBucket      string
	OpenVenueTier          string
	OpenLiquidityTier      string
	OpenSpreadBucket       string
	OpenSlippageBucket     string
	CloseTrendRegime       string
	CloseVolatilityRegime  string
	CloseBTCStrengthRegime string
	CloseFundingRegime     string
	CloseOIRegime          string
	CloseSessionBucket     string
	CloseWeekdayBucket     string
	CloseVenueTier         string
	CloseLiquidityTier     string
	CloseSpreadBucket      string
	CloseSlippageBucket    string
	FromTime               int64
	ToTime                 int64
	MinPnL                 *float64
	MaxPnL                 *float64
	Limit                  int
	Offset                 int
}

type DealReviewCaseListItem struct {
	Case                 DealReviewCase                  `json:"case"`
	TraderName           string                          `json:"trader_name"`
	StrategyName         string                          `json:"strategy_name"`
	OpenReasoning        string                          `json:"open_reasoning"`
	CloseReasoning       string                          `json:"close_reasoning"`
	Labels               []string                        `json:"labels,omitempty"`
	OpenCandidateSources []string                        `json:"open_candidate_sources,omitempty"`
	PriceTimelineSummary *DealReviewPriceTimelineSummary `json:"price_timeline_summary,omitempty"`
	ClassifierAssist     *DealReviewClassifierAssist     `json:"classifier_assist,omitempty"`
}

type DealReviewFilterPreset struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	UserID     string    `gorm:"column:user_id;not null;index:idx_deal_review_filter_presets_user_trader" json:"user_id"`
	TraderID   string    `gorm:"column:trader_id;not null;index:idx_deal_review_filter_presets_user_trader" json:"trader_id"`
	Name       string    `gorm:"column:name;not null;index:idx_deal_review_filter_presets_name" json:"name"`
	FilterJSON string    `gorm:"column:filter_json;type:text;default:'{}'" json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (DealReviewFilterPreset) TableName() string { return "deal_review_filter_presets" }

type DealReviewFilterPresetDetail struct {
	Preset  DealReviewFilterPreset `json:"preset"`
	Filters map[string]any         `json:"filters,omitempty"`
}

type DealReviewDatasetSummary struct {
	TotalDeals               int64   `json:"total_deals"`
	OpenDeals                int64   `json:"open_deals"`
	ClosedDeals              int64   `json:"closed_deals"`
	WinningDeals             int64   `json:"winning_deals"`
	LosingDeals              int64   `json:"losing_deals"`
	FlatDeals                int64   `json:"flat_deals"`
	WinRate                  float64 `json:"win_rate"`
	NetPnL                   float64 `json:"net_pnl"`
	AvgPnL                   float64 `json:"avg_pnl"`
	Expectancy               float64 `json:"expectancy"`
	AvgPnLPct                float64 `json:"avg_pnl_pct"`
	AvgHoldMs                int64   `json:"avg_hold_ms"`
	ProfitFactor             float64 `json:"profit_factor"`
	MaxDrawdown              float64 `json:"max_drawdown_pct"`
	LongDeals                int64   `json:"long_deals"`
	ShortDeals               int64   `json:"short_deals"`
	LongNetPnL               float64 `json:"long_net_pnl"`
	ShortNetPnL              float64 `json:"short_net_pnl"`
	AvgMFECapturedPct        float64 `json:"avg_mfe_captured_pct"`
	AvgProfitGivenBackPct    float64 `json:"avg_profit_given_back_pct"`
	AvgExitEfficiencyScore   float64 `json:"avg_exit_efficiency_score"`
	AvgEntryTimingScore      float64 `json:"avg_entry_timing_score"`
	AvgRiskSizingScore       float64 `json:"avg_risk_sizing_score"`
	BadEntryDeals            int64   `json:"bad_entry_deals"`
	BadExitDeals             int64   `json:"bad_exit_deals"`
	AvoidableLossDeals       int64   `json:"avoidable_loss_deals"`
	StrongEntryWeakExitDeals int64   `json:"strong_entry_weak_exit_deals"`
	WeakEntryLuckyExitDeals  int64   `json:"weak_entry_lucky_exit_deals"`
}

type DealReviewEventDetail struct {
	Event            *DealReviewEvent         `json:"event"`
	CandidateSources []string                 `json:"candidate_sources,omitempty"`
	Snapshot         *DealReviewEventSnapshot `json:"snapshot,omitempty"`
	DecisionRecord   *DecisionRecord          `json:"decision_record,omitempty"`
}

type DealReviewCaseDetail struct {
	Case                 DealReviewCase                  `json:"case"`
	TraderName           string                          `json:"trader_name"`
	StrategyName         string                          `json:"strategy_name"`
	Labels               []string                        `json:"labels,omitempty"`
	OpenCandidateSources []string                        `json:"open_candidate_sources,omitempty"`
	Open                 *DealReviewEventDetail          `json:"open,omitempty"`
	Close                *DealReviewEventDetail          `json:"close,omitempty"`
	PriceTimeline        *DealReviewPriceTimeline        `json:"price_timeline,omitempty"`
	SymbolBehaviorPriors []DealReviewSymbolBehaviorPrior `json:"symbol_behavior_priors,omitempty"`
	LearnedPatterns      []DealReviewLearnedPattern      `json:"learned_patterns,omitempty"`
	ClassifierAssist     *DealReviewClassifierAssist     `json:"classifier_assist,omitempty"`
	AIClassifierAssist   *DealReviewClassifierAssist     `json:"ai_classifier_assist,omitempty"`
}

type DealReviewPriceTimelinePoint struct {
	Source              string  `json:"source"`
	TimestampMs         int64   `json:"timestamp_ms"`
	DecisionCycleNumber int     `json:"decision_cycle_number"`
	MarkPrice           float64 `json:"mark_price"`
	EntryPrice          float64 `json:"entry_price"`
	Quantity            float64 `json:"quantity"`
	UnrealizedPnL       float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct    float64 `json:"unrealized_pnl_pct"`
	InProfit            bool    `json:"in_profit"`
}

type DealReviewPriceTimelineSummary struct {
	CycleSamples        int     `json:"cycle_samples"`
	PlatformSamples     int     `json:"platform_samples"`
	PointCount          int     `json:"point_count"`
	EverInProfit        bool    `json:"ever_in_profit"`
	MaxUnrealizedPnL    float64 `json:"max_unrealized_pnl"`
	MaxUnrealizedPnLPct float64 `json:"max_unrealized_pnl_pct"`
	MinUnrealizedPnL    float64 `json:"min_unrealized_pnl"`
	MinUnrealizedPnLPct float64 `json:"min_unrealized_pnl_pct"`
	HighestMarkPrice    float64 `json:"highest_mark_price"`
	LowestMarkPrice     float64 `json:"lowest_mark_price"`
}

type DealReviewPriceTimeline struct {
	Points  []DealReviewPriceTimelinePoint `json:"points,omitempty"`
	Summary DealReviewPriceTimelineSummary `json:"summary"`
}

type DealReviewAIScanResult struct {
	ExecutiveSummary string                 `json:"executive_summary"`
	Strengths        []string               `json:"strengths,omitempty"`
	Weaknesses       []string               `json:"weaknesses,omitempty"`
	Patterns         []string               `json:"patterns,omitempty"`
	ImmediateActions []DealReviewActionItem `json:"immediate_actions,omitempty"`
	Experiments      []DealReviewActionItem `json:"experiments,omitempty"`
	StrategyPatch    map[string]any         `json:"strategy_patch,omitempty"`
}

type DealReviewActionItem struct {
	Title          string         `json:"title"`
	Rationale      string         `json:"rationale"`
	ExpectedImpact string         `json:"expected_impact,omitempty"`
	Risk           string         `json:"risk,omitempty"`
	ConfigPatch    map[string]any `json:"config_patch,omitempty"`
}

type DealReviewAIScanValidation struct {
	Status                    string                      `json:"status"`
	ValidatedAt               time.Time                   `json:"validated_at"`
	PromotionReady            bool                        `json:"promotion_ready"`
	DatasetCount              int                         `json:"dataset_count"`
	ClosedDealCount           int                         `json:"closed_deal_count"`
	TrainingClosedDealCount   int                         `json:"training_closed_deal_count"`
	HoldoutClosedDealCount    int                         `json:"holdout_closed_deal_count"`
	RecentClosedDealCount     int                         `json:"recent_closed_deal_count"`
	MinClosedDealCount        int                         `json:"min_closed_deal_count"`
	MinHoldoutClosedDealCount int                         `json:"min_holdout_closed_deal_count"`
	MinRecentClosedDealCount  int                         `json:"min_recent_closed_deal_count"`
	ConfigValid               bool                        `json:"config_valid"`
	ConfigWarnings            []string                    `json:"config_warnings,omitempty"`
	BlockingIssues            []string                    `json:"blocking_issues,omitempty"`
	Checks                    []DealReviewValidationCheck `json:"checks,omitempty"`
	Notes                     []string                    `json:"notes,omitempty"`
	RecentSliceLabel          string                      `json:"recent_slice_label,omitempty"`
	TrainingSummary           *DealReviewDatasetSummary   `json:"training_summary,omitempty"`
	HoldoutSummary            *DealReviewDatasetSummary   `json:"holdout_summary,omitempty"`
	RecentSummary             *DealReviewDatasetSummary   `json:"recent_summary,omitempty"`
	Replay                    *DealReviewAIScanReplay     `json:"replay,omitempty"`
}

type DealReviewValidationCheck struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	Scope      string  `json:"scope"`
	Metric     string  `json:"metric"`
	Source     string  `json:"source,omitempty"`
	Comparator string  `json:"comparator"`
	Threshold  float64 `json:"threshold"`
	Actual     float64 `json:"actual"`
	Baseline   float64 `json:"baseline,omitempty"`
	Delta      float64 `json:"delta,omitempty"`
	Passed     bool    `json:"passed"`
	Blocking   bool    `json:"blocking"`
	Message    string  `json:"message"`
}

type DealReviewAIScanReplay struct {
	Supported               bool                      `json:"supported"`
	Mode                    string                    `json:"mode,omitempty"`
	SupportedPaths          []string                  `json:"supported_paths,omitempty"`
	UnsupportedPaths        []string                  `json:"unsupported_paths,omitempty"`
	TrainingReplaySummary   *DealReviewDatasetSummary `json:"training_replay_summary,omitempty"`
	HoldoutReplaySummary    *DealReviewDatasetSummary `json:"holdout_replay_summary,omitempty"`
	RecentReplaySummary     *DealReviewDatasetSummary `json:"recent_replay_summary,omitempty"`
	TrainingNetPnLDelta     float64                   `json:"training_net_pnl_delta"`
	HoldoutNetPnLDelta      float64                   `json:"holdout_net_pnl_delta"`
	RecentNetPnLDelta       float64                   `json:"recent_net_pnl_delta"`
	TrainingClosedDealDelta int                       `json:"training_closed_deal_delta"`
	HoldoutClosedDealDelta  int                       `json:"holdout_closed_deal_delta"`
	RecentClosedDealDelta   int                       `json:"recent_closed_deal_delta"`
	BlockingIssues          []string                  `json:"blocking_issues,omitempty"`
	Notes                   []string                  `json:"notes,omitempty"`
}

type DealReviewAIScanDetail struct {
	Scan          DealReviewAIScan            `json:"scan"`
	Filters       map[string]any              `json:"filters,omitempty"`
	Result        DealReviewAIScanResult      `json:"result"`
	StrategyPatch map[string]any              `json:"strategy_patch,omitempty"`
	Validation    *DealReviewAIScanValidation `json:"validation,omitempty"`
}

type DealReviewStrategyVersionDetail struct {
	Version           DealReviewStrategyVersion             `json:"version"`
	PreviousConfig    map[string]any                        `json:"previous_config,omitempty"`
	NextConfig        map[string]any                        `json:"next_config,omitempty"`
	TargetCohort      map[string]any                        `json:"target_cohort,omitempty"`
	SourceScanSummary string                                `json:"source_scan_summary,omitempty"`
	CompareSummary    string                                `json:"compare_summary,omitempty"`
	Attribution       *DealReviewStrategyVersionAttribution `json:"attribution,omitempty"`
}

type DealReviewStrategyVersionAttribution struct {
	ObservationReady       bool                      `json:"observation_ready"`
	BeforeStart            time.Time                 `json:"before_start"`
	BeforeEnd              time.Time                 `json:"before_end"`
	AfterStart             time.Time                 `json:"after_start"`
	AfterEnd               time.Time                 `json:"after_end"`
	FullBeforeSummary      *DealReviewDatasetSummary `json:"full_before_summary,omitempty"`
	FullAfterSummary       *DealReviewDatasetSummary `json:"full_after_summary,omitempty"`
	TargetBeforeSummary    *DealReviewDatasetSummary `json:"target_before_summary,omitempty"`
	TargetAfterSummary     *DealReviewDatasetSummary `json:"target_after_summary,omitempty"`
	NonTargetBeforeSummary *DealReviewDatasetSummary `json:"non_target_before_summary,omitempty"`
	NonTargetAfterSummary  *DealReviewDatasetSummary `json:"non_target_after_summary,omitempty"`
	Warnings               []string                  `json:"warnings,omitempty"`
	RollbackSuggested      bool                      `json:"rollback_suggested"`
	Note                   string                    `json:"note,omitempty"`
}

type DealReviewChallengerMetrics struct {
	EvaluatedAt          time.Time `json:"evaluated_at"`
	IncumbentPnL         float64   `json:"incumbent_pnl"`
	ChallengerPnL        float64   `json:"challenger_pnl"`
	IncumbentTradeCount  int       `json:"incumbent_trade_count"`
	ChallengerTradeCount int       `json:"challenger_trade_count"`
	IncumbentFees        float64   `json:"incumbent_fees"`
	ChallengerFees       float64   `json:"challenger_fees"`
	IncumbentAvgHoldMs   int64     `json:"incumbent_avg_hold_ms"`
	ChallengerAvgHoldMs  int64     `json:"challenger_avg_hold_ms"`
}

type DealReviewChallengerProtocolEvent struct {
	Timestamp time.Time                    `json:"timestamp"`
	Type      string                       `json:"type"`
	Actor     string                       `json:"actor,omitempty"`
	Message   string                       `json:"message"`
	Metrics   *DealReviewChallengerMetrics `json:"metrics,omitempty"`
	Data      map[string]any               `json:"data,omitempty"`
}

type DealReviewChallengerCompareDetail struct {
	Compare                DealReviewChallengerCompare         `json:"compare"`
	Metrics                *DealReviewChallengerMetrics        `json:"metrics,omitempty"`
	Protocol               []DealReviewChallengerProtocolEvent `json:"protocol,omitempty"`
	IncumbentTraderName    string                              `json:"incumbent_trader_name"`
	ChallengerTraderName   string                              `json:"challenger_trader_name"`
	IncumbentStrategyName  string                              `json:"incumbent_strategy_name"`
	ChallengerStrategyName string                              `json:"challenger_strategy_name"`
	ChallengerExchangeName string                              `json:"challenger_exchange_name"`
	SourceScanSummary      string                              `json:"source_scan_summary"`
}

type DealReviewAnomalySymbol struct {
	Symbol      string  `json:"symbol"`
	Deals       int64   `json:"deals"`
	NetPnL      float64 `json:"net_pnl"`
	WinRate     float64 `json:"win_rate"`
	AvgHoldMs   int64   `json:"avg_hold_ms"`
	LosingDeals int64   `json:"losing_deals"`
}

type DealReviewAnomalyBucket struct {
	Bucket  string  `json:"bucket"`
	Deals   int64   `json:"deals"`
	NetPnL  float64 `json:"net_pnl"`
	WinRate float64 `json:"win_rate"`
}

type DealReviewAnomalyCloseReason struct {
	Reason string  `json:"reason"`
	Deals  int64   `json:"deals"`
	NetPnL float64 `json:"net_pnl"`
	AvgPnL float64 `json:"avg_pnl"`
}

type DealReviewAnomalyGiveBack struct {
	Symbol            string  `json:"symbol"`
	Deals             int64   `json:"deals"`
	NetPnL            float64 `json:"net_pnl"`
	AvgGiveBackPct    float64 `json:"avg_give_back_pct"`
	AvgMFECapturedPct float64 `json:"avg_mfe_captured_pct"`
}

type DealReviewAnomalyStopOut struct {
	Symbol    string  `json:"symbol"`
	Deals     int64   `json:"deals"`
	NetPnL    float64 `json:"net_pnl"`
	AvgHoldMs int64   `json:"avg_hold_ms"`
	AvgMAEPct float64 `json:"avg_mae_pct"`
}

type DealReviewAnomalySizing struct {
	Symbol             string  `json:"symbol"`
	Deals              int64   `json:"deals"`
	NetPnL             float64 `json:"net_pnl"`
	AvgRiskSizingScore float64 `json:"avg_risk_sizing_score"`
	AvgPlannedRiskPct  float64 `json:"avg_planned_risk_pct"`
}

type DealReviewAnomalyCloseReasonQuality struct {
	Reason                 string  `json:"reason"`
	Deals                  int64   `json:"deals"`
	NetPnL                 float64 `json:"net_pnl"`
	AvgExitEfficiencyScore float64 `json:"avg_exit_efficiency_score"`
	AvgMFECapturedPct      float64 `json:"avg_mfe_captured_pct"`
	AvgGiveBackPct         float64 `json:"avg_give_back_pct"`
}

type DealReviewAnomalyExitUncertainty struct {
	Label             string  `json:"label"`
	CloseReason       string  `json:"close_reason"`
	ExitReasonQuality string  `json:"exit_reason_quality"`
	ExitOrigin        string  `json:"exit_origin"`
	Deals             int64   `json:"deals"`
	NetPnL            float64 `json:"net_pnl"`
	AvgPnL            float64 `json:"avg_pnl"`
	SharePct          float64 `json:"share_pct"`
}

type DealReviewAnomalySymbolEdgeFailure struct {
	Symbol                string   `json:"symbol"`
	Side                  string   `json:"side"`
	RegimeSignature       string   `json:"regime_signature"`
	OpenSelectionBucket   string   `json:"open_selection_bucket,omitempty"`
	OpenTrendRegime       string   `json:"open_trend_regime,omitempty"`
	OpenVolatilityRegime  string   `json:"open_volatility_regime,omitempty"`
	OpenOIRegime          string   `json:"open_oi_regime,omitempty"`
	SliceDeals            int64    `json:"slice_deals"`
	SliceNetPnL           float64  `json:"slice_net_pnl"`
	HistoricalDeals       int      `json:"historical_deals"`
	DecisionOpenCount     int      `json:"decision_open_count"`
	AvgDecisionConfidence float64  `json:"avg_decision_confidence"`
	AvgPnLPct             float64  `json:"avg_pnl_pct"`
	ContradictionScore    float64  `json:"contradiction_score"`
	RecommendedAction     string   `json:"recommended_action"`
	SignalTags            []string `json:"signal_tags,omitempty"`
	Summary               string   `json:"summary,omitempty"`
}

type DealReviewAnomalySummary struct {
	ClosedDeals            int64                                 `json:"closed_deals"`
	WorstSymbols           []DealReviewAnomalySymbol             `json:"worst_symbols,omitempty"`
	OvertradedSymbols      []DealReviewAnomalySymbol             `json:"overtraded_symbols,omitempty"`
	WeakBuckets            []DealReviewAnomalyBucket             `json:"weak_buckets,omitempty"`
	WeakCloseReasons       []DealReviewAnomalyCloseReason        `json:"weak_close_reasons,omitempty"`
	ProfitGiveBackHotspots []DealReviewAnomalyGiveBack           `json:"profit_give_back_hotspots,omitempty"`
	EarlyStopOutHotspots   []DealReviewAnomalyStopOut            `json:"early_stop_out_hotspots,omitempty"`
	OversizedLossHotspots  []DealReviewAnomalySizing             `json:"oversized_loss_hotspots,omitempty"`
	CloseReasonQuality     []DealReviewAnomalyCloseReasonQuality `json:"close_reason_quality,omitempty"`
	ExitUncertainty        []DealReviewAnomalyExitUncertainty    `json:"exit_uncertainty,omitempty"`
	SymbolEdgeFailures     []DealReviewAnomalySymbolEdgeFailure  `json:"symbol_edge_failures,omitempty"`
	Notes                  []string                              `json:"notes,omitempty"`
}

func NewDealReviewStore(db *gorm.DB) *DealReviewStore {
	return &DealReviewStore{db: db}
}

func (s *DealReviewStore) initTables() error {
	if err := s.db.AutoMigrate(&DealReviewCase{}, &DealReviewEvent{}, &DealReviewAIScan{}, &DealReviewStrategyVersion{}, &DealReviewChallengerCompare{}, &DealReviewCyclePointRecord{}, &DealReviewMarketPointRecord{}, &DealReviewTrailingUpdateRecord{}, &DealReviewExitIntentRecord{}, &DealReviewClassifierFeedback{}, &DealReviewFilterPreset{}, &DealReviewSymbolBehaviorPrior{}, &DealReviewSymbolBehaviorLiveGuardEvent{}, &DealReviewPatternFeatureRecord{}, &DealReviewLearnedPattern{}, &DealReviewPatternEvidenceRecord{}, &DealReviewPatternValidationRun{}, &DealReviewPatternBacklogCandidate{}, &DealReviewLearnedPatternLiveGuardEvent{}, &DealReviewLearnedPatternLiveGuardRollupCache{}, &DealReviewLearnedPatternLifecycleSnapshot{}, &DealReviewLearnedPatternManualControl{}, &DealReviewLearnedPatternManualControlEvent{}, &DealReviewLearnedPatternIntervention{}); err != nil {
		return fmt.Errorf("failed to migrate deal review tables: %w", err)
	}
	if !s.db.Migrator().HasTable(&DealReviewExitIntentRecord{}) {
		if err := s.db.AutoMigrate(&DealReviewExitIntentRecord{}); err != nil {
			return fmt.Errorf("failed to migrate deal review exit intent table: %w", err)
		}
	}
	if err := s.ensureSymbolBehaviorPriorColumns(); err != nil {
		return fmt.Errorf("failed to ensure deal review symbol prior columns: %w", err)
	}
	return nil
}

func (s *DealReviewStore) CreatePendingDecisionEvent(input *DealReviewDecisionEventInput) (*DealReviewEvent, error) {
	if input == nil {
		return nil, fmt.Errorf("deal review input cannot be nil")
	}
	stage := strings.ToLower(strings.TrimSpace(input.Stage))
	if stage != DealReviewStageOpen && stage != DealReviewStageClose {
		return nil, fmt.Errorf("invalid deal review stage %q", input.Stage)
	}
	symbol := strings.TrimSpace(input.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("deal review symbol cannot be empty")
	}

	event := &DealReviewEvent{
		ID:                  uuid.NewString(),
		UserID:              input.UserID,
		TraderID:            input.TraderID,
		Stage:               stage,
		Source:              DealReviewEventSourceAIDecision,
		Status:              DealReviewEventStatusPending,
		DecisionCycleNumber: input.CycleNumber,
		DecisionTimestamp:   input.DecisionTime.UTC(),
		ExchangeID:          strings.TrimSpace(input.ExchangeID),
		ExchangeOrderID:     strings.TrimSpace(input.ExchangeOrderID),
		Symbol:              symbol,
		Side:                normalizeDealReviewSide(input.Side),
		Action:              strings.TrimSpace(input.Action),
		Quantity:            input.Quantity,
		PositionSizeUSD:     input.PositionSizeUSD,
		Price:               input.Price,
		Leverage:            input.Leverage,
		StopLoss:            input.StopLoss,
		TakeProfit:          input.TakeProfit,
		Confidence:          input.Confidence,
		Reasoning:           strings.TrimSpace(input.Reasoning),
		SelectionBucket:     strings.TrimSpace(input.SelectionBucket),
	}
	if event.DecisionTimestamp.IsZero() {
		event.DecisionTimestamp = time.Now().UTC()
	}

	if len(input.CandidateSources) > 0 {
		sourcesJSON, _ := json.Marshal(input.CandidateSources)
		event.CandidateSourcesJSON = string(sourcesJSON)
	} else {
		event.CandidateSourcesJSON = "[]"
	}

	snapshotJSON, _ := json.Marshal(input.Snapshot)
	if len(snapshotJSON) == 0 || string(snapshotJSON) == "null" {
		event.SnapshotJSON = "{}"
	} else {
		event.SnapshotJSON = string(snapshotJSON)
	}

	var existing DealReviewEvent
	err := s.db.Where("trader_id = ? AND exchange_order_id = ? AND stage = ? AND status != ?",
		event.TraderID, event.ExchangeOrderID, event.Stage, DealReviewEventStatusCanceled).
		Order("created_at DESC").
		First(&existing).Error
	if err == nil {
		existing.DecisionCycleNumber = event.DecisionCycleNumber
		existing.DecisionTimestamp = event.DecisionTimestamp
		existing.Symbol = event.Symbol
		existing.Side = event.Side
		existing.Action = event.Action
		existing.Quantity = event.Quantity
		existing.PositionSizeUSD = event.PositionSizeUSD
		existing.Price = event.Price
		existing.Leverage = event.Leverage
		existing.StopLoss = event.StopLoss
		existing.TakeProfit = event.TakeProfit
		existing.Confidence = event.Confidence
		existing.Reasoning = event.Reasoning
		existing.SelectionBucket = event.SelectionBucket
		existing.CandidateSourcesJSON = event.CandidateSourcesJSON
		existing.SnapshotJSON = event.SnapshotJSON
		existing.Source = DealReviewEventSourceAIDecision
		existing.Status = DealReviewEventStatusPending
		if updateErr := s.db.Save(&existing).Error; updateErr != nil {
			return nil, updateErr
		}
		return &existing, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err := s.db.Create(event).Error; err != nil {
		return nil, err
	}
	return event, nil
}

func (s *DealReviewStore) CancelPendingEvent(traderID, orderID, stage string) error {
	if strings.TrimSpace(orderID) == "" {
		return nil
	}
	return s.db.Model(&DealReviewEvent{}).
		Where("trader_id = ? AND exchange_order_id = ? AND stage = ? AND status = ?", traderID, orderID, stage, DealReviewEventStatusPending).
		Updates(map[string]any{
			"status":     DealReviewEventStatusCanceled,
			"updated_at": time.Now().UTC(),
		}).Error
}

func (s *DealReviewStore) SyncOpenPosition(position *TraderPosition) error {
	if position == nil || position.ID == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		caseRec, err := s.ensureCaseForPositionTx(tx, position)
		if err != nil {
			return err
		}
		return s.linkEventForPositionTx(tx, DealReviewStageOpen, position.EntryOrderID, caseRec, position)
	})
}

func (s *DealReviewStore) SyncClosedPosition(position *TraderPosition) error {
	if position == nil || position.ID == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		caseRec, err := s.ensureCaseForPositionTx(tx, position)
		if err != nil {
			return err
		}
		if err := s.linkEventForPositionTx(tx, DealReviewStageClose, position.ExitOrderID, caseRec, position); err != nil {
			return err
		}
		return s.refreshCaseQualityMetricsTx(tx, caseRec)
	})
}

func (s *DealReviewStore) SyncPosition(position *TraderPosition) error {
	if position == nil || position.ID == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.syncPositionTx(tx, position, true)
	})
}

func (s *DealReviewStore) BackfillExistingPositions() error {
	return s.backfillExistingPositions()
}

func (s *DealReviewStore) BackfillExitAttribution() error {
	return s.backfillExitAttribution()
}

func (s *DealReviewStore) BackfillQualityMetrics() error {
	return s.backfillQualityMetrics()
}

func (s *DealReviewStore) backfillExistingPositions() error {
	var positions []TraderPosition
	return s.db.
		Model(&TraderPosition{}).
		Order("id ASC").
		FindInBatches(&positions, 200, func(tx *gorm.DB, _ int) error {
			for i := range positions {
				if err := s.syncPositionTx(tx, &positions[i], false); err != nil {
					return err
				}
			}
			return nil
		}).Error
}

func (s *DealReviewStore) backfillExitAttribution() error {
	var positions []TraderPosition
	return s.db.
		Model(&TraderPosition{}).
		Where("status = ? OR exit_time > 0", DealReviewCaseStatusClosed).
		Order("id ASC").
		FindInBatches(&positions, 200, func(tx *gorm.DB, _ int) error {
			for i := range positions {
				if err := s.syncPositionTx(tx, &positions[i], false); err != nil {
					return err
				}
			}
			return nil
		}).Error
}

func (s *DealReviewStore) syncPositionTx(tx *gorm.DB, position *TraderPosition, includeTimelineBackfill bool) error {
	if position == nil || position.ID == 0 {
		return nil
	}
	caseRec, err := s.ensureCaseForPositionTx(tx, position)
	if err != nil {
		return err
	}
	if err := s.linkEventForPositionTx(tx, DealReviewStageOpen, position.EntryOrderID, caseRec, position); err != nil {
		return err
	}
	if strings.EqualFold(position.Status, DealReviewCaseStatusClosed) || position.ExitTime > 0 {
		if err := s.linkEventForPositionTx(tx, DealReviewStageClose, position.ExitOrderID, caseRec, position); err != nil {
			return err
		}
		if includeTimelineBackfill {
			if err := s.backfillCaseDecisionCyclePricePointsTx(tx, caseRec, position, time.Now().UTC().UnixMilli(), true); err != nil {
				return err
			}
		}
		if err := s.refreshCaseQualityMetricsTx(tx, caseRec); err != nil {
			return err
		}
	}
	return nil
}

func (s *DealReviewStore) ListCases(userID string, filter DealReviewListFilter) ([]DealReviewCaseListItem, *DealReviewDatasetSummary, int64, error) {
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Limit > 500 {
		filter.Limit = 500
	}
	query := s.buildCaseFilterQuery(userID, filter)

	var total int64
	if err := query.Model(&DealReviewCase{}).Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	var cases []DealReviewCase
	if err := query.
		Order("CASE WHEN exit_time_ms > 0 THEN exit_time_ms ELSE entry_time_ms END DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&cases).Error; err != nil {
		return nil, nil, 0, err
	}

	items, err := s.enrichCaseList(cases)
	if err != nil {
		return nil, nil, 0, err
	}
	summary, err := s.computeDatasetSummary(userID, filter)
	if err != nil {
		return nil, nil, 0, err
	}
	return items, summary, total, nil
}

func (s *DealReviewStore) GetCaseDetail(userID, traderID, caseID string) (*DealReviewCaseDetail, error) {
	var caseRec DealReviewCase
	err := s.db.Where("id = ? AND user_id = ? AND trader_id = ?", caseID, userID, traderID).First(&caseRec).Error
	if err != nil {
		return nil, err
	}
	hydrateDealReviewCaseJSONFields(&caseRec)

	detail := &DealReviewCaseDetail{Case: caseRec}
	var trader Trader
	if err := s.db.Where("id = ?", caseRec.TraderID).First(&trader).Error; err == nil {
		detail.TraderName = trader.Name
	}
	if caseRec.StrategyID != "" {
		var strategy Strategy
		if err := s.db.Where("id = ?", caseRec.StrategyID).First(&strategy).Error; err == nil {
			detail.StrategyName = strategy.Name
		}
	}
	detail.OpenCandidateSources = parseJSONStringSlice(caseRec.OpenCandidateSourcesJSON)
	detail.Labels = parseJSONStringSlice(caseRec.LabelsJSON)

	if caseRec.OpenEventID != "" {
		if openDetail, err := s.buildEventDetail(caseRec.TraderID, caseRec.OpenEventID); err == nil {
			detail.Open = openDetail
		}
	}
	if caseRec.CloseEventID != "" {
		if closeDetail, err := s.buildEventDetail(caseRec.TraderID, caseRec.CloseEventID); err == nil {
			detail.Close = closeDetail
		}
	}
	if timeline, err := s.buildCasePriceTimeline(&caseRec); err == nil && timeline != nil {
		detail.PriceTimeline = timeline
	}
	if assist, err := s.BuildHeuristicClassifierAssist(userID, traderID, &caseRec); err == nil {
		detail.ClassifierAssist = assist
	}
	return detail, nil
}

// GetLatestOpenCaseBySymbol returns the latest open deal-review case for a symbol/side pair.
func (s *DealReviewStore) GetLatestOpenCaseBySymbol(userID, traderID, symbol, side string) (*DealReviewCase, error) {
	var caseRec DealReviewCase
	err := s.db.
		Where(
			"user_id = ? AND trader_id = ? AND symbol = ? AND side = ? AND status = ?",
			userID,
			traderID,
			strings.ToUpper(strings.TrimSpace(symbol)),
			strings.ToUpper(strings.TrimSpace(side)),
			DealReviewCaseStatusOpen,
		).
		Order("entry_time_ms DESC, created_at DESC").
		First(&caseRec).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &caseRec, nil
}

func (s *DealReviewStore) RecordTrailingStopUpdate(input *DealReviewTrailingUpdateInput) error {
	if input == nil {
		return fmt.Errorf("trailing stop update input cannot be nil")
	}
	symbol := strings.ToUpper(strings.TrimSpace(input.Symbol))
	side := normalizeDealReviewSide(input.Side)
	if symbol == "" || side == "" {
		return fmt.Errorf("trailing stop update requires symbol and side")
	}
	if input.NewStopPrice <= 0 {
		return fmt.Errorf("trailing stop update requires new stop price")
	}
	record := &DealReviewTrailingUpdateRecord{
		UserID:               strings.TrimSpace(input.UserID),
		TraderID:             strings.TrimSpace(input.TraderID),
		PositionID:           input.PositionID,
		DealID:               strings.TrimSpace(input.DealID),
		ExchangeID:           strings.TrimSpace(input.ExchangeID),
		Symbol:               symbol,
		Side:                 side,
		PreviousStopPrice:    input.PreviousStopPrice,
		NewStopPrice:         input.NewStopPrice,
		TakeProfitPrice:      input.TakeProfitPrice,
		Quantity:             input.Quantity,
		EntryPrice:           input.EntryPrice,
		MarkPrice:            input.MarkPrice,
		Leverage:             input.Leverage,
		ProfitPct:            input.ProfitPct,
		UnrealizedPnL:        input.UnrealizedPnL,
		UnrealizedPnLPct:     input.UnrealizedPnLPct,
		StopProfitPct:        input.StopProfitPct,
		ProtectsBreakeven:    input.ProtectsBreakeven,
		TierIndex:            input.TierIndex,
		TierTriggerProfitPct: input.TierTriggerProfitPct,
		TrailingMode:         strings.TrimSpace(input.TrailingMode),
		LockProfitPct:        input.LockProfitPct,
		TrailOffsetPct:       input.TrailOffsetPct,
	}
	if record.UserID == "" || record.TraderID == "" {
		return fmt.Errorf("trailing stop update requires user_id and trader_id")
	}
	if input.Timestamp.IsZero() {
		record.TimestampMs = time.Now().UTC().UnixMilli()
	} else {
		record.TimestampMs = input.Timestamp.UTC().UnixMilli()
	}

	if record.PositionID == 0 {
		if pos, err := NewPositionStore(s.db).GetOpenPositionBySymbol(record.TraderID, symbol, side); err == nil && pos != nil {
			record.PositionID = pos.ID
			if record.ExchangeID == "" {
				record.ExchangeID = pos.ExchangeID
			}
		} else if err != nil {
			return err
		}
	}
	if record.DealID == "" {
		if caseRec, err := s.GetLatestOpenCaseBySymbol(record.UserID, record.TraderID, symbol, side); err == nil && caseRec != nil {
			record.DealID = caseRec.ID
			if record.PositionID == 0 {
				record.PositionID = caseRec.PositionID
			}
			if record.ExchangeID == "" {
				record.ExchangeID = caseRec.ExchangeID
			}
		} else if err != nil {
			return err
		}
	}

	return s.db.Create(record).Error
}

func (s *DealReviewStore) RecordExitIntent(input *DealReviewExitIntentInput) error {
	if input == nil {
		return fmt.Errorf("exit intent input cannot be nil")
	}

	symbol := strings.ToUpper(strings.TrimSpace(input.Symbol))
	side := normalizeDealReviewSide(input.Side)
	intentType := strings.TrimSpace(input.IntentType)
	if symbol == "" || side == "" || intentType == "" {
		return fmt.Errorf("exit intent requires symbol, side, and intent type")
	}

	record := &DealReviewExitIntentRecord{
		ID:                  uuid.NewString(),
		UserID:              strings.TrimSpace(input.UserID),
		TraderID:            strings.TrimSpace(input.TraderID),
		PositionID:          input.PositionID,
		DealID:              strings.TrimSpace(input.DealID),
		ExchangeID:          strings.TrimSpace(input.ExchangeID),
		ExchangeOrderID:     strings.TrimSpace(input.ExchangeOrderID),
		Symbol:              symbol,
		Side:                side,
		Action:              strings.TrimSpace(input.Action),
		IntentType:          intentType,
		SourceModule:        strings.TrimSpace(input.SourceModule),
		Status:              DealReviewExitIntentStatusSubmitted,
		Summary:             strings.TrimSpace(input.Summary),
		Reasoning:           strings.TrimSpace(input.Reasoning),
		DecisionCycleNumber: input.DecisionCycleNumber,
		Confidence:          input.Confidence,
		Quantity:            input.Quantity,
		Price:               input.Price,
		EntryPrice:          input.EntryPrice,
	}
	if record.UserID == "" || record.TraderID == "" {
		return fmt.Errorf("exit intent requires user_id and trader_id")
	}
	if input.Timestamp.IsZero() {
		record.TimestampMs = time.Now().UTC().UnixMilli()
	} else {
		record.TimestampMs = input.Timestamp.UTC().UnixMilli()
	}

	if record.PositionID == 0 {
		if pos, err := NewPositionStore(s.db).GetOpenPositionBySymbol(record.TraderID, symbol, side); err == nil && pos != nil {
			record.PositionID = pos.ID
			if record.ExchangeID == "" {
				record.ExchangeID = pos.ExchangeID
			}
		} else if err != nil {
			return err
		}
	}
	if record.DealID == "" {
		if caseRec, err := s.GetLatestOpenCaseBySymbol(record.UserID, record.TraderID, symbol, side); err == nil && caseRec != nil {
			record.DealID = caseRec.ID
			if record.PositionID == 0 {
				record.PositionID = caseRec.PositionID
			}
			if record.ExchangeID == "" {
				record.ExchangeID = caseRec.ExchangeID
			}
		} else if err != nil {
			return err
		}
	}

	return s.db.Create(record).Error
}

func (s *DealReviewStore) CancelExitIntent(traderID, exchangeOrderID string) error {
	traderID = strings.TrimSpace(traderID)
	exchangeOrderID = strings.TrimSpace(exchangeOrderID)
	if traderID == "" || exchangeOrderID == "" {
		return nil
	}
	return s.db.Model(&DealReviewExitIntentRecord{}).
		Where("trader_id = ? AND exchange_order_id = ? AND status = ?", traderID, exchangeOrderID, DealReviewExitIntentStatusSubmitted).
		Updates(map[string]any{
			"status":     DealReviewExitIntentStatusCanceled,
			"updated_at": time.Now().UTC(),
		}).Error
}

func (s *DealReviewStore) ListAnalysisCases(userID string, filter DealReviewListFilter, limit int) ([]DealReviewCaseDetail, *DealReviewDatasetSummary, error) {
	if limit <= 0 {
		limit = 200
	}
	filter.Limit = limit
	filter.Offset = 0
	items, summary, _, err := s.ListCases(userID, filter)
	if err != nil {
		return nil, nil, err
	}
	details := make([]DealReviewCaseDetail, 0, len(items))
	for _, item := range items {
		detail, err := s.GetCaseDetail(userID, item.Case.TraderID, item.Case.ID)
		if err != nil {
			return nil, nil, err
		}
		details = append(details, *detail)
	}
	return details, summary, nil
}

// GetFirstTrailingUpdatesByPosition returns the earliest persisted trailing-stop
// update for each requested position inside the requested trader scope.
func (s *DealReviewStore) GetFirstTrailingUpdatesByPosition(userID, traderID string, positionIDs []int64) (map[int64]DealReviewTrailingUpdateRecord, error) {
	result := make(map[int64]DealReviewTrailingUpdateRecord)
	if len(positionIDs) == 0 {
		return result, nil
	}

	queryIDs := make([]int64, 0, len(positionIDs))
	seen := make(map[int64]struct{}, len(positionIDs))
	for _, positionID := range positionIDs {
		if positionID <= 0 {
			continue
		}
		if _, exists := seen[positionID]; exists {
			continue
		}
		seen[positionID] = struct{}{}
		queryIDs = append(queryIDs, positionID)
	}
	if len(queryIDs) == 0 {
		return result, nil
	}

	var rows []DealReviewTrailingUpdateRecord
	if err := s.db.
		Where("user_id = ? AND trader_id = ? AND position_id IN ?", userID, traderID, queryIDs).
		Order("position_id ASC, timestamp_ms ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		if row.PositionID <= 0 {
			continue
		}
		if _, exists := result[row.PositionID]; exists {
			continue
		}
		result[row.PositionID] = row
	}
	return result, nil
}

func (s *DealReviewStore) SaveAIScan(scan *DealReviewAIScan) error {
	if scan == nil {
		return fmt.Errorf("scan cannot be nil")
	}
	if strings.TrimSpace(scan.ID) == "" {
		scan.ID = uuid.NewString()
	}
	if strings.TrimSpace(scan.ValidationStatus) == "" {
		scan.ValidationStatus = DealReviewAIScanValidationPending
	}
	return s.db.Save(scan).Error
}

func (s *DealReviewStore) SaveStrategyVersion(version *DealReviewStrategyVersion) error {
	if version == nil {
		return fmt.Errorf("strategy version cannot be nil")
	}
	if strings.TrimSpace(version.ID) == "" {
		version.ID = uuid.NewString()
	}
	if version.AppliedAt.IsZero() {
		version.AppliedAt = time.Now().UTC()
	}
	if strings.TrimSpace(version.TargetCohortJSON) == "" {
		version.TargetCohortJSON = "{}"
	}
	return s.db.Save(version).Error
}

func (s *DealReviewStore) SaveChallengerCompare(compare *DealReviewChallengerCompare) error {
	if compare == nil {
		return fmt.Errorf("challenger compare cannot be nil")
	}
	if strings.TrimSpace(compare.ID) == "" {
		compare.ID = uuid.NewString()
	}
	if strings.TrimSpace(compare.Status) == "" {
		compare.Status = DealReviewChallengerStatusStarting
	}
	if strings.TrimSpace(compare.ProtocolJSON) == "" {
		compare.ProtocolJSON = "[]"
	}
	return s.db.Save(compare).Error
}

func (s *DealReviewStore) SaveFilterPreset(preset *DealReviewFilterPreset) error {
	if preset == nil {
		return fmt.Errorf("filter preset cannot be nil")
	}
	if strings.TrimSpace(preset.ID) == "" {
		preset.ID = uuid.NewString()
	}
	preset.Name = strings.TrimSpace(preset.Name)
	if preset.Name == "" {
		return fmt.Errorf("filter preset name cannot be empty")
	}
	if strings.TrimSpace(preset.FilterJSON) == "" {
		preset.FilterJSON = "{}"
	}
	return s.db.Save(preset).Error
}

func (s *DealReviewStore) ListFilterPresets(userID, traderID string) ([]DealReviewFilterPresetDetail, error) {
	var presets []DealReviewFilterPreset
	if err := s.db.
		Where("user_id = ? AND trader_id = ?", userID, traderID).
		Order("updated_at DESC, created_at DESC").
		Find(&presets).Error; err != nil {
		return nil, err
	}
	result := make([]DealReviewFilterPresetDetail, 0, len(presets))
	for _, preset := range presets {
		result = append(result, buildDealReviewFilterPresetDetail(preset))
	}
	return result, nil
}

func (s *DealReviewStore) DeleteFilterPreset(userID, traderID, presetID string) error {
	return s.db.
		Where("id = ? AND user_id = ? AND trader_id = ?", presetID, userID, traderID).
		Delete(&DealReviewFilterPreset{}).Error
}

func buildDealReviewFilterPresetDetail(preset DealReviewFilterPreset) DealReviewFilterPresetDetail {
	detail := DealReviewFilterPresetDetail{
		Preset:  preset,
		Filters: map[string]any{},
	}
	if strings.TrimSpace(preset.FilterJSON) != "" && strings.TrimSpace(preset.FilterJSON) != "{}" {
		_ = json.Unmarshal([]byte(preset.FilterJSON), &detail.Filters)
	}
	return detail
}

func effectiveDealReviewStrategyVersionTime(version DealReviewStrategyVersion) time.Time {
	if !version.AppliedAt.IsZero() {
		return version.AppliedAt.UTC()
	}
	return version.CreatedAt.UTC()
}

func sanitizeDealReviewTargetCohortMap(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	result := map[string]any{}
	for key, value := range raw {
		normalizedKey := strings.TrimSpace(strings.ToLower(key))
		switch normalizedKey {
		case "", "from_time", "to_time", "min_pnl", "max_pnl", "outcome", "status", "limit", "offset", "trader_id":
			continue
		}
		switch typed := value.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			result[normalizedKey] = trimmed
		default:
			result[normalizedKey] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func (s *DealReviewStore) getStrategyVersionNeighbors(version *DealReviewStrategyVersion) (*DealReviewStrategyVersion, *DealReviewStrategyVersion, error) {
	if version == nil {
		return nil, nil, nil
	}
	var previous DealReviewStrategyVersion
	prevErr := s.db.
		Where("user_id = ? AND trader_id = ? AND created_at < ?", version.UserID, version.TraderID, version.CreatedAt).
		Order("created_at DESC").
		First(&previous).Error
	if prevErr != nil && prevErr != gorm.ErrRecordNotFound {
		return nil, nil, prevErr
	}

	var next DealReviewStrategyVersion
	nextErr := s.db.
		Where("user_id = ? AND trader_id = ? AND created_at > ?", version.UserID, version.TraderID, version.CreatedAt).
		Order("created_at ASC").
		First(&next).Error
	if nextErr != nil && nextErr != gorm.ErrRecordNotFound {
		return nil, nil, nextErr
	}

	var prevPtr *DealReviewStrategyVersion
	if prevErr == nil {
		prevCopy := previous
		prevPtr = &prevCopy
	}
	var nextPtr *DealReviewStrategyVersion
	if nextErr == nil {
		nextCopy := next
		nextPtr = &nextCopy
	}
	return prevPtr, nextPtr, nil
}

func summarizeDealReviewClosedCaseRows(cases []DealReviewCase) *DealReviewDatasetSummary {
	summary := &DealReviewDatasetSummary{}
	var grossProfit float64
	var grossLoss float64
	pnlPctSeries := make([]float64, 0, len(cases))

	for _, caseRec := range cases {
		if caseRec.Status != DealReviewCaseStatusClosed {
			continue
		}
		summary.TotalDeals++
		summary.ClosedDeals++
		summary.NetPnL += caseRec.RealizedPnL
		summary.AvgPnLPct += caseRec.RealizedPnLPct
		summary.AvgHoldMs += caseRec.HoldDurationMs
		pnlPctSeries = append(pnlPctSeries, caseRec.RealizedPnLPct)
		if caseRec.Side == "LONG" {
			summary.LongDeals++
			summary.LongNetPnL += caseRec.RealizedPnL
		} else if caseRec.Side == "SHORT" {
			summary.ShortDeals++
			summary.ShortNetPnL += caseRec.RealizedPnL
		}
		switch {
		case caseRec.RealizedPnL > 0:
			summary.WinningDeals++
			grossProfit += caseRec.RealizedPnL
		case caseRec.RealizedPnL < 0:
			summary.LosingDeals++
			grossLoss += math.Abs(caseRec.RealizedPnL)
		default:
			summary.FlatDeals++
		}
	}

	if summary.ClosedDeals > 0 {
		summary.AvgPnL = summary.NetPnL / float64(summary.ClosedDeals)
		summary.Expectancy = summary.AvgPnL
		summary.AvgPnLPct = summary.AvgPnLPct / float64(summary.ClosedDeals)
		summary.AvgHoldMs = int64(float64(summary.AvgHoldMs) / float64(summary.ClosedDeals))
		summary.WinRate = (float64(summary.WinningDeals) / float64(summary.ClosedDeals)) * 100
	}
	if grossLoss > 0 {
		summary.ProfitFactor = grossProfit / grossLoss
	} else if grossProfit > 0 {
		summary.ProfitFactor = grossProfit
	}
	summary.MaxDrawdown = calculateDealReviewCaseMaxDrawdownPct(pnlPctSeries)
	return summary
}

func calculateDealReviewCaseMaxDrawdownPct(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}
	const startingEquity = 100.0
	equity := startingEquity
	peak := startingEquity
	var maxDrawdown float64
	for _, pnl := range pnls {
		equity += pnl
		if equity > peak {
			peak = equity
		}
		if peak <= 0 {
			continue
		}
		drawdown := (peak - equity) / peak * 100
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}
	return maxDrawdown
}

func (s *DealReviewStore) listClosedCasesForVersionWindow(userID, traderID string, start, end time.Time, targetCohort map[string]any) ([]DealReviewCase, error) {
	query := s.db.
		Model(&DealReviewCase{}).
		Where("user_id = ? AND trader_id = ? AND status = ?", userID, traderID, DealReviewCaseStatusClosed)

	if !start.IsZero() {
		query = query.Where("exit_time_ms >= ?", start.UTC().UnixMilli())
	}
	if !end.IsZero() {
		query = query.Where("exit_time_ms < ?", end.UTC().UnixMilli())
	}
	if symbol, ok := targetCohort["symbol"].(string); ok && strings.TrimSpace(symbol) != "" {
		query = query.Where("symbol = ?", strings.ToUpper(strings.TrimSpace(symbol)))
	}
	if side, ok := targetCohort["side"].(string); ok && strings.TrimSpace(side) != "" {
		query = query.Where("side = ?", strings.ToUpper(strings.TrimSpace(side)))
	}

	var cases []DealReviewCase
	if err := query.Order("exit_time_ms ASC").Find(&cases).Error; err != nil {
		return nil, err
	}
	return cases, nil
}

func excludeDealReviewCasesByID(allCases []DealReviewCase, excluded []DealReviewCase) []DealReviewCase {
	if len(allCases) == 0 {
		return nil
	}
	excludedIDs := make(map[string]struct{}, len(excluded))
	for _, item := range excluded {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		excludedIDs[item.ID] = struct{}{}
	}
	if len(excludedIDs) == 0 {
		return append([]DealReviewCase(nil), allCases...)
	}
	result := make([]DealReviewCase, 0, len(allCases))
	for _, item := range allCases {
		if _, skip := excludedIDs[item.ID]; skip {
			continue
		}
		result = append(result, item)
	}
	return result
}

func buildDealReviewAttributionWarnings(
	attribution *DealReviewStrategyVersionAttribution,
) (warnings []string, rollbackSuggested bool) {
	if attribution == nil {
		return nil, false
	}

	fullUnderperforming := false
	targetUnderperforming := false

	if attribution.FullBeforeSummary != nil && attribution.FullAfterSummary != nil &&
		attribution.FullBeforeSummary.ClosedDeals >= 3 && attribution.FullAfterSummary.ClosedDeals >= 3 &&
		attribution.FullAfterSummary.NetPnL < attribution.FullBeforeSummary.NetPnL-0.01 {
		fullUnderperforming = true
		warnings = append(warnings,
			fmt.Sprintf("Full strategy performance is weaker after the change (%+.2f before vs %+.2f after).",
				attribution.FullBeforeSummary.NetPnL,
				attribution.FullAfterSummary.NetPnL,
			),
		)
	}

	if attribution.TargetBeforeSummary != nil && attribution.TargetAfterSummary != nil &&
		attribution.TargetBeforeSummary.ClosedDeals >= 3 && attribution.TargetAfterSummary.ClosedDeals >= 3 &&
		attribution.TargetAfterSummary.NetPnL < attribution.TargetBeforeSummary.NetPnL-0.01 {
		targetUnderperforming = true
		warnings = append(warnings,
			fmt.Sprintf("Target cohort is underperforming after the change (%+.2f before vs %+.2f after).",
				attribution.TargetBeforeSummary.NetPnL,
				attribution.TargetAfterSummary.NetPnL,
			),
		)
	}

	if attribution.NonTargetBeforeSummary != nil && attribution.NonTargetAfterSummary != nil &&
		attribution.NonTargetBeforeSummary.ClosedDeals >= 3 && attribution.NonTargetAfterSummary.ClosedDeals >= 3 &&
		attribution.NonTargetAfterSummary.NetPnL < attribution.NonTargetBeforeSummary.NetPnL-0.01 {
		warnings = append(warnings,
			fmt.Sprintf("Non-target cohort regressed after the change (%+.2f before vs %+.2f after).",
				attribution.NonTargetBeforeSummary.NetPnL,
				attribution.NonTargetAfterSummary.NetPnL,
			),
		)
	}

	rollbackSuggested = targetUnderperforming || (attribution.TargetBeforeSummary == nil && fullUnderperforming) || (fullUnderperforming && targetUnderperforming)
	return warnings, rollbackSuggested
}

func (s *DealReviewStore) buildStrategyVersionAttribution(version *DealReviewStrategyVersion, targetCohort map[string]any, compareSummary string) (*DealReviewStrategyVersionAttribution, error) {
	if version == nil {
		return nil, nil
	}

	appliedAt := effectiveDealReviewStrategyVersionTime(*version)
	if appliedAt.IsZero() {
		return nil, nil
	}

	if strings.TrimSpace(version.SourceType) == "ai_challenger_candidate" && strings.TrimSpace(version.SourceCompareID) != "" {
		note := strings.TrimSpace(compareSummary)
		if note == "" {
			note = "This challenger candidate is evaluated through the linked challenger comparison rather than an in-place strategy apply."
		}
		return &DealReviewStrategyVersionAttribution{
			ObservationReady:  false,
			BeforeEnd:         appliedAt,
			AfterStart:        appliedAt,
			Note:              note,
			RollbackSuggested: false,
		}, nil
	}

	previous, next, err := s.getStrategyVersionNeighbors(version)
	if err != nil {
		return nil, err
	}

	afterStart := appliedAt
	afterEnd := time.Now().UTC()
	if next != nil {
		nextAt := effectiveDealReviewStrategyVersionTime(*next)
		if !nextAt.IsZero() {
			afterEnd = nextAt
		}
	}
	if !afterEnd.After(afterStart) {
		afterEnd = time.Now().UTC()
	}

	beforeEnd := appliedAt
	var beforeStart time.Time
	if previous != nil {
		beforeStart = effectiveDealReviewStrategyVersionTime(*previous)
	}
	if beforeStart.IsZero() || !beforeStart.Before(beforeEnd) {
		if afterEnd.After(afterStart) {
			beforeStart = beforeEnd.Add(-afterEnd.Sub(afterStart))
		} else {
			beforeStart = beforeEnd.Add(-7 * 24 * time.Hour)
		}
	}
	if !beforeStart.Before(beforeEnd) {
		beforeStart = beforeEnd.Add(-7 * 24 * time.Hour)
	}

	fullBeforeCases, err := s.listClosedCasesForVersionWindow(version.UserID, version.TraderID, beforeStart, beforeEnd, nil)
	if err != nil {
		return nil, err
	}
	fullAfterCases, err := s.listClosedCasesForVersionWindow(version.UserID, version.TraderID, afterStart, afterEnd, nil)
	if err != nil {
		return nil, err
	}

	attribution := &DealReviewStrategyVersionAttribution{
		ObservationReady:  len(fullAfterCases) > 0,
		BeforeStart:       beforeStart,
		BeforeEnd:         beforeEnd,
		AfterStart:        afterStart,
		AfterEnd:          afterEnd,
		FullBeforeSummary: summarizeDealReviewClosedCaseRows(fullBeforeCases),
		FullAfterSummary:  summarizeDealReviewClosedCaseRows(fullAfterCases),
	}

	if len(targetCohort) > 0 {
		targetBeforeCases, err := s.listClosedCasesForVersionWindow(version.UserID, version.TraderID, beforeStart, beforeEnd, targetCohort)
		if err != nil {
			return nil, err
		}
		targetAfterCases, err := s.listClosedCasesForVersionWindow(version.UserID, version.TraderID, afterStart, afterEnd, targetCohort)
		if err != nil {
			return nil, err
		}
		attribution.TargetBeforeSummary = summarizeDealReviewClosedCaseRows(targetBeforeCases)
		attribution.TargetAfterSummary = summarizeDealReviewClosedCaseRows(targetAfterCases)
		attribution.NonTargetBeforeSummary = summarizeDealReviewClosedCaseRows(excludeDealReviewCasesByID(fullBeforeCases, targetBeforeCases))
		attribution.NonTargetAfterSummary = summarizeDealReviewClosedCaseRows(excludeDealReviewCasesByID(fullAfterCases, targetAfterCases))
	}

	if attribution.FullAfterSummary != nil && attribution.FullAfterSummary.ClosedDeals == 0 {
		attribution.ObservationReady = false
		attribution.Note = "No closed deals have been observed yet since this version was applied."
	}

	attribution.Warnings, attribution.RollbackSuggested = buildDealReviewAttributionWarnings(attribution)
	return attribution, nil
}

func (s *DealReviewStore) GetStrategyVersion(userID, traderID, versionID string) (*DealReviewStrategyVersionDetail, error) {
	var version DealReviewStrategyVersion
	if err := s.db.Where("id = ? AND user_id = ? AND trader_id = ?", versionID, userID, traderID).First(&version).Error; err != nil {
		return nil, err
	}
	detail := &DealReviewStrategyVersionDetail{
		Version:        version,
		PreviousConfig: map[string]any{},
		NextConfig:     map[string]any{},
		TargetCohort:   map[string]any{},
	}
	if strings.TrimSpace(version.PreviousConfigJSON) != "" {
		_ = json.Unmarshal([]byte(version.PreviousConfigJSON), &detail.PreviousConfig)
	}
	if strings.TrimSpace(version.NextConfigJSON) != "" {
		_ = json.Unmarshal([]byte(version.NextConfigJSON), &detail.NextConfig)
	}
	if strings.TrimSpace(version.TargetCohortJSON) != "" && strings.TrimSpace(version.TargetCohortJSON) != "{}" {
		_ = json.Unmarshal([]byte(version.TargetCohortJSON), &detail.TargetCohort)
		detail.TargetCohort = sanitizeDealReviewTargetCohortMap(detail.TargetCohort)
	}
	if version.SourceScanID != "" {
		var scan DealReviewAIScan
		if err := s.db.Select("id", "summary").Where("id = ?", version.SourceScanID).First(&scan).Error; err == nil {
			detail.SourceScanSummary = strings.TrimSpace(scan.Summary)
		}
	}
	if version.SourceCompareID != "" {
		var compare DealReviewChallengerCompare
		if err := s.db.Select("id", "summary").Where("id = ?", version.SourceCompareID).First(&compare).Error; err == nil {
			detail.CompareSummary = strings.TrimSpace(compare.Summary)
		}
	}
	attribution, err := s.buildStrategyVersionAttribution(&version, detail.TargetCohort, detail.CompareSummary)
	if err != nil {
		return nil, err
	}
	detail.Attribution = attribution
	return detail, nil
}

func (s *DealReviewStore) CaptureDecisionCyclePricePoints(userID string, record *DecisionRecord) error {
	if record == nil || strings.TrimSpace(record.TraderID) == "" || record.CycleNumber <= 0 || len(record.Positions) == 0 {
		return nil
	}

	timestampMs := record.Timestamp.UTC().UnixMilli()
	if timestampMs <= 0 {
		timestampMs = time.Now().UTC().UnixMilli()
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, snapshot := range record.Positions {
			position, err := s.findPositionForTimelineSnapshotTx(tx, record.TraderID, snapshot, timestampMs)
			if err != nil {
				return err
			}
			if position == nil {
				continue
			}
			if err := s.upsertCyclePointTx(tx, userID, record, position, snapshot, timestampMs); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *DealReviewStore) CaptureLivePositionPricePoints(userID, traderID string, positions []PositionSnapshot, capturedAt time.Time, source string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(traderID) == "" || len(positions) == 0 {
		return nil
	}

	timestampMs := capturedAt.UTC().UnixMilli()
	if timestampMs <= 0 {
		timestampMs = time.Now().UTC().UnixMilli()
	}
	timestampMs = normalizeDealReviewLivePointTimestampMs(timestampMs)
	source = normalizeDealReviewLivePointSource(source)

	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, snapshot := range positions {
			position, err := s.findPositionForTimelineSnapshotTx(tx, traderID, snapshot, timestampMs)
			if err != nil {
				return err
			}
			if position == nil {
				continue
			}
			if err := s.upsertMarketPointTx(tx, userID, traderID, position, snapshot, timestampMs, source); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *DealReviewStore) BackfillDecisionCyclePricePoints() error {
	const batchSize = 40
	nowMs := time.Now().UTC().UnixMilli()

	for offset := 0; ; offset += batchSize {
		var cases []DealReviewCase
		if err := s.db.
			Order("CASE WHEN exit_time_ms > 0 THEN exit_time_ms ELSE entry_time_ms END DESC").
			Offset(offset).
			Limit(batchSize).
			Find(&cases).Error; err != nil {
			return err
		}
		if len(cases) == 0 {
			return nil
		}

		for i := range cases {
			if err := s.backfillCaseDecisionCyclePricePoints(&cases[i], nowMs); err != nil {
				return err
			}
		}
	}
}

func (s *DealReviewStore) backfillQualityMetrics() error {
	const batchSize = 80

	for offset := 0; ; offset += batchSize {
		var cases []DealReviewCase
		if err := s.db.
			Where("status = ?", DealReviewCaseStatusClosed).
			Order("CASE WHEN exit_time_ms > 0 THEN exit_time_ms ELSE entry_time_ms END DESC").
			Offset(offset).
			Limit(batchSize).
			Find(&cases).Error; err != nil {
			return err
		}
		if len(cases) == 0 {
			return nil
		}

		positionIDs := make([]int64, 0, len(cases))
		for _, caseRec := range cases {
			if caseRec.PositionID > 0 {
				positionIDs = append(positionIDs, caseRec.PositionID)
			}
		}
		pointsByPosition, err := s.loadTimelinePointsByPositionIDs(positionIDs)
		if err != nil {
			return err
		}

		if err := s.db.Transaction(func(tx *gorm.DB) error {
			for i := range cases {
				quality := computeDealReviewCaseQuality(&cases[i], buildDealReviewTimelinePoints(&cases[i], pointsByPosition[cases[i].PositionID]))
				if err := applyDealReviewCaseQualityTx(tx, &cases[i], quality); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
}

func (s *DealReviewStore) ListStrategyVersions(userID, traderID string, limit int) ([]DealReviewStrategyVersionDetail, error) {
	if limit <= 0 {
		limit = 10
	}
	var versions []DealReviewStrategyVersion
	if err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).
		Order("created_at DESC").
		Limit(limit).
		Find(&versions).Error; err != nil {
		return nil, err
	}
	result := make([]DealReviewStrategyVersionDetail, 0, len(versions))
	for _, version := range versions {
		detail, err := s.GetStrategyVersion(userID, traderID, version.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, *detail)
	}
	return result, nil
}

func (s *DealReviewStore) UpdateCaseReview(userID, traderID, caseID string, labels []string, analystNote string) (*DealReviewCaseDetail, error) {
	labelsJSON, _ := json.Marshal(normalizeDealReviewLabels(labels))
	if err := s.db.Model(&DealReviewCase{}).
		Where("id = ? AND user_id = ? AND trader_id = ?", caseID, userID, traderID).
		Updates(map[string]any{
			"labels_json":  string(labelsJSON),
			"analyst_note": strings.TrimSpace(analystNote),
			"updated_at":   time.Now().UTC(),
		}).Error; err != nil {
		return nil, err
	}
	return s.GetCaseDetail(userID, traderID, caseID)
}

type dealReviewHeuristicClassifierExample struct {
	Case             DealReviewCase
	Labels           []string
	CandidateSources []string
}

type dealReviewClassifierFeedbackCounts struct {
	Accepted int
	Rejected int
}

type dealReviewHeuristicClassifierModel struct {
	Examples       []dealReviewHeuristicClassifierExample
	FeedbackByCase map[string]map[string]string
	GlobalFeedback map[string]dealReviewClassifierFeedbackCounts
}

func (s *DealReviewStore) ApplyClassifierFeedback(userID, traderID, caseID, classifierID, suggestionKey, label, issueType, verdict, rationale string, applyLabel bool) (*DealReviewCaseDetail, error) {
	classifierID = strings.TrimSpace(classifierID)
	label = strings.TrimSpace(label)
	issueType = normalizeDealReviewClassifierIssueType(issueType)
	verdict = normalizeDealReviewClassifierVerdict(verdict)
	if classifierID == "" {
		return nil, fmt.Errorf("classifier_id cannot be empty")
	}
	if verdict == "" {
		return nil, fmt.Errorf("classifier verdict cannot be empty")
	}
	if suggestionKey = strings.TrimSpace(suggestionKey); suggestionKey == "" {
		suggestionKey = buildDealReviewClassifierSuggestionKey(classifierID, label, issueType)
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var caseRec DealReviewCase
		if err := tx.Where("id = ? AND user_id = ? AND trader_id = ?", caseID, userID, traderID).First(&caseRec).Error; err != nil {
			return err
		}

		feedback := &DealReviewClassifierFeedback{
			ID:            uuid.NewString(),
			UserID:        userID,
			TraderID:      traderID,
			CaseID:        caseID,
			ClassifierID:  classifierID,
			SuggestionKey: suggestionKey,
			Label:         label,
			IssueType:     issueType,
			Verdict:       verdict,
			Rationale:     strings.TrimSpace(rationale),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "case_id"},
				{Name: "classifier_id"},
				{Name: "suggestion_key"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"label":      feedback.Label,
				"issue_type": feedback.IssueType,
				"verdict":    feedback.Verdict,
				"rationale":  feedback.Rationale,
				"updated_at": time.Now().UTC(),
			}),
		}).Create(feedback).Error; err != nil {
			return err
		}

		if applyLabel && verdict == DealReviewClassifierVerdictAccepted && label != "" {
			nextLabels := normalizeDealReviewLabels(append(parseJSONStringSlice(caseRec.LabelsJSON), label))
			labelsJSON, _ := json.Marshal(nextLabels)
			if err := tx.Model(&DealReviewCase{}).
				Where("id = ?", caseRec.ID).
				Updates(map[string]any{
					"labels_json": string(labelsJSON),
					"updated_at":  time.Now().UTC(),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return s.GetCaseDetail(userID, traderID, caseID)
}

func (s *DealReviewStore) BuildHeuristicClassifierAssist(userID, traderID string, caseRec *DealReviewCase) (*DealReviewClassifierAssist, error) {
	if caseRec == nil {
		return nil, nil
	}
	model, err := s.buildHeuristicClassifierModel(userID, traderID)
	if err != nil {
		return nil, err
	}
	return model.EvaluateCase(caseRec), nil
}

func (s *DealReviewStore) ListCaseClassifierFeedback(userID, traderID, caseID, classifierID string) ([]DealReviewClassifierFeedback, error) {
	var rows []DealReviewClassifierFeedback
	query := s.db.Where("user_id = ? AND trader_id = ? AND case_id = ?", userID, traderID, caseID)
	if trimmed := strings.TrimSpace(classifierID); trimmed != "" {
		query = query.Where("classifier_id = ?", trimmed)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *DealReviewStore) buildHeuristicClassifierModel(userID, traderID string) (*dealReviewHeuristicClassifierModel, error) {
	model := &dealReviewHeuristicClassifierModel{
		FeedbackByCase: map[string]map[string]string{},
		GlobalFeedback: map[string]dealReviewClassifierFeedbackCounts{},
	}

	var labeledCases []DealReviewCase
	if err := s.db.
		Where("user_id = ? AND trader_id = ? AND labels_json IS NOT NULL AND labels_json != '' AND labels_json != '[]'",
			userID, traderID).
		Find(&labeledCases).Error; err != nil {
		return nil, err
	}
	model.Examples = make([]dealReviewHeuristicClassifierExample, 0, len(labeledCases))
	for _, item := range labeledCases {
		labels := normalizeDealReviewLabels(parseJSONStringSlice(item.LabelsJSON))
		if len(labels) == 0 {
			continue
		}
		filteredLabels := make([]string, 0, len(labels))
		for _, label := range labels {
			if issueType := inferDealReviewClassifierIssueTypeFromLabel(label); issueType != "" || isDealReviewProblemLabel(label) {
				filteredLabels = append(filteredLabels, label)
			}
		}
		if len(filteredLabels) == 0 {
			continue
		}
		model.Examples = append(model.Examples, dealReviewHeuristicClassifierExample{
			Case:             item,
			Labels:           filteredLabels,
			CandidateSources: parseJSONStringSlice(item.OpenCandidateSourcesJSON),
		})
	}

	var feedbackRows []DealReviewClassifierFeedback
	if err := s.db.
		Where("user_id = ? AND trader_id = ? AND classifier_id = ?", userID, traderID, DealReviewClassifierHeuristic).
		Find(&feedbackRows).Error; err != nil {
		return nil, err
	}
	for _, row := range feedbackRows {
		perCase := model.FeedbackByCase[row.CaseID]
		if perCase == nil {
			perCase = map[string]string{}
			model.FeedbackByCase[row.CaseID] = perCase
		}
		perCase[row.SuggestionKey] = normalizeDealReviewClassifierVerdict(row.Verdict)

		counts := model.GlobalFeedback[row.SuggestionKey]
		switch normalizeDealReviewClassifierVerdict(row.Verdict) {
		case DealReviewClassifierVerdictAccepted:
			counts.Accepted++
		case DealReviewClassifierVerdictRejected:
			counts.Rejected++
		}
		model.GlobalFeedback[row.SuggestionKey] = counts
	}

	return model, nil
}

func (m *dealReviewHeuristicClassifierModel) EvaluateCase(caseRec *DealReviewCase) *DealReviewClassifierAssist {
	if caseRec == nil || len(m.Examples) == 0 {
		return nil
	}

	type aggregate struct {
		Label         string
		IssueType     string
		SuggestionKey string
		Score         float64
		EvidenceCount int
		Rationale     string
	}

	currentLabels := map[string]struct{}{}
	for _, label := range normalizeDealReviewLabels(parseJSONStringSlice(caseRec.LabelsJSON)) {
		currentLabels[label] = struct{}{}
	}
	currentSources := parseJSONStringSlice(caseRec.OpenCandidateSourcesJSON)
	caseFeedback := m.FeedbackByCase[caseRec.ID]

	aggregates := map[string]*aggregate{}
	for _, example := range m.Examples {
		if example.Case.ID == caseRec.ID {
			continue
		}
		matchScore := scoreDealReviewClassifierCaseMatch(caseRec, currentSources, &example.Case, example.CandidateSources)
		if matchScore < 4 {
			continue
		}
		for _, label := range example.Labels {
			if _, exists := currentLabels[label]; exists {
				continue
			}
			issueType := inferDealReviewClassifierIssueTypeFromLabel(label)
			if issueType == "" && !isDealReviewProblemLabel(label) {
				continue
			}
			suggestionKey := buildDealReviewClassifierSuggestionKey(DealReviewClassifierHeuristic, label, issueType)
			if caseFeedback != nil && caseFeedback[suggestionKey] == DealReviewClassifierVerdictRejected {
				continue
			}
			entry := aggregates[suggestionKey]
			if entry == nil {
				entry = &aggregate{
					Label:         label,
					IssueType:     issueType,
					SuggestionKey: suggestionKey,
				}
				aggregates[suggestionKey] = entry
			}
			entry.Score += matchScore
			entry.EvidenceCount++
			entry.Rationale = buildDealReviewClassifierRationale(caseRec, entry.EvidenceCount)
		}
	}

	suggestions := make([]DealReviewClassifierSuggestion, 0, len(aggregates))
	for _, entry := range aggregates {
		if entry.EvidenceCount < 2 && entry.Score < 8 {
			continue
		}
		counts := m.GlobalFeedback[entry.SuggestionKey]
		finalScore := entry.Score + float64(counts.Accepted)*0.75 - float64(counts.Rejected)*0.45
		suggestions = append(suggestions, DealReviewClassifierSuggestion{
			ClassifierID:    DealReviewClassifierHeuristic,
			SuggestionKey:   entry.SuggestionKey,
			Label:           entry.Label,
			IssueType:       entry.IssueType,
			Score:           finalScore,
			HighlightLevel:  dealReviewClassifierHighlightLevel(finalScore),
			EvidenceCount:   entry.EvidenceCount,
			AcceptedCount:   counts.Accepted,
			RejectedCount:   counts.Rejected,
			FeedbackVerdict: caseFeedback[entry.SuggestionKey],
			Rationale:       entry.Rationale,
		})
	}

	if len(suggestions) == 0 {
		return nil
	}
	sort.SliceStable(suggestions, func(i, j int) bool {
		if math.Abs(suggestions[i].Score-suggestions[j].Score) > 1e-9 {
			return suggestions[i].Score > suggestions[j].Score
		}
		if suggestions[i].EvidenceCount != suggestions[j].EvidenceCount {
			return suggestions[i].EvidenceCount > suggestions[j].EvidenceCount
		}
		return suggestions[i].Label < suggestions[j].Label
	})
	if len(suggestions) > 4 {
		suggestions = suggestions[:4]
	}

	top := suggestions[0]
	return &DealReviewClassifierAssist{
		Source:         DealReviewClassifierHeuristic,
		Summary:        buildDealReviewClassifierAssistSummary(top, len(suggestions)),
		HighlightScore: top.Score,
		HighlightLevel: top.HighlightLevel,
		Suggestions:    suggestions,
	}
}

func buildDealReviewClassifierAssistSummary(top DealReviewClassifierSuggestion, suggestionCount int) string {
	if suggestionCount <= 1 {
		return fmt.Sprintf("Learned review assist suggests %q from %d matched labeled deals.", top.Label, top.EvidenceCount)
	}
	return fmt.Sprintf("Learned review assist found %d candidate labels; strongest signal is %q from %d matched labeled deals.", suggestionCount, top.Label, top.EvidenceCount)
}

func buildDealReviewClassifierRationale(caseRec *DealReviewCase, evidenceCount int) string {
	if caseRec == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if symbol := strings.TrimSpace(caseRec.Symbol); symbol != "" {
		parts = append(parts, "symbol "+symbol)
	}
	if bucket := strings.TrimSpace(caseRec.OpenSelectionBucket); bucket != "" {
		parts = append(parts, "bucket "+bucket)
	}
	if reason := strings.TrimSpace(caseRec.CloseReason); reason != "" {
		parts = append(parts, "close reason "+reason)
	}
	if side := strings.TrimSpace(caseRec.Side); side != "" {
		parts = append(parts, "side "+side)
	}
	if len(parts) > 3 {
		parts = parts[:3]
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Matched %d labeled historical deals with a similar trade profile.", evidenceCount)
	}
	return fmt.Sprintf("Matched %d labeled historical deals sharing %s.", evidenceCount, strings.Join(parts, ", "))
}

func dealReviewClassifierHighlightLevel(score float64) string {
	switch {
	case score >= 18:
		return "high"
	case score >= 10:
		return "medium"
	default:
		return "low"
	}
}

func scoreDealReviewClassifierCaseMatch(current *DealReviewCase, currentSources []string, example *DealReviewCase, exampleSources []string) float64 {
	if current == nil || example == nil {
		return 0
	}
	score := 0.0
	if strings.EqualFold(strings.TrimSpace(current.Symbol), strings.TrimSpace(example.Symbol)) {
		score += 4
	}
	if strings.EqualFold(strings.TrimSpace(current.Side), strings.TrimSpace(example.Side)) {
		score += 1.5
	}
	if strings.TrimSpace(current.OpenSelectionBucket) != "" &&
		strings.EqualFold(strings.TrimSpace(current.OpenSelectionBucket), strings.TrimSpace(example.OpenSelectionBucket)) {
		score += 3
	}
	if strings.TrimSpace(current.CloseReason) != "" &&
		strings.EqualFold(strings.TrimSpace(current.CloseReason), strings.TrimSpace(example.CloseReason)) {
		score += 3
	}
	if strings.TrimSpace(current.Outcome) != "" &&
		strings.EqualFold(strings.TrimSpace(current.Outcome), strings.TrimSpace(example.Outcome)) {
		score += 1
	}
	overlap := countDealReviewClassifierSourceOverlap(currentSources, exampleSources)
	if overlap > 0 {
		score += math.Min(2, float64(overlap))
	}
	return score
}

func countDealReviewClassifierSourceOverlap(left, right []string) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	seen := map[string]struct{}{}
	for _, item := range left {
		token := normalizeDealReviewClassifierToken(item)
		if token == "" {
			continue
		}
		seen[token] = struct{}{}
	}
	count := 0
	for _, item := range right {
		token := normalizeDealReviewClassifierToken(item)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			count++
		}
	}
	return count
}

func inferDealReviewClassifierIssueTypeFromLabel(label string) string {
	normalized := strings.ToLower(strings.TrimSpace(label))
	switch {
	case normalized == "":
		return ""
	case strings.Contains(normalized, "avoidable loss"),
		strings.Contains(normalized, "should skip"),
		strings.Contains(normalized, "skip trade"):
		return "likely_avoidable_loss"
	case strings.Contains(normalized, "regime mismatch"),
		strings.Contains(normalized, "wrong regime"),
		strings.Contains(normalized, "chop regime"):
		return "likely_regime_mismatch"
	case strings.Contains(normalized, "bad exit"),
		strings.Contains(normalized, "poor exit"),
		strings.Contains(normalized, "late exit"),
		strings.Contains(normalized, "give back"),
		strings.Contains(normalized, "gave back"):
		return "likely_bad_exit"
	case strings.Contains(normalized, "bad trade"),
		strings.Contains(normalized, "bad entry"),
		strings.Contains(normalized, "poor trade"),
		strings.Contains(normalized, "poor entry"),
		strings.Contains(normalized, "weak setup"),
		strings.Contains(normalized, "fomo"),
		strings.Contains(normalized, "overtrade"),
		strings.Contains(normalized, "revenge"),
		strings.Contains(normalized, "chase"):
		return "likely_bad_trade"
	default:
		return ""
	}
}

func isDealReviewProblemLabel(label string) bool {
	normalized := strings.ToLower(strings.TrimSpace(label))
	if normalized == "" {
		return false
	}
	if inferDealReviewClassifierIssueTypeFromLabel(normalized) != "" {
		return true
	}
	for _, token := range []string{"loss", "bad", "weak", "late", "mistake", "wrong", "avoid", "skip"} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	for _, token := range []string{"good entry", "good exit", "good trade", "valid trade"} {
		if strings.Contains(normalized, token) {
			return false
		}
	}
	return false
}

func normalizeDealReviewClassifierIssueType(issueType string) string {
	switch strings.TrimSpace(strings.ToLower(issueType)) {
	case "likely_bad_trade":
		return "likely_bad_trade"
	case "likely_bad_exit":
		return "likely_bad_exit"
	case "likely_avoidable_loss":
		return "likely_avoidable_loss"
	case "likely_regime_mismatch":
		return "likely_regime_mismatch"
	case "other_review_signal":
		return "other_review_signal"
	default:
		return ""
	}
}

func normalizeDealReviewClassifierVerdict(verdict string) string {
	switch strings.TrimSpace(strings.ToLower(verdict)) {
	case DealReviewClassifierVerdictAccepted:
		return DealReviewClassifierVerdictAccepted
	case DealReviewClassifierVerdictRejected:
		return DealReviewClassifierVerdictRejected
	default:
		return ""
	}
}

func buildDealReviewClassifierSuggestionKey(classifierID, label, issueType string) string {
	parts := []string{
		normalizeDealReviewClassifierToken(classifierID),
		normalizeDealReviewClassifierToken(label),
		normalizeDealReviewClassifierToken(issueType),
	}
	return strings.Trim(strings.Join(parts, ":"), ":")
}

func normalizeDealReviewClassifierToken(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range trimmed {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func cloneDealReviewChallengerMetrics(metrics *DealReviewChallengerMetrics) *DealReviewChallengerMetrics {
	if metrics == nil {
		return nil
	}
	copy := *metrics
	return &copy
}

func ParseDealReviewChallengerProtocolJSON(raw string) []DealReviewChallengerProtocolEvent {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" {
		return nil
	}
	var events []DealReviewChallengerProtocolEvent
	if err := json.Unmarshal([]byte(trimmed), &events); err != nil {
		return nil
	}
	for i := range events {
		events[i].Type = strings.TrimSpace(events[i].Type)
		events[i].Actor = strings.TrimSpace(events[i].Actor)
		events[i].Message = strings.TrimSpace(events[i].Message)
		events[i].Metrics = cloneDealReviewChallengerMetrics(events[i].Metrics)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	return events
}

func AppendDealReviewChallengerProtocolEvent(raw string, event DealReviewChallengerProtocolEvent) string {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	event.Type = strings.TrimSpace(event.Type)
	event.Actor = strings.TrimSpace(event.Actor)
	event.Message = strings.TrimSpace(event.Message)
	event.Metrics = cloneDealReviewChallengerMetrics(event.Metrics)

	events := ParseDealReviewChallengerProtocolJSON(raw)
	events = append(events, event)
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	payload, err := json.Marshal(events)
	if err != nil {
		payload, _ = json.Marshal([]DealReviewChallengerProtocolEvent{event})
	}
	return string(payload)
}

func (s *DealReviewStore) GetAIScan(userID, traderID, scanID string) (*DealReviewAIScanDetail, error) {
	var scan DealReviewAIScan
	err := s.db.Where("id = ? AND user_id = ? AND trader_id = ?", scanID, userID, traderID).First(&scan).Error
	if err != nil {
		return nil, err
	}

	result := DealReviewAIScanResult{}
	if strings.TrimSpace(scan.ResultJSON) != "" {
		_ = json.Unmarshal([]byte(scan.ResultJSON), &result)
	}
	filters := map[string]any{}
	if strings.TrimSpace(scan.FilterJSON) != "" {
		_ = json.Unmarshal([]byte(scan.FilterJSON), &filters)
	}
	patch := map[string]any{}
	if strings.TrimSpace(scan.StrategyPatchJSON) != "" {
		_ = json.Unmarshal([]byte(scan.StrategyPatchJSON), &patch)
	}
	var validation *DealReviewAIScanValidation
	if strings.TrimSpace(scan.ValidationJSON) != "" && strings.TrimSpace(scan.ValidationJSON) != "{}" {
		parsed := &DealReviewAIScanValidation{}
		if err := json.Unmarshal([]byte(scan.ValidationJSON), parsed); err == nil {
			if strings.TrimSpace(parsed.Status) == "" {
				parsed.Status = scan.ValidationStatus
			}
			if parsed.ValidatedAt.IsZero() && !scan.ValidatedAt.IsZero() {
				parsed.ValidatedAt = scan.ValidatedAt
			}
			validation = parsed
		}
	}
	return &DealReviewAIScanDetail{
		Scan:          scan,
		Filters:       filters,
		Result:        result,
		StrategyPatch: patch,
		Validation:    validation,
	}, nil
}

func (s *DealReviewStore) ListAIScans(userID, traderID string, limit int) ([]DealReviewAIScanDetail, error) {
	if limit <= 0 {
		limit = 10
	}
	var scans []DealReviewAIScan
	err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).
		Order("created_at DESC").
		Limit(limit).
		Find(&scans).Error
	if err != nil {
		return nil, err
	}
	result := make([]DealReviewAIScanDetail, 0, len(scans))
	for _, scan := range scans {
		detail, err := s.GetAIScan(userID, traderID, scan.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, *detail)
	}
	return result, nil
}

func (s *DealReviewStore) MarkAIScanApplied(userID, traderID, scanID string) error {
	return s.db.Model(&DealReviewAIScan{}).
		Where("id = ? AND user_id = ? AND trader_id = ?", scanID, userID, traderID).
		Updates(map[string]any{
			"applied_at": time.Now().UTC(),
			"updated_at": time.Now().UTC(),
		}).Error
}

func (s *DealReviewStore) UpdateAIScanValidation(userID, traderID, scanID string, validation *DealReviewAIScanValidation) error {
	if validation == nil {
		return fmt.Errorf("validation cannot be nil")
	}
	if validation.ValidatedAt.IsZero() {
		validation.ValidatedAt = time.Now().UTC()
	}
	payload, _ := json.Marshal(validation)
	return s.db.Model(&DealReviewAIScan{}).
		Where("id = ? AND user_id = ? AND trader_id = ?", scanID, userID, traderID).
		Updates(map[string]any{
			"validation_status":  strings.TrimSpace(validation.Status),
			"validation_summary": buildDealReviewValidationSummary(validation),
			"validation_json":    string(payload),
			"validated_at":       validation.ValidatedAt.UTC(),
			"updated_at":         time.Now().UTC(),
		}).Error
}

func (s *DealReviewStore) GetChallengerCompare(userID, traderID, compareID string) (*DealReviewChallengerCompareDetail, error) {
	var compare DealReviewChallengerCompare
	if err := s.db.Where("id = ? AND user_id = ? AND trader_id = ?", compareID, userID, traderID).First(&compare).Error; err != nil {
		return nil, err
	}
	return s.buildChallengerCompareDetail(&compare)
}

func (s *DealReviewStore) GetChallengerCompareRecord(userID, traderID, compareID string) (*DealReviewChallengerCompare, error) {
	var compare DealReviewChallengerCompare
	if err := s.db.Where("id = ? AND user_id = ? AND trader_id = ?", compareID, userID, traderID).First(&compare).Error; err != nil {
		return nil, err
	}
	return &compare, nil
}

func (s *DealReviewStore) ListChallengerCompares(userID, traderID string, limit int) ([]DealReviewChallengerCompareDetail, error) {
	if limit <= 0 {
		limit = 10
	}
	var compares []DealReviewChallengerCompare
	if err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).
		Order("created_at DESC").
		Limit(limit).
		Find(&compares).Error; err != nil {
		return nil, err
	}
	result := make([]DealReviewChallengerCompareDetail, 0, len(compares))
	for i := range compares {
		detail, err := s.buildChallengerCompareDetail(&compares[i])
		if err != nil {
			return nil, err
		}
		result = append(result, *detail)
	}
	return result, nil
}

func (s *DealReviewStore) ListActiveChallengerCompares(limit int) ([]DealReviewChallengerCompare, error) {
	if limit <= 0 {
		limit = 50
	}
	var compares []DealReviewChallengerCompare
	if err := s.db.Where("status IN ?", []string{DealReviewChallengerStatusStarting, DealReviewChallengerStatusRunning}).
		Order("ends_at ASC").
		Limit(limit).
		Find(&compares).Error; err != nil {
		return nil, err
	}
	return compares, nil
}

func (s *DealReviewStore) buildChallengerCompareDetail(compare *DealReviewChallengerCompare) (*DealReviewChallengerCompareDetail, error) {
	if compare == nil {
		return nil, fmt.Errorf("challenger compare cannot be nil")
	}

	detail := &DealReviewChallengerCompareDetail{Compare: *compare}
	if strings.TrimSpace(compare.MetricsJSON) != "" && strings.TrimSpace(compare.MetricsJSON) != "{}" {
		metrics := &DealReviewChallengerMetrics{}
		if err := json.Unmarshal([]byte(compare.MetricsJSON), metrics); err == nil {
			detail.Metrics = metrics
		}
	}
	if strings.TrimSpace(compare.ProtocolJSON) != "" && strings.TrimSpace(compare.ProtocolJSON) != "[]" {
		detail.Protocol = ParseDealReviewChallengerProtocolJSON(compare.ProtocolJSON)
	}

	if compare.IncumbentTraderID != "" {
		var trader Trader
		if err := s.db.Select("id", "name").Where("id = ?", compare.IncumbentTraderID).First(&trader).Error; err == nil {
			detail.IncumbentTraderName = trader.Name
		}
	}
	if compare.ChallengerTraderID != "" {
		var trader Trader
		if err := s.db.Select("id", "name").Where("id = ?", compare.ChallengerTraderID).First(&trader).Error; err == nil {
			detail.ChallengerTraderName = trader.Name
		}
	}
	if compare.IncumbentStrategyID != "" {
		var strategy Strategy
		if err := s.db.Select("id", "name").Where("id = ?", compare.IncumbentStrategyID).First(&strategy).Error; err == nil {
			detail.IncumbentStrategyName = strategy.Name
		}
	}
	if compare.ChallengerStrategyID != "" {
		var strategy Strategy
		if err := s.db.Select("id", "name").Where("id = ?", compare.ChallengerStrategyID).First(&strategy).Error; err == nil {
			detail.ChallengerStrategyName = strategy.Name
		}
	}
	if compare.ChallengerExchangeID != "" {
		var exchange Exchange
		if err := s.db.Select("id", "name", "account_name", "exchange_type").Where("id = ?", compare.ChallengerExchangeID).First(&exchange).Error; err == nil {
			label := strings.TrimSpace(exchange.Name)
			if strings.TrimSpace(exchange.AccountName) != "" {
				label = fmt.Sprintf("%s / %s", label, exchange.AccountName)
			}
			if label == "" {
				label = strings.TrimSpace(exchange.ExchangeType)
			}
			detail.ChallengerExchangeName = label
		}
	}
	if compare.SourceScanID != "" {
		var scan DealReviewAIScan
		if err := s.db.Select("id", "summary").Where("id = ?", compare.SourceScanID).First(&scan).Error; err == nil {
			detail.SourceScanSummary = scan.Summary
		}
	}

	return detail, nil
}

type DealReviewTraderWindowPnL struct {
	RealizedPnL float64
	TotalFees   float64
	ClosedCount int64
	AvgHoldMs   float64
}

func (s *DealReviewStore) GetTraderWindowPnL(traderID string, startMs, endMs int64) (*DealReviewTraderWindowPnL, error) {
	result := &DealReviewTraderWindowPnL{}
	err := s.db.Model(&TraderPosition{}).
		Select("COALESCE(SUM(realized_pnl), 0) as realized_pnl, COALESCE(SUM(fee), 0) as total_fees, COUNT(*) as closed_count, COALESCE(AVG(CASE WHEN exit_time > entry_time THEN exit_time - entry_time ELSE 0 END), 0) as avg_hold_ms").
		Where("trader_id = ? AND status = ? AND exit_time >= ? AND exit_time <= ?", traderID, "CLOSED", startMs, endMs).
		Scan(result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func buildDealReviewValidationSummary(validation *DealReviewAIScanValidation) string {
	if validation == nil {
		return ""
	}
	if strings.TrimSpace(validation.Status) == DealReviewAIScanValidationPassed {
		if validation.RecentClosedDealCount > 0 {
			return fmt.Sprintf("Evidence gate passed on %d closed deals (%d train / %d holdout / %d recent).",
				validation.ClosedDealCount,
				validation.TrainingClosedDealCount,
				validation.HoldoutClosedDealCount,
				validation.RecentClosedDealCount,
			)
		}
		return fmt.Sprintf("Evidence gate passed on %d closed deals (%d train / %d holdout).",
			validation.ClosedDealCount,
			validation.TrainingClosedDealCount,
			validation.HoldoutClosedDealCount,
		)
	}
	if len(validation.BlockingIssues) > 0 {
		return validation.BlockingIssues[0]
	}
	return "Validation has not been completed yet."
}

func (s *DealReviewStore) GetAnomalySummary(userID string, filter DealReviewListFilter) (*DealReviewAnomalySummary, error) {
	query := s.buildCaseFilterQuery(userID, filter).Where("status = ?", DealReviewCaseStatusClosed)
	var cases []DealReviewCase
	if err := query.Find(&cases).Error; err != nil {
		return nil, err
	}

	summary := &DealReviewAnomalySummary{ClosedDeals: int64(len(cases))}
	if len(cases) == 0 {
		return summary, nil
	}

	type symbolAgg struct {
		Deals        int64
		LosingDeals  int64
		NetPnL       float64
		HoldMs       int64
		WinningDeals int64
	}
	type bucketAgg struct {
		Deals  int64
		Wins   int64
		NetPnL float64
	}
	type reasonAgg struct {
		Deals  int64
		NetPnL float64
	}
	type giveBackAgg struct {
		Deals             int64
		NetPnL            float64
		GiveBackPctSum    float64
		MFECapturedPctSum float64
	}
	type stopOutAgg struct {
		Deals     int64
		NetPnL    float64
		HoldMs    int64
		MAEPctSum float64
	}
	type sizingAgg struct {
		Deals              int64
		NetPnL             float64
		RiskSizingScoreSum float64
		PlannedRiskPctSum  float64
	}
	type closeReasonQualityAgg struct {
		Deals                  int64
		NetPnL                 float64
		ExitEfficiencyScoreSum float64
		MFECapturedPctSum      float64
		GiveBackPctSum         float64
	}
	type exitUncertaintyAgg struct {
		Deals  int64
		NetPnL float64
	}

	symbols := map[string]*symbolAgg{}
	buckets := map[string]*bucketAgg{}
	closeReasons := map[string]*reasonAgg{}
	giveBackSymbols := map[string]*giveBackAgg{}
	stopOutSymbols := map[string]*stopOutAgg{}
	sizingSymbols := map[string]*sizingAgg{}
	closeReasonQuality := map[string]*closeReasonQualityAgg{}
	exitUncertainty := map[string]*exitUncertaintyAgg{}
	type symbolPriorSliceAgg struct {
		Deals  int64
		NetPnL float64
	}
	symbolPriorSliceMatches := map[string]*symbolPriorSliceAgg{}

	for _, c := range cases {
		symbol := strings.TrimSpace(strings.ToUpper(c.Symbol))
		if symbol == "" {
			symbol = "UNKNOWN"
		}
		sym := symbols[symbol]
		if sym == nil {
			sym = &symbolAgg{}
			symbols[symbol] = sym
		}
		sym.Deals++
		sym.NetPnL += c.RealizedPnL
		sym.HoldMs += c.HoldDurationMs
		if c.RealizedPnL > 0 {
			sym.WinningDeals++
		}
		if c.RealizedPnL < 0 {
			sym.LosingDeals++
		}

		bucket := strings.TrimSpace(c.OpenSelectionBucket)
		if bucket == "" {
			bucket = "unclassified"
		}
		bk := buckets[bucket]
		if bk == nil {
			bk = &bucketAgg{}
			buckets[bucket] = bk
		}
		bk.Deals++
		bk.NetPnL += c.RealizedPnL
		if c.RealizedPnL > 0 {
			bk.Wins++
		}

		reason := strings.TrimSpace(c.CloseReason)
		if reason == "" {
			reason = "unspecified"
		}
		rs := closeReasons[reason]
		if rs == nil {
			rs = &reasonAgg{}
			closeReasons[reason] = rs
		}
		rs.Deals++
		rs.NetPnL += c.RealizedPnL

		qr := closeReasonQuality[reason]
		if qr == nil {
			qr = &closeReasonQualityAgg{}
			closeReasonQuality[reason] = qr
		}
		qr.Deals++
		qr.NetPnL += c.RealizedPnL
		qr.ExitEfficiencyScoreSum += c.ExitEfficiencyScore
		qr.MFECapturedPctSum += c.MFECapturedPct
		qr.GiveBackPctSum += c.ProfitGivenBackPct

		if isDealReviewExitUncertainty(c) {
			quality := strings.TrimSpace(c.ExitReasonQuality)
			if quality == "" {
				quality = "unspecified"
			}
			origin := strings.TrimSpace(c.ExitOrigin)
			if origin == "" {
				origin = "unspecified"
			}
			key := strings.Join([]string{reason, quality, origin}, "|")
			item := exitUncertainty[key]
			if item == nil {
				item = &exitUncertaintyAgg{}
				exitUncertainty[key] = item
			}
			item.Deals++
			item.NetPnL += c.RealizedPnL
		}

		if isDealReviewProfitGiveBackHotspot(c) {
			gb := giveBackSymbols[symbol]
			if gb == nil {
				gb = &giveBackAgg{}
				giveBackSymbols[symbol] = gb
			}
			gb.Deals++
			gb.NetPnL += c.RealizedPnL
			gb.GiveBackPctSum += c.ProfitGivenBackPct
			gb.MFECapturedPctSum += c.MFECapturedPct
		}

		if isDealReviewEarlyStopOut(c) {
			stop := stopOutSymbols[symbol]
			if stop == nil {
				stop = &stopOutAgg{}
				stopOutSymbols[symbol] = stop
			}
			stop.Deals++
			stop.NetPnL += c.RealizedPnL
			stop.HoldMs += c.HoldDurationMs
			stop.MAEPctSum += math.Abs(c.MaxAdverseExcursionPct)
		}

		if isDealReviewOversizedLoss(c) {
			size := sizingSymbols[symbol]
			if size == nil {
				size = &sizingAgg{}
				sizingSymbols[symbol] = size
			}
			size.Deals++
			size.NetPnL += c.RealizedPnL
			size.RiskSizingScoreSum += c.RiskSizingScore
			size.PlannedRiskPctSum += c.PlannedRiskPct
		}

		if key := dealReviewSymbolBehaviorPriorKeyForCase(&c); key != "" {
			match := symbolPriorSliceMatches[key]
			if match == nil {
				match = &symbolPriorSliceAgg{}
				symbolPriorSliceMatches[key] = match
			}
			match.Deals++
			match.NetPnL += c.RealizedPnL
		}
	}

	worstSymbols := make([]DealReviewAnomalySymbol, 0, len(symbols))
	overtraded := make([]DealReviewAnomalySymbol, 0, len(symbols))
	for symbol, agg := range symbols {
		item := DealReviewAnomalySymbol{
			Symbol:      symbol,
			Deals:       agg.Deals,
			NetPnL:      agg.NetPnL,
			AvgHoldMs:   0,
			LosingDeals: agg.LosingDeals,
		}
		if agg.Deals > 0 {
			item.WinRate = float64(agg.WinningDeals) / float64(agg.Deals) * 100
			item.AvgHoldMs = agg.HoldMs / agg.Deals
		}
		worstSymbols = append(worstSymbols, item)
		if agg.Deals >= 3 {
			overtraded = append(overtraded, item)
		}
	}
	sort.Slice(worstSymbols, func(i, j int) bool {
		if worstSymbols[i].NetPnL == worstSymbols[j].NetPnL {
			return worstSymbols[i].Deals > worstSymbols[j].Deals
		}
		return worstSymbols[i].NetPnL < worstSymbols[j].NetPnL
	})
	sort.Slice(overtraded, func(i, j int) bool {
		if overtraded[i].Deals == overtraded[j].Deals {
			return overtraded[i].NetPnL < overtraded[j].NetPnL
		}
		return overtraded[i].Deals > overtraded[j].Deals
	})

	weakBuckets := make([]DealReviewAnomalyBucket, 0, len(buckets))
	for bucket, agg := range buckets {
		item := DealReviewAnomalyBucket{
			Bucket: bucket,
			Deals:  agg.Deals,
			NetPnL: agg.NetPnL,
		}
		if agg.Deals > 0 {
			item.WinRate = float64(agg.Wins) / float64(agg.Deals) * 100
		}
		weakBuckets = append(weakBuckets, item)
	}
	sort.Slice(weakBuckets, func(i, j int) bool {
		if weakBuckets[i].NetPnL == weakBuckets[j].NetPnL {
			return weakBuckets[i].Deals > weakBuckets[j].Deals
		}
		return weakBuckets[i].NetPnL < weakBuckets[j].NetPnL
	})

	weakReasons := make([]DealReviewAnomalyCloseReason, 0, len(closeReasons))
	for reason, agg := range closeReasons {
		item := DealReviewAnomalyCloseReason{
			Reason: reason,
			Deals:  agg.Deals,
			NetPnL: agg.NetPnL,
		}
		if agg.Deals > 0 {
			item.AvgPnL = agg.NetPnL / float64(agg.Deals)
		}
		weakReasons = append(weakReasons, item)
	}
	sort.Slice(weakReasons, func(i, j int) bool {
		if weakReasons[i].NetPnL == weakReasons[j].NetPnL {
			return weakReasons[i].Deals > weakReasons[j].Deals
		}
		return weakReasons[i].NetPnL < weakReasons[j].NetPnL
	})

	giveBackHotspots := make([]DealReviewAnomalyGiveBack, 0, len(giveBackSymbols))
	for symbol, agg := range giveBackSymbols {
		item := DealReviewAnomalyGiveBack{
			Symbol: symbol,
			Deals:  agg.Deals,
			NetPnL: agg.NetPnL,
		}
		if agg.Deals > 0 {
			item.AvgGiveBackPct = agg.GiveBackPctSum / float64(agg.Deals)
			item.AvgMFECapturedPct = agg.MFECapturedPctSum / float64(agg.Deals)
		}
		giveBackHotspots = append(giveBackHotspots, item)
	}
	sort.Slice(giveBackHotspots, func(i, j int) bool {
		if giveBackHotspots[i].AvgGiveBackPct == giveBackHotspots[j].AvgGiveBackPct {
			return giveBackHotspots[i].Deals > giveBackHotspots[j].Deals
		}
		return giveBackHotspots[i].AvgGiveBackPct > giveBackHotspots[j].AvgGiveBackPct
	})

	stopOutHotspots := make([]DealReviewAnomalyStopOut, 0, len(stopOutSymbols))
	for symbol, agg := range stopOutSymbols {
		item := DealReviewAnomalyStopOut{
			Symbol: symbol,
			Deals:  agg.Deals,
			NetPnL: agg.NetPnL,
		}
		if agg.Deals > 0 {
			item.AvgHoldMs = agg.HoldMs / agg.Deals
			item.AvgMAEPct = agg.MAEPctSum / float64(agg.Deals)
		}
		stopOutHotspots = append(stopOutHotspots, item)
	}
	sort.Slice(stopOutHotspots, func(i, j int) bool {
		if stopOutHotspots[i].Deals == stopOutHotspots[j].Deals {
			return stopOutHotspots[i].AvgHoldMs < stopOutHotspots[j].AvgHoldMs
		}
		return stopOutHotspots[i].Deals > stopOutHotspots[j].Deals
	})

	oversizedLossHotspots := make([]DealReviewAnomalySizing, 0, len(sizingSymbols))
	for symbol, agg := range sizingSymbols {
		item := DealReviewAnomalySizing{
			Symbol: symbol,
			Deals:  agg.Deals,
			NetPnL: agg.NetPnL,
		}
		if agg.Deals > 0 {
			item.AvgRiskSizingScore = agg.RiskSizingScoreSum / float64(agg.Deals)
			item.AvgPlannedRiskPct = agg.PlannedRiskPctSum / float64(agg.Deals)
		}
		oversizedLossHotspots = append(oversizedLossHotspots, item)
	}
	sort.Slice(oversizedLossHotspots, func(i, j int) bool {
		if oversizedLossHotspots[i].AvgRiskSizingScore == oversizedLossHotspots[j].AvgRiskSizingScore {
			return oversizedLossHotspots[i].Deals > oversizedLossHotspots[j].Deals
		}
		return oversizedLossHotspots[i].AvgRiskSizingScore < oversizedLossHotspots[j].AvgRiskSizingScore
	})

	closeReasonQualityItems := make([]DealReviewAnomalyCloseReasonQuality, 0, len(closeReasonQuality))
	for reason, agg := range closeReasonQuality {
		item := DealReviewAnomalyCloseReasonQuality{
			Reason: reason,
			Deals:  agg.Deals,
			NetPnL: agg.NetPnL,
		}
		if agg.Deals > 0 {
			item.AvgExitEfficiencyScore = agg.ExitEfficiencyScoreSum / float64(agg.Deals)
			item.AvgMFECapturedPct = agg.MFECapturedPctSum / float64(agg.Deals)
			item.AvgGiveBackPct = agg.GiveBackPctSum / float64(agg.Deals)
		}
		closeReasonQualityItems = append(closeReasonQualityItems, item)
	}
	sort.Slice(closeReasonQualityItems, func(i, j int) bool {
		if closeReasonQualityItems[i].AvgExitEfficiencyScore == closeReasonQualityItems[j].AvgExitEfficiencyScore {
			return closeReasonQualityItems[i].Deals > closeReasonQualityItems[j].Deals
		}
		return closeReasonQualityItems[i].AvgExitEfficiencyScore < closeReasonQualityItems[j].AvgExitEfficiencyScore
	})

	exitUncertaintyItems := make([]DealReviewAnomalyExitUncertainty, 0, len(exitUncertainty))
	for key, agg := range exitUncertainty {
		parts := strings.SplitN(key, "|", 3)
		reason := "unspecified"
		quality := "unspecified"
		origin := "unspecified"
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			reason = strings.TrimSpace(parts[0])
		}
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			quality = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
			origin = strings.TrimSpace(parts[2])
		}
		item := DealReviewAnomalyExitUncertainty{
			Label:             fmt.Sprintf("%s | %s", reason, quality),
			CloseReason:       reason,
			ExitReasonQuality: quality,
			ExitOrigin:        origin,
			Deals:             agg.Deals,
			NetPnL:            agg.NetPnL,
		}
		if agg.Deals > 0 {
			item.AvgPnL = agg.NetPnL / float64(agg.Deals)
		}
		if len(cases) > 0 {
			item.SharePct = float64(agg.Deals) / float64(len(cases)) * 100
		}
		exitUncertaintyItems = append(exitUncertaintyItems, item)
	}
	sort.Slice(exitUncertaintyItems, func(i, j int) bool {
		if exitUncertaintyItems[i].Deals == exitUncertaintyItems[j].Deals {
			return exitUncertaintyItems[i].NetPnL < exitUncertaintyItems[j].NetPnL
		}
		return exitUncertaintyItems[i].Deals > exitUncertaintyItems[j].Deals
	})

	symbolEdgeFailures := make([]DealReviewAnomalySymbolEdgeFailure, 0)
	if traderID := strings.TrimSpace(filter.TraderID); traderID != "" {
		_, _ = s.RefreshSymbolBehaviorPriorsIfStale(userID, traderID)
		priorFilter := DealReviewSymbolBehaviorPriorFilter{
			Symbol: strings.TrimSpace(strings.ToUpper(filter.Symbol)),
			Side:   normalizeDealReviewSide(filter.Side),
			Limit:  500,
		}
		if priors, err := s.ListSymbolBehaviorPriors(userID, traderID, priorFilter); err == nil {
			for _, prior := range priors {
				if prior.BehaviorBias != DealReviewSymbolBehaviorBiasNegative {
					continue
				}
				if prior.Status != DealReviewSymbolBehaviorPriorStatusCandidate && prior.Status != DealReviewSymbolBehaviorPriorStatusValidated {
					continue
				}
				if prior.ContradictionScore < 0.35 && prior.LossRate < 0.55 && prior.AvgPnLPct > -0.50 {
					continue
				}
				match := symbolPriorSliceMatches[dealReviewSymbolBehaviorPriorKeyForPrior(&prior)]
				if match == nil || match.Deals == 0 {
					continue
				}
				symbolEdgeFailures = append(symbolEdgeFailures, DealReviewAnomalySymbolEdgeFailure{
					Symbol:                prior.Symbol,
					Side:                  prior.Side,
					RegimeSignature:       prior.RegimeSignature,
					OpenSelectionBucket:   prior.OpenSelectionBucket,
					OpenTrendRegime:       prior.OpenTrendRegime,
					OpenVolatilityRegime:  prior.OpenVolatilityRegime,
					OpenOIRegime:          prior.OpenOIRegime,
					SliceDeals:            match.Deals,
					SliceNetPnL:           match.NetPnL,
					HistoricalDeals:       prior.SampleCount,
					DecisionOpenCount:     prior.DecisionOpenCount,
					AvgDecisionConfidence: prior.AvgDecisionConfidence,
					AvgPnLPct:             prior.AvgPnLPct,
					ContradictionScore:    prior.ContradictionScore,
					RecommendedAction:     prior.RecommendedAction,
					SignalTags:            append([]string(nil), prior.SignalTags...),
					Summary:               prior.Summary,
				})
			}
		}
	}
	sort.Slice(symbolEdgeFailures, func(i, j int) bool {
		if symbolEdgeFailures[i].SliceDeals == symbolEdgeFailures[j].SliceDeals {
			if symbolEdgeFailures[i].ContradictionScore == symbolEdgeFailures[j].ContradictionScore {
				if symbolEdgeFailures[i].AvgPnLPct == symbolEdgeFailures[j].AvgPnLPct {
					return symbolEdgeFailures[i].HistoricalDeals > symbolEdgeFailures[j].HistoricalDeals
				}
				return symbolEdgeFailures[i].AvgPnLPct < symbolEdgeFailures[j].AvgPnLPct
			}
			return symbolEdgeFailures[i].ContradictionScore > symbolEdgeFailures[j].ContradictionScore
		}
		return symbolEdgeFailures[i].SliceDeals > symbolEdgeFailures[j].SliceDeals
	})

	summary.WorstSymbols = truncateAnomalySymbols(worstSymbols, 5)
	summary.OvertradedSymbols = truncateAnomalySymbols(overtraded, 5)
	summary.WeakBuckets = truncateAnomalyBuckets(weakBuckets, 5)
	summary.WeakCloseReasons = truncateAnomalyReasons(weakReasons, 5)
	summary.ProfitGiveBackHotspots = truncateAnomalyGiveBacks(giveBackHotspots, 5)
	summary.EarlyStopOutHotspots = truncateAnomalyStopOuts(stopOutHotspots, 5)
	summary.OversizedLossHotspots = truncateAnomalySizings(oversizedLossHotspots, 5)
	summary.CloseReasonQuality = truncateAnomalyCloseReasonQualities(closeReasonQualityItems, 5)
	summary.ExitUncertainty = truncateAnomalyExitUncertainties(exitUncertaintyItems, 5)
	summary.SymbolEdgeFailures = truncateAnomalySymbolEdgeFailures(symbolEdgeFailures, 5)
	summary.Notes = buildAnomalyNotes(summary)
	return summary, nil
}

func (s *DealReviewStore) buildCaseFilterQuery(userID string, filter DealReviewListFilter) *gorm.DB {
	query := s.db.Where("user_id = ?", userID)
	if strings.TrimSpace(filter.TraderID) != "" {
		query = query.Where("trader_id = ?", filter.TraderID)
	}
	if symbol := strings.TrimSpace(strings.ToUpper(filter.Symbol)); symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	if side := normalizeDealReviewSide(filter.Side); side != "" {
		query = query.Where("side = ?", side)
	}
	if status := strings.TrimSpace(strings.ToUpper(filter.Status)); status != "" {
		query = query.Where("status = ?", status)
	}
	if outcome := strings.TrimSpace(strings.ToLower(filter.Outcome)); outcome != "" {
		query = query.Where("outcome = ?", outcome)
	}
	if bucket := strings.TrimSpace(filter.OpenSelectionBucket); bucket != "" {
		query = query.Where("open_selection_bucket = ?", bucket)
	}
	if closeReason := strings.TrimSpace(filter.CloseReason); closeReason != "" {
		query = query.Where("close_reason = ?", closeReason)
	}
	if exitReasonQuality := strings.TrimSpace(filter.ExitReasonQuality); exitReasonQuality != "" {
		query = query.Where("exit_reason_quality = ?", exitReasonQuality)
	}
	stringFilters := map[string]string{
		"open_trend_regime":         filter.OpenTrendRegime,
		"open_volatility_regime":    filter.OpenVolatilityRegime,
		"open_btc_strength_regime":  filter.OpenBTCStrengthRegime,
		"open_funding_regime":       filter.OpenFundingRegime,
		"open_oi_regime":            filter.OpenOIRegime,
		"open_session_bucket":       filter.OpenSessionBucket,
		"open_weekday_bucket":       filter.OpenWeekdayBucket,
		"open_venue_tier":           filter.OpenVenueTier,
		"open_liquidity_tier":       filter.OpenLiquidityTier,
		"open_spread_bucket":        filter.OpenSpreadBucket,
		"open_slippage_bucket":      filter.OpenSlippageBucket,
		"close_trend_regime":        filter.CloseTrendRegime,
		"close_volatility_regime":   filter.CloseVolatilityRegime,
		"close_btc_strength_regime": filter.CloseBTCStrengthRegime,
		"close_funding_regime":      filter.CloseFundingRegime,
		"close_oi_regime":           filter.CloseOIRegime,
		"close_session_bucket":      filter.CloseSessionBucket,
		"close_weekday_bucket":      filter.CloseWeekdayBucket,
		"close_venue_tier":          filter.CloseVenueTier,
		"close_liquidity_tier":      filter.CloseLiquidityTier,
		"close_spread_bucket":       filter.CloseSpreadBucket,
		"close_slippage_bucket":     filter.CloseSlippageBucket,
	}
	for column, value := range stringFilters {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			query = query.Where(column+" = ?", trimmed)
		}
	}
	if filter.FromTime > 0 {
		query = query.Where("(entry_time_ms >= ? OR exit_time_ms >= ?)", filter.FromTime, filter.FromTime)
	}
	if filter.ToTime > 0 {
		query = query.Where("(entry_time_ms <= ? OR exit_time_ms <= ?)", filter.ToTime, filter.ToTime)
	}
	if filter.MinPnL != nil {
		query = query.Where("realized_pnl >= ?", *filter.MinPnL)
	}
	if filter.MaxPnL != nil {
		query = query.Where("realized_pnl <= ?", *filter.MaxPnL)
	}
	return query.Model(&DealReviewCase{})
}

func (s *DealReviewStore) enrichCaseList(cases []DealReviewCase) ([]DealReviewCaseListItem, error) {
	traderNames := map[string]string{}
	strategyNames := map[string]string{}
	eventIDs := make([]string, 0, len(cases)*2)
	for _, c := range cases {
		if c.TraderID != "" {
			traderNames[c.TraderID] = ""
		}
		if c.StrategyID != "" {
			strategyNames[c.StrategyID] = ""
		}
		if c.OpenEventID != "" {
			eventIDs = append(eventIDs, c.OpenEventID)
		}
		if c.CloseEventID != "" {
			eventIDs = append(eventIDs, c.CloseEventID)
		}
	}

	if len(traderNames) > 0 {
		ids := make([]string, 0, len(traderNames))
		for id := range traderNames {
			ids = append(ids, id)
		}
		var traders []Trader
		if err := s.db.Where("id IN ?", ids).Find(&traders).Error; err == nil {
			for _, trader := range traders {
				traderNames[trader.ID] = trader.Name
			}
		}
	}
	if len(strategyNames) > 0 {
		ids := make([]string, 0, len(strategyNames))
		for id := range strategyNames {
			ids = append(ids, id)
		}
		var strategies []Strategy
		if err := s.db.Where("id IN ?", ids).Find(&strategies).Error; err == nil {
			for _, strategy := range strategies {
				strategyNames[strategy.ID] = strategy.Name
			}
		}
	}

	eventMap := map[string]DealReviewEvent{}
	if len(eventIDs) > 0 {
		var events []DealReviewEvent
		if err := s.db.Where("id IN ?", eventIDs).Find(&events).Error; err != nil {
			return nil, err
		}
		for _, event := range events {
			eventMap[event.ID] = event
		}
	}
	timelineSummaries, err := s.buildCaseListPriceTimelineSummaries(cases)
	if err != nil {
		return nil, err
	}

	result := make([]DealReviewCaseListItem, 0, len(cases))
	for _, c := range cases {
		item := DealReviewCaseListItem{
			Case:                 c,
			TraderName:           traderNames[c.TraderID],
			StrategyName:         strategyNames[c.StrategyID],
			Labels:               parseJSONStringSlice(c.LabelsJSON),
			OpenCandidateSources: parseJSONStringSlice(c.OpenCandidateSourcesJSON),
			PriceTimelineSummary: timelineSummaries[c.PositionID],
		}
		hydrateDealReviewCaseJSONFields(&item.Case)
		if event, ok := eventMap[c.OpenEventID]; ok {
			item.OpenReasoning = event.Reasoning
		}
		if event, ok := eventMap[c.CloseEventID]; ok {
			item.CloseReasoning = event.Reasoning
		}
		result = append(result, item)
	}

	if len(cases) > 0 {
		model, err := s.buildHeuristicClassifierModel(cases[0].UserID, cases[0].TraderID)
		if err != nil {
			return nil, err
		} else {
			for i := range result {
				result[i].ClassifierAssist = model.EvaluateCase(&result[i].Case)
			}
		}
	}
	return result, nil
}

func (s *DealReviewStore) buildCaseListPriceTimelineSummaries(cases []DealReviewCase) (map[int64]*DealReviewPriceTimelineSummary, error) {
	if len(cases) == 0 {
		return map[int64]*DealReviewPriceTimelineSummary{}, nil
	}
	positionIDs := make([]int64, 0, len(cases))
	casesByPosition := make(map[int64]DealReviewCase, len(cases))
	for _, caseRec := range cases {
		if caseRec.PositionID <= 0 {
			continue
		}
		positionIDs = append(positionIDs, caseRec.PositionID)
		casesByPosition[caseRec.PositionID] = caseRec
	}
	if len(positionIDs) == 0 {
		return map[int64]*DealReviewPriceTimelineSummary{}, nil
	}

	pointsByPosition, err := s.loadTimelinePointsByPositionIDs(positionIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*DealReviewPriceTimelineSummary, len(positionIDs))
	for positionID, caseRec := range casesByPosition {
		points := buildDealReviewTimelinePoints(&caseRec, pointsByPosition[positionID])
		if len(points) == 0 {
			continue
		}
		summary := computeDealReviewPriceTimelineSummary(points)
		result[positionID] = &summary
	}
	return result, nil
}

func (s *DealReviewStore) computeDatasetSummary(userID string, filter DealReviewListFilter) (*DealReviewDatasetSummary, error) {
	query := s.buildCaseFilterQuery(userID, filter)
	var cases []DealReviewCase
	if err := query.Find(&cases).Error; err != nil {
		return nil, err
	}

	summary := &DealReviewDatasetSummary{}
	var grossProfit, grossLoss float64
	for _, c := range cases {
		summary.TotalDeals++
		if strings.EqualFold(c.Status, DealReviewCaseStatusOpen) {
			summary.OpenDeals++
		} else {
			summary.ClosedDeals++
		}
		switch strings.ToLower(c.Outcome) {
		case "profit":
			summary.WinningDeals++
			grossProfit += c.RealizedPnL
		case "loss":
			summary.LosingDeals++
			grossLoss += math.Abs(c.RealizedPnL)
		case "flat":
			summary.FlatDeals++
		}
		if strings.EqualFold(c.Side, "LONG") {
			summary.LongDeals++
			summary.LongNetPnL += c.RealizedPnL
		} else if strings.EqualFold(c.Side, "SHORT") {
			summary.ShortDeals++
			summary.ShortNetPnL += c.RealizedPnL
		}
		summary.NetPnL += c.RealizedPnL
		summary.AvgPnL += c.RealizedPnL
		summary.AvgPnLPct += c.RealizedPnLPct
		summary.AvgHoldMs += c.HoldDurationMs
		if strings.EqualFold(c.Status, DealReviewCaseStatusClosed) {
			summary.AvgMFECapturedPct += c.MFECapturedPct
			summary.AvgProfitGivenBackPct += c.ProfitGivenBackPct
			summary.AvgExitEfficiencyScore += c.ExitEfficiencyScore
			summary.AvgEntryTimingScore += c.EntryTimingScore
			summary.AvgRiskSizingScore += c.RiskSizingScore
			if isDealReviewBadEntry(c) {
				summary.BadEntryDeals++
			}
			if isDealReviewBadExit(c) {
				summary.BadExitDeals++
			}
			if isDealReviewAvoidableLoss(c) {
				summary.AvoidableLossDeals++
			}
			if isDealReviewStrongEntryWeakExit(c) {
				summary.StrongEntryWeakExitDeals++
			}
			if isDealReviewWeakEntryLuckyExit(c) {
				summary.WeakEntryLuckyExitDeals++
			}
		}
	}
	if summary.TotalDeals > 0 {
		summary.AvgPnL /= float64(summary.TotalDeals)
		summary.AvgPnLPct /= float64(summary.TotalDeals)
		summary.AvgHoldMs /= summary.TotalDeals
	}
	if summary.ClosedDeals > 0 {
		summary.WinRate = float64(summary.WinningDeals) / float64(summary.ClosedDeals) * 100
		summary.AvgMFECapturedPct /= float64(summary.ClosedDeals)
		summary.AvgProfitGivenBackPct /= float64(summary.ClosedDeals)
		summary.AvgExitEfficiencyScore /= float64(summary.ClosedDeals)
		summary.AvgEntryTimingScore /= float64(summary.ClosedDeals)
		summary.AvgRiskSizingScore /= float64(summary.ClosedDeals)
	}
	if grossLoss > 0 {
		summary.ProfitFactor = grossProfit / grossLoss
	}
	return summary, nil
}

func (s *DealReviewStore) ensureCaseForPositionTx(tx *gorm.DB, position *TraderPosition) (*DealReviewCase, error) {
	var caseRec DealReviewCase
	err := s.loadCaseForPositionTx(tx, position.ID, &caseRec)
	if err == nil {
		s.syncCaseFromPosition(&caseRec, position)
		if err := tx.Save(&caseRec).Error; err != nil {
			return nil, err
		}
		return &caseRec, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	var trader Trader
	if err := tx.Where("id = ?", position.TraderID).First(&trader).Error; err != nil {
		return nil, err
	}

	caseRec = DealReviewCase{
		ID:           uuid.NewString(),
		UserID:       trader.UserID,
		TraderID:     position.TraderID,
		PositionID:   position.ID,
		ExchangeID:   position.ExchangeID,
		ExchangeType: position.ExchangeType,
		AIModelID:    trader.AIModelID,
		StrategyID:   trader.StrategyID,
		Symbol:       strings.TrimSpace(position.Symbol),
		Side:         normalizeDealReviewSide(position.Side),
		Status:       DealReviewCaseStatusOpen,
		Outcome:      "open",
	}
	s.syncCaseFromPosition(&caseRec, position)
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "position_id"}},
		DoNothing: true,
	}).Create(&caseRec)
	if result.Error != nil {
		return nil, result.Error
	}

	persisted := caseRec
	if err := s.loadCaseForPositionTx(tx, position.ID, &persisted); err != nil {
		return nil, err
	}
	if persisted.ID != caseRec.ID {
		s.syncCaseFromPosition(&persisted, position)
		if err := tx.Save(&persisted).Error; err != nil {
			return nil, err
		}
		return &persisted, nil
	}
	if result.RowsAffected == 0 {
		s.syncCaseFromPosition(&caseRec, position)
		if err := tx.Save(&caseRec).Error; err != nil {
			return nil, err
		}
	}
	return &persisted, nil
}

func (s *DealReviewStore) loadCaseForPositionTx(tx *gorm.DB, positionID int64, dest *DealReviewCase) error {
	if tx == nil || dest == nil || positionID == 0 {
		return gorm.ErrRecordNotFound
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = tx.Where("position_id = ?", positionID).First(dest).Error
		if lastErr == nil || lastErr != gorm.ErrRecordNotFound {
			return lastErr
		}
		time.Sleep(5 * time.Millisecond)
	}
	return lastErr
}

func (s *DealReviewStore) syncCaseFromPosition(caseRec *DealReviewCase, position *TraderPosition) {
	caseRec.ExchangeID = position.ExchangeID
	caseRec.ExchangeType = position.ExchangeType
	caseRec.Symbol = strings.TrimSpace(position.Symbol)
	caseRec.Side = normalizeDealReviewSide(position.Side)
	caseRec.EntryOrderID = strings.TrimSpace(position.EntryOrderID)
	caseRec.ExitOrderID = strings.TrimSpace(position.ExitOrderID)
	caseRec.EntryTimeMs = position.EntryTime
	caseRec.ExitTimeMs = position.ExitTime
	caseRec.EntryPrice = position.EntryPrice
	caseRec.ExitPrice = position.ExitPrice
	caseRec.EntryQuantity = position.EntryQuantity
	if caseRec.EntryQuantity == 0 {
		caseRec.EntryQuantity = position.Quantity
	}
	if position.ExitTime > 0 {
		caseRec.ExitQuantity = position.EntryQuantity
		if caseRec.ExitQuantity == 0 {
			caseRec.ExitQuantity = position.Quantity
		}
	}
	caseRec.Leverage = position.Leverage
	caseRec.RealizedPnL = position.RealizedPnL
	caseRec.Fee = position.Fee
	positionCloseReason := normalizeCloseReason(position.CloseReason)
	if shouldReplaceCloseReason(caseRec.CloseReason, positionCloseReason) {
		caseRec.CloseReason = positionCloseReason
	}
	if position.ExitTime > position.EntryTime && position.EntryTime > 0 {
		caseRec.HoldDurationMs = position.ExitTime - position.EntryTime
	}

	if strings.EqualFold(position.Status, DealReviewCaseStatusClosed) {
		caseRec.Status = DealReviewCaseStatusClosed
		caseRec.Outcome = classifyDealOutcome(position.RealizedPnL)
		caseRec.RealizedPnLPct = calculateDealPnLPct(position.EntryPrice, caseRec.EntryQuantity, position.Leverage, position.RealizedPnL)
	} else {
		caseRec.Status = DealReviewCaseStatusOpen
		caseRec.Outcome = "open"
	}
}

func (s *DealReviewStore) linkEventForPositionTx(tx *gorm.DB, stage, orderID string, caseRec *DealReviewCase, position *TraderPosition) error {
	event, err := s.findStageEventForCaseTx(tx, caseRec, stage)
	if err != nil {
		return err
	}
	if event == nil {
		if strings.TrimSpace(orderID) != "" {
			event, err = s.findLatestEventTx(tx, position.TraderID, orderID, stage)
			if err != nil {
				return err
			}
		}
	}
	if event == nil {
		event, err = s.findLinkedEventByPositionTx(tx, position.ID, stage)
		if err != nil {
			return err
		}
	}
	if event == nil {
		event, err = s.createSyntheticEventTx(tx, stage, caseRec, position)
		if err != nil {
			return err
		}
	}

	event.DealID = caseRec.ID
	event.PositionID = position.ID
	event.Status = DealReviewEventStatusLinked
	event.ExchangeID = position.ExchangeID
	if event.ExchangeOrderID == "" {
		if stage == DealReviewStageOpen {
			event.ExchangeOrderID = strings.TrimSpace(position.EntryOrderID)
		} else {
			event.ExchangeOrderID = strings.TrimSpace(position.ExitOrderID)
		}
	}
	if stage == DealReviewStageOpen {
		event.Price = position.EntryPrice
		event.Quantity = caseRec.EntryQuantity
		event.Leverage = position.Leverage
	} else {
		event.Price = position.ExitPrice
		event.Quantity = caseRec.ExitQuantity
		event.Leverage = position.Leverage
		event.OutcomePnL = caseRec.RealizedPnL
		event.OutcomePnLPct = caseRec.RealizedPnLPct
		closeInference, err := s.inferCloseReasonTx(tx, caseRec, position, event)
		if err != nil {
			return err
		}
		if closeInference.Reason != "" {
			caseRec.CloseReason = closeInference.Reason
			event.CloseReason = closeInference.Reason
			if shouldReplaceCloseReason(position.CloseReason, closeInference.Reason) {
				position.CloseReason = closeInference.Reason
				if err := tx.Model(&TraderPosition{}).
					Where("id = ?", position.ID).
					Update("close_reason", closeInference.Reason).Error; err != nil {
					return err
				}
			}
		}
		applyDealReviewCloseInferenceToCase(caseRec, closeInference)
		applyDealReviewCloseInferenceToEvent(event, closeInference)
		if shouldReplaceSyntheticCloseReasoning(event.Reasoning) {
			if reasoning := buildInferredCloseReasoning(closeInference); reasoning != "" {
				event.Reasoning = reasoning
			}
		}
	}
	s.enrichEventSnapshotFromDecisionRecordTx(tx, event)
	if err := tx.Save(event).Error; err != nil {
		return err
	}
	eventSnapshot := parseDealReviewEventSnapshot(event.SnapshotJSON)
	_ = applyDealReviewMarketContextToCase(caseRec, stage, eventSnapshot.MarketContext)

	if stage == DealReviewStageOpen {
		caseRec.OpenEventID = event.ID
		caseRec.OpenCycleNumber = event.DecisionCycleNumber
		caseRec.OpenStopLoss = event.StopLoss
		caseRec.OpenTakeProfit = event.TakeProfit
		caseRec.OpenConfidence = event.Confidence
		caseRec.OpenSelectionBucket = event.SelectionBucket
		caseRec.OpenCandidateSourcesJSON = event.CandidateSourcesJSON
	} else {
		caseRec.CloseEventID = event.ID
		caseRec.CloseCycleNumber = event.DecisionCycleNumber
		caseRec.CloseConfidence = event.Confidence
	}
	return tx.Save(caseRec).Error
}

func (s *DealReviewStore) inferCloseReasonTx(tx *gorm.DB, caseRec *DealReviewCase, position *TraderPosition, event *DealReviewEvent) (dealReviewCloseInferenceResult, error) {
	currentReason := normalizeCloseReason(caseRec.CloseReason)
	decisionReason := inferCloseReasonFromDecision(event)

	execContext, err := s.findExitExecutionContextTx(tx, position)
	if err != nil {
		return dealReviewCloseInferenceResult{}, err
	}
	intentReason, err := s.inferCloseReasonFromExitIntentTx(tx, caseRec, position, event, execContext)
	if err != nil {
		return dealReviewCloseInferenceResult{}, err
	}
	trailingReason, err := s.inferCloseReasonFromTrailingUpdateTx(tx, caseRec, position, execContext)
	if err != nil {
		return dealReviewCloseInferenceResult{}, err
	}
	orderReason := inferCloseReasonFromOrderContext(caseRec, execContext)
	targetReason := inferCloseReasonFromTargets(caseRec, position)

	if !closeReasonCanBeRefined(currentReason) {
		switch {
		case currentReason != "" && currentReason == decisionReason.Reason:
			return decisionReason, nil
		case currentReason != "" && currentReason == intentReason.Reason:
			return intentReason, nil
		case currentReason != "" && currentReason == trailingReason.Reason:
			return trailingReason, nil
		case currentReason != "" && currentReason == orderReason.Reason:
			return orderReason, nil
		case currentReason != "" && currentReason == targetReason.Reason:
			return targetReason, nil
		case currentReason != "":
			return buildStoredCloseInference(currentReason, event, execContext), nil
		default:
			return dealReviewCloseInferenceResult{}, nil
		}
	}

	if decisionReason.Reason != "" {
		return decisionReason, nil
	}
	if trailingReason.Reason != "" {
		return trailingReason, nil
	}
	if intentReason.Reason != "" && (orderReason.Reason == "" || normalizeCloseReason(orderReason.Reason) == "manual_exit") {
		return intentReason, nil
	}
	if orderReason.Reason != "" && normalizeCloseReason(orderReason.Reason) != "manual_exit" {
		return orderReason, nil
	}
	if targetReason.Reason != "" {
		return targetReason, nil
	}
	if intentReason.Reason != "" {
		return intentReason, nil
	}
	if orderReason.Reason != "" {
		return orderReason, nil
	}
	if currentReason != "" {
		return buildStoredCloseInference(currentReason, event, execContext), nil
	}
	if event != nil && event.Source == DealReviewEventSourceSync {
		return buildExchangeSyncUnknownInference(execContext, "sync_position_without_exit_evidence"), nil
	}

	return dealReviewCloseInferenceResult{}, nil
}

func (s *DealReviewStore) findStageEventForCaseTx(tx *gorm.DB, caseRec *DealReviewCase, stage string) (*DealReviewEvent, error) {
	if caseRec == nil {
		return nil, nil
	}

	var eventID string
	if stage == DealReviewStageOpen {
		eventID = strings.TrimSpace(caseRec.OpenEventID)
	} else {
		eventID = strings.TrimSpace(caseRec.CloseEventID)
	}
	if eventID == "" {
		return nil, nil
	}

	var event DealReviewEvent
	err := tx.Where("id = ?", eventID).First(&event).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *DealReviewStore) findLatestEventTx(tx *gorm.DB, traderID, orderID, stage string) (*DealReviewEvent, error) {
	var event DealReviewEvent
	err := tx.Where("trader_id = ? AND exchange_order_id = ? AND stage = ? AND status != ?",
		traderID, strings.TrimSpace(orderID), stage, DealReviewEventStatusCanceled).
		Order("created_at DESC").
		First(&event).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *DealReviewStore) findLinkedEventByPositionTx(tx *gorm.DB, positionID int64, stage string) (*DealReviewEvent, error) {
	if positionID == 0 {
		return nil, nil
	}

	var event DealReviewEvent
	err := tx.Where("position_id = ? AND stage = ? AND status != ?", positionID, stage, DealReviewEventStatusCanceled).
		Order("created_at DESC").
		First(&event).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *DealReviewStore) findExitExecutionContextTx(tx *gorm.DB, position *TraderPosition) (*dealReviewExitExecutionContext, error) {
	if position == nil {
		return nil, nil
	}

	exitOrderID := strings.TrimSpace(position.ExitOrderID)
	if exitOrderID == "" {
		return nil, nil
	}

	ctx := &dealReviewExitExecutionContext{}

	var directOrder TraderOrder
	err := tx.Where(
		"trader_id = ? AND exchange_id = ? AND symbol = ? AND exchange_order_id = ?",
		position.TraderID,
		position.ExchangeID,
		strings.TrimSpace(position.Symbol),
		exitOrderID,
	).Order("updated_at DESC").First(&directOrder).Error
	switch {
	case err == nil:
		ctx.Order = &directOrder
	case err != nil && err != gorm.ErrRecordNotFound:
		return nil, err
	}

	var fill TraderFill
	err = tx.Where(
		"trader_id = ? AND exchange_id = ? AND symbol = ? AND exchange_order_id = ?",
		position.TraderID,
		position.ExchangeID,
		strings.TrimSpace(position.Symbol),
		exitOrderID,
	).Order("created_at DESC").First(&fill).Error
	switch {
	case err == nil:
		ctx.Fill = &fill
		if ctx.Order == nil && fill.OrderID > 0 {
			var order TraderOrder
			if orderErr := tx.Where("id = ?", fill.OrderID).First(&order).Error; orderErr == nil {
				ctx.Order = &order
			} else if orderErr != gorm.ErrRecordNotFound {
				return nil, orderErr
			}
		}
	case err != nil && err != gorm.ErrRecordNotFound:
		return nil, err
	}

	if ctx.Order == nil {
		var order TraderOrder
		err = tx.Where(
			"trader_id = ? AND exchange_id = ? AND symbol = ? AND exchange_order_id = ?",
			position.TraderID,
			position.ExchangeID,
			strings.TrimSpace(position.Symbol),
			exitOrderID,
		).Order("created_at DESC").First(&order).Error
		switch {
		case err == nil:
			ctx.Order = &order
		case err != nil && err != gorm.ErrRecordNotFound:
			return nil, err
		}
	}

	if ctx.Order == nil && ctx.Fill == nil {
		return nil, nil
	}
	return ctx, nil
}

func (s *DealReviewStore) findLatestExitIntentTx(tx *gorm.DB, position *TraderPosition) (*DealReviewExitIntentRecord, string, error) {
	if position == nil {
		return nil, "", nil
	}

	symbol := strings.ToUpper(strings.TrimSpace(position.Symbol))
	side := normalizeDealReviewSide(position.Side)

	if exitOrderID := strings.TrimSpace(position.ExitOrderID); exitOrderID != "" {
		var record DealReviewExitIntentRecord
		err := tx.Model(&DealReviewExitIntentRecord{}).
			Where("trader_id = ? AND exchange_order_id = ? AND status != ?", position.TraderID, exitOrderID, DealReviewExitIntentStatusCanceled).
			Order("timestamp_ms DESC, created_at DESC").
			First(&record).Error
		switch {
		case err == nil:
			return &record, "exit_intent_order_match", nil
		case err != nil && err != gorm.ErrRecordNotFound:
			return nil, "", err
		}
	}

	windowStart := position.EntryTime
	if windowStart <= 0 {
		windowStart = time.Now().UTC().Add(-24 * time.Hour).UnixMilli()
	}
	windowEnd := position.ExitTime
	if windowEnd <= 0 {
		windowEnd = time.Now().UTC().UnixMilli()
	}

	query := tx.Model(&DealReviewExitIntentRecord{}).
		Where("trader_id = ? AND symbol = ? AND side = ? AND status != ? AND timestamp_ms >= ? AND timestamp_ms <= ?",
			position.TraderID,
			symbol,
			side,
			DealReviewExitIntentStatusCanceled,
			windowStart,
			windowEnd+int64(10*time.Minute/time.Millisecond),
		)
	if position.ID > 0 {
		query = query.Where("(position_id = ? OR position_id = 0)", position.ID)
	}

	var record DealReviewExitIntentRecord
	err := query.
		Order("timestamp_ms DESC, created_at DESC").
		First(&record).Error
	if err == gorm.ErrRecordNotFound {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	return &record, "exit_intent_time_match", nil
}

func (s *DealReviewStore) linkExitIntentTx(tx *gorm.DB, intent *DealReviewExitIntentRecord, caseRec *DealReviewCase, event *DealReviewEvent) error {
	if tx == nil || intent == nil {
		return nil
	}
	updates := map[string]any{
		"status":     DealReviewExitIntentStatusLinked,
		"updated_at": time.Now().UTC(),
	}
	if caseRec != nil {
		updates["linked_deal_id"] = strings.TrimSpace(caseRec.ID)
	}
	if event != nil {
		updates["linked_event_id"] = strings.TrimSpace(event.ID)
	}
	return tx.Model(&DealReviewExitIntentRecord{}).
		Where("id = ?", intent.ID).
		Updates(updates).Error
}

func (s *DealReviewStore) findLatestTrailingUpdateTx(tx *gorm.DB, position *TraderPosition) (*DealReviewTrailingUpdateRecord, error) {
	if position == nil {
		return nil, nil
	}

	query := tx.Model(&DealReviewTrailingUpdateRecord{}).
		Where("trader_id = ? AND symbol = ? AND side = ? AND timestamp_ms >= ?",
			position.TraderID,
			strings.ToUpper(strings.TrimSpace(position.Symbol)),
			normalizeDealReviewSide(position.Side),
			position.EntryTime,
		)
	if position.ExitTime > 0 {
		query = query.Where("timestamp_ms <= ?", position.ExitTime)
	}

	var record DealReviewTrailingUpdateRecord
	err := query.
		Order("timestamp_ms DESC, id DESC").
		First(&record).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *DealReviewStore) inferCloseReasonFromExitIntentTx(tx *gorm.DB, caseRec *DealReviewCase, position *TraderPosition, event *DealReviewEvent, ctx *dealReviewExitExecutionContext) (dealReviewCloseInferenceResult, error) {
	if position == nil || position.ExitTime <= 0 {
		return dealReviewCloseInferenceResult{}, nil
	}

	intent, matchedBy, err := s.findLatestExitIntentTx(tx, position)
	if err != nil || intent == nil {
		return dealReviewCloseInferenceResult{}, err
	}

	quality := DealReviewExitReasonQualityHighConfidence
	if matchedBy == "exit_intent_order_match" || intent.PositionID == position.ID {
		quality = DealReviewExitReasonQualityExplicit
	}

	reason := "manual_exit"
	origin := DealReviewExitOriginInternalExitIntent
	switch strings.TrimSpace(intent.IntentType) {
	case DealReviewExitIntentTypeAICloseDecision:
		reason = "ai_exit"
		origin = DealReviewExitOriginAIDecision
	case DealReviewExitIntentTypeGridDecision:
		reason = "ai_exit"
		origin = DealReviewExitOriginAIDecision
	case DealReviewExitIntentTypeManualUIClose:
		origin = DealReviewExitOriginManualUIClose
	case DealReviewExitIntentTypeDrawdownGuard, DealReviewExitIntentTypeGridEmergency, DealReviewExitIntentTypeGridBreakout:
		origin = DealReviewExitOriginRiskGuard
	case DealReviewExitIntentTypeGridLevelStop:
		reason = "stop_loss"
		origin = DealReviewExitOriginRiskGuard
	}

	evidence := buildDealReviewOrderExitEvidence(nil, nil, matchedBy)
	if ctx != nil {
		evidence = buildDealReviewOrderExitEvidence(ctx.Order, ctx.Fill, matchedBy)
	}
	evidence.IntentID = intent.ID
	evidence.IntentType = intent.IntentType
	evidence.IntentSourceModule = intent.SourceModule
	evidence.IntentSummary = intent.Summary
	evidence.IntentReasoning = intent.Reasoning
	evidence.IntentConfidence = intent.Confidence
	evidence.IntentCreatedAtMs = intent.TimestampMs
	if reason == "ai_exit" {
		evidence.DecisionCycleNumber = intent.DecisionCycleNumber
		evidence.DecisionAction = strings.TrimSpace(intent.Action)
	}
	if caseRec != nil {
		evidence.TargetStopLoss = caseRec.OpenStopLoss
		evidence.TargetTakeProfit = caseRec.OpenTakeProfit
	}

	durationText := "shortly"
	if position.ExitTime > intent.TimestampMs && intent.TimestampMs > 0 {
		durationText = formatDurationMs(position.ExitTime - intent.TimestampMs)
	}
	summary := strings.TrimSpace(intent.Summary)
	if summary == "" {
		summary = fmt.Sprintf("Matched %s intent from %s before exit.", strings.ReplaceAll(intent.IntentType, "_", " "), durationText)
	} else {
		summary = fmt.Sprintf("%s Issued %s before exit.", summary, durationText)
	}

	if err := s.linkExitIntentTx(tx, intent, caseRec, event); err != nil {
		return dealReviewCloseInferenceResult{}, err
	}

	return dealReviewCloseInferenceResult{
		Reason:              reason,
		InferredBy:          "matched persisted internal exit intent",
		ExitOrigin:          origin,
		ExitReasonQuality:   quality,
		ExitEvidenceSummary: summary,
		ExitEvidence:        evidence,
	}, nil
}

func (s *DealReviewStore) inferCloseReasonFromTrailingUpdateTx(tx *gorm.DB, caseRec *DealReviewCase, position *TraderPosition, ctx *dealReviewExitExecutionContext) (dealReviewCloseInferenceResult, error) {
	if position == nil || position.ExitTime <= 0 {
		return dealReviewCloseInferenceResult{}, nil
	}

	trailingUpdate, err := s.findLatestTrailingUpdateTx(tx, position)
	if err != nil || trailingUpdate == nil || trailingUpdate.NewStopPrice <= 0 {
		return dealReviewCloseInferenceResult{}, err
	}

	matchPrice := position.ExitPrice
	if ctx != nil && ctx.Order != nil && ctx.Order.StopPrice > 0 {
		matchPrice = ctx.Order.StopPrice
	} else if ctx != nil && ctx.Fill != nil && ctx.Fill.Price > 0 {
		matchPrice = ctx.Fill.Price
	}
	if matchPrice <= 0 {
		return dealReviewCloseInferenceResult{}, nil
	}

	if !nearlyEqualRelative(matchPrice, trailingUpdate.NewStopPrice, 0.004) {
		return dealReviewCloseInferenceResult{}, nil
	}

	quality := DealReviewExitReasonQualityHighConfidence
	if ctx != nil && ctx.Order != nil {
		normalizedType := normalizeReasoningHint(ctx.Order.Type)
		if strings.Contains(normalizedType, "stop") || ctx.Order.StopPrice > 0 {
			quality = DealReviewExitReasonQualityExplicit
		}
	}

	evidence := buildDealReviewOrderExitEvidence(nil, nil, "trailing_stop_update_match")
	if ctx != nil {
		evidence = buildDealReviewOrderExitEvidence(ctx.Order, ctx.Fill, "trailing_stop_update_match")
	}
	evidence.TrailingUpdateID = trailingUpdate.ID
	evidence.PreviousStopPrice = trailingUpdate.PreviousStopPrice
	evidence.NewStopPrice = trailingUpdate.NewStopPrice
	evidence.TrailingMode = trailingUpdate.TrailingMode
	evidence.TrailingTierIndex = trailingUpdate.TierIndex
	evidence.TrailingTriggerProfitPct = trailingUpdate.TierTriggerProfitPct
	evidence.TrailingLockProfitPct = trailingUpdate.LockProfitPct
	evidence.TrailingOffsetPct = trailingUpdate.TrailOffsetPct
	evidence.TrailingUpdatedAtMs = trailingUpdate.TimestampMs
	evidence.TrailingUnrealizedPnL = trailingUpdate.UnrealizedPnL
	evidence.TrailingUnrealizedPnLPct = trailingUpdate.UnrealizedPnLPct
	evidence.TrailingStopProfitPct = trailingUpdate.StopProfitPct
	evidence.TrailingProtectsBreakeven = trailingUpdate.ProtectsBreakeven
	evidence.TargetStopLoss = caseRec.OpenStopLoss
	evidence.TargetTakeProfit = caseRec.OpenTakeProfit
	evidence.StopLossDistanceBps = relativeDistanceBps(matchPrice, caseRec.OpenStopLoss)
	evidence.TakeProfitDistanceBps = relativeDistanceBps(matchPrice, caseRec.OpenTakeProfit)

	summary := fmt.Sprintf(
		"Matched close against trailing-stop update to %.8f set %s before exit.",
		trailingUpdate.NewStopPrice,
		formatDurationMs(position.ExitTime-trailingUpdate.TimestampMs),
	)
	if trailingUpdate.PreviousStopPrice > 0 {
		summary += fmt.Sprintf(" Previous stop was %.8f.", trailingUpdate.PreviousStopPrice)
	}
	reason := "trailing_stop"
	if !stopPriceCountsAsProfitProtection(caseRec, matchPrice) {
		reason = "stop_loss"
		switch {
		case stopPriceOnLossSide(caseRec, matchPrice):
			summary += " The tightened stop was still on the loss side of entry, so the exit is classified as stop loss."
		case caseRec != nil && caseRec.RealizedPnLPct < 0:
			summary += " Net realized PnL still finished negative, so the exit is classified as stop loss."
		}
	}

	return dealReviewCloseInferenceResult{
		Reason:              reason,
		InferredBy:          "matched persisted trailing-stop update",
		ExitOrigin:          DealReviewExitOriginTrailingEngine,
		ExitReasonQuality:   quality,
		ExitEvidenceSummary: summary,
		ExitEvidence:        evidence,
	}, nil
}

func (s *DealReviewStore) createSyntheticEventTx(tx *gorm.DB, stage string, caseRec *DealReviewCase, position *TraderPosition) (*DealReviewEvent, error) {
	event := &DealReviewEvent{
		ID:                   uuid.NewString(),
		UserID:               caseRec.UserID,
		TraderID:             caseRec.TraderID,
		DealID:               caseRec.ID,
		PositionID:           position.ID,
		Stage:                stage,
		Source:               DealReviewEventSourceSync,
		Status:               DealReviewEventStatusLinked,
		DecisionTimestamp:    time.Now().UTC(),
		ExchangeID:           position.ExchangeID,
		Symbol:               strings.TrimSpace(position.Symbol),
		Side:                 normalizeDealReviewSide(position.Side),
		Quantity:             position.Quantity,
		Price:                position.EntryPrice,
		Leverage:             position.Leverage,
		Reasoning:            "Position synced without a matching AI decision snapshot.",
		CandidateSourcesJSON: "[]",
		SnapshotJSON:         "{}",
	}
	if stage == DealReviewStageOpen {
		event.Action = strings.ToLower(fmt.Sprintf("open_%s", strings.ToLower(event.Side)))
		event.ExchangeOrderID = strings.TrimSpace(position.EntryOrderID)
		event.Price = position.EntryPrice
		event.Quantity = caseRec.EntryQuantity
	} else {
		event.Action = strings.ToLower(fmt.Sprintf("close_%s", strings.ToLower(event.Side)))
		event.ExchangeOrderID = strings.TrimSpace(position.ExitOrderID)
		event.Price = position.ExitPrice
		event.Quantity = caseRec.ExitQuantity
		event.OutcomePnL = caseRec.RealizedPnL
		event.OutcomePnLPct = caseRec.RealizedPnLPct
		event.CloseReason = caseRec.CloseReason
		reason := caseRec.CloseReason
		if reason == "" {
			reason = "sync"
		}
		event.Reasoning = fmt.Sprintf("Position closed via synced exchange event (%s).", reason)
	}
	s.enrichSyntheticEventFromDecisionTx(tx, event, position, stage)
	if err := tx.Create(event).Error; err != nil {
		return nil, err
	}
	return event, nil
}

func (s *DealReviewStore) enrichSyntheticEventFromDecisionTx(tx *gorm.DB, event *DealReviewEvent, position *TraderPosition, stage string) {
	match, err := s.findMatchedDecisionForPositionTx(tx, position, stage)
	if err != nil || match == nil || match.Record == nil || match.Action == nil {
		return
	}

	event.DecisionCycleNumber = match.Record.CycleNumber
	event.DecisionTimestamp = match.Record.Timestamp.UTC()
	event.Action = strings.TrimSpace(match.Action.Action)
	if match.Action.Quantity > 0 {
		event.Quantity = match.Action.Quantity
	}
	if match.Action.Price > 0 {
		event.Price = match.Action.Price
	}
	if match.Action.Leverage > 0 {
		event.Leverage = match.Action.Leverage
	}
	if match.Action.StopLoss > 0 {
		event.StopLoss = match.Action.StopLoss
	}
	if match.Action.TakeProfit > 0 {
		event.TakeProfit = match.Action.TakeProfit
	}
	if match.Action.Confidence > 0 {
		event.Confidence = match.Action.Confidence
	}
	if reasoning := strings.TrimSpace(match.Action.Reasoning); reasoning != "" {
		event.Reasoning = reasoning
	}
	if bucket := strings.TrimSpace(match.SelectionBucket); bucket != "" {
		event.SelectionBucket = bucket
	}
	if len(match.CandidateSources) > 0 {
		if raw, err := json.Marshal(match.CandidateSources); err == nil {
			event.CandidateSourcesJSON = string(raw)
		}
	}

	snapshot := DealReviewEventSnapshot{
		CandidateCoins:   append([]string(nil), match.Record.CandidateCoins...),
		CandidateDetails: append([]CandidateDetail(nil), match.Record.CandidateDetails...),
		ExecutionLog:     append([]string(nil), match.Record.ExecutionLog...),
	}
	if raw, err := json.Marshal(snapshot); err == nil && string(raw) != "null" {
		event.SnapshotJSON = string(raw)
	}
}

func (s *DealReviewStore) findMatchedDecisionForPositionTx(tx *gorm.DB, position *TraderPosition, stage string) (*matchedDealReviewDecision, error) {
	targetTimeMs := position.EntryTime
	if stage == DealReviewStageClose {
		targetTimeMs = position.ExitTime
	}
	if targetTimeMs <= 0 {
		return nil, nil
	}

	windowStart := time.UnixMilli(targetTimeMs).Add(-6 * time.Hour).UTC()
	windowEnd := time.UnixMilli(targetTimeMs).Add(15 * time.Minute).UTC()

	var dbRecords []DecisionRecordDB
	if err := tx.Where("trader_id = ? AND timestamp >= ? AND timestamp <= ?", position.TraderID, windowStart, windowEnd).
		Order("timestamp DESC").
		Limit(150).
		Find(&dbRecords).Error; err != nil {
		return nil, err
	}

	expectedAction := expectedDealReviewAction(stage, position.Side)
	targetSymbol := strings.ToUpper(strings.TrimSpace(position.Symbol))
	targetTime := time.UnixMilli(targetTimeMs).UTC()

	var bestMatch *matchedDealReviewDecision
	bestDelta := int64(1<<63 - 1)

	for _, dbRecord := range dbRecords {
		record := dbRecord.toRecord()
		for _, action := range record.Decisions {
			if !decisionMatchesDealReview(action, targetSymbol, expectedAction) {
				continue
			}

			actionTime := record.Timestamp.UTC()
			if !action.Timestamp.IsZero() {
				actionTime = action.Timestamp.UTC()
			}
			delta := actionTime.Sub(targetTime).Milliseconds()
			if delta < 0 {
				delta = -delta
			}

			if delta >= bestDelta {
				continue
			}
			bestDelta = delta

			match := &matchedDealReviewDecision{
				Record: record,
				Action: &DecisionAction{
					Action:     action.Action,
					Symbol:     action.Symbol,
					Quantity:   action.Quantity,
					Leverage:   action.Leverage,
					Price:      action.Price,
					StopLoss:   action.StopLoss,
					TakeProfit: action.TakeProfit,
					Confidence: action.Confidence,
					Reasoning:  action.Reasoning,
					OrderID:    action.OrderID,
					Timestamp:  action.Timestamp,
					Success:    action.Success,
					Error:      action.Error,
				},
			}

			for _, detail := range record.CandidateDetails {
				if strings.EqualFold(strings.TrimSpace(detail.Symbol), targetSymbol) {
					match.CandidateSources = append([]string(nil), detail.Sources...)
					match.SelectionBucket = detail.SelectionBucket
					break
				}
			}
			bestMatch = match
		}
	}
	return bestMatch, nil
}

func (s *DealReviewStore) buildEventDetail(traderID, eventID string) (*DealReviewEventDetail, error) {
	var event DealReviewEvent
	if err := s.db.Where("id = ? AND trader_id = ?", eventID, traderID).First(&event).Error; err != nil {
		return nil, err
	}
	hydrateDealReviewEventJSONFields(&event)
	detail := &DealReviewEventDetail{
		Event:            &event,
		CandidateSources: parseJSONStringSlice(event.CandidateSourcesJSON),
	}
	var snapshot DealReviewEventSnapshot
	hasSnapshot := false
	if strings.TrimSpace(event.SnapshotJSON) != "" {
		if err := json.Unmarshal([]byte(event.SnapshotJSON), &snapshot); err == nil {
			hasSnapshot = true
		}
	}
	if event.DecisionCycleNumber > 0 {
		decision, err := NewDecisionStore(s.db).GetByCycleNumber(traderID, event.DecisionCycleNumber)
		if err == nil {
			detail.DecisionRecord = decision
			if mergeDealReviewSnapshotDecisionArtifacts(&snapshot, decision) {
				hasSnapshot = true
			}
		}
	}
	if hasSnapshot {
		detail.Snapshot = &snapshot
	}
	return detail, nil
}

func buildDealReviewTimelinePointFromCycleRecord(record DealReviewCyclePointRecord) DealReviewPriceTimelinePoint {
	return DealReviewPriceTimelinePoint{
		Source:              "cycle",
		TimestampMs:         record.TimestampMs,
		DecisionCycleNumber: record.DecisionCycleNumber,
		MarkPrice:           record.MarkPrice,
		EntryPrice:          record.EntryPrice,
		Quantity:            record.Quantity,
		UnrealizedPnL:       record.UnrealizedPnL,
		UnrealizedPnLPct:    record.UnrealizedPnLPct,
		InProfit:            record.InProfit,
	}
}

func buildDealReviewTimelinePointFromMarketRecord(record DealReviewMarketPointRecord) DealReviewPriceTimelinePoint {
	return DealReviewPriceTimelinePoint{
		Source:           normalizeDealReviewLivePointSource(record.Source),
		TimestampMs:      record.TimestampMs,
		MarkPrice:        record.MarkPrice,
		EntryPrice:       record.EntryPrice,
		Quantity:         record.Quantity,
		UnrealizedPnL:    record.UnrealizedPnL,
		UnrealizedPnLPct: record.UnrealizedPnLPct,
		InProfit:         record.InProfit,
	}
}

func (s *DealReviewStore) loadTimelinePointsByPositionIDs(positionIDs []int64) (map[int64][]DealReviewPriceTimelinePoint, error) {
	return s.loadTimelinePointsByPositionIDsWithDB(s.db, positionIDs)
}

func (s *DealReviewStore) loadTimelinePointsByPositionIDsWithDB(db *gorm.DB, positionIDs []int64) (map[int64][]DealReviewPriceTimelinePoint, error) {
	if len(positionIDs) == 0 {
		return map[int64][]DealReviewPriceTimelinePoint{}, nil
	}

	pointsByPosition := make(map[int64][]DealReviewPriceTimelinePoint, len(positionIDs))

	var cycleRecords []DealReviewCyclePointRecord
	if err := db.
		Where("position_id IN ?", positionIDs).
		Order("timestamp_ms ASC, decision_cycle_number ASC").
		Find(&cycleRecords).Error; err != nil {
		return nil, err
	} else {
		for _, record := range cycleRecords {
			pointsByPosition[record.PositionID] = append(pointsByPosition[record.PositionID], buildDealReviewTimelinePointFromCycleRecord(record))
		}
	}

	var marketRecords []DealReviewMarketPointRecord
	if err := db.
		Where("position_id IN ?", positionIDs).
		Order("timestamp_ms ASC, source ASC").
		Find(&marketRecords).Error; err != nil {
		return nil, err
	} else {
		for _, record := range marketRecords {
			pointsByPosition[record.PositionID] = append(pointsByPosition[record.PositionID], buildDealReviewTimelinePointFromMarketRecord(record))
		}
	}

	return pointsByPosition, nil
}

func buildDealReviewTimelinePoints(caseRec *DealReviewCase, timelinePoints []DealReviewPriceTimelinePoint) []DealReviewPriceTimelinePoint {
	if caseRec == nil {
		return nil
	}
	points := make([]DealReviewPriceTimelinePoint, 0, len(timelinePoints)+2)
	if caseRec.EntryTimeMs > 0 && caseRec.EntryPrice > 0 {
		points = append(points, DealReviewPriceTimelinePoint{
			Source:           "entry",
			TimestampMs:      caseRec.EntryTimeMs,
			MarkPrice:        caseRec.EntryPrice,
			EntryPrice:       caseRec.EntryPrice,
			Quantity:         caseRec.EntryQuantity,
			UnrealizedPnL:    0,
			UnrealizedPnLPct: 0,
			InProfit:         false,
		})
	}
	points = append(points, timelinePoints...)
	if caseRec.ExitTimeMs > 0 && caseRec.ExitPrice > 0 {
		points = append(points, DealReviewPriceTimelinePoint{
			Source:           "exit",
			TimestampMs:      caseRec.ExitTimeMs,
			MarkPrice:        caseRec.ExitPrice,
			EntryPrice:       caseRec.EntryPrice,
			Quantity:         caseRec.ExitQuantity,
			UnrealizedPnL:    caseRec.RealizedPnL,
			UnrealizedPnLPct: calcDealReviewSignedPriceMovePct(caseRec.Side, caseRec.EntryPrice, caseRec.ExitPrice),
			InProfit:         caseRec.RealizedPnL > 0,
		})
	}
	sort.SliceStable(points, func(i, j int) bool {
		if points[i].TimestampMs == points[j].TimestampMs {
			return dealReviewTimelineSourceRank(points[i].Source) < dealReviewTimelineSourceRank(points[j].Source)
		}
		return points[i].TimestampMs < points[j].TimestampMs
	})
	return points
}

func (s *DealReviewStore) buildCasePriceTimeline(caseRec *DealReviewCase) (*DealReviewPriceTimeline, error) {
	if caseRec == nil || caseRec.PositionID <= 0 {
		return nil, nil
	}

	pointsByPosition, err := s.loadTimelinePointsByPositionIDs([]int64{caseRec.PositionID})
	if err != nil {
		return nil, err
	}

	points := buildDealReviewTimelinePoints(caseRec, pointsByPosition[caseRec.PositionID])

	return &DealReviewPriceTimeline{
		Points:  points,
		Summary: computeDealReviewPriceTimelineSummary(points),
	}, nil
}

type dealReviewCaseQuality struct {
	MaxFavorableExcursion    float64
	MaxFavorableExcursionPct float64
	MaxAdverseExcursion      float64
	MaxAdverseExcursionPct   float64
	MFECapturedPct           float64
	ProfitGivenBack          float64
	ProfitGivenBackPct       float64
	TimeToFirstProfitMs      int64
	TimeToMaxDrawdownMs      int64
	PlannedRiskPct           float64
	ExitEfficiencyScore      float64
	EntryTimingScore         float64
	RiskSizingScore          float64
}

func (s *DealReviewStore) refreshCaseQualityMetricsTx(tx *gorm.DB, caseRec *DealReviewCase) error {
	if tx == nil || caseRec == nil || !strings.EqualFold(caseRec.Status, DealReviewCaseStatusClosed) {
		return nil
	}
	pointsByPosition, err := s.loadTimelinePointsByPositionIDsWithDB(tx, []int64{caseRec.PositionID})
	if err != nil {
		return err
	}
	quality := computeDealReviewCaseQuality(caseRec, buildDealReviewTimelinePoints(caseRec, pointsByPosition[caseRec.PositionID]))
	return applyDealReviewCaseQualityTx(tx, caseRec, quality)
}

func applyDealReviewCaseQualityTx(tx *gorm.DB, caseRec *DealReviewCase, quality dealReviewCaseQuality) error {
	if tx == nil || caseRec == nil {
		return nil
	}
	caseRec.MaxFavorableExcursion = quality.MaxFavorableExcursion
	caseRec.MaxFavorableExcursionPct = quality.MaxFavorableExcursionPct
	caseRec.MaxAdverseExcursion = quality.MaxAdverseExcursion
	caseRec.MaxAdverseExcursionPct = quality.MaxAdverseExcursionPct
	caseRec.MFECapturedPct = quality.MFECapturedPct
	caseRec.ProfitGivenBack = quality.ProfitGivenBack
	caseRec.ProfitGivenBackPct = quality.ProfitGivenBackPct
	caseRec.TimeToFirstProfitMs = quality.TimeToFirstProfitMs
	caseRec.TimeToMaxDrawdownMs = quality.TimeToMaxDrawdownMs
	caseRec.PlannedRiskPct = quality.PlannedRiskPct
	caseRec.ExitEfficiencyScore = quality.ExitEfficiencyScore
	caseRec.EntryTimingScore = quality.EntryTimingScore
	caseRec.RiskSizingScore = quality.RiskSizingScore
	return tx.Save(caseRec).Error
}

func computeDealReviewCaseQuality(caseRec *DealReviewCase, points []DealReviewPriceTimelinePoint) dealReviewCaseQuality {
	quality := dealReviewCaseQuality{}
	if caseRec == nil {
		return quality
	}

	if len(points) == 0 {
		return quality
	}

	entryTimeMs := caseRec.EntryTimeMs
	minPointTimestamp := entryTimeMs
	firstProfitFound := false

	for _, point := range points {
		if point.UnrealizedPnL > quality.MaxFavorableExcursion {
			quality.MaxFavorableExcursion = point.UnrealizedPnL
		}
		if point.UnrealizedPnLPct > quality.MaxFavorableExcursionPct {
			quality.MaxFavorableExcursionPct = point.UnrealizedPnLPct
		}
		if point.UnrealizedPnL < quality.MaxAdverseExcursion {
			quality.MaxAdverseExcursion = point.UnrealizedPnL
		}
		if point.UnrealizedPnLPct < quality.MaxAdverseExcursionPct {
			quality.MaxAdverseExcursionPct = point.UnrealizedPnLPct
		}
		if !firstProfitFound && point.UnrealizedPnL > 0 {
			firstProfitFound = true
			if entryTimeMs > 0 && point.TimestampMs > entryTimeMs {
				quality.TimeToFirstProfitMs = point.TimestampMs - entryTimeMs
			}
		}
		if point.UnrealizedPnL == quality.MaxAdverseExcursion && point.TimestampMs >= entryTimeMs {
			minPointTimestamp = point.TimestampMs
		}
	}

	if quality.MaxAdverseExcursion < 0 && entryTimeMs > 0 && minPointTimestamp > entryTimeMs {
		quality.TimeToMaxDrawdownMs = minPointTimestamp - entryTimeMs
	}

	if quality.MaxFavorableExcursion > 0 {
		quality.MFECapturedPct = clampDealReviewScore((caseRec.RealizedPnL / quality.MaxFavorableExcursion) * 100)
		quality.ProfitGivenBack = math.Max(0, quality.MaxFavorableExcursion-caseRec.RealizedPnL)
		quality.ProfitGivenBackPct = math.Max(0, (quality.ProfitGivenBack/quality.MaxFavorableExcursion)*100)
	}

	quality.PlannedRiskPct = calcDealReviewPlannedRiskPct(caseRec)
	quality.ExitEfficiencyScore = calculateDealReviewExitEfficiencyScore(caseRec, quality)
	quality.EntryTimingScore = calculateDealReviewEntryTimingScore(caseRec, quality)
	quality.RiskSizingScore = calculateDealReviewRiskSizingScore(caseRec, quality)

	return quality
}

func calcDealReviewSignedPriceMovePct(side string, entryPrice, markPrice float64) float64 {
	if entryPrice <= 0 || markPrice <= 0 {
		return 0
	}
	if normalizeDealReviewSide(side) == "SHORT" {
		return ((entryPrice - markPrice) / entryPrice) * 100
	}
	return ((markPrice - entryPrice) / entryPrice) * 100
}

func calcDealReviewPlannedRiskPct(caseRec *DealReviewCase) float64 {
	if caseRec == nil || caseRec.EntryPrice <= 0 || caseRec.OpenStopLoss <= 0 {
		return 0
	}
	stopDistancePct := math.Abs(caseRec.EntryPrice-caseRec.OpenStopLoss) / caseRec.EntryPrice * 100
	leverage := math.Max(float64(caseRec.Leverage), 1)
	return stopDistancePct * leverage
}

func calculateDealReviewExitEfficiencyScore(caseRec *DealReviewCase, quality dealReviewCaseQuality) float64 {
	if caseRec == nil {
		return 0
	}
	if quality.MaxFavorableExcursion <= 0 {
		if caseRec.RealizedPnL >= 0 {
			return 100
		}
		return 0
	}
	score := quality.MFECapturedPct
	if quality.ProfitGivenBackPct > 100 {
		score -= math.Min(quality.ProfitGivenBackPct-100, 25)
	}
	return clampDealReviewScore(score)
}

func calculateDealReviewEntryTimingScore(caseRec *DealReviewCase, quality dealReviewCaseQuality) float64 {
	if caseRec == nil {
		return 0
	}

	maxFavPct := math.Max(0, quality.MaxFavorableExcursionPct)
	maxAdvAbsPct := math.Abs(math.Min(0, quality.MaxAdverseExcursionPct))
	holdMs := caseRec.HoldDurationMs
	if holdMs <= 0 && caseRec.ExitTimeMs > caseRec.EntryTimeMs {
		holdMs = caseRec.ExitTimeMs - caseRec.EntryTimeMs
	}
	if holdMs <= 0 {
		holdMs = 1
	}

	favorableScale := clampDealReviewUnit(maxFavPct / 2.0)
	rewardToPain := 0.0
	switch {
	case maxFavPct > 0 && maxAdvAbsPct <= 0.0001:
		rewardToPain = 1
	case maxFavPct > 0:
		rewardToPain = clampDealReviewUnit((maxFavPct / (maxAdvAbsPct + 0.05)) / 2)
	}

	profitSpeed := 0.0
	if quality.TimeToFirstProfitMs > 0 {
		profitSpeed = clampDealReviewUnit(1 - float64(quality.TimeToFirstProfitMs)/float64(holdMs))
	}

	score := 25*favorableScale + 35*rewardToPain + 25*profitSpeed
	if maxFavPct > 0 {
		score += 15
	}
	if caseRec.RealizedPnL > 0 {
		score += 10
	}
	if maxFavPct <= 0 && caseRec.RealizedPnL < 0 {
		score -= 20
	}
	if maxAdvAbsPct > maxFavPct*1.5 && maxAdvAbsPct > 0.25 {
		score -= 20
	}
	return clampDealReviewScore(score)
}

func calculateDealReviewRiskSizingScore(caseRec *DealReviewCase, quality dealReviewCaseQuality) float64 {
	if caseRec == nil {
		return 0
	}

	effectiveRiskPct := quality.PlannedRiskPct
	if effectiveRiskPct <= 0 {
		effectiveRiskPct = math.Abs(quality.MaxAdverseExcursionPct) * math.Max(float64(caseRec.Leverage), 1)
	}
	switch {
	case effectiveRiskPct <= 0.5:
		return 95
	case effectiveRiskPct <= 1:
		return 90
	case effectiveRiskPct <= 2:
		return 80
	case effectiveRiskPct <= 3:
		return 65
	case effectiveRiskPct <= 5:
		return 45
	case effectiveRiskPct <= 8:
		return 25
	default:
		return 10
	}
}

func clampDealReviewUnit(value float64) float64 {
	return math.Max(0, math.Min(value, 1))
}

func clampDealReviewScore(value float64) float64 {
	return math.Max(0, math.Min(value, 100))
}

func (s *DealReviewStore) backfillCaseDecisionCyclePricePoints(caseRec *DealReviewCase, nowMs int64) error {
	if caseRec == nil || caseRec.PositionID <= 0 || caseRec.EntryTimeMs <= 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var position TraderPosition
		if err := tx.Where("id = ?", caseRec.PositionID).First(&position).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		return s.backfillCaseDecisionCyclePricePointsTx(tx, caseRec, &position, nowMs, true)
	})
}

func (s *DealReviewStore) upsertCyclePointRecord(caseRec *DealReviewCase, record *DecisionRecord, snapshot PositionSnapshot, timestampMs int64) error {
	if caseRec == nil || record == nil {
		return nil
	}

	unrealizedPnLPct := calcDealReviewUnrealizedPnLPct(caseRec.Side, snapshot.EntryPrice, snapshot.MarkPrice, snapshot.PositionAmt, snapshot.UnrealizedProfit)
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "position_id"}, {Name: "decision_cycle_number"}},
		DoUpdates: clause.Assignments(map[string]any{
			"deal_id":            caseRec.ID,
			"user_id":            caseRec.UserID,
			"trader_id":          caseRec.TraderID,
			"symbol":             caseRec.Symbol,
			"side":               caseRec.Side,
			"timestamp_ms":       timestampMs,
			"mark_price":         snapshot.MarkPrice,
			"entry_price":        snapshot.EntryPrice,
			"quantity":           math.Abs(snapshot.PositionAmt),
			"unrealized_pnl":     snapshot.UnrealizedProfit,
			"unrealized_pnl_pct": unrealizedPnLPct,
			"in_profit":          snapshot.UnrealizedProfit > 0,
			"updated_at":         time.Now().UTC(),
		}),
	}).Create(&DealReviewCyclePointRecord{
		UserID:              caseRec.UserID,
		TraderID:            caseRec.TraderID,
		DealID:              caseRec.ID,
		PositionID:          caseRec.PositionID,
		Symbol:              caseRec.Symbol,
		Side:                caseRec.Side,
		TimestampMs:         timestampMs,
		DecisionCycleNumber: record.CycleNumber,
		MarkPrice:           snapshot.MarkPrice,
		EntryPrice:          snapshot.EntryPrice,
		Quantity:            math.Abs(snapshot.PositionAmt),
		UnrealizedPnL:       snapshot.UnrealizedProfit,
		UnrealizedPnLPct:    unrealizedPnLPct,
		InProfit:            snapshot.UnrealizedProfit > 0,
	}).Error
}

func (s *DealReviewStore) upsertMarketPointTx(tx *gorm.DB, userID, traderID string, position *TraderPosition, snapshot PositionSnapshot, timestampMs int64, source string) error {
	if tx == nil || position == nil {
		return nil
	}

	caseID := ""
	var caseRec DealReviewCase
	if err := tx.Select("id").Where("position_id = ?", position.ID).First(&caseRec).Error; err == nil {
		caseID = caseRec.ID
	}

	unrealizedPnLPct := calcDealReviewUnrealizedPnLPct(position.Side, snapshot.EntryPrice, snapshot.MarkPrice, snapshot.PositionAmt, snapshot.UnrealizedProfit)
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "position_id"}, {Name: "source"}, {Name: "timestamp_ms"}},
		DoUpdates: clause.Assignments(map[string]any{
			"deal_id":            caseID,
			"user_id":            userID,
			"trader_id":          traderID,
			"symbol":             position.Symbol,
			"side":               position.Side,
			"mark_price":         snapshot.MarkPrice,
			"entry_price":        snapshot.EntryPrice,
			"quantity":           math.Abs(snapshot.PositionAmt),
			"unrealized_pnl":     snapshot.UnrealizedProfit,
			"unrealized_pnl_pct": unrealizedPnLPct,
			"in_profit":          snapshot.UnrealizedProfit > 0,
			"updated_at":         time.Now().UTC(),
		}),
	}).Create(&DealReviewMarketPointRecord{
		UserID:           userID,
		TraderID:         traderID,
		DealID:           caseID,
		PositionID:       position.ID,
		Symbol:           position.Symbol,
		Side:             position.Side,
		Source:           source,
		TimestampMs:      timestampMs,
		MarkPrice:        snapshot.MarkPrice,
		EntryPrice:       snapshot.EntryPrice,
		Quantity:         math.Abs(snapshot.PositionAmt),
		UnrealizedPnL:    snapshot.UnrealizedProfit,
		UnrealizedPnLPct: unrealizedPnLPct,
		InProfit:         snapshot.UnrealizedProfit > 0,
	}).Error
}

func (s *DealReviewStore) backfillCaseDecisionCyclePricePointsTx(tx *gorm.DB, caseRec *DealReviewCase, position *TraderPosition, nowMs int64, resetExisting bool) error {
	if tx == nil || caseRec == nil || position == nil || caseRec.PositionID <= 0 || caseRec.EntryTimeMs <= 0 {
		return nil
	}
	if resetExisting {
		if err := tx.Where("position_id = ?", caseRec.PositionID).Delete(&DealReviewCyclePointRecord{}).Error; err != nil {
			return err
		}
	}

	endMs := caseRec.ExitTimeMs
	if endMs <= 0 {
		endMs = nowMs
	}

	var dbRecords []DecisionRecordDB
	if err := tx.
		Where("trader_id = ? AND timestamp >= ? AND timestamp <= ? AND positions_json IS NOT NULL AND positions_json != '' AND positions_json != '[]'",
			caseRec.TraderID,
			time.UnixMilli(caseRec.EntryTimeMs).UTC(),
			time.UnixMilli(endMs).UTC()).
		Order("timestamp ASC").
		Find(&dbRecords).Error; err != nil {
		return err
	}

	for _, dbRecord := range dbRecords {
		record := dbRecord.toRecord()
		if len(record.Positions) == 0 {
			continue
		}
		timestampMs := record.Timestamp.UTC().UnixMilli()
		for _, snapshot := range record.Positions {
			if !dealReviewSnapshotMatchesPosition(snapshot, position, caseRec, timestampMs) {
				continue
			}
			if err := s.upsertCyclePointTx(tx, caseRec.UserID, record, position, snapshot, timestampMs); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *DealReviewStore) upsertCyclePointTx(tx *gorm.DB, userID string, record *DecisionRecord, position *TraderPosition, snapshot PositionSnapshot, timestampMs int64) error {
	if tx == nil || record == nil || position == nil {
		return nil
	}

	caseID := ""
	var caseRec DealReviewCase
	if err := tx.Select("id").Where("position_id = ?", position.ID).First(&caseRec).Error; err == nil {
		caseID = caseRec.ID
	}

	unrealizedPnLPct := calcDealReviewUnrealizedPnLPct(position.Side, snapshot.EntryPrice, snapshot.MarkPrice, snapshot.PositionAmt, snapshot.UnrealizedProfit)
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "position_id"}, {Name: "decision_cycle_number"}},
		DoUpdates: clause.Assignments(map[string]any{
			"deal_id":            caseID,
			"user_id":            userID,
			"trader_id":          record.TraderID,
			"symbol":             position.Symbol,
			"side":               position.Side,
			"timestamp_ms":       timestampMs,
			"mark_price":         snapshot.MarkPrice,
			"entry_price":        snapshot.EntryPrice,
			"quantity":           math.Abs(snapshot.PositionAmt),
			"unrealized_pnl":     snapshot.UnrealizedProfit,
			"unrealized_pnl_pct": unrealizedPnLPct,
			"in_profit":          snapshot.UnrealizedProfit > 0,
			"updated_at":         time.Now().UTC(),
		}),
	}).Create(&DealReviewCyclePointRecord{
		UserID:              userID,
		TraderID:            record.TraderID,
		DealID:              caseID,
		PositionID:          position.ID,
		Symbol:              position.Symbol,
		Side:                position.Side,
		TimestampMs:         timestampMs,
		DecisionCycleNumber: record.CycleNumber,
		MarkPrice:           snapshot.MarkPrice,
		EntryPrice:          snapshot.EntryPrice,
		Quantity:            math.Abs(snapshot.PositionAmt),
		UnrealizedPnL:       snapshot.UnrealizedProfit,
		UnrealizedPnLPct:    unrealizedPnLPct,
		InProfit:            snapshot.UnrealizedProfit > 0,
	}).Error
}

func (s *DealReviewStore) findPositionForTimelineSnapshotTx(tx *gorm.DB, traderID string, snapshot PositionSnapshot, timestampMs int64) (*TraderPosition, error) {
	if tx == nil || strings.TrimSpace(traderID) == "" || strings.TrimSpace(snapshot.Symbol) == "" {
		return nil, nil
	}

	var candidates []TraderPosition
	err := tx.Where("trader_id = ? AND symbol = ? AND side = ? AND entry_time <= ? AND (exit_time = 0 OR exit_time >= ?)",
		traderID,
		strings.TrimSpace(snapshot.Symbol),
		normalizeDealReviewSide(snapshot.Side),
		timestampMs,
		timestampMs).
		Order("entry_time DESC").
		Find(&candidates).Error
	if err != nil {
		return nil, err
	}

	var best *TraderPosition
	bestScore := math.MaxFloat64
	for i := range candidates {
		candidate := &candidates[i]
		if !dealReviewSnapshotMatchesPosition(snapshot, candidate, nil, timestampMs) {
			continue
		}
		score := dealReviewSnapshotMatchScore(snapshot, candidate, nil, timestampMs)
		if best == nil || score < bestScore {
			best = candidate
			bestScore = score
		}
	}
	return best, nil
}

func dealReviewSnapshotMatchesPosition(snapshot PositionSnapshot, position *TraderPosition, caseRec *DealReviewCase, timestampMs int64) bool {
	if position == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(snapshot.Symbol), strings.TrimSpace(position.Symbol)) {
		return false
	}
	if normalizeDealReviewSide(snapshot.Side) != normalizeDealReviewSide(position.Side) {
		return false
	}
	if timestampMs > 0 {
		if position.EntryTime > 0 && timestampMs < position.EntryTime {
			return false
		}
		if position.ExitTime > 0 && timestampMs > position.ExitTime {
			return false
		}
	}

	expectedEntryPrice := position.EntryPrice
	if caseRec != nil && caseRec.EntryPrice > 0 {
		expectedEntryPrice = caseRec.EntryPrice
	}
	if expectedEntryPrice > 0 && snapshot.EntryPrice > 0 && !dealReviewPricesRoughlyMatch(expectedEntryPrice, snapshot.EntryPrice) {
		return false
	}

	expectedQuantity := position.EntryQuantity
	if expectedQuantity == 0 {
		expectedQuantity = position.Quantity
	}
	if caseRec != nil {
		if caseRec.EntryQuantity > 0 {
			expectedQuantity = caseRec.EntryQuantity
		} else if expectedQuantity == 0 && caseRec.ExitQuantity > 0 {
			expectedQuantity = caseRec.ExitQuantity
		}
	}
	if expectedEntryPrice <= 0 && expectedQuantity > 0 && snapshot.PositionAmt != 0 &&
		!dealReviewQuantitiesRoughlyMatch(expectedQuantity, math.Abs(snapshot.PositionAmt)) {
		return false
	}

	return true
}

func dealReviewSnapshotMatchScore(snapshot PositionSnapshot, position *TraderPosition, caseRec *DealReviewCase, timestampMs int64) float64 {
	if position == nil {
		return math.MaxFloat64
	}

	score := 0.0
	expectedEntryPrice := position.EntryPrice
	if caseRec != nil && caseRec.EntryPrice > 0 {
		expectedEntryPrice = caseRec.EntryPrice
	}
	if expectedEntryPrice > 0 && snapshot.EntryPrice > 0 {
		score += math.Abs(expectedEntryPrice-snapshot.EntryPrice) / math.Max(expectedEntryPrice, 1)
	}

	expectedQuantity := position.EntryQuantity
	if expectedQuantity == 0 {
		expectedQuantity = position.Quantity
	}
	if caseRec != nil {
		if caseRec.EntryQuantity > 0 {
			expectedQuantity = caseRec.EntryQuantity
		} else if expectedQuantity == 0 && caseRec.ExitQuantity > 0 {
			expectedQuantity = caseRec.ExitQuantity
		}
	}
	if expectedQuantity > 0 && snapshot.PositionAmt != 0 {
		score += (math.Abs(expectedQuantity-math.Abs(snapshot.PositionAmt)) / math.Max(expectedQuantity, 1)) * 0.05
	}

	if timestampMs > 0 && position.EntryTime > 0 {
		score += (float64(timestampMs-position.EntryTime) / float64(time.Hour.Milliseconds())) * 0.0001
	}
	return score
}

func dealReviewPricesRoughlyMatch(expected, actual float64) bool {
	if expected <= 0 || actual <= 0 {
		return true
	}
	diff := math.Abs(expected - actual)
	tolerance := math.Max(expected, actual) * 0.0025
	if tolerance < 0.000001 {
		tolerance = 0.000001
	}
	return diff <= tolerance
}

func dealReviewQuantitiesRoughlyMatch(expected, actual float64) bool {
	if expected <= 0 || actual <= 0 {
		return true
	}
	diff := math.Abs(expected - actual)
	tolerance := math.Max(expected, actual) * 0.05
	if tolerance < 0.0001 {
		tolerance = 0.0001
	}
	return diff <= tolerance
}

func calcDealReviewUnrealizedPnLPct(side string, entryPrice, markPrice, quantity, unrealizedPnL float64) float64 {
	notional := math.Abs(quantity) * entryPrice
	if notional > 0 {
		return (unrealizedPnL / notional) * 100
	}
	if entryPrice <= 0 || markPrice <= 0 {
		return 0
	}
	if normalizeDealReviewSide(side) == "SHORT" {
		return ((entryPrice - markPrice) / entryPrice) * 100
	}
	return ((markPrice - entryPrice) / entryPrice) * 100
}

func computeDealReviewPriceTimelineSummary(points []DealReviewPriceTimelinePoint) DealReviewPriceTimelineSummary {
	summary := DealReviewPriceTimelineSummary{}
	if len(points) == 0 {
		return summary
	}

	summary.PointCount = len(points)
	summary.MaxUnrealizedPnL = points[0].UnrealizedPnL
	summary.MaxUnrealizedPnLPct = points[0].UnrealizedPnLPct
	summary.MinUnrealizedPnL = points[0].UnrealizedPnL
	summary.MinUnrealizedPnLPct = points[0].UnrealizedPnLPct
	summary.HighestMarkPrice = points[0].MarkPrice
	summary.LowestMarkPrice = points[0].MarkPrice

	for _, point := range points {
		if point.Source == "cycle" {
			summary.CycleSamples++
		} else if point.Source == "platform" {
			summary.PlatformSamples++
		}
		if point.UnrealizedPnL > 0 {
			summary.EverInProfit = true
		}
		if point.UnrealizedPnL > summary.MaxUnrealizedPnL {
			summary.MaxUnrealizedPnL = point.UnrealizedPnL
		}
		if point.UnrealizedPnLPct > summary.MaxUnrealizedPnLPct {
			summary.MaxUnrealizedPnLPct = point.UnrealizedPnLPct
		}
		if point.UnrealizedPnL < summary.MinUnrealizedPnL {
			summary.MinUnrealizedPnL = point.UnrealizedPnL
		}
		if point.UnrealizedPnLPct < summary.MinUnrealizedPnLPct {
			summary.MinUnrealizedPnLPct = point.UnrealizedPnLPct
		}
		if point.MarkPrice > summary.HighestMarkPrice {
			summary.HighestMarkPrice = point.MarkPrice
		}
		if point.MarkPrice < summary.LowestMarkPrice {
			summary.LowestMarkPrice = point.MarkPrice
		}
	}
	return summary
}

func normalizeDealReviewLivePointSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "platform", "exchange", "monitor":
		return "platform"
	default:
		return strings.ToLower(strings.TrimSpace(source))
	}
}

func normalizeDealReviewLivePointTimestampMs(timestampMs int64) int64 {
	if timestampMs <= 0 {
		return 0
	}
	return (timestampMs / 1000) * 1000
}

func dealReviewTimelineSourceRank(source string) int {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "entry":
		return 0
	case "cycle":
		return 1
	case "platform":
		return 2
	case "exit":
		return 3
	default:
		return 4
	}
}

func (s *DealReviewStore) BackfillEventDecisionArtifacts() error {
	const batchSize = 250

	return s.db.Transaction(func(tx *gorm.DB) error {
		for offset := 0; ; offset += batchSize {
			var events []DealReviewEvent
			if err := tx.Where("decision_cycle_number > 0").
				Order("created_at ASC").
				Offset(offset).
				Limit(batchSize).
				Find(&events).Error; err != nil {
				return err
			}
			if len(events) == 0 {
				return nil
			}

			for i := range events {
				before := strings.TrimSpace(events[i].SnapshotJSON)
				s.enrichEventSnapshotFromDecisionRecordTx(tx, &events[i])
				after := strings.TrimSpace(events[i].SnapshotJSON)
				if before != after {
					if err := tx.Model(&DealReviewEvent{}).
						Where("id = ?", events[i].ID).
						Update("snapshot_json", after).Error; err != nil {
						return err
					}
				}
				if err := s.syncCaseMarketContextFromEventTx(tx, &events[i]); err != nil {
					return err
				}
			}
		}
	})
}

func parseJSONStringSlice(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}

func normalizeDealReviewLabels(labels []string) []string {
	if len(labels) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(labels))
	result := make([]string, 0, len(labels))
	for _, label := range labels {
		normalized := strings.TrimSpace(label)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func truncateAnomalySymbols(items []DealReviewAnomalySymbol, n int) []DealReviewAnomalySymbol {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func truncateAnomalyBuckets(items []DealReviewAnomalyBucket, n int) []DealReviewAnomalyBucket {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func truncateAnomalyReasons(items []DealReviewAnomalyCloseReason, n int) []DealReviewAnomalyCloseReason {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func truncateAnomalyGiveBacks(items []DealReviewAnomalyGiveBack, n int) []DealReviewAnomalyGiveBack {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func truncateAnomalyStopOuts(items []DealReviewAnomalyStopOut, n int) []DealReviewAnomalyStopOut {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func truncateAnomalySizings(items []DealReviewAnomalySizing, n int) []DealReviewAnomalySizing {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func truncateAnomalyCloseReasonQualities(items []DealReviewAnomalyCloseReasonQuality, n int) []DealReviewAnomalyCloseReasonQuality {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func truncateAnomalyExitUncertainties(items []DealReviewAnomalyExitUncertainty, n int) []DealReviewAnomalyExitUncertainty {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func truncateAnomalySymbolEdgeFailures(items []DealReviewAnomalySymbolEdgeFailure, n int) []DealReviewAnomalySymbolEdgeFailure {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func buildAnomalyNotes(summary *DealReviewAnomalySummary) []string {
	notes := []string{}
	if summary == nil {
		return notes
	}
	if len(summary.WorstSymbols) > 0 && summary.WorstSymbols[0].NetPnL < 0 {
		notes = append(notes, fmt.Sprintf("Weakest symbol cohort: %s (%.2f net PnL across %d deals).",
			summary.WorstSymbols[0].Symbol, summary.WorstSymbols[0].NetPnL, summary.WorstSymbols[0].Deals))
	}
	if len(summary.OvertradedSymbols) > 0 && summary.OvertradedSymbols[0].NetPnL < 0 {
		notes = append(notes, fmt.Sprintf("Potential overtrading concentration: %s traded %d times for %.2f net PnL.",
			summary.OvertradedSymbols[0].Symbol, summary.OvertradedSymbols[0].Deals, summary.OvertradedSymbols[0].NetPnL))
	}
	if len(summary.WeakBuckets) > 0 && summary.WeakBuckets[0].NetPnL < 0 {
		notes = append(notes, fmt.Sprintf("Weakest open bucket: %s (%.2f net PnL, %.1f%% win rate).",
			summary.WeakBuckets[0].Bucket, summary.WeakBuckets[0].NetPnL, summary.WeakBuckets[0].WinRate))
	}
	if len(summary.WeakCloseReasons) > 0 && summary.WeakCloseReasons[0].NetPnL < 0 {
		notes = append(notes, fmt.Sprintf("Most expensive close reason: %s (%.2f net PnL across %d closes).",
			summary.WeakCloseReasons[0].Reason, summary.WeakCloseReasons[0].NetPnL, summary.WeakCloseReasons[0].Deals))
	}
	if len(summary.ProfitGiveBackHotspots) > 0 {
		notes = append(notes, fmt.Sprintf("Largest profit give-back hotspot: %s gave back %.1f%% of available MFE on average.",
			summary.ProfitGiveBackHotspots[0].Symbol,
			summary.ProfitGiveBackHotspots[0].AvgGiveBackPct))
	}
	if len(summary.EarlyStopOutHotspots) > 0 {
		notes = append(notes, fmt.Sprintf("Repeated early stop-out cluster: %s with %d fast stop-outs (avg hold %s).",
			summary.EarlyStopOutHotspots[0].Symbol,
			summary.EarlyStopOutHotspots[0].Deals,
			formatDurationForNote(summary.EarlyStopOutHotspots[0].AvgHoldMs)))
	}
	if len(summary.OversizedLossHotspots) > 0 {
		notes = append(notes, fmt.Sprintf("Sizing risk hotspot: %s averages %.1f sizing score and %.1f%% planned risk.",
			summary.OversizedLossHotspots[0].Symbol,
			summary.OversizedLossHotspots[0].AvgRiskSizingScore,
			summary.OversizedLossHotspots[0].AvgPlannedRiskPct))
	}
	if len(summary.CloseReasonQuality) > 0 {
		notes = append(notes, fmt.Sprintf("Weakest close-quality cohort: %s exits average %.1f exit-efficiency score.",
			summary.CloseReasonQuality[0].Reason,
			summary.CloseReasonQuality[0].AvgExitEfficiencyScore))
	}
	if len(summary.ExitUncertainty) > 0 {
		notes = append(notes, fmt.Sprintf("Largest uncertain exit cohort: %s via %s accounts for %.1f%% of closed deals.",
			summary.ExitUncertainty[0].Label,
			summary.ExitUncertainty[0].ExitOrigin,
			summary.ExitUncertainty[0].SharePct))
	}
	if len(summary.SymbolEdgeFailures) > 0 {
		notes = append(notes, fmt.Sprintf("Repeated symbol-specific edge failure: %s %s matched %d filtered deals with %.1f%% contradiction and %.2f%% historical avg PnL.",
			summary.SymbolEdgeFailures[0].Symbol,
			summary.SymbolEdgeFailures[0].Side,
			summary.SymbolEdgeFailures[0].SliceDeals,
			summary.SymbolEdgeFailures[0].ContradictionScore*100,
			summary.SymbolEdgeFailures[0].AvgPnLPct))
	}
	return notes
}

func formatDurationForNote(durationMs int64) string {
	if durationMs <= 0 {
		return "0m"
	}
	return (time.Duration(durationMs) * time.Millisecond).Round(time.Minute).String()
}

func isDealReviewBadEntry(caseRec DealReviewCase) bool {
	return caseRec.EntryTimingScore < 35
}

func isDealReviewBadExit(caseRec DealReviewCase) bool {
	return caseRec.MaxFavorableExcursion > 0.05 && caseRec.ExitEfficiencyScore < 35
}

func isDealReviewAvoidableLoss(caseRec DealReviewCase) bool {
	return caseRec.RealizedPnL < 0 && (caseRec.ProfitGivenBack > 0.05 || caseRec.MaxFavorableExcursion > 0.05)
}

func isDealReviewStrongEntryWeakExit(caseRec DealReviewCase) bool {
	return caseRec.EntryTimingScore >= 70 && caseRec.MaxFavorableExcursion > 0.05 && caseRec.ExitEfficiencyScore < 40
}

func isDealReviewWeakEntryLuckyExit(caseRec DealReviewCase) bool {
	return caseRec.RealizedPnL > 0 && caseRec.EntryTimingScore < 35
}

func isDealReviewProfitGiveBackHotspot(caseRec DealReviewCase) bool {
	return caseRec.MaxFavorableExcursion > 0.05 && (caseRec.ProfitGivenBackPct >= 40 || caseRec.MFECapturedPct <= 45)
}

func isDealReviewEarlyStopOut(caseRec DealReviewCase) bool {
	if caseRec.RealizedPnL >= 0 || caseRec.TimeToFirstProfitMs > 0 {
		return false
	}
	return strings.Contains(caseRec.CloseReason, "stop") || (caseRec.HoldDurationMs > 0 && caseRec.HoldDurationMs <= int64(20*time.Minute/time.Millisecond))
}

func isDealReviewOversizedLoss(caseRec DealReviewCase) bool {
	return caseRec.RealizedPnL < 0 && (caseRec.RiskSizingScore > 0 && caseRec.RiskSizingScore <= 35 || caseRec.PlannedRiskPct >= 5)
}

func isDealReviewExitUncertainty(caseRec DealReviewCase) bool {
	reason := strings.TrimSpace(strings.ToLower(caseRec.CloseReason))
	quality := strings.TrimSpace(caseRec.ExitReasonQuality)
	origin := strings.TrimSpace(caseRec.ExitOrigin)

	return reason == "unknown" ||
		quality == DealReviewExitReasonQualityLowConfidence ||
		origin == DealReviewExitOriginExchangeSyncUnknown ||
		origin == DealReviewExitOriginSyncedCloseFill
}

func normalizeDealReviewSide(side string) string {
	normalized := strings.ToUpper(strings.TrimSpace(side))
	switch normalized {
	case "BUY":
		return "LONG"
	case "SELL":
		return "SHORT"
	default:
		return normalized
	}
}

func normalizeCloseReason(reason string) string {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	switch normalized {
	case "":
		return ""
	case "manual", "manual_close", "manual-close", "user_close", "user-close":
		return "manual_exit"
	case "takeprofit", "take_profit", "tp":
		return "take_profit"
	case "stoploss", "stop_loss", "sl":
		return "stop_loss"
	case "trailing", "trailing-stop", "trailing_stop":
		return "trailing_stop"
	case "strategy", "strategy_exit", "ai_close", "ai_exit":
		return "ai_exit"
	default:
		return normalized
	}
}

func (s *DealReviewStore) enrichEventSnapshotFromDecisionRecordTx(tx *gorm.DB, event *DealReviewEvent) {
	if tx == nil || event == nil || strings.TrimSpace(event.TraderID) == "" || event.DecisionCycleNumber <= 0 {
		return
	}

	record, err := NewDecisionStore(tx).GetByCycleNumber(event.TraderID, event.DecisionCycleNumber)
	if err != nil || record == nil {
		return
	}

	snapshot := parseDealReviewEventSnapshot(event.SnapshotJSON)
	changed := mergeDealReviewSnapshotDecisionArtifacts(&snapshot, record)
	if snapshot.MarketContext == nil {
		if marketContext := buildDealReviewMarketContextFromDecisionRecord(record, event.Stage, event.Symbol, event.Side, event.DecisionTimestamp); marketContext != nil {
			snapshot.MarketContext = marketContext
			changed = true
		}
	}
	if !changed {
		return
	}
	if raw, err := json.Marshal(snapshot); err == nil {
		event.SnapshotJSON = string(raw)
	}
}

func (s *DealReviewStore) syncCaseMarketContextFromEventTx(tx *gorm.DB, event *DealReviewEvent) error {
	if tx == nil || event == nil || strings.TrimSpace(event.DealID) == "" {
		return nil
	}

	var caseRec DealReviewCase
	if err := tx.Where("id = ?", event.DealID).First(&caseRec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	snapshot := parseDealReviewEventSnapshot(event.SnapshotJSON)
	if !applyDealReviewMarketContextToCase(&caseRec, event.Stage, snapshot.MarketContext) {
		return nil
	}
	return tx.Save(&caseRec).Error
}

func parseDealReviewEventSnapshot(raw string) DealReviewEventSnapshot {
	snapshot := DealReviewEventSnapshot{}
	if strings.TrimSpace(raw) == "" {
		return snapshot
	}
	_ = json.Unmarshal([]byte(raw), &snapshot)
	return snapshot
}

func mergeDealReviewSnapshotDecisionArtifacts(snapshot *DealReviewEventSnapshot, record *DecisionRecord) bool {
	if snapshot == nil || record == nil {
		return false
	}

	changed := false
	if snapshot.SystemPrompt == "" && strings.TrimSpace(record.SystemPrompt) != "" {
		snapshot.SystemPrompt = record.SystemPrompt
		changed = true
	}
	if snapshot.UserPrompt == "" && strings.TrimSpace(record.InputPrompt) != "" {
		snapshot.UserPrompt = record.InputPrompt
		changed = true
	}
	if snapshot.DecisionJSON == "" && strings.TrimSpace(record.DecisionJSON) != "" {
		snapshot.DecisionJSON = record.DecisionJSON
		changed = true
	}
	if snapshot.RawResponse == "" && strings.TrimSpace(record.RawResponse) != "" {
		snapshot.RawResponse = record.RawResponse
		changed = true
	}
	if snapshot.CoTTrace == "" && strings.TrimSpace(record.CoTTrace) != "" {
		snapshot.CoTTrace = record.CoTTrace
		changed = true
	}
	if snapshot.AIRequestMs == 0 && record.AIRequestDurationMs > 0 {
		snapshot.AIRequestMs = record.AIRequestDurationMs
		changed = true
	}
	if isZeroAccountSnapshot(snapshot.AccountState) && !isZeroAccountSnapshot(record.AccountState) {
		snapshot.AccountState = record.AccountState
		changed = true
	}
	if len(snapshot.Positions) == 0 && len(record.Positions) > 0 {
		snapshot.Positions = append([]PositionSnapshot(nil), record.Positions...)
		changed = true
	}
	if len(snapshot.CandidateCoins) == 0 && len(record.CandidateCoins) > 0 {
		snapshot.CandidateCoins = append([]string(nil), record.CandidateCoins...)
		changed = true
	}
	if len(snapshot.CandidateDetails) == 0 && len(record.CandidateDetails) > 0 {
		snapshot.CandidateDetails = append([]CandidateDetail(nil), record.CandidateDetails...)
		changed = true
	}
	if len(snapshot.ExecutionLog) == 0 && len(record.ExecutionLog) > 0 {
		snapshot.ExecutionLog = append([]string(nil), record.ExecutionLog...)
		changed = true
	}
	return changed
}

func isZeroAccountSnapshot(snapshot AccountSnapshot) bool {
	return snapshot.TotalBalance == 0 &&
		snapshot.AvailableBalance == 0 &&
		snapshot.TotalUnrealizedProfit == 0 &&
		snapshot.PositionCount == 0 &&
		snapshot.MarginUsedPct == 0 &&
		snapshot.InitialBalance == 0
}

func isAmbiguousCloseReason(reason string) bool {
	switch normalizeCloseReason(reason) {
	case "", "unknown", "sync":
		return true
	default:
		return false
	}
}

func closeReasonCanBeRefined(reason string) bool {
	switch normalizeCloseReason(reason) {
	case "", "unknown", "sync", "manual_exit":
		return true
	default:
		return false
	}
}

func closeReasonStrength(reason string) int {
	switch normalizeCloseReason(reason) {
	case "", "unknown", "sync":
		return 0
	case "manual_exit":
		return 1
	case "ai_exit":
		return 2
	case "stop_loss", "take_profit", "trailing_stop":
		return 3
	default:
		return 2
	}
}

func shouldReplaceCloseReason(existing, candidate string) bool {
	normalizedCandidate := normalizeCloseReason(candidate)
	if normalizedCandidate == "" {
		return false
	}

	normalizedExisting := normalizeCloseReason(existing)
	if normalizedExisting == "manual_exit" && normalizedCandidate == "unknown" {
		return true
	}
	if normalizedExisting == normalizedCandidate {
		return true
	}

	return closeReasonStrength(normalizedCandidate) > closeReasonStrength(normalizedExisting)
}

func expectedDealReviewAction(stage, side string) string {
	normalizedSide := strings.ToLower(strings.TrimSpace(normalizeDealReviewSide(side)))
	if normalizedSide == "" {
		return ""
	}
	return fmt.Sprintf("%s_%s", strings.ToLower(strings.TrimSpace(stage)), normalizedSide)
}

func decisionMatchesDealReview(action DecisionAction, symbol, expectedAction string) bool {
	if !strings.EqualFold(strings.TrimSpace(action.Symbol), symbol) {
		return false
	}
	normalizedAction := strings.ToLower(strings.TrimSpace(action.Action))
	if normalizedAction == expectedAction {
		return true
	}
	switch expectedAction {
	case "close_long":
		return normalizedAction == "auto_close_long"
	case "close_short":
		return normalizedAction == "auto_close_short"
	default:
		return false
	}
}

func inferCloseReasonFromTargets(caseRec *DealReviewCase, position *TraderPosition) dealReviewCloseInferenceResult {
	if caseRec == nil || position == nil || position.EntryPrice <= 0 || position.ExitPrice <= 0 {
		return dealReviewCloseInferenceResult{}
	}

	const targetTolerance = 0.0075 // 0.75%

	entryPrice := position.EntryPrice
	exitPrice := position.ExitPrice
	side := normalizeDealReviewSide(position.Side)
	stopLoss := caseRec.OpenStopLoss
	takeProfit := caseRec.OpenTakeProfit

	switch side {
	case "LONG":
		if stopLoss > 0 && stopLoss < entryPrice && exitPrice <= stopLoss*(1+targetTolerance) {
			distanceBps := relativeDistanceBps(exitPrice, stopLoss)
			return dealReviewCloseInferenceResult{
				Reason:              "stop_loss",
				InferredBy:          "target proximity to stored stop-loss / take-profit",
				ExitOrigin:          DealReviewExitOriginTargetProximity,
				ExitReasonQuality:   DealReviewExitReasonQualityHighConfidence,
				ExitEvidenceSummary: fmt.Sprintf("Exit price landed %.1f bps from the stored stop loss target.", distanceBps),
				ExitEvidence: &DealReviewExitEvidence{
					MatchedBy:             "target_proximity",
					TargetStopLoss:        stopLoss,
					TargetTakeProfit:      takeProfit,
					StopLossDistanceBps:   distanceBps,
					TakeProfitDistanceBps: relativeDistanceBps(exitPrice, takeProfit),
				},
			}
		}
		if takeProfit > 0 && takeProfit > entryPrice && exitPrice >= takeProfit*(1-targetTolerance) {
			distanceBps := relativeDistanceBps(exitPrice, takeProfit)
			return dealReviewCloseInferenceResult{
				Reason:              "take_profit",
				InferredBy:          "target proximity to stored stop-loss / take-profit",
				ExitOrigin:          DealReviewExitOriginTargetProximity,
				ExitReasonQuality:   DealReviewExitReasonQualityHighConfidence,
				ExitEvidenceSummary: fmt.Sprintf("Exit price landed %.1f bps from the stored take profit target.", distanceBps),
				ExitEvidence: &DealReviewExitEvidence{
					MatchedBy:             "target_proximity",
					TargetStopLoss:        stopLoss,
					TargetTakeProfit:      takeProfit,
					StopLossDistanceBps:   relativeDistanceBps(exitPrice, stopLoss),
					TakeProfitDistanceBps: distanceBps,
				},
			}
		}
	case "SHORT":
		if stopLoss > 0 && stopLoss > entryPrice && exitPrice >= stopLoss*(1-targetTolerance) {
			distanceBps := relativeDistanceBps(exitPrice, stopLoss)
			return dealReviewCloseInferenceResult{
				Reason:              "stop_loss",
				InferredBy:          "target proximity to stored stop-loss / take-profit",
				ExitOrigin:          DealReviewExitOriginTargetProximity,
				ExitReasonQuality:   DealReviewExitReasonQualityHighConfidence,
				ExitEvidenceSummary: fmt.Sprintf("Exit price landed %.1f bps from the stored stop loss target.", distanceBps),
				ExitEvidence: &DealReviewExitEvidence{
					MatchedBy:             "target_proximity",
					TargetStopLoss:        stopLoss,
					TargetTakeProfit:      takeProfit,
					StopLossDistanceBps:   distanceBps,
					TakeProfitDistanceBps: relativeDistanceBps(exitPrice, takeProfit),
				},
			}
		}
		if takeProfit > 0 && takeProfit < entryPrice && exitPrice <= takeProfit*(1+targetTolerance) {
			distanceBps := relativeDistanceBps(exitPrice, takeProfit)
			return dealReviewCloseInferenceResult{
				Reason:              "take_profit",
				InferredBy:          "target proximity to stored stop-loss / take-profit",
				ExitOrigin:          DealReviewExitOriginTargetProximity,
				ExitReasonQuality:   DealReviewExitReasonQualityHighConfidence,
				ExitEvidenceSummary: fmt.Sprintf("Exit price landed %.1f bps from the stored take profit target.", distanceBps),
				ExitEvidence: &DealReviewExitEvidence{
					MatchedBy:             "target_proximity",
					TargetStopLoss:        stopLoss,
					TargetTakeProfit:      takeProfit,
					StopLossDistanceBps:   relativeDistanceBps(exitPrice, stopLoss),
					TakeProfitDistanceBps: distanceBps,
				},
			}
		}
	}

	return dealReviewCloseInferenceResult{}
}

func inferCloseReasonFromDecision(event *DealReviewEvent) dealReviewCloseInferenceResult {
	if event == nil {
		return dealReviewCloseInferenceResult{}
	}
	if event.DecisionCycleNumber <= 0 && event.Source != DealReviewEventSourceAIDecision {
		return dealReviewCloseInferenceResult{}
	}

	reasoning := strings.TrimSpace(event.Reasoning)
	if reasoning == "" || strings.HasPrefix(reasoning, "Position closed via synced exchange event") {
		return dealReviewCloseInferenceResult{}
	}

	result := dealReviewCloseInferenceResult{
		Reason:            "ai_exit",
		InferredBy:        "matched AI close decision context",
		ExitOrigin:        DealReviewExitOriginAIDecision,
		ExitReasonQuality: DealReviewExitReasonQualityExplicit,
		ExitEvidence: &DealReviewExitEvidence{
			MatchedBy:           "ai_close_decision",
			DecisionCycleNumber: event.DecisionCycleNumber,
			DecisionAction:      strings.TrimSpace(event.Action),
		},
	}
	normalized := normalizeReasoningHint(reasoning + " " + event.Action)
	switch {
	case strings.Contains(normalized, "trailing stop"):
		result.Reason = "trailing_stop"
	case strings.Contains(normalized, "stop loss"), strings.Contains(normalized, "stop hit"):
		result.Reason = "stop_loss"
	case strings.Contains(normalized, "take profit"), strings.Contains(normalized, "profit take"):
		result.Reason = "take_profit"
	}
	if result.ExitEvidence != nil && result.ExitEvidence.DecisionAction == "" {
		result.ExitEvidence.DecisionAction = strings.TrimSpace(event.Action)
	}
	result.ExitEvidenceSummary = fmt.Sprintf(
		"Matched AI close decision cycle %d (%s).",
		event.DecisionCycleNumber,
		strings.TrimSpace(event.Action),
	)
	return result
}

func inferCloseReasonFromOrderContext(caseRec *DealReviewCase, ctx *dealReviewExitExecutionContext) dealReviewCloseInferenceResult {
	if ctx == nil {
		return dealReviewCloseInferenceResult{}
	}

	if ctx.Order != nil {
		typeHint := dealReviewOrderTypeHint(ctx.Order)
		orderTypeLabel := strings.TrimSpace(ctx.Order.Type)
		if orderTypeLabel == "" {
			orderTypeLabel = "trigger"
		}
		explicitReason := classifyDealReviewTriggerOrderSubtype(ctx.Order)
		triggerOrderLike := explicitReason != "" ||
			strings.Contains(typeHint, "stop") ||
			strings.Contains(typeHint, "take profit") ||
			strings.Contains(typeHint, "takeprofit") ||
			strings.Contains(typeHint, "trigger") ||
			ctx.Order.StopPrice > 0

		if explicitReason != "" {
			return dealReviewCloseInferenceResult{
				Reason:              explicitReason,
				InferredBy:          "matched synced close fill / order",
				ExitOrigin:          DealReviewExitOriginSyncedTriggerOrder,
				ExitReasonQuality:   DealReviewExitReasonQualityExplicit,
				ExitEvidenceSummary: fmt.Sprintf("Matched filled %s trigger order.", orderTypeLabel),
				ExitEvidence:        buildDealReviewOrderExitEvidence(ctx.Order, ctx.Fill, "matched_exit_order"),
			}
		}
		if triggerOrderLike {
			triggerMatchPrice := resolveDealReviewStopMatchPrice(ctx)
			if priceReason := inferCloseReasonFromTriggerPrice(caseRec, triggerMatchPrice); priceReason != "" {
				return dealReviewCloseInferenceResult{
					Reason:              priceReason,
					InferredBy:          "matched synced close fill / order",
					ExitOrigin:          DealReviewExitOriginSyncedTriggerOrder,
					ExitReasonQuality:   DealReviewExitReasonQualityHighConfidence,
					ExitEvidenceSummary: fmt.Sprintf("Matched filled %s trigger order at %.8f near the stored %s target.", orderTypeLabel, triggerMatchPrice, formatDealReviewTargetName(priceReason)),
					ExitEvidence:        buildDealReviewOrderExitEvidence(ctx.Order, ctx.Fill, "matched_trigger_price"),
				}
			}
			if trailingReason := inferCloseReasonFromMovedStopOrder(caseRec, ctx, orderTypeLabel); trailingReason.Reason != "" {
				return trailingReason
			}
			if genericStopReason := inferCloseReasonFromGenericStopOrder(caseRec, ctx, orderTypeLabel); genericStopReason.Reason != "" {
				return genericStopReason
			}
			if strings.Contains(typeHint, "stop order") || strings.EqualFold(strings.TrimSpace(ctx.Order.Type), "Stop") || ctx.Order.StopPrice > 0 {
				return dealReviewCloseInferenceResult{
					Reason:              "unknown",
					InferredBy:          "matched synced close fill / order",
					ExitOrigin:          DealReviewExitOriginSyncedTriggerOrder,
					ExitReasonQuality:   DealReviewExitReasonQualityLowConfidence,
					ExitEvidenceSummary: fmt.Sprintf("Matched filled generic %s trigger order, but the exchange subtype could not be tied to a stored stop loss / take profit target or trailing-stop update.", orderTypeLabel),
					ExitEvidence:        buildDealReviewOrderExitEvidence(ctx.Order, ctx.Fill, "matched_exit_order"),
				}
			}
		}
		if strings.Contains(typeHint, "market") || strings.EqualFold(strings.TrimSpace(ctx.Order.Type), "Market") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(ctx.Order.OrderAction)), "close_") {
			return dealReviewCloseInferenceResult{
				Reason:              "manual_exit",
				InferredBy:          "matched synced close fill / order",
				ExitOrigin:          DealReviewExitOriginSyncedMarketOrder,
				ExitReasonQuality:   DealReviewExitReasonQualityHighConfidence,
				ExitEvidenceSummary: fmt.Sprintf("Matched filled %s close order.", orderTypeLabel),
				ExitEvidence:        buildDealReviewOrderExitEvidence(ctx.Order, ctx.Fill, "matched_exit_order"),
			}
		}
	}

	if ctx.Fill != nil && strings.TrimSpace(ctx.Fill.ExchangeOrderID) != "" {
		return dealReviewCloseInferenceResult{
			Reason:              "unknown",
			InferredBy:          "matched synced close fill / order",
			ExitOrigin:          DealReviewExitOriginSyncedCloseFill,
			ExitReasonQuality:   DealReviewExitReasonQualityLowConfidence,
			ExitEvidenceSummary: "Matched synced close fill without a typed trigger order.",
			ExitEvidence:        buildDealReviewOrderExitEvidence(nil, ctx.Fill, "matched_close_fill"),
		}
	}

	return dealReviewCloseInferenceResult{}
}

func inferCloseReasonFromTriggerPrice(caseRec *DealReviewCase, triggerPrice float64) string {
	if caseRec == nil || triggerPrice <= 0 {
		return ""
	}

	const priceMatchTolerance = 0.0025 // 0.25%

	if caseRec.OpenStopLoss > 0 && nearlyEqualRelative(triggerPrice, caseRec.OpenStopLoss, priceMatchTolerance) {
		return "stop_loss"
	}
	if caseRec.OpenTakeProfit > 0 && nearlyEqualRelative(triggerPrice, caseRec.OpenTakeProfit, priceMatchTolerance) {
		return "take_profit"
	}
	return ""
}

func inferCloseReasonFromMovedStopOrder(caseRec *DealReviewCase, ctx *dealReviewExitExecutionContext, orderTypeLabel string) dealReviewCloseInferenceResult {
	matchPrice := resolveDealReviewStopMatchPrice(ctx)
	if caseRec == nil || ctx == nil || ctx.Order == nil || matchPrice <= 0 {
		return dealReviewCloseInferenceResult{}
	}
	if !isMovedTrailingStopPrice(caseRec, matchPrice) {
		return dealReviewCloseInferenceResult{}
	}

	evidence := buildDealReviewOrderExitEvidence(ctx.Order, ctx.Fill, "matched_moved_stop_order")
	evidence.TargetStopLoss = caseRec.OpenStopLoss
	evidence.TargetTakeProfit = caseRec.OpenTakeProfit
	evidence.PreviousStopPrice = caseRec.OpenStopLoss
	evidence.NewStopPrice = matchPrice
	evidence.StopLossDistanceBps = relativeDistanceBps(matchPrice, caseRec.OpenStopLoss)
	evidence.TakeProfitDistanceBps = relativeDistanceBps(matchPrice, caseRec.OpenTakeProfit)

	summary := fmt.Sprintf(
		"Matched filled %s trigger order at %.8f after the stored stop moved favorably from %.8f; classifying the exit as trailing stop.",
		orderTypeLabel,
		matchPrice,
		caseRec.OpenStopLoss,
	)
	reason := "trailing_stop"
	if !stopPriceCountsAsProfitProtection(caseRec, matchPrice) {
		reason = "stop_loss"
		switch {
		case stopPriceOnLossSide(caseRec, matchPrice):
			summary += " The tightened stop was still on the loss side of entry, so the exit is classified as stop loss."
		case caseRec != nil && caseRec.RealizedPnLPct < 0:
			summary += " Net realized PnL still finished negative, so the exit is classified as stop loss."
		}
	}

	return dealReviewCloseInferenceResult{
		Reason:              reason,
		InferredBy:          "matched synced close fill / order",
		ExitOrigin:          DealReviewExitOriginTrailingEngine,
		ExitReasonQuality:   DealReviewExitReasonQualityHighConfidence,
		ExitEvidenceSummary: summary,
		ExitEvidence:        evidence,
	}
}

func inferCloseReasonFromGenericStopOrder(caseRec *DealReviewCase, ctx *dealReviewExitExecutionContext, orderTypeLabel string) dealReviewCloseInferenceResult {
	matchPrice := resolveDealReviewStopMatchPrice(ctx)
	if caseRec == nil || ctx == nil || ctx.Order == nil || matchPrice <= 0 {
		return dealReviewCloseInferenceResult{}
	}

	evidence := buildDealReviewOrderExitEvidence(ctx.Order, ctx.Fill, "matched_generic_stop_heuristic")
	evidence.TargetStopLoss = caseRec.OpenStopLoss
	evidence.TargetTakeProfit = caseRec.OpenTakeProfit
	evidence.StopLossDistanceBps = relativeDistanceBps(matchPrice, caseRec.OpenStopLoss)
	evidence.TakeProfitDistanceBps = relativeDistanceBps(matchPrice, caseRec.OpenTakeProfit)

	switch {
	case stopPriceOnLossSide(caseRec, matchPrice):
		summary := fmt.Sprintf(
			"Matched generic %s trigger order at %.8f on the loss side of entry; classifying the exit as stop loss.",
			orderTypeLabel,
			matchPrice,
		)
		if caseRec.OpenStopLoss > 0 {
			summary += fmt.Sprintf(" Stored initial stop was %.8f.", caseRec.OpenStopLoss)
		}
		return dealReviewCloseInferenceResult{
			Reason:              "stop_loss",
			InferredBy:          "matched synced close fill / order",
			ExitOrigin:          DealReviewExitOriginSyncedTriggerOrder,
			ExitReasonQuality:   DealReviewExitReasonQualityHighConfidence,
			ExitEvidenceSummary: summary,
			ExitEvidence:        evidence,
		}
	case stopPriceProtectsProfit(caseRec, matchPrice):
		reason := "trailing_stop"
		summary := fmt.Sprintf("Matched generic %s trigger order at %.8f on the profit side of entry; classifying the exit as trailing protection.", orderTypeLabel, matchPrice)
		if !stopPriceCountsAsProfitProtection(caseRec, matchPrice) {
			reason = "stop_loss"
			summary = fmt.Sprintf("Matched generic %s trigger order at %.8f on the profit side of entry, but net realized PnL still finished negative; classifying the exit as stop loss.", orderTypeLabel, matchPrice)
		}
		return dealReviewCloseInferenceResult{
			Reason:              reason,
			InferredBy:          "matched synced close fill / order",
			ExitOrigin:          DealReviewExitOriginTrailingEngine,
			ExitReasonQuality:   DealReviewExitReasonQualityHighConfidence,
			ExitEvidenceSummary: summary,
			ExitEvidence:        evidence,
		}
	default:
		return dealReviewCloseInferenceResult{}
	}
}

func isMovedTrailingStopPrice(caseRec *DealReviewCase, stopPrice float64) bool {
	if caseRec == nil || stopPrice <= 0 || caseRec.OpenStopLoss <= 0 || caseRec.EntryPrice <= 0 {
		return false
	}

	switch normalizeDealReviewSide(caseRec.Side) {
	case "LONG":
		return stopPrice > caseRec.OpenStopLoss &&
			!nearlyEqualRelative(stopPrice, caseRec.OpenStopLoss, 0.001)
	case "SHORT":
		return stopPrice < caseRec.OpenStopLoss &&
			!nearlyEqualRelative(stopPrice, caseRec.OpenStopLoss, 0.001)
	default:
		return false
	}
}

func resolveDealReviewStopMatchPrice(ctx *dealReviewExitExecutionContext) float64 {
	if ctx == nil {
		return 0
	}
	if ctx.Order != nil {
		if ctx.Order.StopPrice > 0 {
			return ctx.Order.StopPrice
		}
		if ctx.Order.AvgFillPrice > 0 {
			return ctx.Order.AvgFillPrice
		}
	}
	if ctx.Fill != nil && ctx.Fill.Price > 0 {
		return ctx.Fill.Price
	}
	return 0
}

func stopPriceOnLossSide(caseRec *DealReviewCase, stopPrice float64) bool {
	if caseRec == nil || stopPrice <= 0 || caseRec.EntryPrice <= 0 {
		return false
	}
	switch normalizeDealReviewSide(caseRec.Side) {
	case "LONG":
		return stopPrice <= caseRec.EntryPrice
	case "SHORT":
		return stopPrice >= caseRec.EntryPrice
	default:
		return false
	}
}

func stopPriceProtectsProfit(caseRec *DealReviewCase, stopPrice float64) bool {
	if caseRec == nil || stopPrice <= 0 || caseRec.EntryPrice <= 0 {
		return false
	}
	switch normalizeDealReviewSide(caseRec.Side) {
	case "LONG":
		return stopPrice > caseRec.EntryPrice
	case "SHORT":
		return stopPrice < caseRec.EntryPrice
	default:
		return false
	}
}

func stopPriceCountsAsProfitProtection(caseRec *DealReviewCase, stopPrice float64) bool {
	if caseRec == nil || caseRec.RealizedPnLPct < 0 {
		return false
	}
	return stopPriceProtectsProfit(caseRec, stopPrice)
}

func buildDealReviewOrderExitEvidence(order *TraderOrder, fill *TraderFill, matchedBy string) *DealReviewExitEvidence {
	evidence := &DealReviewExitEvidence{
		MatchedBy: matchedBy,
	}
	if order != nil {
		evidence.OrderType = strings.TrimSpace(order.Type)
		evidence.VenueOrderType = strings.TrimSpace(order.VenueOrderType)
		evidence.TriggerSubtype = strings.TrimSpace(order.TriggerSubtype)
		evidence.TriggerSource = strings.TrimSpace(order.TriggerSource)
		evidence.OrderAction = strings.TrimSpace(order.OrderAction)
		evidence.OrderStatus = strings.TrimSpace(order.Status)
		evidence.ClientOrderID = strings.TrimSpace(order.ClientOrderID)
		evidence.ExchangeOrderID = strings.TrimSpace(order.ExchangeOrderID)
		evidence.TriggerPrice = order.StopPrice
		evidence.OrderPrice = order.Price
		evidence.ReduceOnly = order.ReduceOnly
		evidence.ClosePosition = order.ClosePosition
		if order.AvgFillPrice > 0 {
			evidence.FillPrice = order.AvgFillPrice
		}
		if order.FilledQuantity > 0 {
			evidence.FillQuantity = order.FilledQuantity
		}
	}
	if fill != nil {
		if evidence.ExchangeOrderID == "" {
			evidence.ExchangeOrderID = strings.TrimSpace(fill.ExchangeOrderID)
		}
		if fill.Price > 0 {
			evidence.FillPrice = fill.Price
		}
		if fill.Quantity > 0 {
			evidence.FillQuantity = fill.Quantity
		}
	}
	return evidence
}

func nearlyEqualRelative(left, right, tolerance float64) bool {
	if left <= 0 || right <= 0 {
		return false
	}
	diff := math.Abs(left - right)
	base := math.Max(math.Abs(left), math.Abs(right))
	if base == 0 {
		return false
	}
	return diff/base <= tolerance
}

func relativeDistanceBps(left, right float64) float64 {
	if left <= 0 || right <= 0 {
		return 0
	}
	return math.Abs(left-right) / math.Max(math.Abs(left), math.Abs(right)) * 10000
}

func normalizeReasoningHint(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", " ", "-", " ", ",", " ", ";", " ", ".", " ", ":", " ", "(", " ", ")", " ")
	return replacer.Replace(normalized)
}

func dealReviewOrderTypeHint(order *TraderOrder) string {
	if order == nil {
		return ""
	}
	parts := []string{
		strings.TrimSpace(order.Type),
		strings.TrimSpace(order.VenueOrderType),
		strings.TrimSpace(order.TriggerSubtype),
		strings.TrimSpace(order.TriggerSource),
		strings.TrimSpace(order.ClientOrderID),
	}
	return normalizeReasoningHint(strings.Join(parts, " "))
}

func classifyDealReviewTriggerOrderSubtype(order *TraderOrder) string {
	typeHint := dealReviewOrderTypeHint(order)
	switch {
	case strings.Contains(typeHint, "trailing stop"), strings.Contains(typeHint, "trailingstop"):
		return "trailing_stop"
	case strings.Contains(typeHint, "take profit"), strings.Contains(typeHint, "takeprofit"), strings.Contains(typeHint, "partialtakeprofit"):
		return "take_profit"
	case strings.Contains(typeHint, "stop loss"), strings.Contains(typeHint, "stoploss"):
		return "stop_loss"
	default:
		return ""
	}
}

func buildExchangeSyncUnknownInference(ctx *dealReviewExitExecutionContext, matchedBy string) dealReviewCloseInferenceResult {
	summary := "Position closed via synced exchange event, but no linked order, fill, trigger subtype, target match, or internal exit intent could identify the exit condition."
	if ctx != nil {
		switch {
		case ctx.Order != nil && ctx.Fill != nil:
			summary = "Matched synced close order/fill, but the exchange data still does not reveal whether the exit was stop loss, take profit, trailing protection, or a manual close."
		case ctx.Order != nil:
			summary = "Matched synced close order, but the exchange data still does not reveal the actual exit condition."
		case ctx.Fill != nil:
			summary = "Matched synced close fill, but there is no typed trigger order or internal exit intent linked to it."
		}
	}
	return dealReviewCloseInferenceResult{
		Reason:              "unknown",
		InferredBy:          "synced exchange event without decisive exit evidence",
		ExitOrigin:          DealReviewExitOriginExchangeSyncUnknown,
		ExitReasonQuality:   DealReviewExitReasonQualityLowConfidence,
		ExitEvidenceSummary: summary,
		ExitEvidence:        buildDealReviewOrderExitEvidence(nilIfNoOrder(ctx), nilIfNoFill(ctx), matchedBy),
	}
}

func nilIfNoOrder(ctx *dealReviewExitExecutionContext) *TraderOrder {
	if ctx == nil {
		return nil
	}
	return ctx.Order
}

func nilIfNoFill(ctx *dealReviewExitExecutionContext) *TraderFill {
	if ctx == nil {
		return nil
	}
	return ctx.Fill
}

func shouldReplaceSyntheticCloseReasoning(reasoning string) bool {
	trimmed := strings.TrimSpace(reasoning)
	return trimmed == "" || strings.HasPrefix(trimmed, "Position closed via synced exchange event")
}

func buildStoredCloseInference(currentReason string, event *DealReviewEvent, ctx *dealReviewExitExecutionContext) dealReviewCloseInferenceResult {
	if event != nil && event.Source == DealReviewEventSourceAIDecision && event.DecisionCycleNumber > 0 && normalizeCloseReason(currentReason) == "ai_exit" {
		return dealReviewCloseInferenceResult{
			Reason:              currentReason,
			InferredBy:          "stored close reason from matched AI close decision",
			ExitOrigin:          DealReviewExitOriginAIDecision,
			ExitReasonQuality:   DealReviewExitReasonQualityExplicit,
			ExitEvidenceSummary: fmt.Sprintf("Using stored close reason from AI close decision cycle %d.", event.DecisionCycleNumber),
			ExitEvidence: &DealReviewExitEvidence{
				MatchedBy:           "stored_close_reason",
				DecisionCycleNumber: event.DecisionCycleNumber,
				DecisionAction:      strings.TrimSpace(event.Action),
			},
		}
	}

	if normalized := normalizeCloseReason(currentReason); normalized == "" || normalized == "unknown" || normalized == "sync" || normalized == "manual_exit" {
		return buildExchangeSyncUnknownInference(ctx, "stored_close_reason_fallback")
	}

	result := dealReviewCloseInferenceResult{
		Reason:              currentReason,
		ExitOrigin:          DealReviewExitOriginStoredPositionReason,
		ExitReasonQuality:   DealReviewExitReasonQualityLowConfidence,
		ExitEvidenceSummary: "Using the stored position close reason because no stronger linked exit evidence was found yet.",
	}
	if ctx != nil {
		if ctx.Order != nil || ctx.Fill != nil {
			result.ExitEvidenceSummary = "Using the stored position close reason; a synced exit order/fill exists but does not identify the trigger type clearly."
			result.ExitEvidence = buildDealReviewOrderExitEvidence(ctx.Order, ctx.Fill, "stored_close_reason_fallback")
		}
	}
	return result
}

func applyDealReviewCloseInferenceToCase(caseRec *DealReviewCase, inference dealReviewCloseInferenceResult) {
	if caseRec == nil {
		return
	}
	caseRec.ExitOrigin = strings.TrimSpace(inference.ExitOrigin)
	caseRec.ExitReasonQuality = strings.TrimSpace(inference.ExitReasonQuality)
	caseRec.ExitEvidenceSummary = strings.TrimSpace(inference.ExitEvidenceSummary)
	caseRec.ExitEvidence = inference.ExitEvidence
	caseRec.ExitEvidenceJSON = marshalDealReviewExitEvidence(inference.ExitEvidence)
}

func applyDealReviewCloseInferenceToEvent(event *DealReviewEvent, inference dealReviewCloseInferenceResult) {
	if event == nil {
		return
	}
	event.ExitOrigin = strings.TrimSpace(inference.ExitOrigin)
	event.ExitReasonQuality = strings.TrimSpace(inference.ExitReasonQuality)
	event.ExitEvidenceSummary = strings.TrimSpace(inference.ExitEvidenceSummary)
	event.ExitEvidence = inference.ExitEvidence
	event.ExitEvidenceJSON = marshalDealReviewExitEvidence(inference.ExitEvidence)
}

func marshalDealReviewExitEvidence(evidence *DealReviewExitEvidence) string {
	if evidence == nil {
		return "{}"
	}
	raw, err := json.Marshal(evidence)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return "{}"
	}
	return string(raw)
}

func parseDealReviewExitEvidence(raw string) *DealReviewExitEvidence {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return nil
	}
	var evidence DealReviewExitEvidence
	if err := json.Unmarshal([]byte(trimmed), &evidence); err != nil {
		return nil
	}
	return &evidence
}

func hydrateDealReviewCaseJSONFields(caseRec *DealReviewCase) {
	if caseRec == nil {
		return
	}
	caseRec.ExitEvidence = parseDealReviewExitEvidence(caseRec.ExitEvidenceJSON)
}

func hydrateDealReviewEventJSONFields(event *DealReviewEvent) {
	if event == nil {
		return
	}
	event.ExitEvidence = parseDealReviewExitEvidence(event.ExitEvidenceJSON)
}

func formatDealReviewTargetName(reason string) string {
	switch normalizeCloseReason(reason) {
	case "stop_loss":
		return "stop loss"
	case "take_profit":
		return "take profit"
	case "trailing_stop":
		return "trailing stop"
	default:
		return strings.ReplaceAll(normalizeCloseReason(reason), "_", " ")
	}
}

func humanizeDealReviewExitOrigin(origin string) string {
	switch strings.TrimSpace(origin) {
	case DealReviewExitOriginAIDecision:
		return "AI decision"
	case DealReviewExitOriginSyncedTriggerOrder:
		return "synced trigger order"
	case DealReviewExitOriginSyncedMarketOrder:
		return "synced market order"
	case DealReviewExitOriginSyncedCloseFill:
		return "synced close fill"
	case DealReviewExitOriginTargetProximity:
		return "target proximity"
	case DealReviewExitOriginTrailingEngine:
		return "trailing engine"
	case DealReviewExitOriginManualUIClose:
		return "manual UI close"
	case DealReviewExitOriginRiskGuard:
		return "risk guard"
	case DealReviewExitOriginInternalExitIntent:
		return "internal exit intent"
	case DealReviewExitOriginStoredPositionReason:
		return "stored position reason"
	case DealReviewExitOriginExchangeSyncUnknown:
		return "synced exchange event"
	default:
		return "synced exchange event"
	}
}

func buildInferredCloseReasoning(inference dealReviewCloseInferenceResult) string {
	reason := normalizeCloseReason(inference.Reason)
	if reason == "" {
		return ""
	}
	base := fmt.Sprintf("Position closed via %s (%s).", humanizeDealReviewExitOrigin(inference.ExitOrigin), reason)
	if quality := strings.TrimSpace(inference.ExitReasonQuality); quality != "" {
		base = strings.TrimSuffix(base, ".") + fmt.Sprintf(" Confidence: %s.", quality)
	}
	if summary := strings.TrimSpace(inference.ExitEvidenceSummary); summary != "" {
		return base + " Evidence: " + strings.TrimSpace(summary)
	}
	if detail := strings.TrimSpace(inference.InferredBy); detail != "" {
		return strings.TrimSuffix(base, ".") + fmt.Sprintf(" Evidence: inferred from %s.", detail)
	}
	return base
}

func classifyDealOutcome(realizedPnL float64) string {
	switch {
	case realizedPnL > 0:
		return "profit"
	case realizedPnL < 0:
		return "loss"
	default:
		return "flat"
	}
}

func calculateDealPnLPct(entryPrice, quantity float64, leverage int, realizedPnL float64) float64 {
	if entryPrice <= 0 || quantity <= 0 {
		return 0
	}
	denominator := entryPrice * quantity
	if leverage > 1 {
		denominator = denominator / float64(leverage)
	}
	if denominator == 0 {
		return 0
	}
	return realizedPnL / denominator * 100
}
