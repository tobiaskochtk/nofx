package trader

import (
	"math"
	"nofx/store"
	"sync"
	"testing"
	"time"
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
