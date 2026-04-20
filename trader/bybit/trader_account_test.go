package bybit

import "testing"

func TestParseClosedPnLResultMapsBybitCloseLongSideAndFee(t *testing.T) {
	trader := &BybitTrader{}

	result := map[string]interface{}{
		"list": []interface{}{
			map[string]interface{}{
				"symbol":        "TONUSDT",
				"side":          "Sell",
				"orderId":       "order-1",
				"avgEntryPrice": "1.4937",
				"avgExitPrice":  "1.4652",
				"qty":           "16",
				"closedPnl":     "-0.4852",
				"cumEntryValue": "23.90",
				"cumExitValue":  "23.44",
				"leverage":      "1",
				"createdTime":   "1775947336000",
				"updatedTime":   "1775954536000",
			},
		},
	}

	records, err := trader.parseClosedPnLResult(result)
	if err != nil {
		t.Fatalf("parseClosedPnLResult() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}

	record := records[0]
	if record.Side != "long" {
		t.Fatalf("record.Side = %q, want long", record.Side)
	}
	if record.Fee < 0.02 || record.Fee > 0.04 {
		t.Fatalf("record.Fee = %.6f, want approx 0.03", record.Fee)
	}
}
