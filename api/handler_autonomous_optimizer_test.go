package api

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"nofx/store"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func TestShouldSelfPauseAutonomousOptimizerAfterThreeLowEvidenceRuns(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autonomous-optimizer.db"))
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

	root, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("store.NewFromGorm() error = %v", err)
	}
	if err := gdb.AutoMigrate(&store.AutonomousOptimizerRun{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	userID := "user-1"
	traderID := "trader-1"
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		run := &store.AutonomousOptimizerRun{
			UserID:           userID,
			TraderID:         traderID,
			ConfigID:         "cfg-1",
			Trigger:          store.AutonomousOptimizerRunTriggerSchedule,
			Status:           store.AutonomousOptimizerStatusInsufficientEvidence,
			PrimaryModelName: "gpt-5.4",
			CriticModelName:  "gpt-5.4",
			StartedAt:        now.Add(time.Duration(i) * time.Minute),
			CompletedAt:      now.Add(time.Duration(i) * time.Minute),
		}
		if err := root.AutonomousOptimizer().SaveRun(run); err != nil {
			t.Fatalf("SaveRun(%d) error = %v", i, err)
		}
		if i == 2 && run.ID == "" {
			t.Fatalf("SaveRun(%d) did not assign ID", i)
		}
	}

	runs, err := root.AutonomousOptimizer().ListRuns(userID, traderID, 3)
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 3 || runs[0] == nil {
		t.Fatalf("ListRuns() = %#v, want 3 runs", runs)
	}

	if !shouldSelfPauseAutonomousOptimizer(store.AutonomousOptimizerStatusInsufficientEvidence, userID, traderID, runs[0].ID, root) {
		t.Fatalf("shouldSelfPauseAutonomousOptimizer() = false, want true")
	}
}
