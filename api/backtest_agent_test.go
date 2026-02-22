package api

import (
	"testing"

	"nofx/backtest"
	"nofx/store"
)

func TestInferStrategyConfigFromDescription(t *testing.T) {
	cfg := inferStrategyConfigFromDescription("Create a momentum strategy for BTC with RSI and MACD on 15m")

	if cfg.CoinSource.SourceType != "static" {
		t.Fatalf("expected static source type, got %s", cfg.CoinSource.SourceType)
	}
	if len(cfg.CoinSource.StaticCoins) == 0 || cfg.CoinSource.StaticCoins[0] != "BTCUSDT" {
		t.Fatalf("expected BTCUSDT as first symbol, got %v", cfg.CoinSource.StaticCoins)
	}
	if !cfg.Indicators.EnableRSI {
		t.Fatal("expected RSI to be enabled")
	}
	if !cfg.Indicators.EnableMACD {
		t.Fatal("expected MACD to be enabled")
	}
	if cfg.Indicators.Klines.PrimaryTimeframe != "15m" {
		t.Fatalf("expected primary timeframe 15m, got %s", cfg.Indicators.Klines.PrimaryTimeframe)
	}
}

func TestGenerateImprovementPlans(t *testing.T) {
	strategyCfg := store.GetDefaultStrategyConfig("en")
	runCfg := backtest.BacktestConfig{
		DecisionTimeframe:    "15m",
		DecisionCadenceNBars: 20,
	}
	metrics := &backtest.Metrics{
		TotalReturnPct: 5.0,
		MaxDrawdownPct: 12.0,
		SharpeRatio:    0.8,
		Trades:         140,
		WinRate:        40,
	}

	plans := generateImprovementPlans(metrics, "improve by adding volatility filter", strategyCfg, runCfg)
	if len(plans) == 0 {
		t.Fatal("expected non-empty improvement plans")
	}

	ids := make(map[string]bool)
	for _, p := range plans {
		ids[p.Suggestion.ID] = true
	}

	for _, required := range []string{"drawdown_shield", "sharpe_boost", "trade_frequency_control", "volatility_filter"} {
		if !ids[required] {
			t.Fatalf("expected improvement %s to be present, got %v", required, ids)
		}
	}
}
