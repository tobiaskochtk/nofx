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
				{Action: "open_long", Symbol: "ETHUSDT"},
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
				{Action: "open_short", Symbol: "XRPUSDT", Confidence: 84},
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
	if review.RejectReasons[0].Reason != "execution_quality" && review.RejectReasons[0].Reason != "low_confidence" {
		t.Fatalf("RejectReasons[0] = %#v, want normalized reject reason", review.RejectReasons[0])
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
}
