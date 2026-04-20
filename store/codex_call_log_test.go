package store

import (
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func TestCodexCallLogRetention(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "codex-call-log.db"))
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

	root, err := NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("NewFromGorm() error = %v", err)
	}
	if err := root.CodexCallLog().initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	base := time.Date(2026, 4, 20, 13, 0, 0, 0, time.UTC)
	for i := 1; i <= CodexCallLogRetention+5; i++ {
		finished := base.Add(time.Duration(i) * time.Second)
		err := root.CodexCallLog().Record(&CodexCallLog{
			RequestStartedAt:   finished.Add(-500 * time.Millisecond),
			ResponseFinishedAt: &finished,
			UserID:             "user-1",
			TraderID:           "trader-1",
			TraderName:         "Gamma-Ray",
			Provider:           "codex",
			Model:              "gpt-5.4",
			Transport:          "codex_cli",
			RequestPrompt:      "prompt-" + strconv.Itoa(i),
			ResponseText:       "response-" + strconv.Itoa(i),
			Success:            true,
			ExitCode:           0,
			DurationMs:         500,
		})
		if err != nil {
			t.Fatalf("Record(%d) error = %v", i, err)
		}
	}

	items, err := root.CodexCallLog().ListRecentForUser("user-1", "trader-1", CodexCallLogRetention)
	if err != nil {
		t.Fatalf("ListRecentForUser() error = %v", err)
	}
	if len(items) != CodexCallLogRetention {
		t.Fatalf("len(items) = %d, want %d", len(items), CodexCallLogRetention)
	}
	if items[0].ResponseText != "response-105" {
		t.Fatalf("items[0].ResponseText = %q, want response-105", items[0].ResponseText)
	}
	if items[len(items)-1].ResponseText != "response-6" {
		t.Fatalf("items[last].ResponseText = %q, want response-6", items[len(items)-1].ResponseText)
	}
}
