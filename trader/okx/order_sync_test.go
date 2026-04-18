package okx

import "testing"

func TestParseOKXAlgoOrderKind(t *testing.T) {
	tests := []struct {
		name          string
		slTriggerPx   string
		tpTriggerPx   string
		triggerPx     string
		slOrdPx       string
		tpOrdPx       string
		triggerPxType string
		slPxType      string
		tpPxType      string
		wantSubtype   string
		wantTrigger   float64
		wantOrder     float64
		wantSource    string
	}{
		{
			name:        "stop loss",
			slTriggerPx: "1.25",
			slOrdPx:     "-1",
			slPxType:    "mark",
			wantSubtype: "stop_loss",
			wantTrigger: 1.25,
			wantOrder:   -1,
			wantSource:  "mark",
		},
		{
			name:        "take profit",
			tpTriggerPx: "1.75",
			tpOrdPx:     "-1",
			tpPxType:    "last",
			wantSubtype: "take_profit",
			wantTrigger: 1.75,
			wantOrder:   -1,
			wantSource:  "last",
		},
		{
			name:          "generic trigger",
			triggerPx:     "1.10",
			triggerPxType: "index",
			wantSubtype:   "",
			wantTrigger:   1.10,
			wantOrder:     0,
			wantSource:    "index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subtype, triggerPrice, orderPrice, source := parseOKXAlgoOrderKind(
				tt.slTriggerPx,
				tt.tpTriggerPx,
				tt.triggerPx,
				tt.slOrdPx,
				tt.tpOrdPx,
				tt.triggerPxType,
				tt.slPxType,
				tt.tpPxType,
			)
			if subtype != tt.wantSubtype || triggerPrice != tt.wantTrigger || orderPrice != tt.wantOrder || source != tt.wantSource {
				t.Fatalf("parseOKXAlgoOrderKind() = (%q, %v, %v, %q), want (%q, %v, %v, %q)",
					subtype, triggerPrice, orderPrice, source,
					tt.wantSubtype, tt.wantTrigger, tt.wantOrder, tt.wantSource)
			}
		})
	}
}

func TestInferOKXTriggerOrderAction(t *testing.T) {
	tests := []struct {
		name    string
		side    string
		posSide string
		want    string
	}{
		{name: "sell long", side: "sell", posSide: "long", want: "close_long"},
		{name: "buy short", side: "buy", posSide: "short", want: "close_short"},
		{name: "unsupported", side: "buy", posSide: "long", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferOKXTriggerOrderAction(tt.side, tt.posSide); got != tt.want {
				t.Fatalf("inferOKXTriggerOrderAction(%q, %q) = %q, want %q", tt.side, tt.posSide, got, tt.want)
			}
		})
	}
}
