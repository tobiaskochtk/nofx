package store

import (
	"fmt"
	"testing"
	"time"
)

func TestDealReviewCreateInBatches_LearnedPatterns(t *testing.T) {
	root := newPositionHistoryTestStore(t, "deal-review-bulk-patterns.db")

	now := time.Now().UTC()
	rows := make([]DealReviewLearnedPattern, 0, 1500)
	for i := 0; i < 1500; i++ {
		rows = append(rows, DealReviewLearnedPattern{
			ID:                        fmt.Sprintf("pattern-%d", i),
			UserID:                    "user-bulk",
			TraderID:                  "trader-bulk",
			ScopeType:                 DealReviewLearnedPatternScopeTraderLocal,
			ScopeKey:                  DealReviewLearnedPatternScopeTraderLocal,
			Side:                      "LONG",
			PatternClass:              DealReviewLearnedPatternClassNegativeEdge,
			Status:                    DealReviewSymbolBehaviorPriorStatusCandidate,
			ValidationLabel:           DealReviewLearnedPatternValidationLabelCandidate,
			RecommendedUse:            DealReviewLearnedPatternRecommendedUseMonitorOnly,
			PatternSignature:          fmt.Sprintf("bucket:momentum|trend:uptrend|idx:%d", i),
			RegimeSignature:           "trend:uptrend | vol:medium | oi:rising | session:asia",
			PatternOrder:              2,
			FeatureCount:              2,
			FeatureSetJSON:            `["bucket:momentum","trend:uptrend"]`,
			SampleCount:               12,
			WinningDeals:              3,
			LosingDeals:               8,
			FlatDeals:                 1,
			SupportCount:              8,
			ContradictCount:           4,
			WinRate:                   0.25,
			LossRate:                  0.67,
			NetPnL:                    -1.25,
			AvgPnL:                    -0.10,
			AvgPnLPct:                 -0.85,
			Expectancy:                -0.10,
			AvgMFEPct:                 0.42,
			AvgMAEPct:                 -1.10,
			GiveBackRate:              0.30,
			AvgGiveBackPct:            18,
			AvgHoldMs:                 int64((25 * time.Minute) / time.Millisecond),
			BaselineWinRate:           0.52,
			BaselineAvgPnLPct:         -0.05,
			LiftWinRate:               -0.27,
			LiftAvgPnLPct:             -0.80,
			TrainingSampleCount:       8,
			ValidationSampleCount:     2,
			ValidationSupportCount:    1,
			ValidationContradictCount: 1,
			ValidationAvgPnLPct:       -0.35,
			ValidationSupportScore:    0.50,
			RecentSampleCount:         2,
			RecentSupportCount:        2,
			RecentContradictCount:     0,
			RecentAvgPnLPct:           -0.90,
			RecentSupportScore:        1.0,
			ConfidenceScore:           0.88,
			StabilityScore:            0.81,
			DriftScore:                0.09,
			RecencyWeight:             0.92,
			CompositeScore:            0.86,
			FalsePositiveScore:        0.04,
			ReverseRiskScore:          0.07,
			Summary:                   "Bulk insert regression row for learned patterns.",
			ValidationAlert:           "Holdout support remains stable.",
			EvidenceJSON:              `[{"case_id":"case-1","position_id":1,"symbol":"ALPHAUSDT","side":"LONG","outcome":"loss","realized_pnl":-0.4,"realized_pnl_pct":-0.8}]`,
			FirstObservedAt:           now.Add(-48 * time.Hour),
			LastObservedAt:            now.Add(-1 * time.Hour),
			BuiltAt:                   now,
		})
	}

	if err := dealReviewCreateInBatches(root.gdb, rows); err != nil {
		t.Fatalf("dealReviewCreateInBatches(learned_patterns) error = %v", err)
	}

	var count int64
	if err := root.gdb.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ?", "user-bulk", "trader-bulk").
		Count(&count).Error; err != nil {
		t.Fatalf("Count(learned_patterns) error = %v", err)
	}
	if count != int64(len(rows)) {
		t.Fatalf("learned pattern count = %d, want %d", count, len(rows))
	}
}

func TestDealReviewCreateInBatches_PatternFeatureRows(t *testing.T) {
	root := newPositionHistoryTestStore(t, "deal-review-bulk-feature-rows.db")

	now := time.Now().UTC()
	rows := make([]DealReviewPatternFeatureRecord, 0, 4000)
	for i := 0; i < 4000; i++ {
		rows = append(rows, DealReviewPatternFeatureRecord{
			UserID:             "user-bulk",
			TraderID:           "trader-bulk",
			CaseID:             fmt.Sprintf("case-%d", i),
			PositionID:         int64(i + 1),
			Symbol:             "ALPHAUSDT",
			Side:               "LONG",
			Outcome:            "profit",
			OpenConfidence:     82,
			RealizedPnL:        0.42,
			RealizedPnLPct:     0.84,
			MaxMFEPct:          1.40,
			MaxMAEPct:          -0.30,
			ProfitGivenBackPct: 12,
			HoldDurationMs:     int64((14 * time.Minute) / time.Millisecond),
			FeatureCount:       4,
			FeaturesJSON:       `["bucket:momentum","trend:uptrend","oi:rising","session:asia"]`,
			QualityFlagsJSON:   `[]`,
			ObservedAt:         now.Add(-time.Duration(i) * time.Minute),
			BuiltAt:            now,
		})
	}

	if err := dealReviewCreateInBatches(root.gdb, rows); err != nil {
		t.Fatalf("dealReviewCreateInBatches(pattern_features) error = %v", err)
	}

	var count int64
	if err := root.gdb.Model(&DealReviewPatternFeatureRecord{}).
		Where("user_id = ? AND trader_id = ?", "user-bulk", "trader-bulk").
		Count(&count).Error; err != nil {
		t.Fatalf("Count(pattern_features) error = %v", err)
	}
	if count != int64(len(rows)) {
		t.Fatalf("pattern feature row count = %d, want %d", count, len(rows))
	}
}
