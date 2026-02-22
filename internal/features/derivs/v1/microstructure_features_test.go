package derivsv1

import (
	"math"
	"testing"
)

func TestComputeAVWAPMetricsProducesFeature6Fields(t *testing.T) {
	candles := makeTestCandles(120)
	metrics := computeAVWAPMetrics(candles, 2.5)

	required := []string{
		"avwap_up_name_3m",
		"avwap_up_dist_atr_3m",
		"avwap_dn_name_3m",
		"avwap_dn_dist_atr_3m",
		"avwap_bias_3m",
		"confidence_avwap_3m",
		"avwap_bias_15m",
		"confidence_avwap_15m",
	}
	for _, key := range required {
		if _, ok := metrics[key]; !ok {
			t.Fatalf("expected key %s in AVWAP metrics", key)
		}
	}

	if conf, ok := metrics["confidence_avwap_3m"].(float64); !ok || conf <= 0 || conf > 1 {
		t.Fatalf("invalid confidence_avwap_3m: %#v", metrics["confidence_avwap_3m"])
	}
}

func TestComputeVolatilityMetricsProducesFeature7Fields(t *testing.T) {
	candles := makeTestCandles(200)
	metrics := computeVolatilityMetrics(candles, 2.0)

	required := []string{
		"bbw_3m",
		"kc_width_3m",
		"bbw_pct_rank_3m",
		"squeeze_on_3m",
		"rv_ratio_3m",
		"vol_regime_3m",
		"confidence_vol_3m",
		"squeeze_on_15m",
		"rv_ratio_15m",
		"vol_regime_15m",
	}
	for _, key := range required {
		if _, ok := metrics[key]; !ok {
			t.Fatalf("expected key %s in volatility metrics", key)
		}
	}

	regime, ok := metrics["vol_regime_3m"].(string)
	if !ok || regime == "" {
		t.Fatalf("invalid vol_regime_3m: %#v", metrics["vol_regime_3m"])
	}
}

func TestComputeLiqMetricsWithNoEventsUsesFallbackDistances(t *testing.T) {
	metrics := computeLiqMetrics(nil, 100, 2)

	count, ok := metrics["event_count_liq_3m"].(int)
	if !ok || count != 0 {
		t.Fatalf("expected event_count_liq_3m=0, got %#v", metrics["event_count_liq_3m"])
	}
	conf, ok := metrics["confidence_liq_3m"].(float64)
	if !ok || conf <= 0 || conf > 1 {
		t.Fatalf("expected fallback confidence_liq_3m in (0,1], got %#v", metrics["confidence_liq_3m"])
	}
	if _, ok := metrics["dist_up_atr_3m"].(float64); !ok {
		t.Fatalf("expected dist_up_atr_3m fallback value")
	}
	if _, ok := metrics["dist_dn_atr_3m"].(float64); !ok {
		t.Fatalf("expected dist_dn_atr_3m fallback value")
	}
	if v, ok := metrics["liq_risk_up_3m"].(int); !ok || v != 0 {
		t.Fatalf("expected liq_risk_up_3m=0 in fallback, got %#v", metrics["liq_risk_up_3m"])
	}
	if v, ok := metrics["liq_risk_down_3m"].(int); !ok || v != 0 {
		t.Fatalf("expected liq_risk_down_3m=0 in fallback, got %#v", metrics["liq_risk_down_3m"])
	}
}

func TestComputeLiqMetricsSingleSideStillFillsBothDistances(t *testing.T) {
	events := []liqEvent{
		{price: 105, size: 2000, side: "Sell", ts: 1},
	}
	metrics := computeLiqMetrics(events, 100, 2)

	if _, ok := metrics["dist_up_atr_3m"].(float64); !ok {
		t.Fatalf("expected dist_up_atr_3m from real event")
	}
	if _, ok := metrics["dist_dn_atr_3m"].(float64); !ok {
		t.Fatalf("expected dist_dn_atr_3m fallback for missing side")
	}
	if v, ok := metrics["liq_risk_down_3m"].(int); !ok || v != 0 {
		t.Fatalf("expected liq_risk_down_3m fallback=0, got %#v", metrics["liq_risk_down_3m"])
	}
}

func makeTestCandles(n int) []klineCandle {
	out := make([]klineCandle, 0, n)
	for i := 0; i < n; i++ {
		base := 100.0 + float64(i)*0.08
		wave := math.Sin(float64(i) * 0.2)
		closeP := base + wave*1.2
		high := closeP + 1.0 + 0.3*math.Sin(float64(i)*0.11)
		low := closeP - 1.0 - 0.3*math.Cos(float64(i)*0.13)
		out = append(out, klineCandle{
			OpenTime: int64(i) * 300000,
			Open:     closeP - 0.2,
			High:     high,
			Low:      low,
			Close:    closeP,
		})
	}
	return out
}
