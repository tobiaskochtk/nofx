package kernel

import (
	"fmt"

	decisionpkg "nofx/decision"
	"nofx/market"
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
		Account:         convertDecisionAccount(ctx.Account),
		MarketDataMap:   copyMarketDataMap(ctx.MarketDataMap),
		BTCETHLeverage:  ctx.BTCETHLeverage,
		AltcoinLeverage: ctx.AltcoinLeverage,
		MaxPositions:    maxPositions,
	}

	if ctx.TradingStats != nil {
		dctx.Performance = map[string]interface{}{
			"sharpe_ratio": ctx.TradingStats.SharpeRatio,
		}
	}

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
				Symbol:  coin.Symbol,
				Sources: append([]string(nil), coin.Sources...),
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
