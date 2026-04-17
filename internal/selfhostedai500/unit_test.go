package selfhostedai500

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScoreSnapshotAppliesPenaltyAdjustments(t *testing.T) {
	snapshot := &marketSnapshot{
		Price:        1,
		OpenInterest: 1_000,
		Volume24H:    500_000,
		SpreadBps:    20,
		PriceChange: map[string]float64{
			"1h":  -0.02,
			"4h":  -0.03,
			"24h": -0.01,
		},
	}

	score, reasons, components := scoreSnapshot(snapshot, scoreContext{}, []float64{500_000, 2_000_000}, []float64{1_000, 5_000_000})

	for _, reason := range []string{"wide_spread", "thin_volume", "negative_momentum"} {
		if !containsString(reasons, reason) {
			t.Fatalf("expected reason %q in %#v", reason, reasons)
		}
	}
	if components.Adjust >= 0 {
		t.Fatalf("expected negative adjustment, got %v", components.Adjust)
	}
	if score != components.Total {
		t.Fatalf("score = %v, total component = %v", score, components.Total)
	}
	if score < 0 || score > 100 {
		t.Fatalf("score should be clamped to [0,100], got %v", score)
	}
}

func TestEvaluateCandidateGateRejectsWeakRelativeStrengthAndRisk(t *testing.T) {
	snapshot := &marketSnapshot{
		Symbol:                "ALTUSDT",
		Price:                 1,
		OpenInterest:          1_000,
		Volume24H:             4_000_000,
		Sources:               []string{"hyperliquid"},
		PriceChange:           map[string]float64{"1h": -0.01, "4h": -0.02, "24h": -0.03},
		RelativeStrengthScore: 35,
		RegimeQualityScore:    32,
		RiskPenaltyScore:      44,
	}

	eligible, failures := evaluateCandidateGate(snapshot)
	if eligible {
		t.Fatal("expected candidate gate to reject weak snapshot")
	}
	for _, want := range []string{"s5_relative_strength", "s6_regime_quality", "s7_risk_penalty"} {
		if !containsString(failures, want) {
			t.Fatalf("expected failure %q in %#v", want, failures)
		}
	}
}

func TestSelectCandidateSnapshotsCapsOutputAndPrefersEligible(t *testing.T) {
	ranked := make([]*marketSnapshot, 0, 24)
	for i := 0; i < 24; i++ {
		failures := []string(nil)
		if i >= 20 {
			failures = []string{"s7_risk_penalty"}
		}
		ranked = append(ranked, &marketSnapshot{
			Symbol:                  normalizePair(string(rune('A' + i))),
			Pair:                    normalizePair(string(rune('A' + i))),
			Score:                   90 - float64(i),
			CandidateEligible:       i < 20,
			CandidateFilterFailures: failures,
		})
	}

	selection := selectCandidateSnapshots(ranked, 70)
	selected := selection.Selected
	if len(selected) != maxSelfhostedCandidateCount {
		t.Fatalf("selected len = %d, want %d", len(selected), maxSelfhostedCandidateCount)
	}
	if selection.ExplorationCount != 0 {
		t.Fatalf("exploration count = %d, want 0", selection.ExplorationCount)
	}
	for _, snapshot := range selected {
		if !candidateEligible(snapshot) {
			t.Fatalf("selected ineligible snapshot: %+v", snapshot)
		}
	}
}

func TestSelectCandidateSnapshotsLowersAdaptiveThresholdForThinEligiblePool(t *testing.T) {
	ranked := make([]*marketSnapshot, 0, 12)
	scores := []float64{75, 74, 73, 72, 71, 69, 68, 67, 66, 65, 64, 64}
	for i, score := range scores {
		ranked = append(ranked, &marketSnapshot{
			Symbol:            normalizePair(string(rune('A' + i))),
			Pair:              normalizePair(string(rune('A' + i))),
			Score:             score,
			CandidateEligible: true,
		})
	}

	selection := selectCandidateSnapshots(ranked, 70)
	if selection.AdaptiveThreshold >= 70 {
		t.Fatalf("adaptive threshold = %v, want below 70", selection.AdaptiveThreshold)
	}
	if selection.AdaptiveThreshold != 64 {
		t.Fatalf("adaptive threshold = %v, want 64", selection.AdaptiveThreshold)
	}
	if len(selection.Selected) != len(scores) {
		t.Fatalf("selected len = %d, want %d", len(selection.Selected), len(scores))
	}
	adaptiveCount := 0
	for _, snapshot := range selection.Selected {
		if selection.Buckets[snapshot.Symbol] == "adaptive" {
			adaptiveCount++
		}
	}
	if adaptiveCount == 0 {
		t.Fatalf("expected adaptive selections, buckets=%#v", selection.Buckets)
	}
}

func TestSelectCandidateSnapshotsAddsBoundedExplorationForSoftNearMisses(t *testing.T) {
	ranked := make([]*marketSnapshot, 0, 14)
	for i := 0; i < 10; i++ {
		ranked = append(ranked, &marketSnapshot{
			Symbol:            normalizePair(string(rune('A' + i))),
			Pair:              normalizePair(string(rune('A' + i))),
			Score:             88 - float64(i),
			Volume24H:         30_000_000 - float64(i*1_000),
			CandidateEligible: true,
		})
	}
	ranked = append(ranked,
		&marketSnapshot{
			Symbol:                  "SOFT1USDT",
			Pair:                    "SOFT1USDT",
			Score:                   84,
			Volume24H:               28_000_000,
			CandidateFilterFailures: []string{"s6_no_short_term_confirmation"},
		},
		&marketSnapshot{
			Symbol:                  "SOFT2USDT",
			Pair:                    "SOFT2USDT",
			Score:                   82,
			Volume24H:               26_000_000,
			CandidateFilterFailures: []string{"s5_relative_strength"},
		},
		&marketSnapshot{
			Symbol:                  "SOFT3USDT",
			Pair:                    "SOFT3USDT",
			Score:                   80,
			Volume24H:               24_000_000,
			CandidateFilterFailures: []string{"s6_regime_quality", "s6_no_short_term_confirmation"},
		},
		&marketSnapshot{
			Symbol:                  "HARD1USDT",
			Pair:                    "HARD1USDT",
			Score:                   86,
			Volume24H:               29_000_000,
			CandidateFilterFailures: []string{"s7_risk_penalty"},
		},
	)

	selection := selectCandidateSnapshots(ranked, 70)
	if selection.ExplorationCount != maxSelfhostedExplorationCount {
		t.Fatalf("exploration count = %d, want %d", selection.ExplorationCount, maxSelfhostedExplorationCount)
	}
	if len(selection.Selected) != 12 {
		t.Fatalf("selected len = %d, want 12", len(selection.Selected))
	}
	if selection.Buckets["SOFT1USDT"] != "exploration" || selection.Buckets["SOFT2USDT"] != "exploration" {
		t.Fatalf("expected SOFT1USDT and SOFT2USDT in exploration bucket, got %#v", selection.Buckets)
	}
	if _, ok := selection.Buckets["HARD1USDT"]; ok {
		t.Fatalf("hard-risk candidate should not be selected, buckets=%#v", selection.Buckets)
	}
}

func TestNormalizeDurationAndSplitDurations(t *testing.T) {
	if got := normalizeDuration(" 4H "); got != "4h" {
		t.Fatalf("normalizeDuration(4H) = %q, want 4h", got)
	}
	if got := normalizeDuration("invalid"); got != "1h" {
		t.Fatalf("normalizeDuration(invalid) = %q, want 1h", got)
	}

	got := splitDurations("4h, invalid,1h,24H,4h")
	want := []string{"4h", "1h", "24h"}
	if len(got) != len(want) {
		t.Fatalf("splitDurations len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitDurations[%d] = %q, want %q (%#v)", i, got[i], want[i], got)
		}
	}
}

func TestNormalizeValidPairRejectsNonTradingSymbols(t *testing.T) {
	if _, ok := normalizeValidPair("币安人生USDT"); ok {
		t.Fatal("expected non-ASCII pair to be rejected")
	}
	if _, ok := normalizeValidPair("BTC-USDT"); ok {
		t.Fatal("expected punctuated pair to be rejected")
	}
	normalized, ok := normalizeValidPair("1000pepe")
	if !ok {
		t.Fatal("expected 1000pepe to be accepted")
	}
	if normalized != "1000PEPEUSDT" {
		t.Fatalf("normalized = %q, want %q", normalized, "1000PEPEUSDT")
	}
}

func TestAggregateUniverseSnapshotsSkipsInvalidSymbols(t *testing.T) {
	aggregated := aggregateUniverseSnapshots([]rawUniverseSnapshot{
		{Symbol: "BTCUSDT", Sources: []string{"binance"}, Price: 10, OpenInterest: 2, Volume24H: 100},
		{Symbol: "币安人生USDT", Sources: []string{"bybit"}, Price: 11, OpenInterest: 3, Volume24H: 120},
	}, 10)
	if len(aggregated) != 1 {
		t.Fatalf("len(aggregated) = %d, want 1", len(aggregated))
	}
	if aggregated[0].Symbol != "BTCUSDT" {
		t.Fatalf("symbol = %q, want BTCUSDT", aggregated[0].Symbol)
	}
}

func TestAuthMiddlewareSupportsQueryAndBearerAndRejectsMissing(t *testing.T) {
	router := NewRouter(NewService(Config{}, nil), "secret-token")

	t.Run("query auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai500/list?auth=secret-token", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("bearer auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai500/list", nil)
		req.Header.Set("Authorization", "Bearer secret-token")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("missing auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai500/list", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

func TestBuildOIRankingCachesSortsByDeltaPercentAndTieBreaker(t *testing.T) {
	caches := buildOIRankingCaches(map[string]*marketSnapshot{
		"AAAUSDT": {
			Symbol:       "AAAUSDT",
			Pair:         "AAAUSDT",
			Price:        1,
			OpenInterest: 100,
			PriceChange:  map[string]float64{"1h": 0.01, "4h": 0, "24h": 0},
			OIDeltas: map[string]oiDelta{
				"1h":  {DeltaPercent: 5, DeltaValue: 50},
				"4h":  {},
				"24h": {},
			},
		},
		"BBBUSDT": {
			Symbol:       "BBBUSDT",
			Pair:         "BBBUSDT",
			Price:        1,
			OpenInterest: 200,
			PriceChange:  map[string]float64{"1h": 0.01, "4h": 0, "24h": 0},
			OIDeltas: map[string]oiDelta{
				"1h":  {DeltaPercent: 5, DeltaValue: 100},
				"4h":  {},
				"24h": {},
			},
		},
		"CCCUSDT": {
			Symbol:       "CCCUSDT",
			Pair:         "CCCUSDT",
			Price:        1,
			OpenInterest: 50,
			PriceChange:  map[string]float64{"1h": -0.02, "4h": 0, "24h": 0},
			OIDeltas: map[string]oiDelta{
				"1h":  {DeltaPercent: -10, DeltaValue: -25},
				"4h":  {},
				"24h": {},
			},
		},
	})

	top := caches["1h"].Top
	low := caches["1h"].Low
	if len(top) != 3 || len(low) != 3 {
		t.Fatalf("unexpected ranking sizes: top=%d low=%d", len(top), len(low))
	}
	if top[0].Symbol != "BBBUSDT" {
		t.Fatalf("top[0].Symbol = %q, want BBBUSDT", top[0].Symbol)
	}
	if top[0].Rank != 1 || top[1].Rank != 2 || top[2].Rank != 3 {
		t.Fatalf("unexpected top ranks: %+v", top)
	}
	if low[0].Symbol != "CCCUSDT" {
		t.Fatalf("low[0].Symbol = %q, want CCCUSDT", low[0].Symbol)
	}
}

func TestNormalizeUniverseExchangesDefaultsAndDeduplicates(t *testing.T) {
	got := normalizeUniverseExchanges("bybit, binance, invalid, hyperliquid, bybit")
	want := []string{"bybit", "binance", "hyperliquid"}
	if len(got) != len(want) {
		t.Fatalf("len(normalizeUniverseExchanges) = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeUniverseExchanges[%d] = %q, want %q (%#v)", i, got[i], want[i], got)
		}
	}

	defaulted := normalizeUniverseExchanges("")
	if len(defaulted) != 3 {
		t.Fatalf("default exchange list len = %d, want 3 (%#v)", len(defaulted), defaulted)
	}
}

func TestAggregateUniverseSnapshotsMergesAcrossSources(t *testing.T) {
	snapshots := aggregateUniverseSnapshots([]rawUniverseSnapshot{
		{
			Symbol:        "BTCUSDT",
			Sources:       []string{"binance"},
			Price:         100,
			PrevDayPrice:  95,
			RefPrice1H:    99,
			OpenInterest:  10,
			Volume24H:     1000,
			VolumeBase24H: 10,
			Funding:       0.001,
			Premium:       0.002,
			SpreadBps:     4,
			IsAvailable:   true,
		},
		{
			Symbol:        "BTCUSDT",
			Sources:       []string{"bybit"},
			Price:         110,
			PrevDayPrice:  100,
			RefPrice1H:    108,
			OpenInterest:  20,
			Volume24H:     3000,
			VolumeBase24H: 30,
			Funding:       0.003,
			Premium:       0.004,
			SpreadBps:     6,
			IsAvailable:   true,
		},
	}, 10)

	if len(snapshots) != 1 {
		t.Fatalf("aggregated snapshot len = %d, want 1", len(snapshots))
	}
	item := snapshots[0]
	if item.Symbol != "BTCUSDT" {
		t.Fatalf("symbol = %q, want BTCUSDT", item.Symbol)
	}
	if item.Volume24H != 4000 {
		t.Fatalf("volume24h = %v, want 4000", item.Volume24H)
	}
	if item.OpenInterest != 30 {
		t.Fatalf("openInterest = %v, want 30", item.OpenInterest)
	}
	if len(item.Sources) != 2 || item.Sources[0] != "binance" || item.Sources[1] != "bybit" {
		t.Fatalf("sources = %#v, want [binance bybit]", item.Sources)
	}
	if item.Price <= 107 || item.Price >= 108 {
		t.Fatalf("weighted price = %v, want around 107.5", item.Price)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
