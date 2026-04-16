package trader

import (
	"fmt"
	"math"
	"nofx/logger"
	"nofx/store"
	tradertypes "nofx/trader/types"
	"strings"
	"time"
)

type trailingStopPositionState struct {
	LastStopPrice    float64
	HasLastStopPrice bool
	TakeProfitPrice  float64
	HasTakeProfit    bool
	HighestTierIndex int
}

func newTrailingStopPositionState() *trailingStopPositionState {
	return &trailingStopPositionState{HighestTierIndex: -1}
}

func (at *AutoTrader) trailingStopConfig() store.TrailingStopConfig {
	if at == nil || at.config.StrategyConfig == nil {
		return store.DefaultTrailingStopConfig()
	}
	return at.config.StrategyConfig.RiskControl.EffectiveTrailingStop()
}

func (at *AutoTrader) startTrailingStopMonitor() {
	cfg := at.trailingStopConfig()
	if !cfg.Enabled {
		return
	}

	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(time.Duration(cfg.CheckIntervalSec) * time.Second)
		defer ticker.Stop()

		logger.Infof("🎯 Started trailing-stop monitor (every %ds)", cfg.CheckIntervalSec)

		for {
			select {
			case <-ticker.C:
				at.updateTrailingStops()
			case <-at.stopMonitorCh:
				logger.Info("⏹ Stopped trailing-stop monitor")
				return
			}
		}
	}()
}

func (at *AutoTrader) updateTrailingStops() {
	cfg := at.trailingStopConfig()
	if !cfg.Enabled {
		return
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Infof("⚠️ Trailing stop: failed to get positions: %v", err)
		return
	}

	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side := normalizeTrailingSide(pos["side"])
		if symbol == "" || side == "" {
			continue
		}

		entryPrice, ok := floatValue(pos["entryPrice"])
		if !ok || entryPrice <= 0 {
			continue
		}

		markPrice, _ := floatValue(pos["markPrice"])
		if markPrice <= 0 {
			if latest, err := at.trader.GetMarketPrice(symbol); err == nil {
				markPrice = latest
			}
		}
		if markPrice <= 0 {
			continue
		}

		quantity, _ := floatValue(pos["positionAmt"])
		quantity = math.Abs(quantity)
		if quantity <= 0 {
			continue
		}

		leverage, _ := floatValue(pos["leverage"])
		if leverage <= 0 {
			leverage = 1
		}

		state := at.ensureTrailingStopPositionState(symbol, side)
		if state == nil {
			state = newTrailingStopPositionState()
		}

		profitPct := calculateLeveragedProfitPct(side, entryPrice, markPrice, leverage)
		currentTierIndex := resolveTrailingTierIndex(cfg.Tiers, profitPct)
		if currentTierIndex > state.HighestTierIndex {
			state.HighestTierIndex = currentTierIndex
		}
		activeTierIndex := state.HighestTierIndex
		if activeTierIndex < 0 {
			at.saveTrailingStopPositionState(symbol, side, state)
			continue
		}

		activeTier := cfg.Tiers[activeTierIndex]
		targetStopProfitPct := calculateTargetStopProfitPct(activeTier, profitPct)
		newStopPrice := calculateTrailingStopPrice(side, entryPrice, markPrice, leverage, targetStopProfitPct)
		if newStopPrice <= 0 {
			continue
		}

		if at.trailingStopUpdateRequiresTakeProfitRestore() && !state.HasTakeProfit {
			logger.Warnf("⚠️ Trailing stop skipped for %s %s: exchange %s cannot preserve TP automatically and no TP target is known",
				symbol, side, at.exchange)
			at.saveTrailingStopPositionState(symbol, side, state)
			continue
		}

		if !shouldUpdateTrailingStop(side, newStopPrice, state.LastStopPrice, state.HasLastStopPrice, cfg.UpdateThresholdPct) {
			at.saveTrailingStopPositionState(symbol, side, state)
			continue
		}

		if err := at.applyTrailingStopUpdate(symbol, side, quantity, newStopPrice, state); err != nil {
			logger.Infof("⚠️ Trailing stop update failed for %s %s: %v", symbol, side, err)
			at.saveTrailingStopPositionState(symbol, side, state)
			continue
		}

		state.LastStopPrice = newStopPrice
		state.HasLastStopPrice = true
		at.saveTrailingStopPositionState(symbol, side, state)

		logger.Infof("🎯 Trailing stop updated: %s %s -> %.8f (profit %.2f%%, tier %.2f%%)",
			symbol, side, newStopPrice, profitPct, activeTier.TriggerProfitPct)
	}
}

func (at *AutoTrader) applyTrailingStopUpdate(symbol, side string, quantity, newStopPrice float64, state *trailingStopPositionState) error {
	if err := at.trader.CancelStopLossOrders(symbol); err != nil {
		return fmt.Errorf("cancel stop loss orders: %w", err)
	}

	positionSide := strings.ToUpper(side)
	if err := at.trader.SetStopLoss(symbol, positionSide, quantity, newStopPrice); err != nil {
		if state != nil && state.HasLastStopPrice {
			if restoreErr := at.trader.SetStopLoss(symbol, positionSide, quantity, state.LastStopPrice); restoreErr != nil {
				logger.Warnf("⚠️ Failed to restore previous stop loss for %s %s: %v", symbol, side, restoreErr)
			}
		}
		if at.trailingStopUpdateRequiresTakeProfitRestore() && state != nil && state.HasTakeProfit {
			if restoreErr := at.trader.SetTakeProfit(symbol, positionSide, quantity, state.TakeProfitPrice); restoreErr != nil {
				logger.Warnf("⚠️ Failed to restore take profit after stop-loss error for %s %s: %v", symbol, side, restoreErr)
			}
		}
		return fmt.Errorf("set stop loss: %w", err)
	}

	if at.trailingStopUpdateRequiresTakeProfitRestore() && state != nil && state.HasTakeProfit {
		if err := at.trader.SetTakeProfit(symbol, positionSide, quantity, state.TakeProfitPrice); err != nil {
			return fmt.Errorf("restore take profit: %w", err)
		}
	}

	return nil
}

func (at *AutoTrader) trailingStopUpdateRequiresTakeProfitRestore() bool {
	switch strings.ToLower(strings.TrimSpace(at.exchange)) {
	case "hyperliquid", "lighter":
		return true
	default:
		return false
	}
}

func (at *AutoTrader) seedTrailingStopState(symbol, side string, stopLoss, takeProfit float64) {
	if at == nil {
		return
	}

	state := at.ensureTrailingStopPositionState(symbol, side)
	if state == nil {
		state = newTrailingStopPositionState()
	}
	if stopLoss > 0 {
		state.LastStopPrice = stopLoss
		state.HasLastStopPrice = true
	}
	if takeProfit > 0 {
		state.TakeProfitPrice = takeProfit
		state.HasTakeProfit = true
	}
	at.saveTrailingStopPositionState(symbol, side, state)
}

func (at *AutoTrader) clearTrailingStopState(symbol, side string) {
	key := trailingStopPositionKey(symbol, side)
	at.trailingStopStateMu.Lock()
	defer at.trailingStopStateMu.Unlock()
	delete(at.trailingStopState, key)
}

func (at *AutoTrader) getTrailingStopPositionState(symbol, side string) (*trailingStopPositionState, bool) {
	key := trailingStopPositionKey(symbol, side)
	at.trailingStopStateMu.RLock()
	defer at.trailingStopStateMu.RUnlock()

	state, ok := at.trailingStopState[key]
	if !ok || state == nil {
		return nil, false
	}

	cloned := *state
	return &cloned, true
}

func (at *AutoTrader) saveTrailingStopPositionState(symbol, side string, state *trailingStopPositionState) {
	if state == nil {
		return
	}

	key := trailingStopPositionKey(symbol, side)
	cloned := *state

	at.trailingStopStateMu.Lock()
	defer at.trailingStopStateMu.Unlock()
	at.trailingStopState[key] = &cloned
}

func (at *AutoTrader) ensureTrailingStopPositionState(symbol, side string) *trailingStopPositionState {
	if state, ok := at.getTrailingStopPositionState(symbol, side); ok {
		return state
	}

	state := newTrailingStopPositionState()
	stopLoss, hasStopLoss, takeProfit, hasTakeProfit := at.loadTrailingTargets(symbol, side)
	if hasStopLoss {
		state.LastStopPrice = stopLoss
		state.HasLastStopPrice = true
	}
	if hasTakeProfit {
		state.TakeProfitPrice = takeProfit
		state.HasTakeProfit = true
	}

	at.saveTrailingStopPositionState(symbol, side, state)
	return state
}

func (at *AutoTrader) loadTrailingTargets(symbol, side string) (stopLoss float64, hasStopLoss bool, takeProfit float64, hasTakeProfit bool) {
	stopLoss, hasStopLoss, takeProfit, hasTakeProfit = at.loadTrailingTargetsFromExchange(symbol, side)
	if hasStopLoss && hasTakeProfit {
		return stopLoss, hasStopLoss, takeProfit, hasTakeProfit
	}

	if at.store == nil {
		return stopLoss, hasStopLoss, takeProfit, hasTakeProfit
	}

	caseRec, err := at.store.DealReview().GetLatestOpenCaseBySymbol(at.userID, at.id, symbol, strings.ToUpper(side))
	if err != nil || caseRec == nil {
		return stopLoss, hasStopLoss, takeProfit, hasTakeProfit
	}

	if !hasStopLoss && caseRec.OpenStopLoss > 0 {
		stopLoss = caseRec.OpenStopLoss
		hasStopLoss = true
	}
	if !hasTakeProfit && caseRec.OpenTakeProfit > 0 {
		takeProfit = caseRec.OpenTakeProfit
		hasTakeProfit = true
	}

	return stopLoss, hasStopLoss, takeProfit, hasTakeProfit
}

func (at *AutoTrader) loadTrailingTargetsFromExchange(symbol, side string) (stopLoss float64, hasStopLoss bool, takeProfit float64, hasTakeProfit bool) {
	orders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return 0, false, 0, false
	}

	for _, order := range orders {
		if !orderMatchesPositionSide(order, side) {
			continue
		}
		triggerPrice := openOrderTriggerPrice(order)
		if triggerPrice <= 0 {
			continue
		}

		if isTakeProfitOrderType(order.Type) && !hasTakeProfit {
			takeProfit = triggerPrice
			hasTakeProfit = true
		}
		if isStopLossOrderType(order.Type) && !hasStopLoss {
			stopLoss = triggerPrice
			hasStopLoss = true
		}
	}

	return stopLoss, hasStopLoss, takeProfit, hasTakeProfit
}

func trailingStopPositionKey(symbol, side string) string {
	return strings.ToUpper(strings.TrimSpace(symbol)) + "_" + normalizeTrailingSide(side)
}

func normalizeTrailingSide(raw any) string {
	switch value := raw.(type) {
	case string:
		side := strings.ToLower(strings.TrimSpace(value))
		switch side {
		case "long", "short":
			return side
		case "buy":
			return "long"
		case "sell":
			return "short"
		case "long_side":
			return "long"
		case "short_side":
			return "short"
		default:
			return ""
		}
	default:
		return ""
	}
}

func calculateLeveragedProfitPct(side string, entryPrice, markPrice, leverage float64) float64 {
	if entryPrice <= 0 || leverage <= 0 {
		return 0
	}
	if side == "long" {
		return ((markPrice - entryPrice) / entryPrice) * leverage * 100
	}
	return ((entryPrice - markPrice) / entryPrice) * leverage * 100
}

func resolveTrailingTierIndex(tiers []store.TrailingStopTier, profitPct float64) int {
	for idx := len(tiers) - 1; idx >= 0; idx-- {
		if profitPct >= tiers[idx].TriggerProfitPct {
			return idx
		}
	}
	return -1
}

func calculateTargetStopProfitPct(tier store.TrailingStopTier, profitPct float64) float64 {
	switch tier.Mode {
	case store.TrailingStopModeTrailOffset:
		target := profitPct - tier.TrailOffsetPct
		if target < 0 {
			return 0
		}
		return target
	default:
		if tier.LockProfitPct < 0 {
			return 0
		}
		return tier.LockProfitPct
	}
}

func calculateTrailingStopPrice(side string, entryPrice, markPrice, leverage, targetStopProfitPct float64) float64 {
	if entryPrice <= 0 || leverage <= 0 {
		return 0
	}

	stopProfitRatio := targetStopProfitPct / (leverage * 100)
	newStopPrice := 0.0
	if side == "long" {
		newStopPrice = entryPrice * (1 + stopProfitRatio)
		if markPrice > 0 && newStopPrice >= markPrice {
			newStopPrice = markPrice * 0.999
		}
	} else {
		newStopPrice = entryPrice * (1 - stopProfitRatio)
		if markPrice > 0 && newStopPrice <= markPrice {
			newStopPrice = markPrice * 1.001
		}
	}
	return newStopPrice
}

func shouldUpdateTrailingStop(side string, newStopPrice, lastStopPrice float64, hasLastStop bool, updateThresholdPct float64) bool {
	if !hasLastStop || lastStopPrice <= 0 {
		return true
	}

	if side == "long" && newStopPrice <= lastStopPrice {
		return false
	}
	if side == "short" && newStopPrice >= lastStopPrice {
		return false
	}

	priceChangePct := math.Abs((newStopPrice-lastStopPrice)/lastStopPrice) * 100
	return priceChangePct >= updateThresholdPct
}

func orderMatchesPositionSide(order tradertypes.OpenOrder, side string) bool {
	positionSide := strings.ToUpper(strings.TrimSpace(order.PositionSide))
	if positionSide == "" || positionSide == "BOTH" {
		return true
	}
	return positionSide == strings.ToUpper(side)
}

func openOrderTriggerPrice(order tradertypes.OpenOrder) float64 {
	if order.StopPrice > 0 {
		return order.StopPrice
	}
	if order.Price > 0 {
		return order.Price
	}
	return 0
}

func isStopLossOrderType(orderType string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(orderType))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "TAKE_PROFIT") || strings.Contains(normalized, "TAKEPROFIT") {
		return false
	}
	return strings.Contains(normalized, "STOP")
}

func isTakeProfitOrderType(orderType string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(orderType))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "TAKE_PROFIT") || strings.Contains(normalized, "TAKEPROFIT")
}

func floatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	default:
		return 0, false
	}
}
