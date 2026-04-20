package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"nofx/logger"
	"nofx/mcp"
	"nofx/store"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	autonomousOptimizerStaleRunTimeout        = 20 * time.Minute
	autonomousOptimizerRecoveredRunRetryDelay = 1 * time.Minute
	autonomousOptimizerRollbackPendingDelay   = 1 * time.Minute
)

type autonomousOptimizerRunMetadata struct {
	WindowStartMs             int64                                         `json:"window_start_ms"`
	WindowEndMs               int64                                         `json:"window_end_ms"`
	ReviewIntervalHours       int                                           `json:"review_interval_hours"`
	ClosedDeals               int64                                         `json:"closed_deals"`
	OpenDeals                 int64                                         `json:"open_deals"`
	WinningDeals              int64                                         `json:"winning_deals"`
	LosingDeals               int64                                         `json:"losing_deals"`
	NetPnL                    float64                                       `json:"net_pnl"`
	AvgPnL                    float64                                       `json:"avg_pnl"`
	Expectancy                float64                                       `json:"expectancy"`
	WinRate                   float64                                       `json:"win_rate"`
	ProfitFactor              float64                                       `json:"profit_factor"`
	MaxDrawdownPct            float64                                       `json:"max_drawdown_pct"`
	DecisionRecordCount       int                                           `json:"decision_record_count"`
	DecisionCandidateCount    int                                           `json:"decision_candidate_count"`
	OpenDecisionCount         int                                           `json:"open_decision_count"`
	RejectedCandidateCount    int                                           `json:"rejected_candidate_count"`
	CyclesWithCandidates      int                                           `json:"cycles_with_candidates"`
	CyclesWithOpenDecisions   int                                           `json:"cycles_with_open_decisions"`
	HoldDecisionCount         int                                           `json:"hold_decision_count"`
	WaitDecisionCount         int                                           `json:"wait_decision_count"`
	DecisionConversionRate    float64                                       `json:"decision_conversion_rate"`
	AvgDecisionConfidence     float64                                       `json:"avg_decision_confidence"`
	AvgMFECapturedPct         float64                                       `json:"avg_mfe_captured_pct"`
	AvgProfitGivenBackPct     float64                                       `json:"avg_profit_given_back_pct"`
	AvgExitEfficiencyScore    float64                                       `json:"avg_exit_efficiency_score"`
	AvgEntryTimingScore       float64                                       `json:"avg_entry_timing_score"`
	AvgRiskSizingScore        float64                                       `json:"avg_risk_sizing_score"`
	BadEntryDeals             int64                                         `json:"bad_entry_deals"`
	BadExitDeals              int64                                         `json:"bad_exit_deals"`
	AvoidableLossDeals        int64                                         `json:"avoidable_loss_deals"`
	StrongEntryWeakExitDeals  int64                                         `json:"strong_entry_weak_exit_deals"`
	RejectReasons             []store.TraderRejectReason                    `json:"reject_reasons,omitempty"`
	ConfidenceBands           []store.TraderConfidenceBand                  `json:"confidence_bands,omitempty"`
	OpportunitySessions       []store.TraderSessionSummary                  `json:"opportunity_sessions,omitempty"`
	OpportunitySymbols        []store.TraderSymbolSummary                   `json:"opportunity_symbols,omitempty"`
	ExecutionStatuses         []store.TraderExecutionStatus                 `json:"execution_statuses,omitempty"`
	RecentOpenExecutions      []store.TraderOpenExecution                   `json:"recent_open_executions,omitempty"`
	RegimeSummaries           []store.TraderRegimeSummary                   `json:"regime_summaries,omitempty"`
	TrailingStopTelemetry     *autonomousOptimizerTrailingStopTelemetry     `json:"trailing_stop_telemetry,omitempty"`
	AdaptiveCooldownTelemetry *autonomousOptimizerAdaptiveCooldownTelemetry `json:"adaptive_cooldown_telemetry,omitempty"`
	SelfPaused                bool                                          `json:"self_paused,omitempty"`
}

type autonomousOptimizerTrailingStopTelemetry struct {
	TrailingExitCount                int                                                       `json:"trailing_exit_count"`
	TrailingProfitExitCount          int                                                       `json:"trailing_profit_exit_count"`
	TrailingLossExitCount            int                                                       `json:"trailing_loss_exit_count"`
	InitialStopLossCount             int                                                       `json:"initial_stop_loss_count"`
	TrailingExitAvgPnLPct            float64                                                   `json:"trailing_exit_avg_pnl_pct"`
	InitialStopLossAvgPnLPct         float64                                                   `json:"initial_stop_loss_avg_pnl_pct"`
	AvgMinutesToFirstUpdate          float64                                                   `json:"avg_minutes_to_first_update"`
	AvgMinutesFromFirstUpdateToExit  float64                                                   `json:"avg_minutes_from_first_update_to_exit"`
	EarlyTighteningCount             int                                                       `json:"early_tightening_count"`
	EarlyTighteningLossCount         int                                                       `json:"early_tightening_loss_count"`
	EarlyTighteningBelowEntryCount   int                                                       `json:"early_tightening_below_entry_count"`
	EarlyTighteningProtectedCount    int                                                       `json:"early_tightening_protected_count"`
	OneCycleExitCount                int                                                       `json:"one_cycle_exit_count"`
	EarlyTighteningOneCycleExitCount int                                                       `json:"early_tightening_one_cycle_exit_count"`
	OneCycleExitThresholdSec         int                                                       `json:"one_cycle_exit_threshold_sec"`
	FirstUpdateAuditCount            int                                                       `json:"first_update_audit_count"`
	BreakevenProtectedCount          int                                                       `json:"breakeven_protected_count"`
	TierBreakdown                    []autonomousOptimizerTrailingStopTierBreakdown            `json:"tier_breakdown,omitempty"`
	ProfitBandBreakdown              []autonomousOptimizerTrailingStopProfitBandBreakdown      `json:"profit_band_breakdown,omitempty"`
	EntryProtectionBreakdown         []autonomousOptimizerTrailingStopEntryProtectionBreakdown `json:"entry_protection_breakdown,omitempty"`
	SampleUpdates                    []autonomousOptimizerTrailingStopUpdateAuditItem          `json:"sample_updates,omitempty"`
}

type autonomousOptimizerTrailingStopTierBreakdown struct {
	TierTriggerProfitPct         float64 `json:"tier_trigger_profit_pct,omitempty"`
	TrailingMode                 string  `json:"trailing_mode,omitempty"`
	AuditCount                   int     `json:"audit_count"`
	EarlyTighteningCount         int     `json:"early_tightening_count"`
	EarlyTighteningLossCount     int     `json:"early_tightening_loss_count"`
	LossExitCount                int     `json:"loss_exit_count"`
	ProfitExitCount              int     `json:"profit_exit_count"`
	BelowEntryCount              int     `json:"below_entry_count"`
	BreakevenOrBetterCount       int     `json:"breakeven_or_better_count"`
	OneCycleExitCount            int     `json:"one_cycle_exit_count"`
	AvgPreUpdateUnrealizedPnLPct float64 `json:"avg_pre_update_unrealized_pnl_pct,omitempty"`
}

type autonomousOptimizerTrailingStopProfitBandBreakdown struct {
	ProfitBand               string  `json:"profit_band"`
	AuditCount               int     `json:"audit_count"`
	EarlyTighteningCount     int     `json:"early_tightening_count"`
	EarlyTighteningLossCount int     `json:"early_tightening_loss_count"`
	LossExitCount            int     `json:"loss_exit_count"`
	ProfitExitCount          int     `json:"profit_exit_count"`
	OneCycleExitCount        int     `json:"one_cycle_exit_count"`
	AvgMinutesToFirstUpdate  float64 `json:"avg_minutes_to_first_update,omitempty"`
}

type autonomousOptimizerTrailingStopEntryProtectionBreakdown struct {
	EntryProtectionState     string `json:"entry_protection_state"`
	AuditCount               int    `json:"audit_count"`
	EarlyTighteningCount     int    `json:"early_tightening_count"`
	EarlyTighteningLossCount int    `json:"early_tightening_loss_count"`
	LossExitCount            int    `json:"loss_exit_count"`
	ProfitExitCount          int    `json:"profit_exit_count"`
	OneCycleExitCount        int    `json:"one_cycle_exit_count"`
}

type autonomousOptimizerTrailingStopUpdateAuditItem struct {
	PositionID                int64   `json:"position_id,omitempty"`
	Symbol                    string  `json:"symbol,omitempty"`
	Side                      string  `json:"side,omitempty"`
	CloseReason               string  `json:"close_reason,omitempty"`
	UpdateSource              string  `json:"update_source,omitempty"`
	TrailingMode              string  `json:"trailing_mode,omitempty"`
	UpdateTimeMs              int64   `json:"update_time_ms,omitempty"`
	MinutesToFirstUpdate      float64 `json:"minutes_to_first_update,omitempty"`
	MinutesFromUpdateToExit   float64 `json:"minutes_from_update_to_exit,omitempty"`
	PreUpdateUnrealizedPnL    float64 `json:"pre_update_unrealized_pnl,omitempty"`
	PreUpdateUnrealizedPnLPct float64 `json:"pre_update_unrealized_pnl_pct,omitempty"`
	PreUpdateProfitBand       string  `json:"pre_update_profit_band,omitempty"`
	StopProfitPct             float64 `json:"stop_profit_pct,omitempty"`
	ProtectsBreakeven         bool    `json:"protects_breakeven,omitempty"`
	EntryProtectionState      string  `json:"entry_protection_state,omitempty"`
	ExitWithinOneCycle        bool    `json:"exit_within_one_cycle,omitempty"`
	RealizedPnLPct            float64 `json:"realized_pnl_pct,omitempty"`
	PreviousStopPrice         float64 `json:"previous_stop_price,omitempty"`
	NewStopPrice              float64 `json:"new_stop_price,omitempty"`
	TierTriggerProfitPct      float64 `json:"tier_trigger_profit_pct,omitempty"`
}

type autonomousOptimizerAdaptiveCooldownConfigSnapshot struct {
	Enabled                       bool `json:"enabled"`
	RequireWeakExecutionRegime    bool `json:"require_weak_execution_regime"`
	RecentTradeWindow             int  `json:"recent_trade_window"`
	MinRecentTrades               int  `json:"min_recent_trades"`
	SameSymbolLossCooldownMinutes int  `json:"same_symbol_loss_cooldown_minutes"`
	PairLossLookbackHours         int  `json:"pair_loss_lookback_hours"`
}

type autonomousOptimizerCooldownOutcomeSummary struct {
	TradeCount int     `json:"trade_count"`
	WinCount   int     `json:"win_count"`
	LossCount  int     `json:"loss_count"`
	NetPnLPct  float64 `json:"net_pnl_pct"`
	AvgPnLPct  float64 `json:"avg_pnl_pct"`
}

type autonomousOptimizerAdaptiveCooldownSymbol struct {
	Symbol               string                                    `json:"symbol"`
	ReentryCount         int                                       `json:"reentry_count"`
	RepeatAfterLossCount int                                       `json:"repeat_after_loss_count"`
	AvgPnLPct            float64                                   `json:"avg_pnl_pct"`
	LastGapMinutes       float64                                   `json:"last_gap_minutes,omitempty"`
	BlockedOutcomes      autonomousOptimizerCooldownOutcomeSummary `json:"blocked_outcomes"`
	PostCooldownOutcomes autonomousOptimizerCooldownOutcomeSummary `json:"post_cooldown_outcomes"`
}

type autonomousOptimizerAdaptiveCooldownRegime struct {
	TrendRegime          string                                    `json:"trend_regime,omitempty"`
	VolatilityRegime     string                                    `json:"volatility_regime,omitempty"`
	OIRegime             string                                    `json:"oi_regime,omitempty"`
	ReentryCount         int                                       `json:"reentry_count"`
	RepeatAfterLossCount int                                       `json:"repeat_after_loss_count"`
	AvgPnLPct            float64                                   `json:"avg_pnl_pct"`
	BlockedOutcomes      autonomousOptimizerCooldownOutcomeSummary `json:"blocked_outcomes"`
	PostCooldownOutcomes autonomousOptimizerCooldownOutcomeSummary `json:"post_cooldown_outcomes"`
}

type autonomousOptimizerAdaptiveCooldownTelemetry struct {
	Config                       autonomousOptimizerAdaptiveCooldownConfigSnapshot `json:"config"`
	CooldownCandidateCount       int                                               `json:"cooldown_candidate_count"`
	SameSessionReentryCount      int                                               `json:"same_session_reentry_count"`
	RepeatAfterLossCount         int                                               `json:"repeat_after_loss_count"`
	RegimeRepeatLossCount        int                                               `json:"regime_repeat_loss_count"`
	BlockedSymbolReentryOutcomes autonomousOptimizerCooldownOutcomeSummary         `json:"blocked_symbol_reentry_outcomes"`
	PostSymbolCooldownOutcomes   autonomousOptimizerCooldownOutcomeSummary         `json:"post_symbol_cooldown_outcomes"`
	BlockedRegimeReentryOutcomes autonomousOptimizerCooldownOutcomeSummary         `json:"blocked_regime_reentry_outcomes"`
	PostRegimeCooldownOutcomes   autonomousOptimizerCooldownOutcomeSummary         `json:"post_regime_cooldown_outcomes"`
	TopSymbols                   []autonomousOptimizerAdaptiveCooldownSymbol       `json:"top_symbols,omitempty"`
	TopRegimes                   []autonomousOptimizerAdaptiveCooldownRegime       `json:"top_regimes,omitempty"`
}

type autonomousOptimizerMonitoringSnapshot struct {
	ClosedDeals            int64   `json:"closed_deals"`
	WinningDeals           int64   `json:"winning_deals"`
	LosingDeals            int64   `json:"losing_deals"`
	NetPnL                 float64 `json:"net_pnl"`
	AvgPnL                 float64 `json:"avg_pnl"`
	WinRate                float64 `json:"win_rate"`
	Expectancy             float64 `json:"expectancy"`
	ProfitFactor           float64 `json:"profit_factor"`
	MaxDrawdownPct         float64 `json:"max_drawdown_pct"`
	DecisionRecordCount    int     `json:"decision_record_count"`
	DecisionCandidateCount int     `json:"decision_candidate_count"`
	OpenDecisionCount      int     `json:"open_decision_count"`
	HoldDecisionCount      int     `json:"hold_decision_count"`
	WaitDecisionCount      int     `json:"wait_decision_count"`
	DecisionConversionRate float64 `json:"decision_conversion_rate"`
	AvgDecisionConfidence  float64 `json:"avg_decision_confidence"`
	AvgMFECapturedPct      float64 `json:"avg_mfe_captured_pct"`
	AvgProfitGivenBackPct  float64 `json:"avg_profit_given_back_pct"`
	AvgExitEfficiencyScore float64 `json:"avg_exit_efficiency_score"`
	AvgEntryTimingScore    float64 `json:"avg_entry_timing_score"`
	AvgRiskSizingScore     float64 `json:"avg_risk_sizing_score"`
	BadEntryDeals          int64   `json:"bad_entry_deals"`
	BadExitDeals           int64   `json:"bad_exit_deals"`
	AvoidableLossDeals     int64   `json:"avoidable_loss_deals"`
}

type autonomousOptimizerMonitoringChainStats struct {
	RootRunID        string  `json:"root_run_id,omitempty"`
	WindowsObserved  int     `json:"windows_observed"`
	NegativeWindows  int     `json:"negative_windows"`
	TotalClosedDeals int64   `json:"total_closed_deals"`
	TotalNetPnL      float64 `json:"total_net_pnl"`
}

type autonomousOptimizerRollbackAssessment struct {
	ShouldRollback          bool                                    `json:"should_rollback"`
	HasEnoughEvidence       bool                                    `json:"has_enough_evidence"`
	TriggerCodes            []string                                `json:"trigger_codes,omitempty"`
	TriggerReasons          []string                                `json:"trigger_reasons,omitempty"`
	CurrentSnapshot         autonomousOptimizerMonitoringSnapshot   `json:"current_snapshot"`
	SourceSnapshot          autonomousOptimizerMonitoringSnapshot   `json:"source_snapshot"`
	Chain                   autonomousOptimizerMonitoringChainStats `json:"chain"`
	MonitoringRootRunID     string                                  `json:"monitoring_root_run_id,omitempty"`
	ObservedClosedDeals     int64                                   `json:"observed_closed_deals"`
	ObservedNetPnL          float64                                 `json:"observed_net_pnl"`
	ObservedNegativeWindows int                                     `json:"observed_negative_windows"`
}

func (s *Server) RunAutonomousOptimizerSupervisor() {
	if err := s.ProcessPendingAutonomousOptimizerRuns(); err != nil {
		logger.Warnf("⚠️ Initial autonomous optimizer sweep failed: %v", err)
	}
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if err := s.ProcessPendingAutonomousOptimizerRuns(); err != nil {
			logger.Warnf("⚠️ Autonomous optimizer supervisor failed: %v", err)
		}
	}
}

func (s *Server) ProcessPendingAutonomousOptimizerRuns() error {
	now := time.Now().UTC()
	if err := s.recoverStaleAutonomousOptimizerState(now); err != nil {
		return err
	}
	configs, err := s.store.AutonomousOptimizer().ListDueConfigs(now, 25)
	if err != nil {
		return err
	}
	for i := range configs {
		cfg := configs[i]
		if err := s.processAutonomousOptimizerConfig(&cfg, now, store.AutonomousOptimizerRunTriggerSchedule); err != nil {
			logger.Warnf("⚠️ Failed to process autonomous optimizer config %s for trader %s: %v", cfg.ID, cfg.TraderID, err)
		}
	}
	return nil
}

func (s *Server) beginAutonomousOptimizerRun(cfg *store.AutonomousOptimizerConfig, now time.Time, trigger string) (*store.AutonomousOptimizerRun, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = store.AutonomousOptimizerRunTriggerSchedule
	}
	runID := uuid.NewString()
	cfg.Status = store.AutonomousOptimizerStatusRunning
	cfg.LastRunID = runID
	cfg.NextRunAt = now.Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)
	if err := s.store.AutonomousOptimizer().SaveConfig(cfg); err != nil {
		return nil, err
	}

	run := &store.AutonomousOptimizerRun{
		ID:                   runID,
		UserID:               cfg.UserID,
		TraderID:             cfg.TraderID,
		ConfigID:             cfg.ID,
		Trigger:              trigger,
		Status:               store.AutonomousOptimizerStatusRunning,
		PrimaryModelConfigID: cfg.PrimaryModelConfigID,
		PrimaryModelName:     cfg.PrimaryModelName,
		CriticModelConfigID:  cfg.CriticModelConfigID,
		CriticModelName:      cfg.CriticModelName,
		SourceTraderID:       cfg.SeedSourceTraderID,
		SourceStrategyID:     cfg.CurrentSeedStrategyID,
		StartedAt:            now,
	}
	if err := s.store.AutonomousOptimizer().SaveRun(run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Server) processAutonomousOptimizerConfig(cfg *store.AutonomousOptimizerConfig, now time.Time, trigger string) error {
	run, err := s.beginAutonomousOptimizerRun(cfg, now, trigger)
	if err != nil {
		return err
	}
	if run == nil {
		return nil
	}

	result, err := s.runAutonomousOptimizerCycle(cfg, run, now)
	if err != nil {
		run.Status = store.AutonomousOptimizerStatusFailed
		run.Summary = fmt.Sprintf("Optimizer window failed: %v", err)
		run.CompletedAt = time.Now().UTC()
		if raw, marshalErr := json.Marshal(map[string]any{"error": err.Error()}); marshalErr == nil {
			run.MetadataJSON = string(raw)
		}
		_ = s.store.AutonomousOptimizer().SaveRun(run)
		cfg.Status = store.AutonomousOptimizerStatusFailed
		cfg.LastRunID = run.ID
		cfg.LastRunAt = run.CompletedAt
		cfg.NextRunAt = run.CompletedAt.Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)
		_ = s.store.AutonomousOptimizer().SaveConfig(cfg)
		return err
	}

	run.Status = result.RunStatus
	run.Summary = result.Summary
	run.AppliedStrategyVersionID = result.AppliedStrategyVersionID
	run.CompletedAt = time.Now().UTC()
	if raw, marshalErr := json.Marshal(sanitizeAutonomousOptimizerJSONMap(result.ConfigPatch)); marshalErr == nil {
		run.ConfigPatchJSON = string(raw)
	}
	if raw, marshalErr := json.Marshal(sanitizeAutonomousOptimizerJSONMap(result.PromptPatch)); marshalErr == nil {
		run.PromptPatchJSON = string(raw)
	}
	if raw, marshalErr := json.Marshal(result.Validation); marshalErr == nil {
		run.ValidationJSON = string(raw)
	}
	if raw, marshalErr := json.Marshal(result.Metadata); marshalErr == nil {
		run.MetadataJSON = string(raw)
	}
	if err := s.store.AutonomousOptimizer().SaveRun(run); err != nil {
		return err
	}

	cfg.LastRunID = run.ID
	cfg.LastRunAt = run.CompletedAt
	cfg.NextRunAt = run.CompletedAt.Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)
	if !result.NextRunAt.IsZero() && result.NextRunAt.After(run.CompletedAt) {
		cfg.NextRunAt = result.NextRunAt
	}
	cfg.Status = result.ConfigStatus
	if strings.TrimSpace(cfg.Status) == "" {
		cfg.Status = result.RunStatus
	}

	if cfg.SelfPauseEnabled && shouldSelfPauseAutonomousOptimizer(run.Status, cfg.UserID, cfg.TraderID, run.ID, s.store) {
		cfg.Status = store.AutonomousOptimizerStatusPaused
		run.Status = store.AutonomousOptimizerStatusPaused
		run.Summary = strings.TrimSpace(run.Summary + " Optimizer paused itself after repeated low-evidence or failed windows.")
		metadata := parseAutonomousOptimizerJSONObject(run.MetadataJSON)
		metadata["self_paused"] = true
		if raw, marshalErr := json.Marshal(metadata); marshalErr == nil {
			run.MetadataJSON = string(raw)
		}
		if err := s.store.AutonomousOptimizer().SaveRun(run); err != nil {
			return err
		}
	}

	return s.store.AutonomousOptimizer().SaveConfig(cfg)
}

func (s *Server) recoverStaleAutonomousOptimizerState(now time.Time) error {
	cutoff := now.Add(-autonomousOptimizerStaleRunTimeout)
	recoveredConfigs := map[string]struct{}{}

	staleRuns, err := s.store.AutonomousOptimizer().ListStaleRunningRuns(cutoff, 25)
	if err != nil {
		return err
	}
	for _, run := range staleRuns {
		if run == nil {
			continue
		}
		if err := s.recoverStaleAutonomousOptimizerRun(run, now); err != nil {
			logger.Warnf("⚠️ Failed to recover stale autonomous optimizer run %s for trader %s: %v", run.ID, run.TraderID, err)
			continue
		}
		recoveredConfigs[run.UserID+"::"+run.TraderID] = struct{}{}
	}

	runningConfigs, err := s.store.AutonomousOptimizer().ListConfigsByStatus(store.AutonomousOptimizerStatusRunning, 25)
	if err != nil {
		return err
	}
	for i := range runningConfigs {
		cfg := runningConfigs[i]
		key := cfg.UserID + "::" + cfg.TraderID
		if _, alreadyRecovered := recoveredConfigs[key]; alreadyRecovered {
			continue
		}
		latestRunningRun, runErr := s.store.AutonomousOptimizer().GetLatestRunByStatus(cfg.UserID, cfg.TraderID, store.AutonomousOptimizerStatusRunning)
		switch {
		case runErr == nil && latestRunningRun != nil && latestRunningRun.StartedAt.After(cutoff):
			if cfg.NextRunAt.IsZero() {
				cfg.NextRunAt = now.Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)
				if err := s.store.AutonomousOptimizer().SaveConfig(&cfg); err != nil {
					logger.Warnf("⚠️ Failed to normalize next review window for active optimizer config %s: %v", cfg.ID, err)
				}
			}
			continue
		case runErr == nil && latestRunningRun != nil:
			if err := s.recoverStaleAutonomousOptimizerRun(latestRunningRun, now); err != nil {
				logger.Warnf("⚠️ Failed to recover late-discovered stale optimizer run %s for trader %s: %v", latestRunningRun.ID, latestRunningRun.TraderID, err)
			}
			continue
		case runErr != nil && !errors.Is(runErr, gorm.ErrRecordNotFound):
			logger.Warnf("⚠️ Failed to inspect running optimizer config %s for trader %s: %v", cfg.ID, cfg.TraderID, runErr)
			continue
		}
		if err := s.recoverOrphanAutonomousOptimizerConfig(&cfg, now); err != nil {
			logger.Warnf("⚠️ Failed to recover orphan running optimizer config %s for trader %s: %v", cfg.ID, cfg.TraderID, err)
		}
	}

	return nil
}

func (s *Server) recoverStaleAutonomousOptimizerRun(run *store.AutonomousOptimizerRun, now time.Time) error {
	if run == nil {
		return nil
	}
	run.Status = store.AutonomousOptimizerStatusFailed
	run.CompletedAt = now
	run.Summary = coalesceAutonomousOptimizerSummary(
		strings.TrimSpace(run.Summary),
		"Recovered stale optimizer run after timeout or restart before completion.",
	)
	metadata := parseAutonomousOptimizerJSONObject(run.MetadataJSON)
	metadata["recovered_stale_run"] = true
	metadata["recovery_reason"] = "run exceeded stale timeout without completion"
	metadata["recovered_at_ms"] = now.UnixMilli()
	metadata["stale_timeout_minutes"] = int(autonomousOptimizerStaleRunTimeout / time.Minute)
	metadata["original_status"] = store.AutonomousOptimizerStatusRunning
	if raw, err := json.Marshal(metadata); err == nil {
		run.MetadataJSON = string(raw)
	}
	if err := s.store.AutonomousOptimizer().SaveRun(run); err != nil {
		return err
	}

	cfg, err := s.store.AutonomousOptimizer().GetConfig(run.UserID, run.TraderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	cfg.Status = store.AutonomousOptimizerStatusFailed
	if !cfg.Enabled {
		cfg.Status = store.AutonomousOptimizerStatusPaused
	}
	cfg.LastRunID = run.ID
	cfg.LastRunAt = now
	cfg.NextRunAt = autonomousOptimizerRecoveredNextRunAt(cfg, run, now)
	return s.store.AutonomousOptimizer().SaveConfig(cfg)
}

func (s *Server) recoverOrphanAutonomousOptimizerConfig(cfg *store.AutonomousOptimizerConfig, now time.Time) error {
	if cfg == nil {
		return nil
	}
	if cfg.Enabled {
		cfg.Status = store.AutonomousOptimizerStatusFailed
		cfg.NextRunAt = autonomousOptimizerRecoveredNextRunAt(cfg, nil, now)
	} else {
		cfg.Status = store.AutonomousOptimizerStatusPaused
		cfg.NextRunAt = time.Time{}
	}
	return s.store.AutonomousOptimizer().SaveConfig(cfg)
}

func autonomousOptimizerRecoveredNextRunAt(cfg *store.AutonomousOptimizerConfig, run *store.AutonomousOptimizerRun, now time.Time) time.Time {
	nextRunAt := now.Add(autonomousOptimizerRecoveredRunRetryDelay)
	if cfg == nil {
		return nextRunAt
	}
	baseTime := cfg.LastRunAt.UTC()
	if run != nil && !run.StartedAt.IsZero() {
		baseTime = run.StartedAt.UTC()
	}
	scheduledFromWindow := baseTime.Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)
	if baseTime.IsZero() {
		scheduledFromWindow = now.Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)
	}
	if scheduledFromWindow.After(now) {
		return scheduledFromWindow
	}
	return nextRunAt
}

func (s *Server) runAutonomousOptimizerCycle(cfg *store.AutonomousOptimizerConfig, run *store.AutonomousOptimizerRun, now time.Time) (*autonomousOptimizerCycleResult, error) {
	bundle, err := s.collectAutonomousOptimizerWindowBundle(cfg, now)
	if err != nil {
		return nil, err
	}

	traderCfg, err := s.store.Trader().Get(cfg.UserID, cfg.TraderID)
	if err != nil {
		return nil, err
	}
	strategyCfg, strategyRecord, err := s.loadStrategyForTrader(cfg.UserID, traderCfg)
	if err != nil {
		return nil, err
	}

	if err := s.attachAutonomousOptimizerTelemetry(cfg, strategyCfg, bundle); err != nil {
		return nil, err
	}

	metadata := buildAutonomousOptimizerMetadataMap(bundle.Metadata)
	metadata["monitoring_snapshot"] = buildAutonomousOptimizerMonitoringSnapshot(bundle)
	metadata["current_prompt_bundle"] = buildAutonomousOptimizerPromptBundle(cfg, traderCfg, strategyCfg)

	if previousRun, err := s.loadAutonomousOptimizerRunIfAny(cfg.UserID, cfg.TraderID, cfg.LastRunID); err == nil && previousRun != nil {
		if result, handled, err := s.evaluateAutonomousOptimizerMonitoring(cfg, previousRun, traderCfg, strategyRecord, strategyCfg, bundle, metadata); handled || err != nil {
			return result, err
		}
	}

	if !autonomousOptimizerHasProposalEvidence(bundle) {
		return &autonomousOptimizerCycleResult{
			RunStatus:    store.AutonomousOptimizerStatusInsufficientEvidence,
			ConfigStatus: store.AutonomousOptimizerStatusInsufficientEvidence,
			Summary: fmt.Sprintf(
				"Only %d closed deals and %d decision cycles (%d candidates) were observed in the last %dh. No autonomous patch was applied.",
				bundle.Metadata.ClosedDeals,
				bundle.Metadata.DecisionRecordCount,
				bundle.Metadata.DecisionCandidateCount,
				cfg.ReviewIntervalHours,
			),
			Metadata:   metadata,
			Validation: map[string]any{"gate_reasons": []string{"insufficient evidence for proposal generation"}},
		}, nil
	}

	recentRuns, _ := s.store.AutonomousOptimizer().ListRuns(cfg.UserID, cfg.TraderID, 8)
	recentRuns = filterAutonomousOptimizerRunsExcluding(recentRuns, run.ID)
	symbolPriorPayload, symbolPriorErr := s.loadAutonomousOptimizerSymbolBehaviorPriorPayload(cfg, bundle)
	if symbolPriorErr != nil {
		logger.Warnf("⚠️ Failed to load autonomous optimizer symbol priors for trader %s: %v", cfg.TraderID, symbolPriorErr)
		metadata["symbol_behavior_priors_error"] = symbolPriorErr.Error()
	}
	if symbolPriorPayload != nil {
		metadata["symbol_behavior_priors"] = symbolPriorPayload
	}
	learnedPatternPayload, learnedPatternErr := s.loadAutonomousOptimizerLearnedPatternPayload(cfg, bundle)
	if learnedPatternErr != nil {
		logger.Warnf("⚠️ Failed to load autonomous optimizer learned patterns for trader %s: %v", cfg.TraderID, learnedPatternErr)
		metadata["learned_patterns_error"] = learnedPatternErr.Error()
	}
	if learnedPatternPayload != nil {
		metadata["learned_patterns"] = learnedPatternPayload
	}
	payload, err := buildAutonomousOptimizerReviewPayload(cfg, traderCfg, strategyRecord, strategyCfg, bundle, recentRuns, symbolPriorPayload, learnedPatternPayload)
	if err != nil {
		return nil, err
	}
	metadata["recent_optimizer_runs"] = summarizeAutonomousOptimizerRuns(recentRuns)
	if latestGateFeedback, ok := payload["latest_gate_feedback"]; ok {
		metadata["latest_gate_feedback"] = latestGateFeedback
	}

	proposerCfg, proposerModelName, err := s.resolveAIScanModel(cfg.UserID, cfg.PrimaryModelConfigID, cfg.PrimaryModelName)
	if err != nil {
		return nil, err
	}
	criticCfg, criticModelName, err := s.resolveAIScanModel(cfg.UserID, cfg.CriticModelConfigID, cfg.CriticModelName)
	if err != nil {
		return nil, err
	}

	proposalSystemPrompt, proposalUserPrompt, err := buildAutonomousOptimizerProposalPrompt(cfg, payload)
	if err != nil {
		return nil, err
	}
	proposerClient := newClientFromModelConfig(proposerCfg, proposerModelName)
	mcp.SetCallerContext(proposerClient, mcp.CallerContext{
		CallerType: "autonomous_optimizer",
		CallerID:   run.ID,
		CallerName: "autonomous_optimizer_proposer",
		UserID:     cfg.UserID,
		TraderID:   cfg.TraderID,
		TraderName: traderCfg.Name,
		Component:  "autonomous_optimizer.proposer",
	})
	proposalConversation, err := s.store.AutonomousOptimizer().GetOrCreateConversation(
		cfg.UserID,
		cfg.TraderID,
		store.AutonomousOptimizerConversationPurposeProposal,
		cfg.PrimaryModelConfigID,
		proposerModelName,
	)
	if err != nil {
		return nil, err
	}
	proposalHistory, err := s.store.AutonomousOptimizer().ListConversationMessages(
		proposalConversation.ID,
		proposalConversation.ReplayMessageLimit*3,
	)
	if err != nil {
		return nil, err
	}
	proposalRequest, proposalReplayCount, err := buildAutonomousOptimizerConversationRequest(
		proposalHistory,
		proposalConversation.ReplayMessageLimit,
		proposalSystemPrompt,
		proposalUserPrompt,
	)
	if err != nil {
		return nil, err
	}
	proposalResponse, err := proposerClient.CallWithRequest(proposalRequest)
	if err != nil {
		return nil, err
	}
	proposal, proposalRaw, err := parseAutonomousOptimizerProposalResponse(proposalResponse)
	if err != nil {
		return nil, err
	}
	metadata["proposal"] = parseAutonomousOptimizerJSONObject(proposalRaw)
	proposalUserReplay := buildAutonomousOptimizerProposalConversationUserReplay(payload)
	proposalAssistantReplay := buildAutonomousOptimizerProposalConversationAssistantReplay(proposal)
	if err := persistAutonomousOptimizerConversationTurn(
		s,
		proposalConversation,
		run.ID,
		proposalSystemPrompt,
		proposalUserPrompt,
		proposalResponse,
		proposalUserReplay,
		proposalAssistantReplay,
	); err != nil {
		return nil, err
	}
	metadata["proposal_conversation"] = map[string]any{
		"conversation_id":           proposalConversation.ID,
		"purpose":                   proposalConversation.Purpose,
		"mode":                      "persistent_replay_history",
		"history_messages_replayed": proposalReplayCount,
		"replay_message_limit":      proposalConversation.ReplayMessageLimit,
	}

	synthesizedBacklogItems := buildAutonomousOptimizerLearnedPatternBacklogProposals(learnedPatternPayload)
	if len(synthesizedBacklogItems) > 0 {
		metadata["learned_pattern_backlog_suggestions"] = map[string]any{
			"count": len(synthesizedBacklogItems),
			"items": synthesizedBacklogItems,
		}
	}
	combinedBacklogItems := append([]autonomousOptimizerBacklogProposal{}, proposal.BacklogItems...)
	combinedBacklogItems = append(combinedBacklogItems, synthesizedBacklogItems...)
	createdBacklogCount, backlogErr := s.saveAutonomousOptimizerBacklogFindings(cfg, run.ID, combinedBacklogItems, proposal.ExecutiveSummary)
	if backlogErr != nil {
		logger.Warnf("⚠️ Failed to persist autonomous optimizer backlog findings for trader %s: %v", cfg.TraderID, backlogErr)
	}
	metadata["backlog_items_created"] = createdBacklogCount
	metadata["backlog_items_ai_suggested"] = len(proposal.BacklogItems)
	metadata["backlog_items_synthesized_from_learned_patterns"] = len(synthesizedBacklogItems)

	criticSystemPrompt, criticUserPrompt, err := buildAutonomousOptimizerCriticPrompt(cfg, payload, proposal)
	if err != nil {
		return nil, err
	}
	criticClient := newClientFromModelConfig(criticCfg, criticModelName)
	mcp.SetCallerContext(criticClient, mcp.CallerContext{
		CallerType: "autonomous_optimizer",
		CallerID:   run.ID,
		CallerName: "autonomous_optimizer_critic",
		UserID:     cfg.UserID,
		TraderID:   cfg.TraderID,
		TraderName: traderCfg.Name,
		Component:  "autonomous_optimizer.critic",
	})
	criticConversation, err := s.store.AutonomousOptimizer().GetOrCreateConversation(
		cfg.UserID,
		cfg.TraderID,
		store.AutonomousOptimizerConversationPurposeCritic,
		cfg.CriticModelConfigID,
		criticModelName,
	)
	if err != nil {
		return nil, err
	}
	criticHistory, err := s.store.AutonomousOptimizer().ListConversationMessages(
		criticConversation.ID,
		criticConversation.ReplayMessageLimit*3,
	)
	if err != nil {
		return nil, err
	}
	criticRequest, criticReplayCount, err := buildAutonomousOptimizerConversationRequest(
		criticHistory,
		criticConversation.ReplayMessageLimit,
		criticSystemPrompt,
		criticUserPrompt,
	)
	if err != nil {
		return nil, err
	}
	criticResponse, err := criticClient.CallWithRequest(criticRequest)
	if err != nil {
		return nil, err
	}
	critic, criticRaw, err := parseAutonomousOptimizerCriticResponse(criticResponse)
	if err != nil {
		return nil, err
	}
	criticUserReplay := buildAutonomousOptimizerCriticConversationUserReplay(payload, proposal)
	criticAssistantReplay := buildAutonomousOptimizerCriticConversationAssistantReplay(critic)
	if err := persistAutonomousOptimizerConversationTurn(
		s,
		criticConversation,
		run.ID,
		criticSystemPrompt,
		criticUserPrompt,
		criticResponse,
		criticUserReplay,
		criticAssistantReplay,
	); err != nil {
		return nil, err
	}
	metadata["critic_conversation"] = map[string]any{
		"conversation_id":           criticConversation.ID,
		"purpose":                   criticConversation.Purpose,
		"mode":                      "persistent_replay_history",
		"history_messages_replayed": criticReplayCount,
		"replay_message_limit":      criticConversation.ReplayMessageLimit,
	}

	return s.evaluateAutonomousOptimizerProposal(cfg, run, traderCfg, strategyRecord, strategyCfg, bundle, recentRuns, metadata, proposal, critic, criticRaw)
}

func (s *Server) evaluateAutonomousOptimizerMonitoring(cfg *store.AutonomousOptimizerConfig, previousRun *store.AutonomousOptimizerRun, traderCfg *store.Trader, strategyRecord *store.Strategy, strategyCfg *store.StrategyConfig, bundle *autonomousOptimizerWindowBundle, metadata map[string]any) (*autonomousOptimizerCycleResult, bool, error) {
	if previousRun == nil {
		return nil, false, nil
	}
	switch previousRun.Status {
	case store.AutonomousOptimizerStatusAutoApplied, store.AutonomousOptimizerStatusMonitoring:
	case store.AutonomousOptimizerStatusRollbackPending:
		sourceRun, err := s.resolveAutonomousOptimizerMonitoringSourceRun(cfg.UserID, cfg.TraderID, previousRun)
		if err != nil {
			return nil, true, err
		}
		if sourceRun == nil {
			sourceRun = previousRun
		}
		pendingAnalysis := autonomousOptimizerRollbackAssessmentFromMetadata(parseAutonomousOptimizerJSONObject(previousRun.MetadataJSON))
		pendingAnalysis.MonitoringRootRunID = sourceRun.ID
		metadata["monitoring_source_status"] = previousRun.Status
		metadata["monitoring_source_run_id"] = sourceRun.ID
		metadata["rollback_analysis"] = buildAutonomousOptimizerRollbackAnalysisMap(pendingAnalysis)
		metadata["rollback_pending_from_run_id"] = previousRun.ID
		metadata["monitoring_verdict"] = "rolled_back"
		result, err := s.rollbackAutonomousOptimizerRun(cfg, sourceRun, traderCfg, strategyRecord, bundle, metadata, pendingAnalysis)
		return result, true, err
	default:
		return nil, false, nil
	}

	_ = traderCfg
	_ = strategyCfg
	sourceRun, err := s.resolveAutonomousOptimizerMonitoringSourceRun(cfg.UserID, cfg.TraderID, previousRun)
	if err != nil {
		return nil, true, err
	}
	if sourceRun == nil {
		sourceRun = previousRun
	}
	sourceSnapshot := autonomousOptimizerMonitoringSnapshotFromRun(sourceRun)
	currentSnapshot := buildAutonomousOptimizerMonitoringSnapshot(bundle)
	recentRuns, _ := s.store.AutonomousOptimizer().ListRuns(cfg.UserID, cfg.TraderID, 12)
	chain := buildAutonomousOptimizerMonitoringChainStats(recentRuns, sourceRun.ID)
	analysis := assessAutonomousOptimizerRollback(sourceSnapshot, currentSnapshot, chain)
	analysis.MonitoringRootRunID = sourceRun.ID
	metadata["monitoring_source_status"] = previousRun.Status
	metadata["monitoring_source_run_id"] = sourceRun.ID
	metadata["monitoring_snapshot"] = currentSnapshot
	metadata["monitoring_source_snapshot"] = sourceSnapshot
	metadata["monitoring_chain"] = chain
	metadata["rollback_analysis"] = buildAutonomousOptimizerRollbackAnalysisMap(analysis)
	metadata["monitoring_window_index"] = chain.WindowsObserved + 1

	if cfg.AutoRollbackEnabled && analysis.ShouldRollback {
		metadata["monitoring_verdict"] = "rollback_pending"
		metadata["rollback_pending_at_ms"] = bundle.WindowEnd.UTC().UnixMilli()
		metadata["rollback_source_run_id"] = sourceRun.ID
		rollbackReason := fmt.Sprintf("Rollback pending for run %s after monitored degradation.", sourceRun.ID)
		if len(analysis.TriggerReasons) > 0 {
			rollbackReason = analysis.TriggerReasons[0]
		}
		return &autonomousOptimizerCycleResult{
			RunStatus:    store.AutonomousOptimizerStatusRollbackPending,
			ConfigStatus: store.AutonomousOptimizerStatusRollbackPending,
			Summary:      fmt.Sprintf("Rollback pending for run %s. %s", sourceRun.ID, rollbackReason),
			Metadata:     metadata,
			Validation: map[string]any{
				"monitoring":             true,
				"rollback_pending":       true,
				"rollback_source_run_id": sourceRun.ID,
				"rollback_trigger_codes": analysis.TriggerCodes,
			},
			NextRunAt: bundle.WindowEnd.UTC().Add(autonomousOptimizerRollbackPendingDelay),
		}, true, nil
	}

	if !analysis.HasEnoughEvidence {
		metadata["monitoring_verdict"] = "monitoring"
		return &autonomousOptimizerCycleResult{
			RunStatus:    store.AutonomousOptimizerStatusMonitoring,
			ConfigStatus: store.AutonomousOptimizerStatusMonitoring,
			Summary: fmt.Sprintf(
				"Applied change from run %s is still in monitoring. %d monitored closed deals have accumulated so far, with %d new closed deals in the latest %dh window.",
				sourceRun.ID,
				analysis.ObservedClosedDeals,
				currentSnapshot.ClosedDeals,
				cfg.ReviewIntervalHours,
			),
			Metadata: metadata,
			Validation: map[string]any{
				"monitoring": true,
			},
		}, true, nil
	}

	metadata["monitoring_verdict"] = "kept"
	return &autonomousOptimizerCycleResult{
		RunStatus:    store.AutonomousOptimizerStatusKept,
		ConfigStatus: store.AutonomousOptimizerStatusKept,
		Summary: fmt.Sprintf(
			"Applied change from run %s was kept after %d monitored closed deals and %.2f cumulative monitored net PnL.",
			sourceRun.ID,
			analysis.ObservedClosedDeals,
			analysis.ObservedNetPnL,
		),
		Metadata: metadata,
		Validation: map[string]any{
			"monitoring": true,
			"verdict":    "kept",
		},
	}, true, nil
}

func (s *Server) evaluateAutonomousOptimizerProposal(cfg *store.AutonomousOptimizerConfig, run *store.AutonomousOptimizerRun, traderCfg *store.Trader, strategyRecord *store.Strategy, strategyCfg *store.StrategyConfig, bundle *autonomousOptimizerWindowBundle, recentRuns []*store.AutonomousOptimizerRun, metadata map[string]any, proposal *autonomousOptimizerProposalResult, critic *autonomousOptimizerCriticResult, criticRaw string) (*autonomousOptimizerCycleResult, error) {
	validation := map[string]any{
		"critic":           parseAutonomousOptimizerJSONObject(criticRaw),
		"gate_reasons":     []string{},
		"deferred_reasons": []string{},
	}
	gateReasons := []string{}
	deferredReasons := []string{}
	appliedComponents := []string{}

	if proposal.ProposalType == "pause_optimizer" {
		if critic.Approved && critic.RecommendedAction == "pause_optimizer" {
			metadata["pause_reason"] = proposal.PauseReason
			return &autonomousOptimizerCycleResult{
				RunStatus:    store.AutonomousOptimizerStatusPaused,
				ConfigStatus: store.AutonomousOptimizerStatusPaused,
				Summary:      coalesceAutonomousOptimizerSummary(proposal.ExecutiveSummary, proposal.PauseReason, "Optimizer paused itself after the latest review window."),
				Metadata:     metadata,
				ConfigPatch:  proposal.ConfigPatch,
				PromptPatch:  proposal.PromptPatch,
				Validation:   validation,
			}, nil
		}
		gateReasons = append(gateReasons, "Critic did not approve pausing the optimizer.")
	}

	if !critic.Approved && critic.RecommendedAction != "approve_backlog_only" {
		gateReasons = append(gateReasons, critic.BlockingIssues...)
	}

	var mergedStrategyCfg *store.StrategyConfig
	var mergedStrategyJSON string
	var configValidation *store.DealReviewAIScanValidation
	if len(proposal.ConfigPatch) > 0 {
		configPaths, configIssues := validateAutonomousOptimizerConfigPatchScope(proposal.ConfigPatch)
		validation["config_patch_paths"] = configPaths
		if len(configIssues) > 0 {
			gateReasons = append(gateReasons, configIssues...)
		} else if !cfg.AutoApplyConfigPatch {
			gateReasons = append(gateReasons, "Config auto-apply is disabled for this optimizer.")
		} else if !autonomousOptimizerCanValidateConfigPatch(bundle) {
			gateReasons = append(gateReasons, "Config patch requires more closed-deal evidence before autonomous apply.")
		} else {
			scanDetail := &store.DealReviewAIScanDetail{
				Filters: buildAutonomousOptimizerValidationDatasetFilters(bundle),
				Result: store.DealReviewAIScanResult{
					ExecutiveSummary: proposal.ExecutiveSummary,
				},
				StrategyPatch: proposal.ConfigPatch,
			}
			var err error
			configValidation, mergedStrategyJSON, _, err = s.validateDealReviewAIScan(cfg.UserID, traderCfg, scanDetail)
			if err != nil {
				return nil, err
			}
			validation["config_validation"] = configValidation
			if configValidation == nil || configValidation.Status != store.DealReviewAIScanValidationPassed {
				gateReasons = append(gateReasons, buildDealReviewValidationBlockMessage(configValidation))
			} else {
				mergedStrategyCfg = &store.StrategyConfig{}
				if err := json.Unmarshal([]byte(mergedStrategyJSON), mergedStrategyCfg); err != nil {
					gateReasons = append(gateReasons, "Merged strategy patch could not be parsed after validation.")
				}
			}
		}
	}

	traderModelCfg, traderModelName := s.resolveAutonomousOptimizerTraderRuntimeModel(cfg.UserID, traderCfg)
	if mergedStrategyCfg == nil {
		mergedStrategyCfg = strategyCfg
	}

	var mergedOptimizerCfg *store.AutonomousOptimizerConfig
	var mergedTraderCfg *store.Trader
	var promptValidation *autonomousOptimizerPromptValidation
	if len(proposal.PromptPatch) > 0 {
		decodedPromptPatch, err := decodeAutonomousOptimizerPromptPatch(proposal.PromptPatch)
		if err != nil {
			gateReasons = append(gateReasons, "Prompt patch could not be decoded.")
		} else if !cfg.AutoApplyPromptPatch {
			gateReasons = append(gateReasons, "Prompt auto-apply is disabled for this optimizer.")
		} else {
			mergedOptimizerCfg, mergedTraderCfg, mergedStrategyCfg, promptValidation, err = validateAutonomousOptimizerPromptPatch(cfg, traderCfg, mergedStrategyCfg, traderModelCfg, traderModelName, decodedPromptPatch)
			if err != nil {
				return nil, err
			}
			validation["prompt_validation"] = promptValidation
			if promptValidation == nil || len(promptValidation.BlockingIssues) > 0 {
				if promptValidation != nil {
					gateReasons = append(gateReasons, promptValidation.BlockingIssues...)
				} else {
					gateReasons = append(gateReasons, "Prompt patch validation failed.")
				}
			}
		}
	}

	canApplyConfig := len(proposal.ConfigPatch) > 0 && configValidation != nil && configValidation.Status == store.DealReviewAIScanValidationPassed
	canApplyPrompt := len(proposal.PromptPatch) > 0 && promptValidation != nil && len(promptValidation.BlockingIssues) == 0

	if !critic.Approved {
		canApplyConfig = false
		canApplyPrompt = false
	}
	if critic.RecommendedAction == "approve_backlog_only" {
		canApplyConfig = false
		canApplyPrompt = false
	}
	var deferredUntil time.Time
	if canApplyConfig || canApplyPrompt {
		if cooldownEnd, sourceRunID := autonomousOptimizerCooldownEnd(cfg, recentRuns, run.StartedAt); !cooldownEnd.IsZero() && cooldownEnd.After(run.StartedAt) {
			deferredReasons = append(deferredReasons, fmt.Sprintf("Auto-apply cooldown is active until %s from run %s.", cooldownEnd.UTC().Format(time.RFC3339), sourceRunID))
			metadata["cooldown_until_ms"] = cooldownEnd.UnixMilli()
			metadata["cooldown_source_run_id"] = sourceRunID
			if cooldownHours := cfg.AutoApplyCooldownHours; cooldownHours > 0 {
				metadata["cooldown_hours"] = cooldownHours
			}
			deferredUntil = cooldownEnd
		}
		if limit := cfg.MaxConsecutiveAutoApplies; limit > 0 {
			if consecutive := countConsecutiveAutonomousApplies(recentRuns); consecutive >= limit {
				deferredReasons = append(deferredReasons, fmt.Sprintf("Auto-apply deferred after %d consecutive apply windows; limit is %d.", consecutive, limit))
				metadata["consecutive_apply_count"] = consecutive
				metadata["consecutive_apply_limit"] = limit
			}
		}
	}

	if proposal.ProposalType == "no_change" && !canApplyConfig && !canApplyPrompt {
		validation["gate_reasons"] = dedupeSortedStrings(gateReasons)
		return &autonomousOptimizerCycleResult{
			RunStatus:    store.AutonomousOptimizerStatusNoChange,
			ConfigStatus: store.AutonomousOptimizerStatusNoChange,
			Summary:      coalesceAutonomousOptimizerSummary(proposal.ExecutiveSummary, critic.Summary, "Optimizer reviewed the window and kept the current config unchanged."),
			Metadata:     metadata,
			ConfigPatch:  proposal.ConfigPatch,
			PromptPatch:  proposal.PromptPatch,
			Validation:   validation,
		}, nil
	}

	if !canApplyConfig && !canApplyPrompt {
		validation["gate_reasons"] = dedupeSortedStrings(gateReasons)
		if proposal.ProposalType == "backlog_only" || critic.RecommendedAction == "approve_backlog_only" {
			return &autonomousOptimizerCycleResult{
				RunStatus:    store.AutonomousOptimizerStatusBacklogOnly,
				ConfigStatus: store.AutonomousOptimizerStatusBacklogOnly,
				Summary:      coalesceAutonomousOptimizerSummary(proposal.ExecutiveSummary, critic.Summary, "Optimizer created backlog findings but did not apply a live patch."),
				Metadata:     metadata,
				ConfigPatch:  proposal.ConfigPatch,
				PromptPatch:  proposal.PromptPatch,
				Validation:   validation,
			}, nil
		}
		return &autonomousOptimizerCycleResult{
			RunStatus:    store.AutonomousOptimizerStatusBlockedByGate,
			ConfigStatus: store.AutonomousOptimizerStatusBlockedByGate,
			Summary:      coalesceAutonomousOptimizerSummary(proposal.ExecutiveSummary, critic.Summary, "Optimizer generated a patch idea, but machine gates blocked the apply."),
			Metadata:     metadata,
			ConfigPatch:  proposal.ConfigPatch,
			PromptPatch:  proposal.PromptPatch,
			Validation:   validation,
		}, nil
	}
	if len(deferredReasons) > 0 {
		validation["gate_reasons"] = dedupeSortedStrings(append(append([]string{}, gateReasons...), deferredReasons...))
		validation["deferred_reasons"] = dedupeSortedStrings(deferredReasons)
		metadata["deferred_reasons"] = dedupeSortedStrings(deferredReasons)
		summary := coalesceAutonomousOptimizerSummary(
			proposal.ExecutiveSummary,
			critic.Summary,
			"Optimizer produced a valid patch idea, but deferred the live apply to the next eligible window.",
		)
		if deferredUntil.IsZero() {
			deferredUntil = run.StartedAt.Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)
		}
		metadata["next_eligible_run_ms"] = deferredUntil.UnixMilli()
		return &autonomousOptimizerCycleResult{
			RunStatus:    store.AutonomousOptimizerStatusDeferredForNextWindow,
			ConfigStatus: store.AutonomousOptimizerStatusDeferredForNextWindow,
			Summary:      summary,
			Metadata:     metadata,
			ConfigPatch:  proposal.ConfigPatch,
			PromptPatch:  proposal.PromptPatch,
			Validation:   validation,
			NextRunAt:    deferredUntil,
		}, nil
	}

	if canApplyConfig {
		appliedComponents = append(appliedComponents, "config_patch")
	}
	if canApplyPrompt {
		appliedComponents = append(appliedComponents, "prompt_patch")
	}
	metadata["applied_components"] = dedupeSortedStrings(appliedComponents)
	validation["gate_reasons"] = dedupeSortedStrings(gateReasons)
	return s.applyAutonomousOptimizerProposal(cfg, traderCfg, strategyRecord, strategyCfg, run, proposal, mergedOptimizerCfg, mergedStrategyCfg, mergedStrategyJSON, mergedTraderCfg, metadata, validation)
}

func (s *Server) applyAutonomousOptimizerProposal(cfg *store.AutonomousOptimizerConfig, traderCfg *store.Trader, strategyRecord *store.Strategy, strategyCfg *store.StrategyConfig, run *store.AutonomousOptimizerRun, proposal *autonomousOptimizerProposalResult, mergedOptimizerCfg *store.AutonomousOptimizerConfig, mergedStrategyCfg *store.StrategyConfig, mergedStrategyJSON string, mergedTraderCfg *store.Trader, metadata map[string]any, validation map[string]any) (*autonomousOptimizerCycleResult, error) {
	_ = strategyCfg
	_ = run
	wasRunning := false
	if existingMemTrader, memErr := s.traderManager.GetTrader(cfg.TraderID); memErr == nil {
		status := existingMemTrader.GetStatus()
		if running, ok := status["is_running"].(bool); ok && running {
			wasRunning = true
		}
	}

	previousStrategyMap := parseAutonomousOptimizerJSONObject(strategyRecord.Config)
	previousTraderSnapshot := buildAutonomousOptimizerTraderSnapshot(traderCfg)
	previousOptimizerSnapshot := buildAutonomousOptimizerConfigPromptSnapshot(cfg)

	finalStrategyJSON := strategyRecord.Config
	finalStrategyMap := previousStrategyMap
	if mergedStrategyCfg != nil {
		body, err := json.Marshal(mergedStrategyCfg)
		if err != nil {
			return nil, err
		}
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		finalStrategyMap = parsed
		finalStrategyJSON, err = marshalPrettyJSONMap(parsed)
		if err != nil {
			return nil, err
		}
	}
	if len(finalStrategyMap) == 0 && strings.TrimSpace(mergedStrategyJSON) != "" {
		finalStrategyJSON = mergedStrategyJSON
		finalStrategyMap = parseAutonomousOptimizerJSONObject(mergedStrategyJSON)
	}
	if mergedTraderCfg == nil {
		clone := *traderCfg
		mergedTraderCfg = &clone
	}
	nextTraderSnapshot := buildAutonomousOptimizerTraderSnapshot(mergedTraderCfg)
	if mergedOptimizerCfg == nil {
		clone := *cfg
		mergedOptimizerCfg = &clone
	}
	nextOptimizerSnapshot := buildAutonomousOptimizerConfigPromptSnapshot(mergedOptimizerCfg)

	version := &store.DealReviewStrategyVersion{
		UserID:             cfg.UserID,
		TraderID:           cfg.TraderID,
		StrategyID:         strategyRecord.ID,
		SourceType:         "autonomous_optimizer_apply",
		Summary:            proposal.ExecutiveSummary,
		ExpectedEffect:     proposal.ExpectedEffect,
		TargetCohortJSON:   buildDealReviewStrategyVersionTargetCohortJSON(buildAutonomousOptimizerValidationDatasetFilters(nil)),
		PreviousConfigJSON: strategyRecord.Config,
		NextConfigJSON:     finalStrategyJSON,
		AppliedAt:          time.Now().UTC(),
	}
	if err := s.store.DealReview().SaveStrategyVersion(version); err != nil {
		return nil, err
	}

	if strategyRecord.Config != finalStrategyJSON {
		strategyRecord.Config = finalStrategyJSON
		if err := s.store.Strategy().Update(strategyRecord); err != nil {
			return nil, err
		}
	}

	traderChanged := traderCfg.CustomPrompt != mergedTraderCfg.CustomPrompt ||
		traderCfg.OverrideBasePrompt != mergedTraderCfg.OverrideBasePrompt ||
		traderCfg.SystemPromptTemplate != mergedTraderCfg.SystemPromptTemplate
	if traderChanged {
		traderCfg.CustomPrompt = mergedTraderCfg.CustomPrompt
		traderCfg.OverrideBasePrompt = mergedTraderCfg.OverrideBasePrompt
		traderCfg.SystemPromptTemplate = mergedTraderCfg.SystemPromptTemplate
		if err := s.store.Trader().Update(traderCfg); err != nil {
			return nil, err
		}
	}
	optimizerPromptChanged := cfg.ProposalPromptInstructions != mergedOptimizerCfg.ProposalPromptInstructions ||
		cfg.CriticPromptInstructions != mergedOptimizerCfg.CriticPromptInstructions
	if optimizerPromptChanged {
		cfg.ProposalPromptInstructions = mergedOptimizerCfg.ProposalPromptInstructions
		cfg.CriticPromptInstructions = mergedOptimizerCfg.CriticPromptInstructions
	}

	if err := s.reloadTraderAfterStrategyChange(cfg.UserID, cfg.TraderID, wasRunning, "autonomous optimizer auto-apply"); err != nil {
		logger.Warnf("⚠️ Failed to reload trader after autonomous optimizer auto-apply: %v", err)
	}

	metadata["apply"] = map[string]any{
		"previous_strategy_config":  previousStrategyMap,
		"next_strategy_config":      finalStrategyMap,
		"previous_trader":           previousTraderSnapshot,
		"next_trader":               nextTraderSnapshot,
		"previous_optimizer_config": previousOptimizerSnapshot,
		"next_optimizer_config":     nextOptimizerSnapshot,
	}
	metadata["monitoring_root_run_id"] = run.ID
	metadata["monitoring_window_index"] = 0

	summary := proposal.ExecutiveSummary
	if summary == "" {
		summary = "Autonomous optimizer applied a validated live patch."
	}
	return &autonomousOptimizerCycleResult{
		RunStatus:                store.AutonomousOptimizerStatusAutoApplied,
		ConfigStatus:             store.AutonomousOptimizerStatusMonitoring,
		Summary:                  summary,
		Metadata:                 metadata,
		ConfigPatch:              proposal.ConfigPatch,
		PromptPatch:              proposal.PromptPatch,
		Validation:               validation,
		AppliedStrategyVersionID: version.ID,
	}, nil
}

func (s *Server) rollbackAutonomousOptimizerRun(cfg *store.AutonomousOptimizerConfig, sourceRun *store.AutonomousOptimizerRun, traderCfg *store.Trader, strategyRecord *store.Strategy, bundle *autonomousOptimizerWindowBundle, metadata map[string]any, analysis autonomousOptimizerRollbackAssessment) (*autonomousOptimizerCycleResult, error) {
	runMetadata := parseAutonomousOptimizerJSONObject(sourceRun.MetadataJSON)
	applyData := parseAutonomousOptimizerNestedObject(runMetadata, "apply")
	previousStrategyConfig := parseAutonomousOptimizerNestedObject(applyData, "previous_strategy_config")
	previousTraderSnapshot := parseAutonomousOptimizerNestedObject(applyData, "previous_trader")
	previousOptimizerSnapshot := parseAutonomousOptimizerNestedObject(applyData, "previous_optimizer_config")

	if len(previousStrategyConfig) == 0 || len(previousTraderSnapshot) == 0 {
		return &autonomousOptimizerCycleResult{
			RunStatus:    store.AutonomousOptimizerStatusFailed,
			ConfigStatus: store.AutonomousOptimizerStatusFailed,
			Summary:      fmt.Sprintf("Rollback was required for run %s, but no previous snapshot was available.", sourceRun.ID),
			Metadata:     metadata,
			Validation: map[string]any{
				"rollback_source_run_id": sourceRun.ID,
			},
		}, nil
	}

	targetStrategyJSON, err := marshalPrettyJSONMap(previousStrategyConfig)
	if err != nil {
		return nil, err
	}
	var targetStrategyCfg store.StrategyConfig
	if err := json.Unmarshal([]byte(targetStrategyJSON), &targetStrategyCfg); err != nil {
		return nil, err
	}
	if warnings, err := validateStrategyConfig(&targetStrategyCfg); err != nil {
		return nil, err
	} else if len(warnings) > 0 {
		logger.Infof("⚠️ Autonomous optimizer rollback warnings: %s", strings.Join(warnings, "; "))
	}

	wasRunning := false
	if existingMemTrader, memErr := s.traderManager.GetTrader(cfg.TraderID); memErr == nil {
		status := existingMemTrader.GetStatus()
		if running, ok := status["is_running"].(bool); ok && running {
			wasRunning = true
		}
	}

	rollbackVersion := &store.DealReviewStrategyVersion{
		UserID:             cfg.UserID,
		TraderID:           cfg.TraderID,
		StrategyID:         strategyRecord.ID,
		SourceType:         "autonomous_optimizer_rollback",
		Summary:            fmt.Sprintf("Autonomous rollback of optimizer run %s", sourceRun.ID),
		ExpectedEffect:     "Restore the previous live configuration after a degrading monitored window.",
		PreviousConfigJSON: strategyRecord.Config,
		NextConfigJSON:     targetStrategyJSON,
		AppliedAt:          time.Now().UTC(),
	}
	if err := s.store.DealReview().SaveStrategyVersion(rollbackVersion); err != nil {
		return nil, err
	}

	strategyRecord.Config = targetStrategyJSON
	if err := s.store.Strategy().Update(strategyRecord); err != nil {
		return nil, err
	}

	restoreAutonomousOptimizerTraderSnapshot(traderCfg, previousTraderSnapshot)
	if err := s.store.Trader().Update(traderCfg); err != nil {
		return nil, err
	}
	restoreAutonomousOptimizerConfigPromptSnapshot(cfg, previousOptimizerSnapshot)

	if err := s.reloadTraderAfterStrategyChange(cfg.UserID, cfg.TraderID, wasRunning, "autonomous optimizer rollback"); err != nil {
		logger.Warnf("⚠️ Failed to reload trader after autonomous rollback: %v", err)
	}

	metadata["rollback_source_run_id"] = sourceRun.ID
	metadata["rollback_analysis"] = buildAutonomousOptimizerRollbackAnalysisMap(analysis)
	metadata["rollback_restored_optimizer_config"] = buildAutonomousOptimizerConfigPromptSnapshot(cfg)
	rollbackReason := fmt.Sprintf("Monitoring drift was detected after %.2f cumulative net PnL across %d monitored closed deals.", analysis.ObservedNetPnL, analysis.ObservedClosedDeals)
	if len(analysis.TriggerReasons) > 0 {
		rollbackReason = analysis.TriggerReasons[0]
	}
	metadata["rollback_reason"] = rollbackReason
	return &autonomousOptimizerCycleResult{
		RunStatus:                store.AutonomousOptimizerStatusRolledBack,
		ConfigStatus:             store.AutonomousOptimizerStatusRolledBack,
		Summary:                  fmt.Sprintf("Autonomous optimizer rolled back run %s. %s", sourceRun.ID, rollbackReason),
		Metadata:                 metadata,
		Validation:               map[string]any{"rollback_source_run_id": sourceRun.ID, "rollback_trigger_codes": analysis.TriggerCodes},
		AppliedStrategyVersionID: rollbackVersion.ID,
	}, nil
}

func (s *Server) saveAutonomousOptimizerBacklogFindings(cfg *store.AutonomousOptimizerConfig, runID string, items []autonomousOptimizerBacklogProposal, reviewSummary string) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	existing, err := s.store.AutonomousOptimizer().ListBacklog(cfg.UserID, cfg.TraderID, 200)
	if err != nil {
		return 0, err
	}
	index := make(map[string]*store.AutonomousOptimizerBacklogItem, len(existing))
	for _, item := range existing {
		if item == nil {
			continue
		}
		index[autonomousOptimizerBacklogMergeKey(item.Category, item.Title)] = item
	}

	created := 0
	for _, proposal := range items {
		if strings.TrimSpace(proposal.Title) == "" {
			continue
		}
		metadata := map[string]any{
			"run_id":         runID,
			"review_summary": clipDealReviewAIScanText(reviewSummary, 500),
		}
		for key, value := range sanitizeAutonomousOptimizerJSONMap(proposal.Metadata) {
			metadata[key] = value
		}
		evidenceJSON := "[]"
		if len(proposal.Evidence) > 0 {
			if raw, marshalErr := json.Marshal(sanitizeAutonomousOptimizerJSONObjectArray(proposal.Evidence)); marshalErr == nil {
				evidenceJSON = string(raw)
			}
		}
		metadataJSON := "{}"
		if raw, marshalErr := json.Marshal(metadata); marshalErr == nil {
			metadataJSON = string(raw)
		}
		item := &store.AutonomousOptimizerBacklogItem{
			UserID:             cfg.UserID,
			TraderID:           cfg.TraderID,
			RunID:              runID,
			Title:              proposal.Title,
			Category:           proposal.Category,
			Description:        proposal.Description,
			ExpectedImpact:     proposal.ExpectedImpact,
			Confidence:         proposal.Confidence,
			ImplementationCost: proposal.ImplementationCost,
			Urgency:            proposal.Urgency,
			RecurrenceCount:    proposal.RecurrenceCount,
			Status:             store.AutonomousOptimizerBacklogStatusNew,
			AIGenerated:        true,
			UserEdited:         false,
			MergedFindingCount: 1,
			EvidenceJSON:       evidenceJSON,
			MetadataJSON:       metadataJSON,
		}
		key := autonomousOptimizerBacklogMergeKey(item.Category, item.Title)
		if current := index[key]; current != nil {
			current.RunID = runID
			current.Description = coalesceAutonomousOptimizerSummary(current.Description, item.Description, current.Description)
			current.ExpectedImpact = coalesceAutonomousOptimizerSummary(current.ExpectedImpact, item.ExpectedImpact, current.ExpectedImpact)
			current.Confidence = maxAutonomousOptimizerValue(current.Confidence, item.Confidence)
			current.ImplementationCost = maxAutonomousOptimizerValue(current.ImplementationCost, item.ImplementationCost)
			current.Urgency = maxAutonomousOptimizerValue(current.Urgency, item.Urgency)
			current.RecurrenceCount += item.RecurrenceCount
			current.MergedFindingCount++
			current.EvidenceJSON = mergeAutonomousOptimizerBacklogEvidenceJSON(current.EvidenceJSON, item.EvidenceJSON)
			current.MetadataJSON = mergeAutonomousOptimizerBacklogMetadataJSON(current.MetadataJSON, item.MetadataJSON)
			current.CompositeScore = buildAutonomousOptimizerBacklogScore(current)
			if err := s.store.AutonomousOptimizer().SaveBacklogItem(current); err != nil {
				return created, err
			}
			continue
		}
		item.CompositeScore = buildAutonomousOptimizerBacklogScore(item)
		if err := s.store.AutonomousOptimizer().SaveBacklogItem(item); err != nil {
			return created, err
		}
		index[key] = item
		created++
	}
	return created, nil
}

func (s *Server) loadAutonomousOptimizerRunIfAny(userID, traderID, runID string) (*store.AutonomousOptimizerRun, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, nil
	}
	return s.store.AutonomousOptimizer().GetRun(userID, traderID, runID)
}

func (s *Server) resolveAutonomousOptimizerTraderRuntimeModel(userID string, traderCfg *store.Trader) (*store.AIModel, string) {
	if traderCfg == nil || strings.TrimSpace(traderCfg.AIModelID) == "" {
		return nil, ""
	}
	modelCfg, err := s.store.AIModel().Get(userID, traderCfg.AIModelID)
	if err != nil || modelCfg == nil {
		return nil, ""
	}
	modelName := strings.TrimSpace(modelCfg.CustomModelName)
	if modelName == "" {
		modelName = defaultModelForProvider(modelCfg.Provider)
	}
	return modelCfg, modelName
}

func buildAutonomousOptimizerMetadataMap(meta autonomousOptimizerRunMetadata) map[string]any {
	body, _ := json.Marshal(meta)
	out := map[string]any{}
	_ = json.Unmarshal(body, &out)
	return out
}

func buildAutonomousOptimizerMonitoringSnapshot(bundle *autonomousOptimizerWindowBundle) autonomousOptimizerMonitoringSnapshot {
	if bundle == nil {
		return autonomousOptimizerMonitoringSnapshot{}
	}
	return autonomousOptimizerMonitoringSnapshot{
		ClosedDeals:            bundle.Metadata.ClosedDeals,
		WinningDeals:           bundle.Metadata.WinningDeals,
		LosingDeals:            bundle.Metadata.LosingDeals,
		NetPnL:                 bundle.Metadata.NetPnL,
		AvgPnL:                 bundle.Metadata.AvgPnL,
		WinRate:                bundle.Metadata.WinRate,
		Expectancy:             bundle.Metadata.Expectancy,
		ProfitFactor:           bundle.Metadata.ProfitFactor,
		MaxDrawdownPct:         bundle.Metadata.MaxDrawdownPct,
		DecisionRecordCount:    bundle.Metadata.DecisionRecordCount,
		DecisionCandidateCount: bundle.Metadata.DecisionCandidateCount,
		OpenDecisionCount:      bundle.Metadata.OpenDecisionCount,
		HoldDecisionCount:      bundle.Metadata.HoldDecisionCount,
		WaitDecisionCount:      bundle.Metadata.WaitDecisionCount,
		DecisionConversionRate: bundle.Metadata.DecisionConversionRate,
		AvgDecisionConfidence:  bundle.Metadata.AvgDecisionConfidence,
		AvgMFECapturedPct:      bundle.Metadata.AvgMFECapturedPct,
		AvgProfitGivenBackPct:  bundle.Metadata.AvgProfitGivenBackPct,
		AvgExitEfficiencyScore: bundle.Metadata.AvgExitEfficiencyScore,
		AvgEntryTimingScore:    bundle.Metadata.AvgEntryTimingScore,
		AvgRiskSizingScore:     bundle.Metadata.AvgRiskSizingScore,
		BadEntryDeals:          bundle.Metadata.BadEntryDeals,
		BadExitDeals:           bundle.Metadata.BadExitDeals,
		AvoidableLossDeals:     bundle.Metadata.AvoidableLossDeals,
	}
}

func autonomousOptimizerMonitoringSnapshotFromRun(run *store.AutonomousOptimizerRun) autonomousOptimizerMonitoringSnapshot {
	if run == nil {
		return autonomousOptimizerMonitoringSnapshot{}
	}
	return autonomousOptimizerMonitoringSnapshotFromMetadata(parseAutonomousOptimizerJSONObject(run.MetadataJSON))
}

func autonomousOptimizerMonitoringSnapshotFromMetadata(metadata map[string]any) autonomousOptimizerMonitoringSnapshot {
	if nested := parseAutonomousOptimizerNestedObject(metadata, "monitoring_snapshot"); len(nested) > 0 {
		metadata = nested
	}
	return autonomousOptimizerMonitoringSnapshot{
		ClosedDeals:            autonomousOptimizerInt64(metadata["closed_deals"]),
		WinningDeals:           autonomousOptimizerInt64(metadata["winning_deals"]),
		LosingDeals:            autonomousOptimizerInt64(metadata["losing_deals"]),
		NetPnL:                 autonomousOptimizerFloat64(metadata["net_pnl"]),
		AvgPnL:                 autonomousOptimizerFloat64(metadata["avg_pnl"]),
		WinRate:                autonomousOptimizerFloat64(metadata["win_rate"]),
		Expectancy:             autonomousOptimizerFloat64(metadata["expectancy"]),
		ProfitFactor:           autonomousOptimizerFloat64(metadata["profit_factor"]),
		MaxDrawdownPct:         autonomousOptimizerFloat64(metadata["max_drawdown_pct"]),
		DecisionRecordCount:    autonomousOptimizerInt(metadata["decision_record_count"]),
		DecisionCandidateCount: autonomousOptimizerInt(metadata["decision_candidate_count"]),
		OpenDecisionCount:      autonomousOptimizerInt(metadata["open_decision_count"]),
		HoldDecisionCount:      autonomousOptimizerInt(metadata["hold_decision_count"]),
		WaitDecisionCount:      autonomousOptimizerInt(metadata["wait_decision_count"]),
		DecisionConversionRate: autonomousOptimizerFloat64(metadata["decision_conversion_rate"]),
		AvgDecisionConfidence:  autonomousOptimizerFloat64(metadata["avg_decision_confidence"]),
		AvgMFECapturedPct:      autonomousOptimizerFloat64(metadata["avg_mfe_captured_pct"]),
		AvgProfitGivenBackPct:  autonomousOptimizerFloat64(metadata["avg_profit_given_back_pct"]),
		AvgExitEfficiencyScore: autonomousOptimizerFloat64(metadata["avg_exit_efficiency_score"]),
		AvgEntryTimingScore:    autonomousOptimizerFloat64(metadata["avg_entry_timing_score"]),
		AvgRiskSizingScore:     autonomousOptimizerFloat64(metadata["avg_risk_sizing_score"]),
		BadEntryDeals:          autonomousOptimizerInt64(metadata["bad_entry_deals"]),
		BadExitDeals:           autonomousOptimizerInt64(metadata["bad_exit_deals"]),
		AvoidableLossDeals:     autonomousOptimizerInt64(metadata["avoidable_loss_deals"]),
	}
}

func buildAutonomousOptimizerRollbackAnalysisMap(analysis autonomousOptimizerRollbackAssessment) map[string]any {
	return map[string]any{
		"should_rollback":           analysis.ShouldRollback,
		"has_enough_evidence":       analysis.HasEnoughEvidence,
		"trigger_codes":             analysis.TriggerCodes,
		"trigger_reasons":           analysis.TriggerReasons,
		"monitoring_root_run_id":    analysis.MonitoringRootRunID,
		"observed_closed_deals":     analysis.ObservedClosedDeals,
		"observed_net_pnl":          analysis.ObservedNetPnL,
		"observed_negative_windows": analysis.ObservedNegativeWindows,
		"current_snapshot":          analysis.CurrentSnapshot,
		"source_snapshot":           analysis.SourceSnapshot,
		"chain":                     analysis.Chain,
	}
}

func autonomousOptimizerRollbackAssessmentFromMetadata(metadata map[string]any) autonomousOptimizerRollbackAssessment {
	analysisMap := parseAutonomousOptimizerNestedObject(metadata, "rollback_analysis")
	if len(analysisMap) == 0 {
		analysisMap = metadata
	}
	assessment := autonomousOptimizerRollbackAssessment{
		ShouldRollback:          autonomousOptimizerBool(analysisMap["should_rollback"]),
		HasEnoughEvidence:       autonomousOptimizerBool(analysisMap["has_enough_evidence"]),
		TriggerCodes:            autonomousOptimizerStringSlice(analysisMap["trigger_codes"]),
		TriggerReasons:          autonomousOptimizerStringSlice(analysisMap["trigger_reasons"]),
		CurrentSnapshot:         autonomousOptimizerMonitoringSnapshotFromMetadata(parseAutonomousOptimizerNestedObject(analysisMap, "current_snapshot")),
		SourceSnapshot:          autonomousOptimizerMonitoringSnapshotFromMetadata(parseAutonomousOptimizerNestedObject(analysisMap, "source_snapshot")),
		MonitoringRootRunID:     autonomousOptimizerString(analysisMap["monitoring_root_run_id"]),
		ObservedClosedDeals:     autonomousOptimizerInt64(analysisMap["observed_closed_deals"]),
		ObservedNetPnL:          autonomousOptimizerFloat64(analysisMap["observed_net_pnl"]),
		ObservedNegativeWindows: autonomousOptimizerInt(analysisMap["observed_negative_windows"]),
	}
	chainMap := parseAutonomousOptimizerNestedObject(analysisMap, "chain")
	assessment.Chain = autonomousOptimizerMonitoringChainStats{
		RootRunID:        autonomousOptimizerString(chainMap["root_run_id"]),
		WindowsObserved:  autonomousOptimizerInt(chainMap["windows_observed"]),
		NegativeWindows:  autonomousOptimizerInt(chainMap["negative_windows"]),
		TotalClosedDeals: autonomousOptimizerInt64(chainMap["total_closed_deals"]),
		TotalNetPnL:      autonomousOptimizerFloat64(chainMap["total_net_pnl"]),
	}
	if assessment.MonitoringRootRunID == "" {
		assessment.MonitoringRootRunID = assessment.Chain.RootRunID
	}
	return assessment
}

func buildAutonomousOptimizerConfigPromptSnapshot(cfg *store.AutonomousOptimizerConfig) map[string]any {
	if cfg == nil {
		return map[string]any{}
	}
	return map[string]any{
		"proposal_instructions": strings.TrimSpace(cfg.ProposalPromptInstructions),
		"critic_instructions":   strings.TrimSpace(cfg.CriticPromptInstructions),
	}
}

func restoreAutonomousOptimizerConfigPromptSnapshot(cfg *store.AutonomousOptimizerConfig, snapshot map[string]any) {
	if cfg == nil {
		return
	}
	cfg.ProposalPromptInstructions = autonomousOptimizerString(snapshot["proposal_instructions"])
	cfg.CriticPromptInstructions = autonomousOptimizerString(snapshot["critic_instructions"])
}

func parseAutonomousOptimizerJSONObject(body string) map[string]any {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func parseAutonomousOptimizerJSONArray(body string) []map[string]any {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return nil
	}
	out := make([]map[string]any, 0)
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil
	}
	return out
}

func parseAutonomousOptimizerNestedObject(parent map[string]any, key string) map[string]any {
	if len(parent) == 0 {
		return map[string]any{}
	}
	value, ok := parent[key]
	if !ok || value == nil {
		return map[string]any{}
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed
	default:
		body, err := json.Marshal(typed)
		if err != nil {
			return map[string]any{}
		}
		out := map[string]any{}
		if err := json.Unmarshal(body, &out); err != nil {
			return map[string]any{}
		}
		return out
	}
}

func autonomousOptimizerBool(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return false
}

func autonomousOptimizerStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return dedupeSortedStrings(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if str, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(str); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return dedupeSortedStrings(out)
	default:
		return nil
	}
}

func autonomousOptimizerString(value any) string {
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str)
	}
	return ""
}

func autonomousOptimizerInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed
		}
	}
	return 0
}

func autonomousOptimizerInt(value any) int {
	return int(autonomousOptimizerInt64(value))
}

func autonomousOptimizerFloat64(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err == nil {
			return parsed
		}
	}
	return 0
}

func (s *Server) resolveAutonomousOptimizerMonitoringSourceRun(userID, traderID string, previousRun *store.AutonomousOptimizerRun) (*store.AutonomousOptimizerRun, error) {
	current := previousRun
	visited := map[string]struct{}{}
	for step := 0; step < 6 && current != nil; step++ {
		if _, seen := visited[current.ID]; seen {
			return current, nil
		}
		visited[current.ID] = struct{}{}
		if current.Status == store.AutonomousOptimizerStatusAutoApplied {
			return current, nil
		}
		metadata := parseAutonomousOptimizerJSONObject(current.MetadataJSON)
		nextID := autonomousOptimizerString(metadata["monitoring_root_run_id"])
		if nextID == "" {
			nextID = autonomousOptimizerString(metadata["monitoring_source_run_id"])
		}
		if nextID == "" {
			nextID = autonomousOptimizerString(metadata["rollback_source_run_id"])
		}
		if nextID == "" || nextID == current.ID {
			return current, nil
		}
		nextRun, err := s.loadAutonomousOptimizerRunIfAny(userID, traderID, nextID)
		if err != nil {
			return nil, err
		}
		if nextRun == nil {
			return current, nil
		}
		current = nextRun
	}
	return current, nil
}

func buildAutonomousOptimizerMonitoringChainStats(runs []*store.AutonomousOptimizerRun, rootRunID string) autonomousOptimizerMonitoringChainStats {
	stats := autonomousOptimizerMonitoringChainStats{RootRunID: rootRunID}
	if rootRunID == "" || len(runs) == 0 {
		return stats
	}
	for _, item := range runs {
		if item == nil || item.ID == rootRunID {
			continue
		}
		metadata := parseAutonomousOptimizerJSONObject(item.MetadataJSON)
		runRootID := autonomousOptimizerString(metadata["monitoring_root_run_id"])
		if runRootID == "" {
			runRootID = autonomousOptimizerString(metadata["monitoring_source_run_id"])
		}
		if runRootID == "" {
			runRootID = autonomousOptimizerString(metadata["rollback_source_run_id"])
		}
		if runRootID != rootRunID {
			continue
		}
		switch item.Status {
		case store.AutonomousOptimizerStatusMonitoring, store.AutonomousOptimizerStatusRollbackPending, store.AutonomousOptimizerStatusKept, store.AutonomousOptimizerStatusRolledBack:
		default:
			continue
		}
		snapshot := autonomousOptimizerMonitoringSnapshotFromMetadata(metadata)
		stats.WindowsObserved++
		stats.TotalClosedDeals += snapshot.ClosedDeals
		stats.TotalNetPnL += snapshot.NetPnL
		if snapshot.ClosedDeals > 0 && snapshot.NetPnL < -0.01 {
			stats.NegativeWindows++
		}
	}
	return stats
}

func assessAutonomousOptimizerRollback(source, current autonomousOptimizerMonitoringSnapshot, chain autonomousOptimizerMonitoringChainStats) autonomousOptimizerRollbackAssessment {
	assessment := autonomousOptimizerRollbackAssessment{
		CurrentSnapshot:         current,
		SourceSnapshot:          source,
		Chain:                   chain,
		ObservedClosedDeals:     chain.TotalClosedDeals + current.ClosedDeals,
		ObservedNetPnL:          chain.TotalNetPnL + current.NetPnL,
		ObservedNegativeWindows: chain.NegativeWindows,
	}
	if current.ClosedDeals > 0 && current.NetPnL < -0.01 {
		assessment.ObservedNegativeWindows++
	}
	assessment.HasEnoughEvidence = assessment.ObservedClosedDeals >= 2

	addTrigger := func(code, reason string) {
		if code == "" || reason == "" {
			return
		}
		assessment.TriggerCodes = append(assessment.TriggerCodes, code)
		assessment.TriggerReasons = append(assessment.TriggerReasons, reason)
	}

	if current.ClosedDeals >= 2 && current.NetPnL < -0.01 {
		addTrigger(
			"negative_monitored_window",
			fmt.Sprintf("The latest monitoring window closed %d deals with %.2f net PnL.", current.ClosedDeals, current.NetPnL),
		)
	}
	if assessment.ObservedClosedDeals >= 2 && assessment.ObservedNegativeWindows >= 2 && assessment.ObservedNetPnL < -0.01 {
		addTrigger(
			"repeated_losing_windows",
			fmt.Sprintf("The monitored run chain has now logged %d negative windows and %.2f cumulative net PnL across %d closed deals.", assessment.ObservedNegativeWindows, assessment.ObservedNetPnL, assessment.ObservedClosedDeals),
		)
	}
	if source.ClosedDeals >= 2 && current.ClosedDeals >= 2 {
		if current.NetPnL < source.NetPnL-0.15 &&
			current.AvgPnL < source.AvgPnL-0.05 &&
			current.WinRate+10 < source.WinRate {
			addTrigger(
				"target_cohort_degradation",
				fmt.Sprintf("The monitored cohort degraded versus the pre-apply baseline: net PnL %.2f -> %.2f, avg PnL %.2f -> %.2f, win rate %.1f%% -> %.1f%%.", source.NetPnL, current.NetPnL, source.AvgPnL, current.AvgPnL, source.WinRate, current.WinRate),
			)
		}
		if source.AvgExitEfficiencyScore > 0 &&
			current.AvgExitEfficiencyScore+15 < source.AvgExitEfficiencyScore &&
			current.AvgProfitGivenBackPct > source.AvgProfitGivenBackPct+15 &&
			current.NetPnL <= source.NetPnL {
			addTrigger(
				"trade_quality_drop",
				fmt.Sprintf("Exit quality fell sharply after apply: exit efficiency %.1f -> %.1f while avg give-back rose %.1f%% -> %.1f%%.", source.AvgExitEfficiencyScore, current.AvgExitEfficiencyScore, source.AvgProfitGivenBackPct, current.AvgProfitGivenBackPct),
			)
		}
		drawdownThreshold := math.Max(source.MaxDrawdownPct*1.5, source.MaxDrawdownPct+2.5)
		if drawdownThreshold < 3 {
			drawdownThreshold = 3
		}
		if current.MaxDrawdownPct > drawdownThreshold && (current.NetPnL < -0.01 || current.AvgPnL < source.AvgPnL) {
			addTrigger(
				"loss_tail_spike",
				fmt.Sprintf("Drawdown spiked from %.2f%% to %.2f%% in the monitored window.", source.MaxDrawdownPct, current.MaxDrawdownPct),
			)
		}
	}
	sourceStallCount := source.HoldDecisionCount + source.WaitDecisionCount
	currentStallCount := current.HoldDecisionCount + current.WaitDecisionCount
	requiredCandidates := int(math.Max(40, float64(source.DecisionCandidateCount)*0.8))
	if source.DecisionCandidateCount >= 40 &&
		source.OpenDecisionCount >= 2 &&
		current.DecisionRecordCount >= autonomousOptimizerMinCyclesForLowTradeReview &&
		current.DecisionCandidateCount >= requiredCandidates &&
		current.OpenDecisionCount*2 <= source.OpenDecisionCount &&
		current.DecisionConversionRate <= math.Max(0.1, source.DecisionConversionRate*0.55) &&
		currentStallCount >= int(math.Max(float64(sourceStallCount+4), float64(sourceStallCount)*1.3)) &&
		current.NetPnL <= 0 {
		addTrigger(
			"inactivity_drift",
			fmt.Sprintf("Trade activity drifted sharply worse after apply: open decisions %d -> %d, conversion %.2f%% -> %.2f%%, stall decisions %d -> %d, despite %d candidates in the latest window.", source.OpenDecisionCount, current.OpenDecisionCount, source.DecisionConversionRate, current.DecisionConversionRate, sourceStallCount, currentStallCount, current.DecisionCandidateCount),
		)
	}

	assessment.TriggerCodes = dedupeSortedStrings(assessment.TriggerCodes)
	assessment.TriggerReasons = dedupeSortedStrings(assessment.TriggerReasons)
	assessment.ShouldRollback = len(assessment.TriggerCodes) > 0
	return assessment
}

func restoreAutonomousOptimizerTraderSnapshot(traderCfg *store.Trader, snapshot map[string]any) {
	if traderCfg == nil || len(snapshot) == 0 {
		return
	}
	if value, ok := snapshot["custom_prompt"].(string); ok {
		traderCfg.CustomPrompt = value
	}
	if value, ok := snapshot["system_prompt_template"].(string); ok {
		traderCfg.SystemPromptTemplate = value
	}
	if value, ok := snapshot["override_base_prompt"].(bool); ok {
		traderCfg.OverrideBasePrompt = value
	}
}

func filterAutonomousOptimizerRunsExcluding(runs []*store.AutonomousOptimizerRun, excludedID string) []*store.AutonomousOptimizerRun {
	if len(runs) == 0 {
		return nil
	}
	filtered := make([]*store.AutonomousOptimizerRun, 0, len(runs))
	for _, item := range runs {
		if item == nil || item.ID == excludedID {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func countConsecutiveAutonomousApplies(runs []*store.AutonomousOptimizerRun) int {
	count := 0
	for _, item := range runs {
		if item == nil {
			continue
		}
		switch item.Status {
		case store.AutonomousOptimizerStatusAutoApplied, store.AutonomousOptimizerStatusMonitoring, store.AutonomousOptimizerStatusRollbackPending:
			count++
		default:
			return count
		}
	}
	return count
}

func autonomousOptimizerCooldownEnd(cfg *store.AutonomousOptimizerConfig, runs []*store.AutonomousOptimizerRun, now time.Time) (time.Time, string) {
	if cfg == nil || cfg.AutoApplyCooldownHours <= 0 {
		return time.Time{}, ""
	}
	for _, item := range runs {
		if item == nil {
			continue
		}
		switch item.Status {
		case store.AutonomousOptimizerStatusAutoApplied, store.AutonomousOptimizerStatusRolledBack:
			baseTime := item.CompletedAt.UTC()
			if baseTime.IsZero() {
				baseTime = item.UpdatedAt.UTC()
			}
			if baseTime.IsZero() {
				baseTime = item.CreatedAt.UTC()
			}
			if baseTime.IsZero() {
				return time.Time{}, item.ID
			}
			cooldownEnd := baseTime.Add(time.Duration(cfg.AutoApplyCooldownHours) * time.Hour)
			if cooldownEnd.After(now) {
				return cooldownEnd, item.ID
			}
			return time.Time{}, item.ID
		}
	}
	return time.Time{}, ""
}

func autonomousOptimizerBacklogMergeKey(category, title string) string {
	return strings.TrimSpace(strings.ToLower(category)) + "::" + strings.TrimSpace(strings.ToLower(title))
}

func mergeAutonomousOptimizerBacklogEvidenceJSON(currentJSON, incomingJSON string) string {
	current := parseAutonomousOptimizerJSONArray(currentJSON)
	incoming := parseAutonomousOptimizerJSONArray(incomingJSON)
	if len(current) == 0 && len(incoming) == 0 {
		return "[]"
	}
	out := make([]map[string]any, 0, len(current)+len(incoming))
	seen := map[string]struct{}{}
	appendItems := func(items []map[string]any) {
		for _, item := range items {
			raw, _ := json.Marshal(item)
			key := string(raw)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, item)
		}
	}
	appendItems(current)
	appendItems(incoming)
	if raw, err := json.Marshal(out); err == nil {
		return string(raw)
	}
	return "[]"
}

func mergeAutonomousOptimizerBacklogMetadataJSON(currentJSON, incomingJSON string) string {
	current := parseAutonomousOptimizerJSONObject(currentJSON)
	incoming := parseAutonomousOptimizerJSONObject(incomingJSON)
	if len(current) == 0 && len(incoming) == 0 {
		return "{}"
	}
	for key, value := range incoming {
		current[key] = value
	}
	if raw, err := json.Marshal(current); err == nil {
		return string(raw)
	}
	return "{}"
}

func maxAutonomousOptimizerValue(left, right float64) float64 {
	if right > left {
		return right
	}
	return left
}

func coalesceAutonomousOptimizerSummary(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func shouldSelfPauseAutonomousOptimizer(status, userID, traderID, currentRunID string, st *store.Store) bool {
	if status != store.AutonomousOptimizerStatusInsufficientEvidence && status != store.AutonomousOptimizerStatusFailed {
		return false
	}
	runs, err := st.AutonomousOptimizer().ListRuns(userID, traderID, 3)
	if err != nil || len(runs) < 3 {
		return false
	}
	if runs[0] == nil || runs[0].ID != currentRunID {
		return false
	}
	for _, item := range runs[:3] {
		if item == nil {
			return false
		}
		switch item.Status {
		case store.AutonomousOptimizerStatusInsufficientEvidence, store.AutonomousOptimizerStatusFailed:
		default:
			return false
		}
	}
	return true
}
