package derivsv1

import "testing"

func TestParseBybitLiqMessage_AllLiquidationV5(t *testing.T) {
	raw := []byte(`{
		"topic":"allLiquidation.BTCUSDT",
		"type":"snapshot",
		"ts":1771770000000,
		"data":[
			{"T":1771770001000,"s":"BTCUSDT","S":"Buy","v":"12.3","p":"67000.5"}
		]
	}`)

	events := parseBybitLiqMessage(raw, "BTCUSDT")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.symbol != "BTCUSDT" {
		t.Fatalf("expected symbol BTCUSDT, got %q", ev.symbol)
	}
	if ev.price <= 0 || ev.size <= 0 {
		t.Fatalf("expected positive price/size, got price=%f size=%f", ev.price, ev.size)
	}
	if ev.side == "" {
		t.Fatalf("expected side to be populated")
	}
	if ev.ts != 1771770001000 {
		t.Fatalf("expected ts from T field, got %d", ev.ts)
	}
}

func TestParseBybitLiqMessage_SymbolFilter(t *testing.T) {
	raw := []byte(`{
		"topic":"allLiquidation.ETHUSDT",
		"type":"snapshot",
		"ts":1771770000000,
		"data":[
			{"T":1771770001000,"s":"ETHUSDT","S":"Sell","v":"2","p":"3500"}
		]
	}`)

	events := parseBybitLiqMessage(raw, "BTCUSDT")
	if len(events) != 0 {
		t.Fatalf("expected 0 events after symbol filter, got %d", len(events))
	}
}
