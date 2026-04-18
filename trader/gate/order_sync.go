package gate

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
)

// GateTrade represents a trade record from Gate fill history
type GateTrade struct {
	Symbol      string
	TradeID     string
	OrderID     string
	Side        string // buy or sell
	FillPrice   float64
	FillQty     float64 // In base currency (e.g., ETH), not contracts
	Fee         float64
	FeeAsset    string
	ExecTime    time.Time
	ProfitLoss  float64
	OrderType   string
	OrderAction string // open_long, open_short, close_long, close_short
}

type GateTriggeredOrder struct {
	TriggerOrderID string
	OrderID        string
	Symbol         string
	Side           string
	PositionSide   string
	TriggerPrice   float64
	OrderPrice     float64
	Quantity       float64
	Status         string
	OrderType      string
	TriggerSubtype string
	TriggerSource  string
	OrderAction    string
	ReduceOnly     bool
	ClosePosition  bool
	CreatedTime    time.Time
	UpdatedTime    time.Time
}

func inferGateTriggerSubtype(size int64, rule int32) string {
	switch {
	case size < 0:
		switch rule {
		case 1:
			return "stop_loss"
		case 2:
			return "take_profit"
		}
	case size > 0:
		switch rule {
		case 1:
			return "take_profit"
		case 2:
			return "stop_loss"
		}
	}
	return ""
}

func inferGateTriggerOrderAction(size int64) string {
	switch {
	case size < 0:
		return "close_long"
	case size > 0:
		return "close_short"
	default:
		return ""
	}
}

func normalizeGateTriggerOrderType(subtype string) string {
	switch strings.TrimSpace(subtype) {
	case "stop_loss":
		return "STOP_MARKET"
	case "take_profit":
		return "TAKE_PROFIT_MARKET"
	default:
		return "STOP_MARKET"
	}
}

func gateTriggerSourceLabel(priceType int32) string {
	switch priceType {
	case 1:
		return "mark_price"
	case 2:
		return "index_price"
	default:
		return "latest_price"
	}
}

func normalizeGateTriggerHistoryStatus(status, finishAs string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "open":
		return "NEW"
	case "finished":
		if strings.EqualFold(strings.TrimSpace(finishAs), "succeeded") {
			return "FILLED"
		}
		return "CANCELED"
	default:
		return strings.ToUpper(strings.TrimSpace(status))
	}
}

func gateFloatSecondsToTime(value float64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	ms := int64(value * 1000)
	return time.UnixMilli(ms).UTC()
}

// GetTrades retrieves trade/fill records from Gate
func (t *GateTrader) GetTrades(startTime time.Time, limit int) ([]GateTrade, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100 // Gate max limit
	}

	opts := &gateapi.GetMyTradesOpts{
		Limit: optional.NewInt32(int32(limit)),
	}

	// Get trades from Gate API
	trades, _, err := t.client.FuturesApi.GetMyTrades(t.ctx, "usdt", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade history: %w", err)
	}

	logger.Infof("📥 Received %d trades from Gate", len(trades))

	result := make([]GateTrade, 0, len(trades))

	for _, trade := range trades {
		// Filter by start time
		createTime := int64(trade.CreateTime)
		if createTime < startTime.Unix() {
			continue
		}

		fillPrice, err := strconv.ParseFloat(trade.Price, 64)
		if err != nil || fillPrice == 0 {
			logger.Infof("⚠️  Gate trade %d: fillPrice parse issue - raw='%s' parsed=%.8f err=%v",
				trade.Id, trade.Price, fillPrice, err)
		}

		// Get quanto_multiplier for this contract to convert size to base currency
		quantoMultiplier := 1.0
		contract, err := t.getContract(trade.Contract)
		if err == nil && contract != nil {
			qm, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
			if qm > 0 {
				quantoMultiplier = qm
			}
		}

		// Convert contract size to actual quantity
		absSize := trade.Size
		if absSize < 0 {
			absSize = -absSize
		}
		fillQty := float64(absSize) * quantoMultiplier

		// Determine side and order action based on size and close_size
		// Gate close_size field determines if trade is opening or closing:
		// close_size=0 && size>0: Open long
		// close_size=0 && size<0: Open short
		// close_size>0 && size>0: Close short (and possibly open long if size > close_size)
		// close_size<0 && size<0: Close long (and possibly open short if |size| > |close_size|)
		side := "BUY"
		orderAction := "open_long"

		if trade.Size > 0 {
			side = "BUY"
			if trade.CloseSize > 0 {
				// Closing short position
				orderAction = "close_short"
			} else {
				// Opening long position
				orderAction = "open_long"
			}
		} else if trade.Size < 0 {
			side = "SELL"
			if trade.CloseSize < 0 {
				// Closing long position
				orderAction = "close_long"
			} else {
				// Opening short position
				orderAction = "open_short"
			}
		}

		// Calculate fee (Gate returns fee as negative value)
		fee, _ := strconv.ParseFloat(trade.Fee, 64)
		if fee < 0 {
			fee = -fee
		}

		// For closed positions, estimate PnL (Gate doesn't directly provide it in trade record)
		pnl := 0.0
		if strings.Contains(orderAction, "close") {
			// PnL would need to be calculated from position history
			// For now, we leave it as 0 and let position builder handle it
		}

		gateTrade := GateTrade{
			Symbol:      trade.Contract,
			TradeID:     fmt.Sprintf("%d", trade.Id),
			OrderID:     trade.OrderId,
			Side:        side,
			FillPrice:   fillPrice,
			FillQty:     fillQty,
			Fee:         fee,
			FeeAsset:    "USDT",
			ExecTime:    time.Unix(createTime, 0).UTC(),
			ProfitLoss:  pnl,
			OrderType:   "MARKET",
			OrderAction: orderAction,
		}

		result = append(result, gateTrade)
	}

	return result, nil
}

func (t *GateTrader) GetTriggeredOrderHistory(startTime time.Time, limit int) ([]GateTriggeredOrder, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	opts := &gateapi.ListPriceTriggeredOrdersOpts{
		Limit: optional.NewInt32(int32(limit)),
	}
	orders, _, err := t.client.FuturesApi.ListPriceTriggeredOrders(t.ctx, "usdt", "finished", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get price-triggered order history: %w", err)
	}

	result := make([]GateTriggeredOrder, 0, len(orders))
	for _, order := range orders {
		finishTime := gateFloatSecondsToTime(order.FinishTime)
		if !startTime.IsZero() && !finishTime.IsZero() && finishTime.Before(startTime.UTC()) {
			continue
		}

		symbol := market.Normalize(strings.ReplaceAll(order.Initial.Contract, "_", ""))
		side := "BUY"
		positionSide := "SHORT"
		if order.Initial.Size < 0 {
			side = "SELL"
			positionSide = "LONG"
		}

		quantityContracts := float64(order.Initial.Size)
		if quantityContracts < 0 {
			quantityContracts = -quantityContracts
		}
		quantity := quantityContracts
		if contract, err := t.getContract(order.Initial.Contract); err == nil && contract != nil {
			if qm, parseErr := strconv.ParseFloat(contract.QuantoMultiplier, 64); parseErr == nil && qm > 0 {
				quantity = quantityContracts * qm
			}
		}

		triggerPrice, _ := strconv.ParseFloat(order.Trigger.Price, 64)
		orderPrice, _ := strconv.ParseFloat(order.Initial.Price, 64)
		triggerSubtype := inferGateTriggerSubtype(order.Initial.Size, order.Trigger.Rule)
		orderID := fmt.Sprintf("trigger:%d", order.Id)
		if order.TradeId > 0 {
			orderID = fmt.Sprintf("%d", order.TradeId)
		}

		result = append(result, GateTriggeredOrder{
			TriggerOrderID: fmt.Sprintf("%d", order.Id),
			OrderID:        orderID,
			Symbol:         symbol,
			Side:           side,
			PositionSide:   positionSide,
			TriggerPrice:   triggerPrice,
			OrderPrice:     orderPrice,
			Quantity:       quantity,
			Status:         normalizeGateTriggerHistoryStatus(order.Status, order.FinishAs),
			OrderType:      strings.TrimSpace(order.OrderType),
			TriggerSubtype: triggerSubtype,
			TriggerSource:  gateTriggerSourceLabel(order.Trigger.PriceType),
			OrderAction:    inferGateTriggerOrderAction(order.Initial.Size),
			ReduceOnly:     order.Initial.ReduceOnly || order.Initial.IsReduceOnly,
			ClosePosition:  order.Initial.Close || order.Initial.IsClose,
			CreatedTime:    gateFloatSecondsToTime(order.CreateTime),
			UpdatedTime:    finishTime,
		})
	}

	return result, nil
}

func (t *GateTrader) syncTriggerOrdersFromGate(traderID string, exchangeID string, exchangeType string, st *store.Store, startTime time.Time) (int, error) {
	triggerOrders, err := t.GetTriggeredOrderHistory(startTime, 100)
	if err != nil {
		return 0, fmt.Errorf("failed to get trigger order history: %w", err)
	}
	if len(triggerOrders) == 0 {
		return 0, nil
	}

	sort.Slice(triggerOrders, func(i, j int) bool {
		return triggerOrders[i].UpdatedTime.UnixMilli() < triggerOrders[j].UpdatedTime.UnixMilli()
	})

	syncedCount := 0
	for _, triggerOrder := range triggerOrders {
		updatedAt := triggerOrder.UpdatedTime.UTC().UnixMilli()
		if updatedAt == 0 {
			updatedAt = triggerOrder.CreatedTime.UTC().UnixMilli()
		}
		createdAt := triggerOrder.CreatedTime.UTC().UnixMilli()
		if createdAt == 0 {
			createdAt = updatedAt
		}

		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,
			ExchangeType:    exchangeType,
			ExchangeOrderID: triggerOrder.OrderID,
			Symbol:          triggerOrder.Symbol,
			Side:            triggerOrder.Side,
			PositionSide:    triggerOrder.PositionSide,
			Type:            normalizeGateTriggerOrderType(triggerOrder.TriggerSubtype),
			VenueOrderType:  triggerOrder.OrderType,
			TriggerSubtype:  triggerOrder.TriggerSubtype,
			TriggerSource:   triggerOrder.TriggerSource,
			Quantity:        triggerOrder.Quantity,
			Price:           triggerOrder.OrderPrice,
			StopPrice:       triggerOrder.TriggerPrice,
			Status:          triggerOrder.Status,
			ReduceOnly:      triggerOrder.ReduceOnly,
			ClosePosition:   triggerOrder.ClosePosition,
			OrderAction:     triggerOrder.OrderAction,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		}
		if triggerOrder.Status == "FILLED" {
			orderRecord.FilledQuantity = triggerOrder.Quantity
			orderRecord.FilledAt = updatedAt
		}

		if err := st.Order().UpsertOrder(orderRecord); err != nil {
			return syncedCount, fmt.Errorf("failed to upsert Gate trigger order %s: %w", triggerOrder.OrderID, err)
		}
		syncedCount++

		if triggerOrder.Status == "FILLED" {
			positions, err := st.Position().GetByExitOrderID(exchangeID, triggerOrder.OrderID)
			if err != nil {
				return syncedCount, fmt.Errorf("failed to load positions for Gate trigger order %s: %w", triggerOrder.OrderID, err)
			}
			for _, pos := range positions {
				if err := st.DealReview().SyncPosition(pos); err != nil {
					logger.Infof("  ⚠️ Failed to resync deal review for Gate trigger order %s / position %d: %v", triggerOrder.OrderID, pos.ID, err)
				}
			}
		}
	}

	return syncedCount, nil
}

// SyncOrdersFromGate syncs Gate exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("gate")
func (t *GateTrader) SyncOrdersFromGate(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing Gate trades from: %s", startTime.Format(time.RFC3339))

	triggerSyncedCount, err := t.syncTriggerOrdersFromGate(traderID, exchangeID, exchangeType, st, startTime)
	if err != nil {
		logger.Infof("⚠️  Gate trigger-order sync failed: %v", err)
	} else if triggerSyncedCount > 0 {
		logger.Infof("📥 Synced %d Gate trigger orders", triggerSyncedCount)
	}

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 100)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from Gate", len(trades))

	// Sort trades by time ASC (oldest first) for proper position building
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].ExecTime.UnixMilli() < trades[j].ExecTime.UnixMilli()
	})

	// Process trades one by one (no transaction to avoid deadlock)
	orderStore := st.Order()
	positionStore := st.Position()
	posBuilder := store.NewPositionBuilder(positionStore)
	syncedCount := 0

	for _, trade := range trades {
		// Normalize symbol (Gate uses BTC_USDT, normalize to BTCUSDT)
		symbol := market.Normalize(strings.ReplaceAll(trade.Symbol, "_", ""))

		// Determine position side from order action
		positionSide := "LONG"
		if strings.Contains(trade.OrderAction, "short") {
			positionSide = "SHORT"
		}

		execTimeMs := trade.ExecTime.UTC().UnixMilli()

		// Check if trade already exists (use exchangeID which is UUID, not exchange type)
		existing, err := orderStore.GetOrderByExchangeID(exchangeID, trade.TradeID)
		if err == nil && existing != nil {
			// Order exists, but still try to update position for close trades
			// This handles the case where order was created but position update failed
			if strings.HasPrefix(trade.OrderAction, "close_") && trade.FillPrice > 0 {
				if err := posBuilder.ProcessTrade(
					traderID, exchangeID, exchangeType,
					symbol, positionSide, trade.OrderAction,
					trade.FillQty, trade.FillPrice, trade.Fee, trade.ProfitLoss,
					execTimeMs, trade.TradeID,
				); err != nil {
					logger.Infof("  ⚠️ Retry position update for existing trade %s failed: %v", trade.TradeID, err)
				}
			}
			continue
		}

		// Normalize side for storage
		side := strings.ToUpper(trade.Side)

		// Create order record
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			ExchangeOrderID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    "BOTH", // Gate uses one-way position mode
			Type:            trade.OrderType,
			OrderAction:     trade.OrderAction,
			Quantity:        trade.FillQty,
			Price:           trade.FillPrice,
			Status:          "FILLED",
			FilledQuantity:  trade.FillQty,
			AvgFillPrice:    trade.FillPrice,
			Commission:      trade.Fee,
			FilledAt:        execTimeMs,
			CreatedAt:       execTimeMs,
			UpdatedAt:       execTimeMs,
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync trade %s: %v", trade.TradeID, err)
			continue
		}

		// Create fill record - use UTC time in milliseconds
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			OrderID:         orderRecord.ID,
			ExchangeOrderID: trade.OrderID,
			ExchangeTradeID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			Price:           trade.FillPrice,
			Quantity:        trade.FillQty,
			QuoteQuantity:   trade.FillPrice * trade.FillQty,
			Commission:      trade.Fee,
			CommissionAsset: trade.FeeAsset,
			RealizedPnL:     trade.ProfitLoss,
			IsMaker:         false,
			CreatedAt:       execTimeMs,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
		}

		// Create/update position record using PositionBuilder
		// Debug: Log the price being passed to ensure it's not 0
		if trade.FillPrice <= 0 {
			logger.Infof("  ⚠️ WARNING: trade %s has FillPrice=%.10f (invalid), skipping position update", trade.TradeID, trade.FillPrice)
		} else {
			if err := posBuilder.ProcessTrade(
				traderID, exchangeID, exchangeType,
				symbol, positionSide, trade.OrderAction,
				trade.FillQty, trade.FillPrice, trade.Fee, trade.ProfitLoss,
				execTimeMs, trade.TradeID,
			); err != nil {
				logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
			} else {
				logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f, price: %.10f)", trade.TradeID, trade.OrderAction, trade.FillQty, trade.FillPrice)
			}
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s",
			trade.TradeID, symbol, side, trade.FillQty, trade.FillPrice, trade.ProfitLoss, trade.Fee, trade.OrderAction)
	}

	logger.Infof("✅ Gate order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task for Gate
func (t *GateTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromGate(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  Gate order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Gate order sync started (interval: %v)", interval)
}
