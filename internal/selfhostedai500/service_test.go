package selfhostedai500

import (
	"testing"
	"time"
)

func TestGetAI500DetailResponseIncludesScoreComponentsAndStartRegime(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	service := &Service{
		cfg: Config{ScoreThreshold: 70},
		snapshots: map[string]*marketSnapshot{
			"BTCUSDT": {
				Symbol:            "BTCUSDT",
				Pair:              "BTCUSDT",
				Price:             100,
				Rank:              1,
				Score:             82,
				StartTime:         now.Add(-2 * time.Hour).Unix(),
				StartPrice:        95,
				LastScore:         78,
				MaxScore:          84,
				MaxPrice:          101,
				IncreasePercent:   5.26,
				ReasonCodes:       []string{"trend_aligned"},
				PriceChange:       map[string]float64{"1h": 0.01, "4h": 0.03, "24h": 0.08},
				ScoreComponents:   scoreComponents{Liquidity: 20, OI: 15, Momentum: 30, Flow: 10, Adjust: 7, Total: 82},
				StartRegimeActive: true,
			},
		},
		lastRefresh: now,
	}

	detail, ok := service.GetAI500DetailResponse("BTC")
	if !ok {
		t.Fatal("expected detail response")
	}
	if detail.CurrentPrice != 100 {
		t.Fatalf("current price = %v, want 100", detail.CurrentPrice)
	}
	if detail.ScoreComponents["total"] != 82 {
		t.Fatalf("total score component = %v, want 82", detail.ScoreComponents["total"])
	}
	if active, _ := detail.StartRegime["active"].(bool); !active {
		t.Fatal("expected active start regime")
	}
}

func TestGetAI500StatsIncludesDistributionBuckets(t *testing.T) {
	service := &Service{
		cfg: Config{ScoreThreshold: 70},
		snapshots: map[string]*marketSnapshot{
			"A": {Score: 45},
			"B": {Score: 75},
			"C": {Score: 92},
		},
		lastRefresh: time.Unix(1_700_000_000, 0).UTC(),
	}

	stats := service.GetAI500Stats()
	if got := stats["active_count"]; got != 2 {
		t.Fatalf("active_count = %v, want 2", got)
	}
	dist, ok := stats["score_distribution"].(map[string]int)
	if !ok {
		t.Fatalf("score_distribution type = %T, want map[string]int", stats["score_distribution"])
	}
	if dist["lt50"] != 1 || dist["70_79"] != 1 || dist["90_100"] != 1 {
		t.Fatalf("unexpected distribution buckets: %#v", dist)
	}
}

func TestGetNetflowRankingHonorsTradeType(t *testing.T) {
	service := &Service{
		netflowRankings: map[string]map[string]map[string]netflowRankingCacheEntry{
			"1h": {
				"future": {
					"institution": {Top: []netflowPositionResponse{{Rank: 1, Symbol: "BTCUSDT", Amount: 100}}},
				},
				"spot": {
					"institution": {Top: []netflowPositionResponse{{Rank: 1, Symbol: "BTCUSDT", Amount: 25}}},
				},
			},
		},
	}

	future := service.GetNetflowRanking("1h", 10, "top", "institution", "future")
	spot := service.GetNetflowRanking("1h", 10, "top", "institution", "spot")
	if len(future) != 1 || len(spot) != 1 {
		t.Fatalf("unexpected result lengths: future=%d spot=%d", len(future), len(spot))
	}
	if future[0].Amount == spot[0].Amount {
		t.Fatalf("expected trade-specific netflow values to differ, got future=%v spot=%v", future[0].Amount, spot[0].Amount)
	}
}

func TestBuildPriceRankingCachesIncludesSpotFlow(t *testing.T) {
	caches := buildPriceRankingCaches(map[string]*marketSnapshot{
		"BTCUSDT": {
			Symbol:                "BTCUSDT",
			Pair:                  "BTCUSDT",
			Price:                 100,
			OpenInterest:          10,
			PriceChange:           map[string]float64{"1h": 0.02, "4h": 0, "24h": 0},
			OIDeltas:              map[string]oiDelta{"1h": {DeltaValue: 10}, "4h": {}, "24h": {}},
			InstitutionFutureFlow: map[string]float64{"1h": 50, "4h": 0, "24h": 0},
			InstitutionSpotFlow:   map[string]float64{"1h": 15, "4h": 0, "24h": 0},
		},
	})

	top := caches["1h"].Top
	if len(top) != 1 {
		t.Fatalf("expected one price ranking entry, got %d", len(top))
	}
	if top[0].SpotFlow != 15 {
		t.Fatalf("spot_flow = %v, want 15", top[0].SpotFlow)
	}
}

func TestGetCoinUnknownIncludeFallsBackToCorePayload(t *testing.T) {
	service := &Service{
		snapshots: map[string]*marketSnapshot{
			"BTCUSDT": {
				Symbol:                "BTCUSDT",
				Pair:                  "BTCUSDT",
				Price:                 100,
				OpenInterest:          10,
				PriceChange:           map[string]float64{"1h": 0.02, "4h": 0.03, "24h": 0.04},
				OIDeltas:              map[string]oiDelta{"1h": {DeltaValue: 10}, "4h": {}, "24h": {}},
				InstitutionFutureFlow: map[string]float64{"1h": 5, "4h": 6, "24h": 7},
				PersonalFutureFlow:    map[string]float64{"1h": 1, "4h": 2, "24h": 3},
				InstitutionSpotFlow:   map[string]float64{"1h": 4, "4h": 5, "24h": 6},
				PersonalSpotFlow:      map[string]float64{"1h": 1, "4h": 1, "24h": 1},
			},
		},
	}

	coin, ok := service.GetCoin("BTC", "unknown_field")
	if !ok {
		t.Fatal("expected BTC coin response")
	}
	if coin.PriceChange == nil || coin.OI == nil || coin.Netflow == nil {
		t.Fatalf("expected fallback core payload, got %+v", coin)
	}
}

func TestGetScoreDebugIncludesProxyInputs(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	service := &Service{
		cfg: Config{ScoreThreshold: 70},
		snapshots: map[string]*marketSnapshot{
			"BTCUSDT": {
				Symbol:                "BTCUSDT",
				Pair:                  "BTCUSDT",
				Price:                 100,
				Volume24H:             5_000_000,
				OpenInterest:          150_000,
				Funding:               0.0001,
				Premium:               0.0003,
				SpreadBps:             3,
				Rank:                  1,
				Score:                 82,
				StartTime:             now.Add(-2 * time.Hour).Unix(),
				StartPrice:            95,
				ReasonCodes:           []string{"trend_aligned"},
				PriceChange:           map[string]float64{"1h": 0.01, "4h": 0.02, "24h": 0.03},
				OIDeltas:              map[string]oiDelta{"1h": {Delta: 100, DeltaValue: 10_000, DeltaPercent: 5}, "4h": {}, "24h": {}},
				InstitutionFutureFlow: map[string]float64{"1h": 50, "4h": 60, "24h": 70},
				PersonalFutureFlow:    map[string]float64{"1h": 10, "4h": 11, "24h": 12},
				InstitutionSpotFlow:   map[string]float64{"1h": 5, "4h": 6, "24h": 7},
				PersonalSpotFlow:      map[string]float64{"1h": 1, "4h": 2, "24h": 3},
				ScoreComponents:       scoreComponents{Liquidity: 20, OI: 15, Momentum: 30, Flow: 10, Adjust: 7, Total: 82},
				StartRegimeActive:     true,
			},
		},
		lastRefresh: now,
	}

	debug, ok := service.GetScoreDebug("BTC")
	if !ok {
		t.Fatal("expected debug response")
	}
	if debug.NetflowProxy["institution"].Future["1h"] != 50 {
		t.Fatalf("institution future 1h = %v, want 50", debug.NetflowProxy["institution"].Future["1h"])
	}
	if debug.OIDeltas["1h"].OIDeltaPercent != 5 {
		t.Fatalf("1h oi delta percent = %v, want 5", debug.OIDeltas["1h"].OIDeltaPercent)
	}
}

func TestGetRankingDebugReturnsKindSpecificPayloads(t *testing.T) {
	service := &Service{
		cfg: Config{ScoreThreshold: 70},
		snapshots: map[string]*marketSnapshot{
			"BTCUSDT": {
				Symbol:       "BTCUSDT",
				Pair:         "BTCUSDT",
				Price:        100,
				Volume24H:    2_000_000,
				OpenInterest: 25_000,
				Score:        80,
				Rank:         1,
				ReasonCodes:  []string{"trend_aligned"},
			},
		},
		oiRankings: map[string]oiRankingCacheEntry{
			"1h": {Top: []oiPositionResponse{{Rank: 1, Symbol: "BTCUSDT"}}, Low: []oiPositionResponse{{Rank: 1, Symbol: "ETHUSDT"}}},
		},
		priceRankings: map[string]priceRankingCacheEntry{
			"1h": {Top: []priceRankingItemResponse{{Symbol: "BTCUSDT"}}, Low: []priceRankingItemResponse{{Symbol: "ETHUSDT"}}},
		},
		netflowRankings: map[string]map[string]map[string]netflowRankingCacheEntry{
			"1h": {
				"future": {
					"institution": {Top: []netflowPositionResponse{{Rank: 1, Symbol: "BTCUSDT"}}, Low: []netflowPositionResponse{{Rank: 1, Symbol: "ETHUSDT"}}},
				},
			},
		},
		lastRefresh: time.Unix(1_700_000_000, 0).UTC(),
	}

	ai500, err := service.GetRankingDebug("ai500", "1h", "1h", 10, "institution", "future")
	if err != nil {
		t.Fatalf("ai500 debug error: %v", err)
	}
	if len(ai500.AI500) != 1 {
		t.Fatalf("ai500 debug count = %d, want 1", len(ai500.AI500))
	}

	oi, err := service.GetRankingDebug("oi", "1h", "1h", 10, "institution", "future")
	if err != nil {
		t.Fatalf("oi debug error: %v", err)
	}
	if oi.OI == nil || len(oi.OI.Top) != 1 || len(oi.OI.Low) != 1 {
		t.Fatalf("unexpected oi debug payload: %+v", oi.OI)
	}

	price, err := service.GetRankingDebug("price", "1h", "1h,4h", 10, "institution", "future")
	if err != nil {
		t.Fatalf("price debug error: %v", err)
	}
	if price.Price == nil || len(price.Price["1h"]["top"]) != 1 {
		t.Fatalf("unexpected price debug payload: %+v", price.Price)
	}

	netflow, err := service.GetRankingDebug("netflow", "1h", "1h", 10, "institution", "future")
	if err != nil {
		t.Fatalf("netflow debug error: %v", err)
	}
	if netflow.Netflow == nil || len(netflow.Netflow.Top) != 1 || len(netflow.Netflow.Low) != 1 {
		t.Fatalf("unexpected netflow debug payload: %+v", netflow.Netflow)
	}
}
