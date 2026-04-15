package kernel

import (
	"fmt"
	"math"
	"strings"

	decisionpkg "nofx/decision"
	"nofx/market"
	"nofx/provider/nofxos"
)

func buildDecisionPayloadPrompt(ctx *Context, maxPositions int) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context is nil")
	}
	dctx := &decisionpkg.Context{
		CurrentTime:     ctx.CurrentTime,
		RuntimeMinutes:  ctx.RuntimeMinutes,
		CallCount:       ctx.CallCount,
		PayloadVersion:  decisionpkg.PayloadSchemaVersion,
		ContextTF:       resolvePayloadContextTF(ctx),
		PriceType:       "mark",
		Exchange:        ctx.Exchange,
		EMAPeriods:      append([]int(nil), ctx.EMAPeriods...),
		RSIPeriods:      append([]int(nil), ctx.RSIPeriods...),
		FeatureFlagsSet: ctx.FeatureFlagsSet,
		EnableF4:        ctx.EnableF4,
		EnableF5:        ctx.EnableF5,
		EnableF6:        ctx.EnableF6,
		EnableF7:        ctx.EnableF7,
		Account:         convertDecisionAccount(ctx.Account),
		MarketDataMap:   copyMarketDataMap(ctx.MarketDataMap),
		BTCETHLeverage:  ctx.BTCETHLeverage,
		AltcoinLeverage: ctx.AltcoinLeverage,
		BTCETHPosRatio:  ctx.BTCETHPosRatio,
		AltcoinPosRatio: ctx.AltcoinPosRatio,
		MinPositionSize: ctx.MinPositionSize,
		MinConfidence:   ctx.MinConfidence,
		MaxPositions:    maxPositions,
	}

	if ctx.TradingStats != nil {
		dctx.Performance = &decisionpkg.PerformanceSummary{
			TotalTrades:    ctx.TradingStats.TotalTrades,
			WinRate:        ctx.TradingStats.WinRate,
			ProfitFactor:   ctx.TradingStats.ProfitFactor,
			SharpeRatio:    ctx.TradingStats.SharpeRatio,
			TotalPnL:       ctx.TradingStats.TotalPnL,
			AvgWin:         ctx.TradingStats.AvgWin,
			AvgLoss:        ctx.TradingStats.AvgLoss,
			MaxDrawdownPct: ctx.TradingStats.MaxDrawdownPct,
		}
	}

	if len(ctx.RecentOrders) > 0 {
		dctx.RecentTrades = make([]decisionpkg.RecentTrade, 0, len(ctx.RecentOrders))
		for _, order := range ctx.RecentOrders {
			dctx.RecentTrades = append(dctx.RecentTrades, decisionpkg.RecentTrade{
				Symbol:        order.Symbol,
				Side:          order.Side,
				PnLPct:        order.PnLPct,
				HoldDuration:  order.HoldDuration,
				ExitTimestamp: order.ExitTimestamp,
			})
		}
	}

	dctx.ExecutionQuality = convertDecisionExecutionQualityMap(ctx.ExecutionQualityMap)
	dctx.VenueTradability = convertDecisionVenueTradabilityMap(ctx.VenueTradabilityMap)
	dctx.QuantFlowMap = convertDecisionQuantFlowMap(ctx.QuantDataMap)
	dctx.MarketLeadership = convertDecisionMarketLeadership(ctx.OIRankingData, ctx.NetFlowRankingData, ctx.PriceRankingData)

	if len(ctx.Positions) > 0 {
		dctx.Positions = make([]decisionpkg.PositionInfo, 0, len(ctx.Positions))
		for _, pos := range ctx.Positions {
			dctx.Positions = append(dctx.Positions, decisionpkg.PositionInfo{
				Symbol:           pos.Symbol,
				Side:             pos.Side,
				EntryPrice:       pos.EntryPrice,
				MarkPrice:        pos.MarkPrice,
				Quantity:         pos.Quantity,
				Leverage:         pos.Leverage,
				UnrealizedPnL:    pos.UnrealizedPnL,
				UnrealizedPnLPct: pos.UnrealizedPnLPct,
				PeakPnLPct:       pos.PeakPnLPct,
				LiquidationPrice: pos.LiquidationPrice,
				MarginUsed:       pos.MarginUsed,
				UpdateTime:       pos.UpdateTime,
			})
		}
	}

	if len(ctx.CandidateCoins) > 0 {
		dctx.CandidateCoins = make([]decisionpkg.CandidateCoin, 0, len(ctx.CandidateCoins))
		for _, coin := range ctx.CandidateCoins {
			dctx.CandidateCoins = append(dctx.CandidateCoins, decisionpkg.CandidateCoin{
				Symbol:          coin.Symbol,
				Sources:         append([]string(nil), coin.Sources...),
				SelectionBucket: strings.TrimSpace(coin.SelectionBucket),
			})
		}
	}

	return decisionpkg.BuildUserPayload(dctx)
}

func convertDecisionAccount(account AccountInfo) decisionpkg.AccountInfo {
	return decisionpkg.AccountInfo{
		TotalEquity:      account.TotalEquity,
		AvailableBalance: account.AvailableBalance,
		UnrealizedPnL:    account.UnrealizedPnL,
		TotalPnL:         account.TotalPnL,
		TotalPnLPct:      account.TotalPnLPct,
		MarginUsed:       account.MarginUsed,
		MarginUsedPct:    account.MarginUsedPct,
		PositionCount:    account.PositionCount,
	}
}

func copyMarketDataMap(in map[string]*market.Data) map[string]*market.Data {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]*market.Data, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func resolvePayloadContextTF(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	for _, tf := range ctx.Timeframes {
		normalized := strings.TrimSpace(strings.ToLower(tf))
		if normalized != "" {
			return normalized
		}
	}
	return ""
}

func convertDecisionQuantFlowMap(in map[string]*QuantData) map[string]*decisionpkg.QuantFlowSummary {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]*decisionpkg.QuantFlowSummary, len(in))
	for symbol, data := range in {
		summary := convertDecisionQuantFlow(data)
		if summary == nil {
			continue
		}
		out[strings.ToUpper(strings.TrimSpace(symbol))] = summary
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func convertDecisionExecutionQualityMap(in map[string]*ExecutionQuality) map[string]*decisionpkg.ExecutionQualitySummary {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]*decisionpkg.ExecutionQualitySummary, len(in))
	for symbol, data := range in {
		summary := convertDecisionExecutionQuality(data)
		if summary == nil {
			continue
		}
		out[strings.ToUpper(strings.TrimSpace(symbol))] = summary
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func convertDecisionVenueTradabilityMap(in map[string]*VenueTradability) map[string]*decisionpkg.VenueTradabilitySummary {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]*decisionpkg.VenueTradabilitySummary, len(in))
	for symbol, data := range in {
		summary := convertDecisionVenueTradability(data)
		if summary == nil {
			continue
		}
		out[strings.ToUpper(strings.TrimSpace(symbol))] = summary
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func convertDecisionExecutionQuality(data *ExecutionQuality) *decisionpkg.ExecutionQualitySummary {
	if data == nil {
		return nil
	}
	summary := &decisionpkg.ExecutionQualitySummary{
		SpreadBps:         roundedPtr(data.SpreadBps, 3),
		LiqScore:          roundedPtr(data.LiqScore, 3),
		DepthBidUSD1Pct:   roundedPtr(data.DepthBidUSD1Pct, 3),
		DepthAskUSD1Pct:   roundedPtr(data.DepthAskUSD1Pct, 3),
		BookImbalance1Pct: roundedPtr(data.BookImbalance1Pct, 3),
		SlippageEst25USD:  roundedPtr(data.SlippageEst25USD, 3),
		SlippageEst100USD: roundedPtr(data.SlippageEst100USD, 3),
	}
	if summary.SpreadBps == nil &&
		summary.LiqScore == nil &&
		summary.DepthBidUSD1Pct == nil &&
		summary.DepthAskUSD1Pct == nil &&
		summary.BookImbalance1Pct == nil &&
		summary.SlippageEst25USD == nil &&
		summary.SlippageEst100USD == nil {
		return nil
	}
	return summary
}

func convertDecisionVenueTradability(data *VenueTradability) *decisionpkg.VenueTradabilitySummary {
	if data == nil {
		return nil
	}
	summary := &decisionpkg.VenueTradabilitySummary{
		VenueSupported: data.VenueSupported,
		OrderBookState: strings.TrimSpace(strings.ToLower(data.OrderBookState)),
		MinNotionalOK:  data.MinNotionalOK,
		PriceSource:    strings.TrimSpace(strings.ToLower(data.PriceSource)),
	}
	if !summary.VenueSupported && summary.OrderBookState == "" && summary.MinNotionalOK == nil && summary.PriceSource == "" {
		return nil
	}
	return summary
}

func convertDecisionQuantFlow(data *QuantData) *decisionpkg.QuantFlowSummary {
	if data == nil {
		return nil
	}
	summary := &decisionpkg.QuantFlowSummary{
		InstFuture15m:     roundedPtr(flowValue(data.Netflow, "institution", "future", "15m"), 3),
		InstFuture1h:      roundedPtr(flowValue(data.Netflow, "institution", "future", "1h"), 3),
		InstSpot1h:        roundedPtr(flowValue(data.Netflow, "institution", "spot", "1h"), 3),
		RetailFuture15m:   roundedPtr(flowValue(data.Netflow, "personal", "future", "15m"), 3),
		OIDelta15mPct:     roundedPtr(oiDeltaPercent(data.OI, "15m"), 3),
		OIDelta1hPct:      roundedPtr(oiDeltaPercent(data.OI, "1h"), 3),
		PriceChange15mPct: roundedScaledPctPtr(priceChange(data.PriceChange, "15m"), 3),
		PriceChange1hPct:  roundedScaledPctPtr(priceChange(data.PriceChange, "1h"), 3),
	}
	if summary.InstFuture15m == nil &&
		summary.InstFuture1h == nil &&
		summary.InstSpot1h == nil &&
		summary.RetailFuture15m == nil &&
		summary.OIDelta15mPct == nil &&
		summary.OIDelta1hPct == nil &&
		summary.PriceChange15mPct == nil &&
		summary.PriceChange1hPct == nil {
		return nil
	}
	return summary
}

func convertDecisionMarketLeadership(
	oiData *nofxos.OIRankingData,
	netflowData *nofxos.NetFlowRankingData,
	priceData *nofxos.PriceRankingData,
) *decisionpkg.MarketLeadershipSummary {
	out := &decisionpkg.MarketLeadershipSummary{
		OITop1h:          convertDecisionOILeaders(oiData, true),
		OILow1h:          convertDecisionOILeaders(oiData, false),
		InstInflowTop1h:  convertDecisionFlowLeaders(netflowData, true),
		InstOutflowTop1h: convertDecisionFlowLeaders(netflowData, false),
		PriceLeaders1h:   convertDecisionPriceLeaders(priceData, "1h", true),
		PriceLeaders4h:   convertDecisionPriceLeaders(priceData, "4h", true),
		PriceLosers1h:    convertDecisionPriceLeaders(priceData, "1h", false),
	}
	if len(out.OITop1h) == 0 &&
		len(out.OILow1h) == 0 &&
		len(out.InstInflowTop1h) == 0 &&
		len(out.InstOutflowTop1h) == 0 &&
		len(out.PriceLeaders1h) == 0 &&
		len(out.PriceLeaders4h) == 0 &&
		len(out.PriceLosers1h) == 0 {
		return nil
	}
	return out
}

func convertDecisionOILeaders(data *nofxos.OIRankingData, top bool) []decisionpkg.OILeadershipItem {
	if data == nil {
		return nil
	}
	source := data.LowPositions
	if top {
		source = data.TopPositions
	}
	limit := minInt(len(source), 3)
	if limit == 0 {
		return nil
	}
	out := make([]decisionpkg.OILeadershipItem, 0, limit)
	for _, item := range source[:limit] {
		out = append(out, decisionpkg.OILeadershipItem{
			Symbol:       strings.ToUpper(strings.TrimSpace(item.Symbol)),
			OIDelta1hPct: item.OIDeltaPercent,
			PriceChgPct:  item.PriceDeltaPercent,
		})
	}
	return out
}

func convertDecisionFlowLeaders(data *nofxos.NetFlowRankingData, inflow bool) []decisionpkg.FlowLeadershipItem {
	if data == nil {
		return nil
	}
	source := data.InstitutionFutureLow
	if inflow {
		source = data.InstitutionFutureTop
	}
	limit := minInt(len(source), 3)
	if limit == 0 {
		return nil
	}
	out := make([]decisionpkg.FlowLeadershipItem, 0, limit)
	for _, item := range source[:limit] {
		out = append(out, decisionpkg.FlowLeadershipItem{
			Symbol: strings.ToUpper(strings.TrimSpace(item.Symbol)),
			Amount: item.Amount,
		})
	}
	return out
}

func convertDecisionPriceLeaders(data *nofxos.PriceRankingData, duration string, top bool) []decisionpkg.PriceLeadershipItem {
	if data == nil || data.Durations == nil {
		return nil
	}
	duration = strings.ToLower(strings.TrimSpace(duration))
	ranking := data.Durations[duration]
	if ranking == nil {
		return nil
	}
	source := ranking.Low
	if top {
		source = ranking.Top
	}
	limit := minInt(len(source), 3)
	if limit == 0 {
		return nil
	}
	out := make([]decisionpkg.PriceLeadershipItem, 0, limit)
	for _, item := range source[:limit] {
		out = append(out, decisionpkg.PriceLeadershipItem{
			Symbol: strings.ToUpper(strings.TrimSpace(item.Symbol)),
			ChgPct: item.PriceDelta * 100,
		})
	}
	return out
}

func flowValue(data *NetflowData, actor, venue, duration string) *float64 {
	if data == nil {
		return nil
	}
	var flowType *FlowTypeData
	switch strings.ToLower(strings.TrimSpace(actor)) {
	case "institution":
		flowType = data.Institution
	case "personal", "retail":
		flowType = data.Personal
	}
	if flowType == nil {
		return nil
	}
	duration = strings.ToLower(strings.TrimSpace(duration))
	switch strings.ToLower(strings.TrimSpace(venue)) {
	case "future", "futures":
		if flowType.Future == nil {
			return nil
		}
		if value, ok := flowType.Future[duration]; ok {
			return &value
		}
	case "spot":
		if flowType.Spot == nil {
			return nil
		}
		if value, ok := flowType.Spot[duration]; ok {
			return &value
		}
	}
	return nil
}

func oiDeltaPercent(oi map[string]*OIData, duration string) *float64 {
	if len(oi) == 0 {
		return nil
	}
	duration = strings.ToLower(strings.TrimSpace(duration))
	for _, exchange := range []string{"binance", "binanceusdm", "bybit"} {
		if data := oi[exchange]; data != nil && data.Delta != nil {
			if delta := data.Delta[duration]; delta != nil {
				value := delta.OIDeltaPercent
				return &value
			}
		}
	}
	for _, data := range oi {
		if data == nil || data.Delta == nil {
			continue
		}
		if delta := data.Delta[duration]; delta != nil {
			value := delta.OIDeltaPercent
			return &value
		}
	}
	return nil
}

func priceChange(changes map[string]float64, duration string) *float64 {
	if len(changes) == 0 {
		return nil
	}
	duration = strings.ToLower(strings.TrimSpace(duration))
	if value, ok := changes[duration]; ok {
		return &value
	}
	return nil
}

func roundedPtr(value *float64, decimals int) *float64 {
	if value == nil {
		return nil
	}
	v := *value
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	result := math.Round(v*factor) / factor
	return &result
}

func roundedScaledPctPtr(value *float64, decimals int) *float64 {
	if value == nil {
		return nil
	}
	scaled := *value * 100
	return roundedPtr(&scaled, decimals)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
