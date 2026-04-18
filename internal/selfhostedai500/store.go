package selfhostedai500

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
)

type StoreConfig struct {
	Type     DBType
	Path     string
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type Store struct {
	db     *sql.DB
	dbType DBType
}

func OpenStore(dbPath string) (*Store, error) {
	return OpenStoreWithConfig(StoreConfig{
		Type: DBTypeSQLite,
		Path: dbPath,
	})
}

func OpenStoreWithConfig(cfg StoreConfig) (*Store, error) {
	cfg.Type = normalizeStoreDBType(string(cfg.Type))
	if cfg.Type == "" {
		cfg.Type = DBTypePostgres
	}

	var (
		db  *sql.DB
		err error
	)

	switch cfg.Type {
	case DBTypePostgres:
		db, err = openPostgres(cfg)
	default:
		db, err = openSQLite(cfg)
	}
	if err != nil {
		return nil, err
	}

	store := &Store{db: db, dbType: cfg.Type}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func openSQLite(cfg StoreConfig) (*sql.DB, error) {
	if strings.TrimSpace(cfg.Path) == "" {
		return nil, fmt.Errorf("db path is required for sqlite")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	return db, nil
}

func openPostgres(cfg StoreConfig) (*sql.DB, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("db host is required for postgres")
	}
	if strings.TrimSpace(cfg.User) == "" {
		return nil, fmt.Errorf("db user is required for postgres")
	}
	if strings.TrimSpace(cfg.DBName) == "" {
		return nil, fmt.Errorf("db name is required for postgres")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(0)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func normalizeStoreDBType(raw string) DBType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(DBTypePostgres):
		return DBTypePostgres
	case string(DBTypeSQLite):
		return DBTypeSQLite
	default:
		return DBTypePostgres
	}
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Store) init() error {
	statements := []string{
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

	if s.dbType == DBTypeSQLite {
		statements = append([]string{
			`PRAGMA journal_mode = WAL;`,
			`PRAGMA busy_timeout = 5000;`,
		}, statements...)
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
	_, err := s.exec(
		`INSERT INTO market_snapshots(symbol, ts, price, open_interest, volume_24h, funding, premium, prev_day_price)
         VALUES(?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(symbol, ts) DO UPDATE SET
             price = excluded.price,
             open_interest = excluded.open_interest,
             volume_24h = excluded.volume_24h,
             funding = excluded.funding,
             premium = excluded.premium,
             prev_day_price = excluded.prev_day_price`,
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
		s.query(`SELECT ts, price, open_interest
         FROM market_snapshots
         WHERE symbol = ? AND ts <= ?
         ORDER BY ts DESC
         LIMIT 1`),
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
	_, err := s.exec(
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
	_, err := s.exec(`DELETE FROM market_snapshots WHERE ts < ?`, before.UTC().Unix())
	return err
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(s.query(query), args...)
}

func (s *Store) query(query string) string {
	if s.dbType != DBTypePostgres {
		return query
	}

	var builder strings.Builder
	index := 1
	for _, r := range query {
		if r == '?' {
			builder.WriteString(fmt.Sprintf("$%d", index))
			index++
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}
