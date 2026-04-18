package okx

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strconv"
	"strings"
	"time"
)

// OKXTrade represents a trade record from OKX fills history
type OKXTrade struct {
	InstID      string
	Symbol      string
	TradeID     string
	OrderID     string
	Side        string // buy or sell
	PosSide     string // long or short
	FillPrice   float64
	FillQty     float64 // In contracts
	FillQtyBase float64 // In base asset (BTC, ETH, etc)
	Fee         float64
	FeeAsset    string
	ExecTime    time.Time
	IsMaker     bool
	OrderType   string
	OrderAction string // open_long, open_short, close_long, close_short
}

type OKXAlgoOrder struct {
	AlgoID         string
	OrderID        string
	AlgoClOrdID    string
	Symbol         string
	Side           string
	PositionSide   string
	OrderType      string
	TriggerSubtype string
	TriggerSource  string
	TriggerPrice   float64
	OrderPrice     float64
	Quantity       float64
	Status         string
	ReduceOnly     bool
	CreatedTime    time.Time
	UpdatedTime    time.Time
	OrderAction    string
}

func normalizeOKXAlgoOrderStatus(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "filled":
		return "FILLED"
	case "effective", "live":
		return "NEW"
	case "canceled", "order_failed", "failed":
		return "CANCELED"
	default:
		return strings.ToUpper(strings.TrimSpace(state))
	}
}

func inferOKXTriggerOrderAction(side, posSide string) string {
	switch {
	case strings.EqualFold(strings.TrimSpace(side), "sell") && strings.EqualFold(strings.TrimSpace(posSide), "long"):
		return "close_long"
	case strings.EqualFold(strings.TrimSpace(side), "buy") && strings.EqualFold(strings.TrimSpace(posSide), "short"):
		return "close_short"
	default:
		return ""
	}
}

func normalizeOKXAlgoTriggerOrderType(subtype string) string {
	switch strings.TrimSpace(subtype) {
	case "stop_loss":
		return "STOP_MARKET"
	case "take_profit":
		return "TAKE_PROFIT_MARKET"
	default:
		return "STOP_MARKET"
	}
}

func parseOKXAlgoOrderKind(
	slTriggerPx string,
	tpTriggerPx string,
	triggerPx string,
	slOrdPx string,
	tpOrdPx string,
	triggerPxType string,
	slTriggerPxType string,
	tpTriggerPxType string,
) (subtype string, triggerPrice float64, orderPrice float64, source string) {
	if price, err := strconv.ParseFloat(strings.TrimSpace(slTriggerPx), 64); err == nil && price > 0 {
		orderPrice, _ = strconv.ParseFloat(strings.TrimSpace(slOrdPx), 64)
		return "stop_loss", price, orderPrice, strings.TrimSpace(slTriggerPxType)
	}
	if price, err := strconv.ParseFloat(strings.TrimSpace(tpTriggerPx), 64); err == nil && price > 0 {
		orderPrice, _ = strconv.ParseFloat(strings.TrimSpace(tpOrdPx), 64)
		return "take_profit", price, orderPrice, strings.TrimSpace(tpTriggerPxType)
	}
	if price, err := strconv.ParseFloat(strings.TrimSpace(triggerPx), 64); err == nil && price > 0 {
		return "", price, 0, strings.TrimSpace(triggerPxType)
	}
	return "", 0, 0, ""
}

func parseOKXBoolField(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "true" || normalized == "1"
}

func parseOKXTimeField(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	ms, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms).UTC(), nil
}

// GetTrades retrieves trade/fill records from OKX
func (t *OKXTrader) GetTrades(startTime time.Time, limit int) ([]OKXTrade, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100 // OKX max limit is 100
	}

	// Build query path
	// OKX fills-history endpoint for historical fills
	path := fmt.Sprintf("/api/v5/trade/fills-history?instType=SWAP&limit=%d", limit)
	if !startTime.IsZero() {
		path += fmt.Sprintf("&begin=%d", startTime.UnixMilli())
	}

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get fills history: %w", err)
	}

	var fills []struct {
		InstID   string `json:"instId"`   // e.g., "BTC-USDT-SWAP"
		TradeID  string `json:"tradeId"`  // Trade ID
		OrdID    string `json:"ordId"`    // Order ID
		BillID   string `json:"billId"`   // Bill ID
		Side     string `json:"side"`     // buy or sell
		PosSide  string `json:"posSide"`  // long, short, or net
		FillPx   string `json:"fillPx"`   // Fill price
		FillSz   string `json:"fillSz"`   // Fill size (contracts)
		Fee      string `json:"fee"`      // Fee (negative for cost)
		FeeCcy   string `json:"feeCcy"`   // Fee currency
		Ts       string `json:"ts"`       // Trade timestamp (ms)
		ExecType string `json:"execType"` // T: taker, M: maker
		Tag      string `json:"tag"`      // Order tag
	}

	if err := json.Unmarshal(data, &fills); err != nil {
		return nil, fmt.Errorf("failed to parse fills: %w", err)
	}

	trades := make([]OKXTrade, 0, len(fills))

	for _, fill := range fills {
		fillPrice, _ := strconv.ParseFloat(fill.FillPx, 64)
		fillSz, _ := strconv.ParseFloat(fill.FillSz, 64)
		fee, _ := strconv.ParseFloat(fill.Fee, 64)
		ts, _ := strconv.ParseInt(fill.Ts, 10, 64)

		// Convert symbol: BTC-USDT-SWAP -> BTCUSDT
		symbol := t.convertSymbolBack(fill.InstID)

		// Convert contract count to base asset quantity
		fillQtyBase := fillSz
		inst, err := t.getInstrument(symbol)
		if err == nil && inst.CtVal > 0 {
			fillQtyBase = fillSz * inst.CtVal
		}

		// Determine order action based on side and posSide
		// OKX uses dual position mode:
		// - buy + long = open long
		// - sell + long = close long
		// - sell + short = open short
		// - buy + short = close short
		orderAction := "open_long"
		posSide := strings.ToLower(fill.PosSide)
		side := strings.ToLower(fill.Side)

		if posSide == "long" {
			if side == "buy" {
				orderAction = "open_long"
			} else {
				orderAction = "close_long"
			}
		} else if posSide == "short" {
			if side == "sell" {
				orderAction = "open_short"
			} else {
				orderAction = "close_short"
			}
		} else {
			// One-way mode (net position)
			if side == "buy" {
				orderAction = "open_long"
			} else {
				orderAction = "open_short"
			}
		}

		trade := OKXTrade{
			InstID:      fill.InstID,
			Symbol:      symbol,
			TradeID:     fill.TradeID,
			OrderID:     fill.OrdID,
			Side:        fill.Side,
			PosSide:     fill.PosSide,
			FillPrice:   fillPrice,
			FillQty:     fillSz,
			FillQtyBase: fillQtyBase,
			Fee:         -fee, // OKX returns negative fee
			FeeAsset:    fill.FeeCcy,
			ExecTime:    time.UnixMilli(ts).UTC(),
			IsMaker:     fill.ExecType == "M",
			OrderType:   "MARKET",
			OrderAction: orderAction,
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

func (t *OKXTrader) getAlgoOrderHistoryByState(startTime time.Time, limit int, state string) ([]OKXAlgoOrder, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	path := fmt.Sprintf("%s?ordType=conditional&state=%s&limit=%d", okxAlgoHistoryPath, state, limit)
	if !startTime.IsZero() {
		path += fmt.Sprintf("&begin=%d", startTime.UnixMilli())
	}

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get algo order history: %w", err)
	}

	var history []struct {
		AlgoID          string `json:"algoId"`
		AlgoClOrdID     string `json:"algoClOrdId"`
		OrdID           string `json:"ordId"`
		InstID          string `json:"instId"`
		Side            string `json:"side"`
		PosSide         string `json:"posSide"`
		OrdType         string `json:"ordType"`
		State           string `json:"state"`
		Sz              string `json:"sz"`
		ReduceOnly      string `json:"reduceOnly"`
		CTime           string `json:"cTime"`
		UTime           string `json:"uTime"`
		TriggerPx       string `json:"triggerPx"`
		TriggerPxType   string `json:"triggerPxType"`
		SlTriggerPx     string `json:"slTriggerPx"`
		SlTriggerPxType string `json:"slTriggerPxType"`
		SlOrdPx         string `json:"slOrdPx"`
		TpTriggerPx     string `json:"tpTriggerPx"`
		TpTriggerPxType string `json:"tpTriggerPxType"`
		TpOrdPx         string `json:"tpOrdPx"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse algo order history: %w", err)
	}

	result := make([]OKXAlgoOrder, 0, len(history))
	for _, item := range history {
		orderAction := inferOKXTriggerOrderAction(item.Side, item.PosSide)
		if orderAction == "" || strings.TrimSpace(item.InstID) == "" {
			continue
		}

		symbol := t.convertSymbolBack(item.InstID)
		quantityContracts, _ := strconv.ParseFloat(strings.TrimSpace(item.Sz), 64)
		quantity := quantityContracts
		if inst, err := t.getInstrument(symbol); err == nil && inst != nil && inst.CtVal > 0 {
			quantity = quantityContracts * inst.CtVal
		}
		subtype, triggerPrice, orderPrice, source := parseOKXAlgoOrderKind(
			item.SlTriggerPx,
			item.TpTriggerPx,
			item.TriggerPx,
			item.SlOrdPx,
			item.TpOrdPx,
			item.TriggerPxType,
			item.SlTriggerPxType,
			item.TpTriggerPxType,
		)

		orderID := strings.TrimSpace(item.OrdID)
		if orderID == "" {
			orderID = strings.TrimSpace(item.AlgoID)
		}

		createdTime, _ := parseOKXTimeField(item.CTime)
		updatedTime, _ := parseOKXTimeField(item.UTime)
		if updatedTime.IsZero() {
			updatedTime = createdTime
		}

		positionSide := "LONG"
		if strings.Contains(orderAction, "short") {
			positionSide = "SHORT"
		}

		result = append(result, OKXAlgoOrder{
			AlgoID:         strings.TrimSpace(item.AlgoID),
			OrderID:        orderID,
			AlgoClOrdID:    strings.TrimSpace(item.AlgoClOrdID),
			Symbol:         market.Normalize(symbol),
			Side:           strings.ToUpper(strings.TrimSpace(item.Side)),
			PositionSide:   positionSide,
			OrderType:      strings.TrimSpace(item.OrdType),
			TriggerSubtype: subtype,
			TriggerSource:  source,
			TriggerPrice:   triggerPrice,
			OrderPrice:     orderPrice,
			Quantity:       quantity,
			Status:         normalizeOKXAlgoOrderStatus(item.State),
			ReduceOnly:     parseOKXBoolField(item.ReduceOnly),
			CreatedTime:    createdTime,
			UpdatedTime:    updatedTime,
			OrderAction:    orderAction,
		})
	}

	return result, nil
}

func (t *OKXTrader) GetAlgoOrderHistory(startTime time.Time, limit int) ([]OKXAlgoOrder, error) {
	states := []string{"filled", "canceled"}
	seen := make(map[string]struct{})
	result := make([]OKXAlgoOrder, 0)

	for _, state := range states {
		orders, err := t.getAlgoOrderHistoryByState(startTime, limit, state)
		if err != nil {
			return nil, err
		}
		for _, order := range orders {
			key := strings.TrimSpace(order.OrderID)
			if key == "" {
				key = strings.TrimSpace(order.AlgoID)
			}
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, order)
		}
	}

	return result, nil
}

func (t *OKXTrader) syncTriggerOrdersFromOKX(traderID string, exchangeID string, exchangeType string, st *store.Store, startTime time.Time) (int, error) {
	triggerOrders, err := t.GetAlgoOrderHistory(startTime, 100)
	if err != nil {
		return 0, fmt.Errorf("failed to get OKX algo order history: %w", err)
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
			ClientOrderID:   triggerOrder.AlgoClOrdID,
			Symbol:          triggerOrder.Symbol,
			Side:            triggerOrder.Side,
			PositionSide:    triggerOrder.PositionSide,
			Type:            normalizeOKXAlgoTriggerOrderType(triggerOrder.TriggerSubtype),
			VenueOrderType:  triggerOrder.OrderType,
			TriggerSubtype:  triggerOrder.TriggerSubtype,
			TriggerSource:   triggerOrder.TriggerSource,
			Quantity:        triggerOrder.Quantity,
			Price:           triggerOrder.OrderPrice,
			StopPrice:       triggerOrder.TriggerPrice,
			Status:          triggerOrder.Status,
			ReduceOnly:      triggerOrder.ReduceOnly,
			ClosePosition:   true,
			OrderAction:     triggerOrder.OrderAction,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		}
		if triggerOrder.Status == "FILLED" {
			orderRecord.FilledQuantity = triggerOrder.Quantity
			orderRecord.FilledAt = updatedAt
		}

		if err := st.Order().UpsertOrder(orderRecord); err != nil {
			return syncedCount, fmt.Errorf("failed to upsert OKX algo order %s: %w", triggerOrder.OrderID, err)
		}
		syncedCount++

		if triggerOrder.Status == "FILLED" {
			positions, err := st.Position().GetByExitOrderID(exchangeID, triggerOrder.OrderID)
			if err != nil {
				return syncedCount, fmt.Errorf("failed to load positions for OKX algo order %s: %w", triggerOrder.OrderID, err)
			}
			for _, pos := range positions {
				if err := st.DealReview().SyncPosition(pos); err != nil {
					logger.Infof("  ⚠️ Failed to resync deal review for OKX algo order %s / position %d: %v", triggerOrder.OrderID, pos.ID, err)
				}
			}
		}
	}

	return syncedCount, nil
}

// SyncOrdersFromOKX syncs OKX exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("okx")
func (t *OKXTrader) SyncOrdersFromOKX(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing OKX trades from: %s", startTime.Format(time.RFC3339))

	triggerSyncedCount, err := t.syncTriggerOrdersFromOKX(traderID, exchangeID, exchangeType, st, startTime)
	if err != nil {
		logger.Infof("⚠️  OKX trigger-order sync failed: %v", err)
	} else if triggerSyncedCount > 0 {
		logger.Infof("📥 Synced %d OKX trigger orders", triggerSyncedCount)
	}

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 100)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from OKX", len(trades))

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
		// Check if trade already exists (use exchangeID which is UUID, not exchange type)
		existing, err := orderStore.GetOrderByExchangeID(exchangeID, trade.TradeID)
		if err == nil && existing != nil {
			continue // Order already exists, skip
		}

		// Normalize symbol
		symbol := market.Normalize(trade.Symbol)

		// Determine position side from order action
		positionSide := "LONG"
		if strings.Contains(trade.OrderAction, "short") {
			positionSide = "SHORT"
		}

		// Normalize side for storage
		side := strings.ToUpper(trade.Side)

		// Create order record - use UTC time in milliseconds to avoid timezone issues
		execTimeMs := trade.ExecTime.UTC().UnixMilli()
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			ExchangeOrderID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            trade.OrderType,
			OrderAction:     trade.OrderAction,
			Quantity:        trade.FillQtyBase,
			Price:           trade.FillPrice,
			Status:          "FILLED",
			FilledQuantity:  trade.FillQtyBase,
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
			Quantity:        trade.FillQtyBase,
			QuoteQuantity:   trade.FillPrice * trade.FillQtyBase,
			Commission:      trade.Fee,
			CommissionAsset: trade.FeeAsset,
			RealizedPnL:     0, // OKX fills don't include PnL per trade
			IsMaker:         trade.IsMaker,
			CreatedAt:       execTimeMs,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, trade.OrderAction,
			trade.FillQtyBase, trade.FillPrice, trade.Fee, 0, // No per-trade PnL from OKX
			execTimeMs, trade.TradeID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
		} else {
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.TradeID, trade.OrderAction, trade.FillQtyBase)
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f fee=%.6f action=%s",
			trade.TradeID, trade.Symbol, side, trade.FillQtyBase, trade.FillPrice, trade.Fee, trade.OrderAction)
	}

	logger.Infof("✅ OKX order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task for OKX
func (t *OKXTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromOKX(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  OKX order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 OKX order sync started (interval: %v)", interval)
}
