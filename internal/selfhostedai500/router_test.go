package selfhostedai500

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCoinNotFoundReturnsStructuredPayload(t *testing.T) {
	router := NewRouter(NewService(Config{}, nil), "")
	req := httptest.NewRequest(http.MethodGet, "/api/coin/abc?include=foo,ai500", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if int(payload["code"].(float64)) != http.StatusNotFound {
		t.Fatalf("code = %v, want %d", payload["code"], http.StatusNotFound)
	}
	data := payload["data"].(map[string]any)
	if data["symbol"] != "ABCUSDT" {
		t.Fatalf("symbol = %v, want ABCUSDT", data["symbol"])
	}
}

func TestAI500NotFoundReturnsStructuredPayload(t *testing.T) {
	router := NewRouter(NewService(Config{}, nil), "")
	req := httptest.NewRequest(http.MethodGet, "/api/ai500/abc", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if int(payload["code"].(float64)) != http.StatusNotFound {
		t.Fatalf("code = %v, want %d", payload["code"], http.StatusNotFound)
	}
	data := payload["data"].(map[string]any)
	if data["symbol"] != "ABCUSDT" {
		t.Fatalf("symbol = %v, want ABCUSDT", data["symbol"])
	}
}

func TestDebugScoreReturnsStructuredPayload(t *testing.T) {
	service := &Service{
		cfg: Config{ScoreThreshold: 70},
		snapshots: map[string]*marketSnapshot{
			"BTCUSDT": {
				Symbol:                "BTCUSDT",
				Pair:                  "BTCUSDT",
				Price:                 100,
				Volume24H:             1_000_000,
				OpenInterest:          25_000,
				Rank:                  1,
				Score:                 80,
				StartTime:             time.Unix(1_700_000_000, 0).Add(-time.Hour).Unix(),
				StartPrice:            95,
				PriceChange:           map[string]float64{"1h": 0.01, "4h": 0.02, "24h": 0.03},
				OIDeltas:              map[string]oiDelta{"1h": {Delta: 1, DeltaValue: 100, DeltaPercent: 2}, "4h": {}, "24h": {}},
				InstitutionFutureFlow: map[string]float64{"1h": 10, "4h": 11, "24h": 12},
				PersonalFutureFlow:    map[string]float64{"1h": 1, "4h": 2, "24h": 3},
				InstitutionSpotFlow:   map[string]float64{"1h": 4, "4h": 5, "24h": 6},
				PersonalSpotFlow:      map[string]float64{"1h": 1, "4h": 1, "24h": 1},
				ScoreComponents:       scoreComponents{Liquidity: 20, OI: 15, Momentum: 30, Flow: 10, Adjust: 5, Total: 80},
				StartRegimeActive:     true,
			},
		},
		lastRefresh: time.Unix(1_700_000_000, 0),
	}
	router := NewRouter(service, "")
	req := httptest.NewRequest(http.MethodGet, "/api/debug/score/btc", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	data := payload["data"].(map[string]any)
	if data["symbol"] != "BTCUSDT" {
		t.Fatalf("symbol = %v, want BTCUSDT", data["symbol"])
	}
}

func TestDebugRankingsRejectsUnsupportedKind(t *testing.T) {
	router := NewRouter(NewService(Config{}, nil), "")
	req := httptest.NewRequest(http.MethodGet, "/api/debug/rankings?kind=unsupported", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
