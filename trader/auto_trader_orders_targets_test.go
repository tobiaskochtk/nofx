package trader

import (
	"math"
	"testing"
)

func TestNormalizeProtectionTargetsKeepsValidShortTargets(t *testing.T) {
	stopLoss, takeProfit, adjusted := normalizeProtectionTargets("SHORT", 100, 98, 106, 92)
	if adjusted {
		t.Fatalf("expected valid short targets to remain unchanged")
	}
	if stopLoss != 106 || takeProfit != 92 {
		t.Fatalf("unexpected normalized targets: stop=%.8f take=%.8f", stopLoss, takeProfit)
	}
}

func TestNormalizeProtectionTargetsSwapsReversedTargets(t *testing.T) {
	stopLoss, takeProfit, adjusted := normalizeProtectionTargets("SHORT", 100, 98, 92, 106)
	if !adjusted {
		t.Fatalf("expected reversed short targets to be swapped")
	}
	if stopLoss != 106 || takeProfit != 92 {
		t.Fatalf("unexpected swapped targets: stop=%.8f take=%.8f", stopLoss, takeProfit)
	}
}

func TestNormalizeProtectionTargetsRebasesShortTargetsAroundEntry(t *testing.T) {
	stopLoss, takeProfit, adjusted := normalizeProtectionTargets("SHORT", 0.69455, 0.55338, 0.58176, 0.50876)
	if !adjusted {
		t.Fatalf("expected short targets to be rebased around the actual entry")
	}
	if !nearlyEqualOrderTarget(stopLoss, 0.72293) {
		t.Fatalf("rebased stop loss = %.8f, want 0.72293000", stopLoss)
	}
	if !nearlyEqualOrderTarget(takeProfit, 0.50876) {
		t.Fatalf("expected already-valid take profit to remain unchanged, got %.8f", takeProfit)
	}
	if stopLoss <= 0.69455 {
		t.Fatalf("rebased short stop loss must stay above entry: %.8f", stopLoss)
	}
	if takeProfit >= 0.69455 {
		t.Fatalf("rebased short take profit must stay below entry: %.8f", takeProfit)
	}
}

func TestNormalizeProtectionTargetsRebasesInvalidShortTakeProfitAroundEntry(t *testing.T) {
	stopLoss, takeProfit, adjusted := normalizeProtectionTargets("SHORT", 0.69455, 0.55338, 0.72293, 0.801)
	if !adjusted {
		t.Fatalf("expected invalid short take profit to be rebased around the actual entry")
	}
	if !nearlyEqualOrderTarget(stopLoss, 0.72293) {
		t.Fatalf("expected already-valid short stop to remain unchanged, got %.8f", stopLoss)
	}
	if !nearlyEqualOrderTarget(takeProfit, 0.44693) {
		t.Fatalf("rebased take profit = %.8f, want 0.44693000", takeProfit)
	}
	if takeProfit >= 0.69455 {
		t.Fatalf("rebased short take profit must stay below entry: %.8f", takeProfit)
	}
}

func TestNormalizeProtectionTargetsRebasesLongTargetsAroundEntry(t *testing.T) {
	stopLoss, takeProfit, adjusted := normalizeProtectionTargets("LONG", 90, 100, 95, 110)
	if !adjusted {
		t.Fatalf("expected long stop loss to be rebased around the actual entry")
	}
	if !nearlyEqualOrderTarget(stopLoss, 85) {
		t.Fatalf("rebased stop loss = %.8f, want 85", stopLoss)
	}
	if takeProfit != 110 {
		t.Fatalf("expected already-valid take profit to remain unchanged, got %.8f", takeProfit)
	}
	if stopLoss >= 90 {
		t.Fatalf("rebased long stop loss must stay below entry: %.8f", stopLoss)
	}
}

func nearlyEqualOrderTarget(left, right float64) bool {
	return math.Abs(left-right) <= 0.0000001
}
