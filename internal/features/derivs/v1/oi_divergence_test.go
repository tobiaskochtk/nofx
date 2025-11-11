package derivsv1

import "testing"

type mockOIHistory struct {
    oi     []float64
    price  []float64
    status string
}

func (m *mockOIHistory) GetOHLC(symbol string, hours int) ([]float64, []float64, error) {
    return append([]float64{}, m.oi...), append([]float64{}, m.price...), nil
}

func (m *mockOIHistory) SourceStatus(symbol string) string {
    return m.status
}

func TestComputeOIFeaturesConfirming(t *testing.T) {
    history := &mockOIHistory{
        oi:     []float64{100, 110, 120, 150},
        price:  []float64{10, 10.5, 11, 12},
        status: "binanceusdm:ok",
    }
    cfg := OIFeatureConfig{
        Symbol:          "TESTUSDT",
        History:         history,
        WindowHours:     48,
        CorrWindowHours: 3,
        Alpha:           0.2,
        MinSamples:      2,
    }
    feat, err := ComputeOIFeatures(cfg)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    state, ok := feat["oi_price_div"].(*string)
    if !ok || state == nil || *state != "confirming" {
        t.Fatalf("expected confirming divergence, got %#v", feat["oi_price_div"])
    }
    if feat["oi_source_status"].(string) != "binanceusdm:ok" {
        t.Fatalf("unexpected source status: %#v", feat["oi_source_status"])
    }
}
