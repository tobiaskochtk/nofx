package store

import (
	"encoding/json"
	"math"
	"strings"
	"time"
)

type DealReviewMarketContextSnapshot struct {
	Source                 string   `json:"source,omitempty"`
	Symbol                 string   `json:"symbol,omitempty"`
	Timeframe              string   `json:"timeframe,omitempty"`
	PriceType              string   `json:"price_type,omitempty"`
	Price                  float64  `json:"price,omitempty"`
	TrendRegime            string   `json:"trend_regime,omitempty"`
	VolatilityRegime       string   `json:"volatility_regime,omitempty"`
	BTCStrengthRegime      string   `json:"btc_strength_regime,omitempty"`
	FundingRegime          string   `json:"funding_regime,omitempty"`
	OIRegime               string   `json:"oi_regime,omitempty"`
	SessionBucket          string   `json:"session_bucket,omitempty"`
	WeekdayBucket          string   `json:"weekday_bucket,omitempty"`
	VenueTier              string   `json:"venue_tier,omitempty"`
	LiquidityTier          string   `json:"liquidity_tier,omitempty"`
	SpreadBucket           string   `json:"spread_bucket,omitempty"`
	SlippageBucket         string   `json:"slippage_bucket,omitempty"`
	PriceChange1h          *float64 `json:"price_change_1h,omitempty"`
	PriceChange4h          *float64 `json:"price_change_4h,omitempty"`
	EMAFast                *float64 `json:"ema_fast,omitempty"`
	MACD                   *float64 `json:"macd,omitempty"`
	RSI                    *float64 `json:"rsi,omitempty"`
	OIDelta1hPct           *float64 `json:"oi_delta_1h_pct,omitempty"`
	FundingBps             *float64 `json:"funding_bps,omitempty"`
	BasisPct               *float64 `json:"basis_pct,omitempty"`
	BTCRelativeStrength1h  *float64 `json:"btc_relative_strength_1h,omitempty"`
	BTCRelativeStrength4h  *float64 `json:"btc_relative_strength_4h,omitempty"`
	BTCRelativeStrengthCtx *float64 `json:"btc_relative_strength_ctx,omitempty"`
	SpreadBps              *float64 `json:"spread_bps,omitempty"`
	SlippageEst25USD       *float64 `json:"slippage_est_25usd,omitempty"`
	SlippageEst100USD      *float64 `json:"slippage_est_100usd,omitempty"`
	LiqScore               *float64 `json:"liq_score,omitempty"`
	VenueSupported         *bool    `json:"venue_supported,omitempty"`
	BookState              string   `json:"book_state,omitempty"`
	PriceSource            string   `json:"price_source,omitempty"`
	FreshnessBucket        string   `json:"freshness_bucket,omitempty"`
}

func (s *DealReviewMarketContextSnapshot) HasAny() bool {
	if s == nil {
		return false
	}
	return s.Source != "" ||
		s.Symbol != "" ||
		s.Timeframe != "" ||
		s.TrendRegime != "" ||
		s.VolatilityRegime != "" ||
		s.BTCStrengthRegime != "" ||
		s.FundingRegime != "" ||
		s.OIRegime != "" ||
		s.SessionBucket != "" ||
		s.WeekdayBucket != "" ||
		s.VenueTier != "" ||
		s.LiquidityTier != "" ||
		s.SpreadBucket != "" ||
		s.SlippageBucket != "" ||
		s.Price > 0 ||
		s.PriceChange1h != nil ||
		s.PriceChange4h != nil ||
		s.EMAFast != nil ||
		s.MACD != nil ||
		s.RSI != nil ||
		s.OIDelta1hPct != nil ||
		s.FundingBps != nil ||
		s.BasisPct != nil ||
		s.BTCRelativeStrength1h != nil ||
		s.BTCRelativeStrength4h != nil ||
		s.BTCRelativeStrengthCtx != nil ||
		s.SpreadBps != nil ||
		s.SlippageEst25USD != nil ||
		s.SlippageEst100USD != nil ||
		s.LiqScore != nil ||
		s.VenueSupported != nil ||
		s.BookState != "" ||
		s.PriceSource != "" ||
		s.FreshnessBucket != ""
}

type dealReviewPromptPayload struct {
	Candidates []dealReviewPromptSymbolBlock `json:"candidates"`
	Positions  []dealReviewPromptSymbolBlock `json:"positions"`
}

type dealReviewPromptSymbolBlock struct {
	Symbol           string                            `json:"sym"`
	Side             string                            `json:"side,omitempty"`
	PxMark           float64                           `json:"px_mark"`
	Context          dealReviewPromptContextBlock      `json:"ctx"`
	Venue            *dealReviewPromptVenueBlock       `json:"venue_tradability,omitempty"`
	Execution        *dealReviewPromptExecutionBlock   `json:"execution_quality,omitempty"`
	FeatureEnv       *dealReviewPromptFeatureEnvBlock  `json:"feature_availability,omitempty"`
	RelativeStrength *dealReviewPromptRelativeStrength `json:"relative_strength,omitempty"`
	Features         dealReviewPromptFeaturesBlock     `json:"feat"`
}

type dealReviewPromptContextBlock struct {
	TF            string   `json:"tf"`
	PxType        string   `json:"px_type"`
	EMAFast       *float64 `json:"ema_fast"`
	MACD          *float64 `json:"macd"`
	RSI           *float64 `json:"rsi"`
	PriceChange1h *float64 `json:"chg_1h,omitempty"`
	PriceChange4h *float64 `json:"chg_4h,omitempty"`
	OIDelta1hPct  *float64 `json:"oi_d1h_pct"`
	FundingBps    *float64 `json:"fund_bps"`
	BasisPct      *float64 `json:"basis_pct"`
}

type dealReviewPromptVenueBlock struct {
	Supported     bool    `json:"supported"`
	Book          string  `json:"book"`
	MinNotionalOK *bool   `json:"min_notional_ok,omitempty"`
	PriceSource   *string `json:"price_source,omitempty"`
}

type dealReviewPromptExecutionBlock struct {
	SpreadBps         *float64 `json:"spread_bps,omitempty"`
	LiqScore          *float64 `json:"liq_score,omitempty"`
	SlippageEst25USD  *float64 `json:"slippage_est_25usd,omitempty"`
	SlippageEst100USD *float64 `json:"slippage_est_100usd,omitempty"`
}

type dealReviewPromptFeatureEnvBlock struct {
	Freshness string `json:"freshness"`
}

type dealReviewPromptRelativeStrength struct {
	VsBTC1h  *float64 `json:"vs_btc_1h,omitempty"`
	VsBTC4h  *float64 `json:"vs_btc_4h,omitempty"`
	VsBTCCtx *float64 `json:"vs_btc_ctx,omitempty"`
	State    string   `json:"state,omitempty"`
}

type dealReviewPromptFeaturesBlock struct {
	Volatility *dealReviewPromptVolatilityBlock `json:"volatility"`
}

type dealReviewPromptVolatilityBlock struct {
	Regime *string `json:"regime"`
}

func buildDealReviewMarketContextFromDecisionRecord(
	record *DecisionRecord,
	stage string,
	symbol string,
	side string,
	decisionTime time.Time,
) *DealReviewMarketContextSnapshot {
	if record == nil || strings.TrimSpace(record.InputPrompt) == "" || strings.TrimSpace(symbol) == "" {
		return nil
	}
	var payload dealReviewPromptPayload
	if err := json.Unmarshal([]byte(record.InputPrompt), &payload); err != nil {
		return nil
	}

	var selected *dealReviewPromptSymbolBlock
	if stage == DealReviewStageClose {
		selected = findDealReviewPromptSymbol(payload.Positions, symbol, side)
		if selected == nil {
			selected = findDealReviewPromptSymbol(payload.Candidates, symbol, side)
		}
	} else {
		selected = findDealReviewPromptSymbol(payload.Candidates, symbol, side)
		if selected == nil {
			selected = findDealReviewPromptSymbol(payload.Positions, symbol, side)
		}
	}
	if selected == nil {
		return nil
	}

	context := &DealReviewMarketContextSnapshot{
		Source:          dealReviewMarketContextSource(stage, selected, symbol, side),
		Symbol:          strings.ToUpper(strings.TrimSpace(selected.Symbol)),
		Timeframe:       strings.TrimSpace(strings.ToLower(selected.Context.TF)),
		PriceType:       strings.TrimSpace(strings.ToLower(selected.Context.PxType)),
		Price:           selected.PxMark,
		PriceChange1h:   cloneFloatPtr(selected.Context.PriceChange1h),
		PriceChange4h:   cloneFloatPtr(selected.Context.PriceChange4h),
		EMAFast:         cloneFloatPtr(selected.Context.EMAFast),
		MACD:            cloneFloatPtr(selected.Context.MACD),
		RSI:             cloneFloatPtr(selected.Context.RSI),
		OIDelta1hPct:    cloneFloatPtr(selected.Context.OIDelta1hPct),
		FundingBps:      cloneFloatPtr(selected.Context.FundingBps),
		BasisPct:        cloneFloatPtr(selected.Context.BasisPct),
		FreshnessBucket: normalizeDealReviewMarketToken(selected.FeatureEnvValue("freshness")),
	}
	if selected.RelativeStrength != nil {
		context.BTCRelativeStrength1h = cloneFloatPtr(selected.RelativeStrength.VsBTC1h)
		context.BTCRelativeStrength4h = cloneFloatPtr(selected.RelativeStrength.VsBTC4h)
		context.BTCRelativeStrengthCtx = cloneFloatPtr(selected.RelativeStrength.VsBTCCtx)
		context.BTCStrengthRegime = normalizeDealReviewBTCStrengthRegime(selected.RelativeStrength.State)
	}
	if selected.Execution != nil {
		context.SpreadBps = cloneFloatPtr(selected.Execution.SpreadBps)
		context.SlippageEst25USD = cloneFloatPtr(selected.Execution.SlippageEst25USD)
		context.SlippageEst100USD = cloneFloatPtr(selected.Execution.SlippageEst100USD)
		context.LiqScore = cloneFloatPtr(selected.Execution.LiqScore)
	}
	if selected.Venue != nil {
		supported := selected.Venue.Supported
		context.VenueSupported = &supported
		context.BookState = normalizeDealReviewMarketToken(selected.Venue.Book)
		if selected.Venue.PriceSource != nil {
			context.PriceSource = normalizeDealReviewMarketToken(*selected.Venue.PriceSource)
		}
	}
	context.TrendRegime = deriveDealReviewTrendRegime(selected.PxMark, selected.Context)
	context.VolatilityRegime = normalizeDealReviewVolatilityRegime(selected.VolatilityRegime())
	context.FundingRegime = deriveDealReviewFundingRegime(selected.Context.FundingBps)
	context.OIRegime = deriveDealReviewOIRegime(selected.Context.OIDelta1hPct)
	context.SessionBucket = deriveDealReviewSessionBucket(decisionTime)
	context.WeekdayBucket = deriveDealReviewWeekdayBucket(decisionTime)
	context.VenueTier = deriveDealReviewVenueTier(selected.Venue)
	context.LiquidityTier = deriveDealReviewLiquidityTier(selected.Execution)
	context.SpreadBucket = deriveDealReviewSpreadBucket(selected.Execution)
	context.SlippageBucket = deriveDealReviewSlippageBucket(selected.Execution)
	if context.BTCStrengthRegime == "" {
		context.BTCStrengthRegime = deriveDealReviewBTCStrengthRegime(selected.RelativeStrength)
	}
	if !context.HasAny() {
		return nil
	}
	return context
}

func findDealReviewPromptSymbol(items []dealReviewPromptSymbolBlock, symbol string, side string) *dealReviewPromptSymbolBlock {
	targetSymbol := strings.ToUpper(strings.TrimSpace(symbol))
	targetSide := normalizeDealReviewSide(side)
	for i := range items {
		itemSymbol := strings.ToUpper(strings.TrimSpace(items[i].Symbol))
		if itemSymbol != targetSymbol {
			continue
		}
		if targetSide != "" && items[i].Side != "" && normalizeDealReviewSide(items[i].Side) != targetSide {
			continue
		}
		return &items[i]
	}
	return nil
}

func applyDealReviewMarketContextToCase(caseRec *DealReviewCase, stage string, context *DealReviewMarketContextSnapshot) bool {
	if caseRec == nil || context == nil {
		return false
	}
	stage = strings.ToLower(strings.TrimSpace(stage))
	changed := false
	assign := func(target *string, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		if *target == value {
			return
		}
		*target = value
		changed = true
	}
	if stage == DealReviewStageClose {
		assign(&caseRec.CloseTrendRegime, context.TrendRegime)
		assign(&caseRec.CloseVolatilityRegime, context.VolatilityRegime)
		assign(&caseRec.CloseBTCStrengthRegime, context.BTCStrengthRegime)
		assign(&caseRec.CloseFundingRegime, context.FundingRegime)
		assign(&caseRec.CloseOIRegime, context.OIRegime)
		assign(&caseRec.CloseSessionBucket, context.SessionBucket)
		assign(&caseRec.CloseWeekdayBucket, context.WeekdayBucket)
		assign(&caseRec.CloseVenueTier, context.VenueTier)
		assign(&caseRec.CloseLiquidityTier, context.LiquidityTier)
		assign(&caseRec.CloseSpreadBucket, context.SpreadBucket)
		assign(&caseRec.CloseSlippageBucket, context.SlippageBucket)
		return changed
	}
	assign(&caseRec.OpenTrendRegime, context.TrendRegime)
	assign(&caseRec.OpenVolatilityRegime, context.VolatilityRegime)
	assign(&caseRec.OpenBTCStrengthRegime, context.BTCStrengthRegime)
	assign(&caseRec.OpenFundingRegime, context.FundingRegime)
	assign(&caseRec.OpenOIRegime, context.OIRegime)
	assign(&caseRec.OpenSessionBucket, context.SessionBucket)
	assign(&caseRec.OpenWeekdayBucket, context.WeekdayBucket)
	assign(&caseRec.OpenVenueTier, context.VenueTier)
	assign(&caseRec.OpenLiquidityTier, context.LiquidityTier)
	assign(&caseRec.OpenSpreadBucket, context.SpreadBucket)
	assign(&caseRec.OpenSlippageBucket, context.SlippageBucket)
	return changed
}

func (b *dealReviewPromptSymbolBlock) VolatilityRegime() string {
	if b == nil || b.Features.Volatility == nil || b.Features.Volatility.Regime == nil {
		return ""
	}
	return *b.Features.Volatility.Regime
}

func (b *dealReviewPromptSymbolBlock) FeatureEnvValue(name string) string {
	if b == nil || b.FeatureEnv == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "freshness":
		return b.FeatureEnv.Freshness
	default:
		return ""
	}
}

func dealReviewMarketContextSource(stage string, selected *dealReviewPromptSymbolBlock, symbol string, side string) string {
	if selected == nil {
		return ""
	}
	if stage == DealReviewStageClose && selected.Side != "" {
		return "position"
	}
	if selected.Side != "" && normalizeDealReviewSide(selected.Side) == normalizeDealReviewSide(side) {
		return "position"
	}
	return "candidate"
}

func cloneFloatPtr(value *float64) *float64 {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return nil
	}
	copy := *value
	return &copy
}

func normalizeDealReviewMarketToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func normalizeDealReviewVolatilityRegime(raw string) string {
	token := normalizeDealReviewMarketToken(raw)
	switch token {
	case "", "unknown":
		return ""
	case "expansion", "expanding", "high", "high_vol", "volatile":
		return "high_vol"
	case "compression", "compressed", "low", "low_vol", "squeeze":
		return "low_vol"
	case "normal", "neutral":
		return "normal"
	default:
		return token
	}
}

func normalizeDealReviewBTCStrengthRegime(raw string) string {
	token := normalizeDealReviewMarketToken(raw)
	switch token {
	case "", "unknown":
		return ""
	case "outperform", "leading", "strong":
		return "outperform"
	case "lagging", "weak", "underperform":
		return "lagging"
	case "neutral", "mixed":
		return "neutral"
	default:
		return token
	}
}

func deriveDealReviewFundingRegime(value *float64) string {
	if value == nil {
		return ""
	}
	switch {
	case *value >= 5:
		return "extreme_longs"
	case *value >= 1.5:
		return "longs_pay"
	case *value <= -5:
		return "extreme_shorts"
	case *value <= -1.5:
		return "shorts_pay"
	default:
		return "neutral"
	}
}

func deriveDealReviewOIRegime(value *float64) string {
	if value == nil {
		return ""
	}
	switch {
	case *value >= 8:
		return "oi_surge"
	case *value >= 2:
		return "oi_rising"
	case *value <= -8:
		return "oi_flush"
	case *value <= -2:
		return "oi_falling"
	default:
		return "oi_flat"
	}
}

func deriveDealReviewTrendRegime(price float64, ctx dealReviewPromptContextBlock) string {
	bullishSignals := 0
	bearishSignals := 0
	rangeSignals := 0

	if ctx.EMAFast != nil && price > 0 {
		diffPct := math.Abs(price-*ctx.EMAFast) / price * 100
		if diffPct <= 0.25 {
			rangeSignals++
		}
		if price >= *ctx.EMAFast {
			bullishSignals++
		} else {
			bearishSignals++
		}
	}
	if ctx.MACD != nil {
		switch {
		case *ctx.MACD >= 0.015:
			bullishSignals++
		case *ctx.MACD <= -0.015:
			bearishSignals++
		default:
			rangeSignals++
		}
	}
	if ctx.PriceChange1h != nil {
		switch {
		case *ctx.PriceChange1h >= 0.35:
			bullishSignals++
		case *ctx.PriceChange1h <= -0.35:
			bearishSignals++
		default:
			rangeSignals++
		}
	}
	if ctx.PriceChange4h != nil {
		switch {
		case *ctx.PriceChange4h >= 0.9:
			bullishSignals++
		case *ctx.PriceChange4h <= -0.9:
			bearishSignals++
		default:
			rangeSignals++
		}
	}
	if ctx.RSI != nil {
		switch {
		case *ctx.RSI >= 57:
			bullishSignals++
		case *ctx.RSI <= 43:
			bearishSignals++
		default:
			rangeSignals++
		}
	}

	if bullishSignals >= 3 && bullishSignals > bearishSignals+1 {
		return "uptrend"
	}
	if bearishSignals >= 3 && bearishSignals > bullishSignals+1 {
		return "downtrend"
	}
	if rangeSignals >= 3 {
		return "chop"
	}
	if bullishSignals > bearishSignals {
		return "uptrend"
	}
	if bearishSignals > bullishSignals {
		return "downtrend"
	}
	if bullishSignals == 0 && bearishSignals == 0 {
		return ""
	}
	return "mixed"
}

func deriveDealReviewBTCStrengthRegime(rel *dealReviewPromptRelativeStrength) string {
	if rel == nil {
		return ""
	}
	if regime := normalizeDealReviewBTCStrengthRegime(rel.State); regime != "" {
		return regime
	}
	sum := 0.0
	count := 0.0
	for _, value := range []*float64{rel.VsBTC1h, rel.VsBTC4h, rel.VsBTCCtx} {
		if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
			continue
		}
		sum += *value
		count++
	}
	if count == 0 {
		return ""
	}
	avg := sum / count
	switch {
	case avg >= 1:
		return "outperform"
	case avg <= -1:
		return "lagging"
	default:
		return "neutral"
	}
}

func deriveDealReviewSessionBucket(decisionTime time.Time) string {
	if decisionTime.IsZero() {
		return ""
	}
	hour := decisionTime.UTC().Hour()
	switch {
	case hour >= 23 || hour < 7:
		return "asia"
	case hour >= 7 && hour < 13:
		return "eu"
	case hour >= 13 && hour < 21:
		return "us"
	default:
		return "off_hours"
	}
}

func deriveDealReviewWeekdayBucket(decisionTime time.Time) string {
	if decisionTime.IsZero() {
		return ""
	}
	return strings.ToLower(decisionTime.UTC().Weekday().String())
}

func deriveDealReviewVenueTier(venue *dealReviewPromptVenueBlock) string {
	if venue == nil {
		return ""
	}
	if !venue.Supported {
		return "unsupported"
	}
	if venue.MinNotionalOK != nil && !*venue.MinNotionalOK {
		return "restricted"
	}
	book := normalizeDealReviewMarketToken(venue.Book)
	switch {
	case strings.Contains(book, "thin"), strings.Contains(book, "partial"):
		return "thin_book"
	case book == "", book == "unknown":
		return "unknown"
	default:
		return "tradable"
	}
}

func deriveDealReviewLiquidityTier(execution *dealReviewPromptExecutionBlock) string {
	if execution == nil || execution.LiqScore == nil {
		return ""
	}
	switch {
	case *execution.LiqScore >= 0.8:
		return "high"
	case *execution.LiqScore >= 0.55:
		return "medium"
	default:
		return "low"
	}
}

func deriveDealReviewSpreadBucket(execution *dealReviewPromptExecutionBlock) string {
	if execution == nil || execution.SpreadBps == nil {
		return ""
	}
	switch {
	case *execution.SpreadBps <= 3:
		return "tight"
	case *execution.SpreadBps <= 8:
		return "normal"
	case *execution.SpreadBps <= 15:
		return "wide"
	default:
		return "extreme"
	}
}

func deriveDealReviewSlippageBucket(execution *dealReviewPromptExecutionBlock) string {
	if execution == nil {
		return ""
	}
	var value *float64
	if execution.SlippageEst100USD != nil {
		value = execution.SlippageEst100USD
	} else {
		value = execution.SlippageEst25USD
	}
	if value == nil {
		return ""
	}
	switch {
	case *value <= 2:
		return "tight"
	case *value <= 5:
		return "normal"
	case *value <= 10:
		return "wide"
	default:
		return "extreme"
	}
}
