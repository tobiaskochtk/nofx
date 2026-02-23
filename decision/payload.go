package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"nofx/market"
	"nofx/pkg/types"
)

const (
	PayloadSchemaVersion     = "trade_snapshot.v4"
	defaultTopK              = 12
	featureCoverageThreshold = 0.95
	featureSLALimitSeconds   = 30
	requiredMaxStaleSeconds  = 300
	defaultContextTF         = "15m"
	defaultContextPriceType  = "mark"
	minScoreEpsilon          = 0.001
)

var requiredCandidateFields = []string{
	"sym",
	"px_mark",
	"side_bias",
	"score",
	"confidence",
	"ctx",
	"qos",
}

var requiredCandidateContextFields = []string{
	"tf",
	"px_type",
	"ema20",
	"macd",
	"rsi7",
	"oi",
	"oi_d1h_pct",
	"fund_bps",
	"basis_pct",
}

var nullableOKFields = []string{
	"candidates[].spread_bps",
	"candidates[].liq_score",
	"candidates[].feat.orderflow",
	"candidates[].feat.risk",
	"candidates[].feat.levels",
	"candidates[].feat.volatility",
}

type livePayload struct {
	Schema         string                `json:"schema"`
	TimestampUTC   string                `json:"ts_utc"`
	Run            runPayload            `json:"run"`
	Account        accountPayload        `json:"account"`
	Defs           defsPayload           `json:"defs"`
	Enums          enumsPayload          `json:"enums"`
	Req            reqPayload            `json:"req"`
	Policy         policyPayload         `json:"pol"`
	Positions      []positionPayload     `json:"positions"`
	Candidates     []candidatePayload    `json:"candidates"`
	OutputContract outputContractPayload `json:"output_contract"`
}

type runPayload struct {
	Cycle    int `json:"cycle"`
	RuntimeS int `json:"runtime_s"`
}

type accountPayload struct {
	Equity     float64 `json:"equity"`
	Balance    float64 `json:"balance"`
	UPnL       float64 `json:"upnl"`
	MarginUsed float64 `json:"margin_used"`
}

type defsPayload struct {
	CtxTF      string `json:"ctx_tf"`
	PxType     string `json:"px_type"`
	StaleS     string `json:"stale_s"`
	FundBps    string `json:"fund_bps"`
	BasisPct   string `json:"basis_pct"`
	OID1hPct   string `json:"oi_d1h_pct"`
	MACD       string `json:"macd"`
	RSI7       string `json:"rsi7"`
	Score      string `json:"score"`
	Confidence string `json:"confidence"`
}

type enumsPayload struct {
	OIDiv []string `json:"oi_div"`
}

type reqPayload struct {
	RequiredTF                  string   `json:"required_tf"`
	MaxStaleS                   int      `json:"max_stale_s"`
	RequiredCandidateFields     []string `json:"required_candidate_fields"`
	RequiredCandidateCtxFields  []string `json:"required_candidate_ctx_fields"`
	NullableOKAny               []string `json:"nullable_ok_any"`
}

type policyPayload struct {
	AllowedActions       []string `json:"allowed_actions"`
	AllowedSides         []string `json:"allowed_sides"`
	SizeCap              float64  `json:"size_cap"`
	MaxLeverage          int      `json:"max_leverage"`
	MaxNewPositions      int      `json:"max_new_positions"`
	ManagePositionsFirst bool     `json:"manage_positions_first"`
	ObeySideBias         bool     `json:"obey_side_bias"`
	ExitOnlyIfInPos      bool     `json:"exit_only_if_in_positions"`
	EntryRule            entryRulePayload `json:"entry_rule"`
	MinScore             float64  `json:"min_score"`
}

type entryRulePayload struct {
	MustEnterIfAnyEligible bool     `json:"must_enter_if_any_eligible"`
	SelectBestBy           []string `json:"select_best_by"`
}

type outputContractPayload struct {
	Format         string                   `json:"format"`
	DecisionMode   string                   `json:"decision_mode"`
	AllowedActions []string                 `json:"allowed_actions"`
	Fields         []string                 `json:"fields"`
	FieldSources   map[string]string        `json:"field_sources"`
	DecisionFields []string                 `json:"decision_fields"`
	Constraints    outputConstraintsPayload `json:"constraints"`
}

type outputConstraintsPayload struct {
	SizePctMax                 float64 `json:"size_pct_max"`
	LeverageMax                int     `json:"leverage_max"`
	MaxNewPositions            int     `json:"max_new_positions"`
	SideMustMatchCandidateBias bool    `json:"side_must_match_candidate_bias"`
	ExitRequiresOpenPosition   bool    `json:"exit_requires_open_position"`
	DecisionsLenMax            int     `json:"decisions_len_max"`
	DecisionsLenMin            int     `json:"decisions_len_min"`
	DecisionsMustBeArray       bool    `json:"decisions_must_be_array"`
}

type positionPayload struct {
	Symbol    string         `json:"sym"`
	Side      string         `json:"side"`
	EntryPx   float64        `json:"entry_px"`
	PxMark    float64        `json:"px_mark"`
	PxLast    *float64       `json:"px_last"`
	PxIndex   *float64       `json:"px_index"`
	PnLPct    *float64       `json:"pnl_pct"`
	Leverage  int            `json:"leverage"`
	LiqPx     *float64       `json:"liq_px"`
	AgeMin    *int           `json:"age_min"`
	SpreadBps *float64       `json:"spread_bps"`
	LiqScore  *float64       `json:"liq_score"`
	Context   contextBlock   `json:"ctx"`
	Features  featurePayload `json:"feat"`
	QoS       symbolQoS      `json:"qos"`
}

type candidatePayload struct {
	Symbol     string         `json:"sym"`
	PxMark     float64        `json:"px_mark"`
	PxLast     *float64       `json:"px_last"`
	PxIndex    *float64       `json:"px_index"`
	SideBias   string         `json:"side_bias"`
	Score      float64        `json:"score"`
	Confidence float64        `json:"confidence"`
	SpreadBps  *float64       `json:"spread_bps"`
	LiqScore   *float64       `json:"liq_score"`
	Context    contextBlock   `json:"ctx"`
	Features   featurePayload `json:"feat"`
	QoS        symbolQoS      `json:"qos"`
}

type contextBlock struct {
	TF                   string         `json:"tf"`
	PxType               string         `json:"px_type"`
	EMA20                *float64       `json:"ema20"`
	MACD                 *float64       `json:"macd"`
	RSI7                 *float64       `json:"rsi7"`
	OI                   *float64       `json:"oi"`
	OIDelta1hPct         *float64       `json:"oi_d1h_pct"`
	OIPriceDiv           *string        `json:"oi_div"`
	OIPriceCorr24h       *float64       `json:"oi_corr"`
	OIZ7d                *float64       `json:"oi_z"`
	FundingBps           *float64       `json:"fund_bps"`
	FundingMedianZ7d     *float64       `json:"fund_z"`
	FundingDispersionBps *float64       `json:"fund_disp_bps"`
	BasisPct             *float64       `json:"basis_pct"`
	BasisZ14d            *float64       `json:"basis_z"`
	Source               contextSources `json:"src"`
}

type contextSources struct {
	OI    string `json:"oi"`
	Fund  string `json:"fund"`
	Basis string `json:"basis"`
}

type featurePayload struct {
	Orderflow  *orderflowPayload  `json:"orderflow"`
	Risk       *riskPayload       `json:"risk"`
	Levels     *levelsPayload     `json:"levels"`
	Volatility *volatilityPayload `json:"volatility"`
}

type orderflowPayload struct {
	CVDShort       *float64 `json:"cvd_short"`
	CVDLong        *float64 `json:"cvd_long"`
	ImbalanceShort *float64 `json:"imbalance_short"`
	ImbalanceLong  *float64 `json:"imbalance_long"`
	TakerBuyRatio  *float64 `json:"taker_buy_ratio"`
	SlopePx20      *float64 `json:"slope_px20"`
	SlopeCVD20     *float64 `json:"slope_cvd20"`
	Confidence     *float64 `json:"confidence"`
}

type riskPayload struct {
	DistUpATR       *float64 `json:"dist_up_atr"`
	DistDnATR       *float64 `json:"dist_dn_atr"`
	LiqRiskUp       *int     `json:"liq_risk_up"`
	LiqRiskDown     *int     `json:"liq_risk_down"`
	PreferDirection *string  `json:"prefer_direction"`
	Confidence      *float64 `json:"confidence"`
}

type levelsPayload struct {
	NearUp      *nearLevelPayload `json:"near_up"`
	NearDn      *nearLevelPayload `json:"near_dn"`
	Bias        *string           `json:"bias"`
	ReclaimUp   *bool             `json:"reclaim_up"`
	RejectionDn *bool             `json:"rejection_down"`
	Confidence  *float64          `json:"confidence"`
}

type nearLevelPayload struct {
	Name    *string  `json:"name"`
	DistATR *float64 `json:"dist_atr"`
}

type volatilityPayload struct {
	BBW            *float64 `json:"bbw"`
	Squeeze        *bool    `json:"squeeze"`
	SqueezeRelease *bool    `json:"squeeze_release"`
	RealizedVol    *float64 `json:"realized_vol"`
	Regime         *string  `json:"regime"`
	Confidence     *float64 `json:"confidence"`
}

type symbolQoS struct {
	StaleS   int      `json:"stale_s"`
	Missing  []string `json:"missing"`
	Coverage float64  `json:"coverage"`
}

type payloadDiagnostics struct {
	FeaturePass map[string][]string
	ByteSize    int
	RuneSize    int
}

// BuildUserPayload exports the compact payload builder for cross-package callers.
func BuildUserPayload(ctx *Context) (string, error) {
	return buildUserPayload(ctx)
}

func buildUserPayload(ctx *Context) (string, error) {
	payload, diag, err := assembleLivePayload(ctx)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode payload: %w", err)
	}
	diag.ByteSize = len(encoded)
	diag.RuneSize = utf8.RuneCount(encoded)
	logPayloadDiagnostics(payload.Schema, diag)
	return string(encoded), nil
}

func assembleLivePayload(ctx *Context) (*livePayload, *payloadDiagnostics, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("context is nil")
	}
	requiredTF := resolveRequiredTF(ctx)
	contextPriceType := resolveContextPriceType(ctx)

	version := strings.TrimSpace(ctx.PayloadVersion)
	if version == "" {
		version = PayloadSchemaVersion
	}
	if version != PayloadSchemaVersion {
		return nil, nil, fmt.Errorf("payload version %s not supported, upgrade to %s", version, PayloadSchemaVersion)
	}

	live := &livePayload{
		Schema:       version,
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
		Run: runPayload{
			Cycle:    ctx.CallCount,
			RuntimeS: maxInt(0, ctx.RuntimeMinutes*60),
		},
		Account: accountPayload{
			Equity:     roundTo(ctx.Account.TotalEquity, 3),
			Balance:    roundTo(ctx.Account.AvailableBalance, 3),
			UPnL:       roundTo(ctx.Account.UnrealizedPnL, 3),
			MarginUsed: roundTo(normalizeMarginUsedRatio(ctx.Account.MarginUsedPct), 3),
		},
		Defs: defsPayload{
			CtxTF:      "all_ctx_indicators_apply_to_this_tf",
			PxType:     "price_type_used_for_decisions (mark|last|index)",
			StaleS:     "seconds since the newest underlying market-data point used for this candidate",
			FundBps:    "funding_rate_bps_per_8h; positive=longs_pay_shorts",
			BasisPct:   "perp_minus_index_pct; negative=perp_below_index",
			OID1hPct:   "open_interest_change_pct_over_1h",
			MACD:       "macd_line_minus_signal_line",
			RSI7:       "rsi_period_7_on_ctx.tf",
			Score:      "normalized_score_0_to_1; higher=better_for_side_bias",
			Confidence: "estimated_signal_reliability_0_to_1",
		},
		Enums: enumsPayload{
			OIDiv: []string{"confirming", "divergent", "neutral"},
		},
		Req: reqPayload{
			RequiredTF:                 requiredTF,
			MaxStaleS:                  requiredMaxStaleSeconds,
			RequiredCandidateFields:    append([]string(nil), requiredCandidateFields...),
			RequiredCandidateCtxFields: append([]string(nil), requiredCandidateContextFields...),
			NullableOKAny:              append([]string(nil), nullableOKFields...),
		},
		Policy: policyPayload{
			AllowedActions:       []string{"HOLD", "ENTER", "EXIT"},
			AllowedSides:         []string{"long", "short"},
			SizeCap:              0.25,
			MaxLeverage:          5,
			MaxNewPositions:      1,
			ManagePositionsFirst: true,
			ObeySideBias:         true,
			ExitOnlyIfInPos:      true,
			EntryRule: entryRulePayload{
				MustEnterIfAnyEligible: true,
				SelectBestBy:           []string{"score", "confidence", "-qos.stale_s"},
			},
			MinScore:             0.70,
		},
		OutputContract: outputContractPayload{
			Format:         "json_only",
			DecisionMode:   "single_best",
			AllowedActions: []string{"HOLD", "ENTER", "EXIT"},
			Fields:         []string{"ts_utc", "cycle", "decisions"},
			FieldSources: map[string]string{
				"ts_utc": "ts_utc",
				"cycle":  "run.cycle",
			},
			DecisionFields: []string{"sym", "action", "side", "size_pct", "leverage", "confidence", "reason_codes"},
			Constraints: outputConstraintsPayload{
				SizePctMax:                 0.25,
				LeverageMax:                5,
				MaxNewPositions:            1,
				SideMustMatchCandidateBias: true,
				ExitRequiresOpenPosition:   true,
				DecisionsLenMax:            1,
				DecisionsLenMin:            0,
				DecisionsMustBeArray:       true,
			},
		},
		Positions:  make([]positionPayload, 0, len(ctx.Positions)),
		Candidates: make([]candidatePayload, 0, len(ctx.CandidateCoins)),
	}

	diag := &payloadDiagnostics{FeaturePass: make(map[string][]string)}
	positionSymbols := make(map[string]struct{}, len(ctx.Positions))
	for _, pos := range ctx.Positions {
		payloadPos := buildPositionPayload(ctx, &pos, diag, live.Req, contextPriceType)
		positionSymbols[payloadPos.Symbol] = struct{}{}
		live.Positions = append(live.Positions, payloadPos)
	}

	candidates := make([]candidatePayload, 0, len(ctx.CandidateCoins))
	for _, coin := range ctx.CandidateCoins {
		symbol := strings.ToUpper(coin.Symbol)
		if _, held := positionSymbols[symbol]; held {
			continue
		}
		payloadCand, ok := buildCandidatePayload(ctx, coin, diag, live.Req, contextPriceType)
		if ok {
			candidates = append(candidates, payloadCand)
		}
	}
	if len(candidates) > 0 {
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})
		limit := defaultTopK
		if len(candidates) < limit {
			limit = len(candidates)
		}
		live.Candidates = candidates[:limit]
	}

	if err := validateLivePayload(live); err != nil {
		return nil, nil, err
	}
	return live, diag, nil
}

func resolveRequiredTF(ctx *Context) string {
	if ctx == nil {
		return defaultContextTF
	}
	tf := strings.TrimSpace(strings.ToLower(ctx.ContextTF))
	if tf != "" {
		return tf
	}
	return defaultContextTF
}

func resolveContextPriceType(ctx *Context) string {
	if ctx == nil {
		return defaultContextPriceType
	}
	pxType := strings.TrimSpace(strings.ToLower(ctx.PriceType))
	switch pxType {
	case "mark", "last", "index":
		return pxType
	default:
		return defaultContextPriceType
	}
}

func buildPositionPayload(ctx *Context, pos *PositionInfo, diag *payloadDiagnostics, req reqPayload, contextPriceType string) positionPayload {
	symbol := strings.ToUpper(pos.Symbol)
	data := ctx.MarketDataMap[symbol]
	context := buildContextBlock(data)
	features := buildFeatureBlock(data, diag, symbol)
	missing := collectMissingFields(&context)
	qos := buildSymbolQoS(data, pos.UpdateTime, missing)

	pxMark := roundTo(pos.MarkPrice, 6)
	if pxMark <= 0 && data != nil {
		pxMark = roundTo(data.CurrentPrice, 6)
	}
	pxLast := optionalRounded(dataCurrentPrice(data), 6)
	pxIndex := deriveIndexPrice(pxMark, context.BasisPct)

	payload := positionPayload{
		Symbol:    symbol,
		Side:      normalizeSide(pos.Side),
		EntryPx:   roundTo(pos.EntryPrice, 6),
		PxMark:    pxMark,
		PxLast:    pxLast,
		PxIndex:   pxIndex,
		Leverage:  pos.Leverage,
		SpreadBps: nil,
		LiqScore:  nil,
		Context:   context,
		Features:  features,
		QoS:       qos,
	}
	if pos.EntryPrice > 0 {
		pnlPct := roundTo(pos.UnrealizedPnLPct, 2)
		payload.PnLPct = &pnlPct
	}
	if pos.LiquidationPrice > 0 {
		val := roundTo(pos.LiquidationPrice, 6)
		payload.LiqPx = &val
	}
	if pos.UpdateTime > 0 {
		age := int(maxFloat(0, float64(time.Now().UnixMilli()-pos.UpdateTime)/60000))
		payload.AgeMin = &age
	}

	if req.RequiredTF != "" {
		payload.Context.TF = req.RequiredTF
	}
	if contextPriceType != "" {
		payload.Context.PxType = contextPriceType
	}
	return payload
}

func buildCandidatePayload(ctx *Context, coin CandidateCoin, diag *payloadDiagnostics, req reqPayload, contextPriceType string) (candidatePayload, bool) {
	symbol := strings.ToUpper(coin.Symbol)
	data := ctx.MarketDataMap[symbol]
	if data == nil {
		return candidatePayload{}, false
	}

	context := buildContextBlock(data)
	features := buildFeatureBlock(data, diag, symbol)
	missing := collectMissingFields(&context)
	qos := buildSymbolQoS(data, 0, missing)

	pxMark := roundTo(data.CurrentPrice, 6)
	pxLast := optionalRounded(data.CurrentPrice, 6)
	pxIndex := deriveIndexPrice(pxMark, context.BasisPct)

	score := roundTo(computeScore(data), 3)
	if score < minScoreEpsilon {
		score = minScoreEpsilon
	}
	confidence := roundTo(derivsConfidenceScore(microDerivs(data)), 3)

	cand := candidatePayload{
		Symbol:     symbol,
		PxMark:     pxMark,
		PxLast:     pxLast,
		PxIndex:    pxIndex,
		SideBias:   deriveSideBias(data),
		Score:      score,
		Confidence: confidence,
		SpreadBps:  nil,
		LiqScore:   nil,
		Context:    context,
		Features:   features,
		QoS:        qos,
	}
	if req.RequiredTF != "" {
		cand.Context.TF = req.RequiredTF
	}
	if contextPriceType != "" {
		cand.Context.PxType = contextPriceType
	}
	return cand, true
}

func buildContextBlock(data *market.Data) contextBlock {
	ctx := contextBlock{
		TF:     defaultContextTF,
		PxType: defaultContextPriceType,
		Source: contextSources{
			OI:    "unknown",
			Fund:  "unknown",
			Basis: "unknown",
		},
	}
	if data == nil {
		return ctx
	}

	ema := roundTo(data.CurrentEMA20, 3)
	macd := roundTo(data.CurrentMACD, 3)
	rsi := roundTo(clampFloat(data.CurrentRSI7, 0, 100), 3)
	ctx.EMA20 = &ema
	ctx.MACD = &macd
	ctx.RSI7 = &rsi

	if data.OpenInterest != nil && data.OpenInterest.Latest > 0 {
		oi := roundTo(data.OpenInterest.Latest, 3)
		ctx.OI = &oi
	}

	derivs := microDerivs(data)
	if derivs != nil {
		if derivs.OIDelta1hPct != nil {
			v := roundTo(*derivs.OIDelta1hPct*100, 3)
			ctx.OIDelta1hPct = &v
		}
		if derivs.OIPriceDiv != nil {
			s := safeString(derivs.OIPriceDiv)
			if s != "" {
				ctx.OIPriceDiv = &s
			}
		}
		if derivs.OIPriceCorr24h != nil {
			v := roundTo(*derivs.OIPriceCorr24h, 3)
			ctx.OIPriceCorr24h = &v
		}
		if derivs.OIZ7d != nil {
			v := roundTo(*derivs.OIZ7d, 3)
			ctx.OIZ7d = &v
		}
		if derivs.FundingLatestBps != nil {
			v := roundTo(*derivs.FundingLatestBps, 3)
			ctx.FundingBps = &v
		} else if data.FundingRate != 0 {
			v := roundTo(data.FundingRate*10_000, 3)
			ctx.FundingBps = &v
		}
		if derivs.FundingMedianZ7d != nil {
			v := roundTo(*derivs.FundingMedianZ7d, 3)
			ctx.FundingMedianZ7d = &v
		}
		if derivs.FundingDispersionBps != nil {
			v := roundTo(*derivs.FundingDispersionBps, 3)
			ctx.FundingDispersionBps = &v
		}
		if derivs.BasisPct != nil {
			v := roundTo(*derivs.BasisPct*100, 3)
			ctx.BasisPct = &v
		}
		if derivs.BasisZ14d != nil {
			v := roundTo(*derivs.BasisZ14d, 3)
			ctx.BasisZ14d = &v
		}
		if status := strings.TrimSpace(derivs.OISourceStatus); status != "" {
			ctx.Source.OI = status
		}
		if status := strings.TrimSpace(derivs.FundingSourceStatus); status != "" {
			ctx.Source.Fund = status
		}
		if status := strings.TrimSpace(derivs.BasisSourceStatus); status != "" {
			ctx.Source.Basis = status
		}
	}
	return ctx
}

func buildFeatureBlock(data *market.Data, diag *payloadDiagnostics, symbol string) featurePayload {
	derivs := microDerivs(data)
	if derivs == nil {
		return featurePayload{}
	}
	features := featurePayload{}
	if f4 := buildOrderflowFeature(data, derivs, diag, symbol); f4 != nil {
		features.Orderflow = f4
	}
	if f5 := buildRiskFeature(data, derivs, diag, symbol); f5 != nil {
		features.Risk = f5
	}
	if f6 := buildLevelsFeature(data, derivs, diag, symbol); f6 != nil {
		features.Levels = f6
	}
	if f7 := buildVolatilityFeature(data, derivs, diag, symbol); f7 != nil {
		features.Volatility = f7
	}
	return features
}

func buildOrderflowFeature(data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *orderflowPayload {
	_, pass := buildQoS(data, market.FeatureKeyF4)
	if !pass {
		return nil
	}
	if diag != nil {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF4)
	}
	return &orderflowPayload{
		CVDShort:       clampPtr(derivs.CVDNotionalZ3mShort),
		CVDLong:        clampPtr(derivs.CVDNotionalZ3mLong),
		ImbalanceShort: clampPtr(derivs.ImbNotionalZ3mShort),
		ImbalanceLong:  clampPtr(derivs.ImbNotionalZ3mLong),
		TakerBuyRatio:  clampPtr(derivs.TBRNotional3m),
		SlopePx20:      clampPtr(derivs.SlopePrice3mShort),
		SlopeCVD20:     clampPtr(derivs.SlopeCVDZ3mShort),
		Confidence:     normalizedConfidence(derivs.ConfidenceCVD3m),
	}
}

func buildRiskFeature(data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *riskPayload {
	_, pass := buildQoS(data, market.FeatureKeyF5)
	if !pass {
		return nil
	}
	if diag != nil {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF5)
	}
	var prefer *string
	if derivs.PreferDirection3m != nil {
		s := strings.ToLower(strings.TrimSpace(*derivs.PreferDirection3m))
		prefer = &s
	}
	return &riskPayload{
		DistUpATR:       positivePtr(derivs.DistUpAtr3m),
		DistDnATR:       positivePtr(derivs.DistDnAtr3m),
		LiqRiskUp:       derivs.LiqRiskUp3m,
		LiqRiskDown:     derivs.LiqRiskDown3m,
		PreferDirection: prefer,
		Confidence:      normalizedConfidence(derivs.ConfidenceLiq3m),
	}
}

func buildLevelsFeature(data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *levelsPayload {
	_, pass := buildQoS(data, market.FeatureKeyF6)
	if !pass {
		return nil
	}
	if diag != nil {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF6)
	}
	out := &levelsPayload{Confidence: normalizedConfidence(derivs.ConfidenceAVWAP3m)}
	if derivs.AVWAPUpName3m != nil || derivs.AVWAPUpDistAtr3m != nil {
		out.NearUp = &nearLevelPayload{
			Name:    normalizeStringPtr(derivs.AVWAPUpName3m),
			DistATR: positivePtr(derivs.AVWAPUpDistAtr3m),
		}
	}
	if derivs.AVWAPDnName3m != nil || derivs.AVWAPDnDistAtr3m != nil {
		out.NearDn = &nearLevelPayload{
			Name:    normalizeStringPtr(derivs.AVWAPDnName3m),
			DistATR: positivePtr(derivs.AVWAPDnDistAtr3m),
		}
	}
	if derivs.AVWAPBias3m != nil {
		bias := strings.ToLower(strings.TrimSpace(*derivs.AVWAPBias3m))
		out.Bias = &bias
	}
	out.ReclaimUp = intFlagToBool(derivs.AVWAPReclaimUp3m)
	out.RejectionDn = intFlagToBool(derivs.AVWAPRejectionDn3m)
	return out
}

func buildVolatilityFeature(data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *volatilityPayload {
	_, pass := buildQoS(data, market.FeatureKeyF7)
	if !pass {
		return nil
	}
	if diag != nil {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF7)
	}
	out := &volatilityPayload{
		BBW:            positivePtr(derivs.BBW3m),
		Squeeze:        intFlagToBool(derivs.SqueezeOn3m),
		SqueezeRelease: intFlagToBool(derivs.SqueezeRelease3m),
		RealizedVol:    positivePtr(derivs.RvRatio3m),
		Confidence:     normalizedConfidence(derivs.ConfidenceVol3m),
	}
	if derivs.VolRegime3m != nil {
		regime := strings.ToLower(strings.TrimSpace(*derivs.VolRegime3m))
		out.Regime = &regime
	}
	return out
}

func collectMissingFields(ctx *contextBlock) []string {
	if ctx == nil {
		return append([]string(nil), requiredCandidateContextFields...)
	}
	missing := make([]string, 0)
	if ctx.EMA20 == nil {
		missing = append(missing, "ema20")
	}
	if ctx.MACD == nil {
		missing = append(missing, "macd")
	}
	if ctx.RSI7 == nil {
		missing = append(missing, "rsi7")
	}
	if ctx.OI == nil {
		missing = append(missing, "oi")
	}
	if ctx.OIDelta1hPct == nil {
		missing = append(missing, "oi_d1h_pct")
	}
	if ctx.FundingBps == nil {
		missing = append(missing, "fund_bps")
	}
	if ctx.BasisPct == nil {
		missing = append(missing, "basis_pct")
	}
	return missing
}

func buildSymbolQoS(data *market.Data, updateMs int64, missing []string) symbolQoS {
	staleS := deriveStaleSeconds(data, updateMs)
	if missing == nil {
		missing = []string{}
	}
	requiredCount := float64(len(requiredCandidateContextFields))
	coverage := 1.0
	if requiredCount > 0 {
		coverage = clampFloat((requiredCount-float64(len(missing)))/requiredCount, 0, 1)
	}
	return symbolQoS{
		StaleS:   staleS,
		Missing:  missing,
		Coverage: roundTo(coverage, 3),
	}
}

func deriveStaleSeconds(data *market.Data, updateMs int64) int {
	stale := requiredMaxStaleSeconds * 2
	if data != nil && !data.CollectedAt.IsZero() {
		stale = int(maxFloat(0, time.Since(data.CollectedAt).Seconds()))
	}
	if updateMs > 0 {
		updateStale := int(maxFloat(0, float64(time.Now().UnixMilli()-updateMs)/1000))
		if stale == 0 || updateStale < stale {
			stale = updateStale
		}
	}
	return stale
}

func deriveIndexPrice(pxMark float64, basisPct *float64) *float64 {
	if pxMark <= 0 || basisPct == nil {
		return nil
	}
	denom := 1 + (*basisPct / 100)
	if math.Abs(denom) < 1e-9 {
		return nil
	}
	indexPx := roundTo(pxMark/denom, 6)
	return &indexPx
}

func deriveSideBias(data *market.Data) string {
	if data == nil {
		return "neutral"
	}
	trendDirRaw := data.PriceChange1h + data.PriceChange4h*0.5
	direction := signFloat(trendDirRaw)
	if direction > 0 {
		return "long"
	}
	if direction < 0 {
		return "short"
	}
	return "neutral"
}

func optionalRounded(value float64, decimals int) *float64 {
	if value == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	v := roundTo(value, decimals)
	return &v
}

func dataCurrentPrice(data *market.Data) float64 {
	if data == nil {
		return 0
	}
	return data.CurrentPrice
}

func normalizeStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	s := strings.ToLower(strings.TrimSpace(*value))
	if s == "" {
		return nil
	}
	return &s
}

func intFlagToBool(value *int) *bool {
	if value == nil {
		return nil
	}
	b := *value != 0
	return &b
}

func normalizeSide(side string) string {
	s := strings.ToLower(strings.TrimSpace(side))
	s = strings.TrimPrefix(s, "open_")
	s = strings.TrimPrefix(s, "close_")
	if s == "long" || s == "short" {
		return s
	}
	return "long"
}

func normalizeMarginUsedRatio(marginUsedPct float64) float64 {
	if marginUsedPct <= 0 {
		return 0
	}
	if marginUsedPct > 1 {
		return marginUsedPct / 100
	}
	return marginUsedPct
}

func microDerivs(data *market.Data) *types.DerivsFeatures {
	if data == nil || data.Snapshot == nil || data.Snapshot.Features.Derivs == nil {
		return nil
	}
	return data.Snapshot.Features.Derivs
}

func computeScore(data *market.Data) float64 {
	if data == nil {
		return 0
	}
	trendDirRaw := data.PriceChange1h + data.PriceChange4h*0.5
	direction := signFloat(trendDirRaw)

	trendStrength := clampFloat((math.Abs(data.PriceChange1h)+math.Abs(data.PriceChange4h)*0.6)/8.0, 0, 1)

	dirCoherence := 0.5
	if direction != 0 {
		macdDir := signFloat(data.CurrentMACD)
		if macdDir == direction {
			dirCoherence += 0.25
		} else if macdDir != 0 {
			dirCoherence -= 0.20
		}
		rsi := clampFloat(data.CurrentRSI7, 0, 100)
		if direction > 0 {
			if rsi >= 45 && rsi <= 72 {
				dirCoherence += 0.20
			}
			if rsi > 80 {
				dirCoherence -= 0.20
			}
		} else {
			if rsi >= 28 && rsi <= 55 {
				dirCoherence += 0.20
			}
			if rsi < 20 {
				dirCoherence -= 0.20
			}
		}
	}
	dirCoherence = clampFloat(dirCoherence, 0, 1)

	d := microDerivs(data)
	confScore := derivsConfidenceScore(d)
	flowAlignment := directionalFlowAlignmentScore(direction, d)
	regimeScore := volatilityRegimeScore(d)

	score := 0.22*trendStrength + 0.26*dirCoherence + 0.22*flowAlignment + 0.14*regimeScore + 0.16*confScore
	return clampFloat(score, 0, 1)
}

// computeRank is kept for compatibility with existing tests and callers.
func computeRank(data *market.Data) float64 {
	return computeScore(data)
}

func derivsConfidenceScore(d *types.DerivsFeatures) float64 {
	if d == nil {
		return 0.5
	}
	confComponents := []*float64{d.ConfidenceCVD3m, d.ConfidenceLiq3m, d.ConfidenceAVWAP3m, d.ConfidenceVol3m}
	var sum float64
	var count float64
	for _, c := range confComponents {
		if c == nil {
			continue
		}
		sum += clampFloat(*c, 0, 1)
		count++
	}
	if count == 0 {
		return 0.5
	}
	return clampFloat(sum/count, 0, 1)
}

func directionalFlowAlignmentScore(direction float64, d *types.DerivsFeatures) float64 {
	if d == nil {
		return 0.5
	}
	score := 0.5
	if d.OIPriceDiv != nil {
		switch safeString(d.OIPriceDiv) {
		case "confirming":
			score += 0.10
		case "divergent":
			score -= 0.15
		}
	}
	if direction != 0 {
		if d.OIDelta1hPct != nil {
			oiDir := signFloat(*d.OIDelta1hPct)
			if oiDir == direction {
				score += 0.20
			} else if oiDir != 0 {
				score -= 0.15
			}
		}
		if d.PreferDirection3m != nil {
			pref := safeString(d.PreferDirection3m)
			if direction > 0 {
				if strings.Contains(pref, "long") {
					score += 0.20
				} else if strings.Contains(pref, "short") {
					score -= 0.20
				}
			} else {
				if strings.Contains(pref, "short") {
					score += 0.20
				} else if strings.Contains(pref, "long") {
					score -= 0.20
				}
			}
		}
		if d.AVWAPBias3m != nil {
			bias := safeString(d.AVWAPBias3m)
			if direction > 0 {
				if strings.Contains(bias, "long") || strings.Contains(bias, "bull") {
					score += 0.15
				} else if strings.Contains(bias, "short") || strings.Contains(bias, "bear") {
					score -= 0.15
				}
			} else {
				if strings.Contains(bias, "short") || strings.Contains(bias, "bear") {
					score += 0.15
				} else if strings.Contains(bias, "long") || strings.Contains(bias, "bull") {
					score -= 0.15
				}
			}
		}
	}
	return clampFloat(score, 0, 1)
}

func volatilityRegimeScore(d *types.DerivsFeatures) float64 {
	if d == nil {
		return 0.55
	}
	score := 0.55
	if d.VolRegime3m != nil {
		regime := safeString(d.VolRegime3m)
		switch {
		case strings.Contains(regime, "expansion"), strings.Contains(regime, "trend"):
			score = 0.90
		case strings.Contains(regime, "volatile"):
			score = 0.75
		case strings.Contains(regime, "compression"), strings.Contains(regime, "squeeze"):
			score = 0.35
		case strings.Contains(regime, "normal"), strings.Contains(regime, "neutral"), strings.Contains(regime, "balanced"):
			score = 0.60
		}
	}
	if d.SqueezeOn3m != nil && *d.SqueezeOn3m == 1 {
		score -= 0.08
	}
	if d.SqueezeRelease3m != nil && *d.SqueezeRelease3m == 1 {
		score += 0.08
	}
	if d.RvRatio3m != nil {
		if *d.RvRatio3m > 1.3 {
			score += 0.05
		}
		if *d.RvRatio3m < 0.8 {
			score -= 0.05
		}
	}
	return clampFloat(score, 0, 1)
}

func buildQoS(data *market.Data, key string) (symbolQoS, bool) {
	if data == nil {
		return symbolQoS{StaleS: featureSLALimitSeconds * 2, Missing: []string{}, Coverage: 0}, false
	}
	stat, ok := data.FeatureQuality(key)
	coverage := 0.0
	age := featureSLALimitSeconds * 2
	if ok {
		coverage = clampFloat(stat.Coverage, 0, 1)
		if !stat.UpdatedAt.IsZero() {
			age = int(maxFloat(0, time.Since(stat.UpdatedAt).Seconds()))
		}
	} else if !data.CollectedAt.IsZero() {
		age = int(maxFloat(0, time.Since(data.CollectedAt).Seconds()))
	}
	qos := symbolQoS{
		StaleS:   age,
		Missing:  []string{},
		Coverage: roundTo(coverage, 3),
	}
	pass := coverage >= featureCoverageThreshold && age <= featureSLALimitSeconds
	return qos, pass
}

func validateLivePayload(payload *livePayload) error {
	if payload.Schema != PayloadSchemaVersion {
		return fmt.Errorf("invalid payload schema %s", payload.Schema)
	}
	if payload.TimestampUTC == "" {
		return fmt.Errorf("ts_utc missing")
	}
	if payload.Req.RequiredTF == "" {
		return fmt.Errorf("req.required_tf missing")
	}
	if payload.Req.MaxStaleS <= 0 {
		return fmt.Errorf("req.max_stale_s invalid")
	}
	if len(payload.Req.RequiredCandidateFields) == 0 {
		return fmt.Errorf("req.required_candidate_fields missing")
	}
	if len(payload.Req.RequiredCandidateCtxFields) == 0 {
		return fmt.Errorf("req.required_candidate_ctx_fields missing")
	}
	if math.IsNaN(payload.Account.Equity) || payload.Account.Equity < 0 {
		return fmt.Errorf("account equity invalid")
	}
	if payload.Policy.SizeCap <= 0 || payload.Policy.SizeCap > 1 {
		return fmt.Errorf("policy size_cap invalid")
	}
	if payload.Policy.MinScore < 0 || payload.Policy.MinScore > 1 {
		return fmt.Errorf("policy min_score invalid")
	}
	if payload.OutputContract.Constraints.DecisionsLenMin < 0 {
		return fmt.Errorf("output_contract.constraints.decisions_len_min invalid")
	}
	if payload.OutputContract.Constraints.DecisionsLenMax < payload.OutputContract.Constraints.DecisionsLenMin {
		return fmt.Errorf("output_contract.constraints.decisions_len_max invalid")
	}
	if err := validateAssets(payload.Positions, payload.Candidates, payload.Req); err != nil {
		return err
	}
	return nil
}

func validateAssets(positions []positionPayload, candidates []candidatePayload, req reqPayload) error {
	for _, pos := range positions {
		if pos.Symbol == "" {
			return fmt.Errorf("position symbol missing")
		}
		if pos.Side != "long" && pos.Side != "short" {
			return fmt.Errorf("position %s side invalid", pos.Symbol)
		}
		if err := validateContext(pos.Context, req.RequiredTF, pos.Symbol, "position"); err != nil {
			return err
		}
		if err := validateSymbolQoS(pos.QoS, pos.Symbol); err != nil {
			return err
		}
	}
	for _, cand := range candidates {
		if cand.Symbol == "" {
			return fmt.Errorf("candidate symbol missing")
		}
		if cand.Score < 0 || cand.Score > 1 {
			return fmt.Errorf("candidate score out of range for %s", cand.Symbol)
		}
		if cand.Confidence < 0 || cand.Confidence > 1 {
			return fmt.Errorf("candidate confidence out of range for %s", cand.Symbol)
		}
		if cand.SideBias != "long" && cand.SideBias != "short" && cand.SideBias != "neutral" {
			return fmt.Errorf("candidate side_bias invalid for %s", cand.Symbol)
		}
		if err := validateContext(cand.Context, req.RequiredTF, cand.Symbol, "candidate"); err != nil {
			return err
		}
		if err := validateSymbolQoS(cand.QoS, cand.Symbol); err != nil {
			return err
		}
	}
	return nil
}

func validateContext(ctx contextBlock, requiredTF, symbol, assetType string) error {
	if requiredTF != "" && ctx.TF != requiredTF {
		return fmt.Errorf("%s %s tf mismatch: got %s want %s", assetType, symbol, ctx.TF, requiredTF)
	}
	if ctx.RSI7 != nil {
		if *ctx.RSI7 < 0 || *ctx.RSI7 > 100 {
			return fmt.Errorf("%s %s rsi7 out of range", assetType, symbol)
		}
	}
	return nil
}

func validateSymbolQoS(q symbolQoS, symbol string) error {
	if q.Coverage < 0 || q.Coverage > 1 {
		return fmt.Errorf("%s qos coverage out of range", symbol)
	}
	if q.StaleS < 0 {
		return fmt.Errorf("%s qos stale_s negative", symbol)
	}
	return nil
}

func logPayloadDiagnostics(version string, diag *payloadDiagnostics) {
	if diag == nil {
		return
	}
	log.Printf("📦 payload ver=%s bytes=%d runes=%d", version, diag.ByteSize, diag.RuneSize)
	if len(diag.FeaturePass) == 0 {
		log.Printf("📉 QoS: no features passed gates this cycle")
		return
	}
	var entries []string
	for sym, feats := range diag.FeaturePass {
		sort.Strings(feats)
		entries = append(entries, fmt.Sprintf("%s[%s]", sym, strings.Join(feats, ",")))
	}
	sort.Strings(entries)
	log.Printf("✅ QoS pass: %s", strings.Join(entries, " | "))
}

func roundTo(val float64, decimals int) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	factor := math.Pow(10, float64(decimals))
	return math.Round(val*factor) / factor
}

func floatPtr(value float64) *float64 {
	return &value
}

func clampPtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := roundTo(*value, 4)
	return &v
}

func positivePtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := math.Abs(*value)
	return floatPtr(roundTo(v, 3))
}

func normalizedConfidence(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := clampFloat(*value, 0, 1)
	return floatPtr(roundTo(v, 3))
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if math.IsNaN(value) {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(*value))
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func signFloat(value float64) float64 {
	if value > 1e-9 {
		return 1
	}
	if value < -1e-9 {
		return -1
	}
	return 0
}
