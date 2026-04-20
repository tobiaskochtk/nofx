package derivsv1

import "testing"

type mockFundingHistory struct {
    latest  map[string]float64
    history map[string][]float64
    status  string
}

func (m *mockFundingHistory) LatestRates(symbol string) (map[string]float64, error) {
    return m.latest, nil
}

func (m *mockFundingHistory) History(symbol string, intervals int) (map[string][]float64, error) {
    return m.history, nil
}

func (m *mockFundingHistory) SourceStatus(symbol string) string {
    return m.status
}

func TestComputeFundingFeatures(t *testing.T) {
    history := &mockFundingHistory{
        latest: map[string]float64{
            "binanceusdm": 0.0001,
            "bybit":      -0.00005,
        },
        history: map[string][]float64{
            "binanceusdm": {0.00008, 0.00009, 0.0001},
            "bybit":      {-0.00004, -0.00005, -0.00005},
        },
        status: "binanceusdm:ok|bybit:ok",
    }
    feat, err := ComputeFundingFeatures(FundingFeatureConfig{
        Symbol:     "TESTUSDT",
        History:    history,
        WindowSize: 3,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if feat["funding_source_status"].(string) == "" {
        t.Fatalf("expected source status to be set")
    }
    if feat["funding_latest_bps"] == nil {
        t.Fatalf("expected latest bps to be present")
    }
    if feat["funding_dispersion_bps"] == nil {
        t.Fatalf("expected dispersion bps to be present")
    }
}
