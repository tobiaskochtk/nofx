package decision

import (
	"testing"
	"time"

	"nofx/internal/snapshot"
	"nofx/market"
	"nofx/pkg/types"
)

func TestAssembleLivePayload_UsesV4Contract(t *testing.T) {
	ctx := &Context{
		PayloadVersion: PayloadSchemaVersion,
		CurrentTime:    "2025-11-16 04:57:11",
		RuntimeMinutes: 1788,
		CallCount:      597,
		Account: AccountInfo{
			TotalEquity:      79.231,
			AvailableBalance: 79.231,
			UnrealizedPnL:    -1.234,
			MarginUsedPct:    2.4,
			PositionCount:    0,
		},
	}

	payload, _, err := assembleLivePayload(ctx)
	if err != nil {
		t.Fatalf("assembleLivePayload failed: %v", err)
	}
	if payload.Schema != PayloadSchemaVersion {
		t.Fatalf("unexpected schema: got %s want %s", payload.Schema, PayloadSchemaVersion)
	}
	if payload.TimestampUTC == "" {
		t.Fatalf("ts_utc should be populated")
	}
	if payload.Run.Cycle != 597 {
		t.Fatalf("unexpected cycle: %d", payload.Run.Cycle)
	}
	if payload.Policy.MinScore != 0.70 {
		t.Fatalf("policy min_score should be 0.70")
	}
	if !payload.Policy.ObeySideBias {
		t.Fatalf("policy obey_side_bias should be true")
	}
	if !payload.Policy.ExitOnlyIfInPos {
		t.Fatalf("policy exit_only_if_in_positions should be true")
	}
	if !payload.Policy.EntryRule.MustEnterIfAnyEligible {
		t.Fatalf("policy entry_rule.must_enter_if_any_eligible should be true")
	}
	if len(payload.Policy.EntryRule.SelectBestBy) != 3 {
		t.Fatalf("policy entry_rule.select_best_by should include 3 sort keys")
	}
	if payload.OutputContract.Format != "json_only" {
		t.Fatalf("output_contract.format should be json_only")
	}
	if payload.OutputContract.DecisionMode != "single_best" {
		t.Fatalf("decision_mode should be single_best")
	}
	if payload.Req.RequiredTF != defaultContextTF {
		t.Fatalf("required_tf should be %s", defaultContextTF)
	}
	if payload.Req.MaxStaleS != requiredMaxStaleSeconds {
		t.Fatalf("max_stale_s should be %d", requiredMaxStaleSeconds)
	}
	if len(payload.Req.RequiredCandidateFields) == 0 || len(payload.Req.RequiredCandidateCtxFields) == 0 {
		t.Fatalf("required candidate field lists should be populated")
	}
	if len(payload.Req.NullableOKAny) == 0 {
		t.Fatalf("nullable_ok_any should be populated")
	}
	if containsString(payload.OutputContract.DecisionFields, "sl_px") || containsString(payload.OutputContract.DecisionFields, "tp_px") {
		t.Fatalf("decision_fields must not include sl_px/tp_px")
	}
	if payload.OutputContract.FieldSources["ts_utc"] != "ts_utc" || payload.OutputContract.FieldSources["cycle"] != "run.cycle" {
		t.Fatalf("field_sources should map ts_utc and cycle deterministically")
	}
	if !payload.OutputContract.Constraints.SideMustMatchCandidateBias {
		t.Fatalf("side_must_match_candidate_bias should be true")
	}
	if !payload.OutputContract.Constraints.ExitRequiresOpenPosition {
		t.Fatalf("exit_requires_open_position should be true")
	}
	if !payload.OutputContract.Constraints.DecisionsMustBeArray {
		t.Fatalf("decisions_must_be_array should be true")
	}
	if payload.OutputContract.Constraints.DecisionsLenMin != 0 || payload.OutputContract.Constraints.DecisionsLenMax != 1 {
		t.Fatalf("decisions length constraints should be [0,1]")
	}
	if payload.Positions == nil || payload.Candidates == nil {
		t.Fatalf("positions/candidates should always be explicit arrays")
	}
}

func TestBuildCandidatePayload_IncludesSemanticFeaturesAndQoS(t *testing.T) {
	now := time.Now().UTC()
	symbol := "BTCUSDT"

	data := &market.Data{
		Symbol:       symbol,
		CollectedAt:  now,
		CurrentPrice: 100.0,
		CurrentEMA20: 99.8,
		CurrentMACD:  0.25,
		CurrentRSI7:  56,
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
					OIDelta1hPct:        floatPtr(0.0125),
					FundingLatestBps:    floatPtr(2.3),
					BasisPct:            floatPtr(0.006),
					ConfidenceCVD3m:     floatPtr(0.8),
					CVDNotionalZ3mShort: floatPtr(1.2),
					ConfidenceLiq3m:     floatPtr(0.7),
					DistUpAtr3m:         floatPtr(0.5),
					DistDnAtr3m:         floatPtr(0.4),
					ConfidenceAVWAP3m:   floatPtr(0.9),
					AVWAPBias3m:         strPtr("prefer_longs"),
					ConfidenceVol3m:     floatPtr(0.85),
					BBW3m:               floatPtr(0.01),
					RvRatio3m:           floatPtr(1.2),
					OISourceStatus:      "binanceusdm:ok",
					FundingSourceStatus: "binanceusdm:ok",
					BasisSourceStatus:   "binanceusdm:ok",
					PreferDirection3m:   strPtr("prefer_longs"),
					AVWAPUpName3m:       strPtr("swing_high"),
					AVWAPUpDistAtr3m:    floatPtr(0.8),
					VolRegime3m:         strPtr("expansion"),
					OIPriceDiv:          strPtr("confirming"),
				},
			},
		},
	}

	ctx := &Context{
		MarketDataMap:  map[string]*market.Data{symbol: data},
		CandidateCoins: []CandidateCoin{{Symbol: symbol, Sources: []string{"ai500", "oi_top"}}},
		PayloadVersion: PayloadSchemaVersion,
		CurrentTime:    "2025-11-16 04:57:11",
		RuntimeMinutes: 1,
		CallCount:      1,
		Account:        AccountInfo{TotalEquity: 100, AvailableBalance: 100},
	}
	diag := &payloadDiagnostics{FeaturePass: make(map[string][]string)}
	cand, ok := buildCandidatePayload(ctx, CandidateCoin{Symbol: symbol}, diag, reqPayload{RequiredTF: defaultContextTF}, defaultContextPriceType)
	if !ok {
		t.Fatalf("buildCandidatePayload should return candidate")
	}
	if cand.Features.Orderflow == nil || cand.Features.Risk == nil || cand.Features.Levels == nil || cand.Features.Volatility == nil {
		t.Fatalf("semantic feat blocks should be present when QoS passes")
	}
	if cand.Score <= 0 || cand.Score > 1 {
		t.Fatalf("score should be in (0,1], got %.3f", cand.Score)
	}
	if cand.Confidence < 0 || cand.Confidence > 1 {
		t.Fatalf("confidence should be in [0,1], got %.3f", cand.Confidence)
	}
	if cand.QoS.Coverage <= 0.9 {
		t.Fatalf("qos coverage should be high, got %.3f", cand.QoS.Coverage)
	}
	if len(diag.FeaturePass[symbol]) != 4 {
		t.Fatalf("expected 4 passed feature gates, got %d", len(diag.FeaturePass[symbol]))
	}
}

func TestBuildContextBlock_IncludesBaseDerivsSignals(t *testing.T) {
	data := &market.Data{
		CurrentEMA20: 100,
		CurrentMACD:  0.3,
		CurrentRSI7:  55,
		OpenInterest: &market.OIData{Latest: 10_000},
		Snapshot: &snapshot.Snapshot{
			Features: snapshot.SnapshotContent{
				Derivs: &types.DerivsFeatures{
					OIDelta1hPct:         floatPtr(0.0125),
					OIPriceDiv:           strPtr("confirming"),
					OIPriceCorr24h:       floatPtr(0.67),
					OIZ7d:                floatPtr(1.8),
					FundingLatestBps:     floatPtr(2.3),
					FundingMedianZ7d:     floatPtr(-0.4),
					FundingDispersionBps: floatPtr(0.9),
					BasisPct:             floatPtr(0.006),
					BasisZ14d:            floatPtr(0.5),
					OISourceStatus:       "binanceusdm:ok",
					FundingSourceStatus:  "binanceusdm:ok",
					BasisSourceStatus:    "binanceusdm:ok",
				},
			},
		},
	}
	ctx := buildContextBlock(data)
	if ctx.OIDelta1hPct == nil || *ctx.OIDelta1hPct == 0 {
		t.Fatalf("oi_d1h_pct should be included")
	}
	if ctx.OIPriceDiv == nil || *ctx.OIPriceDiv != "confirming" {
		t.Fatalf("oi_div should be included")
	}
	if ctx.FundingMedianZ7d == nil || ctx.FundingDispersionBps == nil || ctx.BasisZ14d == nil {
		t.Fatalf("fund_z, fund_disp_bps, basis_z should be included")
	}
	if ctx.Source.OI == "" || ctx.Source.Fund == "" || ctx.Source.Basis == "" {
		t.Fatalf("source status fields should be included")
	}
	if ctx.TF != defaultContextTF || ctx.PxType != defaultContextPriceType {
		t.Fatalf("tf and px_type should be explicit")
	}
}

func TestBuildFeatureBlock_FiltersFailedQoS(t *testing.T) {
	now := time.Now().UTC().Add(-2 * time.Minute)
	data := &market.Data{
		CollectedAt: now,
		FeatureStats: map[string]market.FeatureStat{
			market.FeatureKeyF4: {Coverage: 0.2, UpdatedAt: now},
			market.FeatureKeyF5: {Coverage: 0.2, UpdatedAt: now},
			market.FeatureKeyF6: {Coverage: 0.2, UpdatedAt: now},
			market.FeatureKeyF7: {Coverage: 0.2, UpdatedAt: now},
		},
		Snapshot: &snapshot.Snapshot{
			Features: snapshot.SnapshotContent{
				Derivs: &types.DerivsFeatures{
					ConfidenceCVD3m:   floatPtr(0.8),
					ConfidenceLiq3m:   floatPtr(0.8),
					ConfidenceAVWAP3m: floatPtr(0.8),
					ConfidenceVol3m:   floatPtr(0.8),
				},
			},
		},
	}
	diag := &payloadDiagnostics{FeaturePass: make(map[string][]string)}
	features := buildFeatureBlock(data, diag, "BTCUSDT")
	if features.Orderflow != nil || features.Risk != nil || features.Levels != nil || features.Volatility != nil {
		t.Fatalf("all feature blocks should be nil when QoS gates fail")
	}
	if len(diag.FeaturePass["BTCUSDT"]) != 0 {
		t.Fatalf("no feature should pass qos gate")
	}
}

func TestComputeRank_PrefersDirectionalConfluence(t *testing.T) {
	aligned := &market.Data{
		PriceChange1h: 2.4,
		PriceChange4h: 4.1,
		CurrentMACD:   0.5,
		CurrentRSI7:   61,
		Snapshot: &snapshot.Snapshot{
			Features: snapshot.SnapshotContent{
				Derivs: &types.DerivsFeatures{
					PreferDirection3m: strPtr("prefer_longs"),
					AVWAPBias3m:       strPtr("prefer_longs"),
					OIPriceDiv:        strPtr("confirming"),
					OIDelta1hPct:      floatPtr(0.02),
					VolRegime3m:       strPtr("expansion"),
					ConfidenceCVD3m:   floatPtr(0.9),
					ConfidenceLiq3m:   floatPtr(0.85),
					ConfidenceAVWAP3m: floatPtr(0.88),
					ConfidenceVol3m:   floatPtr(0.83),
				},
			},
		},
	}
	conflict := &market.Data{
		PriceChange1h: 2.4,
		PriceChange4h: 4.1,
		CurrentMACD:   -0.3,
		CurrentRSI7:   86,
		Snapshot: &snapshot.Snapshot{
			Features: snapshot.SnapshotContent{
				Derivs: &types.DerivsFeatures{
					PreferDirection3m: strPtr("prefer_shorts"),
					AVWAPBias3m:       strPtr("prefer_shorts"),
					OIPriceDiv:        strPtr("divergent"),
					OIDelta1hPct:      floatPtr(-0.02),
					VolRegime3m:       strPtr("compression"),
					ConfidenceCVD3m:   floatPtr(0.45),
					ConfidenceLiq3m:   floatPtr(0.40),
					ConfidenceAVWAP3m: floatPtr(0.38),
					ConfidenceVol3m:   floatPtr(0.42),
				},
			},
		},
	}
	if computeRank(aligned) <= computeRank(conflict) {
		t.Fatalf("directional confluence should rank above conflicting setup")
	}
}

func strPtr(v string) *string {
	return &v
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
