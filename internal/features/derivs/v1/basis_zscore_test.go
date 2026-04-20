package derivsv1

import (
    "testing"
    "time"
)

type mockBasisHistory struct {
    latest  map[string]BasisPoint
    history map[string][]BasisPoint
    status  string
}

func (m *mockBasisHistory) Latest(symbol string) (map[string]BasisPoint, error) {
    return m.latest, nil
}

func (m *mockBasisHistory) History(symbol string, hours int) (map[string][]BasisPoint, error) {
    return m.history, nil
}

func (m *mockBasisHistory) SourceStatus(symbol string) string {
    return m.status
}

func TestComputeBasisFeatures(t *testing.T) {
    now := time.Now()
    history := &mockBasisHistory{
        latest: map[string]BasisPoint{
            "binanceusdm": {Timestamp: now, Mark: 101, Index: 100},
            "bybit":       {Timestamp: now, Mark: 99, Index: 100},
        },
        history: map[string][]BasisPoint{
            "binanceusdm": {
                {Timestamp: now.Add(-2 * time.Hour), Mark: 100, Index: 100},
                {Timestamp: now.Add(-1 * time.Hour), Mark: 101, Index: 100},
            },
            "bybit": {
                {Timestamp: now.Add(-2 * time.Hour), Mark: 100, Index: 100},
                {Timestamp: now.Add(-1 * time.Hour), Mark: 99, Index: 100},
            },
        },
        status: "binanceusdm:ok|bybit:ok",
    }
    feat, err := ComputeBasisFeatures(BasisFeatureConfig{
        Symbol:      "TESTUSDT",
        History:     history,
        WindowHours: 48,
        Alpha:       0.3,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if feat["basis_pct"] == nil {
        t.Fatalf("expected basis pct to be set")
    }
    if feat["basis_z_14d"] == nil {
        t.Fatalf("expected basis z to be set")
    }
}
