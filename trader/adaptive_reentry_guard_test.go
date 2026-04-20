package trader

import (
	"testing"
	"time"

	"nofx/store"
)

func TestBuildRecentExecutionRegime_ClassifiesWeakChurn(t *testing.T) {
	trades := []store.RecentTrade{
		{Symbol: "ENAUSDT", PnLPct: -1.8},
		{Symbol: "RAVEUSDT", PnLPct: -0.9},
		{Symbol: "EDGEUSDT", PnLPct: -0.4},
		{Symbol: "AAVEUSDT", PnLPct: 0.3},
		{Symbol: "ORDIUSDT", PnLPct: -0.2},
	}

	regime := buildRecentExecutionRegime(trades)
	if regime == nil {
		t.Fatal("buildRecentExecutionRegime returned nil")
	}
	if regime.FollowThroughState != "weak" {
		t.Fatalf("follow-through state = %q, want weak", regime.FollowThroughState)
	}
	if regime.ChurnRisk != "high" {
		t.Fatalf("churn risk = %q, want high", regime.ChurnRisk)
	}
	if regime.ConsecutiveLosses != 3 {
		t.Fatalf("consecutive losses = %d, want 3", regime.ConsecutiveLosses)
	}
}

func TestAdaptiveReentryGuardReason_BlocksRecentSameSymbolLossesOnlyInWeakRegime(t *testing.T) {
	now := time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC)
	trades := []store.RecentTrade{
		{Symbol: "RAVEUSDT", PnLPct: -1.7, ExitTime: now.Add(-40 * time.Minute).Unix()},
		{Symbol: "RAVEUSDT", PnLPct: -0.8, ExitTime: now.Add(-2 * time.Hour).Unix()},
		{Symbol: "EDGEUSDT", PnLPct: -0.5, ExitTime: now.Add(-3 * time.Hour).Unix()},
		{Symbol: "ENAUSDT", PnLPct: 0.2, ExitTime: now.Add(-4 * time.Hour).Unix()},
		{Symbol: "ORDIUSDT", PnLPct: -0.4, ExitTime: now.Add(-5 * time.Hour).Unix()},
	}

	reason := adaptiveReentryGuardReason("RAVEUSDT", trades, now)
	if reason == "" {
		t.Fatal("expected adaptive re-entry guard to block RAVEUSDT")
	}

	reason = adaptiveReentryGuardReason("AAVEUSDT", trades, now)
	if reason != "" {
		t.Fatalf("unexpected block for fresh symbol: %s", reason)
	}
}

func TestAdaptiveReentryGuardReason_AllowsWinnerReentryInStrongTape(t *testing.T) {
	now := time.Date(2026, 4, 16, 15, 0, 0, 0, time.UTC)
	trades := []store.RecentTrade{
		{Symbol: "RAVEUSDT", PnLPct: 4.8, ExitTime: now.Add(-35 * time.Minute).Unix()},
		{Symbol: "ORDIUSDT", PnLPct: 2.1, ExitTime: now.Add(-90 * time.Minute).Unix()},
		{Symbol: "PIPPINUSDT", PnLPct: 6.4, ExitTime: now.Add(-2 * time.Hour).Unix()},
		{Symbol: "FARTCOINUSDT", PnLPct: 3.7, ExitTime: now.Add(-3 * time.Hour).Unix()},
		{Symbol: "EDGEUSDT", PnLPct: -0.4, ExitTime: now.Add(-4 * time.Hour).Unix()},
	}

	if reason := adaptiveReentryGuardReason("RAVEUSDT", trades, now); reason != "" {
		t.Fatalf("winner re-entry should stay allowed in strong tape, got: %s", reason)
	}
}
