package decision

import (
	"encoding/json"
	"testing"
	"time"

	"nofx/internal/snapshot"
	"nofx/market"
	"nofx/pkg/types"
)

func TestAssembleLivePayload_MetaIsConsolidated(t *testing.T) {
	ctx := &Context{
		PayloadVersion: "3.1",
		CurrentTime:    "2025-11-16 04:57:11",
		RuntimeMinutes: 1788,
		CallCount:      597,
		Account: AccountInfo{
			TotalEquity:      79.231,
			AvailableBalance: 79.231,
			TotalPnLPct:      -2.898,
			MarginUsedPct:    0,
			PositionCount:    0,
		},
	}

	payload, _, err := assembleLivePayload(ctx)
	if err != nil {
		t.Fatalf("assembleLivePayload failed: %v", err)
	}
	if payload.Meta == nil {
		t.Fatalf("meta should not be nil")
	}

	raw, err := json.Marshal(payload.Meta)
	if err != nil {
		t.Fatalf("marshal meta failed: %v", err)
	}
	var meta map[string]interface{}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal meta failed: %v", err)
	}

	if _, ok := meta["summary"]; !ok {
		t.Fatalf("meta.summary should exist")
	}
	if _, ok := meta["account"]; ok {
		t.Fatalf("meta.account should be removed to avoid redundancy")
	}
	if _, ok := meta["positions"]; ok {
		t.Fatalf("meta.positions should be removed to avoid redundancy")
	}
	if _, ok := meta["latest"]; ok {
		t.Fatalf("meta.latest should be removed to avoid candidate mismatch")
	}
}

func TestAssembleLivePayload_MetaIncludesSharpeWhenAvailable(t *testing.T) {
	const sharpe = -0.1344474449
	ctx := &Context{
		PayloadVersion: "3.1",
		CurrentTime:    "2025-11-16 04:57:11",
		RuntimeMinutes: 1788,
		CallCount:      597,
		Account: AccountInfo{
			TotalEquity:      79.231,
			AvailableBalance: 79.231,
			TotalPnLPct:      -2.898,
			MarginUsedPct:    0,
			PositionCount:    0,
		},
		Performance: map[string]interface{}{
			"sharpe_ratio": sharpe,
		},
	}

	payload, _, err := assembleLivePayload(ctx)
	if err != nil {
		t.Fatalf("assembleLivePayload failed: %v", err)
	}
	if payload.Meta == nil || payload.Meta.SharpeRatio == nil {
		t.Fatalf("meta.sharpe_ratio should exist when performance provides it")
	}
	if *payload.Meta.SharpeRatio != sharpe {
		t.Fatalf("unexpected sharpe ratio: got %v want %v", *payload.Meta.SharpeRatio, sharpe)
	}
}

func TestBuildCandidatePayload_IncludesAllFSignalsWhenSnapshotPresent(t *testing.T) {
	now := time.Now().UTC()
	symbol := "BTCUSDT"

	data := &market.Data{
		Symbol:       symbol,
		CollectedAt:  now,
		CurrentPrice: 100.0,
		OpenInterest: &market.OIData{Latest: 12345},
		FeatureStats: map[string]market.FeatureStat{
			market.FeatureKeyF4: {Coverage: 1, UpdatedAt: now},
			market.FeatureKeyF5: {Coverage: 1, UpdatedAt: now},
			market.FeatureKeyF6: {Coverage: 1, UpdatedAt: now},
			market.FeatureKeyF7: {Coverage: 1, UpdatedAt: now},
		},
		Snapshot: &snapshot.Snapshot{
			Symbol: symbol,
			Features: snapshot.SnapshotContent{
				Derivs: &types.DerivsFeatures{
					ConfidenceCVD3m:     floatPtr(0.8),
					CVDNotionalZ3mShort: floatPtr(1.2),
					ConfidenceLiq3m:     floatPtr(0.7),
					DistUpAtr3m:         floatPtr(0.5),
					ConfidenceAVWAP3m:   floatPtr(0.9),
					AVWAPBias3m:         strPtr("prefer_longs"),
					ConfidenceVol3m:     floatPtr(0.85),
					BBW3m:               floatPtr(0.01),
					ConfidenceCVD15m:    floatPtr(0.78),
					ConfidenceLiq15m:    floatPtr(0.72),
					DistUpAtr15m:        floatPtr(0.6),
					DistDnAtr15m:        floatPtr(0.4),
				},
			},
		},
	}

	ctx := &Context{
		MarketDataMap: map[string]*market.Data{
			symbol: data,
		},
		CandidateCoins: []CandidateCoin{{Symbol: symbol}},
	}
	diag := &payloadDiagnostics{FeaturePass: make(map[string][]string)}
	cand, ok := buildCandidatePayload(ctx, symbol, diag)
	if !ok {
		t.Fatalf("buildCandidatePayload should return candidate")
	}
	if cand.Features == nil {
		t.Fatalf("candidate features should not be nil")
	}
	if cand.Features.F4 == nil || cand.Features.F5 == nil || cand.Features.F6 == nil || cand.Features.F7 == nil {
		t.Fatalf("all f4-f7 should be present when snapshot provides data")
	}
	if cand.Features.F4.QoS.Coverage < 0.95 || cand.Features.F5.QoS.Coverage < 0.95 || cand.Features.F6.QoS.Coverage < 0.95 || cand.Features.F7.QoS.Coverage < 0.95 {
		t.Fatalf("all f4-f7 qos coverage should pass gate")
	}
	if len(diag.FeaturePass[symbol]) != 4 {
		t.Fatalf("expected 4 passed feature gates, got %d", len(diag.FeaturePass[symbol]))
	}
}

func strPtr(v string) *string {
	return &v
}
