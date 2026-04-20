package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"strings"
	"time"
)

// executeDecisionWithRecord executes AI decision and records detailed information
func (at *AutoTrader) executeDecisionWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction, record *store.DecisionRecord) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord, record)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord, record)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord, record)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord, record)
	case "hold", "wait":
		// No execution needed, just record
		return nil
	default:
		return fmt.Errorf("unknown action: %s", decision.Action)
	}
}

// executeOpenLongWithRecord executes open long position and records detailed information
func (at *AutoTrader) executeOpenLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction, record *store.DecisionRecord) error {
	logger.Infof("  📈 Open long: %s", decision.Symbol)

	if err := at.enforceSymbolBehaviorLiveGuard(record, actionRecord); err != nil {
		return err
	}
	if err := at.enforceLearnedPatternLiveGuard(record, actionRecord); err != nil {
		return err
	}

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit
	if err := at.enforceMaxPositions(len(positions)); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
			return fmt.Errorf("❌ %s already has long position, close it first", decision.Symbol)
		}
	}
	if err := at.enforceAdaptiveSameSymbolReentry(decision.Symbol); err != nil {
		return err
	}

	// Get current price
	marketData, err := market.GetWithExchange(decision.Symbol, at.exchange)
	if err != nil {
		return err
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Get equity for position value ratio check
	equity := 0.0
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		equity = eq
	} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		equity = eq
	} else {
		equity = availableBalance // Fallback to available balance
	}

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// ⚠️ Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, doesn't affect trading
	}

	// Open position
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}
	actionRecord.ExchangeOrderID = extractOrderIDFromResult(order)

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f", order["orderId"], quantity)

	// Record position opening time
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	stopLoss, takeProfit, protectionRefPrice, adjusted := at.normalizeOpenProtectionTargets(decision.Symbol, "LONG", marketData.CurrentPrice, decision.StopLoss, decision.TakeProfit)
	if adjusted {
		logger.Infof("  ⚠️ Rebased protective targets for %s LONG around exchange reference %.8f: SL %.8f -> %.8f, TP %.8f -> %.8f",
			decision.Symbol, protectionRefPrice, decision.StopLoss, stopLoss, decision.TakeProfit, takeProfit)
	}
	decision.StopLoss = stopLoss
	decision.TakeProfit = takeProfit

	// Set stop loss and take profit using the normalized exchange reference.
	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
		logger.Infof("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, decision.TakeProfit); err != nil {
		logger.Infof("  ⚠ Failed to set take profit: %v", err)
	}

	// Record order to database and poll for confirmation after the final protective targets are known.
	at.recordAndConfirmOrder(order, decision.Symbol, "open_long", quantity, marketData.CurrentPrice, decision.Leverage, 0, at.buildDealReviewDecisionInput(record, decision, actionRecord))
	at.seedTrailingStopState(decision.Symbol, "long", decision.StopLoss, decision.TakeProfit)

	return nil
}

// executeOpenShortWithRecord executes open short position and records detailed information
func (at *AutoTrader) executeOpenShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction, record *store.DecisionRecord) error {
	logger.Infof("  📉 Open short: %s", decision.Symbol)

	if err := at.enforceSymbolBehaviorLiveGuard(record, actionRecord); err != nil {
		return err
	}
	if err := at.enforceLearnedPatternLiveGuard(record, actionRecord); err != nil {
		return err
	}

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit
	if err := at.enforceMaxPositions(len(positions)); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
			return fmt.Errorf("❌ %s already has short position, close it first", decision.Symbol)
		}
	}
	if err := at.enforceAdaptiveSameSymbolReentry(decision.Symbol); err != nil {
		return err
	}

	// Get current price
	marketData, err := market.GetWithExchange(decision.Symbol, at.exchange)
	if err != nil {
		return err
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Get equity for position value ratio check
	equity := 0.0
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		equity = eq
	} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		equity = eq
	} else {
		equity = availableBalance // Fallback to available balance
	}

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// ⚠️ Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, doesn't affect trading
	}

	// Open position
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}
	actionRecord.ExchangeOrderID = extractOrderIDFromResult(order)

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f", order["orderId"], quantity)

	// Record position opening time
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	stopLoss, takeProfit, protectionRefPrice, adjusted := at.normalizeOpenProtectionTargets(decision.Symbol, "SHORT", marketData.CurrentPrice, decision.StopLoss, decision.TakeProfit)
	if adjusted {
		logger.Infof("  ⚠️ Rebased protective targets for %s SHORT around exchange reference %.8f: SL %.8f -> %.8f, TP %.8f -> %.8f",
			decision.Symbol, protectionRefPrice, decision.StopLoss, stopLoss, decision.TakeProfit, takeProfit)
	}
	decision.StopLoss = stopLoss
	decision.TakeProfit = takeProfit

	// Set stop loss and take profit using the normalized exchange reference.
	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
		logger.Infof("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, decision.TakeProfit); err != nil {
		logger.Infof("  ⚠ Failed to set take profit: %v", err)
	}

	// Record order to database and poll for confirmation after the final protective targets are known.
	at.recordAndConfirmOrder(order, decision.Symbol, "open_short", quantity, marketData.CurrentPrice, decision.Leverage, 0, at.buildDealReviewDecisionInput(record, decision, actionRecord))
	at.seedTrailingStopState(decision.Symbol, "short", decision.StopLoss, decision.TakeProfit)

	return nil
}

func (at *AutoTrader) normalizeOpenProtectionTargets(symbol, positionSide string, decisionReferencePrice, stopLoss, takeProfit float64) (float64, float64, float64, bool) {
	referencePrice := at.resolveOpenProtectionReferencePrice(symbol, positionSide, decisionReferencePrice)
	if referencePrice <= 0 {
		return stopLoss, takeProfit, decisionReferencePrice, false
	}

	normalizedStop, normalizedTake, adjusted := normalizeProtectionTargets(positionSide, referencePrice, decisionReferencePrice, stopLoss, takeProfit)
	return normalizedStop, normalizedTake, referencePrice, adjusted
}

func (at *AutoTrader) resolveOpenProtectionReferencePrice(symbol, positionSide string, fallbackPrice float64) float64 {
	expectedSymbol := market.Normalize(symbol)
	expectedSide := normalizeTrailingSide(positionSide)
	if expectedSymbol == "" || expectedSide == "" {
		return fallbackPrice
	}

	const attempts = 5
	for attempt := 0; attempt < attempts; attempt++ {
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				rawSymbol, _ := pos["symbol"].(string)
				if market.Normalize(rawSymbol) != expectedSymbol {
					continue
				}
				if normalizeTrailingSide(pos["side"]) != expectedSide {
					continue
				}
				if entryPrice, ok := floatValue(pos["entryPrice"]); ok && entryPrice > 0 {
					return entryPrice
				}
				if markPrice, ok := floatValue(pos["markPrice"]); ok && markPrice > 0 {
					return markPrice
				}
			}
		}
		if attempt < attempts-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	return fallbackPrice
}

func normalizeProtectionTargets(positionSide string, referencePrice, decisionReferencePrice, stopLoss, takeProfit float64) (float64, float64, bool) {
	if referencePrice <= 0 {
		return stopLoss, takeProfit, false
	}

	side := strings.ToUpper(strings.TrimSpace(positionSide))
	stopValid := protectionStopValid(side, referencePrice, stopLoss)
	takeValid := protectionTakeValid(side, referencePrice, takeProfit)
	if stopValid && takeValid {
		return stopLoss, takeProfit, false
	}

	if protectionStopValid(side, referencePrice, takeProfit) && protectionTakeValid(side, referencePrice, stopLoss) {
		return takeProfit, stopLoss, true
	}

	minOffset := math.Abs(referencePrice) * 0.0005
	if minOffset <= 0 {
		minOffset = 0.00000001
	}
	if decisionReferencePrice <= 0 {
		decisionReferencePrice = referencePrice
	}

	normalizedStop := stopLoss
	if stopLoss > 0 && !stopValid {
		stopDistance := math.Abs(stopLoss - decisionReferencePrice)
		if stopDistance < minOffset {
			stopDistance = minOffset
		}
		if side == "LONG" {
			normalizedStop = referencePrice - stopDistance
		} else {
			normalizedStop = referencePrice + stopDistance
		}
	}

	normalizedTake := takeProfit
	if takeProfit > 0 && !takeValid {
		takeDistance := math.Abs(takeProfit - decisionReferencePrice)
		if takeDistance < minOffset {
			takeDistance = minOffset
		}
		if side == "LONG" {
			normalizedTake = referencePrice + takeDistance
		} else {
			normalizedTake = referencePrice - takeDistance
		}
	}

	return normalizedStop, normalizedTake, normalizedStop != stopLoss || normalizedTake != takeProfit
}

func protectionStopValid(positionSide string, referencePrice, stopPrice float64) bool {
	if referencePrice <= 0 || stopPrice <= 0 {
		return false
	}

	switch strings.ToUpper(strings.TrimSpace(positionSide)) {
	case "LONG":
		return stopPrice < referencePrice
	case "SHORT":
		return stopPrice > referencePrice
	default:
		return false
	}
}

func protectionTakeValid(positionSide string, referencePrice, takeProfit float64) bool {
	if referencePrice <= 0 || takeProfit <= 0 {
		return false
	}

	switch strings.ToUpper(strings.TrimSpace(positionSide)) {
	case "LONG":
		return takeProfit > referencePrice
	case "SHORT":
		return takeProfit < referencePrice
	default:
		return false
	}
}

// executeCloseLongWithRecord executes close long position and records detailed information
func (at *AutoTrader) executeCloseLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction, record *store.DecisionRecord) error {
	logger.Infof("  🔄 Close long: %s", decision.Symbol)

	// Get current price
	marketData, err := market.GetWithExchange(decision.Symbol, at.exchange)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity - prioritize local database for accurate quantity
	var entryPrice float64
	var quantity float64

	// First try to get from local database (more accurate for quantity)
	if at.store != nil {
		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "LONG"); err == nil && openPos != nil {
			quantity = openPos.Quantity
			entryPrice = openPos.EntryPrice
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Fallback to exchange API if local data not found
	if quantity == 0 {
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
					if ep, ok := pos["entryPrice"].(float64); ok {
						entryPrice = ep
					}
					if amt, ok := pos["positionAmt"].(float64); ok && amt > 0 {
						quantity = amt
					}
					break
				}
			}
		}
		logger.Infof("  📊 Using exchange position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
	}

	// Close position
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = close all
	if err != nil {
		return err
	}
	actionRecord.ExchangeOrderID = extractOrderIDFromResult(order)

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "close_long", quantity, marketData.CurrentPrice, 0, entryPrice, at.buildDealReviewDecisionInput(record, decision, actionRecord))
	at.clearTrailingStopState(decision.Symbol, "long")

	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// executeCloseShortWithRecord executes close short position and records detailed information
func (at *AutoTrader) executeCloseShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction, record *store.DecisionRecord) error {
	logger.Infof("  🔄 Close short: %s", decision.Symbol)

	// Get current price
	marketData, err := market.GetWithExchange(decision.Symbol, at.exchange)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity - prioritize local database for accurate quantity
	var entryPrice float64
	var quantity float64

	// First try to get from local database (more accurate for quantity)
	if at.store != nil {
		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "SHORT"); err == nil && openPos != nil {
			quantity = openPos.Quantity
			entryPrice = openPos.EntryPrice
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Fallback to exchange API if local data not found
	if quantity == 0 {
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
					if ep, ok := pos["entryPrice"].(float64); ok {
						entryPrice = ep
					}
					if amt, ok := pos["positionAmt"].(float64); ok {
						quantity = -amt // positionAmt is negative for short
					}
					break
				}
			}
		}
		logger.Infof("  📊 Using exchange position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
	}

	// Close position
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = close all
	if err != nil {
		return err
	}
	actionRecord.ExchangeOrderID = extractOrderIDFromResult(order)

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "close_short", quantity, marketData.CurrentPrice, 0, entryPrice, at.buildDealReviewDecisionInput(record, decision, actionRecord))
	at.clearTrailingStopState(decision.Symbol, "short")

	logger.Infof("  ✓ Position closed successfully")
	return nil
}
