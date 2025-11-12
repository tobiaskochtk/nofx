package market

import (
	"log"
	"math"
	"sort"
	"time"

	"nofx/pkg/types"
)

const (
	microPreferredBars3m   = 480
	microFeature4MinBars   = 120
	microZShortWindow      = 90
	microZLongWindow       = 360
	microSlopeShortWindow  = 20
	microSlopeMidWindow    = 60
	microHeatmapRangeFloor = 0.004 // 0.4% intrabar move treated as potential liquidation spike
	microHeatmapWindowDays = 30
	microHalfLifeDays      = 7
)

// enrichMicrostructureFeatures populates Feature4 & Feature5 fields for 3m + optional 15m scopes.
func enrichMicrostructureFeatures(symbol string, base3m []Kline, dest *types.DerivsFeatures, carrier *Data) error {
	log.Printf("🔄 [Microstructure] %s: ENTRY - base3m len=%d, dest==nil: %v", symbol, len(base3m), dest == nil)
	if dest == nil || len(base3m) == 0 {
		log.Printf("⚠️ [Microstructure] %s: early return (dest==nil: %v, base3m len=%d)", symbol, dest == nil, len(base3m))
		return nil
	}

	bars3m, err := ensureMicrostructureBars(symbol, "3m", base3m, microPreferredBars3m)
	if err != nil {
		log.Printf("⚠️ [Microstructure] %s: unable to extend bars: %v", symbol, err)
	}
	log.Printf("📊 [Microstructure] %s: base3m=%d bars, ensured bars3m=%d (min required: %d)", symbol, len(base3m), len(bars3m), microFeature4MinBars)
	if carrier != nil {
		coverage := math.Min(1, float64(len(bars3m))/float64(microPreferredBars3m))
		lastTs := time.Now().UTC()
		if len(bars3m) > 0 {
			lastTs = time.UnixMilli(bars3m[len(bars3m)-1].CloseTime).UTC()
		}
		carrier.recordFeatureMeta(FeatureKeyF4, coverage, lastTs)
		carrier.recordFeatureMeta(FeatureKeyF5, coverage, lastTs)
	}
	if len(bars3m) < microFeature4MinBars {
		log.Printf("⏳ [Microstructure] %s: insufficient bars (%d/%d)", symbol, len(bars3m), microFeature4MinBars)
		return nil
	}
	log.Printf("✓ [Microstructure] %s: Computing Feature 4 & 5 with %d bars", symbol, len(bars3m))

	computeFeature4ForScope("3m", bars3m, dest)

	resampled15m := resampleKlines(bars3m, 5)
	if len(resampled15m) >= microFeature4MinBars/2 {
		computeFeature4ForScope("15m", resampled15m, dest)
	}

	computeFeature5ForScope("3m", bars3m, dest)
	if len(resampled15m) >= microSlopeMidWindow {
		computeFeature5ForScope("15m", resampled15m, dest)
	}

	return nil
}

func ensureMicrostructureBars(symbol, interval string, existing []Kline, preferred int) ([]Kline, error) {
	if len(existing) >= preferred {
		return existing, nil
	}
	limit := preferred
	if limit > 1500 {
		limit = 1500
	}
	api := NewAPIClient()
	klines, err := api.GetKlines(symbol, interval, limit)
	if err != nil {
		return existing, err
	}
	return klines, nil
}

func computeFeature4ForScope(scope string, bars []Kline, dest *types.DerivsFeatures) {
	if len(bars) < microSlopeMidWindow {
		return
	}
	prices := make([]float64, len(bars))
	buyNotional := make([]float64, len(bars))
	sellNotional := make([]float64, len(bars))
	imbalanceSeries := make([]float64, len(bars))
	cvdSeries := make([]float64, len(bars))

	var cumulative float64
	for i, k := range bars {
		prices[i] = k.Close
		buy := safePositive(k.TakerBuyQuoteVolume)
		sell := safePositive(k.QuoteVolume - k.TakerBuyQuoteVolume)
		buyNotional[i] = buy
		sellNotional[i] = sell
		total := buy + sell
		if total > 0 {
			imbalanceSeries[i] = (buy - sell) / total
		}
		cumulative += buy - sell
		cvdSeries[i] = cumulative
	}

	shortZ := zScore(cvdSeries, microZShortWindow)
	longZ := zScore(cvdSeries, microZLongWindow)
	imbShortZ := zScore(imbalanceSeries, microZShortWindow)
	imbLongZ := zScore(imbalanceSeries, microZLongWindow)
	tbr := takerBuyRatio(buyNotional, sellNotional)

	priceSlopeShort, priceR2Short := slopeAndR2(prices, microSlopeShortWindow)
	priceSlopeMid, priceR2Mid := slopeAndR2(prices, microSlopeMidWindow)
	rollingCVDZ := rollingZSeries(cvdSeries, microZShortWindow)
	rollingImbZ := rollingZSeries(imbalanceSeries, microZShortWindow)
	cvdSlopeShort, cvdR2Short := slopeAndR2(rollingCVDZ, microSlopeShortWindow)
	cvdSlopeMid, cvdR2Mid := slopeAndR2(rollingCVDZ, microSlopeMidWindow)
	imbSlopeShort, imbR2Short := slopeAndR2(rollingImbZ, microSlopeShortWindow)
	imbSlopeMid, imbR2Mid := slopeAndR2(rollingImbZ, microSlopeMidWindow)

	bearShort, bullShort := classifyDivergenceSlope(priceSlopeShort, cvdSlopeShort)
	bearMid, bullMid := classifyDivergenceSlope(priceSlopeMid, cvdSlopeMid)
	confidence := deriveCVDConfidence(len(bars), priceR2Short, cvdR2Short, imbR2Short)

	scopeAssigner := func(fn func()) {
		fn()
	}

	scopeAssigner(func() {
		switch scope {
		case "3m":
			dest.CVDNotionalZ3mShort = floatPtr(shortZ)
			dest.CVDNotionalZ3mLong = floatPtr(longZ)
			dest.ImbNotionalZ3mShort = floatPtr(imbShortZ)
			dest.ImbNotionalZ3mLong = floatPtr(imbLongZ)
			dest.TBRNotional3m = floatPtr(tbr)
			dest.SlopePrice3mShort = floatPtr(priceSlopeShort)
			dest.SlopePrice3mMid = floatPtr(priceSlopeMid)
			dest.R2Price3mShort = floatPtr(priceR2Short)
			dest.R2Price3mMid = floatPtr(priceR2Mid)
			dest.SlopeCVDZ3mShort = floatPtr(cvdSlopeShort)
			dest.SlopeCVDZ3mMid = floatPtr(cvdSlopeMid)
			dest.R2CVDZ3mShort = floatPtr(cvdR2Short)
			dest.R2CVDZ3mMid = floatPtr(cvdR2Mid)
			dest.SlopeImb3mShort = floatPtr(imbSlopeShort)
			dest.SlopeImb3mMid = floatPtr(imbSlopeMid)
			dest.R2Imb3mShort = floatPtr(imbR2Short)
			dest.R2Imb3mMid = floatPtr(imbR2Mid)
			dest.DivBearShort3m = intPtr(bearShort)
			dest.DivBullShort3m = intPtr(bullShort)
			dest.DivBearMid3m = intPtr(bearMid)
			dest.DivBullMid3m = intPtr(bullMid)
			dest.ConfidenceCVD3m = floatPtr(confidence)
		case "15m":
			dest.CVDNotionalZ15mShort = floatPtr(shortZ)
			dest.CVDNotionalZ15mLong = floatPtr(longZ)
			dest.ImbNotionalZ15mShort = floatPtr(imbShortZ)
			dest.ImbNotionalZ15mLong = floatPtr(imbLongZ)
			dest.TBRNotional15m = floatPtr(tbr)
			dest.DivBearMid15m = intPtr(bearMid)
			dest.DivBullMid15m = intPtr(bullMid)
			dest.ConfidenceCVD15m = floatPtr(confidence)
		}
	})
}

func computeFeature5ForScope(scope string, bars []Kline, dest *types.DerivsFeatures) {
	if len(bars) < microSlopeShortWindow {
		return
	}
	atr := calculateATR(bars, 14)
	if atr == 0 {
		return
	}
	currentPrice := bars[len(bars)-1].Close
	events := buildLiquidationEvents(bars)
	if len(events) == 0 {
		return
	}
	upCluster := nearestCluster(events, currentPrice, atr, true)
	downCluster := nearestCluster(events, currentPrice, atr, false)
	avgNotional := averageEventNotional(events)
	totalEvents := len(events)
	clusterStrengthUp := clusterStrength(events, upCluster, atr)
	clusterStrengthDown := clusterStrength(events, downCluster, atr)
	distUpPct, distUpAtr := clusterDistance(currentPrice, atr, upCluster)
	distDnPct, distDnAtr := clusterDistance(currentPrice, atr, downCluster)
	preferDirection := derivePreferenceFromDistances(distUpAtr, distDnAtr)
	confidence := deriveLiqConfidence(totalEvents, clusterStrengthUp, clusterStrengthDown)
	liqRiskUp := riskFlag(distUpAtr)
	liqRiskDown := riskFlag(distDnAtr)

	switch scope {
	case "3m":
		dest.DistUpPct3m = floatPtr(distUpPct)
		dest.DistUpAtr3m = floatPtr(distUpAtr)
		dest.ClusterStrengthUp3m = floatPtr(clusterStrengthUp)
		dest.DistDnPct3m = floatPtr(distDnPct)
		dest.DistDnAtr3m = floatPtr(distDnAtr)
		dest.ClusterStrengthDown3m = floatPtr(clusterStrengthDown)
		dest.LiqRiskUp3m = intPtr(liqRiskUp)
		dest.LiqRiskDown3m = intPtr(liqRiskDown)
		dest.PreferDirection3m = stringPtr(preferDirection)
		dest.BucketUsd3m = floatPtr(avgNotional)
		dest.EventCountLiq3m = intPtr(totalEvents)
		dest.Atr3m = floatPtr(atr)
		dest.ConfidenceLiq3m = floatPtr(confidence)
	case "15m":
		dest.DistUpAtr15m = floatPtr(distUpAtr)
		dest.DistDnAtr15m = floatPtr(distDnAtr)
		dest.PreferDirection15m = stringPtr(preferDirection)
		dest.ConfidenceLiq15m = floatPtr(confidence)
	}
}

func buildLiquidationEvents(bars []Kline) []liqEvent {
	now := time.Now()
	windowStart := now.Add(-microHeatmapWindowDays * 24 * time.Hour)
	halfLife := microHalfLifeDays * 24 * time.Hour
	events := make([]liqEvent, 0, len(bars)/4)
	for _, k := range bars {
		ts := time.UnixMilli(k.CloseTime)
		if ts.Before(windowStart) {
			continue
		}
		notional := safePositive(k.QuoteVolume)
		if notional == 0 {
			continue
		}
		low := math.Max(k.Low, 1e-9)
		rangePct := (k.High - k.Low) / low
		if rangePct < microHeatmapRangeFloor {
			continue
		}
		var direction string
		var price float64
		if k.Close >= k.Open {
			direction = "up"
			price = k.High
		} else {
			direction = "down"
			price = k.Low
		}
		decay := math.Pow(0.5, now.Sub(ts).Hours()/halfLife.Hours())
		weight := notional * rangePct * decay
		events = append(events, liqEvent{
			Price:    price,
			Weight:   weight,
			Notional: notional,
			Dir:      direction,
		})
	}
	return events
}

type liqEvent struct {
	Price    float64
	Weight   float64
	Notional float64
	Dir      string
}

func nearestCluster(events []liqEvent, current, atr float64, wantUp bool) *liqEvent {
	var filtered []liqEvent
	for _, ev := range events {
		if wantUp && ev.Price > current {
			filtered = append(filtered, ev)
		} else if !wantUp && ev.Price < current {
			filtered = append(filtered, ev)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.Slice(filtered, func(i, j int) bool {
		return math.Abs(filtered[i].Price-current) < math.Abs(filtered[j].Price-current)
	})
	return &filtered[0]
}

func clusterStrength(events []liqEvent, target *liqEvent, atr float64) float64 {
	if target == nil || atr == 0 {
		return 0
	}
	sum := 0.0
	for _, ev := range events {
		if math.Abs(ev.Price-target.Price) <= atr {
			sum += ev.Weight
		}
	}
	return sum
}

func averageEventNotional(events []liqEvent) float64 {
	if len(events) == 0 {
		return 0
	}
	sum := 0.0
	for _, ev := range events {
		sum += ev.Notional
	}
	return sum / float64(len(events))
}

func derivePreferenceFromDistances(distUpAtr, distDnAtr float64) string {
	switch {
	case math.IsNaN(distUpAtr) && math.IsNaN(distDnAtr):
		return "neutral"
	case math.IsNaN(distUpAtr):
		return "prefer_longs"
	case math.IsNaN(distDnAtr):
		return "prefer_shorts"
	default:
		if math.Abs(distDnAtr) < math.Abs(distUpAtr) {
			return "prefer_longs"
		}
		return "prefer_shorts"
	}
}

func deriveCVDConfidence(barCount int, r2Price, r2CVD, r2Imb float64) float64 {
	base := math.Min(1, float64(barCount)/float64(microSlopeMidWindow)) * 0.2
	trendQuality := clamp01((r2Price + r2CVD + r2Imb) / 3)
	return clamp01(base + 0.8*trendQuality)
}

func deriveLiqConfidence(events int, strengthUp, strengthDown float64) float64 {
	eventScore := math.Min(1, float64(events)/50)
	strengthScore := clamp01((strengthUp + strengthDown) / 1e6)
	return clamp01(0.5*eventScore + 0.5*strengthScore)
}

func clusterDistance(current, atr float64, cluster *liqEvent) (float64, float64) {
	if cluster == nil || atr == 0 || current == 0 {
		return math.NaN(), math.NaN()
	}
	delta := cluster.Price - current
	return (delta / current) * 100, delta / atr
}

func riskFlag(distATR float64) int {
	if math.IsNaN(distATR) {
		return 0
	}
	if math.Abs(distATR) <= 1 {
		return 1
	}
	return 0
}

func zScore(series []float64, window int) float64 {
	if len(series) == 0 {
		return math.NaN()
	}
	if window <= 0 || window > len(series) {
		window = len(series)
	}
	sub := series[len(series)-window:]
	avg := meanFloat(sub)
	std := stddevFloat(sub, avg)
	if std == 0 {
		return 0
	}
	return (sub[len(sub)-1] - avg) / std
}

func takerBuyRatio(buy, sell []float64) float64 {
	if len(buy) == 0 {
		return math.NaN()
	}
	lastBuy := buy[len(buy)-1]
	lastSell := sell[len(sell)-1]
	total := lastBuy + lastSell
	if total == 0 {
		return math.NaN()
	}
	return lastBuy / total
}

func slopeAndR2(series []float64, window int) (float64, float64) {
	if len(series) < window || window < 2 {
		return math.NaN(), math.NaN()
	}
	segment := series[len(series)-window:]
	return linearRegression(segment)
}

func linearRegression(y []float64) (float64, float64) {
	n := len(y)
	if n < 2 {
		return math.NaN(), math.NaN()
	}
	var sumX, sumY, sumXY, sumXX float64
	for i, val := range y {
		x := float64(i)
		sumX += x
		sumY += val
		sumXY += x * val
		sumXX += x * x
	}
	denom := float64(n)*sumXX - sumX*sumX
	if denom == 0 {
		return 0, 0
	}
	slope := (float64(n)*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / float64(n)
	var ssTot, ssRes float64
	meanY := sumY / float64(n)
	for i, val := range y {
		pred := slope*float64(i) + intercept
		ssRes += math.Pow(val-pred, 2)
		ssTot += math.Pow(val-meanY, 2)
	}
	var r2 float64
	if ssTot == 0 {
		r2 = 0
	} else {
		r2 = 1 - ssRes/ssTot
	}
	return slope, clamp01(r2)
}

func rollingZSeries(series []float64, window int) []float64 {
	if len(series) == 0 {
		return nil
	}
	out := make([]float64, len(series))
	for i := range series {
		w := window
		if i+1 < w {
			w = i + 1
		}
		sub := series[i+1-w : i+1]
		out[i] = zScore(sub, len(sub))
	}
	return out
}

func classifyDivergenceSlope(priceSlope, cvdSlope float64) (int, int) {
	bearish := 0
	bullish := 0
	if !math.IsNaN(priceSlope) && !math.IsNaN(cvdSlope) {
		if priceSlope > 0 && cvdSlope < 0 {
			bearish = 1
		}
		if priceSlope < 0 && cvdSlope > 0 {
			bullish = 1
		}
	}
	return bearish, bullish
}

func meanFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stddevFloat(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += math.Pow(v-mean, 2)
	}
	return math.Sqrt(sum / float64(len(values)))
}

func safePositive(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	return v
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
