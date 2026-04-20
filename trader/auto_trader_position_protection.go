package trader

import (
	"math"
	"nofx/logger"
	tradertypes "nofx/trader/types"
	"strings"
)

type inferredProtectionTargets struct {
	StopLoss        float64
	HasStopLoss     bool
	StopLossValid   bool
	TakeProfit      float64
	HasTakeProfit   bool
	TakeProfitValid bool
}

func (at *AutoTrader) auditOpenPositionProtection() {
	if at == nil || at.trader == nil {
		return
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Infof("⚠️ [%s] Protection audit failed to load positions: %v", at.name, err)
		return
	}

	for _, pos := range positions {
		at.auditPositionProtection(pos)
	}
}

func (at *AutoTrader) auditPositionProtection(pos map[string]interface{}) {
	if at == nil || pos == nil {
		return
	}

	symbol, _ := pos["symbol"].(string)
	side := normalizeTrailingSide(pos["side"])
	entryPrice, _ := floatValue(pos["entryPrice"])
	quantity, _ := floatValue(pos["positionAmt"])
	quantity = math.Abs(quantity)
	if symbol == "" || side == "" || entryPrice <= 0 || quantity <= 0 {
		return
	}

	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		logger.Infof("⚠️ [%s] Protection audit failed to load open orders for %s: %v", at.name, symbol, err)
		return
	}

	inferred := inferProtectionTargetsFromOpenOrders(side, entryPrice, openOrders)
	state, _ := at.getTrailingStopPositionState(symbol, side)

	stopLoss := inferred.StopLoss
	hasStopLoss := inferred.HasStopLoss
	if !hasStopLoss && state != nil && state.HasLastStopPrice {
		stopLoss = state.LastStopPrice
		hasStopLoss = true
	}

	takeProfit := inferred.TakeProfit
	hasTakeProfit := inferred.HasTakeProfit
	restoreTakeProfit := inferred.HasTakeProfit
	if !restoreTakeProfit && len(openOrders) == 0 && state != nil && state.HasTakeProfit && !state.HasActivated {
		takeProfit = state.TakeProfitPrice
		hasTakeProfit = true
		restoreTakeProfit = true
	}

	sideUpper := strings.ToUpper(side)
	stopValid := hasStopLoss && protectionStopValid(sideUpper, entryPrice, stopLoss)
	takeValid := hasTakeProfit && protectionTakeValid(sideUpper, entryPrice, takeProfit)

	needsRepair := len(openOrders) == 0 || !stopValid || (restoreTakeProfit && hasTakeProfit && !takeValid)
	if !needsRepair {
		return
	}
	if !hasStopLoss {
		logger.Infof("⚠️ [%s] Protection audit found %s %s without a recoverable stop-loss target", at.name, symbol, sideUpper)
		return
	}

	normalizedStop, normalizedTake, adjusted := normalizeProtectionTargets(sideUpper, entryPrice, entryPrice, stopLoss, takeProfit)
	if !protectionStopValid(sideUpper, entryPrice, normalizedStop) {
		logger.Infof("⚠️ [%s] Protection audit skipped %s %s because stop %.8f is still invalid around entry %.8f",
			at.name, symbol, sideUpper, normalizedStop, entryPrice)
		return
	}

	restoreNormalizedTake := restoreTakeProfit && hasTakeProfit && protectionTakeValid(sideUpper, entryPrice, normalizedTake)

	logger.Infof("🛡️ [%s] Repairing protective orders for %s %s @ %.8f (stop %.8f -> %.8f, take %.8f -> %.8f, adjusted=%v, open_orders=%d)",
		at.name, symbol, sideUpper, entryPrice, stopLoss, normalizedStop, takeProfit, normalizedTake, adjusted, len(openOrders))

	if err := at.trader.CancelStopOrders(symbol); err != nil {
		logger.Infof("⚠️ [%s] Failed to cancel existing protective orders for %s: %v", at.name, symbol, err)
		return
	}
	if err := at.trader.SetStopLoss(symbol, sideUpper, quantity, normalizedStop); err != nil {
		logger.Infof("⚠️ [%s] Failed to restore stop-loss for %s: %v", at.name, symbol, err)
		return
	}
	if restoreNormalizedTake {
		if err := at.trader.SetTakeProfit(symbol, sideUpper, quantity, normalizedTake); err != nil {
			logger.Infof("⚠️ [%s] Failed to restore take-profit for %s: %v", at.name, symbol, err)
		}
	}

	nextState := at.ensureTrailingStopPositionState(symbol, side)
	if nextState == nil {
		nextState = newTrailingStopPositionState()
	}
	nextState.LastStopPrice = normalizedStop
	nextState.HasLastStopPrice = true
	if restoreNormalizedTake {
		nextState.TakeProfitPrice = normalizedTake
		nextState.HasTakeProfit = true
	}
	at.saveTrailingStopPositionState(symbol, side, nextState)
}

func inferProtectionTargetsFromOpenOrders(side string, entryPrice float64, orders []tradertypes.OpenOrder) inferredProtectionTargets {
	result := inferredProtectionTargets{}
	normalizedSide := normalizeTrailingSide(side)
	sideUpper := strings.ToUpper(normalizedSide)
	if normalizedSide == "" {
		return result
	}

	var explicitStops []float64
	var explicitTakes []float64
	var genericOrders []float64

	for _, order := range orders {
		if !orderMatchesPositionSide(order, normalizedSide) {
			continue
		}
		triggerPrice := openOrderTriggerPrice(order)
		if triggerPrice <= 0 {
			continue
		}

		switch {
		case isTakeProfitOrderType(order.Type):
			explicitTakes = append(explicitTakes, triggerPrice)
		case isStopLossOrderType(order.Type) && !isGenericStopOrderType(order.Type):
			explicitStops = append(explicitStops, triggerPrice)
		default:
			genericOrders = append(genericOrders, triggerPrice)
		}
	}

	result.StopLoss, result.HasStopLoss, result.StopLossValid = selectProtectionCandidate(sideUpper, entryPrice, explicitStops, true)
	result.TakeProfit, result.HasTakeProfit, result.TakeProfitValid = selectProtectionCandidate(sideUpper, entryPrice, explicitTakes, false)

	generic := inferGenericProtectionTargets(sideUpper, entryPrice, genericOrders)
	if !result.HasStopLoss && generic.HasStopLoss {
		result.StopLoss = generic.StopLoss
		result.HasStopLoss = true
		result.StopLossValid = generic.StopLossValid
	}
	if !result.HasTakeProfit && generic.HasTakeProfit {
		result.TakeProfit = generic.TakeProfit
		result.HasTakeProfit = true
		result.TakeProfitValid = generic.TakeProfitValid
	}

	return result
}

func selectProtectionCandidate(side string, entryPrice float64, prices []float64, wantStop bool) (float64, bool, bool) {
	bestValid, hasValid := nearestProtectionPrice(entryPrice, prices, func(price float64) bool {
		if wantStop {
			return protectionStopValid(side, entryPrice, price)
		}
		return protectionTakeValid(side, entryPrice, price)
	})
	if hasValid {
		return bestValid, true, true
	}

	bestInvalid, hasInvalid := nearestProtectionPrice(entryPrice, prices, func(price float64) bool {
		if wantStop {
			return !protectionStopValid(side, entryPrice, price)
		}
		return !protectionTakeValid(side, entryPrice, price)
	})
	if hasInvalid {
		return bestInvalid, true, false
	}

	return 0, false, false
}

func nearestProtectionPrice(entryPrice float64, prices []float64, match func(price float64) bool) (float64, bool) {
	best := 0.0
	bestDistance := 0.0
	found := false

	for _, price := range prices {
		if price <= 0 || !match(price) {
			continue
		}
		distance := math.Abs(price - entryPrice)
		if !found || distance < bestDistance {
			best = price
			bestDistance = distance
			found = true
		}
	}

	return best, found
}

func inferGenericProtectionTargets(side string, entryPrice float64, prices []float64) inferredProtectionTargets {
	result := inferredProtectionTargets{}
	if len(prices) == 0 || entryPrice <= 0 {
		return result
	}

	aboveEntry := make([]float64, 0, len(prices))
	belowEntry := make([]float64, 0, len(prices))
	for _, price := range prices {
		switch {
		case price > entryPrice:
			aboveEntry = append(aboveEntry, price)
		case price < entryPrice:
			belowEntry = append(belowEntry, price)
		}
	}

	switch side {
	case "LONG":
		if stop, ok := closestProtectionPrice(entryPrice, belowEntry); ok {
			result.StopLoss = stop
			result.HasStopLoss = true
			result.StopLossValid = true
		} else if stop, ok := closestProtectionPrice(entryPrice, aboveEntry); ok {
			result.StopLoss = stop
			result.HasStopLoss = true
			result.StopLossValid = false
		}

		takeCandidates := aboveEntry
		if result.HasStopLoss && !result.StopLossValid && len(takeCandidates) > 1 {
			takeCandidates = removeOneProtectionPrice(takeCandidates, result.StopLoss)
		}
		if take, ok := farthestProtectionPrice(entryPrice, takeCandidates); ok {
			result.TakeProfit = take
			result.HasTakeProfit = true
			result.TakeProfitValid = true
		} else if take, ok := farthestProtectionPrice(entryPrice, belowEntry); ok {
			result.TakeProfit = take
			result.HasTakeProfit = true
			result.TakeProfitValid = false
		}

	case "SHORT":
		if stop, ok := closestProtectionPrice(entryPrice, aboveEntry); ok {
			result.StopLoss = stop
			result.HasStopLoss = true
			result.StopLossValid = true
		} else if stop, ok := closestProtectionPrice(entryPrice, belowEntry); ok {
			result.StopLoss = stop
			result.HasStopLoss = true
			result.StopLossValid = false
		}

		takeCandidates := belowEntry
		if result.HasStopLoss && !result.StopLossValid && len(takeCandidates) > 1 {
			takeCandidates = removeOneProtectionPrice(takeCandidates, result.StopLoss)
		}
		if take, ok := farthestProtectionPrice(entryPrice, takeCandidates); ok {
			result.TakeProfit = take
			result.HasTakeProfit = true
			result.TakeProfitValid = true
		} else if take, ok := farthestProtectionPrice(entryPrice, aboveEntry); ok {
			result.TakeProfit = take
			result.HasTakeProfit = true
			result.TakeProfitValid = false
		}
	}

	return result
}

func closestProtectionPrice(entryPrice float64, prices []float64) (float64, bool) {
	best := 0.0
	bestDistance := 0.0
	found := false
	for _, price := range prices {
		if price <= 0 {
			continue
		}
		distance := math.Abs(price - entryPrice)
		if !found || distance < bestDistance {
			best = price
			bestDistance = distance
			found = true
		}
	}
	return best, found
}

func farthestProtectionPrice(entryPrice float64, prices []float64) (float64, bool) {
	best := 0.0
	bestDistance := 0.0
	found := false
	for _, price := range prices {
		if price <= 0 {
			continue
		}
		distance := math.Abs(price - entryPrice)
		if !found || distance > bestDistance {
			best = price
			bestDistance = distance
			found = true
		}
	}
	return best, found
}

func removeOneProtectionPrice(prices []float64, target float64) []float64 {
	result := make([]float64, 0, len(prices))
	removed := false
	for _, price := range prices {
		if !removed && math.Abs(price-target) <= 0.0000001 {
			removed = true
			continue
		}
		result = append(result, price)
	}
	return result
}

func isGenericStopOrderType(orderType string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(orderType))
	switch normalized {
	case "", "STOP", "STOPORDER", "STOP_ORDER", "TRIGGER", "TRIGGERORDER", "TRIGGER_ORDER":
		return true
	default:
		return false
	}
}
