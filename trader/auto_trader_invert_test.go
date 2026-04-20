package trader

import (
	"nofx/kernel"
	"testing"
)

func TestInvertDecisionOpenLongMirrorsTargetsAroundCurrentPrice(t *testing.T) {
	decision := kernel.Decision{
		Symbol:     "SOLUSDT",
		Action:     "open_long",
		StopLoss:   95,
		TakeProfit: 110,
		Reasoning:  "trend continuation",
	}

	inverted, mode := invertDecision(decision, 100)

	if inverted.Action != "open_short" {
		t.Fatalf("expected open_short, got %s", inverted.Action)
	}
	if mode != "mirror_price" {
		t.Fatalf("expected mirror_price mode, got %s", mode)
	}
	if inverted.StopLoss != 105 {
		t.Fatalf("expected mirrored stop loss 105, got %.2f", inverted.StopLoss)
	}
	if inverted.TakeProfit != 90 {
		t.Fatalf("expected mirrored take profit 90, got %.2f", inverted.TakeProfit)
	}
}

func TestInvertDecisionOpenShortMirrorsTargetsAroundCurrentPrice(t *testing.T) {
	decision := kernel.Decision{
		Symbol:     "ETHUSDT",
		Action:     "open_short",
		StopLoss:   2500,
		TakeProfit: 2300,
		Reasoning:  "breakdown setup",
	}

	inverted, mode := invertDecision(decision, 2400)

	if inverted.Action != "open_long" {
		t.Fatalf("expected open_long, got %s", inverted.Action)
	}
	if mode != "mirror_price" {
		t.Fatalf("expected mirror_price mode, got %s", mode)
	}
	if inverted.StopLoss != 2300 {
		t.Fatalf("expected mirrored stop loss 2300, got %.2f", inverted.StopLoss)
	}
	if inverted.TakeProfit != 2500 {
		t.Fatalf("expected mirrored take profit 2500, got %.2f", inverted.TakeProfit)
	}
}

func TestInvertDecisionFallsBackToSwapWithoutPrice(t *testing.T) {
	decision := kernel.Decision{
		Symbol:     "BTCUSDT",
		Action:     "open_long",
		StopLoss:   94000,
		TakeProfit: 100000,
	}

	inverted, mode := invertDecision(decision, 0)

	if inverted.Action != "open_short" {
		t.Fatalf("expected open_short, got %s", inverted.Action)
	}
	if mode != "swap_targets" {
		t.Fatalf("expected swap_targets mode, got %s", mode)
	}
	if inverted.StopLoss != 100000 || inverted.TakeProfit != 94000 {
		t.Fatalf("unexpected swapped targets: stop=%.2f take=%.2f", inverted.StopLoss, inverted.TakeProfit)
	}
}

func TestInvertDecisionCloseActionsSwapSides(t *testing.T) {
	tests := []struct {
		action string
		want   string
	}{
		{action: "close_long", want: "close_short"},
		{action: "close_short", want: "close_long"},
	}

	for _, tt := range tests {
		inverted, _ := invertDecision(kernel.Decision{Action: tt.action}, 0)
		if inverted.Action != tt.want {
			t.Fatalf("action %s: expected %s, got %s", tt.action, tt.want, inverted.Action)
		}
	}
}
