package trader

import (
	"database/sql"
	"math"
	"nofx/store"
	"path/filepath"
	"sync"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

type mockTrailingTrader struct {
	positions  []map[string]interface{}
	openOrders []OpenOrder

	cancelStopLossCalls int
	stopLossCalls       int
	takeProfitCalls     int

	lastStopLossSymbol string
	lastStopLossSide   string
	lastStopLossPrice  float64

	lastTakeProfitSymbol string
	lastTakeProfitSide   string
	lastTakeProfitPrice  float64
}

func (m *mockTrailingTrader) GetBalance() (map[string]interface{}, error) {
	return map[string]interface{}{"availableBalance": 1000.0}, nil
}

func (m *mockTrailingTrader) GetPositions() ([]map[string]interface{}, error) {
	return m.positions, nil
}

func (m *mockTrailingTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{"orderId": int64(1)}, nil
}

func (m *mockTrailingTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{"orderId": int64(1)}, nil
}

func (m *mockTrailingTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return map[string]interface{}{"orderId": int64(1)}, nil
}

func (m *mockTrailingTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return map[string]interface{}{"orderId": int64(1)}, nil
}

func (m *mockTrailingTrader) SetLeverage(symbol string, leverage int) error         { return nil }
func (m *mockTrailingTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (m *mockTrailingTrader) GetMarketPrice(symbol string) (float64, error)         { return 100, nil }

func (m *mockTrailingTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	m.stopLossCalls++
	m.lastStopLossSymbol = symbol
	m.lastStopLossSide = positionSide
	m.lastStopLossPrice = stopPrice
	return nil
}

func (m *mockTrailingTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	m.takeProfitCalls++
	m.lastTakeProfitSymbol = symbol
	m.lastTakeProfitSide = positionSide
	m.lastTakeProfitPrice = takeProfitPrice
	return nil
}

func (m *mockTrailingTrader) CancelStopLossOrders(symbol string) error {
	m.cancelStopLossCalls++
	return nil
}

func (m *mockTrailingTrader) CancelTakeProfitOrders(symbol string) error { return nil }
func (m *mockTrailingTrader) CancelAllOrders(symbol string) error        { return nil }
func (m *mockTrailingTrader) CancelStopOrders(symbol string) error       { return nil }
func (m *mockTrailingTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "", nil
}
func (m *mockTrailingTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "NEW"}, nil
}

func (m *mockTrailingTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	return nil, nil
}

func (m *mockTrailingTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	return m.openOrders, nil
}

func TestUpdateTrailingStopsShortMovesStopLossUsingStrategyTiers(t *testing.T) {
	strategyConfig := store.GetDefaultStrategyConfig("en")
	strategyConfig.RiskControl.TrailingStop.Enabled = true
	strategyConfig.RiskControl.TrailingStop.UpdateThresholdPct = 0.05
	strategyConfig.RiskControl.TrailingStop.Tiers = []store.TrailingStopTier{
		{
			TriggerProfitPct: 1.0,
			Mode:             store.TrailingStopModeLockProfit,
			LockProfitPct:    0.4,
		},
	}

	mockTrader := &mockTrailingTrader{
		positions: []map[string]interface{}{
			{
				"symbol":      "SIGNUSDT",
				"side":        "short",
				"entryPrice":  100.0,
				"markPrice":   98.0,
				"positionAmt": -1.0,
				"leverage":    5.0,
			},
		},
	}

	at := &AutoTrader{
		exchange:            "binance",
		config:              AutoTraderConfig{StrategyConfig: &strategyConfig},
		trader:              mockTrader,
		trailingStopState:   make(map[string]*trailingStopPositionState),
		trailingStopStateMu: sync.RWMutex{},
	}
	at.seedTrailingStopState("SIGNUSDT", "short", 102.0, 95.0)

	at.updateTrailingStops()

	if mockTrader.cancelStopLossCalls != 1 {
		t.Fatalf("CancelStopLossOrders calls = %d, want 1", mockTrader.cancelStopLossCalls)
	}
	if mockTrader.stopLossCalls != 1 {
		t.Fatalf("SetStopLoss calls = %d, want 1", mockTrader.stopLossCalls)
	}
	if mockTrader.takeProfitCalls != 0 {
		t.Fatalf("SetTakeProfit calls = %d, want 0 on binance-style exchanges", mockTrader.takeProfitCalls)
	}

	wantStop := 99.92
	if math.Abs(mockTrader.lastStopLossPrice-wantStop) > 0.000001 {
		t.Fatalf("new stop loss = %.8f, want %.8f", mockTrader.lastStopLossPrice, wantStop)
	}
}

func TestUpdateTrailingStopsRestoresTakeProfitOnCoupledExchange(t *testing.T) {
	strategyConfig := store.GetDefaultStrategyConfig("en")
	strategyConfig.RiskControl.TrailingStop.Enabled = true
	strategyConfig.RiskControl.TrailingStop.UpdateThresholdPct = 0.05
	strategyConfig.RiskControl.TrailingStop.Tiers = []store.TrailingStopTier{
		{
			TriggerProfitPct: 1.0,
			Mode:             store.TrailingStopModeLockProfit,
			LockProfitPct:    0.5,
		},
	}

	mockTrader := &mockTrailingTrader{
		positions: []map[string]interface{}{
			{
				"symbol":      "BTCUSDT",
				"side":        "long",
				"entryPrice":  100.0,
				"markPrice":   102.0,
				"positionAmt": 1.0,
				"leverage":    5.0,
			},
		},
	}

	at := &AutoTrader{
		exchange:            "hyperliquid",
		config:              AutoTraderConfig{StrategyConfig: &strategyConfig},
		trader:              mockTrader,
		trailingStopState:   make(map[string]*trailingStopPositionState),
		trailingStopStateMu: sync.RWMutex{},
	}
	at.seedTrailingStopState("BTCUSDT", "long", 95.0, 110.0)

	at.updateTrailingStops()

	if mockTrader.stopLossCalls != 1 {
		t.Fatalf("SetStopLoss calls = %d, want 1", mockTrader.stopLossCalls)
	}
	if mockTrader.takeProfitCalls != 1 {
		t.Fatalf("SetTakeProfit calls = %d, want 1 for coupled exchanges", mockTrader.takeProfitCalls)
	}
	if mockTrader.lastTakeProfitPrice != 110.0 {
		t.Fatalf("restored take profit = %.2f, want 110.00", mockTrader.lastTakeProfitPrice)
	}
}

func TestUpdateTrailingStopsSkipsCoupledExchangeWithoutKnownTakeProfit(t *testing.T) {
	strategyConfig := store.GetDefaultStrategyConfig("en")
	strategyConfig.RiskControl.TrailingStop.Enabled = true
	strategyConfig.RiskControl.TrailingStop.UpdateThresholdPct = 0.05
	strategyConfig.RiskControl.TrailingStop.Tiers = []store.TrailingStopTier{
		{
			TriggerProfitPct: 1.0,
			Mode:             store.TrailingStopModeLockProfit,
			LockProfitPct:    0.5,
		},
	}

	mockTrader := &mockTrailingTrader{
		positions: []map[string]interface{}{
			{
				"symbol":      "ETHUSDT",
				"side":        "long",
				"entryPrice":  100.0,
				"markPrice":   102.0,
				"positionAmt": 1.0,
				"leverage":    5.0,
			},
		},
	}

	at := &AutoTrader{
		exchange:            "hyperliquid",
		config:              AutoTraderConfig{StrategyConfig: &strategyConfig},
		trader:              mockTrader,
		trailingStopState:   make(map[string]*trailingStopPositionState),
		trailingStopStateMu: sync.RWMutex{},
	}

	at.updateTrailingStops()

	if mockTrader.cancelStopLossCalls != 0 {
		t.Fatalf("CancelStopLossOrders calls = %d, want 0 when TP is unknown", mockTrader.cancelStopLossCalls)
	}
	if mockTrader.stopLossCalls != 0 {
		t.Fatalf("SetStopLoss calls = %d, want 0 when TP is unknown", mockTrader.stopLossCalls)
	}
}

func TestUpdateTrailingStopsRespectsFirstTightenDelay(t *testing.T) {
	strategyConfig := store.GetDefaultStrategyConfig("en")
	strategyConfig.RiskControl.TrailingStop.Enabled = true
	strategyConfig.RiskControl.TrailingStop.UpdateThresholdPct = 0.05
	strategyConfig.RiskControl.TrailingStop.FirstTightenDelaySec = 300
	strategyConfig.RiskControl.TrailingStop.Tiers = []store.TrailingStopTier{
		{
			TriggerProfitPct: 1.0,
			Mode:             store.TrailingStopModeLockProfit,
			LockProfitPct:    0.5,
		},
	}

	mockTrader := &mockTrailingTrader{
		positions: []map[string]interface{}{
			{
				"symbol":      "ALPHAUSDT",
				"side":        "long",
				"entryPrice":  100.0,
				"markPrice":   102.0,
				"positionAmt": 1.0,
				"leverage":    5.0,
			},
		},
	}

	at := &AutoTrader{
		exchange:              "binance",
		config:                AutoTraderConfig{StrategyConfig: &strategyConfig},
		trader:                mockTrader,
		positionFirstSeenTime: map[string]int64{trailingStopPositionKey("ALPHAUSDT", "long"): time.Now().UTC().Add(-45 * time.Second).UnixMilli()},
		trailingStopState:     make(map[string]*trailingStopPositionState),
		trailingStopStateMu:   sync.RWMutex{},
	}
	at.seedTrailingStopState("ALPHAUSDT", "long", 96.0, 110.0)

	at.updateTrailingStops()

	if mockTrader.cancelStopLossCalls != 0 {
		t.Fatalf("CancelStopLossOrders calls = %d, want 0 while first-tighten delay is active", mockTrader.cancelStopLossCalls)
	}
	if mockTrader.stopLossCalls != 0 {
		t.Fatalf("SetStopLoss calls = %d, want 0 while first-tighten delay is active", mockTrader.stopLossCalls)
	}
}

func TestUpdateTrailingStopsRespectsMinFirstUpdateProfitPct(t *testing.T) {
	strategyConfig := store.GetDefaultStrategyConfig("en")
	strategyConfig.RiskControl.TrailingStop.Enabled = true
	strategyConfig.RiskControl.TrailingStop.UpdateThresholdPct = 0.05
	strategyConfig.RiskControl.TrailingStop.MinFirstUpdateProfitPct = 12
	strategyConfig.RiskControl.TrailingStop.Tiers = []store.TrailingStopTier{
		{
			TriggerProfitPct: 1.0,
			Mode:             store.TrailingStopModeLockProfit,
			LockProfitPct:    0.5,
		},
	}

	mockTrader := &mockTrailingTrader{
		positions: []map[string]interface{}{
			{
				"symbol":      "BETAUSDT",
				"side":        "long",
				"entryPrice":  100.0,
				"markPrice":   102.0,
				"positionAmt": 1.0,
				"leverage":    5.0,
			},
		},
	}

	at := &AutoTrader{
		exchange:            "binance",
		config:              AutoTraderConfig{StrategyConfig: &strategyConfig},
		trader:              mockTrader,
		trailingStopState:   make(map[string]*trailingStopPositionState),
		trailingStopStateMu: sync.RWMutex{},
	}
	at.seedTrailingStopState("BETAUSDT", "long", 96.0, 110.0)

	at.updateTrailingStops()

	if mockTrader.stopLossCalls != 0 {
		t.Fatalf("SetStopLoss calls = %d, want 0 while profit gate blocks the first tighten", mockTrader.stopLossCalls)
	}
}

func TestUpdateTrailingStopsIgnoresFirstUpdateGuardsAfterActivation(t *testing.T) {
	strategyConfig := store.GetDefaultStrategyConfig("en")
	strategyConfig.RiskControl.TrailingStop.Enabled = true
	strategyConfig.RiskControl.TrailingStop.UpdateThresholdPct = 0.05
	strategyConfig.RiskControl.TrailingStop.FirstTightenDelaySec = 600
	strategyConfig.RiskControl.TrailingStop.MinFirstUpdateProfitPct = 12
	strategyConfig.RiskControl.TrailingStop.Tiers = []store.TrailingStopTier{
		{
			TriggerProfitPct: 1.0,
			Mode:             store.TrailingStopModeLockProfit,
			LockProfitPct:    0.5,
		},
	}

	mockTrader := &mockTrailingTrader{
		positions: []map[string]interface{}{
			{
				"symbol":      "GAMMAUSDT",
				"side":        "long",
				"entryPrice":  100.0,
				"markPrice":   102.0,
				"positionAmt": 1.0,
				"leverage":    5.0,
			},
		},
	}

	at := &AutoTrader{
		exchange:            "binance",
		config:              AutoTraderConfig{StrategyConfig: &strategyConfig},
		trader:              mockTrader,
		trailingStopState:   make(map[string]*trailingStopPositionState),
		trailingStopStateMu: sync.RWMutex{},
	}
	at.saveTrailingStopPositionState("GAMMAUSDT", "long", &trailingStopPositionState{
		LastStopPrice:    96.0,
		HasLastStopPrice: true,
		TakeProfitPrice:  110.0,
		HasTakeProfit:    true,
		HighestTierIndex: 0,
		FirstSeenAtMs:    time.Now().UTC().UnixMilli(),
		HasActivated:     true,
	})

	at.updateTrailingStops()

	if mockTrader.stopLossCalls != 1 {
		t.Fatalf("SetStopLoss calls = %d, want 1 after activation even with first-update guards configured", mockTrader.stopLossCalls)
	}
}

func TestUpdateTrailingStopsPersistsDealReviewTrailingUpdate(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "trailing-deal-review.db"))
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

	st, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("store.NewFromGorm() error = %v", err)
	}
	if err := st.GormDB().AutoMigrate(
		&store.Trader{},
		&store.TraderPosition{},
		&store.DealReviewCase{},
		&store.DealReviewEvent{},
		&store.DealReviewTrailingUpdateRecord{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	traderRecord := &store.Trader{
		ID:             "trailing-review-trader",
		UserID:         "trailing-review-user",
		Name:           "Trailing Review Trader",
		AIModelID:      "model-trailing-review",
		ExchangeID:     "exchange-trailing-review",
		InitialBalance: 1000,
	}
	if err := st.Trader().Create(traderRecord); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-10 * time.Minute).UnixMilli()
	position := &store.TraderPosition{
		TraderID:      traderRecord.ID,
		ExchangeID:    traderRecord.ExchangeID,
		ExchangeType:  "binance",
		Symbol:        "SIGNUSDT",
		Side:          "SHORT",
		EntryQuantity: 1,
		Quantity:      1,
		EntryPrice:    100,
		EntryOrderID:  "entry-sign-review-1",
		EntryTime:     entryTime,
		Leverage:      5,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := st.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	strategyConfig := store.GetDefaultStrategyConfig("en")
	strategyConfig.RiskControl.TrailingStop.Enabled = true
	strategyConfig.RiskControl.TrailingStop.UpdateThresholdPct = 0.05
	strategyConfig.RiskControl.TrailingStop.Tiers = []store.TrailingStopTier{
		{
			TriggerProfitPct: 1.0,
			Mode:             store.TrailingStopModeLockProfit,
			LockProfitPct:    0.4,
		},
	}

	mockTrader := &mockTrailingTrader{
		positions: []map[string]interface{}{
			{
				"symbol":           "SIGNUSDT",
				"side":             "short",
				"entryPrice":       100.0,
				"markPrice":        98.0,
				"positionAmt":      -1.0,
				"leverage":         5.0,
				"unRealizedProfit": 2.0,
			},
		},
	}

	at := &AutoTrader{
		id:                  traderRecord.ID,
		exchange:            "binance",
		exchangeID:          traderRecord.ExchangeID,
		config:              AutoTraderConfig{StrategyConfig: &strategyConfig},
		trader:              mockTrader,
		store:               st,
		userID:              traderRecord.UserID,
		trailingStopState:   make(map[string]*trailingStopPositionState),
		trailingStopStateMu: sync.RWMutex{},
	}
	at.seedTrailingStopState("SIGNUSDT", "short", 102.0, 95.0)

	at.updateTrailingStops()

	var updates []store.DealReviewTrailingUpdateRecord
	if err := st.GormDB().
		Where("trader_id = ? AND symbol = ? AND side = ?", traderRecord.ID, "SIGNUSDT", "SHORT").
		Order("timestamp_ms ASC, id ASC").
		Find(&updates).Error; err != nil {
		t.Fatalf("query trailing updates error = %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("trailing updates len = %d, want 1", len(updates))
	}
	if updates[0].PositionID != position.ID {
		t.Fatalf("position_id = %d, want %d", updates[0].PositionID, position.ID)
	}
	if updates[0].DealID == "" {
		t.Fatal("expected deal_id to be linked on trailing update")
	}
	if updates[0].PreviousStopPrice != 102.0 {
		t.Fatalf("previous_stop_price = %.2f, want 102.00", updates[0].PreviousStopPrice)
	}
	if math.Abs(updates[0].NewStopPrice-99.92) > 0.000001 {
		t.Fatalf("new_stop_price = %.8f, want 99.92000000", updates[0].NewStopPrice)
	}
	if updates[0].TrailingMode != store.TrailingStopModeLockProfit {
		t.Fatalf("trailing_mode = %q, want %q", updates[0].TrailingMode, store.TrailingStopModeLockProfit)
	}
	if updates[0].UnrealizedPnL != 2.0 {
		t.Fatalf("unrealized_pnl = %.2f, want 2.00", updates[0].UnrealizedPnL)
	}
	if updates[0].UnrealizedPnLPct != 2.0 {
		t.Fatalf("unrealized_pnl_pct = %.2f, want 2.00", updates[0].UnrealizedPnLPct)
	}
	if updates[0].StopProfitPct != 0.4 {
		t.Fatalf("stop_profit_pct = %.2f, want 0.40", updates[0].StopProfitPct)
	}
	if !updates[0].ProtectsBreakeven {
		t.Fatal("expected trailing update to protect breakeven for the tightened short stop")
	}
}
