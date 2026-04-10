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
	if !containsString(payload.OutputContract.DecisionFields, "stops_targets") {
		t.Fatalf("decision_fields should include stops_targets for ENTER decisions")
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
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"3m": {
				Timeframe: "3m",
				Klines: []market.KlineBar{
					{Close: 95, Volume: 100},
					{Close: 96, Volume: 110},
					{Close: 97, Volume: 120},
					{Close: 98, Volume: 130},
					{Close: 99, Volume: 140},
					{Close: 100, Volume: 260},
				},
				EMA20Values: []float64{98.8, 99.4},
				EMA50Values: []float64{97.5, 98.5},
				MACDValues:  []float64{0.11, 0.25},
				RSI7Values:  []float64{52, 56},
				RSI14Values: []float64{49, 53},
				ATR14:       1.2,
			},
			"15m": {
				Timeframe: "15m",
				Klines: []market.KlineBar{
					{Close: 92, Volume: 190},
					{Close: 93, Volume: 200},
					{Close: 94, Volume: 210},
					{Close: 95, Volume: 220},
					{Close: 97, Volume: 230},
					{Close: 100, Volume: 320},
				},
				EMA20Values: []float64{97.2, 98.4},
				EMA50Values: []float64{95.7, 96.8},
				MACDValues:  []float64{-0.08, 0.14},
				RSI7Values:  []float64{48, 58},
				RSI14Values: []float64{45, 54},
				ATR14:       2.1,
			},
			"2h": {
				Timeframe:   "2h",
				Klines:      []market.KlineBar{{Close: 91}, {Close: 100}},
				EMA20Values: []float64{94.2, 96.4},
				EMA50Values: []float64{92.3, 94.1},
				MACDValues:  []float64{-0.30, 0.05},
				RSI7Values:  []float64{44, 57},
				RSI14Values: []float64{42, 52},
				ATR14:       4.7,
			},
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
		VenueTradability: map[string]*VenueTradabilitySummary{
			symbol: {
				VenueSupported: true,
				OrderBookState: "ok",
				MinNotionalOK:  boolPtr(true),
				PriceSource:    "exchange",
			},
		},
		ExecutionQuality: map[string]*ExecutionQualitySummary{
			symbol: {
				SpreadBps:         floatPtr(4.2),
				LiqScore:          floatPtr(0.81),
				DepthBidUSD1Pct:   floatPtr(12500),
				DepthAskUSD1Pct:   floatPtr(11800),
				BookImbalance1Pct: floatPtr(0.03),
				SlippageEst25USD:  floatPtr(0.6),
				SlippageEst100USD: floatPtr(1.1),
			},
		},
		QuantFlowMap: map[string]*QuantFlowSummary{
			symbol: {
				InstFuture15m:     floatPtr(12.3),
				InstFuture1h:      floatPtr(22.8),
				OIDelta15mPct:     floatPtr(3.1),
				PriceChange15mPct: floatPtr(1.4),
			},
		},
		RecentTrades: []RecentTrade{
			{Symbol: symbol, Side: "long", PnLPct: -2.4, HoldDuration: "48m", ExitTimestamp: time.Now().Add(-30 * time.Minute).Unix()},
			{Symbol: symbol, Side: "long", PnLPct: -1.2, HoldDuration: "33m", ExitTimestamp: time.Now().Add(-2 * time.Hour).Unix()},
			{Symbol: symbol, Side: "short", PnLPct: 1.8, HoldDuration: "2h", ExitTimestamp: time.Now().Add(-6 * time.Hour).Unix()},
		},
		PayloadVersion: PayloadSchemaVersion,
		ContextTF:      "3m",
		CurrentTime:    "2025-11-16 04:57:11",
		RuntimeMinutes: 1,
		CallCount:      1,
		Account:        AccountInfo{TotalEquity: 100, AvailableBalance: 100},
	}
	diag := &payloadDiagnostics{FeaturePass: make(map[string][]string)}
	cand, ok := buildCandidatePayload(ctx, CandidateCoin{Symbol: symbol, Sources: []string{"ai500", "oi_top"}}, diag, reqPayload{RequiredTF: "3m"}, defaultContextPriceType)
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
	if len(cand.SourceTags) != 2 || cand.SourceTags[0] != "ai500" || cand.SourceTags[1] != "oi_top" {
		t.Fatalf("candidate source tags should be preserved")
	}
	if len(cand.Timeframes) != 3 {
		t.Fatalf("expected 3 timeframe summaries, got %d", len(cand.Timeframes))
	}
	if cand.Timeframes[0].TF != "3m" || cand.Timeframes[1].TF != "15m" || cand.Timeframes[2].TF != "2h" {
		t.Fatalf("timeframe summaries should prioritize primary then longer confirmations")
	}
	if cand.Context.PriceChange1h == nil || cand.Context.PriceChange4h == nil {
		t.Fatalf("1h and 4h price changes should be included in context")
	}
	if cand.QuantFlow == nil || cand.QuantFlow.InstFuture1h == nil || *cand.QuantFlow.InstFuture1h != 22.8 {
		t.Fatalf("quant flow summary should be included for candidates")
	}
	if cand.Execution == nil || cand.Execution.SpreadBps == nil || *cand.Execution.SpreadBps != 4.2 {
		t.Fatalf("execution quality should be included for candidates")
	}
	if cand.Venue == nil || !cand.Venue.Supported || cand.Venue.Book != "ok" {
		t.Fatalf("venue tradability should be included for candidates")
	}
	if cand.SpreadBps == nil || cand.LiqScore == nil {
		t.Fatalf("top-level spread_bps and liq_score should be populated from execution quality")
	}
	if cand.Volume == nil || cand.Volume.VolRatio20 == nil || cand.Volume.BreakoutVolumeConfirmed == nil {
		t.Fatalf("volume participation should be included when timeframe volumes exist")
	}
	if cand.SymbolMem == nil || cand.SymbolMem.MinutesSinceClose == nil || cand.SymbolMem.SameSymbolLossStreak != 2 || !cand.SymbolMem.CooldownHit {
		t.Fatalf("symbol memory should include cooldown and loss-streak context")
	}
	if cand.FeatureEnv == nil || cand.FeatureEnv.F4 != "ok" || cand.FeatureEnv.F10 != "ok" || cand.FeatureEnv.F11 != "ok" || cand.FeatureEnv.F12 != "ok" {
		t.Fatalf("feature availability should classify live signal blocks correctly")
	}
	if len(diag.FeaturePass[symbol]) != 4 {
		t.Fatalf("expected 4 passed feature gates, got %d", len(diag.FeaturePass[symbol]))
	}
}

func TestBuildCandidatePayload_IncludesRelativeStrengthAndAvailabilityReasons(t *testing.T) {
	ctx := &Context{
		PayloadVersion: PayloadSchemaVersion,
		ContextTF:      "3m",
		CurrentTime:    "2025-11-16 04:57:11",
		RuntimeMinutes: 1,
		CallCount:      1,
		Account:        AccountInfo{TotalEquity: 100, AvailableBalance: 100},
		MarketDataMap: map[string]*market.Data{
			"ETHUSDT": {
				Symbol:        "ETHUSDT",
				CollectedAt:   time.Now().UTC().Add(-40 * time.Second),
				CurrentPrice:  104,
				PriceChange1h: 2.5,
				PriceChange4h: 6.0,
				CurrentEMA20:  101.5,
				CurrentMACD:   0.4,
				CurrentRSI7:   58,
				TimeframeData: map[string]*market.TimeframeSeriesData{
					"3m": {
						Timeframe: "3m",
						Klines: []market.KlineBar{
							{Close: 100, Volume: 10},
							{Close: 104, Volume: 12},
						},
					},
				},
			},
			"BTCUSDT": {
				Symbol:        "BTCUSDT",
				CollectedAt:   time.Now().UTC().Add(-20 * time.Second),
				CurrentPrice:  101000,
				PriceChange1h: 0.9,
				PriceChange4h: 2.0,
				CurrentEMA20:  100400,
				CurrentMACD:   0.2,
				CurrentRSI7:   55,
				TimeframeData: map[string]*market.TimeframeSeriesData{
					"3m": {
						Timeframe: "3m",
						Klines: []market.KlineBar{
							{Close: 100000, Volume: 100},
							{Close: 101000, Volume: 110},
						},
					},
				},
			},
		},
		VenueTradability: map[string]*VenueTradabilitySummary{
			"ETHUSDT": {
				VenueSupported: true,
				OrderBookState: "not_exposed",
				PriceSource:    "exchange",
			},
		},
		CandidateCoins: []CandidateCoin{{Symbol: "ETHUSDT", Sources: []string{"ai500"}}},
	}

	diag := &payloadDiagnostics{FeaturePass: make(map[string][]string)}
	cand, ok := buildCandidatePayload(ctx, CandidateCoin{Symbol: "ETHUSDT", Sources: []string{"ai500"}}, diag, reqPayload{RequiredTF: "3m"}, defaultContextPriceType)
	if !ok {
		t.Fatalf("buildCandidatePayload should return candidate")
	}
	if cand.RelStrength == nil || cand.RelStrength.State != "outperform" {
		t.Fatalf("relative strength should show BTC outperformance context")
	}
	if cand.RelStrength.VsBTC1h == nil || *cand.RelStrength.VsBTC1h <= 0 {
		t.Fatalf("relative strength 1h delta should be positive")
	}
	if cand.FeatureEnv == nil || cand.FeatureEnv.F10 != "orderbook_not_exposed" {
		t.Fatalf("feature availability should explain missing execution block")
	}
}

func TestAssembleLivePayload_IncludesBenchmarkPerformanceAndTradeMemory(t *testing.T) {
	ctx := &Context{
		PayloadVersion: PayloadSchemaVersion,
		ContextTF:      "3m",
		CurrentTime:    "2025-11-16 04:57:11",
		RuntimeMinutes: 12,
		CallCount:      8,
		Account:        AccountInfo{TotalEquity: 250, AvailableBalance: 180},
		Performance: &PerformanceSummary{
			TotalTrades:    21,
			WinRate:        57.14,
			ProfitFactor:   1.62,
			SharpeRatio:    0.94,
			TotalPnL:       123.45,
			AvgWin:         18.2,
			AvgLoss:        -10.7,
			MaxDrawdownPct: 11.8,
		},
		RecentTrades: []RecentTrade{
			{Symbol: "SOLUSDT", Side: "long", PnLPct: 3.2, HoldDuration: "2h10m"},
			{Symbol: "ETHUSDT", Side: "short", PnLPct: -1.4, HoldDuration: "38m"},
		},
		MarketDataMap: map[string]*market.Data{
			"BTCUSDT": {
				Symbol:        "BTCUSDT",
				CurrentPrice:  100000,
				PriceChange1h: 0.8,
				PriceChange4h: 2.4,
				CurrentEMA20:  99400,
				CurrentMACD:   22.1,
				CurrentRSI7:   61.3,
				TimeframeData: map[string]*market.TimeframeSeriesData{
					"3m": {
						Timeframe:   "3m",
						Klines:      []market.KlineBar{{Close: 99500}, {Close: 100000}},
						EMA20Values: []float64{99600, 99820},
						EMA50Values: []float64{99200, 99520},
						MACDValues:  []float64{10.0, 22.1},
						RSI7Values:  []float64{55, 61.3},
						ATR14:       210,
					},
					"15m": {
						Timeframe:   "15m",
						Klines:      []market.KlineBar{{Close: 98000}, {Close: 100000}},
						EMA20Values: []float64{98600, 99250},
						EMA50Values: []float64{97500, 98200},
						MACDValues:  []float64{-12.0, 18.4},
						RSI7Values:  []float64{49, 60.1},
						ATR14:       430,
					},
				},
			},
		},
	}

	payload, _, err := assembleLivePayload(ctx)
	if err != nil {
		t.Fatalf("assembleLivePayload failed: %v", err)
	}
	if payload.Benchmark == nil || payload.Benchmark.Symbol != "BTCUSDT" {
		t.Fatalf("benchmark payload should include BTC context")
	}
	if payload.Performance == nil || payload.Performance.TotalTrades != 21 {
		t.Fatalf("performance summary should be included")
	}
	if len(payload.RecentTrades) != 2 || payload.RecentTrades[0].Symbol != "SOLUSDT" {
		t.Fatalf("recent trade memory should be included")
	}
	if len(payload.Benchmark.Timeframes) != 2 || payload.Benchmark.Timeframes[0].TF != "3m" {
		t.Fatalf("benchmark timeframe summaries should be included")
	}
}

func TestAssembleLivePayload_IncludesMarketLeadership(t *testing.T) {
	ctx := &Context{
		PayloadVersion: PayloadSchemaVersion,
		CurrentTime:    "2025-11-16 04:57:11",
		RuntimeMinutes: 3,
		CallCount:      2,
		Account:        AccountInfo{TotalEquity: 100, AvailableBalance: 80},
		MarketLeadership: &MarketLeadershipSummary{
			OITop1h: []OILeadershipItem{
				{Symbol: "SOLUSDT", OIDelta1hPct: 8.2, PriceChgPct: 3.1},
			},
			InstInflowTop1h: []FlowLeadershipItem{
				{Symbol: "BTCUSDT", Amount: 1520000},
			},
			PriceLeaders4h: []PriceLeadershipItem{
				{Symbol: "ETHUSDT", ChgPct: 7.6},
			},
		},
	}

	payload, _, err := assembleLivePayload(ctx)
	if err != nil {
		t.Fatalf("assembleLivePayload failed: %v", err)
	}
	if payload.MarketLeadership == nil {
		t.Fatalf("market leadership block should be included")
	}
	if len(payload.MarketLeadership.OITop1h) != 1 || payload.MarketLeadership.OITop1h[0].Symbol != "SOLUSDT" {
		t.Fatalf("oi leadership should be preserved")
	}
	if len(payload.MarketLeadership.InstInflowTop1h) != 1 || payload.MarketLeadership.InstInflowTop1h[0].Symbol != "BTCUSDT" {
		t.Fatalf("institution flow leadership should be preserved")
	}
	if len(payload.MarketLeadership.PriceLeaders4h) != 1 || payload.MarketLeadership.PriceLeaders4h[0].ChgPct != 7.6 {
		t.Fatalf("price leadership should be preserved")
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

func boolPtr(v bool) *bool {
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
