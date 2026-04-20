package store

import (
	"testing"
	"time"
)

func TestCreateOrderDuplicateReturnsExistingRow(t *testing.T) {
	root := newPositionHistoryTestStore(t, "order-duplicate.db")

	order := &TraderOrder{
		TraderID:        "trader-a",
		ExchangeID:      "exchange-a",
		ExchangeType:    "bybit",
		ExchangeOrderID: "exec-123",
		Symbol:          "ETHUSDT",
		Side:            "BUY",
		Type:            "Market",
		Quantity:        0.1,
		Status:          "FILLED",
		FilledQuantity:  0.1,
		AvgFillPrice:    2500,
		Commission:      0.1,
		CommissionAsset: "USDT",
		PositionSide:    "BOTH",
		TimeInForce:     "GTC",
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
		FilledAt:        time.Now().UTC().UnixMilli(),
	}
	if err := root.Order().CreateOrder(order); err != nil {
		t.Fatalf("CreateOrder(first) error = %v", err)
	}

	duplicate := &TraderOrder{
		TraderID:        "trader-b",
		ExchangeID:      "exchange-a",
		ExchangeType:    "bybit",
		ExchangeOrderID: "exec-123",
		Symbol:          "ETHUSDT",
		Side:            "BUY",
		Type:            "Market",
		Quantity:        0.1,
		Status:          "FILLED",
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
	}
	if err := root.Order().CreateOrder(duplicate); err != nil {
		t.Fatalf("CreateOrder(duplicate) error = %v", err)
	}
	if duplicate.ID != order.ID {
		t.Fatalf("duplicate.ID = %d, want %d", duplicate.ID, order.ID)
	}
}

func TestCreateOrderAllowsSameExchangeOrderIDAcrossDifferentExchanges(t *testing.T) {
	root := newPositionHistoryTestStore(t, "order-composite-unique.db")

	first := &TraderOrder{
		TraderID:        "trader-a",
		ExchangeID:      "exchange-a",
		ExchangeType:    "bybit",
		ExchangeOrderID: "shared-order-id",
		Symbol:          "ETHUSDT",
		Side:            "BUY",
		Type:            "Market",
		Quantity:        0.1,
		Status:          "FILLED",
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
	}
	second := &TraderOrder{
		TraderID:        "trader-b",
		ExchangeID:      "exchange-b",
		ExchangeType:    "binance",
		ExchangeOrderID: "shared-order-id",
		Symbol:          "ETHUSDT",
		Side:            "BUY",
		Type:            "Market",
		Quantity:        0.1,
		Status:          "FILLED",
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
	}

	if err := root.Order().CreateOrder(first); err != nil {
		t.Fatalf("CreateOrder(first) error = %v", err)
	}
	if err := root.Order().CreateOrder(second); err != nil {
		t.Fatalf("CreateOrder(second) error = %v", err)
	}
	if first.ID == 0 || second.ID == 0 {
		t.Fatal("expected both orders to be persisted")
	}
	if first.ID == second.ID {
		t.Fatalf("expected distinct rows, got same id %d", first.ID)
	}
}

func TestCreateFillDuplicateReturnsExistingRow(t *testing.T) {
	root := newPositionHistoryTestStore(t, "fill-duplicate.db")

	fill := &TraderFill{
		TraderID:        "trader-a",
		ExchangeID:      "exchange-a",
		ExchangeType:    "bybit",
		OrderID:         1,
		ExchangeOrderID: "order-123",
		ExchangeTradeID: "exec-123",
		Symbol:          "ETHUSDT",
		Side:            "BUY",
		Price:           2500,
		Quantity:        0.1,
		QuoteQuantity:   250,
		Commission:      0.1,
		CommissionAsset: "USDT",
		CreatedAt:       time.Now().UTC().UnixMilli(),
	}
	if err := root.Order().CreateFill(fill); err != nil {
		t.Fatalf("CreateFill(first) error = %v", err)
	}

	duplicate := &TraderFill{
		TraderID:        "trader-b",
		ExchangeID:      "exchange-a",
		ExchangeType:    "bybit",
		OrderID:         2,
		ExchangeOrderID: "order-456",
		ExchangeTradeID: "exec-123",
		Symbol:          "ETHUSDT",
		Side:            "BUY",
		Price:           2500,
		Quantity:        0.1,
		QuoteQuantity:   250,
		Commission:      0.1,
		CommissionAsset: "USDT",
		CreatedAt:       time.Now().UTC().UnixMilli(),
	}
	if err := root.Order().CreateFill(duplicate); err != nil {
		t.Fatalf("CreateFill(duplicate) error = %v", err)
	}
	if duplicate.ID != fill.ID {
		t.Fatalf("duplicate.ID = %d, want %d", duplicate.ID, fill.ID)
	}
}

func TestCreateFillAllowsSameExchangeTradeIDAcrossDifferentExchanges(t *testing.T) {
	root := newPositionHistoryTestStore(t, "fill-composite-unique.db")

	first := &TraderFill{
		TraderID:        "trader-a",
		ExchangeID:      "exchange-a",
		ExchangeType:    "bybit",
		OrderID:         1,
		ExchangeOrderID: "order-a",
		ExchangeTradeID: "shared-trade-id",
		Symbol:          "ETHUSDT",
		Side:            "BUY",
		Price:           2500,
		Quantity:        0.1,
		QuoteQuantity:   250,
		Commission:      0.1,
		CommissionAsset: "USDT",
		CreatedAt:       time.Now().UTC().UnixMilli(),
	}
	second := &TraderFill{
		TraderID:        "trader-b",
		ExchangeID:      "exchange-b",
		ExchangeType:    "binance",
		OrderID:         2,
		ExchangeOrderID: "order-b",
		ExchangeTradeID: "shared-trade-id",
		Symbol:          "ETHUSDT",
		Side:            "BUY",
		Price:           2501,
		Quantity:        0.2,
		QuoteQuantity:   500.2,
		Commission:      0.2,
		CommissionAsset: "USDT",
		CreatedAt:       time.Now().UTC().UnixMilli(),
	}

	if err := root.Order().CreateFill(first); err != nil {
		t.Fatalf("CreateFill(first) error = %v", err)
	}
	if err := root.Order().CreateFill(second); err != nil {
		t.Fatalf("CreateFill(second) error = %v", err)
	}
	if first.ID == 0 || second.ID == 0 {
		t.Fatal("expected both fills to be persisted")
	}
	if first.ID == second.ID {
		t.Fatalf("expected distinct rows, got same id %d", first.ID)
	}
}

func TestIsUniqueConstraintError(t *testing.T) {
	if !isUniqueConstraintError(assertErr("UNIQUE constraint failed: trader_orders.exchange_order_id")) {
		t.Fatal("expected sqlite unique constraint to be detected")
	}
	if !isUniqueConstraintError(assertErr("duplicate key value violates unique constraint \"idx_orders_exchange_unique\"")) {
		t.Fatal("expected postgres unique constraint to be detected")
	}
	if isUniqueConstraintError(assertErr("database is locked")) {
		t.Fatal("did not expect non-unique error to be detected as unique constraint")
	}
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}
