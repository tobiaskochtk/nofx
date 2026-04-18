package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestConfigureSQLiteSQLDBEnablesWALAndBusyTimeout(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "sqlite-runtime-test.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	if err := configureSQLiteSQLDB(db); err != nil {
		t.Fatalf("configureSQLiteSQLDB() error = %v", err)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode scan error = %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout scan error = %v", err)
	}
	if busyTimeout != sqliteBusyTimeoutMillis {
		t.Fatalf("busy_timeout = %d, want %d", busyTimeout, sqliteBusyTimeoutMillis)
	}
}

func TestRetrySQLiteWriteRetriesTransientLockErrors(t *testing.T) {
	root := newPositionHistoryTestStore(t, "sqlite-retry.db")

	attempts := 0
	err := retrySQLiteWrite(root.gdb, "unit-test transient retry", func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("disk I/O error: permission denied")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retrySQLiteWrite() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetrySQLiteWriteDoesNotRetryNonTransientErrors(t *testing.T) {
	root := newPositionHistoryTestStore(t, "sqlite-no-retry.db")

	attempts := 0
	err := retrySQLiteWrite(root.gdb, "unit-test non-transient", func() error {
		attempts++
		return fmt.Errorf("permanent failure")
	})
	if err == nil {
		t.Fatal("retrySQLiteWrite() error = nil, want non-nil")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
