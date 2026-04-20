package trader

import (
	"errors"
	"testing"
	"time"

	"nofx/kernel"
	tradertypes "nofx/trader/types"
)

type mockExecutionQualityTrader struct {
	priceCalls map[string]int
	prices     map[string]float64
	priceErrs  map[string]error
}

func (m *mockExecutionQualityTrader) recordPriceCall(symbol string) {
	if m.priceCalls == nil {
		m.priceCalls = make(map[string]int)
	}
	m.priceCalls[symbol]++
}

func (m *mockExecutionQualityTrader) GetBalance() (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *mockExecutionQualityTrader) GetPositions() ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *mockExecutionQualityTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *mockExecutionQualityTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *mockExecutionQualityTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *mockExecutionQualityTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *mockExecutionQualityTrader) SetLeverage(symbol string, leverage int) error {
	return nil
}

func (m *mockExecutionQualityTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

func (m *mockExecutionQualityTrader) GetMarketPrice(symbol string) (float64, error) {
	m.recordPriceCall(symbol)
	if err, ok := m.priceErrs[symbol]; ok {
		return 0, err
	}
	if price, ok := m.prices[symbol]; ok {
		return price, nil
	}
	return 0, nil
}

func (m *mockExecutionQualityTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}

func (m *mockExecutionQualityTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}

func (m *mockExecutionQualityTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

func (m *mockExecutionQualityTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

func (m *mockExecutionQualityTrader) CancelAllOrders(symbol string) error {
	return nil
}

func (m *mockExecutionQualityTrader) CancelStopOrders(symbol string) error {
	return nil
}

func (m *mockExecutionQualityTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "", nil
}

func (m *mockExecutionQualityTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *mockExecutionQualityTrader) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) {
	return nil, nil
}

func (m *mockExecutionQualityTrader) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) {
	return nil, nil
}

func TestCollectExecutionSignalsFiltersAndCachesUnsupportedCandidates(t *testing.T) {
	tr := &mockExecutionQualityTrader{
		prices: map[string]float64{
			"BTCUSDT": 68000,
		},
		priceErrs: map[string]error{
			"KPEPEUSDT": errors.New("API error: params error: symbol invalid"),
		},
	}

	at := &AutoTrader{
		name:                        "test-trader",
		trader:                      tr,
		unsupportedCandidateSymbols: make(map[string]int64),
	}

	candidates := []kernel.CandidateCoin{
		{Symbol: "KPEPEUSDT", Sources: []string{"ai500"}},
		{Symbol: "BTCUSDT", Sources: []string{"ai500"}},
	}

	executionOut, venueOut, filtered := at.collectExecutionSignals(nil, candidates)
	if executionOut != nil {
		t.Fatalf("executionOut = %#v, want nil when order book fetcher is unavailable", executionOut)
	}
	if venueOut == nil {
		t.Fatal("venueOut = nil, want venue tradability data")
	}
	if venueOut["KPEPEUSDT"] == nil || venueOut["KPEPEUSDT"].OrderBookState != "venue_unsupported" {
		t.Fatalf("KPEPEUSDT venue state = %#v, want venue_unsupported", venueOut["KPEPEUSDT"])
	}
	if len(filtered) != 1 || filtered[0].Symbol != "BTCUSDT" {
		t.Fatalf("filtered candidates = %#v, want only BTCUSDT", filtered)
	}
	if !at.isUnsupportedCandidateSymbolCached("KPEPEUSDT") {
		t.Fatal("expected KPEPEUSDT to be cached as unsupported")
	}
	firstCalls := tr.priceCalls["KPEPEUSDT"]
	if firstCalls != 1 {
		t.Fatalf("KPEPEUSDT price calls = %d, want 1", firstCalls)
	}

	_, _, filteredAgain := at.collectExecutionSignals(nil, candidates)
	if len(filteredAgain) != 1 || filteredAgain[0].Symbol != "BTCUSDT" {
		t.Fatalf("filteredAgain = %#v, want only BTCUSDT", filteredAgain)
	}
	if tr.priceCalls["KPEPEUSDT"] != firstCalls {
		t.Fatalf("expected cached unsupported symbol to avoid new price calls, got %d -> %d", firstCalls, tr.priceCalls["KPEPEUSDT"])
	}
}
