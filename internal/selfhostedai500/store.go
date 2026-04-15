package selfhostedai500

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(dbPath string) (*Store, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("db path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &Store{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) init() error {
	statements := []string{
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
		`CREATE TABLE IF NOT EXISTS market_snapshots (
            symbol TEXT NOT NULL,
            ts INTEGER NOT NULL,
            price REAL NOT NULL,
            open_interest REAL NOT NULL,
            volume_24h REAL NOT NULL,
            funding REAL NOT NULL,
            premium REAL NOT NULL,
            prev_day_price REAL NOT NULL,
            PRIMARY KEY(symbol, ts)
        );`,
		`CREATE INDEX IF NOT EXISTS idx_market_snapshots_symbol_ts ON market_snapshots(symbol, ts DESC);`,
		`CREATE TABLE IF NOT EXISTS score_state (
            symbol TEXT PRIMARY KEY,
            start_time INTEGER NOT NULL,
            start_price REAL NOT NULL,
            last_score REAL NOT NULL,
            max_score REAL NOT NULL,
            max_price REAL NOT NULL,
            last_updated_at INTEGER NOT NULL
        );`,
	}

	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) InsertSnapshot(snapshot *marketSnapshot) error {
	if snapshot == nil {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO market_snapshots(symbol, ts, price, open_interest, volume_24h, funding, premium, prev_day_price)
         VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.Symbol,
		snapshot.UpdatedAt.UTC().Unix(),
		snapshot.Price,
		snapshot.OpenInterest,
		snapshot.Volume24H,
		snapshot.Funding,
		snapshot.Premium,
		snapshot.PrevDayPrice,
	)
	return err
}

func (s *Store) LatestSnapshotBefore(symbol string, target time.Time) (*historyPoint, error) {
	row := s.db.QueryRow(
		`SELECT ts, price, open_interest
         FROM market_snapshots
         WHERE symbol = ? AND ts <= ?
         ORDER BY ts DESC
         LIMIT 1`,
		symbol,
		target.UTC().Unix(),
	)

	var ts int64
	var point historyPoint
	point.Symbol = symbol
	if err := row.Scan(&ts, &point.Price, &point.OpenInterest); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	point.Timestamp = time.Unix(ts, 0).UTC()
	return &point, nil
}

func (s *Store) LoadScoreState() (map[string]scoreStateRecord, error) {
	rows, err := s.db.Query(`SELECT symbol, start_time, start_price, last_score, max_score, max_price, last_updated_at FROM score_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]scoreStateRecord)
	for rows.Next() {
		var item scoreStateRecord
		if err := rows.Scan(&item.Symbol, &item.StartTime, &item.StartPrice, &item.LastScore, &item.MaxScore, &item.MaxPrice, &item.LastUpdatedAt); err != nil {
			return nil, err
		}
		result[item.Symbol] = item
	}
	return result, rows.Err()
}

func (s *Store) SaveScoreState(snapshot *marketSnapshot) error {
	if snapshot == nil {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO score_state(symbol, start_time, start_price, last_score, max_score, max_price, last_updated_at)
         VALUES(?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(symbol) DO UPDATE SET
             start_time = excluded.start_time,
             start_price = excluded.start_price,
             last_score = excluded.last_score,
             max_score = excluded.max_score,
             max_price = excluded.max_price,
             last_updated_at = excluded.last_updated_at`,
		snapshot.Symbol,
		snapshot.StartTime,
		snapshot.StartPrice,
		snapshot.LastScore,
		snapshot.MaxScore,
		snapshot.MaxPrice,
		snapshot.UpdatedAt.UTC().Unix(),
	)
	return err
}

func (s *Store) PruneSnapshots(before time.Time) error {
	_, err := s.db.Exec(`DELETE FROM market_snapshots WHERE ts < ?`, before.UTC().Unix())
	return err
}
