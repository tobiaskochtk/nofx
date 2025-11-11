package rolling

import "testing"

func TestEWStatsZeroVariance(t *testing.T) {
    stats := &EWStats{Alpha: 0.2}
    stats.Update(42)
    if z := stats.Z(42); z != 0 {
        t.Fatalf("expected z=0 when variance is zero, got %v", z)
    }
}
