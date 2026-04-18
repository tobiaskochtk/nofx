package store

import (
	"database/sql"
	"strings"
	"time"

	"nofx/logger"

	"gorm.io/gorm"
)

const (
	sqliteBusyTimeoutMillis   = 20000
	sqliteWriteRetryAttempts  = 4
	sqliteWriteRetryDelayBase = 200 * time.Millisecond
)

func configureSQLiteSQLDB(db *sql.DB) error {
	if db == nil {
		return nil
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = FULL",
		"PRAGMA busy_timeout = 20000",
	}
	for _, stmt := range pragmas {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

func retrySQLiteWrite(db *gorm.DB, operation string, fn func() error) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "sqlite" {
		return fn()
	}

	delay := sqliteWriteRetryDelayBase
	var lastErr error
	for attempt := 1; attempt <= sqliteWriteRetryAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !isRetryableSQLiteWriteError(lastErr) || attempt == sqliteWriteRetryAttempts {
			return lastErr
		}

		logger.Warnf("⚠️ SQLite transient write failure during %s (attempt %d/%d): %v", operation, attempt, sqliteWriteRetryAttempts, lastErr)
		time.Sleep(delay)
		delay *= 2
	}

	return lastErr
}

func isRetryableSQLiteWriteError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "database is locked"):
		return true
	case strings.Contains(msg, "database table is locked"):
		return true
	case strings.Contains(msg, "database schema is locked"):
		return true
	case strings.Contains(msg, "database is busy"):
		return true
	case strings.Contains(msg, "disk i/o error: permission denied"):
		return true
	default:
		return false
	}
}
