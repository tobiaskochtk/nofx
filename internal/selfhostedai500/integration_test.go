package selfhostedai500

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistsSnapshotsAndScoreState(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "selfhosted-ai500.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore error: %v", err)
	}

	now := time.Unix(1_700_000_000, 0).UTC()
	oldSnapshot := integrationSnapshot("BTCUSDT", now.Add(-2*time.Hour), 100, 1_000, 3_000_000, 78, 1)
	currentSnapshot := integrationSnapshot("BTCUSDT", now, 110, 1_250, 4_000_000, 82, 1)

	if err := store.InsertSnapshot(oldSnapshot); err != nil {
		t.Fatalf("InsertSnapshot old error: %v", err)
	}
	if err := store.InsertSnapshot(currentSnapshot); err != nil {
		t.Fatalf("InsertSnapshot current error: %v", err)
	}
	if err := store.SaveScoreState(currentSnapshot); err != nil {
		t.Fatalf("SaveScoreState error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close store error: %v", err)
	}

	reopened, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("reopen store error: %v", err)
	}
	defer reopened.Close()

	point, err := reopened.LatestSnapshotBefore("BTCUSDT", now.Add(-90*time.Minute))
	if err != nil {
		t.Fatalf("LatestSnapshotBefore error: %v", err)
	}
	if point == nil || point.Price != 100 {
		t.Fatalf("persisted snapshot lookup = %+v, want price 100", point)
	}

	state, err := reopened.LoadScoreState()
	if err != nil {
		t.Fatalf("LoadScoreState error: %v", err)
	}
	record, ok := state["BTCUSDT"]
	if !ok {
		t.Fatal("expected BTCUSDT score state")
	}
	if record.LastScore != currentSnapshot.LastScore {
		t.Fatalf("last_score = %v, want %v", record.LastScore, currentSnapshot.LastScore)
	}

	if err := reopened.PruneSnapshots(now.Add(-90 * time.Minute)); err != nil {
		t.Fatalf("PruneSnapshots error: %v", err)
	}
	prunedPoint, err := reopened.LatestSnapshotBefore("BTCUSDT", now.Add(-90*time.Minute))
	if err != nil {
		t.Fatalf("LatestSnapshotBefore after prune error: %v", err)
	}
	if prunedPoint != nil {
		t.Fatalf("expected historical snapshot to be pruned, got %+v", prunedPoint)
	}
}

func TestRouterEndpointContractsWithSQLiteBackedService(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "selfhosted-ai500.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore error: %v", err)
	}
	defer store.Close()

	now := time.Unix(1_700_000_000, 0).UTC()
	btc := integrationSnapshot("BTCUSDT", now, 110, 1_250, 4_000_000, 82, 1)
	eth := integrationSnapshot("ETHUSDT", now, 90, 900, 3_500_000, 75, 2)

	for _, snapshot := range []*marketSnapshot{btc, eth} {
		if err := store.InsertSnapshot(snapshot); err != nil {
			t.Fatalf("InsertSnapshot error: %v", err)
		}
		if err := store.SaveScoreState(snapshot); err != nil {
			t.Fatalf("SaveScoreState error: %v", err)
		}
	}

	service := NewService(Config{ScoreThreshold: 70}, store)
	service.scoreState = map[string]scoreStateRecord{
		"BTCUSDT": {
			Symbol:        "BTCUSDT",
			StartTime:     btc.StartTime,
			StartPrice:    btc.StartPrice,
			LastScore:     btc.LastScore,
			MaxScore:      btc.MaxScore,
			MaxPrice:      btc.MaxPrice,
			LastUpdatedAt: btc.UpdatedAt.Unix(),
		},
		"ETHUSDT": {
			Symbol:        "ETHUSDT",
			StartTime:     eth.StartTime,
			StartPrice:    eth.StartPrice,
			LastScore:     eth.LastScore,
			MaxScore:      eth.MaxScore,
			MaxPrice:      eth.MaxPrice,
			LastUpdatedAt: eth.UpdatedAt.Unix(),
		},
	}
	service.snapshots = map[string]*marketSnapshot{
		"BTCUSDT": btc,
		"ETHUSDT": eth,
	}
	service.oiRankings = buildOIRankingCaches(service.snapshots)
	service.priceRankings = buildPriceRankingCaches(service.snapshots)
	service.netflowRankings = buildNetflowRankingCaches(service.snapshots)
	service.lastRefresh = now
	service.ready = true

	router := NewRouter(service, "test-token")

	t.Run("ai500 list contract", func(t *testing.T) {
		payload := performJSONRequest(t, router, "/api/ai500/list?auth=test-token")
		if payload["success"] != true {
			t.Fatalf("success = %v, want true", payload["success"])
		}
		data := payload["data"].(map[string]any)
		coins := data["coins"].([]any)
		if len(coins) != 2 {
			t.Fatalf("coin count = %d, want 2", len(coins))
		}
		firstCoin := coins[0].(map[string]any)
		if _, ok := firstCoin["selection_bucket"]; !ok {
			t.Fatalf("expected selection_bucket in ai500 list payload: %+v", firstCoin)
		}
	})

	t.Run("coin detail contract", func(t *testing.T) {
		payload := performJSONRequest(t, router, "/api/coin/BTC?include=netflow,oi,price,ai500&auth=test-token")
		if payload["success"] != true {
			t.Fatalf("success = %v, want true", payload["success"])
		}
		if int(payload["code"].(float64)) != 0 {
			t.Fatalf("code = %v, want 0", payload["code"])
		}
		data := payload["data"].(map[string]any)
		if _, ok := data["ai500"]; !ok {
			t.Fatalf("expected ai500 field in coin payload: %+v", data)
		}
		if _, ok := data["oi"]; !ok {
			t.Fatalf("expected oi field in coin payload: %+v", data)
		}
	})

	t.Run("oi ranking contract", func(t *testing.T) {
		payload := performJSONRequest(t, router, "/api/oi/top-ranking?duration=1h&limit=1&auth=test-token")
		if payload["success"] != true {
			t.Fatalf("success = %v, want true", payload["success"])
		}
		data := payload["data"].(map[string]any)
		positions := data["positions"].([]any)
		if len(positions) != 1 {
			t.Fatalf("positions len = %d, want 1", len(positions))
		}
	})

	t.Run("price ranking contract", func(t *testing.T) {
		payload := performJSONRequest(t, router, "/api/price/ranking?duration=1h,4h&limit=1&auth=test-token")
		if payload["success"] != true {
			t.Fatalf("success = %v, want true", payload["success"])
		}
		data := payload["data"].(map[string]any)
		durations := data["durations"].([]any)
		if len(durations) != 2 {
			t.Fatalf("durations len = %d, want 2", len(durations))
		}
		rankingData := data["data"].(map[string]any)
		if _, ok := rankingData["1h"]; !ok {
			t.Fatalf("expected 1h ranking bucket: %+v", rankingData)
		}
		if _, ok := rankingData["4h"]; !ok {
			t.Fatalf("expected 4h ranking bucket: %+v", rankingData)
		}
	})

	t.Run("debug score contract", func(t *testing.T) {
		payload := performJSONRequest(t, router, "/api/debug/score/BTC?auth=test-token")
		if payload["success"] != true {
			t.Fatalf("success = %v, want true", payload["success"])
		}
		data := payload["data"].(map[string]any)
		if _, ok := data["score_components"]; !ok {
			t.Fatalf("expected score_components in debug payload: %+v", data)
		}
		if _, ok := data["adaptive_threshold"]; !ok {
			t.Fatalf("expected adaptive_threshold in debug payload: %+v", data)
		}
	})
}

func integrationSnapshot(symbol string, updatedAt time.Time, price, oi, volume24H, score float64, rank int) *marketSnapshot {
	return &marketSnapshot{
		Symbol:       symbol,
		Pair:         symbol,
		Price:        price,
		PrevDayPrice: price * 0.95,
		OpenInterest: oi,
		Volume24H:    volume24H,
		Funding:      0.0001,
		Premium:      0.0002,
		SpreadBps:    5,
		UpdatedAt:    updatedAt,
		PriceChange: map[string]float64{
			"1h":  0.01,
			"4h":  0.03,
			"24h": 0.05,
		},
		OIDeltas: map[string]oiDelta{
			"1h":  {Delta: oi * 0.05, DeltaValue: price * oi * 0.05, DeltaPercent: 5},
			"4h":  {Delta: oi * 0.08, DeltaValue: price * oi * 0.08, DeltaPercent: 8},
			"24h": {Delta: oi * 0.12, DeltaValue: price * oi * 0.12, DeltaPercent: 12},
		},
		InstitutionFutureFlow: map[string]float64{
			"1h":  50_000,
			"4h":  80_000,
			"24h": 120_000,
		},
		PersonalFutureFlow: map[string]float64{
			"1h":  -10_000,
			"4h":  -5_000,
			"24h": 15_000,
		},
		InstitutionSpotFlow: map[string]float64{
			"1h":  10_000,
			"4h":  15_000,
			"24h": 20_000,
		},
		PersonalSpotFlow: map[string]float64{
			"1h":  2_000,
			"4h":  3_000,
			"24h": 5_000,
		},
		Score:             score,
		StartTime:         updatedAt.Add(-4 * time.Hour).Unix(),
		StartPrice:        price * 0.92,
		LastScore:         score - 2,
		MaxScore:          score + 3,
		MaxPrice:          price * 1.04,
		IncreasePercent:   6.5,
		Rank:              rank,
		ReasonCodes:       []string{"trend_aligned", "daily_strength"},
		ScoreComponents:   scoreComponents{Liquidity: 22, OI: 18, Momentum: 28, Flow: 12, Adjust: 5, Total: score},
		StartRegimeActive: true,
	}
}

func performJSONRequest(t *testing.T, router http.Handler, path string) map[string]any {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d, body=%s", path, rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("GET %s unmarshal error: %v", path, err)
	}
	return payload
}
