package market

import (
	"log"
	"math"
	"sort"
	"time"

	"nofx/pkg/types"
)

const (
	structuralPreferredBars3m    = 600
	structuralMinBars3m          = 240
	structuralMinBars15m         = 80
	structuralBandWindow         = 200
	structuralSwingLookback      = 120
	structuralCrossWindow        = 10
	structuralPercentileLookback = 200
	structuralRvShortWindow      = 20
	structuralRvLongWindow       = 160
)

type anchorMetric struct {
	name     string
	index    int
	price    float64
	sigma    float64
	anchorTs time.Time
}

type avwapResult struct {
	upAnchor      *anchorMetric
	downAnchor    *anchorMetric
	reclaimUp     int
	rejectionDown int
	bullCount     int
	bearCount     int
	bias          string
	confidence    float64
}

type volResult struct {
	bbwRatio           float64
	kcWidthRatio       float64
	bbwPctRank         float64
	squeezeOn          int
	squeezePersistence int
	squeezeRelease     int
	expansionRate      float64
	rvRatio            float64
	rvRatioZ           float64
	regime             string
	confidence         float64
}

func enrichStructuralFeatures(symbol string, base3m []Kline, dest *types.DerivsFeatures, carrier *Data) error {
	if dest == nil || len(base3m) == 0 {
		return nil
	}

	bars3m, err := ensureStructuralBars(symbol, "3m", base3m, structuralPreferredBars3m)
	if err != nil && len(bars3m) == 0 {
		return err
	}

	log.Printf("📊 [Structural] %s: base3m=%d bars, fetched=%d bars (min required: %d)",
		symbol, len(base3m), len(bars3m), structuralMinBars3m)

	if carrier != nil {
		coverage := math.Min(1, float64(len(bars3m))/float64(structuralPreferredBars3m))
		lastTs := time.Now().UTC()
		if len(bars3m) > 0 {
			lastTs = time.UnixMilli(bars3m[len(bars3m)-1].CloseTime).UTC()
		}
		carrier.recordFeatureMeta(FeatureKeyF6, coverage, lastTs)
		carrier.recordFeatureMeta(FeatureKeyF7, coverage, lastTs)
	}

	if len(bars3m) >= structuralMinBars3m {
		enrichAVWAP("3m", bars3m, dest)
		enrichVolatility("3m", bars3m, dest)
		log.Printf("✓ [Structural] %s: AVWAP & Volatility features computed for 3m", symbol)
	} else {
		log.Printf("⏳ [Structural] %s: Insufficient bars (%d/%d) - features will be null",
			symbol, len(bars3m), structuralMinBars3m)
	}

	resampled15m := resampleKlines(bars3m, 5)
	if len(resampled15m) >= structuralMinBars15m {
		enrichAVWAP("15m", resampled15m, dest)
		enrichVolatility("15m", resampled15m, dest)
	}
	return nil
}

func ensureStructuralBars(symbol, interval string, existing []Kline, preferred int) ([]Kline, error) {
	if len(existing) >= preferred {
		log.Printf("📈 [Structural] %s: Using existing %d bars (preferred: %d)", symbol, len(existing), preferred)
		return existing, nil
	}
	limit := preferred
	if limit < preferred {
		limit = preferred
	}
	if limit > 1500 {
		limit = 1500
	}
	log.Printf("🔄 [Structural] %s: Fetching %d bars from API (have: %d, need: %d)",
		symbol, limit, len(existing), preferred)
	api := NewAPIClient()
	klines, err := api.GetKlines(symbol, interval, limit)
	if err != nil {
		log.Printf("❌ [Structural] %s: API fetch failed: %v", symbol, err)
		return existing, err
	}
	log.Printf("✓ [Structural] %s: Fetched %d bars from API", symbol, len(klines))
	return klines, nil
}

func enrichAVWAP(scope string, bars []Kline, dest *types.DerivsFeatures) {
	if len(bars) == 0 {
		return
	}
	atr := calculateATR(bars, 14)
	if atr <= 0 {
		return
	}
	anchors := collectAnchors(bars)
	if len(anchors) == 0 {
		return
	}
	currentPrice := bars[len(bars)-1].Close
	result := summarizeAnchors(bars, anchors, currentPrice, atr)
	if result == nil {
		return
	}

	switch scope {
	case "3m":
		assignAvwapFields3m(result, dest, atr, currentPrice)
	case "15m":
		assignAvwapFields15m(result, dest, atr, currentPrice)
	}
}

func assignAvwapFields3m(res *avwapResult, dest *types.DerivsFeatures, atr, price float64) {
	if res.upAnchor != nil {
		dest.AVWAPUpName3m = stringPtr(res.upAnchor.name)
		dest.AVWAPUpPrice3m = floatPtr(res.upAnchor.price)
		dest.AVWAPUpDistAtr3m = floatPtr(distanceInAtr(res.upAnchor.price, price, atr))
		dest.AVWAPUpBand1DistAtr3m = floatPtr(distanceInAtr(res.upAnchor.price+res.upAnchor.sigma, price, atr))
	}
	if res.downAnchor != nil {
		dest.AVWAPDnName3m = stringPtr(res.downAnchor.name)
		dest.AVWAPDnPrice3m = floatPtr(res.downAnchor.price)
		dest.AVWAPDnDistAtr3m = floatPtr(distanceInAtr(res.downAnchor.price, price, atr))
		dest.AVWAPDnBand1DistAtr3m = floatPtr(distanceInAtr(res.downAnchor.price-res.downAnchor.sigma, price, atr))
	}
	dest.AVWAPReclaimUp3m = intPtr(res.reclaimUp)
	dest.AVWAPRejectionDn3m = intPtr(res.rejectionDown)
	dest.AVWAPConfluenceBull3m = intPtr(res.bullCount)
	dest.AVWAPConfluenceBear3m = intPtr(res.bearCount)
	if res.bias != "" {
		dest.AVWAPBias3m = stringPtr(res.bias)
	}
	dest.ConfidenceAVWAP3m = floatPtr(res.confidence)
}

func assignAvwapFields15m(res *avwapResult, dest *types.DerivsFeatures, atr, price float64) {
	if res.upAnchor != nil {
		dest.AVWAPUpName15m = stringPtr(res.upAnchor.name)
		dest.AVWAPUpDistAtr15m = floatPtr(distanceInAtr(res.upAnchor.price, price, atr))
	}
	if res.downAnchor != nil {
		dest.AVWAPDnName15m = stringPtr(res.downAnchor.name)
		dest.AVWAPDnDistAtr15m = floatPtr(distanceInAtr(res.downAnchor.price, price, atr))
	}
	dest.AVWAPReclaimUp15m = intPtr(res.reclaimUp)
	dest.AVWAPRejectionDn15m = intPtr(res.rejectionDown)
	if res.bias != "" {
		dest.AVWAPBias15m = stringPtr(res.bias)
	}
	dest.ConfidenceAVWAP15m = floatPtr(res.confidence)
}

func distanceInAtr(anchorPrice, price, atr float64) float64 {
	if atr <= 0 {
		return 0
	}
	return math.Abs(anchorPrice-price) / atr
}

func collectAnchors(bars []Kline) []*anchorMetric {
	if len(bars) == 0 {
		return nil
	}
	lastTs := time.UnixMilli(bars[len(bars)-1].CloseTime).UTC()
	var anchors []*anchorMetric
	if a := anchorFromTime("DAILY_OPEN", bars, startOfDay(lastTs)); a != nil {
		anchors = append(anchors, a)
	}
	if a := anchorFromTime("WEEK_OPEN", bars, startOfWeek(lastTs)); a != nil {
		anchors = append(anchors, a)
	}
	if a := anchorFromTime("MONTH_OPEN", bars, startOfMonth(lastTs)); a != nil {
		anchors = append(anchors, a)
	}
	if a := anchorFromExtreme("ATH", bars, true); a != nil {
		anchors = append(anchors, a)
	}
	if a := anchorFromExtreme("ATL", bars, false); a != nil {
		anchors = append(anchors, a)
	}
	if a := anchorFromPeriodExtreme("YTD_HI", bars, startOfYear(lastTs), true); a != nil {
		anchors = append(anchors, a)
	}
	if a := anchorFromPeriodExtreme("YTD_LO", bars, startOfYear(lastTs), false); a != nil {
		anchors = append(anchors, a)
	}
	if a := anchorFromSwing("SWING_HI", bars, true); a != nil {
		anchors = append(anchors, a)
	}
	if a := anchorFromSwing("SWING_LO", bars, false); a != nil {
		anchors = append(anchors, a)
	}
	return anchors
}

func anchorFromTime(name string, bars []Kline, ts time.Time) *anchorMetric {
	target := ts.UnixMilli()
	idx := -1
	for i := range bars {
		if bars[i].OpenTime >= target {
			idx = i
			break
		}
	}
	if idx == -1 || idx >= len(bars)-5 {
		return nil
	}
	price, sigma := computeAnchoredVWAP(bars, idx)
	return &anchorMetric{
		name:     name,
		index:    idx,
		price:    price,
		sigma:    sigma,
		anchorTs: time.UnixMilli(bars[idx].OpenTime).UTC(),
	}
}

func anchorFromExtreme(name string, bars []Kline, wantHigh bool) *anchorMetric {
	bestIdx := -1
	bestVal := math.Inf(-1)
	if !wantHigh {
		bestVal = math.Inf(1)
	}
	for i, k := range bars {
		value := k.High
		if !wantHigh {
			value = k.Low
		}
		if wantHigh && value > bestVal {
			bestVal = value
			bestIdx = i
		}
		if !wantHigh && value < bestVal {
			bestVal = value
			bestIdx = i
		}
	}
	if bestIdx == -1 || bestIdx >= len(bars)-5 {
		return nil
	}
	price, sigma := computeAnchoredVWAP(bars, bestIdx)
	return &anchorMetric{
		name:     name,
		index:    bestIdx,
		price:    price,
		sigma:    sigma,
		anchorTs: time.UnixMilli(bars[bestIdx].OpenTime).UTC(),
	}
}

func anchorFromPeriodExtreme(name string, bars []Kline, start time.Time, wantHigh bool) *anchorMetric {
	target := start.UnixMilli()
	startIdx := -1
	for i := range bars {
		if bars[i].OpenTime >= target {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return nil
	}
	bestIdx := startIdx
	bestVal := bars[startIdx].High
	if !wantHigh {
		bestVal = bars[startIdx].Low
	}
	for i := startIdx; i < len(bars); i++ {
		val := bars[i].High
		if !wantHigh {
			val = bars[i].Low
		}
		if wantHigh && val > bestVal {
			bestVal = val
			bestIdx = i
		}
		if !wantHigh && val < bestVal {
			bestVal = val
			bestIdx = i
		}
	}
	if bestIdx >= len(bars)-2 {
		bestIdx = len(bars) - 2
	}
	price, sigma := computeAnchoredVWAP(bars, bestIdx)
	return &anchorMetric{
		name:     name,
		index:    bestIdx,
		price:    price,
		sigma:    sigma,
		anchorTs: time.UnixMilli(bars[bestIdx].OpenTime).UTC(),
	}
}

func anchorFromSwing(name string, bars []Kline, high bool) *anchorMetric {
	if len(bars) < structuralSwingLookback {
		return nil
	}
	start := len(bars) - structuralSwingLookback
	if start < 0 {
		start = 0
	}
	targetIdx := start
	targetVal := bars[start].Close
	for i := start; i < len(bars); i++ {
		val := bars[i].High
		if !high {
			val = bars[i].Low
		}
		if high && val > targetVal {
			targetVal = val
			targetIdx = i
		}
		if !high && val < targetVal {
			targetVal = val
			targetIdx = i
		}
	}
	if targetIdx >= len(bars)-2 {
		targetIdx = len(bars) - 2
	}
	price, sigma := computeAnchoredVWAP(bars, targetIdx)
	return &anchorMetric{
		name:     name,
		index:    targetIdx,
		price:    price,
		sigma:    sigma,
		anchorTs: time.UnixMilli(bars[targetIdx].OpenTime).UTC(),
	}
}

func computeAnchoredVWAP(bars []Kline, startIdx int) (float64, float64) {
	if startIdx < 0 || startIdx >= len(bars) {
		return 0, 0
	}
	var sumPV float64
	var sumV float64
	for i := startIdx; i < len(bars); i++ {
		v := bars[i].Volume
		if v <= 0 {
			v = 1
		}
		sumPV += bars[i].Close * v
		sumV += v
	}
	if sumV == 0 {
		return bars[len(bars)-1].Close, 0
	}
	avwap := sumPV / sumV
	var sumDiffSq float64
	count := 0
	for i := startIdx; i < len(bars); i++ {
		diff := bars[i].Close - avwap
		sumDiffSq += diff * diff
		count++
	}
	sigma := 0.0
	if count > 1 {
		sigma = math.Sqrt(sumDiffSq / float64(count-1))
	}
	return avwap, sigma
}

func summarizeAnchors(bars []Kline, anchors []*anchorMetric, currentPrice, atr float64) *avwapResult {
	var above []*anchorMetric
	var below []*anchorMetric
	minDist := math.MaxFloat64
	for _, a := range anchors {
		if a == nil || math.IsNaN(a.price) || math.IsInf(a.price, 0) {
			continue
		}
		dist := math.Abs(a.price-currentPrice) / atr
		if dist < minDist {
			minDist = dist
		}
		if a.price >= currentPrice {
			above = append(above, a)
		} else {
			below = append(below, a)
		}
	}
	if len(above) == 0 && len(below) == 0 {
		return nil
	}
	upAnchor := nearestAnchor(above, currentPrice, true)
	downAnchor := nearestAnchor(below, currentPrice, false)
	reclaim := 0
	rejection := 0
	if upAnchor != nil {
		reclaim = detectCross(bars, upAnchor.price, structuralCrossWindow, true)
	}
	if downAnchor != nil {
		rejection = detectCross(bars, downAnchor.price, structuralCrossWindow, false)
	}
	bull, bear := confluenceCounts(anchors, currentPrice, atr)
	bias := "neutral"
	if bull > bear+1 {
		bias = "prefer_longs"
	} else if bear > bull+1 {
		bias = "prefer_shorts"
	}
	confidence := deriveAvwapConfidence(len(anchors), minDist)

	return &avwapResult{
		upAnchor:      upAnchor,
		downAnchor:    downAnchor,
		reclaimUp:     reclaim,
		rejectionDown: rejection,
		bullCount:     bull,
		bearCount:     bear,
		bias:          bias,
		confidence:    confidence,
	}
}

func deriveAvwapConfidence(anchorCount int, minDist float64) float64 {
	if anchorCount == 0 {
		return 0
	}
	score := 0.2
	score += math.Min(0.6, float64(anchorCount)/10.0)
	if minDist < 0.5 {
		score += 0.2
	} else if minDist < 1.0 {
		score += 0.1
	}
	return math.Max(0, math.Min(1, score))
}

func nearestAnchor(list []*anchorMetric, price float64, above bool) *anchorMetric {
	if len(list) == 0 {
		return nil
	}
	sort.Slice(list, func(i, j int) bool {
		if above {
			return list[i].price < list[j].price
		}
		return list[i].price > list[j].price
	})
	best := list[0]
	bestDist := math.Abs(best.price - price)
	for _, a := range list {
		dist := math.Abs(a.price - price)
		if dist < bestDist {
			best = a
			bestDist = dist
		}
	}
	return best
}

func detectCross(bars []Kline, level float64, window int, crossUp bool) int {
	if len(bars) < 2 {
		return 0
	}
	start := len(bars) - window
	if start < 1 {
		start = 1
	}
	for i := start; i < len(bars); i++ {
		prev := bars[i-1].Close
		curr := bars[i].Close
		if crossUp && prev <= level && curr > level {
			return 1
		}
		if !crossUp && prev >= level && curr < level {
			return 1
		}
	}
	return 0
}

func confluenceCounts(anchors []*anchorMetric, price, atr float64) (int, int) {
	if atr <= 0 {
		return 0, 0
	}
	var bull, bear int
	for _, a := range anchors {
		if a == nil {
			continue
		}
		dist := math.Abs(a.price-price) / atr
		if dist > 0.5 {
			continue
		}
		if a.price <= price {
			bull++
		} else {
			bear++
		}
	}
	return bull, bear
}

func startOfDay(ts time.Time) time.Time {
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
}

func startOfWeek(ts time.Time) time.Time {
	dayStart := startOfDay(ts)
	offset := int(dayStart.Weekday() - time.Monday)
	if offset < 0 {
		offset = 6
	}
	return dayStart.AddDate(0, 0, -offset)
}

func startOfMonth(ts time.Time) time.Time {
	return time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func startOfYear(ts time.Time) time.Time {
	return time.Date(ts.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
}

func resampleKlines(bars []Kline, group int) []Kline {
	if group <= 1 || len(bars) < group {
		return nil
	}
	offset := len(bars) % group
	resampled := make([]Kline, 0, len(bars)/group)
	for i := offset; i+group <= len(bars); i += group {
		chunk := bars[i : i+group]
		resampled = append(resampled, combineChunk(chunk))
	}
	return resampled
}

func combineChunk(chunk []Kline) Kline {
	if len(chunk) == 0 {
		return Kline{}
	}
	open := chunk[0]
	close := chunk[len(chunk)-1]
	high := chunk[0].High
	low := chunk[0].Low
	var volume, quote, takerBase, takerQuote float64
	for _, k := range chunk {
		if k.High > high {
			high = k.High
		}
		if k.Low < low {
			low = k.Low
		}
		volume += k.Volume
		quote += k.QuoteVolume
		takerBase += k.TakerBuyBaseVolume
		takerQuote += k.TakerBuyQuoteVolume
	}
	return Kline{
		OpenTime:            open.OpenTime,
		Open:                open.Open,
		High:                high,
		Low:                 low,
		Close:               close.Close,
		CloseTime:           close.CloseTime,
		Volume:              volume,
		QuoteVolume:         quote,
		TakerBuyBaseVolume:  takerBase,
		TakerBuyQuoteVolume: takerQuote,
		Trades:              close.Trades,
	}
}

func enrichVolatility(scope string, bars []Kline, dest *types.DerivsFeatures) {
	if len(bars) < 30 {
		return
	}
	result := computeVolatilityMetrics(bars)
	if result == nil {
		return
	}
	switch scope {
	case "3m":
		dest.BBW3m = floatPtr(result.bbwRatio)
		dest.KCWidth3m = floatPtr(result.kcWidthRatio)
		dest.BBWPctRank3m = floatPtr(result.bbwPctRank * 100)
		dest.SqueezeOn3m = intPtr(result.squeezeOn)
		dest.SqueezePersist3m = intPtr(result.squeezePersistence)
		dest.SqueezeRelease3m = intPtr(result.squeezeRelease)
		dest.BBWExpansionRate3m = floatPtr(result.expansionRate)
		dest.RvRatio3m = floatPtr(result.rvRatio)
		dest.RvRatioZ3m = floatPtr(result.rvRatioZ)
		if result.regime != "" {
			dest.VolRegime3m = stringPtr(result.regime)
		}
		dest.ConfidenceVol3m = floatPtr(result.confidence)
	case "15m":
		dest.SqueezeOn15m = intPtr(result.squeezeOn)
		dest.SqueezeRelease15m = intPtr(result.squeezeRelease)
		dest.BBWPctRank15m = floatPtr(result.bbwPctRank * 100)
		dest.RvRatio15m = floatPtr(result.rvRatio)
		if result.regime != "" {
			dest.VolRegime15m = stringPtr(result.regime)
		}
		dest.ConfidenceVol15m = floatPtr(result.confidence)
	}
}

func computeVolatilityMetrics(bars []Kline) *volResult {
	closes := extractCloses(bars)
	if len(closes) < 30 {
		return nil
	}
	bbwSeries, kcSeries := buildWidthSeries(bars)
	if len(bbwSeries) == 0 || len(kcSeries) == 0 {
		return nil
	}
	currentBBW := bbwSeries[len(bbwSeries)-1]
	currentKC := kcSeries[len(kcSeries)-1]
	squeezeOn := 0
	if currentBBW < currentKC {
		squeezeOn = 1
	}
	persistence := squeezeStreak(bbwSeries, kcSeries)
	release := squeezeReleaseFlag(bbwSeries, kcSeries)
	pctRank := percentileRank(bbwSeries, structuralPercentileLookback)
	expansion := expansionRate(bbwSeries)
	rvRatio, rvZ := realizedVolRatio(closes)
	regime := classifyRegime(squeezeOn, pctRank, expansion, rvRatio)
	confidence := deriveVolConfidence(len(bars), rvRatio, rvZ)

	return &volResult{
		bbwRatio:           ratioSafe(currentBBW),
		kcWidthRatio:       ratioSafe(currentKC),
		bbwPctRank:         pctRank,
		squeezeOn:          squeezeOn,
		squeezePersistence: persistence,
		squeezeRelease:     release,
		expansionRate:      expansion,
		rvRatio:            rvRatio,
		rvRatioZ:           rvZ,
		regime:             regime,
		confidence:         confidence,
	}
}

func extractCloses(bars []Kline) []float64 {
	out := make([]float64, len(bars))
	for i, k := range bars {
		out[i] = k.Close
	}
	return out
}

func buildWidthSeries(bars []Kline) ([]float64, []float64) {
	if len(bars) < 25 {
		return nil, nil
	}
	var bbwSeries []float64
	var kcSeries []float64
	for i := 20; i < len(bars); i++ {
		window := bars[i-19 : i+1]
		sma := simpleMovingAverage(window)
		std := standardDeviation(window, sma)
		if sma == 0 {
			continue
		}
		bbw := (4 * std) / sma
		kc := keltnerWidth(window, sma)
		bbwSeries = append(bbwSeries, bbw)
		kcSeries = append(kcSeries, kc)
	}
	return bbwSeries, kcSeries
}

func simpleMovingAverage(bars []Kline) float64 {
	sum := 0.0
	for _, k := range bars {
		sum += k.Close
	}
	return sum / float64(len(bars))
}

func standardDeviation(bars []Kline, mean float64) float64 {
	if len(bars) <= 1 {
		return 0
	}
	sum := 0.0
	for _, k := range bars {
		diff := k.Close - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(bars)))
}

func keltnerWidth(bars []Kline, mid float64) float64 {
	if len(bars) < 2 || mid == 0 {
		return 0
	}
	trs := make([]float64, len(bars))
	for i := 1; i < len(bars); i++ {
		high := bars[i].High
		low := bars[i].Low
		prevClose := bars[i-1].Close
		trs[i] = math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
	}
	ema := emaSeries(trs[1:], 20)
	return (2 * 1.5 * ema) / mid
}

func emaSeries(values []float64, period int) float64 {
	if len(values) == 0 {
		return 0
	}
	alpha := 2.0 / float64(period+1)
	ema := values[0]
	for i := 1; i < len(values); i++ {
		ema = alpha*values[i] + (1-alpha)*ema
	}
	return ema
}

func squeezeStreak(bbw, kc []float64) int {
	streak := 0
	for i := len(bbw) - 1; i >= 0; i-- {
		if bbw[i] < kc[i] {
			streak++
		} else {
			break
		}
	}
	return streak
}

func squeezeReleaseFlag(bbw, kc []float64) int {
	if len(bbw) < 2 {
		return 0
	}
	current := bbw[len(bbw)-1] < kc[len(kc)-1]
	prev := bbw[len(bbw)-2] < kc[len(kc)-2]
	if !current && prev {
		return 1
	}
	return 0
}

func percentileRank(series []float64, lookback int) float64 {
	if len(series) == 0 {
		return 0
	}
	start := len(series) - lookback
	if start < 0 {
		start = 0
	}
	window := series[start:]
	current := window[len(window)-1]
	count := 0
	for _, v := range window {
		if v <= current {
			count++
		}
	}
	return float64(count) / float64(len(window))
}

func expansionRate(series []float64) float64 {
	if len(series) == 0 {
		return 0
	}
	start := len(series) - structuralPercentileLookback
	if start < 0 {
		start = 0
	}
	window := series[start:]
	minVal := math.MaxFloat64
	for _, v := range window {
		if v < minVal {
			minVal = v
		}
	}
	if minVal == 0 {
		return 0
	}
	return series[len(series)-1]/minVal - 1
}

func realizedVolRatio(closes []float64) (float64, float64) {
	if len(closes) < structuralRvLongWindow+1 {
		return 0, 0
	}
	shortVol := realizedVol(closes[len(closes)-structuralRvShortWindow:])
	longVol := realizedVol(closes[len(closes)-structuralRvLongWindow:])
	if longVol == 0 {
		return 0, 0
	}
	ratio := shortVol / longVol
	z := ratioZScore(closes)
	return ratio, z
}

func realizedVol(closes []float64) float64 {
	if len(closes) < 2 {
		return 0
	}
	var returns []float64
	for i := 1; i < len(closes); i++ {
		if closes[i-1] == 0 {
			continue
		}
		ret := math.Log(closes[i] / closes[i-1])
		returns = append(returns, ret)
	}
	if len(returns) == 0 {
		return 0
	}
	mean := mean(returns)
	var sum float64
	for _, r := range returns {
		diff := r - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(returns)))
}

func ratioZScore(closes []float64) float64 {
	if len(closes) < structuralRvLongWindow+structuralRvShortWindow {
		return 0
	}
	var ratios []float64
	for i := structuralRvLongWindow; i < len(closes); i++ {
		shortSlice := closes[i-structuralRvShortWindow+1 : i+1]
		longSlice := closes[i-structuralRvLongWindow+1 : i+1]
		shortVol := realizedVol(shortSlice)
		longVol := realizedVol(longSlice)
		if longVol == 0 {
			continue
		}
		ratios = append(ratios, shortVol/longVol)
	}
	if len(ratios) < 2 {
		return 0
	}
	current := ratios[len(ratios)-1]
	m := mean(ratios)
	std := stddev(ratios, m)
	if std == 0 {
		return 0
	}
	return (current - m) / std
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64, mean float64) float64 {
	if len(vals) <= 1 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		diff := v - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(vals)))
}

func classifyRegime(squeezeOn int, pctRank, expansion, rvRatio float64) string {
	if squeezeOn == 1 || pctRank < 0.3 {
		return "compression"
	}
	if expansion > 0.25 || rvRatio > 1.3 {
		return "expansion"
	}
	return "balanced"
}

func deriveVolConfidence(barCount int, ratio, z float64) float64 {
	conf := 0.0
	if barCount >= structuralRvLongWindow {
		conf += 0.5
	}
	if ratio > 0 && !math.IsNaN(z) {
		conf += 0.5 * math.Max(0, 1-math.Min(math.Abs(ratio-1), 1))
	}
	if conf > 1 {
		conf = 1
	}
	return conf
}

func ratioSafe(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return val
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	val := v
	return &val
}

func floatPtr(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	val := v
	return &val
}

func intPtr(v int) *int {
	val := v
	return &val
}
