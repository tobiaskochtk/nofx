package kernel

import (
	"testing"

	"nofx/market"
	"nofx/store"
)

func TestMergeCandidateCoin_PreservesSelectionBucketAcrossMixedSources(t *testing.T) {
	candidateMap := make(map[string]CandidateCoin)

	mergeCandidateCoin(candidateMap, CandidateCoin{
		Symbol:          "solusdt",
		Sources:         []string{"ai500"},
		SelectionBucket: "adaptive",
	})
	mergeCandidateCoin(candidateMap, CandidateCoin{
		Symbol:  "SOLUSDT",
		Sources: []string{"oi_low"},
	})

	coin, ok := candidateMap["SOLUSDT"]
	if !ok {
		t.Fatal("expected merged candidate to exist")
	}
	if coin.SelectionBucket != "adaptive" {
		t.Fatalf("selection bucket = %q, want adaptive", coin.SelectionBucket)
	}
	if len(coin.Sources) != 2 || coin.Sources[0] != "ai500" || coin.Sources[1] != "oi_low" {
		t.Fatalf("sources = %#v, want [ai500 oi_low]", coin.Sources)
	}
}

func TestBuildGridContextFromMarketData_UsesCoherentSeriesEMAValues(t *testing.T) {
	series := &market.TimeframeSeriesData{
		Timeframe:   "5m",
		EMA20Values: []float64{101.5, 102.5},
		EMA50Values: []float64{98.2, 99.4},
		RSI14Values: []float64{54, 57},
		BOLLUpper:   []float64{104, 105},
		BOLLMiddle:  []float64{100, 101},
		BOLLLower:   []float64{96, 97},
	}

	ctx := BuildGridContextFromMarketData(&market.Data{
		Symbol:        "BTCUSDT",
		CurrentPrice:  103,
		CurrentMACD:   0.6,
		CurrentEMA20:  88,
		TimeframeData: map[string]*market.TimeframeSeriesData{"5m": series},
		LongerTermContext: &market.LongerTermData{
			EMA20: 77,
			EMA50: 0,
		},
	}, &store.GridStrategyConfig{Symbol: "BTCUSDT"})

	if ctx.EMA20 != 102.5 {
		t.Fatalf("EMA20 = %.2f, want 102.50 from 5m timeframe data", ctx.EMA20)
	}
	if ctx.EMA50 != 99.4 {
		t.Fatalf("EMA50 = %.2f, want 99.40 from 5m timeframe data", ctx.EMA50)
	}
	if ctx.BollingerUpper != 105 || ctx.BollingerMiddle != 101 || ctx.BollingerLower != 97 {
		t.Fatalf("unexpected bollinger values: %+v", ctx)
	}
	if ctx.RSI14 != 57 {
		t.Fatalf("RSI14 = %.2f, want 57", ctx.RSI14)
	}
}
