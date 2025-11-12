package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"nofx/market"
	"nofx/pkg/types"
)

const (
	PayloadSchemaVersion     = "3.1"
	defaultTopK              = 12 // Increased to support all configured coins (was 8)
	featureCoverageThreshold = 0.95
	featureSLALimitSeconds   = 30
	minRankEpsilon           = 0.001
)

var (
	bootDict = map[string]string{
		"ts":        "timestamp ISO-8601 UTC",
		"acct":      "account",
		"eq":        "equity",
		"bal":       "balance",
		"upnl":      "unrealized_pnl_pct",
		"marg":      "margin_pct",
		"pos":       "positions",
		"sym":       "symbol",
		"side":      "side",
		"px":        "last_price",
		"ent":       "entry_price",
		"lev":       "leverage",
		"liq":       "liq_price",
		"age_min":   "minutes open",
		"ctx":       "context fields",
		"atr3":      "ATR_3m_len14",
		"ema20":     "EMA20",
		"macd":      "MACD",
		"rsi7":      "RSI7",
		"oi":        "open_interest_last",
		"fund_bps":  "funding in bps",
		"basis_pct": "basis in %",
		"f":         "features",
		"f4":        "microstructure_cvd",
		"f5":        "liq_heatmap",
		"f6":        "avwap",
		"f7":        "vol_regime",
		"qos":       "quality_of_signals",
		"cov":       "coverage_0to1",
		"age_s":     "seconds_since_latest_sample",
		"c15":       "15m_confirm",
		"rank":      "server-side rank score",
		"pol":       "policy",
		"topk":      "top_k candidates",
		"conf":      "confidence_0to1",
	}
	bootOnce sync.Once
)

type bootMessage struct {
	Version string            `json:"ver"`
	Dict    map[string]string `json:"dict"`
}

type feedEnvelope struct {
	Boot *bootMessage `json:"boot,omitempty"`
	*livePayload
}

type livePayload struct {
	Version    string             `json:"ver"`
	Timestamp  string             `json:"ts"`
	Account    accountPayload     `json:"acct"`
	Positions  []positionPayload  `json:"pos,omitempty"`
	Candidates []candidatePayload `json:"cands,omitempty"`
	Policy     policyPayload      `json:"pol"`
	TopK       int                `json:"topk"`
	Meta       *metaPayload       `json:"meta,omitempty"`
}

type accountPayload struct {
	Equity           float64 `json:"eq"`
	Balance          float64 `json:"bal"`
	UnrealizedPnLPct float64 `json:"upnl"`
	MarginPct        float64 `json:"marg"`
}

type positionPayload struct {
	Symbol    string          `json:"sym"`
	Side      string          `json:"side"`
	Entry     float64         `json:"ent"`
	Price     float64         `json:"px"`
	Leverage  int             `json:"lev"`
	LiqPrice  *float64        `json:"liq,omitempty"`
	AgeMin    *int            `json:"age_min,omitempty"`
	Context   *contextBlock   `json:"ctx,omitempty"`
	Features  *featureBlock   `json:"f,omitempty"`
	Confirm15 *confirmation15 `json:"c15,omitempty"`
}

type candidatePayload struct {
	Symbol   string        `json:"sym"`
	Price    float64       `json:"px"`
	Rank     float64       `json:"rank"`
	Context  *contextBlock `json:"ctx,omitempty"`
	Features *featureBlock `json:"f,omitempty"`
}

type contextBlock struct {
	EMA20      *float64 `json:"ema20,omitempty"`
	MACD       *float64 `json:"macd,omitempty"`
	RSI7       *float64 `json:"rsi7,omitempty"`
	ATR3       *float64 `json:"atr3,omitempty"`
	OI         *float64 `json:"oi,omitempty"`
	FundingBps *float64 `json:"fund_bps,omitempty"`
	BasisPct   *float64 `json:"basis_pct,omitempty"`
}

type featureBlock struct {
	F4 *feature4Payload `json:"f4,omitempty"`
	F5 *feature5Payload `json:"f5,omitempty"`
	F6 *feature6Payload `json:"f6,omitempty"`
	F7 *feature7Payload `json:"f7,omitempty"`
}

type featureQoS struct {
	Coverage  float64 `json:"cov"`
	AgeSecond int     `json:"age_s"`
}

type feature4Payload struct {
	ShortCVD   *float64        `json:"cvd_s,omitempty"`
	LongCVD    *float64        `json:"cvd_l,omitempty"`
	ShortImb   *float64        `json:"imb_s,omitempty"`
	LongImb    *float64        `json:"imb_l,omitempty"`
	TBR        *float64        `json:"tbr,omitempty"`
	Slopes     *feature4Slopes `json:"slopes,omitempty"`
	Div        *feature4Div    `json:"div,omitempty"`
	Confidence *float64        `json:"conf,omitempty"`
	QoS        featureQoS      `json:"qos"`
}

type feature4Slopes struct {
	PriceShort *float64 `json:"p20,omitempty"`
	CVDSlope   *float64 `json:"cvd20,omitempty"`
}

type feature4Div struct {
	BearShort *int `json:"bear_s,omitempty"`
	BearMid   *int `json:"bear_m,omitempty"`
}

type feature5Payload struct {
	Dist       *distancePayload `json:"dist,omitempty"`
	Prefer     string           `json:"prefer,omitempty"`
	Risk       *liqRiskPayload  `json:"risk,omitempty"`
	Confidence *float64         `json:"conf,omitempty"`
	QoS        featureQoS       `json:"qos"`
}

type distancePayload struct {
	UpATR   *float64 `json:"up_atr,omitempty"`
	DownATR *float64 `json:"dn_atr,omitempty"`
}

type liqRiskPayload struct {
	Up   *int `json:"up,omitempty"`
	Down *int `json:"dn,omitempty"`
}

type feature6Payload struct {
	Near       *nearbyLevels `json:"near,omitempty"`
	Bias       string        `json:"bias,omitempty"`
	ReclaimUp  *int          `json:"reclaim_up,omitempty"`
	RejectDown *int          `json:"reject_dn,omitempty"`
	Confidence *float64      `json:"conf,omitempty"`
	QoS        featureQoS    `json:"qos"`
}

type nearbyLevels struct {
	Up   *nearLevel `json:"up,omitempty"`
	Down *nearLevel `json:"dn,omitempty"`
}

type nearLevel struct {
	Name    string   `json:"name,omitempty"`
	DistATR *float64 `json:"dist_atr,omitempty"`
}

type feature7Payload struct {
	BBW        *float64   `json:"bbw,omitempty"`
	Squeeze    *int       `json:"sq,omitempty"`
	RVRatio    *float64   `json:"rv,omitempty"`
	Regime     string     `json:"reg,omitempty"`
	Confidence *float64   `json:"conf,omitempty"`
	QoS        featureQoS `json:"qos"`
}

type confirmation15 struct {
	F4 *confirmationF4 `json:"f4,omitempty"`
	F5 *confirmationF5 `json:"f5,omitempty"`
}

type confirmationF4 struct {
	Confidence *float64 `json:"conf,omitempty"`
}

type confirmationF5 struct {
	Dist       *distancePayload `json:"dist,omitempty"`
	Prefer     string           `json:"prefer,omitempty"`
	Confidence *float64         `json:"conf,omitempty"`
}

type policyPayload struct {
	SizeCap    float64 `json:"size_cap"`
	HoldOnNull bool    `json:"hold_on_null"`
	Need15m    bool    `json:"need_15m"`
	MinConf    float64 `json:"min_conf"`
	SLASecs    int     `json:"sla_s,omitempty"`
}

type metaPayload struct {
	Summary   string `json:"summary"`
	Account   string `json:"account"`
	Positions string `json:"positions"`
	Latest    string `json:"latest,omitempty"`
}

type payloadDiagnostics struct {
	FeaturePass map[string][]string
	ByteSize    int
	RuneSize    int
}

func buildUserPayload(ctx *Context) (string, error) {
	payload, diag, err := assembleLivePayload(ctx)
	if err != nil {
		return "", err
	}
	envelope := feedEnvelope{
		livePayload: payload,
	}
	if shouldAttachBoot() {
		envelope.Boot = &bootMessage{
			Version: PayloadSchemaVersion,
			Dict:    bootDict,
		}
	}
	encoded, err := json.Marshal(&envelope)
	if err != nil {
		return "", fmt.Errorf("encode payload: %w", err)
	}
	diag.ByteSize = len(encoded)
	diag.RuneSize = utf8.RuneCount(encoded)
	logPayloadDiagnostics(payload.Version, diag)
	return string(encoded), nil
}

func shouldAttachBoot() bool {
	send := false
	bootOnce.Do(func() {
		send = true
	})
	return send
}

func assembleLivePayload(ctx *Context) (*livePayload, *payloadDiagnostics, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("context is nil")
	}
	version := strings.TrimSpace(ctx.PayloadVersion)
	if version == "" {
		version = PayloadSchemaVersion
	}
	if version != PayloadSchemaVersion {
		return nil, nil, fmt.Errorf("payload version %s not supported, upgrade to %s", version, PayloadSchemaVersion)
	}
	acct := accountPayload{
		Equity:           roundTo(ctx.Account.TotalEquity, 3),
		Balance:          roundTo(ctx.Account.AvailableBalance, 3),
		UnrealizedPnLPct: roundTo(ctx.Account.TotalPnLPct, 3),
		MarginPct:        roundTo(ctx.Account.MarginUsedPct, 3),
	}
	live := &livePayload{
		Version:   version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Account:   acct,
		Policy: policyPayload{
			SizeCap:    0.25,
			HoldOnNull: true,
			Need15m:    true,
			MinConf:    0.7,
			SLASecs:    featureSLALimitSeconds,
		},
		TopK: defaultTopK,
	}
	diag := &payloadDiagnostics{FeaturePass: make(map[string][]string)}
	positionSymbols := make(map[string]struct{}, len(ctx.Positions))
	for _, pos := range ctx.Positions {
		positionSymbols[strings.ToUpper(pos.Symbol)] = struct{}{}
		payloadPos := buildPositionPayload(ctx, &pos, diag)
		live.Positions = append(live.Positions, payloadPos)
	}
	candidates := make([]candidatePayload, 0, len(ctx.CandidateCoins))
	for _, coin := range ctx.CandidateCoins {
		symbol := strings.ToUpper(coin.Symbol)
		if _, held := positionSymbols[symbol]; held {
			continue
		}
		payloadCand, ok := buildCandidatePayload(ctx, symbol, diag)
		if ok {
			candidates = append(candidates, payloadCand)
		}
	}
	if len(candidates) > 0 {
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].Rank > candidates[j].Rank
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
	live.Meta = buildMetaPayload(ctx)
	return live, diag, nil
}

func buildPositionPayload(ctx *Context, pos *PositionInfo, diag *payloadDiagnostics) positionPayload {
	payload := positionPayload{
		Symbol:   strings.ToUpper(pos.Symbol),
		Side:     strings.ToLower(pos.Side),
		Entry:    roundTo(pos.EntryPrice, 4),
		Price:    roundTo(pos.MarkPrice, 4),
		Leverage: pos.Leverage,
	}
	if pos.LiquidationPrice > 0 {
		val := roundTo(pos.LiquidationPrice, 4)
		payload.LiqPrice = &val
	}
	if pos.UpdateTime > 0 {
		age := int(maxFloat(0, float64(time.Now().UnixMilli()-pos.UpdateTime)/60000))
		payload.AgeMin = &age
	}
	data := ctx.MarketDataMap[payload.Symbol]
	if data != nil {
		payload.Context = buildContextBlock(data)
		payload.Features, payload.Confirm15 = buildFeatureBlock(data, diag, payload.Symbol)
	}
	return payload
}

func buildCandidatePayload(ctx *Context, symbol string, diag *payloadDiagnostics) (candidatePayload, bool) {
	data := ctx.MarketDataMap[symbol]
	if data == nil {
		return candidatePayload{}, false
	}
	price := roundTo(data.CurrentPrice, 4)
	cand := candidatePayload{
		Symbol: symbol,
		Price:  price,
	}
	cand.Context = buildContextBlock(data)
	cand.Features, _ = buildFeatureBlock(data, diag, symbol)
	cand.Rank = roundTo(computeRank(data), 3)
	if cand.Rank < minRankEpsilon {
		cand.Rank = minRankEpsilon
	}
	return cand, true
}

func buildContextBlock(data *market.Data) *contextBlock {
	if data == nil {
		return nil
	}
	ctx := &contextBlock{}
	ctx.EMA20 = floatPtr(roundTo(data.CurrentEMA20, 3))
	ctx.MACD = floatPtr(roundTo(data.CurrentMACD, 3))
	rsi := clampFloat(data.CurrentRSI7, 0, 100)
	ctx.RSI7 = floatPtr(roundTo(rsi, 3))
	if data.LongerTermContext != nil && data.LongerTermContext.ATR3 > 0 {
		ctx.ATR3 = floatPtr(roundTo(data.LongerTermContext.ATR3, 4))
	}
	if data.OpenInterest != nil && data.OpenInterest.Latest > 0 {
		ctx.OI = floatPtr(roundTo(data.OpenInterest.Latest, 3))
	}
	derivs := microDerivs(data)
	if derivs != nil {
		if derivs.FundingLatestBps != nil {
			ctx.FundingBps = floatPtr(roundTo(*derivs.FundingLatestBps, 3))
		} else if data.FundingRate != 0 {
			ctx.FundingBps = floatPtr(roundTo(data.FundingRate*10_000, 3))
		}
		if derivs.BasisPct != nil {
			ctx.BasisPct = floatPtr(roundTo(*derivs.BasisPct*100, 3))
		}
	}
	return ctx
}

func buildFeatureBlock(data *market.Data, diag *payloadDiagnostics, symbol string) (*featureBlock, *confirmation15) {
	derivs := microDerivs(data)
	if derivs == nil {
		return nil, nil
	}
	features := &featureBlock{}
	confirm := &confirmation15{}
	if f4 := buildFeature4(data, derivs, diag, symbol); f4 != nil {
		features.F4 = f4
	}
	if f5, c15 := buildFeature5(data, derivs, diag, symbol); f5 != nil {
		features.F5 = f5
		if c15 != nil {
			confirm.F5 = c15
		}
	}
	if f6 := buildFeature6(data, derivs, diag, symbol); f6 != nil {
		features.F6 = f6
	}
	if f7 := buildFeature7(data, derivs, diag, symbol); f7 != nil {
		features.F7 = f7
	}
	if c15 := buildConfirmF4(derivs); c15 != nil {
		confirm.F4 = c15
	}
	if features.F4 == nil && features.F5 == nil && features.F6 == nil && features.F7 == nil {
		features = nil
	}
	if confirm.F4 == nil && confirm.F5 == nil {
		confirm = nil
	}
	return features, confirm
}

func buildFeature4(data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *feature4Payload {
	qos, pass := buildQoS(data, market.FeatureKeyF4)
	if pass {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF4)
	}
	f4 := &feature4Payload{QoS: qos}
	f4.ShortCVD = clampPtr(derivs.CVDNotionalZ3mShort)
	f4.LongCVD = clampPtr(derivs.CVDNotionalZ3mLong)
	f4.ShortImb = clampPtr(derivs.ImbNotionalZ3mShort)
	f4.LongImb = clampPtr(derivs.ImbNotionalZ3mLong)
	f4.TBR = clampPtr(derivs.TBRNotional3m)
	f4.Slopes = &feature4Slopes{
		PriceShort: clampPtr(derivs.SlopePrice3mShort),
		CVDSlope:   clampPtr(derivs.SlopeCVDZ3mShort),
	}
	f4.Div = &feature4Div{
		BearShort: derivs.DivBearShort3m,
		BearMid:   derivs.DivBearMid3m,
	}
	f4.Confidence = normalizedConfidence(derivs.ConfidenceCVD3m)
	return f4
}

func buildFeature5(data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) (*feature5Payload, *confirmationF5) {
	qos, pass := buildQoS(data, market.FeatureKeyF5)
	if pass {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF5)
	}
	f5 := &feature5Payload{QoS: qos}
	f5.Dist = &distancePayload{
		UpATR:   positivePtr(derivs.DistUpAtr3m),
		DownATR: positivePtr(derivs.DistDnAtr3m),
	}
	if derivs.PreferDirection3m != nil {
		f5.Prefer = strings.ToLower(*derivs.PreferDirection3m)
	}
	f5.Risk = &liqRiskPayload{Up: derivs.LiqRiskUp3m, Down: derivs.LiqRiskDown3m}
	f5.Confidence = normalizedConfidence(derivs.ConfidenceLiq3m)
	confirm := buildConfirmF5(derivs)
	return f5, confirm
}

func buildFeature6(data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *feature6Payload {
	qos, pass := buildQoS(data, market.FeatureKeyF6)
	if pass {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF6)
	}
	f6 := &feature6Payload{QoS: qos}
	near := &nearbyLevels{}
	if derivs.AVWAPUpName3m != nil || derivs.AVWAPUpDistAtr3m != nil {
		near.Up = &nearLevel{Name: safeString(derivs.AVWAPUpName3m), DistATR: positivePtr(derivs.AVWAPUpDistAtr3m)}
	}
	if derivs.AVWAPDnName3m != nil || derivs.AVWAPDnDistAtr3m != nil {
		near.Down = &nearLevel{Name: safeString(derivs.AVWAPDnName3m), DistATR: positivePtr(derivs.AVWAPDnDistAtr3m)}
	}
	if near.Up != nil || near.Down != nil {
		f6.Near = near
	}
	if derivs.AVWAPBias3m != nil {
		f6.Bias = strings.ToLower(*derivs.AVWAPBias3m)
	}
	f6.ReclaimUp = derivs.AVWAPReclaimUp3m
	f6.RejectDown = derivs.AVWAPRejectionDn3m
	f6.Confidence = normalizedConfidence(derivs.ConfidenceAVWAP3m)
	return f6
}

func buildFeature7(data *market.Data, derivs *types.DerivsFeatures, diag *payloadDiagnostics, symbol string) *feature7Payload {
	qos, pass := buildQoS(data, market.FeatureKeyF7)
	if pass {
		diag.FeaturePass[symbol] = append(diag.FeaturePass[symbol], market.FeatureKeyF7)
	}
	f7 := &feature7Payload{QoS: qos}
	f7.BBW = positivePtr(derivs.BBW3m)
	f7.Squeeze = derivs.SqueezeOn3m
	f7.RVRatio = positivePtr(derivs.RvRatio3m)
	if derivs.VolRegime3m != nil {
		f7.Regime = strings.ToLower(*derivs.VolRegime3m)
	}
	f7.Confidence = normalizedConfidence(derivs.ConfidenceVol3m)
	return f7
}

func buildConfirmF4(derivs *types.DerivsFeatures) *confirmationF4 {
	if derivs == nil || derivs.ConfidenceCVD15m == nil {
		return nil
	}
	return &confirmationF4{Confidence: normalizedConfidence(derivs.ConfidenceCVD15m)}
}

func buildConfirmF5(derivs *types.DerivsFeatures) *confirmationF5 {
	if derivs == nil {
		return nil
	}
	if derivs.DistUpAtr15m == nil && derivs.DistDnAtr15m == nil && derivs.ConfidenceLiq15m == nil {
		return nil
	}
	conf := &confirmationF5{
		Dist:       &distancePayload{UpATR: positivePtr(derivs.DistUpAtr15m), DownATR: positivePtr(derivs.DistDnAtr15m)},
		Confidence: normalizedConfidence(derivs.ConfidenceLiq15m),
	}
	if derivs.PreferDirection15m != nil {
		conf.Prefer = strings.ToLower(*derivs.PreferDirection15m)
	}
	return conf
}

func computeRank(data *market.Data) float64 {
	if data == nil {
		return 0
	}
	var scores []float64
	momentum := (math.Abs(data.PriceChange1h) + math.Abs(data.PriceChange4h)*0.5) / 10
	scores = append(scores, clampFloat(momentum, 0, 1))
	rsiScore := math.Abs(data.CurrentRSI7-50) / 50
	scores = append(scores, clampFloat(rsiScore, 0, 1))
	if data.Snapshot != nil && data.Snapshot.Features.Derivs != nil {
		d := data.Snapshot.Features.Derivs
		confComponents := []*float64{d.ConfidenceCVD3m, d.ConfidenceLiq3m, d.ConfidenceAVWAP3m, d.ConfidenceVol3m}
		var sum float64
		var count float64
		for _, c := range confComponents {
			if c != nil {
				sum += clampFloat(*c, 0, 1)
				count++
			}
		}
		if count > 0 {
			scores = append(scores, sum/count)
		}
	}
	if len(scores) == 0 {
		return 0
	}
	sum := 0.0
	for _, s := range scores {
		sum += clampFloat(s, 0, 1)
	}
	return clampFloat(sum/float64(len(scores)), 0, 1)
}

func buildMetaPayload(ctx *Context) *metaPayload {
	if ctx == nil {
		return nil
	}
	summary := fmt.Sprintf("Time: %s | Cycle: #%d | Running: %d minutes", ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes)
	acct := ctx.Account
	var balancePct float64
	if acct.TotalEquity > 0 {
		balancePct = (acct.AvailableBalance / acct.TotalEquity) * 100
	}
	accountLine := fmt.Sprintf(
		"Account: Equity %.2f | Balance %.2f (%.1f%%) | P&L %+.2f%% | Margin %.1f%% | Positions %d",
		roundTo(acct.TotalEquity, 2),
		roundTo(acct.AvailableBalance, 2),
		roundTo(balancePct, 1),
		roundTo(acct.TotalPnLPct, 2),
		roundTo(acct.MarginUsedPct, 1),
		acct.PositionCount,
	)
	positionLine := "Positions: none"
	if len(ctx.Positions) > 0 {
		lines := make([]string, 0, len(ctx.Positions))
		for i, pos := range ctx.Positions {
			value := math.Abs(pos.Quantity) * pos.MarkPrice
			age := describeMinutes(pos.UpdateTime)
			line := fmt.Sprintf(
				"%d. %s %s | Entry %.4f Current %.4f | Qty %.4f | Value %.2f USDT | P&L %+.2f%% (%+.2f USDT) | Max %.2f%% | Lev %dx | Margin %.0f | Liq %.4f | Held %s",
				i+1,
				pos.Symbol,
				strings.ToUpper(pos.Side),
				pos.EntryPrice,
				pos.MarkPrice,
				pos.Quantity,
				value,
				pos.UnrealizedPnLPct,
				pos.UnrealizedPnL,
				pos.PeakPnLPct,
				pos.Leverage,
				pos.MarginUsed,
				pos.LiquidationPrice,
				age,
			)
			lines = append(lines, line)
		}
		positionLine = strings.Join(lines, "\n")
	}
	latestLine := buildLatestLine(ctx)
	return &metaPayload{
		Summary:   summary,
		Account:   accountLine,
		Positions: positionLine,
		Latest:    latestLine,
	}
}

func describeMinutes(updateMs int64) string {
	if updateMs <= 0 {
		return "n/a"
	}
	delta := time.Now().UnixMilli() - updateMs
	if delta < 0 {
		delta = 0
	}
	mins := int(delta / 60000)
	if mins < 1 {
		return "<1 minute"
	}
	if mins < 60 {
		return fmt.Sprintf("%d minutes", mins)
	}
	return fmt.Sprintf("%d hours %d minutes", mins/60, mins%60)
}

func buildLatestLine(ctx *Context) string {
	if ctx == nil || len(ctx.MarketDataMap) == 0 {
		return ""
	}
	var symbol string
	if len(ctx.Positions) > 0 {
		symbol = ctx.Positions[0].Symbol
	}
	if symbol == "" {
		if _, ok := ctx.MarketDataMap["BTCUSDT"]; ok {
			symbol = "BTCUSDT"
		} else {
			for sym := range ctx.MarketDataMap {
				symbol = sym
				break
			}
		}
	}
	data := ctx.MarketDataMap[symbol]
	if data == nil {
		return ""
	}
	return fmt.Sprintf(
		"%s | current_price=%.4f current_ema20=%.3f current_macd=%.3f current_rsi7=%.3f",
		symbol,
		data.CurrentPrice,
		data.CurrentEMA20,
		data.CurrentMACD,
		data.CurrentRSI7,
	)
}

func buildQoS(data *market.Data, key string) (featureQoS, bool) {
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
	qos := featureQoS{
		Coverage:  roundTo(coverage, 3),
		AgeSecond: age,
	}
	pass := coverage >= featureCoverageThreshold && age <= featureSLALimitSeconds
	return qos, pass
}

func validateLivePayload(payload *livePayload) error {
	if payload.Version != PayloadSchemaVersion {
		return fmt.Errorf("invalid payload version %s", payload.Version)
	}
	if payload.Timestamp == "" {
		return fmt.Errorf("timestamp missing")
	}
	if math.IsNaN(payload.Account.Equity) || payload.Account.Equity < 0 {
		return fmt.Errorf("account equity invalid")
	}
	if err := validateAssets(payload.Positions, payload.Candidates); err != nil {
		return err
	}
	for _, cand := range payload.Candidates {
		if cand.Rank < 0 || cand.Rank > 1 {
			return fmt.Errorf("candidate rank out of range for %s", cand.Symbol)
		}
	}
	if payload.Policy.SizeCap <= 0 || payload.Policy.SizeCap > 1 {
		return fmt.Errorf("policy size_cap invalid")
	}
	if payload.Policy.MinConf < 0 || payload.Policy.MinConf > 1 {
		return fmt.Errorf("policy min_conf invalid")
	}
	return nil
}

func validateAssets(positions []positionPayload, candidates []candidatePayload) error {
	for _, pos := range positions {
		if pos.Symbol == "" {
			return fmt.Errorf("position symbol missing")
		}
		if pos.Side != "long" && pos.Side != "short" {
			return fmt.Errorf("position %s side invalid", pos.Symbol)
		}
		if pos.Context != nil && pos.Context.RSI7 != nil {
			if *pos.Context.RSI7 < 0 || *pos.Context.RSI7 > 100 {
				return fmt.Errorf("position %s RSI out of range", pos.Symbol)
			}
		}
		if err := validateFeatureQoS(pos.Features, pos.Symbol); err != nil {
			return err
		}
	}
	for _, cand := range candidates {
		if cand.Symbol == "" {
			return fmt.Errorf("candidate symbol missing")
		}
		if cand.Context != nil && cand.Context.RSI7 != nil {
			if *cand.Context.RSI7 < 0 || *cand.Context.RSI7 > 100 {
				return fmt.Errorf("candidate %s RSI out of range", cand.Symbol)
			}
		}
		if err := validateFeatureQoS(cand.Features, cand.Symbol); err != nil {
			return err
		}
	}
	return nil
}

func validateFeatureQoS(features *featureBlock, symbol string) error {
	if features == nil {
		return nil
	}
	checks := []struct {
		name    string
		payload interface{}
	}{
		{"f4", features.F4},
		{"f5", features.F5},
		{"f6", features.F6},
		{"f7", features.F7},
	}
	for _, item := range checks {
		switch v := item.payload.(type) {
		case *feature4Payload:
			if err := ensureQoS(v.QoS, symbol, item.name); err != nil {
				return err
			}
			if v.Confidence != nil && (*v.Confidence < 0 || *v.Confidence > 1) {
				return fmt.Errorf("%s %s confidence out of range", symbol, item.name)
			}
		case *feature5Payload:
			if err := ensureQoS(v.QoS, symbol, item.name); err != nil {
				return err
			}
			if v.Confidence != nil && (*v.Confidence < 0 || *v.Confidence > 1) {
				return fmt.Errorf("%s %s confidence out of range", symbol, item.name)
			}
			if v.Dist != nil {
				if v.Dist.UpATR != nil && *v.Dist.UpATR < 0 {
					return fmt.Errorf("%s f5 up_atr negative", symbol)
				}
				if v.Dist.DownATR != nil && *v.Dist.DownATR < 0 {
					return fmt.Errorf("%s f5 dn_atr negative", symbol)
				}
			}
		case *feature6Payload:
			if err := ensureQoS(v.QoS, symbol, item.name); err != nil {
				return err
			}
			if v.Confidence != nil && (*v.Confidence < 0 || *v.Confidence > 1) {
				return fmt.Errorf("%s %s confidence out of range", symbol, item.name)
			}
		case *feature7Payload:
			if err := ensureQoS(v.QoS, symbol, item.name); err != nil {
				return err
			}
			if v.Confidence != nil && (*v.Confidence < 0 || *v.Confidence > 1) {
				return fmt.Errorf("%s %s confidence out of range", symbol, item.name)
			}
		}
	}
	return nil
}

func ensureQoS(q featureQoS, symbol, feature string) error {
	if q.Coverage < 0 || q.Coverage > 1 {
		return fmt.Errorf("%s %s coverage out of range", symbol, feature)
	}
	if q.AgeSecond < 0 {
		return fmt.Errorf("%s %s age negative", symbol, feature)
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
