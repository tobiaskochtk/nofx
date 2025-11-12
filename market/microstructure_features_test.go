package market

import (
	"testing"
	"time"

	"nofx/pkg/types"
)

func TestEnrichMicrostructureFeaturesPopulatesFeature4(t *testing.T) {
	bars := makeTestBars(500, 3*time.Minute)
	dest := &types.DerivsFeatures{}
	if err := enrichMicrostructureFeatures("TESTUSDT", bars, dest); err != nil {
		t.Fatalf("enrichMicrostructureFeatures error: %v", err)
	}
	if dest.CVDNotionalZ3mShort == nil || dest.TBRNotional3m == nil {
		t.Fatalf("expected Feature4 fields to be populated, got %+v", dest)
	}
	if dest.DistUpAtr3m == nil || dest.ConfidenceLiq3m == nil {
		t.Fatalf("expected Feature5 fields to be populated, got %+v", dest)
	}
}

func makeTestBars(count int, interval time.Duration) []Kline {
	bars := make([]Kline, count)
	base := time.Now().Add(-interval * time.Duration(count))
	price := 100.0
	for i := 0; i < count; i++ {
		ts := base.Add(interval * time.Duration(i+1))
		open := price
		high := open * (1 + 0.002)
		low := open * (1 - 0.002)
		close := open * (1 + 0.001)
		volume := 1000 + float64(i%50)
		bars[i] = Kline{
			OpenTime:            ts.Add(-interval).UnixMilli(),
			CloseTime:           ts.UnixMilli(),
			Open:                open,
			High:                high,
			Low:                 low,
			Close:               close,
			Volume:              volume,
			QuoteVolume:         volume * close,
			TakerBuyBaseVolume:  volume * 0.55,
			TakerBuyQuoteVolume: volume * close * 0.55,
			Trades:              100 + i%20,
		}
		price = close
	}
	return bars
}
