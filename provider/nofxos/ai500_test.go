package nofxos

import "testing"

func TestNormalizeValidSymbolRejectsNonASCIIPairs(t *testing.T) {
	if _, ok := NormalizeValidSymbol("币安人生USDT"); ok {
		t.Fatal("expected non-ASCII pair to be rejected")
	}
	if _, ok := NormalizeValidSymbol("BTC-USDT"); ok {
		t.Fatal("expected punctuated pair to be rejected")
	}
	normalized, ok := NormalizeValidSymbol("1000pepe")
	if !ok {
		t.Fatal("expected 1000pepe to be accepted")
	}
	if normalized != "1000PEPEUSDT" {
		t.Fatalf("normalized = %q, want %q", normalized, "1000PEPEUSDT")
	}
}

func TestFilterValidAI500CoinsDropsInvalidPairs(t *testing.T) {
	coins, dropped := filterValidAI500Coins([]CoinData{
		{Pair: "BTCUSDT", Score: 91},
		{Pair: "币安人生USDT", Score: 99},
		{Pair: "1000PEPE", Score: 88},
	})
	if dropped != 1 {
		t.Fatalf("dropped = %d, want 1", dropped)
	}
	if len(coins) != 2 {
		t.Fatalf("len(coins) = %d, want 2", len(coins))
	}
	if coins[0].Pair != "BTCUSDT" || coins[1].Pair != "1000PEPEUSDT" {
		t.Fatalf("unexpected normalized pairs: %#v", coins)
	}
}
