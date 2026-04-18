package gate

import "testing"

func TestInferGateTriggerSubtype(t *testing.T) {
	tests := []struct {
		name string
		size int64
		rule int32
		want string
	}{
		{name: "close long stop loss", size: -5, rule: 1, want: "stop_loss"},
		{name: "close long take profit", size: -5, rule: 2, want: "take_profit"},
		{name: "close short take profit", size: 5, rule: 1, want: "take_profit"},
		{name: "close short stop loss", size: 5, rule: 2, want: "stop_loss"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferGateTriggerSubtype(tt.size, tt.rule); got != tt.want {
				t.Fatalf("inferGateTriggerSubtype(%d, %d) = %q, want %q", tt.size, tt.rule, got, tt.want)
			}
		})
	}
}

func TestNormalizeGateTriggerHistoryStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		finishAs string
		want     string
	}{
		{name: "finished succeeded", status: "finished", finishAs: "succeeded", want: "FILLED"},
		{name: "finished cancelled", status: "finished", finishAs: "cancelled", want: "CANCELED"},
		{name: "open", status: "open", finishAs: "", want: "NEW"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeGateTriggerHistoryStatus(tt.status, tt.finishAs); got != tt.want {
				t.Fatalf("normalizeGateTriggerHistoryStatus(%q, %q) = %q, want %q", tt.status, tt.finishAs, got, tt.want)
			}
		})
	}
}
