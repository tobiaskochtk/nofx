package trader

import (
	"math"
	tradertypes "nofx/trader/types"
	"testing"
)

func TestInferProtectionTargetsFromOpenOrdersClassifiesGenericShortStops(t *testing.T) {
	orders := []tradertypes.OpenOrder{
		{OrderID: "stop", Side: "Buy", Type: "Stop", StopPrice: 0.58175, Quantity: 85},
		{OrderID: "take", Side: "Buy", Type: "Stop", StopPrice: 0.50876, Quantity: 85},
	}

	inferred := inferProtectionTargetsFromOpenOrders("short", 0.58464541, orders)
	if !inferred.HasStopLoss {
		t.Fatal("expected stop-loss candidate to be inferred")
	}
	if inferred.StopLoss != 0.58175 {
		t.Fatalf("stop-loss candidate = %.8f, want 0.58175000", inferred.StopLoss)
	}
	if inferred.StopLossValid {
		t.Fatalf("expected inferred stop-loss to be marked invalid below the actual short entry")
	}
	if !inferred.HasTakeProfit {
		t.Fatal("expected take-profit candidate to be inferred")
	}
	if inferred.TakeProfit != 0.50876 {
		t.Fatalf("take-profit candidate = %.8f, want 0.50876000", inferred.TakeProfit)
	}
	if !inferred.TakeProfitValid {
		t.Fatal("expected inferred take-profit to stay valid below the short entry")
	}
}

func TestNormalizeProtectionTargetsRepairsInvalidShortExchangeOrders(t *testing.T) {
	stopLoss, takeProfit, adjusted := normalizeProtectionTargets("SHORT", 0.58464541, 0.58464541, 0.58175, 0.50876)
	if !adjusted {
		t.Fatal("expected invalid short stop-loss to be repaired around the exchange entry")
	}
	if !nearlyEqualProtectionTarget(stopLoss, 0.58754082) {
		t.Fatalf("repaired stop-loss = %.8f, want 0.58754082", stopLoss)
	}
	if !nearlyEqualProtectionTarget(takeProfit, 0.50876) {
		t.Fatalf("take-profit = %.8f, want unchanged 0.50876000", takeProfit)
	}
	if stopLoss <= 0.58464541 {
		t.Fatalf("repaired short stop-loss must be above entry, got %.8f", stopLoss)
	}
}

func TestInferProtectionTargetsFromOpenOrdersUsesExplicitTypesFirst(t *testing.T) {
	orders := []tradertypes.OpenOrder{
		{OrderID: "generic-stop", Side: "Sell", Type: "Stop", StopPrice: 99},
		{OrderID: "explicit-stop", Side: "Sell", Type: "StopLoss", StopPrice: 95},
		{OrderID: "explicit-take", Side: "Sell", Type: "TakeProfit", StopPrice: 112},
	}

	inferred := inferProtectionTargetsFromOpenOrders("long", 100, orders)
	if !inferred.HasStopLoss || inferred.StopLoss != 95 {
		t.Fatalf("explicit stop-loss should win, got %.8f", inferred.StopLoss)
	}
	if !inferred.HasTakeProfit || inferred.TakeProfit != 112 {
		t.Fatalf("explicit take-profit should win, got %.8f", inferred.TakeProfit)
	}
	if !inferred.StopLossValid || !inferred.TakeProfitValid {
		t.Fatalf("expected explicit targets to be marked valid: %#v", inferred)
	}
}

func nearlyEqualProtectionTarget(left, right float64) bool {
	return math.Abs(left-right) <= 0.0000001
}
