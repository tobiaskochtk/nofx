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

func TestDecisionStoreGetBucketReview(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "decision-review.db"))
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

	store := NewDecisionStore(gdb)
	if err := store.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	now := time.Now().UTC()

	records := []*DecisionRecord{
		{
			TraderID:         "trader-1",
			CycleNumber:      100,
			Timestamp:        now.Add(-3 * time.Hour),
			CandidateMetaVer: DecisionCandidateMetadataVersion,
			CandidateDetails: []CandidateDetail{
				{Symbol: "ETHUSDT", SelectionBucket: "primary", Sources: []string{"ai500"}},
				{Symbol: "SOLUSDT", SelectionBucket: "adaptive", Sources: []string{"ai500"}},
			},
			Decisions: []DecisionAction{
				{Action: "open_long", Symbol: "ETHUSDT", Success: true, ExchangeOrderID: "1001"},
			},
			Success: true,
		},
		{
			TraderID:         "trader-1",
			CycleNumber:      101,
			Timestamp:        now.Add(-1 * time.Hour),
			CandidateMetaVer: DecisionCandidateMetadataVersion,
			CandidateDetails: []CandidateDetail{
				{Symbol: "XRPUSDT", SelectionBucket: "exploration", Sources: []string{"ai500"}},
				{Symbol: "DOGEUSDT", SelectionBucket: "primary", Sources: []string{"ai500"}},
			},
			Decisions: []DecisionAction{
				{Action: "open_short", Symbol: "XRPUSDT", Confidence: 84, Success: true, ExchangeOrderID: "1002"},
				{Action: "hold", Symbol: "DOGEUSDT", Confidence: 62, Reasoning: "confidence below threshold"},
				{Action: "wait", Confidence: 41, Reasoning: "spread too wide"},
			},
			Success: true,
		},
		{
			TraderID:         "trader-1",
			CycleNumber:      102,
			Timestamp:        now.Add(-30 * time.Minute),
			CandidateMetaVer: 0,
			CandidateCoins:   []string{"BTCUSDT"},
			Success:          true,
		},
	}

	for _, record := range records {
		if err := store.LogDecision(record); err != nil {
			t.Fatalf("LogDecision() error = %v", err)
		}
	}

	review, err := store.GetBucketReview("trader-1", 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("GetBucketReview() error = %v", err)
	}

	if review.RecordCount != 2 {
		t.Fatalf("RecordCount = %d, want 2", review.RecordCount)
	}
	if review.LegacyRecordCount != 1 {
		t.Fatalf("LegacyRecordCount = %d, want 1", review.LegacyRecordCount)
	}
	if review.TotalCandidates != 4 {
		t.Fatalf("TotalCandidates = %d, want 4", review.TotalCandidates)
	}
	if review.TotalOpenDecisions != 2 {
		t.Fatalf("TotalOpenDecisions = %d, want 2", review.TotalOpenDecisions)
	}
	if review.HoldDecisionCount != 1 {
		t.Fatalf("HoldDecisionCount = %d, want 1", review.HoldDecisionCount)
	}
	if review.WaitDecisionCount != 1 {
		t.Fatalf("WaitDecisionCount = %d, want 1", review.WaitDecisionCount)
	}
	if review.DecisionConversionRate != 50 {
		t.Fatalf("DecisionConversionRate = %.1f, want 50.0", review.DecisionConversionRate)
	}
	if review.AvgDecisionConfidence < 62 || review.AvgDecisionConfidence > 63 {
		t.Fatalf("AvgDecisionConfidence = %.1f, want about 62.3", review.AvgDecisionConfidence)
	}
	if review.CyclesWithCandidates != 2 {
		t.Fatalf("CyclesWithCandidates = %d, want 2", review.CyclesWithCandidates)
	}
	if len(review.Buckets) != 3 {
		t.Fatalf("len(Buckets) = %d, want 3", len(review.Buckets))
	}
	if len(review.RecentCycles) != 2 {
		t.Fatalf("len(RecentCycles) = %d, want 2", len(review.RecentCycles))
	}
	if review.RecentCycles[0].CycleNumber != 101 {
		t.Fatalf("RecentCycles[0].CycleNumber = %d, want 101", review.RecentCycles[0].CycleNumber)
	}
	if review.CoverageHours < 2.9 || review.CoverageHours > 3.2 {
		t.Fatalf("CoverageHours = %.1f, want about 3h", review.CoverageHours)
	}
	if len(review.RejectReasons) < 2 {
		t.Fatalf("RejectReasons = %#v, want at least two entries", review.RejectReasons)
	}
	if review.RejectReasons[0].Reason != "low_confidence" &&
		review.RejectReasons[0].Reason != "spread_slippage_concerns" &&
		review.RejectReasons[0].Reason != "unspecified" {
		t.Fatalf("RejectReasons[0] = %#v, want normalized reject reason", review.RejectReasons[0])
	}
	if review.RejectedCandidateCount != 2 {
		t.Fatalf("RejectedCandidateCount = %d, want 2", review.RejectedCandidateCount)
	}
	if len(review.ConfidenceBands) != 3 {
		t.Fatalf("ConfidenceBands = %#v, want three populated bands", review.ConfidenceBands)
	}
	if len(review.OpportunitySessions) == 0 || review.OpportunitySessions[0].Session == "" {
		t.Fatalf("OpportunitySessions = %#v, want populated session summaries", review.OpportunitySessions)
	}
	if len(review.OpportunitySymbols) < 2 {
		t.Fatalf("OpportunitySymbols = %#v, want populated symbol summaries", review.OpportunitySymbols)
	}
	if len(review.ExecutionStatuses) == 0 || review.ExecutionStatuses[0].Status != "submitted" {
		t.Fatalf("ExecutionStatuses = %#v, want submitted execution status", review.ExecutionStatuses)
	}
}

func TestDecisionStoreGetBucketReviewPersistsRegimeTelemetry(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "decision-review-regime.db"))
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

	store := NewDecisionStore(gdb)
	if err := store.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	record := &DecisionRecord{
		TraderID:         "trader-regime",
		CycleNumber:      7,
		Timestamp:        time.Now().UTC().Add(-20 * time.Minute),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		InputPrompt:      `{"candidates":[{"sym":"ETHUSDT","px_mark":2500,"ctx":{"tf":"15m","px_type":"mark","ema_fast":2482,"macd":1.4,"rsi":61,"oi_d1h_pct":2.1,"fund_bps":0.9,"basis_pct":0.2},"venue_tradability":{"supported":true,"book":"live","price_source":"orderbook"},"execution_quality":{"spread_bps":2.3,"liq_score":0.82,"slippage_est_25usd":0.6,"slippage_est_100usd":1.1},"feature_availability":{"freshness":"fresh"},"relative_strength":{"vs_btc_1h":0.4,"state":"strong"},"feat":{"volatility":{"regime":"expansion"}}},{"sym":"DOGEUSDT","px_mark":0.14,"ctx":{"tf":"15m","px_type":"mark","ema_fast":0.139,"macd":0.02,"rsi":48,"oi_d1h_pct":-1.5,"fund_bps":-0.1,"basis_pct":0.05},"venue_tradability":{"supported":true,"book":"live","price_source":"orderbook"},"execution_quality":{"spread_bps":14.1,"liq_score":0.21,"slippage_est_25usd":6.8,"slippage_est_100usd":18.4},"feature_availability":{"freshness":"fresh"},"relative_strength":{"vs_btc_1h":-0.2,"state":"mixed"},"feat":{"volatility":{"regime":"compression"}}}]}`,
		CandidateDetails: []CandidateDetail{
			{Symbol: "ETHUSDT", SelectionBucket: "primary", Sources: []string{"ai500"}},
			{Symbol: "DOGEUSDT", SelectionBucket: "exploration", Sources: []string{"ai500"}},
		},
		Decisions: []DecisionAction{
			{Action: "open_long", Symbol: "ETHUSDT", Confidence: 86, Success: true, ExchangeOrderID: "2001"},
			{Action: "hold", Symbol: "DOGEUSDT", Confidence: 49, Reasoning: "waiting for confirmation"},
		},
		Success: true,
	}
	if err := store.LogDecision(record); err != nil {
		t.Fatalf("LogDecision() error = %v", err)
	}

	review, err := store.GetBucketReview("trader-regime", 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("GetBucketReview() error = %v", err)
	}
	if len(review.RecentOpenExecutions) != 1 {
		t.Fatalf("RecentOpenExecutions = %#v, want one execution sample", review.RecentOpenExecutions)
	}
	if review.RecentOpenExecutions[0].LiquidityTier == "" {
		t.Fatalf("RecentOpenExecutions[0] = %#v, want persisted liquidity tier", review.RecentOpenExecutions[0])
	}
	if len(review.RegimeSummaries) == 0 {
		t.Fatalf("RegimeSummaries = %#v, want persisted regime summary", review.RegimeSummaries)
	}
	if review.RegimeSummaries[0].TrendRegime == "" || review.RegimeSummaries[0].VolatilityRegime == "" {
		t.Fatalf("RegimeSummaries[0] = %#v, want trend and volatility regime", review.RegimeSummaries[0])
	}
}

func TestDecisionStoreGetBucketReviewRecordsExplicitRejectReasons(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "decision-review-reject-reasons.db"))
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

	store := NewDecisionStore(gdb)
	if err := store.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	record := &DecisionRecord{
		TraderID:         "trader-rejects",
		CycleNumber:      11,
		Timestamp:        time.Now().UTC().Add(-15 * time.Minute),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		CandidateDetails: []CandidateDetail{
			{Symbol: "ENAUSDT", SelectionBucket: "primary"},
			{
				Symbol:          "RAVEUSDT",
				SelectionBucket: "adaptive",
				MarketContext: &DealReviewMarketContextSnapshot{
					TrendRegime:      "uptrend",
					VolatilityRegime: "high_vol",
					OIRegime:         "flat",
					RSI:              floatPtr(72),
				},
			},
		},
		Decisions: []DecisionAction{
			{Action: "hold", Symbol: "ENAUSDT", Reasoning: "late breakout, OI flat, volume not confirming after the move"},
			{Action: "wait", Symbol: "RAVEUSDT", Reasoning: "same-symbol cooldown after recent loss"},
		},
		Success: true,
	}
	if err := store.LogDecision(record); err != nil {
		t.Fatalf("LogDecision() error = %v", err)
	}

	review, err := store.GetBucketReview("trader-rejects", 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("GetBucketReview() error = %v", err)
	}

	reasons := map[string]int{}
	for _, item := range review.RejectReasons {
		reasons[item.Reason] = item.Count
	}
	for _, reason := range []string{"late_breakout", "oi_not_confirming", "volume_not_confirming", "same_symbol_cooldown"} {
		if reasons[reason] == 0 {
			t.Fatalf("RejectReasons = %#v, want %q to be counted", review.RejectReasons, reason)
		}
	}
	if reasons["unspecified"] != 0 {
		t.Fatalf("RejectReasons = %#v, want explicit reasons instead of unspecified", review.RejectReasons)
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
