package api

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"nofx/store"
)

func TestApplyStrategyPatchMergesNestedMaps(t *testing.T) {
	existing := `{
		"execution": {
			"max_positions": 3,
			"filters": {
				"min_confidence": 70,
				"max_spread_bps": 25
			}
		},
		"risk": {
			"take_profit_pct": 4
		}
	}`

	patch := map[string]any{
		"execution": map[string]any{
			"filters": map[string]any{
				"min_confidence": 78,
			},
		},
		"risk": map[string]any{
			"stop_loss_pct": 1.8,
		},
	}

	mergedJSON, err := applyStrategyPatch(existing, patch)
	if err != nil {
		t.Fatalf("applyStrategyPatch() error = %v", err)
	}

	var merged map[string]any
	if err := json.Unmarshal([]byte(mergedJSON), &merged); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	execution := merged["execution"].(map[string]any)
	filters := execution["filters"].(map[string]any)
	risk := merged["risk"].(map[string]any)

	if execution["max_positions"].(float64) != 3 {
		t.Fatalf("execution.max_positions = %v, want 3", execution["max_positions"])
	}
	if filters["min_confidence"].(float64) != 78 {
		t.Fatalf("filters.min_confidence = %v, want 78", filters["min_confidence"])
	}
	if filters["max_spread_bps"].(float64) != 25 {
		t.Fatalf("filters.max_spread_bps = %v, want 25", filters["max_spread_bps"])
	}
	if risk["take_profit_pct"].(float64) != 4 {
		t.Fatalf("risk.take_profit_pct = %v, want 4", risk["take_profit_pct"])
	}
	if risk["stop_loss_pct"].(float64) != 1.8 {
		t.Fatalf("risk.stop_loss_pct = %v, want 1.8", risk["stop_loss_pct"])
	}
}

func TestBuildDealReviewAnalysisPayloadIncludesCaseContexts(t *testing.T) {
	traderCfg := &store.Trader{ID: "trader-1", Name: "Prompt Trader"}
	strategyRecord := &store.Strategy{ID: "strategy-1", Name: "Prompt Strategy", Config: `{"risk":{"max_positions":3}}`}
	strategyCfg := &store.StrategyConfig{}

	cases := []store.DealReviewCaseDetail{
		{
			Case: store.DealReviewCase{
				ID:              "case-1",
				Symbol:          "RAVEUSDT",
				Side:            "LONG",
				Outcome:         "loss",
				RealizedPnL:     -1.25,
				RealizedPnLPct:  -3.2,
				HoldDurationMs:  15 * 60 * 1000,
				CloseReason:     "take_profit",
				OpenStopLoss:    3.03,
				OpenTakeProfit:  3.39,
				OpenConfidence:  72,
				CloseConfidence: 0,
			},
			OpenCandidateSources: []string{"trend_align", "oi_up"},
			Open: &store.DealReviewEventDetail{
				Event: &store.DealReviewEvent{
					DecisionCycleNumber: 771,
					Action:              "open_long",
					Reasoning:           "trend_align, breakout follow-through",
					Confidence:          72,
					Price:               3.44184,
					Quantity:            4,
					StopLoss:            3.035,
					TakeProfit:          3.395,
				},
				Snapshot: &store.DealReviewEventSnapshot{
					SystemPrompt: "system prompt",
					UserPrompt:   "market context prompt",
					DecisionJSON: `{"decisions":[{"sym":"RAVEUSDT","action":"ENTER"}]}`,
					RawResponse:  "<decision>{...}</decision>",
					ExecutionLog: []string{"selected RAVEUSDT", "submitted order"},
					AccountState: store.AccountSnapshot{TotalBalance: 1200, AvailableBalance: 900, PositionCount: 1},
					CandidateCoins: []string{
						"RAVEUSDT",
						"TONUSDT",
					},
				},
			},
			Close: &store.DealReviewEventDetail{
				Event: &store.DealReviewEvent{
					DecisionCycleNumber: 0,
					Action:              "close_long",
					Reasoning:           "Position closed via synced exchange event (take_profit).",
					CloseReason:         "take_profit",
				},
			},
			PriceTimeline: &store.DealReviewPriceTimeline{
				Summary: store.DealReviewPriceTimelineSummary{
					CycleSamples:        3,
					EverInProfit:        true,
					MaxUnrealizedPnL:    0.42,
					MaxUnrealizedPnLPct: 3.1,
					MinUnrealizedPnL:    -0.18,
					MinUnrealizedPnLPct: -1.4,
				},
			},
		},
	}

	summary := &store.DealReviewDatasetSummary{TotalDeals: 1, ClosedDeals: 1, LosingDeals: 1}
	payload, err := buildDealReviewAnalysisPayload(
		traderCfg,
		strategyRecord,
		strategyCfg,
		store.DealReviewListFilter{TraderID: traderCfg.ID, Status: "CLOSED"},
		cases,
		summary,
	)
	if err != nil {
		t.Fatalf("buildDealReviewAnalysisPayload() error = %v", err)
	}

	contexts, ok := payload["case_contexts"].([]map[string]any)
	if !ok || len(contexts) != 1 {
		t.Fatalf("case_contexts = %#v, want single context entry", payload["case_contexts"])
	}

	openContext, ok := contexts[0]["open_context"].(map[string]any)
	if !ok {
		t.Fatalf("open_context = %#v, want map", contexts[0]["open_context"])
	}
	if openContext["input_prompt"] != "market context prompt" {
		t.Fatalf("input_prompt = %#v, want market context prompt", openContext["input_prompt"])
	}
	if openContext["system_prompt"] != "system prompt" {
		t.Fatalf("system_prompt = %#v, want system prompt", openContext["system_prompt"])
	}
	if _, ok := openContext["account_state"].(store.AccountSnapshot); !ok {
		t.Fatalf("account_state = %#v, want AccountSnapshot", openContext["account_state"])
	}
	if _, ok := contexts[0]["price_timeline_summary"].(store.DealReviewPriceTimelineSummary); !ok {
		t.Fatalf("price_timeline_summary = %#v, want DealReviewPriceTimelineSummary", contexts[0]["price_timeline_summary"])
	}
}

func TestBuildDealReviewReplayValidationSupportsConfidenceFilter(t *testing.T) {
	original := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			MinConfidence:                60,
			BTCETHMaxPositionValueRatio:  5,
			AltcoinMaxPositionValueRatio: 1,
			MinRiskRewardRatio:           1.5,
			MinPositionSize:              10,
		},
	}
	merged := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			MinConfidence:                80,
			BTCETHMaxPositionValueRatio:  5,
			AltcoinMaxPositionValueRatio: 1,
			MinRiskRewardRatio:           1.5,
			MinPositionSize:              10,
		},
	}

	training := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "RAVEUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -2, OpenConfidence: 65, EntryPrice: 1, EntryQuantity: 20}},
		{Case: store.DealReviewCase{Symbol: "TONUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 1.5, OpenConfidence: 88, EntryPrice: 1, EntryQuantity: 20}},
	}
	holdout := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "ETHUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 1.2, OpenConfidence: 90, EntryPrice: 100, EntryQuantity: 0.2}},
	}

	replay := buildDealReviewReplayValidation(
		original,
		merged,
		map[string]any{"risk_control": map[string]any{"min_confidence": 80}},
		training,
		holdout,
		holdout,
		"latest 1 holdout deals",
		1,
	)
	if replay == nil || !replay.Supported {
		t.Fatalf("replay = %#v, want supported replay", replay)
	}
	if replay.TrainingNetPnLDelta <= 0 {
		t.Fatalf("training delta = %v, want positive delta after filtering low-confidence loser", replay.TrainingNetPnLDelta)
	}
	if got := replay.SupportedPaths[0]; got != "risk_control.min_confidence" {
		t.Fatalf("supported path = %q, want risk_control.min_confidence", got)
	}
}

func TestBuildDealReviewReplayValidationBlocksHoldoutRegression(t *testing.T) {
	original := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			BTCETHMaxPositionValueRatio:  5,
			AltcoinMaxPositionValueRatio: 1,
			MinPositionSize:              10,
		},
	}
	merged := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			BTCETHMaxPositionValueRatio:  5,
			AltcoinMaxPositionValueRatio: 0.5,
			MinPositionSize:              10,
		},
	}

	training := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "RAVEUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 2, EntryPrice: 1, EntryQuantity: 20}},
	}
	holdout := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "TONUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 4, EntryPrice: 1, EntryQuantity: 20}},
	}

	replay := buildDealReviewReplayValidation(
		original,
		merged,
		map[string]any{"risk_control": map[string]any{"altcoin_max_position_value_ratio": 0.5}},
		training,
		holdout,
		holdout,
		"latest 1 holdout deals",
		1,
	)
	if replay == nil || !replay.Supported {
		t.Fatalf("replay = %#v, want supported replay", replay)
	}
	if replay.HoldoutNetPnLDelta >= 0 {
		t.Fatalf("holdout delta = %v, want negative delta after reducing alt sizing", replay.HoldoutNetPnLDelta)
	}
	if len(replay.BlockingIssues) == 0 {
		t.Fatalf("blocking issues = %#v, want replay regression blocking issue", replay.BlockingIssues)
	}
}

func TestParseDealReviewCaseClassifierResponse(t *testing.T) {
	response := `{
	  "summary":"This looks like an avoidable loss after a weak breakout entry.",
	  "highlight_level":"high",
	  "suggestions":[
	    {
	      "label":"avoidable loss",
	      "issue_type":"likely_avoidable_loss",
	      "highlight_level":"high",
	      "rationale":"Two warning signs were visible before entry and the stop was reached quickly."
	    },
	    {
	      "label":"bad trade",
	      "issue_type":"likely_bad_trade",
	      "highlight_level":"medium",
	      "rationale":"The setup quality looked below the trader's usual standard."
	    }
	  ]
	}`

	assist, err := parseDealReviewCaseClassifierResponse(response)
	if err != nil {
		t.Fatalf("parseDealReviewCaseClassifierResponse() error = %v", err)
	}
	if assist.Source != store.DealReviewClassifierAIAssist {
		t.Fatalf("assist.Source = %q, want %q", assist.Source, store.DealReviewClassifierAIAssist)
	}
	if assist.HighlightLevel != "high" {
		t.Fatalf("assist.HighlightLevel = %q, want high", assist.HighlightLevel)
	}
	if len(assist.Suggestions) != 2 {
		t.Fatalf("len(assist.Suggestions) = %d, want 2", len(assist.Suggestions))
	}
	if assist.Suggestions[0].Label != "avoidable loss" {
		t.Fatalf("top suggestion = %q, want avoidable loss", assist.Suggestions[0].Label)
	}
	if assist.Suggestions[0].SuggestionKey == "" {
		t.Fatal("expected AI assist suggestion key to be populated")
	}
}

func TestBuildDealReviewAIScanDisagreementSummaryFlagsMaterialSplit(t *testing.T) {
	left := &store.DealReviewAIScanDetail{
		Scan: store.DealReviewAIScan{
			Provider:         "openai",
			ModelName:        "gpt-5.4",
			ValidationStatus: store.DealReviewAIScanValidationFailed,
		},
		Filters: map[string]any{
			"symbol": "BTCUSDT",
			"side":   "LONG",
		},
		Result: store.DealReviewAIScanResult{
			ExecutiveSummary: "Trend regime setup for BTC longs",
			ImmediateActions: []store.DealReviewActionItem{
				{Title: "Reduce BTC long aggressiveness"},
			},
			Patterns: []string{"Trend regime losses cluster on late momentum entries"},
		},
		StrategyPatch: map[string]any{
			"execution": map[string]any{"max_positions": 2},
		},
	}
	right := &store.DealReviewAIScanDetail{
		Scan: store.DealReviewAIScan{
			Provider:         "gemini",
			ModelName:        "gemini-3-pro-preview",
			ValidationStatus: store.DealReviewAIScanValidationFailed,
		},
		Filters: map[string]any{
			"symbol": "ETHUSDT",
			"side":   "SHORT",
		},
		Result: store.DealReviewAIScanResult{
			ExecutiveSummary: "Chop regime setup for ETH shorts",
			ImmediateActions: []store.DealReviewActionItem{
				{Title: "Increase ETH short participation"},
			},
			Patterns: []string{"Chop regime mean reversion opportunity"},
		},
		StrategyPatch: map[string]any{
			"risk_control": map[string]any{"min_confidence": 82},
		},
	}

	recommendations := buildDealReviewAIScanRecommendationBlock(left, right)
	evidence := buildDealReviewAIScanEvidenceBlock(left, right)
	targets := buildDealReviewAIScanTargetCohortBlock(
		deriveDealReviewAIScanCohortTags(left, nil),
		deriveDealReviewAIScanCohortTags(right, nil),
	)
	score, level, summary, flags := buildDealReviewAIScanDisagreementSummary(
		left,
		right,
		recommendations,
		evidence,
		targets,
	)

	if score < 60 {
		t.Fatalf("conflict score = %v, want material split >= 60", score)
	}
	if level != "high" {
		t.Fatalf("level = %q, want high", level)
	}
	if !strings.Contains(strings.ToLower(summary), "disagreement") &&
		!strings.Contains(strings.ToLower(summary), "split") {
		t.Fatalf("summary = %q, want disagreement framing", summary)
	}
	flagCodes := map[string]struct{}{}
	for _, flag := range flags {
		flagCodes[flag.Code] = struct{}{}
	}
	if _, ok := flagCodes["mixed_recommendation"]; !ok {
		t.Fatalf("flags = %#v, want mixed_recommendation", flags)
	}
	if _, ok := flagCodes["low_confidence_disagreement"]; !ok {
		t.Fatalf("flags = %#v, want low_confidence_disagreement", flags)
	}
}

func TestBuildDealReviewAIScanLeaderboardsRanksModelsByCohort(t *testing.T) {
	scans := []store.DealReviewAIScanDetail{
		{
			Scan: store.DealReviewAIScan{
				ID:               "scan-gpt",
				Provider:         "openai",
				ModelName:        "gpt-5.4",
				ValidationStatus: store.DealReviewAIScanValidationPassed,
			},
			Filters: map[string]any{"symbol": "BTCUSDT"},
			Result: store.DealReviewAIScanResult{
				ExecutiveSummary: "Trend regime improvements for BTC momentum",
			},
			Validation: &store.DealReviewAIScanValidation{
				Status:         store.DealReviewAIScanValidationPassed,
				PromotionReady: true,
				Replay: &store.DealReviewAIScanReplay{
					HoldoutNetPnLDelta: 1.4,
				},
			},
		},
		{
			Scan: store.DealReviewAIScan{
				ID:               "scan-gemini",
				Provider:         "gemini",
				ModelName:        "gemini-3-pro-preview",
				ValidationStatus: store.DealReviewAIScanValidationFailed,
			},
			Filters: map[string]any{"symbol": "BTCUSDT"},
			Result: store.DealReviewAIScanResult{
				ExecutiveSummary: "Trend regime but weaker BTC setup",
			},
			Validation: &store.DealReviewAIScanValidation{
				Status:         store.DealReviewAIScanValidationFailed,
				PromotionReady: false,
				Replay: &store.DealReviewAIScanReplay{
					HoldoutNetPnLDelta: -0.8,
				},
			},
		},
	}
	versions := []store.DealReviewStrategyVersionDetail{
		{
			Version: store.DealReviewStrategyVersion{SourceScanID: "scan-gpt"},
			TargetCohort: map[string]any{
				"symbol": "BTCUSDT",
			},
			Attribution: &store.DealReviewStrategyVersionAttribution{
				TargetBeforeSummary: &store.DealReviewDatasetSummary{NetPnL: -1.0},
				TargetAfterSummary:  &store.DealReviewDatasetSummary{NetPnL: 1.5},
			},
		},
		{
			Version: store.DealReviewStrategyVersion{SourceScanID: "scan-gemini"},
			TargetCohort: map[string]any{
				"symbol": "BTCUSDT",
			},
			Attribution: &store.DealReviewStrategyVersionAttribution{
				TargetBeforeSummary: &store.DealReviewDatasetSummary{NetPnL: 0.4},
				TargetAfterSummary:  &store.DealReviewDatasetSummary{NetPnL: -0.6},
			},
		},
	}
	compares := []store.DealReviewChallengerCompareDetail{
		{
			Compare: store.DealReviewChallengerCompare{
				SourceScanID:       "scan-gpt",
				Status:             store.DealReviewChallengerStatusCompleted,
				WinnerTraderID:     "challenger",
				ChallengerTraderID: "challenger",
			},
		},
		{
			Compare: store.DealReviewChallengerCompare{
				SourceScanID:       "scan-gemini",
				Status:             store.DealReviewChallengerStatusCompleted,
				WinnerTraderID:     "incumbent",
				ChallengerTraderID: "challenger-2",
			},
		},
	}

	leaderboards := buildDealReviewAIScanLeaderboards(
		scans,
		versions,
		compares,
		[]dealReviewCohortTag{{Key: "regime:trend", Label: "Trend regime", Dimension: "regime"}},
	)
	if len(leaderboards) == 0 {
		t.Fatal("expected at least one leaderboard group")
	}

	var trendGroup *dealReviewAIScanLeaderboardGroup
	for i := range leaderboards {
		if leaderboards[i].CohortKey == "regime:trend" {
			trendGroup = &leaderboards[i]
			break
		}
	}
	if trendGroup == nil {
		t.Fatalf("leaderboards = %#v, want trend regime group", leaderboards)
	}
	if !trendGroup.Relevant {
		t.Fatalf("trend group relevant = false, want true")
	}
	if len(trendGroup.Entries) < 2 {
		t.Fatalf("trend group entries = %#v, want 2 models", trendGroup.Entries)
	}
	if trendGroup.Entries[0].ModelLabel != "openai / gpt-5.4" {
		t.Fatalf("top model = %q, want openai / gpt-5.4", trendGroup.Entries[0].ModelLabel)
	}
	if trendGroup.Entries[0].UsefulnessScore <= trendGroup.Entries[1].UsefulnessScore {
		t.Fatalf("entries = %#v, want first model usefulness score to rank above second", trendGroup.Entries)
	}
}

func TestBuildDealReviewReplayValidationTracksRecentLiveLikeSlice(t *testing.T) {
	original := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			MinConfidence:                60,
			BTCETHMaxPositionValueRatio:  5,
			AltcoinMaxPositionValueRatio: 1,
			MinRiskRewardRatio:           1.5,
			MinPositionSize:              10,
		},
	}
	merged := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			MinConfidence:                85,
			BTCETHMaxPositionValueRatio:  5,
			AltcoinMaxPositionValueRatio: 1,
			MinRiskRewardRatio:           1.5,
			MinPositionSize:              10,
		},
	}

	training := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "BTCUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 2, OpenConfidence: 90, EntryPrice: 100, EntryQuantity: 0.2}},
	}
	holdout := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "RAVEUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 1.4, OpenConfidence: 70, EntryPrice: 1, EntryQuantity: 20}},
		{Case: store.DealReviewCase{Symbol: "TONUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -0.7, OpenConfidence: 88, EntryPrice: 1, EntryQuantity: 20}},
	}
	recent := []store.DealReviewCaseDetail{
		holdout[0],
	}

	replay := buildDealReviewReplayValidation(
		original,
		merged,
		map[string]any{"risk_control": map[string]any{"min_confidence": 85}},
		training,
		holdout,
		recent,
		"latest 1 holdout deals",
		1,
	)
	if replay == nil || !replay.Supported {
		t.Fatalf("replay = %#v, want supported replay", replay)
	}
	if replay.RecentReplaySummary == nil {
		t.Fatalf("recent replay summary = nil, want populated recent replay summary")
	}
	if replay.RecentNetPnLDelta >= 0 {
		t.Fatalf("recent delta = %v, want negative recent live-like delta", replay.RecentNetPnLDelta)
	}
	if len(replay.BlockingIssues) == 0 {
		t.Fatalf("blocking issues = %#v, want recent live-like degradation blocking issue", replay.BlockingIssues)
	}
}

func TestBuildDealReviewReplayValidationSupportsExcludedCoins(t *testing.T) {
	original := &store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{
			SourceType:    "ai500",
			ExcludedCoins: nil,
		},
		RiskControl: store.RiskControlConfig{
			MinPositionSize: 10,
		},
	}
	merged := &store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{
			SourceType:    "ai500",
			ExcludedCoins: []string{"RAVEUSDT"},
		},
		RiskControl: store.RiskControlConfig{
			MinPositionSize: 10,
		},
	}

	training := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "RAVEUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -2, EntryPrice: 1, EntryQuantity: 20}},
		{Case: store.DealReviewCase{Symbol: "TONUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 1.2, EntryPrice: 1, EntryQuantity: 20}},
	}
	holdout := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "RAVEUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -1.1, EntryPrice: 1, EntryQuantity: 20}},
	}

	replay := buildDealReviewReplayValidation(
		original,
		merged,
		map[string]any{"coin_source": map[string]any{"excluded_coins": []any{"RAVEUSDT"}}},
		training,
		holdout,
		holdout,
		"latest 1 holdout deals",
		1,
	)
	if replay == nil || !replay.Supported {
		t.Fatalf("replay = %#v, want supported replay", replay)
	}
	foundPath := false
	for _, path := range replay.SupportedPaths {
		if path == "coin_source.excluded_coins" {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Fatalf("supported paths = %#v, want coin_source.excluded_coins", replay.SupportedPaths)
	}
	if replay.TrainingNetPnLDelta <= 0 {
		t.Fatalf("training delta = %v, want positive delta after excluding a losing symbol", replay.TrainingNetPnLDelta)
	}
}

func TestBuildDealReviewReplayValidationSupportsRestrictiveLeverageCaps(t *testing.T) {
	original := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			AltcoinMaxLeverage: 10,
			MinPositionSize:    10,
		},
	}
	merged := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			AltcoinMaxLeverage: 5,
			MinPositionSize:    10,
		},
	}

	training := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "RAVEUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -2, EntryPrice: 1, EntryQuantity: 20, Leverage: 10}},
	}
	holdout := []store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Symbol: "TONUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -1.5, EntryPrice: 1, EntryQuantity: 20, Leverage: 10}},
	}

	replay := buildDealReviewReplayValidation(
		original,
		merged,
		map[string]any{"risk_control": map[string]any{"altcoin_max_leverage": 5}},
		training,
		holdout,
		holdout,
		"latest 1 holdout deals",
		1,
	)
	if replay == nil || !replay.Supported {
		t.Fatalf("replay = %#v, want supported replay", replay)
	}
	if replay.TrainingNetPnLDelta <= 0 {
		t.Fatalf("training delta = %v, want positive delta after leverage cap reduces loss size", replay.TrainingNetPnLDelta)
	}
	if replay.HoldoutNetPnLDelta <= 0 {
		t.Fatalf("holdout delta = %v, want positive delta after leverage cap reduces holdout loss size", replay.HoldoutNetPnLDelta)
	}
}

func TestBuildDealReviewReplayValidationSupportsMaxPositions(t *testing.T) {
	original := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			MaxPositions:    3,
			MinPositionSize: 10,
		},
	}
	merged := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			MaxPositions:    1,
			MinPositionSize: 10,
		},
	}

	training := []store.DealReviewCaseDetail{
		{
			Case: store.DealReviewCase{Symbol: "RAVEUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -1.5, EntryPrice: 1, EntryQuantity: 20},
			Open: &store.DealReviewEventDetail{
				Snapshot: &store.DealReviewEventSnapshot{
					AccountState: store.AccountSnapshot{PositionCount: 1, TotalBalance: 1000},
				},
			},
		},
		{
			Case: store.DealReviewCase{Symbol: "TONUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 0.8, EntryPrice: 1, EntryQuantity: 20},
			Open: &store.DealReviewEventDetail{
				Snapshot: &store.DealReviewEventSnapshot{
					AccountState: store.AccountSnapshot{PositionCount: 0, TotalBalance: 1000},
				},
			},
		},
	}
	holdout := []store.DealReviewCaseDetail{
		{
			Case: store.DealReviewCase{Symbol: "ETHUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -0.6, EntryPrice: 100, EntryQuantity: 0.2},
			Open: &store.DealReviewEventDetail{
				Snapshot: &store.DealReviewEventSnapshot{
					AccountState: store.AccountSnapshot{PositionCount: 1, TotalBalance: 1000},
				},
			},
		},
	}

	replay := buildDealReviewReplayValidation(
		original,
		merged,
		map[string]any{"risk_control": map[string]any{"max_positions": 1}},
		training,
		holdout,
		holdout,
		"latest 1 holdout deals",
		1,
	)
	if replay == nil || !replay.Supported {
		t.Fatalf("replay = %#v, want supported replay", replay)
	}
	if replay.TrainingNetPnLDelta <= 0 {
		t.Fatalf("training delta = %v, want positive delta after restrictive max_positions removes blocked loser", replay.TrainingNetPnLDelta)
	}
}

func TestBuildDealReviewReplayValidationSupportsMaxMarginUsage(t *testing.T) {
	original := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			MaxMarginUsage: 0.90,
		},
	}
	merged := &store.StrategyConfig{
		RiskControl: store.RiskControlConfig{
			MaxMarginUsage: 0.50,
		},
	}

	training := []store.DealReviewCaseDetail{
		{
			Case: store.DealReviewCase{Symbol: "RAVEUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -2, EntryPrice: 10, EntryQuantity: 20, Leverage: 2},
			Open: &store.DealReviewEventDetail{
				Event: &store.DealReviewEvent{PositionSizeUSD: 200, Leverage: 2},
				Snapshot: &store.DealReviewEventSnapshot{
					AccountState: store.AccountSnapshot{TotalBalance: 1000, MarginUsedPct: 42},
				},
			},
		},
		{
			Case: store.DealReviewCase{Symbol: "TONUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 0.7, EntryPrice: 1, EntryQuantity: 20, Leverage: 2},
			Open: &store.DealReviewEventDetail{
				Event: &store.DealReviewEvent{PositionSizeUSD: 20, Leverage: 2},
				Snapshot: &store.DealReviewEventSnapshot{
					AccountState: store.AccountSnapshot{TotalBalance: 1000, MarginUsedPct: 10},
				},
			},
		},
	}
	holdout := []store.DealReviewCaseDetail{
		{
			Case: store.DealReviewCase{Symbol: "ETHUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -1.1, EntryPrice: 100, EntryQuantity: 1, Leverage: 2},
			Open: &store.DealReviewEventDetail{
				Event: &store.DealReviewEvent{PositionSizeUSD: 100, Leverage: 2},
				Snapshot: &store.DealReviewEventSnapshot{
					AccountState: store.AccountSnapshot{TotalBalance: 1000, MarginUsedPct: 48},
				},
			},
		},
	}

	replay := buildDealReviewReplayValidation(
		original,
		merged,
		map[string]any{"risk_control": map[string]any{"max_margin_usage": 0.5}},
		training,
		holdout,
		holdout,
		"latest 1 holdout deals",
		1,
	)
	if replay == nil || !replay.Supported {
		t.Fatalf("replay = %#v, want supported replay", replay)
	}
	if replay.TrainingNetPnLDelta <= 0 {
		t.Fatalf("training delta = %v, want positive delta after restrictive max_margin gate removes blocked loser", replay.TrainingNetPnLDelta)
	}
	if replay.HoldoutNetPnLDelta <= 0 {
		t.Fatalf("holdout delta = %v, want positive delta after restrictive max_margin gate removes blocked holdout loser", replay.HoldoutNetPnLDelta)
	}
}

func TestBuildDealReviewReplayValidationSupportsCandidateSourceRestrictions(t *testing.T) {
	original := &store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{
			SourceType: "mixed",
			UseAI500:   true,
			AI500Limit: 3,
			UseOITop:   true,
			OITopLimit: 3,
		},
	}
	merged := &store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{
			SourceType: "mixed",
			UseAI500:   true,
			AI500Limit: 1,
			UseOITop:   false,
			OITopLimit: 3,
		},
	}

	training := []store.DealReviewCaseDetail{
		{
			Case:                 store.DealReviewCase{Symbol: "TONUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: -1.7, EntryPrice: 1, EntryQuantity: 20},
			OpenCandidateSources: []string{"ai500", "oi_top"},
			Open: &store.DealReviewEventDetail{
				Snapshot: &store.DealReviewEventSnapshot{
					CandidateCoins: []string{"RAVEUSDT", "TONUSDT"},
					CandidateDetails: []store.CandidateDetail{
						{Symbol: "RAVEUSDT", Sources: []string{"ai500"}},
						{Symbol: "TONUSDT", Sources: []string{"ai500", "oi_top"}},
					},
				},
			},
		},
		{
			Case:                 store.DealReviewCase{Symbol: "RAVEUSDT", Side: "LONG", Status: "CLOSED", RealizedPnL: 0.9, EntryPrice: 1, EntryQuantity: 20},
			OpenCandidateSources: []string{"ai500"},
			Open: &store.DealReviewEventDetail{
				Snapshot: &store.DealReviewEventSnapshot{
					CandidateCoins: []string{"RAVEUSDT", "TONUSDT"},
					CandidateDetails: []store.CandidateDetail{
						{Symbol: "RAVEUSDT", Sources: []string{"ai500"}},
						{Symbol: "TONUSDT", Sources: []string{"ai500", "oi_top"}},
					},
				},
			},
		},
	}
	holdout := []store.DealReviewCaseDetail{
		training[0],
	}

	replay := buildDealReviewReplayValidation(
		original,
		merged,
		map[string]any{"coin_source": map[string]any{"ai500_limit": 1, "use_oi_top": false}},
		training,
		holdout,
		holdout,
		"latest 1 holdout deals",
		1,
	)
	if replay == nil || !replay.Supported {
		t.Fatalf("replay = %#v, want supported replay", replay)
	}
	if replay.TrainingNetPnLDelta <= 0 {
		t.Fatalf("training delta = %v, want positive delta after source restriction removes blocked loser", replay.TrainingNetPnLDelta)
	}
	foundAI500Limit := false
	foundOITopToggle := false
	for _, path := range replay.SupportedPaths {
		if path == "coin_source.ai500_limit" {
			foundAI500Limit = true
		}
		if path == "coin_source.use_oi_top" {
			foundOITopToggle = true
		}
	}
	if !foundAI500Limit || !foundOITopToggle {
		t.Fatalf("supported paths = %#v, want coin_source.ai500_limit and coin_source.use_oi_top", replay.SupportedPaths)
	}
}

func TestSummarizeClosedDealReviewCasesTracksExpectancyAndDrawdown(t *testing.T) {
	summary := summarizeClosedDealReviewCases([]store.DealReviewCaseDetail{
		{Case: store.DealReviewCase{Status: store.DealReviewCaseStatusClosed, Side: "LONG", RealizedPnL: 2.5, RealizedPnLPct: 5}},
		{Case: store.DealReviewCase{Status: store.DealReviewCaseStatusClosed, Side: "LONG", RealizedPnL: -4, RealizedPnLPct: -10}},
		{Case: store.DealReviewCase{Status: store.DealReviewCaseStatusClosed, Side: "SHORT", RealizedPnL: 1, RealizedPnLPct: 2}},
	})
	if summary == nil {
		t.Fatalf("summary = nil, want populated summary")
	}
	if math.Abs(summary.Expectancy-summary.AvgPnL) > 1e-9 {
		t.Fatalf("expectancy = %v, want %v", summary.Expectancy, summary.AvgPnL)
	}
	if summary.MaxDrawdown < 9.5 || summary.MaxDrawdown > 9.6 {
		t.Fatalf("max drawdown = %v, want approx 9.52", summary.MaxDrawdown)
	}
}

func TestApplyDealReviewValidationMetricGatesBlocksHoldoutMetricRegression(t *testing.T) {
	validation := &store.DealReviewAIScanValidation{
		ClosedDealCount:           12,
		TrainingClosedDealCount:   9,
		HoldoutClosedDealCount:    3,
		RecentClosedDealCount:     3,
		MinClosedDealCount:        10,
		MinHoldoutClosedDealCount: 3,
		MinRecentClosedDealCount:  3,
		HoldoutSummary: &store.DealReviewDatasetSummary{
			ClosedDeals:  3,
			WinRate:      66.67,
			AvgPnLPct:    0.8,
			NetPnL:       1.5,
			ProfitFactor: 1.5,
			MaxDrawdown:  2,
			Expectancy:   0.5,
		},
		RecentSummary: &store.DealReviewDatasetSummary{
			ClosedDeals:  3,
			AvgPnLPct:    0.6,
			NetPnL:       1.2,
			ProfitFactor: 1.3,
			MaxDrawdown:  2,
			Expectancy:   0.4,
		},
		Replay: &store.DealReviewAIScanReplay{
			Supported: true,
			TrainingReplaySummary: &store.DealReviewDatasetSummary{
				ClosedDeals:  9,
				ProfitFactor: 0.8,
				MaxDrawdown:  3,
				Expectancy:   0.2,
			},
			HoldoutReplaySummary: &store.DealReviewDatasetSummary{
				ClosedDeals:  3,
				WinRate:      33.33,
				AvgPnLPct:    0.1,
				NetPnL:       0.5,
				ProfitFactor: 0.3,
				MaxDrawdown:  12,
				Expectancy:   0.1,
			},
			RecentReplaySummary: &store.DealReviewDatasetSummary{
				ClosedDeals:  3,
				AvgPnLPct:    -0.1,
				NetPnL:       0.4,
				ProfitFactor: 0.4,
				MaxDrawdown:  11,
				Expectancy:   0.1,
			},
		},
	}

	applyDealReviewValidationMetricGates(validation)

	if len(validation.Checks) == 0 {
		t.Fatalf("checks = %#v, want populated validation checks", validation.Checks)
	}
	if len(validation.BlockingIssues) == 0 {
		t.Fatalf("blocking issues = %#v, want metric gate failures", validation.BlockingIssues)
	}
	foundHoldoutProfitFactor := false
	for _, item := range validation.Checks {
		if item.Key == "holdout_profit_factor_floor" {
			foundHoldoutProfitFactor = true
			if item.Passed {
				t.Fatalf("holdout profit-factor check = %#v, want failed check", item)
			}
		}
	}
	if !foundHoldoutProfitFactor {
		t.Fatalf("checks = %#v, want holdout profit-factor gate", validation.Checks)
	}
}

func TestDidDealReviewChallengerMetricsChangeDetectsPnLDelta(t *testing.T) {
	previous := &store.DealReviewChallengerMetrics{
		IncumbentPnL:         1.2,
		ChallengerPnL:        0.8,
		IncumbentTradeCount:  2,
		ChallengerTradeCount: 1,
	}
	next := &store.DealReviewChallengerMetrics{
		IncumbentPnL:         1.2,
		ChallengerPnL:        1.1,
		IncumbentTradeCount:  2,
		ChallengerTradeCount: 1,
	}

	if !didDealReviewChallengerMetricsChange(previous, next) {
		t.Fatalf("expected challenger metrics change to be detected")
	}
}

func TestAppendDealReviewChallengerProtocolEventPersistsTimeline(t *testing.T) {
	timestamp := time.Date(2026, time.April, 15, 10, 30, 0, 0, time.UTC)
	raw := store.AppendDealReviewChallengerProtocolEvent("", store.DealReviewChallengerProtocolEvent{
		Timestamp: timestamp,
		Type:      "running",
		Actor:     "system",
		Message:   "Compare started.",
		Metrics: &store.DealReviewChallengerMetrics{
			EvaluatedAt:          timestamp,
			IncumbentPnL:         0,
			ChallengerPnL:        0,
			IncumbentTradeCount:  0,
			ChallengerTradeCount: 0,
		},
	})

	events := store.ParseDealReviewChallengerProtocolJSON(raw)
	if len(events) != 1 {
		t.Fatalf("protocol events len = %d, want 1", len(events))
	}
	if events[0].Type != "running" {
		t.Fatalf("protocol event type = %q, want running", events[0].Type)
	}
	if events[0].Metrics == nil {
		t.Fatalf("protocol event metrics = nil, want populated metrics")
	}
	if !events[0].Timestamp.Equal(timestamp) {
		t.Fatalf("protocol event timestamp = %v, want %v", events[0].Timestamp, timestamp)
	}
}
