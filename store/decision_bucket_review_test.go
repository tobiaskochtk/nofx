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
			},
			Decisions: []DecisionAction{
				{Action: "open_short", Symbol: "XRPUSDT"},
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
	if review.TotalCandidates != 3 {
		t.Fatalf("TotalCandidates = %d, want 3", review.TotalCandidates)
	}
	if review.TotalOpenDecisions != 2 {
		t.Fatalf("TotalOpenDecisions = %d, want 2", review.TotalOpenDecisions)
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
}
