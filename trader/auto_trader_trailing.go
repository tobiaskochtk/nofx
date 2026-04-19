package trader

import (
	"fmt"
	"math"
	"nofx/logger"
	"nofx/store"
	tradertypes "nofx/trader/types"
	"strconv"
	"strings"
	"time"
)

type trailingStopPositionState struct {
	LastStopPrice    float64
	HasLastStopPrice bool
	TakeProfitPrice  float64
	HasTakeProfit    bool
	HighestTierIndex int
	FirstSeenAtMs    int64
	HasActivated     bool
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
		at.updateTrailingStops()

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

	at.updateTrailingStopsWithPositions(positions, time.Now().UTC())
}

func (at *AutoTrader) updateTrailingStopsWithPositions(positions []map[string]interface{}, checkTime time.Time) {
	cfg := at.trailingStopConfig()
	if !cfg.Enabled || len(positions) == 0 {
		return
	}
	if checkTime.IsZero() {
		checkTime = time.Now().UTC()
	}
	nowMs := checkTime.UTC().UnixMilli()
	if nowMs <= 0 {
		nowMs = time.Now().UTC().UnixMilli()
	}

	at.trailingStopEvalMu.Lock()
	defer at.trailingStopEvalMu.Unlock()

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
		positionStartMs := at.resolveTrailingPositionStartMs(symbol, side, pos, state)
		if positionStartMs > 0 {
			state.FirstSeenAtMs = positionStartMs
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
		if !state.HasActivated {
			if cfg.FirstTightenDelaySec > 0 && positionStartMs > 0 {
				minAllowedUpdateMs := positionStartMs + int64(cfg.FirstTightenDelaySec)*1000
				if nowMs < minAllowedUpdateMs {
					at.saveTrailingStopPositionState(symbol, side, state)
					continue
				}
			}
			if cfg.MinFirstUpdateProfitPct > 0 && profitPct < cfg.MinFirstUpdateProfitPct {
				at.saveTrailingStopPositionState(symbol, side, state)
				continue
			}
		}
		targetStopProfitPct := calculateTargetStopProfitPct(activeTier, profitPct)
		newStopPrice := calculateTrailingStopPrice(side, entryPrice, markPrice, leverage, targetStopProfitPct)
		if newStopPrice <= 0 {
			continue
		}
		unrealizedPnL, unrealizedPnLPct := resolveTrailingUpdateUnrealizedPnL(pos, side, entryPrice, markPrice, quantity)
		protectsBreakeven := trailingStopProtectsBreakeven(side, entryPrice, newStopPrice)

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

		previousStopPrice := state.LastStopPrice
		if err := at.applyTrailingStopUpdate(symbol, side, quantity, newStopPrice, state); err != nil {
			logger.Infof("⚠️ Trailing stop update failed for %s %s: %v", symbol, side, err)
			at.saveTrailingStopPositionState(symbol, side, state)
			continue
		}

		state.LastStopPrice = newStopPrice
		state.HasLastStopPrice = true
		state.HasActivated = true
		at.saveTrailingStopPositionState(symbol, side, state)
		at.recordTrailingStopUpdate(symbol, side, previousStopPrice, newStopPrice, state, quantity, entryPrice, markPrice, int(leverage), profitPct, unrealizedPnL, unrealizedPnLPct, targetStopProfitPct, protectsBreakeven, activeTierIndex, activeTier)

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

func (at *AutoTrader) recordTrailingStopUpdate(symbol, side string, previousStopPrice, newStopPrice float64, state *trailingStopPositionState, quantity, entryPrice, markPrice float64, leverage int, profitPct, unrealizedPnL, unrealizedPnLPct, stopProfitPct float64, protectsBreakeven bool, tierIndex int, tier store.TrailingStopTier) {
	if at == nil || at.store == nil || newStopPrice <= 0 {
		return
	}

	input := &store.DealReviewTrailingUpdateInput{
		UserID:               at.userID,
		TraderID:             at.id,
		ExchangeID:           at.exchangeID,
		Symbol:               symbol,
		Side:                 side,
		Timestamp:            time.Now().UTC(),
		PreviousStopPrice:    previousStopPrice,
		NewStopPrice:         newStopPrice,
		Quantity:             quantity,
		EntryPrice:           entryPrice,
		MarkPrice:            markPrice,
		Leverage:             leverage,
		ProfitPct:            profitPct,
		UnrealizedPnL:        unrealizedPnL,
		UnrealizedPnLPct:     unrealizedPnLPct,
		StopProfitPct:        stopProfitPct,
		ProtectsBreakeven:    protectsBreakeven,
		TierIndex:            tierIndex,
		TierTriggerProfitPct: tier.TriggerProfitPct,
		TrailingMode:         tier.Mode,
		LockProfitPct:        tier.LockProfitPct,
		TrailOffsetPct:       tier.TrailOffsetPct,
	}
	if state != nil && state.HasTakeProfit {
		input.TakeProfitPrice = state.TakeProfitPrice
	}
	if err := at.store.DealReview().RecordTrailingStopUpdate(input); err != nil {
		logger.Warnf("⚠️ Failed to persist trailing-stop update for %s %s: %v", symbol, side, err)
	}
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
	if state.FirstSeenAtMs == 0 && at.positionFirstSeenTime != nil {
		if firstSeen, ok := at.positionFirstSeenTime[trailingStopPositionKey(symbol, side)]; ok && firstSeen > 0 {
			state.FirstSeenAtMs = firstSeen
		}
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
		state.HasActivated = at.isTrailingStopAlreadyActivated(symbol, side, stopLoss)
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

func (at *AutoTrader) resolveTrailingPositionStartMs(symbol, side string, pos map[string]interface{}, state *trailingStopPositionState) int64 {
	if state != nil && state.FirstSeenAtMs > 0 {
		return state.FirstSeenAtMs
	}

	if at != nil && at.store != nil {
		if dbPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, strings.ToUpper(side)); err == nil && dbPos != nil && dbPos.EntryTime > 0 {
			return dbPos.EntryTime
		}
	}

	if createdTime := trailingPositionCreatedTimeMs(pos); createdTime > 0 {
		return createdTime
	}

	if at == nil {
		return 0
	}
	if at.positionFirstSeenTime == nil {
		at.positionFirstSeenTime = make(map[string]int64)
	}
	key := trailingStopPositionKey(symbol, side)
	if firstSeen, ok := at.positionFirstSeenTime[key]; ok && firstSeen > 0 {
		return firstSeen
	}

	nowMs := time.Now().UTC().UnixMilli()
	at.positionFirstSeenTime[key] = nowMs
	return nowMs
}

func (at *AutoTrader) isTrailingStopAlreadyActivated(symbol, side string, currentStopPrice float64) bool {
	if at == nil || at.store == nil || currentStopPrice <= 0 {
		return false
	}
	caseRec, err := at.store.DealReview().GetLatestOpenCaseBySymbol(at.userID, at.id, symbol, strings.ToUpper(side))
	if err != nil || caseRec == nil || caseRec.OpenStopLoss <= 0 {
		return false
	}
	return isTrailingStopMovedFavorably(side, caseRec.OpenStopLoss, currentStopPrice)
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

func trailingPositionCreatedTimeMs(pos map[string]interface{}) int64 {
	if pos == nil {
		return 0
	}
	for _, key := range []string{"createdTime", "created_at_ms", "entryTime"} {
		if value, ok := pos[key]; ok {
			if numeric, ok := floatValue(value); ok && numeric > 0 {
				return int64(numeric)
			}
		}
	}
	return 0
}

func resolveTrailingUpdateUnrealizedPnL(pos map[string]interface{}, side string, entryPrice, markPrice, quantity float64) (float64, float64) {
	unrealizedPnL := 0.0
	found := false
	if pos != nil {
		for _, key := range []string{"unRealizedProfit", "unrealizedProfit", "unrealized_pnl", "unrealizedPnL"} {
			if value, ok := pos[key]; ok {
				if numeric, ok := floatValue(value); ok {
					unrealizedPnL = numeric
					found = true
					break
				}
			}
		}
	}
	if !found {
		unrealizedPnL = calculateLinearUnrealizedPnL(side, entryPrice, markPrice, quantity)
	}

	notional := math.Abs(entryPrice * quantity)
	if notional <= 0 {
		return unrealizedPnL, 0
	}
	return unrealizedPnL, unrealizedPnL / notional * 100
}

func calculateLinearUnrealizedPnL(side string, entryPrice, markPrice, quantity float64) float64 {
	if entryPrice <= 0 || markPrice <= 0 || quantity <= 0 {
		return 0
	}
	if normalizeTrailingSide(side) == "long" {
		return (markPrice - entryPrice) * quantity
	}
	return (entryPrice - markPrice) * quantity
}

func isTrailingStopMovedFavorably(side string, initialStopPrice, currentStopPrice float64) bool {
	if initialStopPrice <= 0 || currentStopPrice <= 0 {
		return false
	}
	const movedStopTolerance = 0.001 // 0.10%
	switch normalizeTrailingSide(side) {
	case "long":
		return currentStopPrice > initialStopPrice &&
			!nearlyEqualRelative(currentStopPrice, initialStopPrice, movedStopTolerance)
	case "short":
		return currentStopPrice < initialStopPrice &&
			!nearlyEqualRelative(currentStopPrice, initialStopPrice, movedStopTolerance)
	default:
		return false
	}
}

func trailingStopProtectsBreakeven(side string, entryPrice, stopPrice float64) bool {
	if entryPrice <= 0 || stopPrice <= 0 {
		return false
	}
	switch normalizeTrailingSide(side) {
	case "long":
		return stopPrice >= entryPrice
	case "short":
		return stopPrice <= entryPrice
	default:
		return false
	}
}

func nearlyEqualRelative(left, right, tolerance float64) bool {
	if left <= 0 || right <= 0 {
		return false
	}
	diff := math.Abs(left - right)
	base := math.Max(math.Abs(left), math.Abs(right))
	if base == 0 {
		return false
	}
	return diff/base <= tolerance
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
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
