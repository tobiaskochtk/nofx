package bybit

import "testing"

func TestParseBybitTriggerOrdersResult(t *testing.T) {
	list := []map[string]interface{}{
		{
			"symbol":         "RAVEUSDT",
			"orderId":        "stop-order-1",
			"orderLinkId":    "sl-rave-1",
			"side":           "Sell",
			"orderType":      "Market",
			"stopOrderType":  "StopLoss",
			"triggerBy":      "LastPrice",
			"triggerPrice":   "3.674",
			"price":          "0",
			"qty":            "3",
			"cumExecQty":     "3",
			"avgPrice":       "3.6695",
			"cumExecFee":     "0.021",
			"orderStatus":    "Filled",
			"reduceOnly":     true,
			"closeOnTrigger": true,
			"createdTime":    "1776018000000",
			"updatedTime":    "1776019000000",
		},
		{
			"symbol":         "TONUSDT",
			"orderId":        "tp-order-1",
			"side":           "Buy",
			"orderType":      "Market",
			"stopOrderType":  "TakeProfit",
			"triggerPrice":   "1.317",
			"price":          "0",
			"qty":            "18.1",
			"cumExecQty":     "0",
			"avgPrice":       "",
			"cumExecFee":     "0",
			"orderStatus":    "Untriggered",
			"reduceOnly":     true,
			"closeOnTrigger": false,
			"createdTime":    "1776010000000",
			"updatedTime":    "1776010100000",
		},
	}

	orders, err := parseBybitTriggerOrdersResult(list)
	if err != nil {
		t.Fatalf("parseBybitTriggerOrdersResult() error = %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("len(orders) = %d, want 2", len(orders))
	}

	if orders[0].OrderAction != "close_long" {
		t.Fatalf("orders[0].OrderAction = %q, want close_long", orders[0].OrderAction)
	}
	if orders[0].Status != "FILLED" {
		t.Fatalf("orders[0].Status = %q, want FILLED", orders[0].Status)
	}
	if orders[0].StopOrderType != "StopLoss" {
		t.Fatalf("orders[0].StopOrderType = %q, want StopLoss", orders[0].StopOrderType)
	}
	if orders[0].OrderLinkID != "sl-rave-1" {
		t.Fatalf("orders[0].OrderLinkID = %q, want sl-rave-1", orders[0].OrderLinkID)
	}
	if orders[0].TriggerBy != "LastPrice" {
		t.Fatalf("orders[0].TriggerBy = %q, want LastPrice", orders[0].TriggerBy)
	}
	if orders[1].OrderAction != "close_short" {
		t.Fatalf("orders[1].OrderAction = %q, want close_short", orders[1].OrderAction)
	}
	if orders[1].Status != "NEW" {
		t.Fatalf("orders[1].Status = %q, want NEW", orders[1].Status)
	}
	if orders[1].TriggerPrice != 1.317 {
		t.Fatalf("orders[1].TriggerPrice = %v, want 1.317", orders[1].TriggerPrice)
	}
}
