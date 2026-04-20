package decision

import (
	"encoding/json"
	"testing"
)

func TestDecisionUnmarshalJSON_OldFormat(t *testing.T) {
	jsonData := `{
		"symbol": "BTCUSDT",
		"action": "open_long",
		"leverage": 5,
		"position_size_usd": 2200,
		"stop_loss": 97000,
		"take_profit": 101000,
		"confidence": 85,
		"reasoning": "Strong breakout"
	}`

	var d Decision
	if err := json.Unmarshal([]byte(jsonData), &d); err != nil {
		t.Fatalf("Failed to unmarshal old format: %v", err)
	}

	if d.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", d.Symbol)
	}
	if d.Leverage != 5 {
		t.Errorf("Expected leverage 5, got %d", d.Leverage)
	}
	if d.PositionSizeUSD != 2200 {
		t.Errorf("Expected position_size_usd 2200, got %.2f", d.PositionSizeUSD)
	}
	if d.StopLoss != 97000 {
		t.Errorf("Expected stop_loss 97000, got %.2f", d.StopLoss)
	}
	if d.Reasoning != "Strong breakout" {
		t.Errorf("Expected reasoning 'Strong breakout', got '%s'", d.Reasoning)
	}
}

func TestDecisionUnmarshalJSON_NewFormat(t *testing.T) {
	jsonData := `{
		"sym": "BTCUSDT",
		"action": "open_long",
		"lev": 5,
		"size_pct": 0.23,
		"stops_targets": {
			"sl": 97000,
			"tp": 101000
		},
		"reason_codes": ["bull_div_s", "dist_ok", "conf_ok"]
	}`

	var d Decision
	if err := json.Unmarshal([]byte(jsonData), &d); err != nil {
		t.Fatalf("Failed to unmarshal new format: %v", err)
	}

	if d.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", d.Symbol)
	}
	if d.Leverage != 5 {
		t.Errorf("Expected leverage 5, got %d", d.Leverage)
	}
	// size_pct is stored as negative until conversion
	if d.PositionSizeUSD != -0.23 {
		t.Errorf("Expected position_size_usd -0.23 (marker), got %.4f", d.PositionSizeUSD)
	}
	if d.StopLoss != 97000 {
		t.Errorf("Expected stop_loss 97000, got %.2f", d.StopLoss)
	}
	if d.TakeProfit != 101000 {
		t.Errorf("Expected take_profit 101000, got %.2f", d.TakeProfit)
	}
	expectedReasoning := "bull_div_s, dist_ok, conf_ok"
	if d.Reasoning != expectedReasoning {
		t.Errorf("Expected reasoning '%s', got '%s'", expectedReasoning, d.Reasoning)
	}
}

func TestDecisionUnmarshalJSON_MixedFormat(t *testing.T) {
	// Test that new format fields take precedence when both are present
	jsonData := `{
		"symbol": "ETHUSDT",
		"sym": "BTCUSDT",
		"action": "open_short",
		"leverage": 10,
		"lev": 5,
		"position_size_usd": 5000,
		"size_pct": 0.15
	}`

	var d Decision
	if err := json.Unmarshal([]byte(jsonData), &d); err != nil {
		t.Fatalf("Failed to unmarshal mixed format: %v", err)
	}

	// Old format should be used first (parsed by struct tags)
	// New format only used if old format is empty/zero
	if d.Symbol != "ETHUSDT" {
		t.Errorf("Expected symbol ETHUSDT (old format first), got %s", d.Symbol)
	}
	if d.Leverage != 10 {
		t.Errorf("Expected leverage 10 (old format first), got %d", d.Leverage)
	}
}

func TestValidateDecisions_SizePctConversion(t *testing.T) {
	decisions := []Decision{
		{
			Symbol:          "BTCUSDT",
			Action:          "open_long",
			Leverage:        5,
			PositionSizeUSD: -0.20, // Negative indicates size_pct format (20%)
			StopLoss:        97000,
			TakeProfit:      101000,
			Confidence:      85,
			Reasoning:       "Test trade",
		},
	}

	accountEquity := 10000.0
	btcEthLeverage := 10
	altcoinLeverage := 5

	err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage)
	if err != nil {
		t.Fatalf("validateDecisions failed: %v", err)
	}

	expectedUSD := accountEquity * 0.20 // 20% of 10000 = 2000
	if decisions[0].PositionSizeUSD != expectedUSD {
		t.Errorf("Expected position_size_usd %.2f after conversion, got %.2f",
			expectedUSD, decisions[0].PositionSizeUSD)
	}
}

func TestDecisionUnmarshalJSON_EmptyFields(t *testing.T) {
	// Test handling of empty strings in new format
	jsonData := `{
		"sym": "",
		"action": "hold",
		"reasoning": ""
	}`

	var d Decision
	if err := json.Unmarshal([]byte(jsonData), &d); err != nil {
		t.Fatalf("Failed to unmarshal with empty fields: %v", err)
	}

	// Empty sym should not override empty symbol
	if d.Symbol != "" {
		t.Errorf("Expected empty symbol, got '%s'", d.Symbol)
	}
	if d.Action != "hold" {
		t.Errorf("Expected action 'hold', got '%s'", d.Action)
	}
}

func TestDecisionUnmarshalJSON_OnlyReasonCodes(t *testing.T) {
	jsonData := `{
		"symbol": "ETHUSDT",
		"action": "open_long",
		"reason_codes": ["trend_up", "volume_confirm"]
	}`

	var d Decision
	if err := json.Unmarshal([]byte(jsonData), &d); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	expectedReasoning := "trend_up, volume_confirm"
	if d.Reasoning != expectedReasoning {
		t.Errorf("Expected reasoning '%s', got '%s'", expectedReasoning, d.Reasoning)
	}
}

func TestDecisionUnmarshalJSON_StopsTargetsOnly(t *testing.T) {
	jsonData := `{
		"symbol": "SOLUSDT",
		"action": "open_long",
		"stops_targets": {
			"sl": 150.5,
			"tp": 180.0
		}
	}`

	var d Decision
	if err := json.Unmarshal([]byte(jsonData), &d); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.StopLoss != 150.5 {
		t.Errorf("Expected stop_loss 150.5, got %.2f", d.StopLoss)
	}
	if d.TakeProfit != 180.0 {
		t.Errorf("Expected take_profit 180.0, got %.2f", d.TakeProfit)
	}
}
