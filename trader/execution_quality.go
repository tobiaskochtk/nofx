package trader

import (
	"math"
	"sort"
	"strings"

	"nofx/kernel"
	"nofx/logger"
)

const (
	executionQualityDepthLevels    = 15
	executionQualityCandidateLimit = 12
	executionDepthBandPct          = 0.01
	executionDepthTargetUSD        = 5000.0
	executionSpreadTargetBps       = 20.0
	executionSlippageTargetBps     = 20.0
	executionSmallNotionalUSD      = 25.0
	executionMediumNotionalUSD     = 100.0
)

type orderBookFetcher interface {
	GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error)
}

type minNotionalGetter interface {
	GetMinNotional(symbol string) float64
}

func (at *AutoTrader) collectExecutionSignals(
	positionInfos []kernel.PositionInfo,
	candidateCoins []kernel.CandidateCoin,
) (map[string]*kernel.ExecutionQuality, map[string]*kernel.VenueTradability, []kernel.CandidateCoin) {
	fetcher, hasOrderBook := at.trader.(orderBookFetcher)
	minGetter, hasMinNotional := at.trader.(minNotionalGetter)

	filteredCandidateCoins, cachedUnsupported := at.filterCachedUnsupportedCandidateCoins(candidateCoins)
	symbols := make([]string, 0, len(positionInfos)+minInt(len(filteredCandidateCoins), executionQualityCandidateLimit))
	seen := make(map[string]struct{}, len(positionInfos)+len(candidateCoins))
	candidateSymbolSet := make(map[string]struct{}, minInt(len(filteredCandidateCoins), executionQualityCandidateLimit))

	appendSymbol := func(symbol string) {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" {
			return
		}
		if _, exists := seen[symbol]; exists {
			return
		}
		symbols = append(symbols, symbol)
		seen[symbol] = struct{}{}
	}

	for _, pos := range positionInfos {
		appendSymbol(pos.Symbol)
	}
	for i, coin := range filteredCandidateCoins {
		if i >= executionQualityCandidateLimit {
			break
		}
		symbol := strings.ToUpper(strings.TrimSpace(coin.Symbol))
		if symbol == "" {
			continue
		}
		candidateSymbolSet[symbol] = struct{}{}
		appendSymbol(symbol)
	}

	if len(symbols) == 0 {
		return nil, nil, filteredCandidateCoins
	}

	logger.Infof("📚 [%s] Fetching execution signals for %d symbols...", at.name, len(symbols))

	executionOut := make(map[string]*kernel.ExecutionQuality, len(symbols))
	venueOut := make(map[string]*kernel.VenueTradability, len(symbols))
	freshUnsupported := make(map[string]struct{})
	for _, symbol := range symbols {
		venue := &kernel.VenueTradability{
			VenueSupported: false,
			OrderBookState: "venue_unsupported",
			PriceSource:    "none",
		}

		price, err := at.trader.GetMarketPrice(symbol)
		if err != nil {
			venue.OrderBookState = classifyVenueError(err)
			venueOut[symbol] = venue
			if venue.OrderBookState == "venue_unsupported" {
				if _, isCandidate := candidateSymbolSet[symbol]; isCandidate {
					at.markUnsupportedCandidateSymbol(symbol)
					freshUnsupported[symbol] = struct{}{}
					continue
				}
			}
			logger.Infof("⚠️ [%s] Venue unavailable for %s: %v", at.name, symbol, err)
			continue
		}
		if price > 0 {
			venue.PriceSource = "exchange"
		}
		at.clearUnsupportedCandidateSymbol(symbol)
		venue.VenueSupported = true
		venue.OrderBookState = "not_exposed"

		if hasMinNotional {
			minNotional := minGetter.GetMinNotional(symbol)
			if minNotional > 0 {
				ok := executionSmallNotionalUSD >= minNotional
				venue.MinNotionalOK = &ok
			}
		}

		if !hasOrderBook {
			venueOut[symbol] = venue
			continue
		}

		bids, asks, err := fetcher.GetOrderBook(symbol, executionQualityDepthLevels)
		if err != nil {
			venue.OrderBookState = classifyVenueError(err)
			venueOut[symbol] = venue
			logger.Infof("⚠️ [%s] Failed to get order book for %s: %v", at.name, symbol, err)
			continue
		}
		summary := summarizeExecutionQuality(bids, asks)
		if summary == nil {
			venue.OrderBookState = "empty"
			venueOut[symbol] = venue
			continue
		}
		venue.OrderBookState = "ok"
		executionOut[symbol] = summary
		venueOut[symbol] = venue
	}

	if len(executionOut) > 0 {
		logger.Infof("📚 [%s] Execution quality ready for %d symbols", at.name, len(executionOut))
	}
	if len(venueOut) > 0 {
		logger.Infof("📚 [%s] Venue tradability ready for %d symbols", at.name, len(venueOut))
	}
	removedSymbols := make(map[string]struct{}, len(cachedUnsupported)+len(freshUnsupported))
	for _, symbol := range cachedUnsupported {
		removedSymbols[symbol] = struct{}{}
	}
	for symbol := range freshUnsupported {
		removedSymbols[symbol] = struct{}{}
	}
	if len(removedSymbols) > 0 {
		filteredCandidateCoins = filterUnsupportedCandidateCoins(filteredCandidateCoins, removedSymbols)
		logger.Infof("🧹 [%s] Removed %d venue-unsupported candidate(s): %s", at.name, len(removedSymbols), strings.Join(sortedSymbolKeys(removedSymbols), ", "))
	}

	if len(executionOut) == 0 {
		executionOut = nil
	}
	if len(venueOut) == 0 {
		venueOut = nil
	}

	return executionOut, venueOut, filteredCandidateCoins
}

func summarizeExecutionQuality(bids, asks [][]float64) *kernel.ExecutionQuality {
	normalizedBids := normalizeBookLevels(bids, true)
	normalizedAsks := normalizeBookLevels(asks, false)
	if len(normalizedBids) == 0 || len(normalizedAsks) == 0 {
		return nil
	}

	bestBid := normalizedBids[0][0]
	bestAsk := normalizedAsks[0][0]
	if bestBid <= 0 || bestAsk <= 0 || bestAsk < bestBid {
		return nil
	}

	mid := (bestBid + bestAsk) / 2
	if mid <= 0 {
		return nil
	}

	spreadBps := ((bestAsk - bestBid) / mid) * 10_000
	bidDepth := depthWithinBandUSD(normalizedBids, bestBid*(1-executionDepthBandPct), bestBid)
	askDepth := depthWithinBandUSD(normalizedAsks, bestAsk, bestAsk*(1+executionDepthBandPct))
	bookImbalance := 0.0
	if bidDepth+askDepth > 0 {
		bookImbalance = (bidDepth - askDepth) / (bidDepth + askDepth)
	}

	buySlip25 := slippageBpsForNotional(normalizedAsks, executionSmallNotionalUSD, bestAsk, true)
	sellSlip25 := slippageBpsForNotional(normalizedBids, executionSmallNotionalUSD, bestBid, false)
	buySlip100 := slippageBpsForNotional(normalizedAsks, executionMediumNotionalUSD, bestAsk, true)
	sellSlip100 := slippageBpsForNotional(normalizedBids, executionMediumNotionalUSD, bestBid, false)

	worstSlip25 := maxNonNil(buySlip25, sellSlip25)
	worstSlip100 := maxNonNil(buySlip100, sellSlip100)
	liqScore := liquidityScore(spreadBps, bidDepth, askDepth, worstSlip100)

	return &kernel.ExecutionQuality{
		SpreadBps:         roundedPtr(spreadBps, 3),
		LiqScore:          roundedPtr(liqScore, 3),
		DepthBidUSD1Pct:   roundedPtr(bidDepth, 3),
		DepthAskUSD1Pct:   roundedPtr(askDepth, 3),
		BookImbalance1Pct: roundedPtr(bookImbalance, 3),
		SlippageEst25USD:  roundedPtrValue(worstSlip25, 3),
		SlippageEst100USD: roundedPtrValue(worstSlip100, 3),
	}
}

func normalizeBookLevels(levels [][]float64, descending bool) [][]float64 {
	out := make([][]float64, 0, len(levels))
	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		price := level[0]
		qty := level[1]
		if price <= 0 || qty <= 0 || math.IsNaN(price) || math.IsNaN(qty) || math.IsInf(price, 0) || math.IsInf(qty, 0) {
			continue
		}
		out = append(out, []float64{price, qty})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if descending {
			return out[i][0] > out[j][0]
		}
		return out[i][0] < out[j][0]
	})
	return out
}

func depthWithinBandUSD(levels [][]float64, minPrice, maxPrice float64) float64 {
	total := 0.0
	for _, level := range levels {
		price := level[0]
		qty := level[1]
		if price < minPrice || price > maxPrice {
			continue
		}
		total += price * qty
	}
	return total
}

func slippageBpsForNotional(levels [][]float64, targetNotional, bestPrice float64, isBuy bool) *float64 {
	if targetNotional <= 0 || bestPrice <= 0 {
		return nil
	}

	remaining := targetNotional
	totalQuote := 0.0
	totalBase := 0.0

	for _, level := range levels {
		price := level[0]
		qty := level[1]
		availableNotional := price * qty
		if availableNotional <= 0 {
			continue
		}
		takeNotional := math.Min(remaining, availableNotional)
		totalQuote += takeNotional
		totalBase += takeNotional / price
		remaining -= takeNotional
		if remaining <= 1e-9 {
			break
		}
	}

	if totalBase <= 0 || remaining > 1e-6 {
		return nil
	}

	avgFill := totalQuote / totalBase
	if avgFill <= 0 {
		return nil
	}

	var bps float64
	if isBuy {
		bps = ((avgFill - bestPrice) / bestPrice) * 10_000
	} else {
		bps = ((bestPrice - avgFill) / bestPrice) * 10_000
	}

	return &bps
}

func liquidityScore(spreadBps, bidDepthUSD, askDepthUSD float64, worstSlip100 *float64) float64 {
	spreadScore := clampFloat64(1-(spreadBps/executionSpreadTargetBps), 0, 1)
	depthScore := clampFloat64(math.Min(bidDepthUSD, askDepthUSD)/executionDepthTargetUSD, 0, 1)

	slippageScore := 0.0
	if worstSlip100 != nil {
		slippageScore = clampFloat64(1-(*worstSlip100/executionSlippageTargetBps), 0, 1)
	}

	return 0.40*spreadScore + 0.35*depthScore + 0.25*slippageScore
}

func roundedPtr(value float64, decimals int) *float64 {
	v := roundFloat(value, decimals)
	return &v
}

func roundedPtrValue(value *float64, decimals int) *float64 {
	if value == nil {
		return nil
	}
	v := roundFloat(*value, decimals)
	return &v
}

func roundFloat(value float64, decimals int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	factor := math.Pow(10, float64(decimals))
	return math.Round(value*factor) / factor
}

func clampFloat64(value, minValue, maxValue float64) float64 {
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

func maxNonNil(a, b *float64) *float64 {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if *a > *b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func classifyVenueError(err error) string {
	if err == nil {
		return "unavailable"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "asset id"),
		strings.Contains(msg, "not found"),
		strings.Contains(msg, "unknown symbol"),
		strings.Contains(msg, "invalid symbol"),
		strings.Contains(msg, "symbol invalid"),
		strings.Contains(msg, "unsupported"),
		strings.Contains(msg, "does not exist"),
		strings.Contains(msg, "venue unavailable"):
		return "venue_unsupported"
	case strings.Contains(msg, "invalid order book"),
		strings.Contains(msg, "empty"),
		strings.Contains(msg, "no order book"):
		return "empty"
	default:
		return "unavailable"
	}
}
