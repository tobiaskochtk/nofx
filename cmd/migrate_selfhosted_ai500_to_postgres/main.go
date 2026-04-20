package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"nofx/internal/selfhostedai500"

	_ "modernc.org/sqlite"
)

type migrationConfig struct {
	sqlitePath string
	pgHost     string
	pgPort     int
	pgUser     string
	pgPassword string
	pgDB       string
	pgSSLMode  string
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
}

func parseFlags() migrationConfig {
	cfg := migrationConfig{}
	flag.StringVar(&cfg.sqlitePath, "sqlite-path", envOrDefault("SELFHOSTED_AI500_SQLITE_PATH", "data/selfhosted-ai500/selfhosted-ai500.db"), "path to the source SQLite database")
	flag.StringVar(&cfg.pgHost, "pg-host", envOrDefault("SELFHOSTED_AI500_DB_HOST", "localhost"), "PostgreSQL host")
	flag.IntVar(&cfg.pgPort, "pg-port", envIntOrDefault("SELFHOSTED_AI500_DB_PORT", 5432), "PostgreSQL port")
	flag.StringVar(&cfg.pgUser, "pg-user", envOrDefault("SELFHOSTED_AI500_DB_USER", "nofx"), "PostgreSQL user")
	flag.StringVar(&cfg.pgPassword, "pg-password", os.Getenv("SELFHOSTED_AI500_DB_PASSWORD"), "PostgreSQL password")
	flag.StringVar(&cfg.pgDB, "pg-db", envOrDefault("SELFHOSTED_AI500_DB_NAME", "nofx"), "PostgreSQL database name")
	flag.StringVar(&cfg.pgSSLMode, "pg-sslmode", envOrDefault("SELFHOSTED_AI500_DB_SSLMODE", "disable"), "PostgreSQL sslmode")
	flag.Parse()
	return cfg
}

func run(cfg migrationConfig) error {
	source, err := sql.Open("sqlite", cfg.sqlitePath)
	if err != nil {
		return fmt.Errorf("open sqlite source: %w", err)
	}
	defer source.Close()

	targetStore, err := selfhostedai500.OpenStoreWithConfig(selfhostedai500.StoreConfig{
		Type:     selfhostedai500.DBTypePostgres,
		Host:     cfg.pgHost,
		Port:     cfg.pgPort,
		User:     cfg.pgUser,
		Password: cfg.pgPassword,
		DBName:   cfg.pgDB,
		SSLMode:  cfg.pgSSLMode,
	})
	if err != nil {
		return fmt.Errorf("open postgres target: %w", err)
	}
	defer targetStore.Close()

	target := targetStore.DB()
	if target == nil {
		return fmt.Errorf("target postgres database is nil")
	}

	tables := []string{"market_snapshots", "score_state"}
	if err := ensureTargetTablesEmpty(target, tables); err != nil {
		return err
	}

	tx, err := target.Begin()
	if err != nil {
		return fmt.Errorf("begin postgres transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	snapshotCount, err := copyMarketSnapshots(source, tx)
	if err != nil {
		return err
	}
	stateCount, err := copyScoreState(source, tx)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit postgres transaction: %w", err)
	}

	log.Printf("migrated selfhosted-ai500: market_snapshots=%d score_state=%d", snapshotCount, stateCount)
	return nil
}

func ensureTargetTablesEmpty(target *sql.DB, tables []string) error {
	for _, table := range tables {
		var count int64
		query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, table)
		if err := target.QueryRow(query).Scan(&count); err != nil {
			return fmt.Errorf("count target table %s: %w", table, err)
		}
		if count > 0 {
			return fmt.Errorf("target table %s is not empty (%d rows); aborting", table, count)
		}
	}
	return nil
}

func copyMarketSnapshots(source *sql.DB, target *sql.Tx) (int64, error) {
	rows, err := source.Query(`
		SELECT symbol, ts, price, open_interest, volume_24h, funding, premium, prev_day_price
		FROM market_snapshots
		ORDER BY symbol, ts
	`)
	if err != nil {
		return 0, fmt.Errorf("query sqlite market_snapshots: %w", err)
	}
	defer rows.Close()

	stmt, err := target.Prepare(`
		INSERT INTO market_snapshots(symbol, ts, price, open_interest, volume_24h, funding, premium, prev_day_price)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT(symbol, ts) DO UPDATE SET
			price = excluded.price,
			open_interest = excluded.open_interest,
			volume_24h = excluded.volume_24h,
			funding = excluded.funding,
			premium = excluded.premium,
			prev_day_price = excluded.prev_day_price
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare postgres market_snapshots insert: %w", err)
	}
	defer stmt.Close()

	var count int64
	for rows.Next() {
		var (
			symbol       string
			ts           int64
			price        float64
			openInterest float64
			volume24H    float64
			funding      float64
			premium      float64
			prevDayPrice float64
		)
		if err := rows.Scan(&symbol, &ts, &price, &openInterest, &volume24H, &funding, &premium, &prevDayPrice); err != nil {
			return count, fmt.Errorf("scan sqlite market_snapshots: %w", err)
		}
		if _, err := stmt.Exec(symbol, ts, price, openInterest, volume24H, funding, premium, prevDayPrice); err != nil {
			return count, fmt.Errorf("insert postgres market_snapshots after %d rows: %w", count, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("iterate sqlite market_snapshots: %w", err)
	}
	return count, nil
}

func copyScoreState(source *sql.DB, target *sql.Tx) (int64, error) {
	rows, err := source.Query(`
		SELECT symbol, start_time, start_price, last_score, max_score, max_price, last_updated_at
		FROM score_state
		ORDER BY symbol
	`)
	if err != nil {
		return 0, fmt.Errorf("query sqlite score_state: %w", err)
	}
	defer rows.Close()

	stmt, err := target.Prepare(`
		INSERT INTO score_state(symbol, start_time, start_price, last_score, max_score, max_price, last_updated_at)
		VALUES($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT(symbol) DO UPDATE SET
			start_time = excluded.start_time,
			start_price = excluded.start_price,
			last_score = excluded.last_score,
			max_score = excluded.max_score,
			max_price = excluded.max_price,
			last_updated_at = excluded.last_updated_at
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare postgres score_state insert: %w", err)
	}
	defer stmt.Close()

	var count int64
	for rows.Next() {
		var (
			symbol        string
			startTime     int64
			startPrice    float64
			lastScore     float64
			maxScore      float64
			maxPrice      float64
			lastUpdatedAt int64
		)
		if err := rows.Scan(&symbol, &startTime, &startPrice, &lastScore, &maxScore, &maxPrice, &lastUpdatedAt); err != nil {
			return count, fmt.Errorf("scan sqlite score_state: %w", err)
		}
		if _, err := stmt.Exec(symbol, startTime, startPrice, lastScore, maxScore, maxPrice, lastUpdatedAt); err != nil {
			return count, fmt.Errorf("insert postgres score_state after %d rows: %w", count, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("iterate sqlite score_state: %w", err)
	}
	return count, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
