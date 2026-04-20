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

func TestLiqFetchCircuitDoesNotRetriggerDuringActiveVenueCooldown(t *testing.T) {
	circuit := newLiqFetchCircuit()
	now := time.Now()

	triggered, until := circuit.recordVenueFailure("bybit", fmt.Errorf("read tcp: i/o timeout"), now)
	if triggered {
		t.Fatal("cooldown triggered too early on first timeout")
	}

	triggered, until = circuit.recordVenueFailure("bybit", fmt.Errorf("context deadline exceeded"), now.Add(time.Second))
	if !triggered {
		t.Fatal("cooldown should trigger on second timeout")
	}

	triggeredAgain, sameUntil := circuit.recordVenueFailure("bybit", fmt.Errorf("read tcp: i/o timeout"), now.Add(2*time.Second))
	if triggeredAgain {
		t.Fatal("cooldown should not retrigger while already active")
	}
	if !sameUntil.Equal(until) {
		t.Fatalf("recordVenueFailure() returned cooldown deadline %v, want existing %v", sameUntil, until)
	}
}

func TestMicroLogThrottleSuppressesRepeatedKeys(t *testing.T) {
	throttle := newMicroLogThrottle()
	now := time.Now()

	if !throttle.allow("liq:BTCUSDT", now, time.Minute) {
		t.Fatal("first log should be allowed")
	}
	if throttle.allow("liq:BTCUSDT", now.Add(10*time.Second), time.Minute) {
		t.Fatal("repeated log inside throttle window should be suppressed")
	}
	if !throttle.allow("liq:BTCUSDT", now.Add(time.Minute+time.Second), time.Minute) {
		t.Fatal("log should be allowed again after throttle window")
	}
}

func TestLiqEventStoreRecentKeepsNewestAndPrunesExpired(t *testing.T) {
	store := newLiqEventStore()
	now := time.Now()

	store.add(
		liqEvent{symbol: "BTCUSDT", ts: now.Add(-liqEventRetention - time.Minute).UnixMilli(), price: 1, size: 1},
		liqEvent{symbol: "BTCUSDT", ts: now.Add(-2 * time.Minute).UnixMilli(), price: 2, size: 2},
		liqEvent{symbol: "BTCUSDT", ts: now.Add(-1 * time.Minute).UnixMilli(), price: 3, size: 3},
	)

	events := store.recent("BTCUSDT", now.Add(-10*time.Minute), 10)
	if len(events) != 2 {
		t.Fatalf("expected 2 recent events after pruning, got %d", len(events))
	}
	if events[0].price != 2 || events[1].price != 3 {
		t.Fatalf("unexpected events order/content: %#v", events)
	}
}

func TestLiqEventStoreAddSkipsDuplicates(t *testing.T) {
	store := newLiqEventStore()
	now := time.Now()
	event := liqEvent{symbol: "BTCUSDT", ts: now.UnixMilli(), side: "SELL", price: 65000, size: 3250}

	store.add(event, event)
	events := store.recent("BTCUSDT", now.Add(-time.Minute), 10)
	if len(events) != 1 {
		t.Fatalf("expected 1 cached event after duplicate add, got %d", len(events))
	}
}

func TestParseBinanceForceOrderMessagePopulatesSymbol(t *testing.T) {
	raw := []byte(`{
		"e":"forceOrder",
		"E":1568014460893,
		"o":{
			"s":"BTCUSDT",
			"S":"SELL",
			"p":"9910",
			"ap":"9910",
			"q":"0.014",
			"l":"0.014",
			"z":"0.014",
			"T":1568014460893
		}
	}`)

	event := parseBinanceForceOrderMessage(raw, "BTCUSDT")
	if event == nil {
		t.Fatal("expected force order event")
	}
	if event.symbol != "BTCUSDT" {
		t.Fatalf("expected symbol BTCUSDT, got %q", event.symbol)
	}
	if event.price <= 0 || event.size <= 0 {
		t.Fatalf("expected positive price/size, got price=%f size=%f", event.price, event.size)
	}
}
