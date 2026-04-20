package store

import (
	"database/sql"
)

const (
	sqliteBusyTimeoutMillis = 20000
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
