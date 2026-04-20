package store

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestDerivsLiquidationStoreSaveBatchAndListRecent(t *testing.T) {
	gdb, err := openTestDerivsLiquidationDB(t, filepath.Join(t.TempDir(), "derivs-liquidation-test.db"))
	if err != nil {
		t.Fatalf("openTestDerivsLiquidationDB() error = %v", err)
	}

	liqStore := NewDerivsLiquidationStore(gdb)
	if err := liqStore.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	events := []DerivsLiquidationEvent{
		{
			Venue:            "binance",
			Symbol:           "BTCUSDT",
			EventTimestampMs: now.Add(-2 * time.Minute).UnixMilli(),
			Side:             "SELL",
			Price:            65000,
			Size:             3250,
			SourceTopic:      "!forceOrder@arr",
		},
		{
			Venue:            "bybit",
			Symbol:           "BTCUSDT",
			EventTimestampMs: now.Add(-1 * time.Minute).UnixMilli(),
			Side:             "BUY",
			Price:            65100,
			Size:             6510,
			SourceTopic:      "allLiquidation.BTCUSDT",
		},
		{
			Venue:            "binance",
			Symbol:           "BTCUSDT",
			EventTimestampMs: now.Add(-2 * time.Minute).UnixMilli(),
			Side:             "SELL",
			Price:            65000,
			Size:             3250,
			SourceTopic:      "!forceOrder@arr",
		},
	}
	if err := liqStore.SaveBatch(events); err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}

	recent, err := liqStore.ListRecent("BTCUSDT", now.Add(-10*time.Minute), 10)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("ListRecent() returned %d events, want 2 after dedupe", len(recent))
	}
	if recent[0].Venue != "binance" || recent[1].Venue != "bybit" {
		t.Fatalf("ListRecent() returned unexpected venues/order: %#v", recent)
	}
	if !recent[0].EventTime.Before(recent[1].EventTime) {
		t.Fatalf("ListRecent() should return ascending event time order, got %#v", recent)
	}
}

func TestDerivsLiquidationStorePruneOlderThan(t *testing.T) {
	gdb, err := openTestDerivsLiquidationDB(t, filepath.Join(t.TempDir(), "derivs-liquidation-prune.db"))
	if err != nil {
		t.Fatalf("openTestDerivsLiquidationDB() error = %v", err)
	}

	liqStore := NewDerivsLiquidationStore(gdb)
	if err := liqStore.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	if err := liqStore.SaveBatch([]DerivsLiquidationEvent{
		{
			Venue:            "binance",
			Symbol:           "ETHUSDT",
			EventTimestampMs: now.Add(-5 * time.Hour).UnixMilli(),
			Side:             "SELL",
			Price:            3000,
			Size:             6000,
		},
		{
			Venue:            "binance",
			Symbol:           "ETHUSDT",
			EventTimestampMs: now.Add(-30 * time.Minute).UnixMilli(),
			Side:             "BUY",
			Price:            3010,
			Size:             3010,
		},
	}); err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}

	rows, err := liqStore.PruneOlderThan(now.Add(-4 * time.Hour))
	if err != nil {
		t.Fatalf("PruneOlderThan() error = %v", err)
	}
	if rows != 1 {
		t.Fatalf("PruneOlderThan() removed %d rows, want 1", rows)
	}

	recent, err := liqStore.ListRecent("ETHUSDT", now.Add(-24*time.Hour), 10)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}
	if len(recent) != 1 || recent[0].Side != "BUY" {
		t.Fatalf("remaining events = %#v, want single recent BUY event", recent)
	}
}

func openTestDerivsLiquidationDB(t *testing.T, path string) (*gorm.DB, error) {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        path,
	}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db, nil
}
