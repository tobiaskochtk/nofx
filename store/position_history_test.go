package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func newPositionHistoryTestStore(t *testing.T, dbName string) *Store {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), dbName))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := configureSQLiteSQLDB(sqlDB); err != nil {
		t.Fatalf("configureSQLiteSQLDB() error = %v", err)
	}

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}
	return root
}

func TestCreateFromClosedPnLRefreshesExactClosedPositionInsteadOfDuplicating(t *testing.T) {
	root := newPositionHistoryTestStore(t, "closed-pnl-refresh.db")

	entryTime := time.Now().UTC().Add(-80 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-5 * time.Minute).UnixMilli()

	existing := &TraderPosition{
		TraderID:           "trader-refresh",
		ExchangeID:         "exchange-refresh",
		ExchangeType:       "bybit",
		ExchangePositionID: "sync-ton-long-1",
		Symbol:             "TONUSDT",
		Side:               "LONG",
		Quantity:           16,
		EntryQuantity:      16,
		EntryPrice:         1.4937,
		EntryTime:          entryTime,
		ExitPrice:          1.4652,
		ExitOrderID:        "exec-ton-close-1",
		ExitTime:           exitTime,
		RealizedPnL:        -0.48,
		Fee:                0.02,
		Status:             DealReviewCaseStatusClosed,
		CloseReason:        "sync",
		Source:             "sync",
		CreatedAt:          entryTime,
		UpdatedAt:          exitTime,
	}
	if err := root.gdb.Create(existing).Error; err != nil {
		t.Fatalf("Create(existing) error = %v", err)
	}

	created, err := root.Position().CreateFromClosedPnL("trader-refresh", "exchange-refresh", "bybit", &ClosedPnLRecord{
		Symbol:      "TONUSDT",
		Side:        "long",
		EntryPrice:  1.4937,
		ExitPrice:   1.4652,
		Quantity:    16,
		RealizedPnL: -0.4852,
		Fee:         0.0283,
		Leverage:    5,
		EntryTime:   exitTime,
		ExitTime:    exitTime,
		OrderID:     "bybit-close-order-1",
		ExchangeID:  "closed-pnl-ton-1",
		CloseType:   "take_profit",
	})
	if err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}
	if created {
		t.Fatal("expected exact match to refresh existing closed position instead of creating a duplicate")
	}

	var positions []TraderPosition
	if err := root.gdb.Where("trader_id = ?", "trader-refresh").Find(&positions).Error; err != nil {
		t.Fatalf("Find(positions) error = %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("len(positions) = %d, want 1", len(positions))
	}
	if positions[0].RealizedPnL != -0.4852 {
		t.Fatalf("RealizedPnL = %.4f, want -0.4852", positions[0].RealizedPnL)
	}
	if positions[0].Fee != 0.0283 {
		t.Fatalf("Fee = %.4f, want 0.0283", positions[0].Fee)
	}
	if positions[0].ExitOrderID != "bybit-close-order-1" {
		t.Fatalf("ExitOrderID = %q, want bybit-close-order-1", positions[0].ExitOrderID)
	}
	if positions[0].CloseReason != "take_profit" {
		t.Fatalf("CloseReason = %q, want take_profit", positions[0].CloseReason)
	}
}

func TestCreateFromClosedPnLSkipsContainedPartialCloseForExistingSyncPosition(t *testing.T) {
	root := newPositionHistoryTestStore(t, "closed-pnl-contained.db")

	entryTime := time.Now().UTC().Add(-25 * time.Minute).UnixMilli()
	partialExitTime := time.Now().UTC().Add(-6 * time.Minute).UnixMilli()
	finalExitTime := time.Now().UTC().Add(-1 * time.Minute).UnixMilli()

	existing := &TraderPosition{
		TraderID:           "trader-contained",
		ExchangeID:         "exchange-contained",
		ExchangeType:       "bybit",
		ExchangePositionID: "sync-ordi-long-1",
		Symbol:             "ORDIUSDT",
		Side:               "LONG",
		Quantity:           16.06,
		EntryQuantity:      16.06,
		EntryPrice:         4.945,
		EntryTime:          entryTime,
		ExitPrice:          4.99104,
		ExitOrderID:        "exec-ordi-close-final",
		ExitTime:           finalExitTime,
		RealizedPnL:        0.75,
		Fee:                0.0877,
		Status:             DealReviewCaseStatusClosed,
		CloseReason:        "sync",
		Source:             "sync",
		CreatedAt:          entryTime,
		UpdatedAt:          finalExitTime,
	}
	if err := root.gdb.Create(existing).Error; err != nil {
		t.Fatalf("Create(existing) error = %v", err)
	}

	created, err := root.Position().CreateFromClosedPnL("trader-contained", "exchange-contained", "bybit", &ClosedPnLRecord{
		Symbol:      "ORDIUSDT",
		Side:        "long",
		EntryPrice:  4.945,
		ExitPrice:   4.99081931,
		Quantity:    16.05,
		RealizedPnL: 0.64769152,
		Fee:         0.08770848,
		Leverage:    5,
		EntryTime:   partialExitTime,
		ExitTime:    partialExitTime,
		OrderID:     "bybit-close-order-partial",
		ExchangeID:  "closed-pnl-ordi-partial",
		CloseType:   "unknown",
	})
	if err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}
	if created {
		t.Fatal("expected contained partial close to be skipped instead of creating a duplicate")
	}

	var count int64
	if err := root.gdb.Model(&TraderPosition{}).Where("trader_id = ?", "trader-contained").Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	refreshed, err := root.Position().GetByID(existing.ID)
	if err != nil {
		t.Fatalf("GetByID(existing) error = %v", err)
	}
	if refreshed.RealizedPnL != 0.75 {
		t.Fatalf("RealizedPnL = %.2f, want 0.75", refreshed.RealizedPnL)
	}
	if refreshed.ExitTime != finalExitTime {
		t.Fatalf("ExitTime = %d, want %d", refreshed.ExitTime, finalExitTime)
	}
}

func TestCleanupRedundantClosedPnLPositionsRemovesDuplicateReviewCases(t *testing.T) {
	root := newPositionHistoryTestStore(t, "closed-pnl-cleanup.db")

	trader := &Trader{
		ID:             "trader-cleanup",
		UserID:         "user-cleanup",
		Name:           "Cleanup Trader",
		AIModelID:      "model-cleanup",
		ExchangeID:     "exchange-cleanup",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-45 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-2 * time.Minute).UnixMilli()

	syncPosition := &TraderPosition{
		TraderID:           trader.ID,
		ExchangeID:         trader.ExchangeID,
		ExchangeType:       "bybit",
		ExchangePositionID: "sync-bio-long-1",
		Symbol:             "BIOUSDT",
		Side:               "LONG",
		Quantity:           1834,
		EntryQuantity:      1834,
		EntryPrice:         0.04359609,
		EntryTime:          entryTime,
		ExitPrice:          0.04516,
		ExitOrderID:        "exec-bio-close-1",
		ExitTime:           exitTime,
		RealizedPnL:        2.86,
		Fee:                0.0895,
		Status:             DealReviewCaseStatusClosed,
		CloseReason:        "sync",
		Source:             "sync",
		CreatedAt:          entryTime,
		UpdatedAt:          exitTime,
	}
	if err := root.gdb.Create(syncPosition).Error; err != nil {
		t.Fatalf("Create(syncPosition) error = %v", err)
	}

	duplicate := &TraderPosition{
		TraderID:           trader.ID,
		ExchangeID:         trader.ExchangeID,
		ExchangeType:       "bybit",
		ExchangePositionID: "closed-pnl-bio-long-1",
		Symbol:             "BIOUSDT",
		Side:               "LONG",
		Quantity:           1834,
		EntryQuantity:      1834,
		EntryPrice:         0.0435961,
		EntryTime:          exitTime,
		ExitPrice:          0.04516,
		ExitOrderID:        "bybit-bio-close-1",
		ExitTime:           exitTime,
		RealizedPnL:        2.77868171,
		Fee:                0.08952829,
		Status:             DealReviewCaseStatusClosed,
		CloseReason:        "unknown",
		Source:             "closed_pnl_sync",
		CreatedAt:          exitTime,
		UpdatedAt:          exitTime,
	}
	if err := root.gdb.Create(duplicate).Error; err != nil {
		t.Fatalf("Create(duplicate) error = %v", err)
	}

	if err := root.DealReview().BackfillExistingPositions(); err != nil {
		t.Fatalf("DealReview().BackfillExistingPositions() error = %v", err)
	}

	var beforeCases int64
	if err := root.gdb.Model(&DealReviewCase{}).Where("trader_id = ?", trader.ID).Count(&beforeCases).Error; err != nil {
		t.Fatalf("Count(before cases) error = %v", err)
	}
	if beforeCases != 2 {
		t.Fatalf("beforeCases = %d, want 2", beforeCases)
	}

	removed, refreshed, err := root.Position().CleanupRedundantClosedPnLPositions()
	if err != nil {
		t.Fatalf("CleanupRedundantClosedPnLPositions() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if refreshed != 1 {
		t.Fatalf("refreshed = %d, want 1", refreshed)
	}

	var positionCount int64
	if err := root.gdb.Model(&TraderPosition{}).Where("trader_id = ?", trader.ID).Count(&positionCount).Error; err != nil {
		t.Fatalf("Count(positionCount) error = %v", err)
	}
	if positionCount != 1 {
		t.Fatalf("positionCount = %d, want 1", positionCount)
	}

	var caseCount int64
	if err := root.gdb.Model(&DealReviewCase{}).Where("trader_id = ?", trader.ID).Count(&caseCount).Error; err != nil {
		t.Fatalf("Count(caseCount) error = %v", err)
	}
	if caseCount != 1 {
		t.Fatalf("caseCount = %d, want 1", caseCount)
	}
}
