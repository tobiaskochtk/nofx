package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func TestDealReviewLearnedPatternsRebuildAndMatch(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-learned-patterns.db"))
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
		ID:             "trader-learned-patterns",
		UserID:         "user-learned-patterns",
		Name:           "Learned Pattern Trader",
		AIModelID:      "model-learned-patterns",
		ExchangeID:     "exchange-learned-patterns",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}
	companionTrader := &Trader{
		ID:             "trader-learned-patterns-companion",
		UserID:         trader.UserID,
		Name:           "Learned Pattern Companion Trader",
		AIModelID:      "model-learned-patterns",
		ExchangeID:     "exchange-learned-patterns",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(companionTrader); err != nil {
		t.Fatalf("Trader().Create(companion) error = %v", err)
	}

	now := time.Now().UTC()
	cases := []DealReviewCase{
		{
			ID:                   "case-pos-1",
			UserID:               trader.UserID,
			TraderID:             trader.ID,
			PositionID:           1,
			Symbol:               "ALPHAUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "profit",
			OpenSelectionBucket:  "momentum",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "medium",
			OpenOIRegime:         "rising",
			OpenSessionBucket:    "asia",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_1",
			OpenLiquidityTier:    "deep",
			OpenSpreadBucket:     "tight",
			OpenSlippageBucket:   "low",
			OpenConfidence:       84,
			PlannedRiskPct:       0.85,
			OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
				"ai500",
				"oi_top",
			}),
			RealizedPnL:              1.2,
			RealizedPnLPct:           1.8,
			MaxFavorableExcursionPct: 2.5,
			MaxAdverseExcursionPct:   -0.4,
			ProfitGivenBackPct:       12,
			HoldDurationMs:           int64((22 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-6 * time.Hour).UnixMilli(),
		},
		{
			ID:                   "case-pos-2",
			UserID:               trader.UserID,
			TraderID:             trader.ID,
			PositionID:           2,
			Symbol:               "BETAUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "profit",
			OpenSelectionBucket:  "momentum",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "medium",
			OpenOIRegime:         "rising",
			OpenSessionBucket:    "asia",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_1",
			OpenLiquidityTier:    "deep",
			OpenSpreadBucket:     "tight",
			OpenSlippageBucket:   "low",
			OpenConfidence:       86,
			PlannedRiskPct:       0.90,
			OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
				"ai500",
				"oi_top",
			}),
			RealizedPnL:              1.0,
			RealizedPnLPct:           1.5,
			MaxFavorableExcursionPct: 2.0,
			MaxAdverseExcursionPct:   -0.5,
			ProfitGivenBackPct:       8,
			HoldDurationMs:           int64((18 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-4 * time.Hour).UnixMilli(),
		},
		{
			ID:                   "case-pos-3",
			UserID:               trader.UserID,
			TraderID:             trader.ID,
			PositionID:           3,
			Symbol:               "TONUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "profit",
			OpenSelectionBucket:  "momentum",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "medium",
			OpenOIRegime:         "rising",
			OpenSessionBucket:    "asia",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_1",
			OpenLiquidityTier:    "deep",
			OpenSpreadBucket:     "tight",
			OpenSlippageBucket:   "low",
			OpenConfidence:       82,
			PlannedRiskPct:       0.80,
			OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
				"ai500",
				"oi_top",
			}),
			RealizedPnL:              0.9,
			RealizedPnLPct:           1.2,
			MaxFavorableExcursionPct: 1.9,
			MaxAdverseExcursionPct:   -0.3,
			ProfitGivenBackPct:       5,
			HoldDurationMs:           int64((16 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-2 * time.Hour).UnixMilli(),
		},
		{
			ID:                   "case-neg-1",
			UserID:               trader.UserID,
			TraderID:             trader.ID,
			PositionID:           4,
			Symbol:               "RAVEUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "loss",
			OpenSelectionBucket:  "breakout",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "high",
			OpenOIRegime:         "flat",
			OpenSessionBucket:    "us_open",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_2",
			OpenLiquidityTier:    "thin",
			OpenSpreadBucket:     "wide",
			OpenSlippageBucket:   "high",
			OpenConfidence:       69,
			PlannedRiskPct:       1.70,
			OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
				"ai500",
				"breakout",
			}),
			RealizedPnL:              -1.1,
			RealizedPnLPct:           -1.7,
			MaxFavorableExcursionPct: 0.4,
			MaxAdverseExcursionPct:   -2.2,
			ProfitGivenBackPct:       0,
			HoldDurationMs:           int64((11 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-5 * time.Hour).UnixMilli(),
		},
		{
			ID:                   "case-neg-2",
			UserID:               trader.UserID,
			TraderID:             trader.ID,
			PositionID:           5,
			Symbol:               "RAVEUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "loss",
			OpenSelectionBucket:  "breakout",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "high",
			OpenOIRegime:         "flat",
			OpenSessionBucket:    "us_open",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_2",
			OpenLiquidityTier:    "thin",
			OpenSpreadBucket:     "wide",
			OpenSlippageBucket:   "high",
			OpenConfidence:       72,
			PlannedRiskPct:       1.60,
			OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
				"ai500",
				"breakout",
			}),
			RealizedPnL:              -0.8,
			RealizedPnLPct:           -1.3,
			MaxFavorableExcursionPct: 0.5,
			MaxAdverseExcursionPct:   -1.8,
			ProfitGivenBackPct:       0,
			HoldDurationMs:           int64((14 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-3 * time.Hour).UnixMilli(),
		},
		{
			ID:                   "case-neg-3",
			UserID:               trader.UserID,
			TraderID:             trader.ID,
			PositionID:           6,
			Symbol:               "RAVEUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "loss",
			OpenSelectionBucket:  "breakout",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "high",
			OpenOIRegime:         "flat",
			OpenSessionBucket:    "us_open",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_2",
			OpenLiquidityTier:    "thin",
			OpenSpreadBucket:     "wide",
			OpenSlippageBucket:   "high",
			OpenConfidence:       70,
			PlannedRiskPct:       1.80,
			OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
				"ai500",
				"breakout",
			}),
			RealizedPnL:              -1.4,
			RealizedPnLPct:           -2.1,
			MaxFavorableExcursionPct: 0.3,
			MaxAdverseExcursionPct:   -2.6,
			ProfitGivenBackPct:       0,
			HoldDurationMs:           int64((9 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-1 * time.Hour).UnixMilli(),
		},
		{
			ID:                   "case-global-pos-1",
			UserID:               trader.UserID,
			TraderID:             companionTrader.ID,
			PositionID:           101,
			Symbol:               "OMEGAUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "profit",
			OpenSelectionBucket:  "momentum",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "medium",
			OpenOIRegime:         "rising",
			OpenSessionBucket:    "asia",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_1",
			OpenLiquidityTier:    "deep",
			OpenSpreadBucket:     "tight",
			OpenSlippageBucket:   "low",
			OpenConfidence:       88,
			PlannedRiskPct:       0.82,
			OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
				"ai500",
				"oi_top",
			}),
			RealizedPnL:              1.3,
			RealizedPnLPct:           1.9,
			MaxFavorableExcursionPct: 2.6,
			MaxAdverseExcursionPct:   -0.4,
			ProfitGivenBackPct:       7,
			HoldDurationMs:           int64((21 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-90 * time.Minute).UnixMilli(),
		},
		{
			ID:                   "case-global-pos-2",
			UserID:               trader.UserID,
			TraderID:             companionTrader.ID,
			PositionID:           102,
			Symbol:               "SIGMAUSDT",
			Side:                 "LONG",
			Status:               DealReviewCaseStatusClosed,
			Outcome:              "profit",
			OpenSelectionBucket:  "momentum",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "medium",
			OpenOIRegime:         "rising",
			OpenSessionBucket:    "asia",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_1",
			OpenLiquidityTier:    "deep",
			OpenSpreadBucket:     "tight",
			OpenSlippageBucket:   "low",
			OpenConfidence:       85,
			PlannedRiskPct:       0.78,
			OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
				"ai500",
				"oi_top",
			}),
			RealizedPnL:              1.1,
			RealizedPnLPct:           1.6,
			MaxFavorableExcursionPct: 2.1,
			MaxAdverseExcursionPct:   -0.5,
			ProfitGivenBackPct:       6,
			HoldDurationMs:           int64((19 * time.Minute) / time.Millisecond),
			ExitTimeMs:               now.Add(-70 * time.Minute).UnixMilli(),
		},
	}

	for idx := range cases {
		if err := root.gdb.Create(&cases[idx]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", idx, err)
		}
	}
	currentTraderCaseCount := 0
	for _, item := range cases {
		if item.TraderID == trader.ID {
			currentTraderCaseCount++
		}
	}

	refresh, err := root.DealReview().RefreshLearnedPatternsIfStale(trader.UserID, trader.ID)
	if err != nil {
		t.Fatalf("RefreshLearnedPatternsIfStale() error = %v", err)
	}
	if refresh == nil {
		t.Fatal("RefreshLearnedPatternsIfStale() returned nil result")
	}
	if !refresh.Rebuilt {
		t.Fatalf("refresh.Rebuilt = %v, want true", refresh.Rebuilt)
	}
	if refresh.FeatureCount != currentTraderCaseCount {
		t.Fatalf("refresh.FeatureCount = %d, want %d", refresh.FeatureCount, currentTraderCaseCount)
	}
	if refresh.PatternCount == 0 {
		t.Fatal("refresh.PatternCount = 0, want > 0")
	}

	var featureCount int64
	if err := root.gdb.Model(&DealReviewPatternFeatureRecord{}).
		Where("user_id = ? AND trader_id = ?", trader.UserID, trader.ID).
		Count(&featureCount).Error; err != nil {
		t.Fatalf("Count(feature rows) error = %v", err)
	}
	if int(featureCount) != currentTraderCaseCount {
		t.Fatalf("feature row count = %d, want %d", featureCount, currentTraderCaseCount)
	}

	var evidenceCount int64
	if err := root.gdb.Model(&DealReviewPatternEvidenceRecord{}).
		Where("user_id = ? AND trader_id = ?", trader.UserID, trader.ID).
		Count(&evidenceCount).Error; err != nil {
		t.Fatalf("Count(evidence rows) error = %v", err)
	}
	if evidenceCount == 0 {
		t.Fatal("evidence row count = 0, want > 0")
	}

	var validationRuns []DealReviewPatternValidationRun
	if err := root.gdb.Model(&DealReviewPatternValidationRun{}).
		Where("user_id = ? AND trader_id = ?", trader.UserID, trader.ID).
		Order("built_at DESC").
		Find(&validationRuns).Error; err != nil {
		t.Fatalf("Find(validation runs) error = %v", err)
	}
	if len(validationRuns) != 1 {
		t.Fatalf("validation run count = %d, want 1", len(validationRuns))
	}
	if validationRuns[0].FeatureCount != currentTraderCaseCount {
		t.Fatalf("validationRuns[0].FeatureCount = %d, want %d", validationRuns[0].FeatureCount, currentTraderCaseCount)
	}
	if validationRuns[0].EvidenceCount != int(evidenceCount) {
		t.Fatalf("validationRuns[0].EvidenceCount = %d, want %d", validationRuns[0].EvidenceCount, evidenceCount)
	}
	if validationRuns[0].PatternCount != refresh.PatternCount {
		t.Fatalf("validationRuns[0].PatternCount = %d, want %d", validationRuns[0].PatternCount, refresh.PatternCount)
	}

	var backlogCount int64
	if err := root.gdb.Model(&DealReviewPatternBacklogCandidate{}).
		Where("user_id = ? AND trader_id = ?", trader.UserID, trader.ID).
		Count(&backlogCount).Error; err != nil {
		t.Fatalf("Count(backlog candidates) error = %v", err)
	}
	if backlogCount == 0 {
		t.Fatal("backlog candidate count = 0, want > 0")
	}

	positivePatterns, err := root.DealReview().ListLearnedPatterns(trader.UserID, trader.ID, DealReviewLearnedPatternFilter{
		Side:    "LONG",
		Feature: "bucket_momentum",
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("ListLearnedPatterns(positive) error = %v", err)
	}
	if len(positivePatterns) == 0 {
		t.Fatal("ListLearnedPatterns(positive) returned 0 items, want > 0")
	}

	foundTraderLocalPositive := false
	traderLocalMomentumSampleCount := 0
	foundTraderLocalTriplePositive := false
	for _, pattern := range positivePatterns {
		if pattern.ScopeType == DealReviewLearnedPatternScopeTraderLocal &&
			pattern.PatternClass == DealReviewLearnedPatternClassPositiveEdge {
			foundTraderLocalPositive = true
			if pattern.SampleCount > traderLocalMomentumSampleCount {
				traderLocalMomentumSampleCount = pattern.SampleCount
			}
			if pattern.PatternOrder == 3 {
				foundTraderLocalTriplePositive = true
			}
		}
	}
	if !foundTraderLocalPositive {
		t.Fatal("expected at least one trader-local positive learned pattern for bucket_momentum")
	}
	if !foundTraderLocalTriplePositive {
		t.Fatal("expected at least one trader-local positive triple pattern for bucket_momentum")
	}
	if len(positivePatterns[0].Evidence) == 0 {
		t.Fatal("expected persisted evidence rows to hydrate onto learned patterns")
	}

	regimeFiltered, err := root.DealReview().ListLearnedPatterns(trader.UserID, trader.ID, DealReviewLearnedPatternFilter{
		Side:           "LONG",
		Regime:         "trend_uptrend",
		MinConfidence:  0.10,
		MinSampleCount: 3,
		Limit:          100,
	})
	if err != nil {
		t.Fatalf("ListLearnedPatterns(regime/confidence/sample) error = %v", err)
	}
	if len(regimeFiltered) == 0 {
		t.Fatal("ListLearnedPatterns(regime/confidence/sample) returned 0 items, want > 0")
	}
	foundRegimeLocal := false
	for _, pattern := range regimeFiltered {
		if pattern.ScopeType == DealReviewLearnedPatternScopeRegimeLocal && pattern.ScopeKey != "" {
			foundRegimeLocal = true
			break
		}
	}
	if !foundRegimeLocal {
		t.Fatal("expected at least one regime-local learned pattern with a non-empty scope key")
	}

	globalPatterns, err := root.DealReview().ListLearnedPatterns(trader.UserID, trader.ID, DealReviewLearnedPatternFilter{
		Side:      "LONG",
		ScopeType: DealReviewLearnedPatternScopeGlobal,
		Feature:   "bucket_momentum",
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("ListLearnedPatterns(global) error = %v", err)
	}
	if len(globalPatterns) == 0 {
		t.Fatal("ListLearnedPatterns(global) returned 0 items, want > 0")
	}
	foundGlobalPositive := false
	foundExpandedGlobalSample := false
	for _, pattern := range globalPatterns {
		if pattern.PatternClass != DealReviewLearnedPatternClassPositiveEdge {
			continue
		}
		foundGlobalPositive = true
		if pattern.SampleCount > traderLocalMomentumSampleCount {
			foundExpandedGlobalSample = true
		}
	}
	if !foundGlobalPositive {
		t.Fatal("expected at least one global positive learned pattern for bucket_momentum")
	}
	if !foundExpandedGlobalSample {
		t.Fatal("expected a global momentum pattern to use a larger user-wide sample than the trader-local scope")
	}

	negativePatterns, err := root.DealReview().ListLearnedPatterns(trader.UserID, trader.ID, DealReviewLearnedPatternFilter{
		Symbol:  "RAVEUSDT",
		Side:    "LONG",
		Feature: "bucket_breakout",
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("ListLearnedPatterns(negative) error = %v", err)
	}
	if len(negativePatterns) == 0 {
		t.Fatal("ListLearnedPatterns(negative) returned 0 items, want > 0")
	}

	foundRAVESymbolNegative := false
	for _, pattern := range negativePatterns {
		if pattern.ScopeType == DealReviewLearnedPatternScopeSymbol &&
			pattern.Symbol == "RAVEUSDT" &&
			pattern.PatternClass == DealReviewLearnedPatternClassNegativeEdge {
			foundRAVESymbolNegative = true
		}
	}
	if !foundRAVESymbolNegative {
		t.Fatal("expected a RAVEUSDT symbol-scope negative learned pattern for bucket_breakout")
	}

	matches, err := root.DealReview().ListMatchingLearnedPatterns(trader.UserID, trader.ID, &DealReviewCase{
		Symbol:               "RAVEUSDT",
		Side:                 "LONG",
		OpenSelectionBucket:  "breakout",
		OpenTrendRegime:      "uptrend",
		OpenVolatilityRegime: "high",
		OpenOIRegime:         "flat",
		OpenSessionBucket:    "us_open",
		OpenWeekdayBucket:    "monday",
		OpenVenueTier:        "tier_2",
		OpenLiquidityTier:    "thin",
		OpenSpreadBucket:     "wide",
		OpenSlippageBucket:   "high",
		OpenConfidence:       71,
		PlannedRiskPct:       1.75,
		OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
			"ai500",
			"breakout",
		}),
	}, 10)
	if err != nil {
		t.Fatalf("ListMatchingLearnedPatterns() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("ListMatchingLearnedPatterns() returned 0 matches, want > 0")
	}

	foundMatchedRAVENegative := false
	for _, pattern := range matches {
		if pattern.ScopeType == DealReviewLearnedPatternScopeSymbol &&
			pattern.Symbol == "RAVEUSDT" &&
			pattern.PatternClass == DealReviewLearnedPatternClassNegativeEdge &&
			pattern.MatchScore > 0 {
			foundMatchedRAVENegative = true
		}
	}
	if !foundMatchedRAVENegative {
		t.Fatal("expected a matched negative RAVEUSDT symbol pattern for the breakout setup")
	}

	regimeLocalNegativePatterns, err := root.DealReview().ListLearnedPatterns(trader.UserID, trader.ID, DealReviewLearnedPatternFilter{
		Side:      "LONG",
		ScopeType: DealReviewLearnedPatternScopeRegimeLocal,
		Feature:   "bucket_breakout",
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("ListLearnedPatterns(regime_local negative) error = %v", err)
	}
	if len(regimeLocalNegativePatterns) == 0 {
		t.Fatal("expected regime-local negative patterns for breakout setup")
	}
	foundMatchedRegimeLocal := false
	for _, pattern := range regimeLocalNegativePatterns {
		if pattern.PatternClass != DealReviewLearnedPatternClassNegativeEdge {
			continue
		}
		matched, score := MatchDealReviewLearnedPattern(&DealReviewCase{
			Symbol:               "RAVEUSDT",
			Side:                 "LONG",
			OpenSelectionBucket:  "breakout",
			OpenTrendRegime:      "uptrend",
			OpenVolatilityRegime: "high",
			OpenOIRegime:         "flat",
			OpenSessionBucket:    "us_open",
			OpenWeekdayBucket:    "monday",
			OpenVenueTier:        "tier_2",
			OpenLiquidityTier:    "thin",
			OpenSpreadBucket:     "wide",
			OpenSlippageBucket:   "high",
			OpenConfidence:       71,
			PlannedRiskPct:       1.75,
			OpenCandidateSourcesJSON: marshalDealReviewStringArray([]string{
				"ai500",
				"breakout",
			}),
		}, &pattern)
		if matched && score > 0 {
			foundMatchedRegimeLocal = true
			break
		}
	}
	if !foundMatchedRegimeLocal {
		t.Fatal("expected a regime-local negative pattern to match the breakout setup")
	}
}

func TestRefreshLearnedPatternsIfStale_UsesValidationRunWhenNoPatternsExist(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-learned-patterns-empty.db"))
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
		ID:             "trader-learned-patterns-no-patterns",
		UserID:         "user-learned-patterns-no-patterns",
		Name:           "No Pattern Trader",
		AIModelID:      "model-learned-patterns",
		ExchangeID:     "exchange-learned-patterns",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	now := time.Now().UTC()
	cases := []DealReviewCase{
		{
			ID:                "case-mixed-1",
			UserID:            trader.UserID,
			TraderID:          trader.ID,
			PositionID:        1,
			Symbol:            "MIXUSDT",
			Side:              "LONG",
			Status:            DealReviewCaseStatusClosed,
			Outcome:           "profit",
			OpenConfidence:    80,
			RealizedPnL:       1.0,
			RealizedPnLPct:    1.1,
			HoldDurationMs:    int64((10 * time.Minute) / time.Millisecond),
			ExitTimeMs:        now.Add(-3 * time.Hour).UnixMilli(),
			OpenSessionBucket: "asia",
		},
		{
			ID:                "case-mixed-2",
			UserID:            trader.UserID,
			TraderID:          trader.ID,
			PositionID:        2,
			Symbol:            "MIXUSDT",
			Side:              "LONG",
			Status:            DealReviewCaseStatusClosed,
			Outcome:           "loss",
			OpenConfidence:    80,
			RealizedPnL:       -1.0,
			RealizedPnLPct:    -1.1,
			HoldDurationMs:    int64((11 * time.Minute) / time.Millisecond),
			ExitTimeMs:        now.Add(-2 * time.Hour).UnixMilli(),
			OpenSessionBucket: "us_open",
		},
	}
	for idx := range cases {
		if err := root.gdb.Create(&cases[idx]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", idx, err)
		}
	}

	first, err := root.DealReview().RefreshLearnedPatternsIfStale(trader.UserID, trader.ID)
	if err != nil {
		t.Fatalf("first RefreshLearnedPatternsIfStale() error = %v", err)
	}
	if first == nil {
		t.Fatal("first RefreshLearnedPatternsIfStale() returned nil result")
	}
	if !first.Rebuilt {
		t.Fatalf("first.Rebuilt = %v, want true", first.Rebuilt)
	}
	if first.PatternCount != 0 {
		t.Fatalf("first.PatternCount = %d, want 0", first.PatternCount)
	}

	second, err := root.DealReview().RefreshLearnedPatternsIfStale(trader.UserID, trader.ID)
	if err != nil {
		t.Fatalf("second RefreshLearnedPatternsIfStale() error = %v", err)
	}
	if second == nil {
		t.Fatal("second RefreshLearnedPatternsIfStale() returned nil result")
	}
	if second.Rebuilt {
		t.Fatalf("second.Rebuilt = %v, want false", second.Rebuilt)
	}

	var validationRunCount int64
	if err := root.gdb.Model(&DealReviewPatternValidationRun{}).
		Where("user_id = ? AND trader_id = ?", trader.UserID, trader.ID).
		Count(&validationRunCount).Error; err != nil {
		t.Fatalf("Count(validation runs) error = %v", err)
	}
	if validationRunCount != 1 {
		t.Fatalf("validation run count = %d, want 1", validationRunCount)
	}
}

func TestBuildDealReviewLearnedPatternReportingSummary(t *testing.T) {
	items := []DealReviewLearnedPattern{
		{
			ID:               "positive-confirmed",
			ScopeType:        DealReviewLearnedPatternScopeTraderLocal,
			Side:             "LONG",
			PatternClass:     DealReviewLearnedPatternClassPositiveEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
			PatternSignature: "bucket:momentum + trend:uptrend",
			SampleCount:      8,
			CompositeScore:   0.82,
			LiftAvgPnLPct:    1.10,
		},
		{
			ID:               "negative-false-positive",
			ScopeType:        DealReviewLearnedPatternScopeTraderLocal,
			Side:             "LONG",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelFalsePositive,
			PatternSignature: "bucket:breakout",
			SampleCount:      6,
			CompositeScore:   0.60,
			LiftAvgPnLPct:    -0.75,
		},
		{
			ID:               "symbol-reverse-risk",
			ScopeType:        DealReviewLearnedPatternScopeSymbol,
			Symbol:           "RAVEUSDT",
			Side:             "LONG",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelReverseRisk,
			PatternSignature: "bucket:breakout + vol:high",
			SampleCount:      5,
			CompositeScore:   0.74,
			LiftAvgPnLPct:    -1.40,
		},
		{
			ID:               "symbol-drifting",
			ScopeType:        DealReviewLearnedPatternScopeSymbol,
			Symbol:           "TONUSDT",
			Side:             "SHORT",
			PatternClass:     DealReviewLearnedPatternClassPositiveEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelDrifting,
			PatternSignature: "bucket:mean_revert + session:asia",
			SampleCount:      4,
			CompositeScore:   0.57,
			LiftAvgPnLPct:    0.48,
		},
	}

	summary := BuildDealReviewLearnedPatternReportingSummary(items)
	if summary == nil {
		t.Fatal("BuildDealReviewLearnedPatternReportingSummary() returned nil summary")
	}
	if summary.TotalCount != 4 {
		t.Fatalf("summary.TotalCount = %d, want 4", summary.TotalCount)
	}
	if summary.PositiveCount != 2 {
		t.Fatalf("summary.PositiveCount = %d, want 2", summary.PositiveCount)
	}
	if summary.NegativeCount != 2 {
		t.Fatalf("summary.NegativeCount = %d, want 2", summary.NegativeCount)
	}
	if summary.ConfirmedCount != 1 {
		t.Fatalf("summary.ConfirmedCount = %d, want 1", summary.ConfirmedCount)
	}
	if summary.FalsePositiveCount != 1 {
		t.Fatalf("summary.FalsePositiveCount = %d, want 1", summary.FalsePositiveCount)
	}
	if summary.ReverseRiskCount != 1 {
		t.Fatalf("summary.ReverseRiskCount = %d, want 1", summary.ReverseRiskCount)
	}
	if summary.DriftingCount != 1 {
		t.Fatalf("summary.DriftingCount = %d, want 1", summary.DriftingCount)
	}
	if len(summary.TopPositivePatterns) == 0 {
		t.Fatal("summary.TopPositivePatterns = 0, want > 0")
	}
	if len(summary.TopNegativePatterns) == 0 {
		t.Fatal("summary.TopNegativePatterns = 0, want > 0")
	}
	if len(summary.TopSymbolOverrides) == 0 {
		t.Fatal("summary.TopSymbolOverrides = 0, want > 0")
	}
	if summary.TopSymbolOverrides[0].Symbol != "RAVEUSDT" {
		t.Fatalf("top symbol override = %q, want RAVEUSDT", summary.TopSymbolOverrides[0].Symbol)
	}
	if len(summary.Notes) == 0 {
		t.Fatal("summary.Notes = 0, want > 0")
	}
}

func TestBuildDealReviewLearnedPatternCombos_LimitsTriples(t *testing.T) {
	features := []string{
		"bucket:momentum",
		"trend:uptrend",
		"vol:medium",
		"oi:rising",
		"session:asia",
		"conf:high",
		"risk:normal",
		"btc:strong",
		"funding:neutral",
		"venue:tier_1",
	}

	combos := buildDealReviewLearnedPatternCombos(features)
	if len(combos) == 0 {
		t.Fatal("buildDealReviewLearnedPatternCombos() returned 0 combos, want > 0")
	}

	tripleCount := 0
	for _, combo := range combos {
		if len(combo) > 3 {
			t.Fatalf("combo length = %d, want <= 3", len(combo))
		}
		if len(combo) == 3 {
			tripleCount++
		}
	}
	if tripleCount == 0 {
		t.Fatal("expected limited triple combos to be generated")
	}
	if tripleCount > dealReviewLearnedPatternTripleComboLimit {
		t.Fatalf("tripleCount = %d, want <= %d", tripleCount, dealReviewLearnedPatternTripleComboLimit)
	}
}

func TestBuildDealReviewLearnedPatternRecommendedUse_MonitoringRule(t *testing.T) {
	use := buildDealReviewLearnedPatternRecommendedUse(
		DealReviewLearnedPatternClassNegativeEdge,
		DealReviewLearnedPatternValidationLabelConfirmed,
		0.91,
		0.94,
		0.82,
		0.04,
		0.05,
		12,
		0.88,
	)
	if use != DealReviewLearnedPatternRecommendedUseMonitoringRule {
		t.Fatalf("recommended use = %q, want %q", use, DealReviewLearnedPatternRecommendedUseMonitoringRule)
	}
}

func TestHydrateDealReviewLearnedPattern_UpgradesLegacyPromptHintToMonitoringRule(t *testing.T) {
	item := &DealReviewLearnedPattern{
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		ValidationLabel:        DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUsePromptHint,
		FeatureSetJSON:         `["bucket:breakout"]`,
		SampleCount:            10,
		CompositeScore:         0.90,
		ConfidenceScore:        0.93,
		ValidationSupportScore: 0.81,
		FalsePositiveScore:     0.03,
		DriftScore:             0.04,
		LiftAvgPnLPct:          -0.77,
		EvidenceJSON:           `[]`,
	}

	hydrateDealReviewLearnedPattern(item)

	if item.RecommendedUse != DealReviewLearnedPatternRecommendedUseMonitoringRule {
		t.Fatalf("item.RecommendedUse = %q, want %q", item.RecommendedUse, DealReviewLearnedPatternRecommendedUseMonitoringRule)
	}
	if len(item.FeatureSet) != 1 || item.FeatureSet[0] != "bucket:breakout" {
		t.Fatalf("item.FeatureSet = %#v, want hydrated features", item.FeatureSet)
	}
}

func TestBuildDealReviewLearnedPatternLifecycle_ActiveMonitoringRule(t *testing.T) {
	now := time.Now().UTC()
	pattern := &DealReviewLearnedPattern{
		ID:                     "pattern-active",
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		ValidationLabel:        DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitoringRule,
		ValidationSupportScore: 0.84,
		RecentSampleCount:      4,
		RecentSupportCount:     3,
		RecentSupportScore:     0.74,
		DriftScore:             0.05,
		RecencyWeight:          0.82,
		LastObservedAt:         now.Add(-6 * time.Hour),
	}

	lifecycle := buildDealReviewLearnedPatternLifecycle(pattern, dealReviewLearnedPatternLiveGuardPatternStats{
		PatternRef:              pattern.StableKey,
		TotalEvents:             2,
		MonitorOnlyCount:        2,
		LatestDecisionTimestamp: now,
	}, now)
	if lifecycle == nil {
		t.Fatal("lifecycle = nil, want active lifecycle")
	}
	if lifecycle.Status != DealReviewLearnedPatternLifecycleStatusActive {
		t.Fatalf("lifecycle.Status = %q, want %q", lifecycle.Status, DealReviewLearnedPatternLifecycleStatusActive)
	}
	if lifecycle.ExpiryScore >= 0.55 {
		t.Fatalf("lifecycle.ExpiryScore = %.2f, want healthy rule below warning threshold", lifecycle.ExpiryScore)
	}
}

func TestBuildDealReviewLearnedPatternLifecycle_RollbackWatch(t *testing.T) {
	now := time.Now().UTC()
	pattern := &DealReviewLearnedPattern{
		ID:                     "pattern-rollback",
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		ValidationLabel:        DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitoringRule,
		ValidationSupportScore: 0.38,
		RecentSampleCount:      2,
		RecentSupportCount:     0,
		RecentSupportScore:     0.12,
		DriftScore:             0.19,
		RecencyWeight:          0.18,
		LastObservedAt:         now.Add(-6 * 24 * time.Hour),
	}

	lifecycle := buildDealReviewLearnedPatternLifecycle(pattern, dealReviewLearnedPatternLiveGuardPatternStats{
		PatternRef:              pattern.StableKey,
		TotalEvents:             6,
		HardBlockedCount:        4,
		MonitorOnlyCount:        1,
		MatchedUnqualifiedCount: 1,
		LatestDecisionTimestamp: now,
	}, now)
	if lifecycle == nil {
		t.Fatal("lifecycle = nil, want rollback watch lifecycle")
	}
	if lifecycle.Status != DealReviewLearnedPatternLifecycleStatusRollbackWatch {
		t.Fatalf("lifecycle.Status = %q, want %q", lifecycle.Status, DealReviewLearnedPatternLifecycleStatusRollbackWatch)
	}
	if lifecycle.RollbackScore < 0.60 {
		t.Fatalf("lifecycle.RollbackScore = %.2f, want strong rollback pressure", lifecycle.RollbackScore)
	}
}

func TestBuildDealReviewLearnedPatternReportingSummary_LifecycleCounts(t *testing.T) {
	items := []DealReviewLearnedPattern{
		{
			ID:               "pattern-live-active",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitoringRule,
			PatternSignature: "bucket:breakout",
			SampleCount:      8,
			CompositeScore:   0.89,
			Lifecycle: &DealReviewLearnedPatternLifecycle{
				Status:      DealReviewLearnedPatternLifecycleStatusActive,
				ExpiryScore: 0.18,
			},
		},
		{
			ID:               "pattern-live-expiring",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelDrifting,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitoringRule,
			PatternSignature: "bucket:asia",
			SampleCount:      7,
			CompositeScore:   0.78,
			Lifecycle: &DealReviewLearnedPatternLifecycle{
				Status:      DealReviewLearnedPatternLifecycleStatusDegrading,
				ExpiryScore: 0.72,
			},
		},
		{
			ID:               "pattern-live-rollback",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitoringRule,
			PatternSignature: "bucket:risk",
			SampleCount:      10,
			CompositeScore:   0.84,
			Lifecycle: &DealReviewLearnedPatternLifecycle{
				Status:        DealReviewLearnedPatternLifecycleStatusRollbackWatch,
				RollbackScore: 0.81,
			},
		},
	}

	summary := BuildDealReviewLearnedPatternReportingSummary(items)
	if summary == nil {
		t.Fatal("summary = nil")
	}
	if summary.MonitoringRuleCount != 3 {
		t.Fatalf("summary.MonitoringRuleCount = %d, want 3", summary.MonitoringRuleCount)
	}
	if summary.LifecycleActiveCount != 1 || summary.LifecycleDegradingCount != 1 || summary.LifecycleRollbackWatchCount != 1 {
		t.Fatalf("lifecycle counts = %#v, want 1/1/1", summary)
	}
	if len(summary.TopExpiringMonitoringRules) != 1 || summary.TopExpiringMonitoringRules[0].ID != "pattern-live-expiring" {
		t.Fatalf("TopExpiringMonitoringRules = %#v, want pattern-live-expiring", summary.TopExpiringMonitoringRules)
	}
	if len(summary.TopRollbackWatchPatterns) != 1 || summary.TopRollbackWatchPatterns[0].ID != "pattern-live-rollback" {
		t.Fatalf("TopRollbackWatchPatterns = %#v, want pattern-live-rollback", summary.TopRollbackWatchPatterns)
	}
}

func TestBuildDealReviewLearnedPatternLifecycleTrend_ComputesDurationsAndFragility(t *testing.T) {
	base := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC)
	history := []DealReviewLearnedPatternLifecycleSnapshot{
		{
			ID:                          "snap-4",
			LifecycleStatus:             DealReviewLearnedPatternLifecycleStatusRollbackWatch,
			StatusChanged:               true,
			RecentGuardEventCount:       3,
			LastObservedToGuardLagHours: 96,
			CapturedAt:                  base.Add(36 * time.Hour),
		},
		{
			ID:                          "snap-3",
			LifecycleStatus:             DealReviewLearnedPatternLifecycleStatusDegrading,
			StatusChanged:               true,
			RecentGuardEventCount:       2,
			LastObservedToGuardLagHours: 80,
			CapturedAt:                  base.Add(24 * time.Hour),
		},
		{
			ID:                          "snap-2",
			LifecycleStatus:             DealReviewLearnedPatternLifecycleStatusActive,
			StatusChanged:               false,
			RecentGuardEventCount:       1,
			LastObservedToGuardLagHours: 24,
			CapturedAt:                  base.Add(12 * time.Hour),
		},
		{
			ID:                          "snap-1",
			LifecycleStatus:             DealReviewLearnedPatternLifecycleStatusActive,
			StatusChanged:               false,
			RecentGuardEventCount:       0,
			LastObservedToGuardLagHours: 0,
			CapturedAt:                  base,
		},
	}

	trend := buildDealReviewLearnedPatternLifecycleTrend(history)
	if trend == nil {
		t.Fatal("trend = nil")
	}
	if trend.SnapshotCount != 4 {
		t.Fatalf("SnapshotCount = %d, want 4", trend.SnapshotCount)
	}
	if trend.StatusChangeCount != 2 {
		t.Fatalf("StatusChangeCount = %d, want 2", trend.StatusChangeCount)
	}
	if trend.ActiveHours != 24 {
		t.Fatalf("ActiveHours = %.0f, want 24", trend.ActiveHours)
	}
	if trend.DegradingHours != 12 {
		t.Fatalf("DegradingHours = %.0f, want 12", trend.DegradingHours)
	}
	if trend.RollbackWatchHours != 0 {
		t.Fatalf("RollbackWatchHours = %.0f, want 0 for last open-ended segment", trend.RollbackWatchHours)
	}
	if trend.StaleGuardSnapshotCount != 2 {
		t.Fatalf("StaleGuardSnapshotCount = %d, want 2", trend.StaleGuardSnapshotCount)
	}
	if !trend.Fragile {
		t.Fatalf("Fragile = %v, want true", trend.Fragile)
	}
	if trend.LastStatusChangeAt != base.Add(36*time.Hour) {
		t.Fatalf("LastStatusChangeAt = %v, want %v", trend.LastStatusChangeAt, base.Add(36*time.Hour))
	}
	if trend.Summary == "" {
		t.Fatal("Summary is empty, want lifecycle trend summary text")
	}
}

func TestBuildDealReviewLearnedPatternReportingSummary_TracksFragileRules(t *testing.T) {
	items := []DealReviewLearnedPattern{
		{
			ID:               "pattern-fragile",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitoringRule,
			PatternSignature: "bucket:fragile",
			CompositeScore:   0.91,
			LifecycleTrend: &DealReviewLearnedPatternLifecycleTrend{
				Fragile:                 true,
				DegradingShare:          0.40,
				RollbackWatchShare:      0.25,
				StaleGuardSnapshotCount: 3,
				StaleGuardSnapshotShare: 0.50,
				StatusChangeCount:       2,
				Summary:                 "Lifecycle trend is fragile.",
			},
		},
		{
			ID:               "pattern-healthy",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitoringRule,
			PatternSignature: "bucket:healthy",
			CompositeScore:   0.89,
			LifecycleTrend: &DealReviewLearnedPatternLifecycleTrend{
				Fragile:           false,
				ActiveShare:       0.80,
				StatusChangeCount: 0,
				Summary:           "Lifecycle trend: mostly active.",
			},
		},
	}

	summary := BuildDealReviewLearnedPatternReportingSummary(items)
	if summary == nil {
		t.Fatal("summary = nil")
	}
	if summary.LifecycleFragileCount != 1 {
		t.Fatalf("LifecycleFragileCount = %d, want 1", summary.LifecycleFragileCount)
	}
	if summary.LifecycleLaggingGuardCount != 1 {
		t.Fatalf("LifecycleLaggingGuardCount = %d, want 1", summary.LifecycleLaggingGuardCount)
	}
	if len(summary.TopFragileMonitoringRules) != 1 || summary.TopFragileMonitoringRules[0].ID != "pattern-fragile" {
		t.Fatalf("TopFragileMonitoringRules = %#v, want pattern-fragile", summary.TopFragileMonitoringRules)
	}
}

func TestBuildDealReviewLearnedPatternLiveGuardAttributionRollup_ClassifiesOverblocking(t *testing.T) {
	base := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC)
	events := []DealReviewLearnedPatternLiveGuardEvent{
		{
			ID:                "guard-3",
			Effect:            DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified,
			DecisionTimestamp: base.Add(3 * time.Hour),
			Attribution: &DealReviewLearnedPatternLiveGuardEventAttribution{
				Status: DealReviewLearnedPatternLiveGuardAttributionStatusThresholdMissedProfit,
			},
		},
		{
			ID:                "guard-2",
			Effect:            DealReviewLearnedPatternLiveGuardEffectMonitorOnly,
			DecisionTimestamp: base.Add(2 * time.Hour),
			Attribution: &DealReviewLearnedPatternLiveGuardEventAttribution{
				Status: DealReviewLearnedPatternLiveGuardAttributionStatusWarningNotConfirmed,
			},
		},
		{
			ID:                "guard-1",
			Effect:            DealReviewLearnedPatternLiveGuardEffectHardBlocked,
			DecisionTimestamp: base.Add(1 * time.Hour),
			Attribution: &DealReviewLearnedPatternLiveGuardEventAttribution{
				Status: DealReviewLearnedPatternLiveGuardAttributionStatusOverblocked,
			},
		},
	}

	rollup := buildDealReviewLearnedPatternLiveGuardAttributionRollup(events)
	if rollup == nil {
		t.Fatal("rollup = nil")
	}
	if rollup.EventCount != 3 {
		t.Fatalf("EventCount = %d, want 3", rollup.EventCount)
	}
	if rollup.QualifiedEventCount != 2 {
		t.Fatalf("QualifiedEventCount = %d, want 2", rollup.QualifiedEventCount)
	}
	if rollup.ResolvedEventCount != 3 {
		t.Fatalf("ResolvedEventCount = %d, want 3", rollup.ResolvedEventCount)
	}
	if rollup.OverblockingEvidenceCount != 3 {
		t.Fatalf("OverblockingEvidenceCount = %d, want 3", rollup.OverblockingEvidenceCount)
	}
	if rollup.AttributionLabel != DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking {
		t.Fatalf(
			"AttributionLabel = %q, want %q",
			rollup.AttributionLabel,
			DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking,
		)
	}
	if rollup.OverblockingRate != 1 {
		t.Fatalf("OverblockingRate = %.2f, want 1", rollup.OverblockingRate)
	}
	if rollup.LatestEventAt != base.Add(3*time.Hour) {
		t.Fatalf("LatestEventAt = %v, want %v", rollup.LatestEventAt, base.Add(3*time.Hour))
	}
	if rollup.Summary == "" {
		t.Fatal("Summary is empty, want overblocking summary text")
	}
}

func TestBuildDealReviewLearnedPatternLiveGuardAttributionDelta_ClassifiesNewlyOverblocking(t *testing.T) {
	base := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC)
	events := []DealReviewLearnedPatternLiveGuardEvent{
		{
			ID:                "guard-recent-2",
			Effect:            DealReviewLearnedPatternLiveGuardEffectMonitorOnly,
			DecisionTimestamp: base.Add(4 * time.Hour),
			Attribution: &DealReviewLearnedPatternLiveGuardEventAttribution{
				Status: DealReviewLearnedPatternLiveGuardAttributionStatusWarningNotConfirmed,
			},
		},
		{
			ID:                "guard-recent-1",
			Effect:            DealReviewLearnedPatternLiveGuardEffectHardBlocked,
			DecisionTimestamp: base.Add(3 * time.Hour),
			Attribution: &DealReviewLearnedPatternLiveGuardEventAttribution{
				Status: DealReviewLearnedPatternLiveGuardAttributionStatusOverblocked,
			},
		},
		{
			ID:                "guard-prior-2",
			Effect:            DealReviewLearnedPatternLiveGuardEffectMonitorOnly,
			DecisionTimestamp: base.Add(2 * time.Hour),
			Attribution: &DealReviewLearnedPatternLiveGuardEventAttribution{
				Status: DealReviewLearnedPatternLiveGuardAttributionStatusWarningConfirmed,
			},
		},
		{
			ID:                "guard-prior-1",
			Effect:            DealReviewLearnedPatternLiveGuardEffectHardBlocked,
			DecisionTimestamp: base.Add(1 * time.Hour),
			Attribution: &DealReviewLearnedPatternLiveGuardEventAttribution{
				Status: DealReviewLearnedPatternLiveGuardAttributionStatusCorrectlyBlocked,
			},
		},
	}

	delta := buildDealReviewLearnedPatternLiveGuardAttributionDelta(events, 2)
	if delta == nil {
		t.Fatal("delta = nil")
	}
	if delta.TrendLabel != DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking {
		t.Fatalf(
			"TrendLabel = %q, want %q",
			delta.TrendLabel,
			DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking,
		)
	}
	if delta.RecentResolvedEventCount != 2 || delta.PriorResolvedEventCount != 2 {
		t.Fatalf(
			"resolved counts = %d/%d, want 2/2",
			delta.RecentResolvedEventCount,
			delta.PriorResolvedEventCount,
		)
	}
	if delta.OverblockingRateDelta <= 0 {
		t.Fatalf("OverblockingRateDelta = %.2f, want > 0", delta.OverblockingRateDelta)
	}
	if delta.Summary == "" {
		t.Fatal("Summary is empty, want delta summary text")
	}
}

func TestHydrateLearnedPatternLiveGuardAttribution_AggregatesPatternEvents(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	now := time.Now().UTC()

	pattern := DealReviewLearnedPattern{
		ID:           "pattern-rollup",
		StableKey:    "stable-rollup",
		UserID:       "user-live-guard",
		TraderID:     "trader-live-guard",
		ScopeType:    DealReviewLearnedPatternScopeSymbol,
		Symbol:       "RAVEUSDT",
		Side:         "LONG",
		PatternClass: DealReviewLearnedPatternClassNegativeEdge,
	}

	events := []DealReviewLearnedPatternLiveGuardEvent{
		{
			ID:                        "guard-rollup-1",
			UserID:                    pattern.UserID,
			TraderID:                  pattern.TraderID,
			Symbol:                    pattern.Symbol,
			Side:                      pattern.Side,
			Effect:                    DealReviewLearnedPatternLiveGuardEffectHardBlocked,
			DecisionTimestamp:         now.Add(-30 * time.Hour),
			MatchedPatternID:          pattern.ID,
			MatchedPatternStableKey:   pattern.StableKey,
			MatchedPatternSignature:   "bucket:breakout",
			MatchedPatternClass:       pattern.PatternClass,
			MatchedPatternScopeType:   pattern.ScopeType,
			MatchedPatternSampleCount: 8,
		},
		{
			ID:                      "guard-rollup-2",
			UserID:                  pattern.UserID,
			TraderID:                pattern.TraderID,
			Symbol:                  pattern.Symbol,
			Side:                    pattern.Side,
			Effect:                  DealReviewLearnedPatternLiveGuardEffectMonitorOnly,
			DecisionTimestamp:       now.Add(-8 * time.Hour),
			MatchedPatternID:        pattern.ID,
			MatchedPatternSignature: "bucket:breakout",
			MatchedPatternClass:     pattern.PatternClass,
			MatchedPatternScopeType: pattern.ScopeType,
		},
	}
	for idx := range events {
		if err := root.gdb.Create(&events[idx]).Error; err != nil {
			t.Fatalf("Create(event %d) error = %v", idx, err)
		}
	}

	cases := []DealReviewCase{
		{
			ID:             "guard-rollup-case-1",
			UserID:         pattern.UserID,
			TraderID:       pattern.TraderID,
			PositionID:     1001,
			Symbol:         pattern.Symbol,
			Side:           pattern.Side,
			Status:         DealReviewCaseStatusClosed,
			Outcome:        "loss",
			EntryTimeMs:    now.Add(-29 * time.Hour).UnixMilli(),
			ExitTimeMs:     now.Add(-28 * time.Hour).UnixMilli(),
			RealizedPnL:    -1.1,
			RealizedPnLPct: -1.8,
		},
		{
			ID:             "guard-rollup-case-2",
			UserID:         pattern.UserID,
			TraderID:       pattern.TraderID,
			PositionID:     1002,
			Symbol:         pattern.Symbol,
			Side:           pattern.Side,
			Status:         DealReviewCaseStatusClosed,
			Outcome:        "profit",
			EntryTimeMs:    now.Add(-7 * time.Hour).UnixMilli(),
			ExitTimeMs:     now.Add(-6 * time.Hour).UnixMilli(),
			RealizedPnL:    0.8,
			RealizedPnLPct: 1.2,
		},
	}
	for idx := range cases {
		if err := root.gdb.Create(&cases[idx]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", idx, err)
		}
	}

	items := []DealReviewLearnedPattern{pattern}
	if err := root.DealReview().hydrateLearnedPatternLiveGuardAttribution(
		pattern.UserID,
		pattern.TraderID,
		items,
		4,
	); err != nil {
		t.Fatalf("hydrateLearnedPatternLiveGuardAttribution() error = %v", err)
	}
	if items[0].LiveGuardAttribution == nil {
		t.Fatal("LiveGuardAttribution = nil")
	}
	if items[0].LiveGuardAttribution.EventCount != 2 {
		t.Fatalf("EventCount = %d, want 2", items[0].LiveGuardAttribution.EventCount)
	}
	if items[0].LiveGuardAttribution.ResolvedEventCount != 2 {
		t.Fatalf("ResolvedEventCount = %d, want 2", items[0].LiveGuardAttribution.ResolvedEventCount)
	}
	if items[0].LiveGuardAttribution.ProtectiveEvidenceCount != 1 {
		t.Fatalf(
			"ProtectiveEvidenceCount = %d, want 1",
			items[0].LiveGuardAttribution.ProtectiveEvidenceCount,
		)
	}
	if items[0].LiveGuardAttribution.OverblockingEvidenceCount != 1 {
		t.Fatalf(
			"OverblockingEvidenceCount = %d, want 1",
			items[0].LiveGuardAttribution.OverblockingEvidenceCount,
		)
	}
	if items[0].LiveGuardAttribution.AttributionLabel != DealReviewLearnedPatternLiveGuardAttributionLabelMixed {
		t.Fatalf(
			"AttributionLabel = %q, want %q",
			items[0].LiveGuardAttribution.AttributionLabel,
			DealReviewLearnedPatternLiveGuardAttributionLabelMixed,
		)
	}
	if items[0].LiveGuardAttribution.Summary == "" {
		t.Fatal("Summary is empty, want hydrated live-guard attribution summary")
	}
}

func TestHydrateLearnedPatternLiveGuardAttribution_UsesPersistedCacheUntilStale(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	now := time.Now().UTC()

	pattern := DealReviewLearnedPattern{
		ID:        "pattern-rollup-cache-hit",
		StableKey: "stable-rollup-cache-hit",
		UserID:    "user-live-guard",
		TraderID:  "trader-live-guard",
		Symbol:    "RAVEUSDT",
		Side:      "LONG",
	}
	event := DealReviewLearnedPatternLiveGuardEvent{
		ID:                      "guard-cache-hit-1",
		UserID:                  pattern.UserID,
		TraderID:                pattern.TraderID,
		Symbol:                  pattern.Symbol,
		Side:                    pattern.Side,
		Effect:                  DealReviewLearnedPatternLiveGuardEffectHardBlocked,
		DecisionTimestamp:       now.Add(-4 * time.Hour),
		MatchedPatternID:        pattern.ID,
		MatchedPatternStableKey: pattern.StableKey,
	}
	caseRec := DealReviewCase{
		ID:             "guard-cache-hit-case-1",
		UserID:         pattern.UserID,
		TraderID:       pattern.TraderID,
		PositionID:     1101,
		Symbol:         pattern.Symbol,
		Side:           pattern.Side,
		Status:         DealReviewCaseStatusClosed,
		Outcome:        "loss",
		EntryTimeMs:    now.Add(-3 * time.Hour).UnixMilli(),
		ExitTimeMs:     now.Add(-2 * time.Hour).UnixMilli(),
		RealizedPnL:    -0.9,
		RealizedPnLPct: -1.4,
	}
	if err := root.gdb.Create(&event).Error; err != nil {
		t.Fatalf("Create(event) error = %v", err)
	}
	if err := root.gdb.Create(&caseRec).Error; err != nil {
		t.Fatalf("Create(case) error = %v", err)
	}

	items := []DealReviewLearnedPattern{pattern}
	if err := root.DealReview().hydrateLearnedPatternLiveGuardAttribution(
		pattern.UserID,
		pattern.TraderID,
		items,
		4,
	); err != nil {
		t.Fatalf("first hydrateLearnedPatternLiveGuardAttribution() error = %v", err)
	}
	if items[0].LiveGuardAttribution == nil {
		t.Fatal("first LiveGuardAttribution = nil")
	}

	var cache DealReviewLearnedPatternLiveGuardRollupCache
	if err := root.gdb.
		Where(
			"user_id = ? AND trader_id = ? AND pattern_ref = ? AND event_limit = ?",
			pattern.UserID,
			pattern.TraderID,
			pattern.StableKey,
			4,
		).
		First(&cache).Error; err != nil {
		t.Fatalf("load rollup cache error = %v", err)
	}
	cache.RollupJSON = `{"event_count":1,"attribution_label":"pending","summary":"cached live-guard summary"}`
	cache.DeltaJSON = `{"recent_event_count":1,"trend_label":"stable","summary":"cached live-guard delta summary"}`
	cache.RefreshAfter = now.Add(30 * time.Minute)
	if err := root.gdb.Save(&cache).Error; err != nil {
		t.Fatalf("Save(cache) error = %v", err)
	}

	cachedItems := []DealReviewLearnedPattern{pattern}
	if err := root.DealReview().hydrateLearnedPatternLiveGuardAttribution(
		pattern.UserID,
		pattern.TraderID,
		cachedItems,
		4,
	); err != nil {
		t.Fatalf("cached hydrateLearnedPatternLiveGuardAttribution() error = %v", err)
	}
	if cachedItems[0].LiveGuardAttribution == nil {
		t.Fatal("cached LiveGuardAttribution = nil")
	}
	if cachedItems[0].LiveGuardAttribution.Summary != "cached live-guard summary" {
		t.Fatalf(
			"cached summary = %q, want cached live-guard summary",
			cachedItems[0].LiveGuardAttribution.Summary,
		)
	}
	if cachedItems[0].LiveGuardAttributionDelta == nil {
		t.Fatal("cached LiveGuardAttributionDelta = nil")
	}
	if cachedItems[0].LiveGuardAttributionDelta.Summary != "cached live-guard delta summary" {
		t.Fatalf(
			"cached delta summary = %q, want cached live-guard delta summary",
			cachedItems[0].LiveGuardAttributionDelta.Summary,
		)
	}

	newEvent := DealReviewLearnedPatternLiveGuardEvent{
		ID:                      "guard-cache-hit-2",
		UserID:                  pattern.UserID,
		TraderID:                pattern.TraderID,
		Symbol:                  pattern.Symbol,
		Side:                    pattern.Side,
		Effect:                  DealReviewLearnedPatternLiveGuardEffectMonitorOnly,
		DecisionTimestamp:       now.Add(-90 * time.Minute),
		MatchedPatternID:        pattern.ID,
		MatchedPatternStableKey: pattern.StableKey,
	}
	newCase := DealReviewCase{
		ID:             "guard-cache-hit-case-2",
		UserID:         pattern.UserID,
		TraderID:       pattern.TraderID,
		PositionID:     1102,
		Symbol:         pattern.Symbol,
		Side:           pattern.Side,
		Status:         DealReviewCaseStatusClosed,
		Outcome:        "profit",
		EntryTimeMs:    now.Add(-70 * time.Minute).UnixMilli(),
		ExitTimeMs:     now.Add(-50 * time.Minute).UnixMilli(),
		RealizedPnL:    0.6,
		RealizedPnLPct: 0.9,
	}
	if err := root.gdb.Create(&newEvent).Error; err != nil {
		t.Fatalf("Create(new event) error = %v", err)
	}
	if err := root.gdb.Create(&newCase).Error; err != nil {
		t.Fatalf("Create(new case) error = %v", err)
	}

	refreshedItems := []DealReviewLearnedPattern{pattern}
	if err := root.DealReview().hydrateLearnedPatternLiveGuardAttribution(
		pattern.UserID,
		pattern.TraderID,
		refreshedItems,
		4,
	); err != nil {
		t.Fatalf("refreshed hydrateLearnedPatternLiveGuardAttribution() error = %v", err)
	}
	if refreshedItems[0].LiveGuardAttribution == nil {
		t.Fatal("refreshed LiveGuardAttribution = nil")
	}
	if refreshedItems[0].LiveGuardAttribution.Summary == "cached live-guard summary" {
		t.Fatal("stale cached rollup reused after new event count changed")
	}
	if refreshedItems[0].LiveGuardAttribution.EventCount != 2 {
		t.Fatalf(
			"refreshed EventCount = %d, want 2",
			refreshedItems[0].LiveGuardAttribution.EventCount,
		)
	}
}

func TestHydrateLearnedPatternLiveGuardAttribution_RecomputesExpiredCache(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	now := time.Now().UTC()

	pattern := DealReviewLearnedPattern{
		ID:        "pattern-rollup-cache-expired",
		StableKey: "stable-rollup-cache-expired",
		UserID:    "user-live-guard",
		TraderID:  "trader-live-guard",
		Symbol:    "RAVEUSDT",
		Side:      "LONG",
	}
	event := DealReviewLearnedPatternLiveGuardEvent{
		ID:                      "guard-cache-expired-1",
		UserID:                  pattern.UserID,
		TraderID:                pattern.TraderID,
		Symbol:                  pattern.Symbol,
		Side:                    pattern.Side,
		Effect:                  DealReviewLearnedPatternLiveGuardEffectHardBlocked,
		DecisionTimestamp:       now.Add(-5 * time.Hour),
		MatchedPatternID:        pattern.ID,
		MatchedPatternStableKey: pattern.StableKey,
	}
	caseRec := DealReviewCase{
		ID:             "guard-cache-expired-case-1",
		UserID:         pattern.UserID,
		TraderID:       pattern.TraderID,
		PositionID:     1201,
		Symbol:         pattern.Symbol,
		Side:           pattern.Side,
		Status:         DealReviewCaseStatusClosed,
		Outcome:        "loss",
		EntryTimeMs:    now.Add(-4 * time.Hour).UnixMilli(),
		ExitTimeMs:     now.Add(-3 * time.Hour).UnixMilli(),
		RealizedPnL:    -1.0,
		RealizedPnLPct: -1.5,
	}
	if err := root.gdb.Create(&event).Error; err != nil {
		t.Fatalf("Create(event) error = %v", err)
	}
	if err := root.gdb.Create(&caseRec).Error; err != nil {
		t.Fatalf("Create(case) error = %v", err)
	}

	items := []DealReviewLearnedPattern{pattern}
	if err := root.DealReview().hydrateLearnedPatternLiveGuardAttribution(
		pattern.UserID,
		pattern.TraderID,
		items,
		4,
	); err != nil {
		t.Fatalf("first hydrateLearnedPatternLiveGuardAttribution() error = %v", err)
	}

	var cache DealReviewLearnedPatternLiveGuardRollupCache
	if err := root.gdb.
		Where(
			"user_id = ? AND trader_id = ? AND pattern_ref = ? AND event_limit = ?",
			pattern.UserID,
			pattern.TraderID,
			pattern.StableKey,
			4,
		).
		First(&cache).Error; err != nil {
		t.Fatalf("load rollup cache error = %v", err)
	}
	cache.RollupJSON = `{"event_count":1,"attribution_label":"pending","summary":"expired cache summary"}`
	cache.DeltaJSON = `{"recent_event_count":1,"trend_label":"stable","summary":"expired cache delta summary"}`
	cache.RefreshAfter = now.Add(-time.Minute)
	if err := root.gdb.Save(&cache).Error; err != nil {
		t.Fatalf("Save(cache) error = %v", err)
	}

	refreshedItems := []DealReviewLearnedPattern{pattern}
	if err := root.DealReview().hydrateLearnedPatternLiveGuardAttribution(
		pattern.UserID,
		pattern.TraderID,
		refreshedItems,
		4,
	); err != nil {
		t.Fatalf("refreshed hydrateLearnedPatternLiveGuardAttribution() error = %v", err)
	}
	if refreshedItems[0].LiveGuardAttribution == nil {
		t.Fatal("refreshed LiveGuardAttribution = nil")
	}
	if refreshedItems[0].LiveGuardAttribution.Summary == "expired cache summary" {
		t.Fatal("expired cache summary reused instead of recomputing")
	}
	if refreshedItems[0].LiveGuardAttributionDelta == nil {
		t.Fatal("refreshed LiveGuardAttributionDelta = nil")
	}
	if refreshedItems[0].LiveGuardAttributionDelta.Summary == "expired cache delta summary" {
		t.Fatal("expired cache delta summary reused instead of recomputing")
	}
}

func TestRecordLearnedPatternLiveGuardEvent_InvalidatesRollupCache(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	now := time.Now().UTC()

	pattern := DealReviewLearnedPattern{
		ID:                     "pattern-rollup-cache-invalidate",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		ScopeType:              DealReviewLearnedPatternScopeSymbol,
		ScopeKey:               "symbol:RAVEUSDT:LONG",
		StableKey:              "stable-rollup-cache-invalidate",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		Status:                 "validated",
		ValidationLabel:        DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitoringRule,
		PatternSignature:       "bucket:breakout",
		PatternOrder:           1,
		FeatureSetJSON:         `["bucket:breakout","trend:uptrend"]`,
		SampleCount:            12,
		CompositeScore:         0.92,
		ConfidenceScore:        0.90,
		ValidationSupportScore: 0.82,
		FalsePositiveScore:     0.04,
		DriftScore:             0.05,
		LastObservedAt:         now.Add(-2 * time.Hour),
		BuiltAt:                now.Add(-2 * time.Hour),
	}
	hydrateDealReviewLearnedPattern(&pattern)
	if err := root.gdb.Create(&pattern).Error; err != nil {
		t.Fatalf("Create(pattern) error = %v", err)
	}

	cacheRecord := DealReviewLearnedPatternLiveGuardRollupCache{
		UserID:           pattern.UserID,
		TraderID:         pattern.TraderID,
		PatternRef:       pattern.StableKey,
		PatternID:        pattern.ID,
		PatternStableKey: pattern.StableKey,
		EventLimit:       4,
		SourceEventCount: 1,
		LatestEventAt:    now.Add(-2 * time.Hour),
		RefreshAfter:     now.Add(30 * time.Minute),
		BuiltAt:          now,
		RollupJSON:       `{"event_count":1,"summary":"cached before invalidation"}`,
	}
	if err := root.gdb.Create(&cacheRecord).Error; err != nil {
		t.Fatalf("Create(cacheRecord) error = %v", err)
	}

	probe := &DealReviewLearnedPatternLiveGuardProbe{
		CycleNumber:       44,
		DecisionTimestamp: now,
		Action:            "open_long",
		Symbol:            pattern.Symbol,
		Side:              pattern.Side,
	}
	assessment := &DealReviewLearnedPatternLiveGuardAssessment{
		Enabled:        true,
		Mode:           LearnedPatternLiveGuardModeMonitor,
		Effect:         DealReviewLearnedPatternLiveGuardEffectMonitorOnly,
		Qualified:      true,
		MatchScore:     0.94,
		MatchedPattern: &pattern,
	}

	if err := root.DealReview().RecordLearnedPatternLiveGuardEvent(
		pattern.UserID,
		pattern.TraderID,
		probe,
		assessment,
	); err != nil {
		t.Fatalf("RecordLearnedPatternLiveGuardEvent() error = %v", err)
	}

	var remaining int64
	if err := root.gdb.Model(&DealReviewLearnedPatternLiveGuardRollupCache{}).
		Where(
			"user_id = ? AND trader_id = ? AND pattern_ref = ?",
			pattern.UserID,
			pattern.TraderID,
			pattern.StableKey,
		).
		Count(&remaining).Error; err != nil {
		t.Fatalf("Count(cache) error = %v", err)
	}
	if remaining != 0 {
		t.Fatalf("remaining cache rows = %d, want 0 after invalidation", remaining)
	}
}

func TestBuildDealReviewLearnedPatternReportingSummary_TracksLiveGuardAttribution(t *testing.T) {
	items := []DealReviewLearnedPattern{
		{
			ID:               "pattern-protective",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitoringRule,
			PatternSignature: "bucket:protective",
			CompositeScore:   0.82,
			LiveGuardAttribution: &DealReviewLearnedPatternLiveGuardAttributionRollup{
				AttributionLabel:        DealReviewLearnedPatternLiveGuardAttributionLabelProtective,
				ResolvedEventCount:      3,
				ProtectiveRate:          0.67,
				ConfidenceScore:         0.50,
				ProtectiveEvidenceCount: 2,
			},
			LiveGuardAttributionDelta: &DealReviewLearnedPatternLiveGuardAttributionDelta{
				TrendLabel:               DealReviewLearnedPatternLiveGuardAttributionTrendImproving,
				ProtectiveRateDelta:      0.35,
				RecentResolvedEventCount: 3,
				ConfidenceScore:          0.56,
			},
		},
		{
			ID:               "pattern-overblocking",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitoringRule,
			PatternSignature: "bucket:overblocking",
			CompositeScore:   0.88,
			LiveGuardAttribution: &DealReviewLearnedPatternLiveGuardAttributionRollup{
				AttributionLabel:          DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking,
				ResolvedEventCount:        4,
				OverblockingRate:          0.75,
				ConfidenceScore:           0.75,
				OverblockingEvidenceCount: 3,
			},
			LiveGuardAttributionDelta: &DealReviewLearnedPatternLiveGuardAttributionDelta{
				TrendLabel:               DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking,
				OverblockingRateDelta:    0.50,
				RecentResolvedEventCount: 4,
				PriorResolvedEventCount:  3,
				ConfidenceScore:          0.78,
				RecentOverblockingRate:   0.75,
				PriorOverblockingRate:    0.25,
				RecentAttributionLabel:   DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking,
				PriorAttributionLabel:    DealReviewLearnedPatternLiveGuardAttributionLabelMixed,
			},
		},
		{
			ID:               "pattern-mixed",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			ValidationLabel:  DealReviewLearnedPatternValidationLabelDrifting,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitorOnly,
			PatternSignature: "bucket:mixed",
			CompositeScore:   0.61,
			LiveGuardAttribution: &DealReviewLearnedPatternLiveGuardAttributionRollup{
				AttributionLabel:   DealReviewLearnedPatternLiveGuardAttributionLabelMixed,
				ResolvedEventCount: 2,
			},
			LiveGuardAttributionDelta: &DealReviewLearnedPatternLiveGuardAttributionDelta{
				TrendLabel:               DealReviewLearnedPatternLiveGuardAttributionTrendDegrading,
				OverblockingRateDelta:    0.22,
				RecentResolvedEventCount: 2,
				PriorResolvedEventCount:  3,
				ConfidenceScore:          0.49,
				RecentOverblockingRate:   0.50,
				PriorOverblockingRate:    0.28,
				RecentAttributionLabel:   DealReviewLearnedPatternLiveGuardAttributionLabelMixed,
				PriorAttributionLabel:    DealReviewLearnedPatternLiveGuardAttributionLabelProtective,
			},
		},
	}

	summary := BuildDealReviewLearnedPatternReportingSummary(items)
	if summary == nil {
		t.Fatal("summary = nil")
	}
	if summary.LiveGuardProtectiveCount != 1 {
		t.Fatalf("LiveGuardProtectiveCount = %d, want 1", summary.LiveGuardProtectiveCount)
	}
	if summary.LiveGuardOverblockingCount != 1 {
		t.Fatalf("LiveGuardOverblockingCount = %d, want 1", summary.LiveGuardOverblockingCount)
	}
	if summary.LiveGuardImprovingCount != 1 {
		t.Fatalf("LiveGuardImprovingCount = %d, want 1", summary.LiveGuardImprovingCount)
	}
	if summary.LiveGuardDegradingCount != 1 {
		t.Fatalf("LiveGuardDegradingCount = %d, want 1", summary.LiveGuardDegradingCount)
	}
	if summary.LiveGuardNewlyOverblockingCount != 1 {
		t.Fatalf("LiveGuardNewlyOverblockingCount = %d, want 1", summary.LiveGuardNewlyOverblockingCount)
	}
	if len(summary.TopOverblockingMonitoringRules) != 1 || summary.TopOverblockingMonitoringRules[0].ID != "pattern-overblocking" {
		t.Fatalf("TopOverblockingMonitoringRules = %#v, want pattern-overblocking", summary.TopOverblockingMonitoringRules)
	}
	if len(summary.TopImprovingMonitoringRules) != 1 || summary.TopImprovingMonitoringRules[0].ID != "pattern-protective" {
		t.Fatalf("TopImprovingMonitoringRules = %#v, want pattern-protective", summary.TopImprovingMonitoringRules)
	}
	if len(summary.TopDegradingMonitoringRules) != 2 || summary.TopDegradingMonitoringRules[0].ID != "pattern-overblocking" {
		t.Fatalf("TopDegradingMonitoringRules = %#v, want pattern-overblocking first", summary.TopDegradingMonitoringRules)
	}
}

func TestBuildDealReviewLearnedPatternActionHint_SuggestsSuppress(t *testing.T) {
	pattern := &DealReviewLearnedPattern{
		ID:              "pattern-suppress",
		PatternClass:    DealReviewLearnedPatternClassNegativeEdge,
		ValidationLabel: DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:  DealReviewLearnedPatternRecommendedUseMonitoringRule,
		LiveGuardAttribution: &DealReviewLearnedPatternLiveGuardAttributionRollup{
			AttributionLabel:   DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking,
			ResolvedEventCount: 4,
			ConfidenceScore:    0.78,
			OverblockingRate:   0.75,
		},
		LiveGuardAttributionDelta: &DealReviewLearnedPatternLiveGuardAttributionDelta{
			TrendLabel:               DealReviewLearnedPatternLiveGuardAttributionTrendDegrading,
			RecentResolvedEventCount: 4,
			PriorResolvedEventCount:  4,
			RecentOverblockingRate:   0.75,
			PriorOverblockingRate:    0.25,
			OverblockingRateDelta:    0.50,
			ConfidenceScore:          0.74,
		},
		Lifecycle: &DealReviewLearnedPatternLifecycle{
			Status:        DealReviewLearnedPatternLifecycleStatusRollbackWatch,
			RollbackScore: 0.72,
		},
	}

	hint := buildDealReviewLearnedPatternActionHint(pattern)
	if hint == nil {
		t.Fatal("hint = nil")
	}
	if hint.RecommendedAction != DealReviewLearnedPatternActionHintSuppress {
		t.Fatalf("RecommendedAction = %q, want suppress", hint.RecommendedAction)
	}
	if hint.PriorityLabel == "" || hint.Summary == "" || hint.AutoNote == "" {
		t.Fatalf("hint = %#v, want populated priority, summary, and auto note", hint)
	}
}

func TestBuildDealReviewLearnedPatternActionHint_SuggestsRetire(t *testing.T) {
	pattern := &DealReviewLearnedPattern{
		ID:              "pattern-retire",
		PatternClass:    DealReviewLearnedPatternClassNegativeEdge,
		ValidationLabel: DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:  DealReviewLearnedPatternRecommendedUseMonitoringRule,
		LiveGuardAttributionDelta: &DealReviewLearnedPatternLiveGuardAttributionDelta{
			TrendLabel:               DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking,
			RecentResolvedEventCount: 3,
			PriorResolvedEventCount:  3,
			RecentOverblockingRate:   1,
			PriorOverblockingRate:    0.33,
			ConfidenceScore:          0.88,
		},
		Lifecycle: &DealReviewLearnedPatternLifecycle{
			Status:      DealReviewLearnedPatternLifecycleStatusExpired,
			ExpiryScore: 0.93,
		},
	}

	hint := buildDealReviewLearnedPatternActionHint(pattern)
	if hint == nil {
		t.Fatal("hint = nil")
	}
	if hint.RecommendedAction != DealReviewLearnedPatternActionHintRetire {
		t.Fatalf("RecommendedAction = %q, want retire", hint.RecommendedAction)
	}
	if hint.PriorityLabel != DealReviewLearnedPatternActionHintPriorityCritical {
		t.Fatalf("PriorityLabel = %q, want critical", hint.PriorityLabel)
	}
}

func TestBuildDealReviewLearnedPatternActionHint_SuggestsRearm(t *testing.T) {
	pattern := &DealReviewLearnedPattern{
		ID:                 "pattern-rearm",
		PatternClass:       DealReviewLearnedPatternClassNegativeEdge,
		ValidationLabel:    DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:     DealReviewLearnedPatternRecommendedUseMonitorOnly,
		BaseRecommendedUse: DealReviewLearnedPatternRecommendedUseMonitoringRule,
		ManualControl: &DealReviewLearnedPatternManualControl{
			ControlState: DealReviewLearnedPatternManualControlStateSuppressed,
		},
		LiveGuardAttribution: &DealReviewLearnedPatternLiveGuardAttributionRollup{
			AttributionLabel:   DealReviewLearnedPatternLiveGuardAttributionLabelProtective,
			ResolvedEventCount: 3,
			ProtectiveRate:     0.67,
		},
		LiveGuardAttributionDelta: &DealReviewLearnedPatternLiveGuardAttributionDelta{
			TrendLabel:               DealReviewLearnedPatternLiveGuardAttributionTrendImproving,
			RecentResolvedEventCount: 3,
			PriorResolvedEventCount:  2,
			RecentProtectiveRate:     0.67,
			PriorProtectiveRate:      0.25,
			RecentOverblockingRate:   0.33,
			ConfidenceScore:          0.58,
		},
	}

	hint := buildDealReviewLearnedPatternActionHint(pattern)
	if hint == nil {
		t.Fatal("hint = nil")
	}
	if hint.RecommendedAction != DealReviewLearnedPatternActionHintRearm {
		t.Fatalf("RecommendedAction = %q, want rearm", hint.RecommendedAction)
	}
}

func TestBuildDealReviewLearnedPatternLiveActionHint_SuggestsSuppressionCandidate(t *testing.T) {
	pattern := &DealReviewLearnedPattern{
		ID:              "pattern-live-action-suppress",
		PatternClass:    DealReviewLearnedPatternClassNegativeEdge,
		ValidationLabel: DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:  DealReviewLearnedPatternRecommendedUseMonitoringRule,
		ActionHint: &DealReviewLearnedPatternActionHint{
			RecommendedAction: DealReviewLearnedPatternActionHintSuppress,
			PriorityLabel:     DealReviewLearnedPatternActionHintPriorityCritical,
			ConfidenceScore:   0.84,
		},
		LiveGuardAttributionDelta: &DealReviewLearnedPatternLiveGuardAttributionDelta{
			TrendLabel:               DealReviewLearnedPatternLiveGuardAttributionTrendDegrading,
			RecentResolvedEventCount: 4,
			PriorResolvedEventCount:  3,
			RecentOverblockingRate:   0.74,
			PriorOverblockingRate:    0.22,
			RecentProtectiveRate:     0.26,
			PriorProtectiveRate:      0.78,
			ConfidenceScore:          0.79,
		},
		Lifecycle: &DealReviewLearnedPatternLifecycle{
			Status:        DealReviewLearnedPatternLifecycleStatusRollbackWatch,
			RollbackScore: 0.74,
		},
	}

	hint := buildDealReviewLearnedPatternLiveActionHint(pattern)
	if hint == nil {
		t.Fatal("hint = nil")
	}
	if hint.CandidateKind != DealReviewLearnedPatternLiveActionKindSuppression {
		t.Fatalf("CandidateKind = %q, want suppression", hint.CandidateKind)
	}
	if hint.RecommendedAction != DealReviewLearnedPatternActionHintSuppress {
		t.Fatalf("RecommendedAction = %q, want suppress", hint.RecommendedAction)
	}
	if hint.Summary == "" {
		t.Fatalf("Summary = %q, want populated summary", hint.Summary)
	}
}

func TestBuildDealReviewLearnedPatternLiveActionHint_SuggestsRollbackCandidate(t *testing.T) {
	pattern := &DealReviewLearnedPattern{
		ID:              "pattern-live-action-rollback",
		PatternClass:    DealReviewLearnedPatternClassNegativeEdge,
		ValidationLabel: DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:  DealReviewLearnedPatternRecommendedUseMonitoringRule,
		ActionHint: &DealReviewLearnedPatternActionHint{
			RecommendedAction: DealReviewLearnedPatternActionHintRetire,
			PriorityLabel:     DealReviewLearnedPatternActionHintPriorityCritical,
			ConfidenceScore:   0.91,
		},
		LiveGuardAttributionDelta: &DealReviewLearnedPatternLiveGuardAttributionDelta{
			TrendLabel:               DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking,
			RecentResolvedEventCount: 4,
			PriorResolvedEventCount:  3,
			RecentOverblockingRate:   0.92,
			PriorOverblockingRate:    0.31,
			RecentProtectiveRate:     0.08,
			PriorProtectiveRate:      0.69,
			ConfidenceScore:          0.88,
		},
		Lifecycle: &DealReviewLearnedPatternLifecycle{
			Status:      DealReviewLearnedPatternLifecycleStatusExpired,
			ExpiryScore: 0.93,
		},
	}

	hint := buildDealReviewLearnedPatternLiveActionHint(pattern)
	if hint == nil {
		t.Fatal("hint = nil")
	}
	if hint.CandidateKind != DealReviewLearnedPatternLiveActionKindRollback {
		t.Fatalf("CandidateKind = %q, want rollback", hint.CandidateKind)
	}
	if hint.RecommendedAction != DealReviewLearnedPatternActionHintRetire {
		t.Fatalf("RecommendedAction = %q, want retire", hint.RecommendedAction)
	}
	if hint.Summary == "" {
		t.Fatalf("Summary = %q, want populated summary", hint.Summary)
	}
}

func TestListLearnedPatterns_FiltersDirectLiveActionCandidatesAndInterventionState(t *testing.T) {
	root := openDealReviewInterventionTestStore(t)
	now := time.Now().UTC()

	patterns := []DealReviewLearnedPattern{
		{
			ID:                 "pattern-open-rollback",
			StableKey:          "stable-open-rollback",
			UserID:             "user-pattern-filter",
			TraderID:           "trader-pattern-filter",
			ScopeType:          DealReviewLearnedPatternScopeTraderLocal,
			ScopeKey:           "scope:open-rollback",
			Symbol:             "RAVEUSDT",
			Side:               "LONG",
			PatternClass:       DealReviewLearnedPatternClassNegativeEdge,
			Status:             "validated",
			ValidationLabel:    DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:     DealReviewLearnedPatternRecommendedUseMonitoringRule,
			PatternSignature:   "bucket:breakout|trend:uptrend",
			FeatureSetJSON:     `["bucket:breakout","trend:uptrend"]`,
			SampleCount:        7,
			SupportCount:       5,
			CompositeScore:     0.58,
			ConfidenceScore:    0.62,
			LastObservedAt:     now.Add(-1 * time.Hour),
			BuiltAt:            now.Add(-1 * time.Hour),
			BaseRecommendedUse: DealReviewLearnedPatternRecommendedUseMonitoringRule,
		},
		{
			ID:               "pattern-resolved-suppress",
			StableKey:        "stable-resolved-suppress",
			UserID:           "user-pattern-filter",
			TraderID:         "trader-pattern-filter",
			ScopeType:        DealReviewLearnedPatternScopeTraderLocal,
			ScopeKey:         "scope:resolved-suppress",
			Symbol:           "TAOUSDT",
			Side:             "SHORT",
			PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
			Status:           "validated",
			ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitoringRule,
			PatternSignature: "bucket:fade|trend:downtrend",
			FeatureSetJSON:   `["bucket:fade","trend:downtrend"]`,
			SampleCount:      9,
			SupportCount:     6,
			CompositeScore:   0.91,
			ConfidenceScore:  0.87,
			LastObservedAt:   now.Add(-2 * time.Hour),
			BuiltAt:          now.Add(-2 * time.Hour),
		},
		{
			ID:               "pattern-no-intervention",
			StableKey:        "stable-no-intervention",
			UserID:           "user-pattern-filter",
			TraderID:         "trader-pattern-filter",
			ScopeType:        DealReviewLearnedPatternScopeTraderLocal,
			ScopeKey:         "scope:no-intervention",
			Symbol:           "BTCUSDT",
			Side:             "LONG",
			PatternClass:     DealReviewLearnedPatternClassPositiveEdge,
			Status:           "validated",
			ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:   DealReviewLearnedPatternRecommendedUsePromptHint,
			PatternSignature: "bucket:trend-follow|trend:uptrend",
			FeatureSetJSON:   `["bucket:trend-follow","trend:uptrend"]`,
			SampleCount:      12,
			SupportCount:     8,
			CompositeScore:   0.95,
			ConfidenceScore:  0.89,
			LastObservedAt:   now.Add(-30 * time.Minute),
			BuiltAt:          now.Add(-30 * time.Minute),
		},
	}
	for idx := range patterns {
		hydrateDealReviewLearnedPattern(&patterns[idx])
		if err := root.GormDB().Create(&patterns[idx]).Error; err != nil {
			t.Fatalf("Create(pattern[%d]) error = %v", idx, err)
		}
	}

	interventions := []DealReviewLearnedPatternIntervention{
		{
			ID:                         "intervention-open-rollback",
			UserID:                     "user-pattern-filter",
			TraderID:                   "trader-pattern-filter",
			PatternID:                  "pattern-open-rollback",
			PatternStableKey:           "stable-open-rollback",
			EventType:                  DealReviewLearnedPatternInterventionEventTypeManualAction,
			EventStatus:                DealReviewLearnedPatternInterventionStatusOpen,
			SuggestedAction:            DealReviewLearnedPatternActionHintRetire,
			AppliedAction:              DealReviewLearnedPatternManualControlActionRetire,
			DirectLiveActionCandidate:  true,
			DirectLiveActionKind:       DealReviewLearnedPatternLiveActionKindRollback,
			DirectLiveActionSummary:    "Rollback-grade open candidate.",
			DirectLiveActionConfidence: 0.92,
			FirstSeenAt:                now.Add(-15 * time.Minute),
			LastSeenAt:                 now.Add(-5 * time.Minute),
		},
		{
			ID:                         "intervention-resolved-suppress",
			UserID:                     "user-pattern-filter",
			TraderID:                   "trader-pattern-filter",
			PatternID:                  "pattern-resolved-suppress",
			PatternStableKey:           "stable-resolved-suppress",
			EventType:                  DealReviewLearnedPatternInterventionEventTypeManualAction,
			EventStatus:                DealReviewLearnedPatternInterventionStatusAccepted,
			SuggestedAction:            DealReviewLearnedPatternActionHintSuppress,
			AppliedAction:              DealReviewLearnedPatternManualControlActionSuppress,
			DirectLiveActionCandidate:  true,
			DirectLiveActionKind:       DealReviewLearnedPatternLiveActionKindSuppression,
			DirectLiveActionSummary:    "Suppression candidate already resolved.",
			DirectLiveActionConfidence: 0.81,
			FirstSeenAt:                now.Add(-45 * time.Minute),
			LastSeenAt:                 now.Add(-35 * time.Minute),
			ResolvedAt:                 now.Add(-35 * time.Minute),
		},
	}
	for idx := range interventions {
		if err := root.GormDB().Create(&interventions[idx]).Error; err != nil {
			t.Fatalf("Create(intervention[%d]) error = %v", idx, err)
		}
	}

	items, err := root.DealReview().ListLearnedPatterns(
		"user-pattern-filter",
		"trader-pattern-filter",
		DealReviewLearnedPatternFilter{
			DirectLiveActionCandidate: true,
			Limit:                     10,
		},
	)
	if err != nil {
		t.Fatalf("ListLearnedPatterns(direct_live_action_candidate) error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("direct_live_action_candidate len = %d, want 2", len(items))
	}
	if items[0].ID != "pattern-open-rollback" {
		t.Fatalf("direct_live_action_candidate first = %q, want open rollback candidate first", items[0].ID)
	}

	openOnly, err := root.DealReview().ListLearnedPatterns(
		"user-pattern-filter",
		"trader-pattern-filter",
		DealReviewLearnedPatternFilter{
			DirectLiveActionCandidate: true,
			InterventionState:         DealReviewLearnedPatternInterventionStateFilterOpen,
			Limit:                     10,
		},
	)
	if err != nil {
		t.Fatalf("ListLearnedPatterns(open direct candidates) error = %v", err)
	}
	if len(openOnly) != 1 || openOnly[0].ID != "pattern-open-rollback" {
		t.Fatalf("open direct candidates = %#v, want only pattern-open-rollback", openOnly)
	}

	rollbackOnly, err := root.DealReview().ListLearnedPatterns(
		"user-pattern-filter",
		"trader-pattern-filter",
		DealReviewLearnedPatternFilter{
			LiveActionKind: DealReviewLearnedPatternLiveActionKindRollback,
			Limit:          10,
		},
	)
	if err != nil {
		t.Fatalf("ListLearnedPatterns(rollback live action kind) error = %v", err)
	}
	if len(rollbackOnly) != 1 || rollbackOnly[0].ID != "pattern-open-rollback" {
		t.Fatalf("rollback live action kind = %#v, want only pattern-open-rollback", rollbackOnly)
	}

	resolvedOnly, err := root.DealReview().ListLearnedPatterns(
		"user-pattern-filter",
		"trader-pattern-filter",
		DealReviewLearnedPatternFilter{
			InterventionState: DealReviewLearnedPatternInterventionStateFilterResolved,
			Limit:             10,
		},
	)
	if err != nil {
		t.Fatalf("ListLearnedPatterns(resolved interventions) error = %v", err)
	}
	if len(resolvedOnly) != 1 || resolvedOnly[0].ID != "pattern-resolved-suppress" {
		t.Fatalf("resolved interventions = %#v, want only pattern-resolved-suppress", resolvedOnly)
	}

	noIntervention, err := root.DealReview().ListLearnedPatterns(
		"user-pattern-filter",
		"trader-pattern-filter",
		DealReviewLearnedPatternFilter{
			InterventionState: DealReviewLearnedPatternInterventionStateFilterNone,
			Limit:             10,
		},
	)
	if err != nil {
		t.Fatalf("ListLearnedPatterns(no interventions) error = %v", err)
	}
	if len(noIntervention) != 1 || noIntervention[0].ID != "pattern-no-intervention" {
		t.Fatalf("no interventions = %#v, want only pattern-no-intervention", noIntervention)
	}
}

func TestPersistLearnedPatternLifecycleSnapshotsTx_HydratesHistory(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	now := time.Now().UTC()

	pattern := DealReviewLearnedPattern{
		ID:                     "pattern-history",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		ScopeType:              DealReviewLearnedPatternScopeSymbol,
		ScopeKey:               "symbol:RAVEUSDT:LONG",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		Status:                 "validated",
		ValidationLabel:        DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitoringRule,
		PatternSignature:       "bucket:breakout",
		RegimeSignature:        "bucket=breakout | trend=uptrend",
		PatternOrder:           1,
		FeatureSetJSON:         `["bucket:breakout","trend:uptrend"]`,
		SampleCount:            11,
		CompositeScore:         0.91,
		ConfidenceScore:        0.92,
		ValidationSupportScore: 0.84,
		ValidationSupportCount: 5,
		ValidationSampleCount:  6,
		RecentSupportScore:     0.78,
		RecentSupportCount:     4,
		RecentSampleCount:      5,
		RecencyWeight:          0.82,
		DriftScore:             0.05,
		LastObservedAt:         now.Add(-2 * time.Hour),
		BuiltAt:                now.Add(-2 * time.Hour),
	}
	hydrateDealReviewLearnedPattern(&pattern)
	if err := root.gdb.Create(&pattern).Error; err != nil {
		t.Fatalf("Create(pattern) error = %v", err)
	}

	if err := root.gdb.Transaction(func(tx *gorm.DB) error {
		return root.DealReview().persistLearnedPatternLifecycleSnapshotsTx(
			tx,
			pattern.UserID,
			pattern.TraderID,
			[]DealReviewLearnedPattern{pattern},
			now.Add(-90*time.Minute),
			dealReviewLearnedPatternLifecycleSnapshotSourceRebuild,
			"",
		)
	}); err != nil {
		t.Fatalf("persist active lifecycle snapshot error = %v", err)
	}

	pattern.ValidationLabel = DealReviewLearnedPatternValidationLabelDrifting
	pattern.ValidationSupportScore = 0.42
	pattern.RecentSupportScore = 0.20
	pattern.RecentSupportCount = 1
	pattern.RecentSampleCount = 4
	pattern.DriftScore = 0.18
	pattern.RecencyWeight = 0.28
	pattern.LastObservedAt = now.Add(-5 * 24 * time.Hour)

	if err := root.gdb.Transaction(func(tx *gorm.DB) error {
		return root.DealReview().persistLearnedPatternLifecycleSnapshotsTx(
			tx,
			pattern.UserID,
			pattern.TraderID,
			[]DealReviewLearnedPattern{pattern},
			now,
			dealReviewLearnedPatternLifecycleSnapshotSourceRebuild,
			"",
		)
	}); err != nil {
		t.Fatalf("persist degrading lifecycle snapshot error = %v", err)
	}

	items := []DealReviewLearnedPattern{pattern}
	if err := root.DealReview().hydrateLearnedPatternLifecycleHistory(
		pattern.UserID,
		pattern.TraderID,
		items,
		4,
	); err != nil {
		t.Fatalf("hydrateLearnedPatternLifecycleHistory() error = %v", err)
	}
	if len(items[0].LifecycleHistory) != 2 {
		t.Fatalf("lifecycle history len = %d, want 2", len(items[0].LifecycleHistory))
	}
	if items[0].LifecycleHistory[0].LifecycleStatus != DealReviewLearnedPatternLifecycleStatusDegrading {
		t.Fatalf(
			"latest lifecycle status = %q, want %q",
			items[0].LifecycleHistory[0].LifecycleStatus,
			DealReviewLearnedPatternLifecycleStatusDegrading,
		)
	}
	if items[0].LifecycleHistory[1].LifecycleStatus != DealReviewLearnedPatternLifecycleStatusActive {
		t.Fatalf(
			"previous lifecycle status = %q, want %q",
			items[0].LifecycleHistory[1].LifecycleStatus,
			DealReviewLearnedPatternLifecycleStatusActive,
		)
	}
	if !items[0].LifecycleHistory[0].StatusChanged {
		t.Fatal("latest lifecycle snapshot StatusChanged = false, want true")
	}
}

func TestApplyLearnedPatternManualControl_SuppressAndRearm(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	now := time.Now().UTC()

	pattern := DealReviewLearnedPattern{
		ID:                     "pattern-manual-control",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		ScopeType:              DealReviewLearnedPatternScopeSymbol,
		ScopeKey:               "symbol:RAVEUSDT:LONG",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		Status:                 "validated",
		ValidationLabel:        DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitoringRule,
		PatternSignature:       "bucket:breakout",
		PatternOrder:           1,
		FeatureSetJSON:         `["bucket:breakout","trend:uptrend"]`,
		SampleCount:            12,
		CompositeScore:         0.93,
		ConfidenceScore:        0.94,
		ValidationSupportScore: 0.82,
		FalsePositiveScore:     0.04,
		DriftScore:             0.05,
		RecentSupportScore:     0.78,
		RecentSupportCount:     4,
		RecentSampleCount:      5,
		RecencyWeight:          0.84,
		LastObservedAt:         now.Add(-90 * time.Minute),
		BuiltAt:                now.Add(-90 * time.Minute),
	}
	hydrateDealReviewLearnedPattern(&pattern)
	if err := root.gdb.Create(&pattern).Error; err != nil {
		t.Fatalf("Create(pattern) error = %v", err)
	}

	control, err := root.DealReview().ApplyLearnedPatternManualControl(
		pattern.UserID,
		pattern.TraderID,
		pattern.ID,
		pattern.StableKey,
		DealReviewLearnedPatternManualControlActionSuppress,
		"Suppress until new validation confirms this setup again.",
	)
	if err != nil {
		t.Fatalf("ApplyLearnedPatternManualControl(suppress) error = %v", err)
	}
	if control == nil || control.ControlState != DealReviewLearnedPatternManualControlStateSuppressed {
		t.Fatalf("control after suppress = %#v, want suppressed", control)
	}

	items, err := root.DealReview().ListLearnedPatterns(
		pattern.UserID,
		pattern.TraderID,
		DealReviewLearnedPatternFilter{StableKey: pattern.StableKey, Limit: 4},
	)
	if err != nil {
		t.Fatalf("ListLearnedPatterns() after suppress error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListLearnedPatterns() len = %d, want 1", len(items))
	}
	if items[0].BaseRecommendedUse != DealReviewLearnedPatternRecommendedUseMonitoringRule {
		t.Fatalf("BaseRecommendedUse = %q, want monitoring_rule", items[0].BaseRecommendedUse)
	}
	if items[0].RecommendedUse != DealReviewLearnedPatternRecommendedUseMonitorOnly {
		t.Fatalf("RecommendedUse after suppress = %q, want monitor_only", items[0].RecommendedUse)
	}
	if items[0].ManualControl == nil || items[0].ManualControl.ControlState != DealReviewLearnedPatternManualControlStateSuppressed {
		t.Fatalf("ManualControl after suppress = %#v, want suppressed", items[0].ManualControl)
	}
	if len(items[0].ManualControlHistory) != 1 {
		t.Fatalf("ManualControlHistory len after suppress = %d, want 1", len(items[0].ManualControlHistory))
	}

	reasons := dealReviewLearnedPatternLiveDisqualificationReasons(
		&items[0],
		DefaultLearnedPatternLiveGuardConfig(),
		0.95,
	)
	foundSuppressed := false
	for _, reason := range reasons {
		if reason == "analyst_control=suppressed" {
			foundSuppressed = true
			break
		}
	}
	if !foundSuppressed {
		t.Fatalf("disqualification reasons = %#v, want analyst_control=suppressed", reasons)
	}

	control, err = root.DealReview().ApplyLearnedPatternManualControl(
		pattern.UserID,
		pattern.TraderID,
		pattern.ID,
		pattern.StableKey,
		DealReviewLearnedPatternManualControlActionRearm,
		"Re-arm after manual review.",
	)
	if err != nil {
		t.Fatalf("ApplyLearnedPatternManualControl(rearm) error = %v", err)
	}
	if control == nil || control.ControlState != DealReviewLearnedPatternManualControlStateLive {
		t.Fatalf("control after rearm = %#v, want live", control)
	}

	items, err = root.DealReview().ListLearnedPatterns(
		pattern.UserID,
		pattern.TraderID,
		DealReviewLearnedPatternFilter{StableKey: pattern.StableKey, Limit: 4},
	)
	if err != nil {
		t.Fatalf("ListLearnedPatterns() after rearm error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListLearnedPatterns() after rearm len = %d, want 1", len(items))
	}
	if items[0].RecommendedUse != DealReviewLearnedPatternRecommendedUseMonitoringRule {
		t.Fatalf("RecommendedUse after rearm = %q, want monitoring_rule", items[0].RecommendedUse)
	}
	if items[0].ManualControl == nil || items[0].ManualControl.ControlState != DealReviewLearnedPatternManualControlStateLive {
		t.Fatalf("ManualControl after rearm = %#v, want live", items[0].ManualControl)
	}
	if len(items[0].ManualControlHistory) != 2 {
		t.Fatalf("ManualControlHistory len after rearm = %d, want 2", len(items[0].ManualControlHistory))
	}
}
