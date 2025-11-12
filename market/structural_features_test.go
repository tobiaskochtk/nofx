package market

import (
	"math"
	"testing"
	"time"

	"nofx/pkg/types"
)

func TestEnrichStructuralFeaturesPopulatesFields(t *testing.T) {
	bars := mockStructuralBars(structuralPreferredBars3m, 1.0)
	dest := &types.DerivsFeatures{}

	if err := enrichStructuralFeatures("TESTUSDT", bars, dest, nil); err != nil {
		t.Fatalf("enrichStructuralFeatures error: %v", err)
	}

	if dest.AVWAPUpName3m == nil && dest.AVWAPDnName3m == nil {
		t.Fatalf("expected at least one AVWAP anchor to be populated, got up=%v down=%v", dest.AVWAPUpName3m, dest.AVWAPDnName3m)
	}
	if dest.ConfidenceAVWAP3m == nil || *dest.ConfidenceAVWAP3m <= 0 {
		t.Fatalf("expected confidence_avwap_3m to be > 0, got %v", dest.ConfidenceAVWAP3m)
	}
	if dest.BBW3m == nil || dest.KCWidth3m == nil {
		t.Fatalf("expected volatility widths to be populated, bbw=%v kc=%v", dest.BBW3m, dest.KCWidth3m)
	}
	if dest.VolRegime3m == nil || *dest.VolRegime3m == "" {
		t.Fatalf("expected vol_regime_3m to be populated, got %v", dest.VolRegime3m)
	}
	if dest.SqueezeOn15m == nil {
		t.Fatalf("expected 15m confirmation fields to be populated")
	}
}

func mockStructuralBars(count int, startPrice float64) []Kline {
	bars := make([]Kline, count)
	baseTime := time.Now().Add(-time.Duration(count) * 3 * time.Minute)
	price := startPrice
	for i := 0; i < count; i++ {
		openTime := baseTime.Add(time.Duration(i) * 3 * time.Minute).UnixMilli()
		closeTime := baseTime.Add(time.Duration(i+1) * 3 * time.Minute).UnixMilli()

		// introduce a mild trend and oscillation
		price += 0.01
		high := price + 0.05
		low := price - 0.05
		close := price + 0.02*math.Sin(float64(i)/10)

		bars[i] = Kline{
			OpenTime:            openTime,
			Open:                price,
			High:                high,
			Low:                 low,
			Close:               close,
			CloseTime:           closeTime,
			Volume:              1000 + float64(i%50),
			QuoteVolume:         5000 + float64(i%25),
			TakerBuyBaseVolume:  600 + float64(i%40),
			TakerBuyQuoteVolume: 3000 + float64(i%30),
			Trades:              100 + i%10,
		}
	}
	return bars
}
