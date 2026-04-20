package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func TestDealReviewSymbolBehaviorPriorsRebuildAndMatch(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-symbol-priors.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-symbol-priors",
		UserID:         "user-symbol-priors",
		Name:           "Symbol Prior Trader",
		AIModelID:      "model-symbol-priors",
		ExchangeID:     "exchange-symbol-priors",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	now := time.Now().UTC()
	cases := []DealReviewCase{
		{
			ID:                       "case-rave-1",
			UserID:                   trader.UserID,
			TraderID:                 trader.ID,
			PositionID:               1,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "loss",
			OpenSelectionBucket:      "breakout",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high",
			OpenOIRegime:             "rising",
			RealizedPnL:              -1.4,
			RealizedPnLPct:           -2.5,
			MaxFavorableExcursionPct: 0.8,
			MaxAdverseExcursionPct:   -3.2,
			ProfitGivenBackPct:       0,
			HoldDurationMs:           int64((18 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-3 * time.Hour).UnixMilli(),
			CloseReason:              "stop_loss",
			ExitOrigin:               DealReviewExitOriginSyncedTriggerOrder,
		},
		{
			ID:                       "case-rave-2",
			UserID:                   trader.UserID,
			TraderID:                 trader.ID,
			PositionID:               2,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "loss",
			OpenSelectionBucket:      "breakout",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high",
			OpenOIRegime:             "rising",
			RealizedPnL:              -0.8,
			RealizedPnLPct:           -1.3,
			MaxFavorableExcursionPct: 0.5,
			MaxAdverseExcursionPct:   -1.6,
			ProfitGivenBackPct:       0,
			HoldDurationMs:           int64((12 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-2 * time.Hour).UnixMilli(),
			CloseReason:              "stop_loss",
			ExitOrigin:               DealReviewExitOriginSyncedTriggerOrder,
		},
		{
			ID:                       "case-rave-3",
			UserID:                   trader.UserID,
			TraderID:                 trader.ID,
			PositionID:               3,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "loss",
			OpenSelectionBucket:      "breakout",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high",
			OpenOIRegime:             "rising",
			RealizedPnL:              -1.1,
			RealizedPnLPct:           -1.9,
			MaxFavorableExcursionPct: 0.6,
			MaxAdverseExcursionPct:   -2.1,
			ProfitGivenBackPct:       0,
			HoldDurationMs:           int64((14 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-90 * time.Minute).UnixMilli(),
			CloseReason:              "stop_loss",
			ExitOrigin:               DealReviewExitOriginSyncedTriggerOrder,
		},
		{
			ID:                       "case-rave-4",
			UserID:                   trader.UserID,
			TraderID:                 trader.ID,
			PositionID:               4,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "profit",
			OpenSelectionBucket:      "breakout",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high",
			OpenOIRegime:             "rising",
			RealizedPnL:              0.2,
			RealizedPnLPct:           0.4,
			MaxFavorableExcursionPct: 1.1,
			MaxAdverseExcursionPct:   -0.7,
			ProfitGivenBackPct:       42,
			HoldDurationMs:           int64((11 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-60 * time.Minute).UnixMilli(),
			CloseReason:              "manual_exit",
			ExitOrigin:               DealReviewExitOriginAIDecision,
		},
		{
			ID:                       "case-rave-5",
			UserID:                   trader.UserID,
			TraderID:                 trader.ID,
			PositionID:               5,
			Symbol:                   "RAVEUSDT",
			Side:                     "LONG",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "loss",
			OpenSelectionBucket:      "breakout",
			OpenTrendRegime:          "uptrend",
			OpenVolatilityRegime:     "high",
			OpenOIRegime:             "rising",
			RealizedPnL:              -0.6,
			RealizedPnLPct:           -0.9,
			MaxFavorableExcursionPct: 0.3,
			MaxAdverseExcursionPct:   -1.2,
			ProfitGivenBackPct:       0,
			HoldDurationMs:           int64((9 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-20 * time.Minute).UnixMilli(),
			CloseReason:              "stop_loss",
			ExitOrigin:               DealReviewExitOriginSyncedTriggerOrder,
		},
		{
			ID:                       "case-other",
			UserID:                   trader.UserID,
			TraderID:                 trader.ID,
			PositionID:               6,
			Symbol:                   "TONUSDT",
			Side:                     "SHORT",
			Status:                   DealReviewCaseStatusClosed,
			Outcome:                  "profit",
			OpenSelectionBucket:      "mean_revert",
			OpenTrendRegime:          "sideways",
			OpenVolatilityRegime:     "medium",
			OpenOIRegime:             "flat",
			RealizedPnL:              1.1,
			RealizedPnLPct:           2.2,
			MaxFavorableExcursionPct: 3.0,
			MaxAdverseExcursionPct:   -0.9,
			ProfitGivenBackPct:       5,
			HoldDurationMs:           int64((30 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-10 * time.Minute).UnixMilli(),
			CloseReason:              "take_profit",
			ExitOrigin:               DealReviewExitOriginSyncedTriggerOrder,
		},
	}
	for i := range cases {
		if err := root.gdb.Create(&cases[i]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", i, err)
		}
	}

	decisionRecords := []*DecisionRecord{
		{
			TraderID:         trader.ID,
			CycleNumber:      1001,
			Timestamp:        now.Add(-95 * time.Minute).UTC(),
			CandidateMetaVer: DecisionCandidateMetadataVersion,
			CandidateDetails: []CandidateDetail{
				{
					Symbol:          "RAVEUSDT",
					Sources:         []string{"ai500", "oi_top"},
					SelectionBucket: "breakout",
					MarketContext: &DealReviewMarketContextSnapshot{
						TrendRegime:      "uptrend",
						VolatilityRegime: "high",
						OIRegime:         "rising",
					},
				},
			},
			Decisions: []DecisionAction{
				{
					Action:     "open_long",
					Symbol:     "RAVEUSDT",
					Confidence: 78,
					Reasoning:  "oi_up, ema_align, rel_strength, fresh",
					MarketContext: &DealReviewMarketContextSnapshot{
						TrendRegime:      "uptrend",
						VolatilityRegime: "high",
						OIRegime:         "rising",
					},
					Execution: &DecisionExecutionTelemetry{
						TerminalStatus: "submitted",
					},
				},
			},
		},
		{
			TraderID:         trader.ID,
			CycleNumber:      1002,
			Timestamp:        now.Add(-70 * time.Minute).UTC(),
			CandidateMetaVer: DecisionCandidateMetadataVersion,
			CandidateDetails: []CandidateDetail{
				{
					Symbol:          "RAVEUSDT",
					Sources:         []string{"ai500"},
					SelectionBucket: "breakout",
					MarketContext: &DealReviewMarketContextSnapshot{
						TrendRegime:      "uptrend",
						VolatilityRegime: "high",
						OIRegime:         "rising",
					},
				},
			},
			Decisions: []DecisionAction{
				{
					Action:     "open_long",
					Symbol:     "RAVEUSDT",
					Confidence: 74,
					Reasoning:  "oi_up_price_up, ema_align, fresh",
					MarketContext: &DealReviewMarketContextSnapshot{
						TrendRegime:      "uptrend",
						VolatilityRegime: "high",
						OIRegime:         "rising",
					},
					Execution: &DecisionExecutionTelemetry{
						TerminalStatus: "filled",
					},
				},
			},
		},
	}
	for idx, record := range decisionRecords {
		candidateDetailsJSON, _ := json.Marshal(record.CandidateDetails)
		decisionsJSON, _ := json.Marshal(record.Decisions)
		if err := root.gdb.Create(&DecisionRecordDB{
			TraderID:         record.TraderID,
			CycleNumber:      record.CycleNumber,
			Timestamp:        record.Timestamp,
			CandidateDetails: string(candidateDetailsJSON),
			CandidateMetaVer: record.CandidateMetaVer,
			Decisions:        string(decisionsJSON),
			Success:          true,
		}).Error; err != nil {
			t.Fatalf("Create(decision record %d) error = %v", idx, err)
		}
	}

	refresh, err := root.DealReview().RebuildSymbolBehaviorPriors(trader.UserID, trader.ID)
	if err != nil {
		t.Fatalf("RebuildSymbolBehaviorPriors() error = %v", err)
	}
	if refresh.ItemCount != 1 {
		t.Fatalf("refresh.ItemCount = %d, want 1", refresh.ItemCount)
	}

	priors, err := root.DealReview().ListSymbolBehaviorPriors(trader.UserID, trader.ID, DealReviewSymbolBehaviorPriorFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListSymbolBehaviorPriors() error = %v", err)
	}
	if len(priors) != 1 {
		t.Fatalf("len(priors) = %d, want 1", len(priors))
	}
	prior := priors[0]
	if prior.Symbol != "RAVEUSDT" {
		t.Fatalf("prior.Symbol = %q, want RAVEUSDT", prior.Symbol)
	}
	if prior.Status != DealReviewSymbolBehaviorPriorStatusCandidate {
		t.Fatalf("prior.Status = %q, want candidate", prior.Status)
	}
	if prior.BehaviorBias != DealReviewSymbolBehaviorBiasNegative {
		t.Fatalf("prior.BehaviorBias = %q, want negative", prior.BehaviorBias)
	}
	if prior.RecommendedAction != "penalize_setup" {
		t.Fatalf("prior.RecommendedAction = %q, want penalize_setup", prior.RecommendedAction)
	}
	if prior.SampleCount != 5 {
		t.Fatalf("prior.SampleCount = %d, want 5", prior.SampleCount)
	}
	if len(prior.Evidence) != 5 {
		t.Fatalf("len(prior.Evidence) = %d, want 5", len(prior.Evidence))
	}
	if prior.DecisionOpenCount != 2 {
		t.Fatalf("prior.DecisionOpenCount = %d, want 2", prior.DecisionOpenCount)
	}
	if prior.DecisionCycleCount != 2 {
		t.Fatalf("prior.DecisionCycleCount = %d, want 2", prior.DecisionCycleCount)
	}
	if prior.AvgDecisionConfidence < 70 {
		t.Fatalf("prior.AvgDecisionConfidence = %.2f, want >= 70", prior.AvgDecisionConfidence)
	}
	if prior.ContradictionScore <= 0 {
		t.Fatalf("prior.ContradictionScore = %.4f, want > 0", prior.ContradictionScore)
	}
	if len(prior.DecisionEvidence) != 2 {
		t.Fatalf("len(prior.DecisionEvidence) = %d, want 2", len(prior.DecisionEvidence))
	}
	if len(prior.SignalTags) == 0 {
		t.Fatalf("prior.SignalTags = %#v, want non-empty", prior.SignalTags)
	}
	if prior.SignalClusterKey == "" {
		t.Fatalf("prior.SignalClusterKey = %q, want non-empty", prior.SignalClusterKey)
	}
	if len(prior.SignalClusters) == 0 {
		t.Fatalf("prior.SignalClusters = %#v, want non-empty", prior.SignalClusters)
	}
	if !containsString(prior.SignalClusters, "bucket_breakout") {
		t.Fatalf("prior.SignalClusters = %#v, want bucket_breakout", prior.SignalClusters)
	}
	if !containsString(prior.SignalClusters, "src_ai500") {
		t.Fatalf("prior.SignalClusters = %#v, want src_ai500", prior.SignalClusters)
	}
	if prior.AvgPnLPct >= 0 {
		t.Fatalf("prior.AvgPnLPct = %.4f, want negative", prior.AvgPnLPct)
	}
	if prior.GiveBackRate <= 0 {
		t.Fatalf("prior.GiveBackRate = %.4f, want > 0", prior.GiveBackRate)
	}
	if prior.DecisionEvidence[0].SignalClusterKey == "" {
		t.Fatalf("decision evidence cluster key = %q, want non-empty", prior.DecisionEvidence[0].SignalClusterKey)
	}
	if len(prior.DecisionEvidence[0].SignalClusters) == 0 {
		t.Fatalf("decision evidence signal clusters = %#v, want non-empty", prior.DecisionEvidence[0].SignalClusters)
	}
	clusterFiltered, err := root.DealReview().ListSymbolBehaviorPriors(trader.UserID, trader.ID, DealReviewSymbolBehaviorPriorFilter{
		SignalCluster: "bucket_breakout",
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("ListSymbolBehaviorPriors(cluster filter) error = %v", err)
	}
	if len(clusterFiltered) != 1 || clusterFiltered[0].ID != prior.ID {
		t.Fatalf("clusterFiltered = %#v, want only %q", clusterFiltered, prior.ID)
	}
	clusterMiss, err := root.DealReview().ListSymbolBehaviorPriors(trader.UserID, trader.ID, DealReviewSymbolBehaviorPriorFilter{
		SignalCluster: "ctx_rsi_oversold",
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("ListSymbolBehaviorPriors(cluster miss) error = %v", err)
	}
	if len(clusterMiss) != 0 {
		t.Fatalf("clusterMiss = %#v, want empty", clusterMiss)
	}

	match, err := root.DealReview().ListMatchingSymbolBehaviorPriors(trader.UserID, trader.ID, &DealReviewCase{
		Symbol:               "RAVEUSDT",
		Side:                 "LONG",
		OpenSelectionBucket:  "breakout",
		OpenTrendRegime:      "uptrend",
		OpenVolatilityRegime: "high",
		OpenOIRegime:         "rising",
	}, 3)
	if err != nil {
		t.Fatalf("ListMatchingSymbolBehaviorPriors() error = %v", err)
	}
	if len(match) != 1 {
		t.Fatalf("len(match) = %d, want 1", len(match))
	}
	if match[0].MatchScore < 0.8 {
		t.Fatalf("match[0].MatchScore = %.4f, want >= 0.8", match[0].MatchScore)
	}

	anomalies, err := root.DealReview().GetAnomalySummary(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("GetAnomalySummary() error = %v", err)
	}
	if len(anomalies.SymbolEdgeFailures) == 0 {
		t.Fatalf("SymbolEdgeFailures = %#v, want non-empty", anomalies.SymbolEdgeFailures)
	}
	if anomalies.SymbolEdgeFailures[0].Symbol != "RAVEUSDT" {
		t.Fatalf("SymbolEdgeFailures[0].Symbol = %q, want RAVEUSDT", anomalies.SymbolEdgeFailures[0].Symbol)
	}
	if anomalies.SymbolEdgeFailures[0].SliceDeals != 5 {
		t.Fatalf("SymbolEdgeFailures[0].SliceDeals = %d, want 5", anomalies.SymbolEdgeFailures[0].SliceDeals)
	}
	if anomalies.SymbolEdgeFailures[0].DecisionOpenCount != 2 {
		t.Fatalf("SymbolEdgeFailures[0].DecisionOpenCount = %d, want 2", anomalies.SymbolEdgeFailures[0].DecisionOpenCount)
	}
	if len(anomalies.Notes) == 0 {
		t.Fatalf("anomaly notes = %#v, want non-empty", anomalies.Notes)
	}
}

func TestDealReviewSymbolBehaviorPriorsValidationLifecycle(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-symbol-priors-validation.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-symbol-priors-lifecycle",
		UserID:         "user-symbol-priors-lifecycle",
		Name:           "Symbol Prior Lifecycle Trader",
		AIModelID:      "model-symbol-priors-lifecycle",
		ExchangeID:     "exchange-symbol-priors-lifecycle",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	base := time.Now().UTC().Add(-120 * time.Hour)
	addCase := func(id string, positionID int64, symbol string, pnl, pnlPct float64, exitOffset time.Duration) DealReviewCase {
		outcome := "flat"
		switch {
		case pnl > 0:
			outcome = "profit"
		case pnl < 0:
			outcome = "loss"
		}
		return DealReviewCase{
			ID:                   id,
			UserID:               trader.UserID,
			TraderID:             trader.ID,
			PositionID:           positionID,
			Symbol:               symbol,
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              outcome,
			OpenSelectionBucket:  "breakout",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "high",
			OpenOIRegime:         "rising",
			RealizedPnL:          pnl,
			RealizedPnLPct:       pnlPct,
			ExitTimeMs:           base.Add(exitOffset).UnixMilli(),
		}
	}

	var cases []DealReviewCase
	positionID := int64(1000)
	appendSeries := func(symbol string, values []struct {
		pnl    float64
		pnlPct float64
		offset time.Duration
	}) {
		for idx, item := range values {
			positionID++
			cases = append(cases, addCase(
				symbol+"-"+time.Now().UTC().Add(time.Duration(idx)*time.Millisecond).Format("150405.000000000"),
				positionID,
				symbol,
				item.pnl,
				item.pnlPct,
				item.offset,
			))
		}
	}

	appendSeries("VALIDUSDT", []struct {
		pnl    float64
		pnlPct float64
		offset time.Duration
	}{
		{-1.5, -1.8, 1 * time.Hour},
		{-1.2, -1.4, 2 * time.Hour},
		{-1.1, -1.2, 3 * time.Hour},
		{-0.9, -1.0, 4 * time.Hour},
		{-1.0, -1.1, 5 * time.Hour},
		{-0.8, -0.9, 6 * time.Hour},
		{-0.7, -0.8, 7 * time.Hour},
		{-0.6, -0.7, 8 * time.Hour},
		{-0.5, -0.6, 9 * time.Hour},
	})
	appendSeries("REJECTUSDT", []struct {
		pnl    float64
		pnlPct float64
		offset time.Duration
	}{
		{-1.8, -2.0, 30 * time.Hour},
		{-1.3, -1.4, 31 * time.Hour},
		{-1.0, -1.1, 32 * time.Hour},
		{0.9, 1.0, 33 * time.Hour},
		{0.8, 0.9, 34 * time.Hour},
	})
	appendSeries("EXPIREUSDT", []struct {
		pnl    float64
		pnlPct float64
		offset time.Duration
	}{
		{-1.6, -1.8, 50 * time.Hour},
		{-1.3, -1.5, 51 * time.Hour},
		{-1.1, -1.2, 52 * time.Hour},
		{-1.0, -1.1, 53 * time.Hour},
		{-0.8, -0.9, 54 * time.Hour},
		{-0.7, -0.8, 55 * time.Hour},
		{-0.6, -0.7, 56 * time.Hour},
		{0.6, 0.7, 57 * time.Hour},
		{0.7, 0.8, 58 * time.Hour},
	})

	for idx := range cases {
		if err := root.gdb.Create(&cases[idx]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", idx, err)
		}
	}

	refresh, err := root.DealReview().RebuildSymbolBehaviorPriors(trader.UserID, trader.ID)
	if err != nil {
		t.Fatalf("RebuildSymbolBehaviorPriors() error = %v", err)
	}
	if refresh.ItemCount != 3 {
		t.Fatalf("refresh.ItemCount = %d, want 3", refresh.ItemCount)
	}

	priors, err := root.DealReview().ListSymbolBehaviorPriors(trader.UserID, trader.ID, DealReviewSymbolBehaviorPriorFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListSymbolBehaviorPriors() error = %v", err)
	}
	if len(priors) != 3 {
		t.Fatalf("len(priors) = %d, want 3", len(priors))
	}

	bySymbol := make(map[string]DealReviewSymbolBehaviorPrior, len(priors))
	for _, prior := range priors {
		bySymbol[prior.Symbol] = prior
	}

	validated := bySymbol["VALIDUSDT"]
	if validated.Status != DealReviewSymbolBehaviorPriorStatusValidated {
		t.Fatalf("VALIDUSDT status = %q, want validated", validated.Status)
	}
	if validated.ValidationSampleCount != 2 || validated.ValidationSupportCount != 2 {
		t.Fatalf("VALIDUSDT validation counts = %d/%d, want 2/2", validated.ValidationSupportCount, validated.ValidationSampleCount)
	}
	if validated.RecentSampleCount != 2 || validated.RecentSupportCount != 2 {
		t.Fatalf("VALIDUSDT recent counts = %d/%d, want 2/2", validated.RecentSupportCount, validated.RecentSampleCount)
	}
	if validated.ValidationSupportScore < 0.8 {
		t.Fatalf("VALIDUSDT validation support score = %.4f, want >= 0.8", validated.ValidationSupportScore)
	}
	if validated.DriftScore > 0.25 {
		t.Fatalf("VALIDUSDT drift score = %.4f, want <= 0.25", validated.DriftScore)
	}
	if !strings.Contains(validated.ValidationSummary, "Holdout supported") {
		t.Fatalf("VALIDUSDT validation summary = %q, want holdout supported note", validated.ValidationSummary)
	}
	if validated.ValidationLabel != DealReviewSymbolBehaviorValidationLabelConfirmed {
		t.Fatalf("VALIDUSDT validation label = %q, want confirmed", validated.ValidationLabel)
	}
	if validated.FalsePositiveScore >= 0.35 {
		t.Fatalf("VALIDUSDT false positive score = %.4f, want < 0.35", validated.FalsePositiveScore)
	}

	rejected := bySymbol["REJECTUSDT"]
	if rejected.Status != DealReviewSymbolBehaviorPriorStatusRejected {
		t.Fatalf("REJECTUSDT status = %q, want rejected", rejected.Status)
	}
	if rejected.ValidationSampleCount != 2 || rejected.ValidationContradictCount != 2 {
		t.Fatalf("REJECTUSDT validation contradict counts = %d/%d, want 2/2", rejected.ValidationContradictCount, rejected.ValidationSampleCount)
	}
	if rejected.ValidationSupportScore > 0.2 {
		t.Fatalf("REJECTUSDT validation support score = %.4f, want <= 0.2", rejected.ValidationSupportScore)
	}
	if !strings.Contains(rejected.ValidationSummary, "rejected") {
		t.Fatalf("REJECTUSDT validation summary = %q, want rejected note", rejected.ValidationSummary)
	}
	if rejected.ValidationLabel != DealReviewSymbolBehaviorValidationLabelFalsePositive {
		t.Fatalf("REJECTUSDT validation label = %q, want false_positive", rejected.ValidationLabel)
	}
	if rejected.FalsePositiveScore < 0.75 {
		t.Fatalf("REJECTUSDT false positive score = %.4f, want >= 0.75", rejected.FalsePositiveScore)
	}

	expired := bySymbol["EXPIREUSDT"]
	if expired.Status != DealReviewSymbolBehaviorPriorStatusExpired {
		t.Fatalf("EXPIREUSDT status = %q, want expired", expired.Status)
	}
	if expired.ValidationSampleCount != 2 || expired.ValidationSupportCount != 2 {
		t.Fatalf("EXPIREUSDT validation support counts = %d/%d, want 2/2", expired.ValidationSupportCount, expired.ValidationSampleCount)
	}
	if expired.RecentSampleCount != 2 || expired.RecentContradictCount != 2 {
		t.Fatalf("EXPIREUSDT recent contradict counts = %d/%d, want 2/2", expired.RecentContradictCount, expired.RecentSampleCount)
	}
	if expired.DriftScore < 0.5 {
		t.Fatalf("EXPIREUSDT drift score = %.4f, want >= 0.5", expired.DriftScore)
	}
	if !strings.Contains(expired.ValidationSummary, "expired") {
		t.Fatalf("EXPIREUSDT validation summary = %q, want expired note", expired.ValidationSummary)
	}
	if expired.ValidationLabel != DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk {
		t.Fatalf("EXPIREUSDT validation label = %q, want false_negative_risk", expired.ValidationLabel)
	}
	if expired.FalseNegativeScore < 0.6 {
		t.Fatalf("EXPIREUSDT false negative score = %.4f, want >= 0.6", expired.FalseNegativeScore)
	}

	summary := BuildDealReviewSymbolBehaviorPriorReportingSummary(priors)
	if summary == nil {
		t.Fatal("BuildDealReviewSymbolBehaviorPriorReportingSummary() returned nil summary")
	}
	if summary.TotalCount != 3 {
		t.Fatalf("summary.TotalCount = %d, want 3", summary.TotalCount)
	}
	if summary.ConfirmedCount != 1 {
		t.Fatalf("summary.ConfirmedCount = %d, want 1", summary.ConfirmedCount)
	}
	if summary.FalsePositiveCount != 1 {
		t.Fatalf("summary.FalsePositiveCount = %d, want 1", summary.FalsePositiveCount)
	}
	if summary.FalseNegativeRiskCount != 1 {
		t.Fatalf("summary.FalseNegativeRiskCount = %d, want 1", summary.FalseNegativeRiskCount)
	}
	if len(summary.TopFalsePositives) != 1 || summary.TopFalsePositives[0].Symbol != "REJECTUSDT" {
		t.Fatalf("summary.TopFalsePositives = %#v, want REJECTUSDT", summary.TopFalsePositives)
	}
	foundExpiredRisk := false
	for _, item := range summary.TopFalseNegativeRisks {
		if item.Symbol == "EXPIREUSDT" {
			foundExpiredRisk = true
			break
		}
	}
	if !foundExpiredRisk {
		t.Fatalf("summary.TopFalseNegativeRisks = %#v, want EXPIREUSDT included", summary.TopFalseNegativeRisks)
	}
}

func TestBuildDealReviewSymbolBehaviorPriorReportingSummary_IncludesClusterRollups(t *testing.T) {
	priors := []DealReviewSymbolBehaviorPrior{
		{
			ID:                     "prior-a",
			Symbol:                 "RAVEUSDT",
			Side:                   "LONG",
			Status:                 DealReviewSymbolBehaviorPriorStatusValidated,
			BehaviorBias:           DealReviewSymbolBehaviorBiasNegative,
			ValidationLabel:        DealReviewSymbolBehaviorValidationLabelFalsePositive,
			SampleCount:            9,
			DecisionOpenCount:      4,
			AvgPnLPct:              -1.4,
			CompositeScore:         0.78,
			ContradictionScore:     0.72,
			ValidationSupportScore: 0.68,
			SignalClusters:         []string{"bucket_breakout", "src_ai500"},
		},
		{
			ID:                     "prior-b",
			Symbol:                 "TONUSDT",
			Side:                   "LONG",
			Status:                 DealReviewSymbolBehaviorPriorStatusValidated,
			BehaviorBias:           DealReviewSymbolBehaviorBiasNegative,
			ValidationLabel:        DealReviewSymbolBehaviorValidationLabelConfirmed,
			SampleCount:            7,
			DecisionOpenCount:      3,
			AvgPnLPct:              -0.8,
			CompositeScore:         0.71,
			ContradictionScore:     0.64,
			ValidationSupportScore: 0.61,
			SignalClusters:         []string{"bucket_breakout", "ctx_rsi_hot"},
		},
		{
			ID:                     "prior-c",
			Symbol:                 "LINKUSDT",
			Side:                   "LONG",
			Status:                 DealReviewSymbolBehaviorPriorStatusCandidate,
			BehaviorBias:           DealReviewSymbolBehaviorBiasPositive,
			ValidationLabel:        DealReviewSymbolBehaviorValidationLabelDrifting,
			SampleCount:            5,
			DecisionOpenCount:      2,
			AvgPnLPct:              0.4,
			CompositeScore:         0.52,
			ContradictionScore:     0.18,
			ValidationSupportScore: 0.43,
			SignalClusters:         []string{"ctx_rsi_hot"},
		},
	}

	summary := BuildDealReviewSymbolBehaviorPriorReportingSummary(priors)
	if summary == nil {
		t.Fatal("BuildDealReviewSymbolBehaviorPriorReportingSummary() returned nil summary")
	}
	if len(summary.TopSignalClusters) == 0 {
		t.Fatalf("summary.TopSignalClusters = %#v, want non-empty", summary.TopSignalClusters)
	}
	if summary.TopSignalClusters[0].ClusterLabel != "bucket_breakout" {
		t.Fatalf("top cluster = %#v, want bucket_breakout first", summary.TopSignalClusters[0])
	}
	if summary.TopSignalClusters[0].PriorCount != 2 {
		t.Fatalf("bucket_breakout prior count = %d, want 2", summary.TopSignalClusters[0].PriorCount)
	}
	if summary.TopSignalClusters[0].SymbolCount != 2 {
		t.Fatalf("bucket_breakout symbol count = %d, want 2", summary.TopSignalClusters[0].SymbolCount)
	}
	if !containsString(summary.TopSignalClusters[0].TopSymbols, "RAVEUSDT") {
		t.Fatalf("bucket_breakout top symbols = %#v, want RAVEUSDT", summary.TopSignalClusters[0].TopSymbols)
	}
	foundClusterNote := false
	for _, note := range summary.Notes {
		if strings.Contains(note, "Top normalized cluster") && strings.Contains(note, "bucket_breakout") {
			foundClusterNote = true
			break
		}
	}
	if !foundClusterNote {
		t.Fatalf("summary.Notes = %#v, want top normalized cluster note", summary.Notes)
	}
}

func TestDealReviewSymbolBehaviorPriorsInitAddsLegacyColumns(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-symbol-priors-legacy.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	if err := gdb.Exec(`
		CREATE TABLE deal_review_symbol_behavior_priors (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			trader_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			status TEXT DEFAULT 'observed',
			behavior_bias TEXT DEFAULT 'mixed',
			recommended_action TEXT DEFAULT 'observe',
			open_selection_bucket TEXT DEFAULT '',
			open_trend_regime TEXT DEFAULT '',
			open_volatility_regime TEXT DEFAULT '',
			open_oi_regime TEXT DEFAULT '',
			regime_signature TEXT DEFAULT '',
			sample_count INTEGER DEFAULT 0,
			winning_deals INTEGER DEFAULT 0,
			losing_deals INTEGER DEFAULT 0,
			flat_deals INTEGER DEFAULT 0,
			win_rate REAL DEFAULT 0,
			loss_rate REAL DEFAULT 0,
			net_pnl REAL DEFAULT 0,
			avg_pnl REAL DEFAULT 0,
			avg_pnl_pct REAL DEFAULT 0,
			expectancy REAL DEFAULT 0,
			avg_mfe_pct REAL DEFAULT 0,
			avg_mae_pct REAL DEFAULT 0,
			give_back_rate REAL DEFAULT 0,
			avg_give_back_pct REAL DEFAULT 0,
			avg_hold_ms INTEGER DEFAULT 0,
			confidence_score REAL DEFAULT 0,
			recency_weight REAL DEFAULT 0,
			stability_score REAL DEFAULT 0,
			composite_score REAL DEFAULT 0,
			summary TEXT DEFAULT '',
			evidence_json TEXT DEFAULT '[]',
			first_observed_at DATETIME,
			last_observed_at DATETIME,
			built_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create legacy table error = %v", err)
	}

	store := NewDealReviewStore(gdb)
	if err := store.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	requiredFields := []string{
		"DecisionCycleCount",
		"DecisionOpenCount",
		"AvgDecisionConfidence",
		"TrainingSampleCount",
		"ValidationSampleCount",
		"ValidationSupportCount",
		"ValidationContradictCount",
		"ValidationAvgPnLPct",
		"ValidationSupportScore",
		"RecentSampleCount",
		"RecentSupportCount",
		"RecentContradictCount",
		"RecentAvgPnLPct",
		"RecentSupportScore",
		"DriftScore",
		"ContradictionScore",
		"ValidationSummary",
		"SignalClusterKey",
		"SignalTagsJSON",
		"SignalClustersJSON",
		"DecisionEvidenceJSON",
	}
	for _, fieldName := range requiredFields {
		if !gdb.Migrator().HasColumn(&DealReviewSymbolBehaviorPrior{}, fieldName) {
			t.Fatalf("expected %s column to exist after initTables()", fieldName)
		}
	}
}

func TestDealReviewSymbolBehaviorPriorsRefreshHandlesMissingDecisionHistory(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-symbol-priors-refresh-null-max.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-symbol-priors-refresh",
		UserID:         "user-symbol-priors-refresh",
		Name:           "Symbol Prior Refresh Trader",
		AIModelID:      "model-symbol-priors-refresh",
		ExchangeID:     "exchange-symbol-priors-refresh",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	caseRec := &DealReviewCase{
		ID:                   "case-refresh-null-max",
		UserID:               trader.UserID,
		TraderID:             trader.ID,
		PositionID:           101,
		Symbol:               "NULLMAXUSDT",
		Side:                 "LONG",
		Status:               DealReviewCaseStatusClosed,
		Outcome:              "loss",
		OpenSelectionBucket:  "breakout",
		OpenTrendRegime:      "uptrend",
		OpenVolatilityRegime: "high",
		OpenOIRegime:         "rising",
		RealizedPnL:          -0.5,
		RealizedPnLPct:       -1.2,
		ExitTimeMs:           time.Now().UTC().Add(-30 * time.Minute).UnixMilli(),
	}
	if err := root.gdb.Create(caseRec).Error; err != nil {
		t.Fatalf("Create(case) error = %v", err)
	}

	refresh, err := root.DealReview().RefreshSymbolBehaviorPriorsIfStale(trader.UserID, trader.ID)
	if err != nil {
		t.Fatalf("RefreshSymbolBehaviorPriorsIfStale() error = %v", err)
	}
	if refresh == nil {
		t.Fatal("RefreshSymbolBehaviorPriorsIfStale() returned nil result")
	}
	if !refresh.Rebuilt {
		t.Fatalf("refresh.Rebuilt = %v, want true", refresh.Rebuilt)
	}
	if refresh.ItemCount != 0 {
		t.Fatalf("refresh.ItemCount = %d, want 0", refresh.ItemCount)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}
