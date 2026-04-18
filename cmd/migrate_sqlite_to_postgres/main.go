package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"nofx/store"

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

type postgresColumn struct {
	Name     string
	DataType string
	Nullable bool
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
}

func parseFlags() migrationConfig {
	cfg := migrationConfig{}
	flag.StringVar(&cfg.sqlitePath, "sqlite-path", envOrDefault("SQLITE_PATH", "data/data.db"), "path to the source SQLite database")
	flag.StringVar(&cfg.pgHost, "pg-host", envOrDefault("DB_HOST", "localhost"), "PostgreSQL host")
	flag.IntVar(&cfg.pgPort, "pg-port", envIntOrDefault("DB_PORT", 5432), "PostgreSQL port")
	flag.StringVar(&cfg.pgUser, "pg-user", envOrDefault("DB_USER", "nofx"), "PostgreSQL user")
	flag.StringVar(&cfg.pgPassword, "pg-password", os.Getenv("DB_PASSWORD"), "PostgreSQL password")
	flag.StringVar(&cfg.pgDB, "pg-db", envOrDefault("DB_NAME", "nofx"), "PostgreSQL database name")
	flag.StringVar(&cfg.pgSSLMode, "pg-sslmode", envOrDefault("DB_SSLMODE", "disable"), "PostgreSQL sslmode")
	flag.Parse()
	return cfg
}

func run(cfg migrationConfig) error {
	if strings.TrimSpace(cfg.sqlitePath) == "" {
		return fmt.Errorf("sqlite-path is required")
	}

	source, err := sql.Open("sqlite", cfg.sqlitePath)
	if err != nil {
		return fmt.Errorf("open sqlite source: %w", err)
	}
	defer source.Close()

	source.SetMaxOpenConns(1)
	source.SetMaxIdleConns(1)

	targetStore, err := store.NewWithConfig(store.DBConfig{
		Type:     store.DBTypePostgres,
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

	target, err := targetStore.GormDB().DB()
	if err != nil {
		return fmt.Errorf("get postgres sql.DB: %w", err)
	}

	sourceTables, err := listSQLiteTables(source)
	if err != nil {
		return fmt.Errorf("list sqlite tables: %w", err)
	}
	if len(sourceTables) == 0 {
		return fmt.Errorf("no SQLite tables found in %s", cfg.sqlitePath)
	}

	targetTables, err := listPostgresTables(target)
	if err != nil {
		return fmt.Errorf("list postgres tables: %w", err)
	}

	migratable, skipped := intersectTables(sourceTables, targetTables)
	if len(migratable) == 0 {
		return fmt.Errorf("no overlapping tables found between SQLite source and PostgreSQL target")
	}

	log.Printf("source tables: %s", strings.Join(sourceTables, ", "))
	log.Printf("target tables: %s", strings.Join(targetTables, ", "))
	log.Printf("migrating tables: %s", strings.Join(migratable, ", "))
	if len(skipped) > 0 {
		log.Printf("skipping SQLite-only tables not present in target schema: %s", strings.Join(skipped, ", "))
	}

	if err := ensureTargetTablesEmpty(target, migratable); err != nil {
		return err
	}

	order, err := buildDependencyOrder(source, migratable)
	if err != nil {
		return fmt.Errorf("build table dependency order: %w", err)
	}
	log.Printf("dependency order: %s", strings.Join(order, " -> "))

	tx, err := target.Begin()
	if err != nil {
		return fmt.Errorf("begin postgres transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, table := range order {
		copied, err := copyTable(source, tx, table)
		if err != nil {
			return err
		}
		log.Printf("copied %d rows into %s", copied, table)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit postgres transaction: %w", err)
	}

	if err := resetPostgresSequences(target, migratable); err != nil {
		return fmt.Errorf("reset postgres sequences: %w", err)
	}

	log.Printf("SQLite -> PostgreSQL migration completed successfully")
	return nil
}

func listSQLiteTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func listPostgresTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func intersectTables(sourceTables, targetTables []string) ([]string, []string) {
	targetSet := make(map[string]struct{}, len(targetTables))
	for _, table := range targetTables {
		targetSet[table] = struct{}{}
	}

	var migratable []string
	var skipped []string
	for _, table := range sourceTables {
		if _, ok := targetSet[table]; ok {
			migratable = append(migratable, table)
			continue
		}
		skipped = append(skipped, table)
	}
	return migratable, skipped
}

func ensureTargetTablesEmpty(target *sql.DB, tables []string) error {
	for _, table := range tables {
		var count int64
		query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quotePostgresIdent(table))
		if err := target.QueryRow(query).Scan(&count); err != nil {
			return fmt.Errorf("count target table %s: %w", table, err)
		}
		if count > 0 {
			return fmt.Errorf("target table %s is not empty (%d rows); aborting to avoid duplicate imports", table, count)
		}
	}
	return nil
}

func buildDependencyOrder(source *sql.DB, tables []string) ([]string, error) {
	tableSet := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		tableSet[table] = struct{}{}
	}

	deps := make(map[string]map[string]struct{}, len(tables))
	reverse := make(map[string]map[string]struct{}, len(tables))
	indegree := make(map[string]int, len(tables))

	for _, table := range tables {
		deps[table] = make(map[string]struct{})
		reverse[table] = make(map[string]struct{})
	}

	for _, table := range tables {
		refs, err := sqliteForeignKeys(source, table)
		if err != nil {
			return nil, fmt.Errorf("read foreign keys for %s: %w", table, err)
		}
		for _, ref := range refs {
			if ref == table {
				continue
			}
			if _, ok := tableSet[ref]; !ok {
				continue
			}
			if _, exists := deps[table][ref]; exists {
				continue
			}
			deps[table][ref] = struct{}{}
			reverse[ref][table] = struct{}{}
			indegree[table]++
		}
	}

	var ready []string
	for _, table := range tables {
		if indegree[table] == 0 {
			ready = append(ready, table)
		}
	}
	sort.Strings(ready)

	var order []string
	for len(ready) > 0 {
		table := ready[0]
		ready = ready[1:]
		order = append(order, table)

		var dependents []string
		for dependent := range reverse[table] {
			dependents = append(dependents, dependent)
		}
		sort.Strings(dependents)

		for _, dependent := range dependents {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sort.Strings(ready)
	}

	if len(order) == len(tables) {
		return order, nil
	}

	remaining := make([]string, 0, len(tables)-len(order))
	done := make(map[string]struct{}, len(order))
	for _, table := range order {
		done[table] = struct{}{}
	}
	for _, table := range tables {
		if _, ok := done[table]; !ok {
			remaining = append(remaining, table)
		}
	}
	sort.Strings(remaining)
	return append(order, remaining...), nil
}

func sqliteForeignKeys(source *sql.DB, table string) ([]string, error) {
	query := fmt.Sprintf(`PRAGMA foreign_key_list(%s)`, quoteSQLiteIdent(table))
	rows, err := source.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []string
	seen := make(map[string]struct{})
	for rows.Next() {
		var (
			id       int
			seq      int
			refTable string
			fromCol  string
			toCol    string
			onUpdate string
			onDelete string
			match    string
		)
		if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		if _, ok := seen[refTable]; ok {
			continue
		}
		seen[refTable] = struct{}{}
		refs = append(refs, refTable)
	}
	sort.Strings(refs)
	return refs, rows.Err()
}

func copyTable(source *sql.DB, target *sql.Tx, table string) (int64, error) {
	sourceColumns, err := sqliteColumns(source, table)
	if err != nil {
		return 0, fmt.Errorf("load sqlite columns for %s: %w", table, err)
	}
	targetColumns, err := postgresColumns(target, table)
	if err != nil {
		return 0, fmt.Errorf("load postgres columns for %s: %w", table, err)
	}

	targetColumnMap := make(map[string]postgresColumn, len(targetColumns))
	targetColumnSet := make(map[string]struct{}, len(targetColumns))
	for _, column := range targetColumns {
		targetColumnMap[column.Name] = column
		targetColumnSet[column.Name] = struct{}{}
	}

	var columns []string
	for _, column := range sourceColumns {
		if _, ok := targetColumnSet[column]; ok {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return 0, nil
	}

	sourceCount, err := countRows(source, table)
	if err != nil {
		return 0, fmt.Errorf("count sqlite rows for %s: %w", table, err)
	}
	if sourceCount == 0 {
		return 0, nil
	}

	selectSQL := fmt.Sprintf(
		`SELECT %s FROM %s`,
		joinQuoted(columns, quotePostgresIdent),
		quotePostgresIdent(table),
	)
	rows, err := source.Query(selectSQL)
	if err != nil {
		return 0, fmt.Errorf("select sqlite rows for %s: %w", table, err)
	}
	defer rows.Close()

	insertSQL := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		quotePostgresIdent(table),
		joinQuoted(columns, quotePostgresIdent),
		makeDollarPlaceholders(len(columns)),
	)
	stmt, err := target.Prepare(insertSQL)
	if err != nil {
		return 0, fmt.Errorf("prepare postgres insert for %s: %w", table, err)
	}
	defer stmt.Close()

	var copied int64
	for rows.Next() {
		rawValues := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range rawValues {
			dest[i] = &rawValues[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return copied, fmt.Errorf("scan sqlite row for %s: %w", table, err)
		}

		values := make([]any, len(columns))
		for i, column := range columns {
			targetColumn := targetColumnMap[column]
			value, warningText, err := normalizeValueForTarget(rawValues[i], targetColumn)
			if err != nil {
				return copied, fmt.Errorf("normalize %s.%s: %w", table, column, err)
			}
			if warningText != "" {
				log.Printf("warning: %s.%s row %d: %s", table, column, copied+1, warningText)
			}
			values[i] = value
		}

		if _, err := stmt.Exec(values...); err != nil {
			return copied, fmt.Errorf("insert postgres row for %s after %d rows: %w", table, copied, err)
		}
		copied++
	}
	if err := rows.Err(); err != nil {
		return copied, fmt.Errorf("iterate sqlite rows for %s: %w", table, err)
	}
	if copied != sourceCount {
		return copied, fmt.Errorf("row-count mismatch for %s: copied %d, expected %d", table, copied, sourceCount)
	}
	return copied, nil
}

func sqliteColumns(source *sql.DB, table string) ([]string, error) {
	query := fmt.Sprintf(`PRAGMA table_info(%s)`, quoteSQLiteIdent(table))
	rows, err := source.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

func postgresColumns(target *sql.Tx, table string) ([]postgresColumn, error) {
	rows, err := target.Query(`
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []postgresColumn
	for rows.Next() {
		var column postgresColumn
		var isNullable string
		if err := rows.Scan(&column.Name, &column.DataType, &isNullable); err != nil {
			return nil, err
		}
		column.Nullable = strings.EqualFold(isNullable, "YES")
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func countRows(db *sql.DB, table string) (int64, error) {
	var count int64
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quotePostgresIdent(table))
	if err := db.QueryRow(query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func resetPostgresSequences(target *sql.DB, migratedTables []string) error {
	tableSet := make(map[string]struct{}, len(migratedTables))
	for _, table := range migratedTables {
		tableSet[table] = struct{}{}
	}

	rows, err := target.Query(`
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND column_default LIKE 'nextval(%'
		ORDER BY table_name, ordinal_position
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var table string
		var column string
		if err := rows.Scan(&table, &column); err != nil {
			return err
		}
		if _, ok := tableSet[table]; !ok {
			continue
		}
		stmt := fmt.Sprintf(
			`SELECT setval(pg_get_serial_sequence('public.%s', '%s'), COALESCE((SELECT MAX(%s) FROM %s), 1), (SELECT COUNT(*) > 0 FROM %s))`,
			table,
			column,
			quotePostgresIdent(column),
			quotePostgresIdent(table),
			quotePostgresIdent(table),
		)
		if _, err := target.Exec(stmt); err != nil {
			return fmt.Errorf("reset sequence for %s.%s: %w", table, column, err)
		}
	}
	return rows.Err()
}

func normalizeValueForTarget(value any, targetColumn postgresColumn) (any, string, error) {
	if value == nil {
		return nil, "", nil
	}

	switch v := value.(type) {
	case []byte:
		return normalizeValueForTarget(string(v), targetColumn)
	case time.Time:
		return v.UTC(), "", nil
	}

	normalizedValue, err := func() (any, error) {
		switch strings.ToLower(targetColumn.DataType) {
		case "boolean":
			return toBool(value)
		case "smallint", "integer", "bigint":
			return toInt64(value)
		case "real", "double precision", "numeric":
			return toFloat64(value)
		case "timestamp without time zone", "timestamp with time zone":
			return toTimeValue(value)
		default:
			return value, nil
		}
	}()
	if err == nil {
		return normalizedValue, "", nil
	}

	if targetColumn.Nullable {
		return nil, fmt.Sprintf("coerced invalid %s value %q to NULL (%v)", targetColumn.DataType, previewValue(value), err), nil
	}

	return nil, "", err
}

func toBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case int64:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case int:
		return v != 0, nil
	case float64:
		return v != 0, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "t", "true", "yes", "y", "on":
			return true, nil
		case "0", "f", "false", "no", "n", "off", "":
			return false, nil
		default:
			return false, fmt.Errorf("unsupported boolean string %q", v)
		}
	default:
		return false, fmt.Errorf("unsupported boolean type %T", value)
	}
}

func toInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, err
		}
		return n, nil
	default:
		return 0, fmt.Errorf("unsupported integer type %T", value)
	}
}

func toFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, err
		}
		return f, nil
	default:
		return 0, fmt.Errorf("unsupported float type %T", value)
	}
}

func toTimeValue(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v.UTC(), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return time.Time{}, errors.New("empty timestamp string")
		}
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999999Z07:00",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02 15:04:05",
			"2006-01-02",
		} {
			if ts, err := time.Parse(layout, trimmed); err == nil {
				return ts.UTC(), nil
			}
			if ts, err := time.ParseInLocation(layout, trimmed, time.UTC); err == nil {
				return ts.UTC(), nil
			}
		}
		return time.Time{}, fmt.Errorf("unsupported timestamp string %q", trimmed)
	case int64:
		if v > 1_000_000_000_000 {
			return time.UnixMilli(v).UTC(), nil
		}
		return time.Unix(v, 0).UTC(), nil
	case float64:
		return toTimeValue(int64(v))
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp type %T", value)
	}
}

func quoteSQLiteIdent(identifier string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(identifier, `"`, `""`))
}

func quotePostgresIdent(identifier string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(identifier, `"`, `""`))
}

func joinQuoted(values []string, quote func(string) string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = quote(value)
	}
	return strings.Join(quoted, ", ")
}

func makeDollarPlaceholders(count int) string {
	placeholders := make([]string, count)
	for i := 1; i <= count; i++ {
		placeholders[i-1] = fmt.Sprintf("$%d", i)
	}
	return strings.Join(placeholders, ", ")
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

func previewValue(value any) string {
	const limit = 120
	text := fmt.Sprintf("%v", value)
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}
