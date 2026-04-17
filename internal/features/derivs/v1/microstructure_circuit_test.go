package derivsv1

import (
	"fmt"
	"testing"
	"time"
)

func TestLiqFetchCircuitEngagesVenueCooldownAfterRepeatedTimeouts(t *testing.T) {
	circuit := newLiqFetchCircuit()
	now := time.Now()

	if !circuit.shouldAttemptVenue("bybit", now) {
		t.Fatal("bybit should be attemptable before any failures")
	}

	triggered, until := circuit.recordVenueFailure("bybit", fmt.Errorf("read tcp: i/o timeout"), now)
	if triggered {
		t.Fatal("cooldown triggered too early on first timeout")
	}
	if !until.IsZero() {
		t.Fatalf("unexpected cooldown deadline on first timeout: %v", until)
	}

	triggered, until = circuit.recordVenueFailure("bybit", fmt.Errorf("context deadline exceeded"), now.Add(time.Second))
	if !triggered {
		t.Fatal("cooldown did not trigger on repeated timeout")
	}
	if !until.After(now) {
		t.Fatalf("cooldown deadline %v is not in the future", until)
	}
	if circuit.shouldAttemptVenue("bybit", now.Add(2*time.Second)) {
		t.Fatal("bybit should be suppressed during cooldown")
	}

	circuit.recordVenueSuccess("bybit")
	if !circuit.shouldAttemptVenue("bybit", now.Add(2*time.Second)) {
		t.Fatal("bybit should be attemptable again after success reset")
	}
}

func TestLiqFetchCircuitMarksAndClearsSymbolCooldown(t *testing.T) {
	circuit := newLiqFetchCircuit()
	now := time.Now()

	if circuit.shouldSkipSymbol("BTCUSDT", now) {
		t.Fatal("symbol should not be cooled down initially")
	}

	circuit.markSymbolCooldown("BTCUSDT", now)
	if !circuit.shouldSkipSymbol("BTCUSDT", now.Add(time.Second)) {
		t.Fatal("symbol cooldown not active after mark")
	}
	if circuit.shouldSkipSymbol("BTCUSDT", now.Add(liqSymbolCooldown+time.Second)) {
		t.Fatal("symbol cooldown still active after expiry")
	}

	circuit.markSymbolCooldown("BTCUSDT", now)
	circuit.clearSymbolCooldown("BTCUSDT")
	if circuit.shouldSkipSymbol("BTCUSDT", now.Add(time.Second)) {
		t.Fatal("symbol cooldown should be cleared explicitly")
	}
}
