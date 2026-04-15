package trader

import (
	"fmt"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
)

func (at *AutoTrader) maybeInvertDecisions(decisions []kernel.Decision, marketDataMap map[string]*market.Data) []kernel.Decision {
	if !at.config.InvertSignals || len(decisions) == 0 {
		return decisions
	}

	inverted := make([]kernel.Decision, len(decisions))
	for i, decision := range decisions {
		currentPrice := 0.0
		if marketDataMap != nil {
			if marketData, ok := marketDataMap[decision.Symbol]; ok && marketData != nil {
				currentPrice = marketData.CurrentPrice
			}
		}

		inverted[i], _ = invertDecision(decision, currentPrice)
	}

	return inverted
}

func invertDecision(decision kernel.Decision, currentPrice float64) (kernel.Decision, string) {
	inverted := decision
	mode := ""

	switch decision.Action {
	case "open_long":
		inverted.Action = "open_short"
		inverted.StopLoss, inverted.TakeProfit, mode = mirrorTargets(decision.StopLoss, decision.TakeProfit, currentPrice, inverted.Action)
	case "open_short":
		inverted.Action = "open_long"
		inverted.StopLoss, inverted.TakeProfit, mode = mirrorTargets(decision.StopLoss, decision.TakeProfit, currentPrice, inverted.Action)
	case "close_long":
		inverted.Action = "close_short"
	case "close_short":
		inverted.Action = "close_long"
	default:
		return inverted, mode
	}

	if inverted.Action != decision.Action {
		reasoning := decision.Reasoning
		if reasoning == "" {
			reasoning = "inverse signal mode"
		}
		if mode != "" {
			inverted.Reasoning = fmt.Sprintf("[inverse:%s] %s", mode, reasoning)
		} else {
			inverted.Reasoning = fmt.Sprintf("[inverse] %s", reasoning)
		}
	}

	return inverted, mode
}

func mirrorTargets(stopLoss, takeProfit, currentPrice float64, action string) (float64, float64, string) {
	if currentPrice > 0 && stopLoss > 0 && takeProfit > 0 {
		mirroredStop := (2 * currentPrice) - stopLoss
		mirroredTake := (2 * currentPrice) - takeProfit
		if targetsValidForAction(action, mirroredStop, mirroredTake) {
			return mirroredStop, mirroredTake, "mirror_price"
		}
	}

	if targetsValidForAction(action, takeProfit, stopLoss) {
		return takeProfit, stopLoss, "swap_targets"
	}

	return stopLoss, takeProfit, ""
}

func targetsValidForAction(action string, stopLoss, takeProfit float64) bool {
	if stopLoss <= 0 || takeProfit <= 0 {
		return false
	}

	switch action {
	case "open_long":
		return stopLoss < takeProfit
	case "open_short":
		return stopLoss > takeProfit
	default:
		return true
	}
}

func logInvertedDecisions(name string, original, inverted []kernel.Decision) {
	if len(original) == 0 || len(original) != len(inverted) {
		return
	}

	logger.Infof("🪞 [%s] Inverse signal mode enabled, mirroring %d AI decisions", name, len(inverted))
	for i := range original {
		if original[i].Action == inverted[i].Action {
			continue
		}
		logger.Infof("🪞 [%s] %s %s -> %s", name, original[i].Symbol, original[i].Action, inverted[i].Action)
	}
}
