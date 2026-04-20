package decision

import (
	"testing"
)

func TestRemoveInvisibleRunes_TildeReplacement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Tilde in reasoning text",
			input:    `{"reasoning": "Risk-reward ~1:3 with ATR-based stops."}`,
			expected: `{"reasoning": "Risk-reward 约1:3 with ATR-based stops."}`,
		},
		{
			name:     "Multiple tildes",
			input:    `{"notes": "Target ~$2.50, risk ~0.5%"}`,
			expected: `{"notes": "Target 约$2.50, risk 约0.5%"}`,
		},
		{
			name:     "Tilde in numeric context (would be invalid JSON anyway)",
			input:    `{"leverage": 10~20}`,
			expected: `{"leverage": 10约20}`,
		},
		{
			name:     "Real-world example from error log",
			input:    `"reasoning": "Strong multi-signal confluence: EMA20 support (96.72), MACD bullish (0.371), RSI neutral-bullish (62.97), f5 prefers longs (0.966 conf), f6 neutral bias with swing_hi target, f7 expansion regime. Risk-reward ~1:3 with ATR-based stops."`,
			expected: `"reasoning": "Strong multi-signal confluence: EMA20 support (96.72), MACD bullish (0.371), RSI neutral-bullish (62.97), f5 prefers longs (0.966 conf), f6 neutral bias with swing_hi target, f7 expansion regime. Risk-reward 约1:3 with ATR-based stops."`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeInvisibleRunes(tt.input)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}
