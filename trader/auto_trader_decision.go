package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"nofx/telemetry"
	"time"
)

// saveEquitySnapshot saves equity snapshot independently (for drawing profit curve, decoupled from AI decision)
func (at *AutoTrader) saveEquitySnapshot(ctx *kernel.Context) {
	if at.store == nil || ctx == nil {
		return
	}

	// Calculate per-trader PnL from positions
	var realizedPnL, netPnL float64
	if summary, err := at.store.Position().GetTraderPnLSummary(at.id); err == nil {
		realizedPnL = summary.RealizedPnL
		netPnL = realizedPnL + ctx.Account.UnrealizedPnL
	}

	snapshot := &store.EquitySnapshot{
		TraderID:      at.id,
		Timestamp:     time.Now().UTC(),
		TotalEquity:   ctx.Account.TotalEquity,
		Balance:       ctx.Account.TotalEquity - ctx.Account.UnrealizedPnL,
		UnrealizedPnL: ctx.Account.UnrealizedPnL,
		PositionCount: ctx.Account.PositionCount,
		MarginUsedPct: ctx.Account.MarginUsedPct,
		RealizedPnL:   realizedPnL,
		NetPnL:        netPnL,
	}

	if err := at.store.Equity().Save(snapshot); err != nil {
		logger.Infof("⚠️ Failed to save equity snapshot: %v", err)
	}
}

// saveDecision saves AI decision log to database (only records AI input/output, for debugging)
func (at *AutoTrader) saveDecision(record *store.DecisionRecord) error {
	if at.store == nil {
		return nil
	}

	if record.CycleNumber <= 0 {
		at.cycleNumber++
		record.CycleNumber = at.cycleNumber
	} else if record.CycleNumber > at.cycleNumber {
		at.cycleNumber = record.CycleNumber
	}
	record.TraderID = at.id

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	if err := at.store.Decision().LogDecision(record); err != nil {
		logger.Infof("⚠️ Failed to save decision record: %v", err)
		return err
	}
	if err := at.store.DealReview().CaptureDecisionCyclePricePoints(at.userID, record); err != nil {
		logger.Infof("⚠️ Failed to capture deal-review cycle price points: %v", err)
	}

	logger.Infof("📝 Decision record saved: trader=%s, cycle=%d", at.id, at.cycleNumber)
	return nil
}

func (at *AutoTrader) buildDealReviewDecisionInput(record *store.DecisionRecord, decision *kernel.Decision, actionRecord *store.DecisionAction) *store.DealReviewDecisionEventInput {
	if at.store == nil || record == nil || decision == nil {
		return nil
	}

	side := "LONG"
	stage := store.DealReviewStageOpen
	switch decision.Action {
	case "open_short", "close_short":
		side = "SHORT"
	}
	switch decision.Action {
	case "close_long", "close_short":
		stage = store.DealReviewStageClose
	}

	selectionBucket := ""
	var candidateSources []string
	for _, detail := range record.CandidateDetails {
		if detail.Symbol == decision.Symbol {
			selectionBucket = detail.SelectionBucket
			candidateSources = append(candidateSources, detail.Sources...)
			break
		}
	}

	snapshot := store.DealReviewEventSnapshot{
		AccountState:     record.AccountState,
		Positions:        append([]store.PositionSnapshot(nil), record.Positions...),
		CandidateCoins:   append([]string(nil), record.CandidateCoins...),
		CandidateDetails: append([]store.CandidateDetail(nil), record.CandidateDetails...),
		ExecutionLog:     append([]string(nil), record.ExecutionLog...),
		SystemPrompt:     record.SystemPrompt,
		UserPrompt:       record.InputPrompt,
		DecisionJSON:     record.DecisionJSON,
		RawResponse:      record.RawResponse,
		CoTTrace:         record.CoTTrace,
		AIRequestMs:      record.AIRequestDurationMs,
	}

	return &store.DealReviewDecisionEventInput{
		UserID:           at.userID,
		TraderID:         at.id,
		ExchangeID:       at.exchangeID,
		Stage:            stage,
		CycleNumber:      record.CycleNumber,
		DecisionTime:     actionRecord.Timestamp,
		Symbol:           decision.Symbol,
		Side:             side,
		Action:           decision.Action,
		Quantity:         actionRecord.Quantity,
		PositionSizeUSD:  decision.PositionSizeUSD,
		Price:            actionRecord.Price,
		Leverage:         decision.Leverage,
		StopLoss:         decision.StopLoss,
		TakeProfit:       decision.TakeProfit,
		Confidence:       decision.Confidence,
		Reasoning:        decision.Reasoning,
		SelectionBucket:  selectionBucket,
		CandidateSources: candidateSources,
		Snapshot:         snapshot,
	}
}

// GetStatus gets system status (for API)
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	at.isRunningMutex.RLock()
	isRunning := at.isRunning
	at.isRunningMutex.RUnlock()

	result := map[string]interface{}{
		"trader_id":       at.id,
		"trader_name":     at.name,
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_running":      isRunning,
		"start_time":      at.startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(at.startTime).Minutes()),
		"call_count":      at.callCount,
		"initial_balance": at.initialBalance,
		"scan_interval":   at.config.ScanInterval.String(),
		"invert_signals":  at.config.InvertSignals,
		"stop_until":      at.stopUntil.Format(time.RFC3339),
		"last_reset_time": at.lastResetTime.Format(time.RFC3339),
		"ai_provider":     aiProvider,
	}

	// Add strategy info
	if at.config.StrategyConfig != nil {
		result["strategy_type"] = at.config.StrategyConfig.StrategyType
		if at.config.StrategyConfig.GridConfig != nil {
			result["grid_symbol"] = at.config.StrategyConfig.GridConfig.Symbol
		}

		cfg := at.trailingStopConfig()
		reentryCfg := at.adaptiveReentryGuardConfig()
		at.trailingStopStateMu.RLock()
		trackedPositions := len(at.trailingStopState)
		at.trailingStopStateMu.RUnlock()
		result["trailing_stop"] = map[string]interface{}{
			"enabled":                     cfg.Enabled,
			"check_interval_sec":          cfg.CheckIntervalSec,
			"update_threshold_pct":        cfg.UpdateThresholdPct,
			"first_tighten_delay_sec":     cfg.FirstTightenDelaySec,
			"min_first_update_profit_pct": cfg.MinFirstUpdateProfitPct,
			"tier_count":                  len(cfg.Tiers),
			"tracked_positions":           trackedPositions,
		}
		result["adaptive_reentry_guard"] = map[string]interface{}{
			"enabled":                           reentryCfg.Enabled,
			"require_weak_execution_regime":     reentryCfg.RequireWeakExecutionRegime,
			"recent_trade_window":               reentryCfg.RecentTradeWindow,
			"min_recent_trades":                 reentryCfg.MinRecentTrades,
			"same_symbol_loss_cooldown_minutes": reentryCfg.SameSymbolLossCooldownMinutes,
			"pair_loss_lookback_hours":          reentryCfg.PairLossLookbackHours,
		}
	}

	return result
}

// GetAccountInfo gets account information (for API)
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Get account fields
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0
	totalEquity := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Use totalEquity directly if provided by trader (more accurate)
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		totalEquity = eq
	} else {
		// Fallback: Total Equity = Wallet balance + Unrealized profit
		totalEquity = totalWalletBalance + totalUnrealizedProfit
	}

	// Get positions to calculate total margin
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnLCalculated := 0.0
	for _, pos := range positions {
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		totalUnrealizedPnLCalculated += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// Verify unrealized P&L consistency (API value vs calculated from positions)
	// Note: Lighter API may return 0 for unrealized PnL, this is a known limitation
	diff := math.Abs(totalUnrealizedProfit - totalUnrealizedPnLCalculated)
	if diff > 5.0 { // Only warn if difference is significant (> 5 USDT)
		logger.Infof("⚠️ Unrealized P&L inconsistency (Lighter API limitation): API=%.4f, Calculated=%.4f, Diff=%.4f",
			totalUnrealizedProfit, totalUnrealizedPnLCalculated, diff)
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	} else {
		logger.Infof("⚠️ Initial Balance abnormal: %.2f, cannot calculate P&L percentage", at.initialBalance)
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// Core fields
		"total_equity":      totalEquity,           // Account equity = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // Wallet balance (excluding unrealized P&L)
		"unrealized_profit": totalUnrealizedProfit, // Unrealized P&L (official value from exchange API)
		"available_balance": availableBalance,      // Available balance

		// P&L statistics
		"total_pnl":       totalPnL,          // Total P&L = equity - initial
		"total_pnl_pct":   totalPnLPct,       // Total P&L percentage
		"initial_balance": at.initialBalance, // Initial balance
		"daily_pnl":       at.dailyPnL,       // Daily P&L

		// Position information
		"position_count":  len(positions),  // Position count
		"margin_used":     totalMarginUsed, // Margin used
		"margin_used_pct": marginUsedPct,   // Margin usage rate
	}, nil
}

// GetPerformance returns per-trader performance based on their own positions.
// Unlike GetAccountInfo which shows the shared exchange account equity,
// this method calculates PnL from the trader's own realized and unrealized trades.
func (at *AutoTrader) GetPerformance() (map[string]interface{}, error) {
	if at.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	// Get realized PnL summary from closed positions
	summary, err := at.store.Position().GetTraderPnLSummary(at.id)
	if err != nil {
		return nil, fmt.Errorf("failed to get PnL summary: %w", err)
	}

	// Get unrealized PnL from exchange for this trader's open positions
	unrealizedPnL := 0.0
	openPositionCount := 0
	totalMarginUsed := 0.0

	if summary.OpenCount > 0 {
		// Get the trader's tracked open positions from DB
		dbPositions, err := at.store.Position().GetOpenPositions(at.id)
		if err != nil {
			logger.Infof("⚠️ Failed to get open positions from DB for %s: %v", at.id, err)
		}

		// Get live positions from exchange to get current unrealized PnL
		exchangePositions, err := at.trader.GetPositions()
		if err != nil {
			logger.Infof("⚠️ Failed to get exchange positions for %s: %v", at.id, err)
		} else {
			// Build a set of symbols this trader owns (from DB positions)
			traderSymbols := make(map[string]bool)
			for _, dbPos := range dbPositions {
				sym := dbPos.Symbol
				traderSymbols[sym] = true
			}

			// Sum unrealized PnL only for positions belonging to this trader
			for _, pos := range exchangePositions {
				symbol := pos["symbol"].(string)
				if traderSymbols[symbol] {
					if pnl, ok := pos["unRealizedProfit"].(float64); ok {
						unrealizedPnL += pnl
					}
					openPositionCount++
					markPrice := pos["markPrice"].(float64)
					quantity := pos["positionAmt"].(float64)
					if quantity < 0 {
						quantity = -quantity
					}
					leverage := 10
					if lev, ok := pos["leverage"].(float64); ok {
						leverage = int(lev)
					}
					totalMarginUsed += (quantity * markPrice) / float64(leverage)
				}
			}
		}
	}

	// Net PnL = realized (from closed trades) + unrealized (from open positions) - fees
	netPnL := summary.RealizedPnL + unrealizedPnL
	netPnLPct := 0.0
	if at.initialBalance > 0 {
		netPnLPct = (netPnL / at.initialBalance) * 100
	}

	return map[string]interface{}{
		"realized_pnl":    summary.RealizedPnL,
		"unrealized_pnl":  unrealizedPnL,
		"total_fees":      summary.TotalFees,
		"net_pnl":         netPnL,
		"net_pnl_pct":     netPnLPct,
		"closed_trades":   summary.ClosedCount,
		"open_positions":  openPositionCount,
		"margin_used":     totalMarginUsed,
		"initial_balance": at.initialBalance,
	}, nil
}

// GetPositions gets position list (for API)
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		// Calculate margin used
		marginUsed := (quantity * markPrice) / float64(leverage)

		// Calculate P&L percentage (based on margin)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		result = append(result, map[string]interface{}{
			"symbol":             symbol,
			"side":               side,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		})
	}

	return result, nil
}

// recordAndConfirmOrder polls order status for actual fill data and records position
// action: open_long, open_short, close_long, close_short
// entryPrice: entry price when closing (0 when opening)
func (at *AutoTrader) recordAndConfirmOrder(orderResult map[string]interface{}, symbol, action string, quantity float64, price float64, leverage int, entryPrice float64, reviewInput *store.DealReviewDecisionEventInput) {
	if at.store == nil {
		return
	}

	orderID := extractOrderIDFromResult(orderResult)

	if orderID == "" || orderID == "0" {
		logger.Infof("  ⚠️ Order ID is empty, skipping record")
		return
	}
	if reviewInput != nil {
		reviewInput.ExchangeOrderID = orderID
		if _, err := at.store.DealReview().CreatePendingDecisionEvent(reviewInput); err != nil {
			logger.Infof("  ⚠️ Failed to create deal-review event: %v", err)
		}
		if action == "close_long" || action == "close_short" {
			at.recordAICloseExitIntent(orderID, action, quantity, price, entryPrice, reviewInput)
		}
	}

	// Determine positionSide
	var positionSide string
	switch action {
	case "open_long", "close_long":
		positionSide = "LONG"
	case "open_short", "close_short":
		positionSide = "SHORT"
	}

	var actualPrice = price
	var actualQty = quantity
	var fee float64

	// Exchanges with OrderSync: Skip immediate order recording, let OrderSync handle it
	// This ensures accurate data from GetTrades API and avoids duplicate records
	switch at.exchange {
	case "binance", "lighter", "hyperliquid", "bybit", "okx", "bitget", "aster", "kucoin", "gate":
		logger.Infof("  📝 Order submitted (id: %s), will be synced by OrderSync", orderID)
		return
	}

	// For exchanges without OrderSync (e.g., Binance): record immediately and poll for fill data
	orderRecord := at.createOrderRecord(orderID, symbol, action, positionSide, quantity, price, leverage)
	if err := at.store.Order().CreateOrder(orderRecord); err != nil {
		logger.Infof("  ⚠️ Failed to record order: %v", err)
	} else {
		logger.Infof("  📝 Order recorded: %s [%s] %s", orderID, action, symbol)
	}

	// Wait for order to be filled and get actual fill data
	time.Sleep(500 * time.Millisecond)
	for i := 0; i < 5; i++ {
		status, err := at.trader.GetOrderStatus(symbol, orderID)
		if err == nil {
			statusStr, _ := status["status"].(string)
			if statusStr == "FILLED" {
				// Get actual fill price
				if avgPrice, ok := status["avgPrice"].(float64); ok && avgPrice > 0 {
					actualPrice = avgPrice
				}
				// Get actual executed quantity
				if execQty, ok := status["executedQty"].(float64); ok && execQty > 0 {
					actualQty = execQty
				}
				// Get commission/fee
				if commission, ok := status["commission"].(float64); ok {
					fee = commission
				}
				logger.Infof("  ✅ Order filled: avgPrice=%.6f, qty=%.6f, fee=%.6f", actualPrice, actualQty, fee)

				// Update order status to FILLED
				if err := at.store.Order().UpdateOrderStatus(orderRecord.ID, "FILLED", actualQty, actualPrice, fee); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
				}

				// Record fill details
				at.recordOrderFill(orderRecord.ID, orderID, symbol, action, actualPrice, actualQty, fee)
				break
			} else if statusStr == "CANCELED" || statusStr == "EXPIRED" || statusStr == "REJECTED" {
				logger.Infof("  ⚠️ Order %s, skipping position record", statusStr)
				if reviewInput != nil {
					if err := at.store.DealReview().CancelPendingEvent(at.id, orderID, reviewInput.Stage); err != nil {
						logger.Infof("  ⚠️ Failed to cancel pending deal-review event: %v", err)
					}
				}
				if action == "close_long" || action == "close_short" {
					if err := at.store.DealReview().CancelExitIntent(at.id, orderID); err != nil {
						logger.Infof("  ⚠️ Failed to cancel exit intent: %v", err)
					}
				}
				// Update order status
				if err := at.store.Order().UpdateOrderStatus(orderRecord.ID, statusStr, 0, 0, 0); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
				}
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Normalize symbol for position record consistency
	normalizedSymbolForPosition := market.Normalize(symbol)

	logger.Infof("  📝 Recording position (ID: %s, action: %s, price: %.6f, qty: %.6f, fee: %.4f)",
		orderID, action, actualPrice, actualQty, fee)

	// Record position change with actual fill data (use normalized symbol)
	at.recordPositionChange(orderID, normalizedSymbolForPosition, positionSide, action, actualQty, actualPrice, leverage, entryPrice, fee)

	// Send anonymous trade statistics for experience improvement (async, non-blocking)
	// This helps us understand overall product usage across all deployments
	telemetry.TrackTrade(telemetry.TradeEvent{
		Exchange:  at.exchange,
		TradeType: action,
		Symbol:    symbol,
		AmountUSD: actualPrice * actualQty,
		Leverage:  leverage,
		UserID:    at.userID,
		TraderID:  at.id,
	})
}

// recordPositionChange records position change (create record on open, update record on close)
func (at *AutoTrader) recordPositionChange(orderID, symbol, side, action string, quantity, price float64, leverage int, entryPrice float64, fee float64) {
	if at.store == nil {
		return
	}

	switch action {
	case "open_long", "open_short":
		// Open position: create new position record
		nowMs := time.Now().UTC().UnixMilli()
		pos := &store.TraderPosition{
			TraderID:     at.id,
			ExchangeID:   at.exchangeID, // Exchange account UUID
			ExchangeType: at.exchange,   // Exchange type: binance/bybit/okx/etc
			Symbol:       symbol,
			Side:         side, // LONG or SHORT
			Quantity:     quantity,
			EntryPrice:   price,
			EntryOrderID: orderID,
			EntryTime:    nowMs,
			Leverage:     leverage,
			Status:       "OPEN",
			CreatedAt:    nowMs,
			UpdatedAt:    nowMs,
		}
		if err := at.store.Position().Create(pos); err != nil {
			logger.Infof("  ⚠️ Failed to record position: %v", err)
		} else {
			logger.Infof("  📊 Position recorded [%s] %s %s @ %.4f", at.id[:8], symbol, side, price)
		}

	case "close_long", "close_short":
		// Close position using PositionBuilder for consistent handling
		// PositionBuilder will handle both cases:
		// 1. If open position exists: close it properly
		// 2. If no open position (e.g., table cleared): create a closed position record
		posBuilder := store.NewPositionBuilder(at.store.Position())
		if err := posBuilder.ProcessTrade(
			at.id, at.exchangeID, at.exchange,
			symbol, side, action,
			quantity, price, fee, 0, // realizedPnL will be calculated
			time.Now().UTC().UnixMilli(), orderID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to process close position: %v", err)
		} else {
			logger.Infof("  ✅ Position closed [%s] %s %s @ %.4f", at.id[:8], symbol, side, price)
		}
	}
}

// createOrderRecord creates an order record struct from order details
func (at *AutoTrader) createOrderRecord(orderID, symbol, action, positionSide string, quantity, price float64, leverage int) *store.TraderOrder {
	// Determine order type (market for auto trader)
	orderType := "MARKET"

	// Determine side (BUY/SELL)
	var side string
	switch action {
	case "open_long", "close_short":
		side = "BUY"
	case "open_short", "close_long":
		side = "SELL"
	}

	// Use action as orderAction directly (keep lowercase format)
	orderAction := action

	// Determine if it's a reduce only order
	reduceOnly := (action == "close_long" || action == "close_short")

	// Normalize symbol for consistency
	normalizedSymbol := market.Normalize(symbol)

	return &store.TraderOrder{
		TraderID:        at.id,
		ExchangeID:      at.exchangeID,
		ExchangeType:    at.exchange,
		ExchangeOrderID: orderID,
		Symbol:          normalizedSymbol,
		Side:            side,
		PositionSide:    positionSide,
		Type:            orderType,
		TimeInForce:     "GTC",
		Quantity:        quantity,
		Price:           price,
		Status:          "NEW",
		FilledQuantity:  0,
		AvgFillPrice:    0,
		Commission:      0,
		CommissionAsset: "USDT",
		Leverage:        leverage,
		ReduceOnly:      reduceOnly,
		ClosePosition:   reduceOnly,
		OrderAction:     orderAction,
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
	}
}

// recordOrderFill records order fill/trade details
func (at *AutoTrader) recordOrderFill(orderRecordID int64, exchangeOrderID, symbol, action string, price, quantity, fee float64) {
	if at.store == nil {
		return
	}

	// Determine side (BUY/SELL)
	var side string
	switch action {
	case "open_long", "close_short":
		side = "BUY"
	case "open_short", "close_long":
		side = "SELL"
	}

	// Generate a simple trade ID (exchange doesn't always provide one)
	tradeID := fmt.Sprintf("%s-%d", exchangeOrderID, time.Now().UnixNano())

	// Normalize symbol for consistency
	normalizedSymbol := market.Normalize(symbol)

	fill := &store.TraderFill{
		TraderID:        at.id,
		ExchangeID:      at.exchangeID,
		ExchangeType:    at.exchange,
		OrderID:         orderRecordID,
		ExchangeOrderID: exchangeOrderID,
		ExchangeTradeID: tradeID,
		Symbol:          normalizedSymbol,
		Side:            side,
		Price:           price,
		Quantity:        quantity,
		QuoteQuantity:   price * quantity,
		Commission:      fee,
		CommissionAsset: "USDT",
		RealizedPnL:     0,     // Will be calculated for close orders
		IsMaker:         false, // Market orders are usually taker
		CreatedAt:       time.Now().UTC().UnixMilli(),
	}

	// Calculate realized PnL for close orders
	if action == "close_long" || action == "close_short" {
		// Try to get the entry price from the open position
		var positionSide string
		if action == "close_long" {
			positionSide = "LONG"
		} else {
			positionSide = "SHORT"
		}

		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, positionSide); err == nil && openPos != nil {
			if positionSide == "LONG" {
				fill.RealizedPnL = (price - openPos.EntryPrice) * quantity
			} else {
				fill.RealizedPnL = (openPos.EntryPrice - price) * quantity
			}
		}
	}

	if err := at.store.Order().CreateFill(fill); err != nil {
		logger.Infof("  ⚠️ Failed to record fill: %v", err)
	} else {
		logger.Infof("  📋 Fill recorded: %.4f @ %.6f, fee: %.4f", quantity, price, fee)
	}
}

// GetOpenOrders returns open orders (pending SL/TP) from exchange
func (at *AutoTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	return at.trader.GetOpenOrders(symbol)
}
