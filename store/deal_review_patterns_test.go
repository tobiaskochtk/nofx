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
	}

	for idx := range cases {
		if err := root.gdb.Create(&cases[idx]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", idx, err)
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
	if refresh.FeatureCount != len(cases) {
		t.Fatalf("refresh.FeatureCount = %d, want %d", refresh.FeatureCount, len(cases))
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
	if int(featureCount) != len(cases) {
		t.Fatalf("feature row count = %d, want %d", featureCount, len(cases))
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
	for _, pattern := range positivePatterns {
		if pattern.ScopeType == DealReviewLearnedPatternScopeTraderLocal &&
			pattern.PatternClass == DealReviewLearnedPatternClassPositiveEdge {
			foundTraderLocalPositive = true
		}
	}
	if !foundTraderLocalPositive {
		t.Fatal("expected at least one trader-local positive learned pattern for bucket_momentum")
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
			break
		}
	}
	if !foundMatchedRAVENegative {
		t.Fatal("expected a matched negative RAVEUSDT symbol pattern for the breakout setup")
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
