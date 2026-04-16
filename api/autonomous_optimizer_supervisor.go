package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nofx/logger"
	"nofx/store"
)

type autonomousOptimizerRunMetadata struct {
	WindowStartMs           int64   `json:"window_start_ms"`
	WindowEndMs             int64   `json:"window_end_ms"`
	ReviewIntervalHours     int     `json:"review_interval_hours"`
	ClosedDeals             int64   `json:"closed_deals"`
	OpenDeals               int64   `json:"open_deals"`
	WinningDeals            int64   `json:"winning_deals"`
	LosingDeals             int64   `json:"losing_deals"`
	NetPnL                  float64 `json:"net_pnl"`
	WinRate                 float64 `json:"win_rate"`
	DecisionRecordCount     int     `json:"decision_record_count"`
	DecisionCandidateCount  int     `json:"decision_candidate_count"`
	CyclesWithCandidates    int     `json:"cycles_with_candidates"`
	CyclesWithOpenDecisions int     `json:"cycles_with_open_decisions"`
	SelfPaused              bool    `json:"self_paused,omitempty"`
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
	configs, err := s.store.AutonomousOptimizer().ListDueConfigs(now, 25)
	if err != nil {
		return err
	}
	for i := range configs {
		cfg := configs[i]
		if err := s.processAutonomousOptimizerConfig(&cfg, now); err != nil {
			logger.Warnf("⚠️ Failed to process autonomous optimizer config %s for trader %s: %v", cfg.ID, cfg.TraderID, err)
		}
	}
	return nil
}

func (s *Server) processAutonomousOptimizerConfig(cfg *store.AutonomousOptimizerConfig, now time.Time) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	cfg.Status = store.AutonomousOptimizerStatusRunning
	if err := s.store.AutonomousOptimizer().SaveConfig(cfg); err != nil {
		return err
	}

	run := &store.AutonomousOptimizerRun{
		UserID:               cfg.UserID,
		TraderID:             cfg.TraderID,
		ConfigID:             cfg.ID,
		Trigger:              store.AutonomousOptimizerRunTriggerSchedule,
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
		return err
	}

	status, summary, metadata, err := s.evaluateAutonomousOptimizerWindow(cfg, now)
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

	run.Status = status
	run.Summary = summary
	run.CompletedAt = time.Now().UTC()
	if raw, marshalErr := json.Marshal(metadata); marshalErr == nil {
		run.MetadataJSON = string(raw)
	}
	if err := s.store.AutonomousOptimizer().SaveRun(run); err != nil {
		return err
	}

	cfg.LastRunID = run.ID
	cfg.LastRunAt = run.CompletedAt
	cfg.NextRunAt = run.CompletedAt.Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)
	cfg.Status = status

	if cfg.SelfPauseEnabled && shouldSelfPauseAutonomousOptimizer(status, cfg.UserID, cfg.TraderID, run.ID, s.store) {
		cfg.Status = store.AutonomousOptimizerStatusPaused
		metadata.SelfPaused = true
		run.Status = store.AutonomousOptimizerStatusPaused
		run.Summary = strings.TrimSpace(run.Summary + " Optimizer paused itself after repeated low-evidence or failed windows.")
		if raw, marshalErr := json.Marshal(metadata); marshalErr == nil {
			run.MetadataJSON = string(raw)
		}
		if err := s.store.AutonomousOptimizer().SaveRun(run); err != nil {
			return err
		}
	}

	return s.store.AutonomousOptimizer().SaveConfig(cfg)
}

func (s *Server) evaluateAutonomousOptimizerWindow(cfg *store.AutonomousOptimizerConfig, now time.Time) (string, string, autonomousOptimizerRunMetadata, error) {
	window := time.Duration(cfg.ReviewIntervalHours) * time.Hour
	if window <= 0 {
		window = time.Duration(store.AutonomousOptimizerDefaultReviewIntervalHours) * time.Hour
	}
	windowStart := now.Add(-window)
	if !cfg.LastRunAt.IsZero() && cfg.LastRunAt.After(windowStart) {
		windowStart = cfg.LastRunAt
	}

	filter := store.DealReviewListFilter{
		TraderID: cfg.TraderID,
		FromTime: windowStart.UnixMilli(),
		ToTime:   now.UnixMilli(),
		Limit:    500,
		Offset:   0,
	}
	_, summary, _, err := s.store.DealReview().ListCases(cfg.UserID, filter)
	if err != nil {
		return "", "", autonomousOptimizerRunMetadata{}, err
	}
	if summary == nil {
		summary = &store.DealReviewDatasetSummary{}
	}

	bucketReview, err := s.store.Decision().GetBucketReview(cfg.TraderID, window, 50)
	if err != nil {
		return "", "", autonomousOptimizerRunMetadata{}, err
	}
	metadata := autonomousOptimizerRunMetadata{
		WindowStartMs:           windowStart.UnixMilli(),
		WindowEndMs:             now.UnixMilli(),
		ReviewIntervalHours:     cfg.ReviewIntervalHours,
		ClosedDeals:             summary.ClosedDeals,
		OpenDeals:               summary.OpenDeals,
		WinningDeals:            summary.WinningDeals,
		LosingDeals:             summary.LosingDeals,
		NetPnL:                  summary.NetPnL,
		WinRate:                 summary.WinRate,
		DecisionRecordCount:     bucketReview.RecordCount,
		DecisionCandidateCount:  bucketReview.TotalCandidates,
		CyclesWithCandidates:    bucketReview.CyclesWithCandidates,
		CyclesWithOpenDecisions: bucketReview.CyclesWithOpenDecisions,
	}

	switch {
	case summary.ClosedDeals >= 3:
		return store.AutonomousOptimizerStatusNoChange,
			fmt.Sprintf("Window recorded %d closed deals and %d decision cycles. Scheduler processed the cohort and kept the current config unchanged.", summary.ClosedDeals, bucketReview.RecordCount),
			metadata,
			nil
	case bucketReview.RecordCount > 0:
		return store.AutonomousOptimizerStatusInsufficientEvidence,
			fmt.Sprintf("Only %d closed deals in the last %dh, but %d decision cycles and %d candidates were observed. No autonomous patch was applied.", summary.ClosedDeals, cfg.ReviewIntervalHours, bucketReview.RecordCount, bucketReview.TotalCandidates),
			metadata,
			nil
	default:
		return store.AutonomousOptimizerStatusInsufficientEvidence,
			fmt.Sprintf("No closed deals or candidate-cycle evidence were recorded in the last %dh. No autonomous patch was applied.", cfg.ReviewIntervalHours),
			metadata,
			nil
	}
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
