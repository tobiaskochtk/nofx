package bybit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BybitTrade represents a trade record from Bybit execution list
type BybitTrade struct {
	Symbol      string
	OrderID     string
	ExecID      string
	Side        string // Buy or Sell
	ExecPrice   float64
	ExecQty     float64
	ExecFee     float64
	ExecTime    time.Time
	IsMaker     bool
	OrderType   string
	ClosedSize  float64 // For close orders
	ClosedPnL   float64
	OrderAction string // open_long, open_short, close_long, close_short
}

type BybitTriggerOrder struct {
	Symbol         string
	OrderID        string
	Side           string
	OrderType      string
	StopOrderType  string
	TriggerPrice   float64
	OrderPrice     float64
	Quantity       float64
	FilledQuantity float64
	AvgFillPrice   float64
	ExecFee        float64
	Status         string
	ReduceOnly     bool
	CloseOnTrigger bool
	CreatedTime    time.Time
	UpdatedTime    time.Time
	OrderAction    string
}

// GetTrades retrieves trade/execution records from Bybit
func (t *BybitTrader) GetTrades(startTime time.Time, limit int) ([]BybitTrade, error) {
	return t.getTradesViaHTTP(startTime, limit)
}

func (t *BybitTrader) GetTriggerOrderHistory(startTime time.Time, limit int) ([]BybitTriggerOrder, error) {
	return t.getTriggerOrderHistoryViaHTTP(startTime, limit)
}

// getTradesViaHTTP makes direct HTTP call to Bybit API for execution list
func (t *BybitTrader) getTradesViaHTTP(startTime time.Time, limit int) ([]BybitTrade, error) {
	// Build query string
	queryParams := fmt.Sprintf("category=linear&startTime=%d&limit=%d", startTime.UnixMilli(), limit)
	url := "https://api.bybit.com/v5/execution/list?" + queryParams

	// Generate timestamp
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	recvWindow := "10000"

	// Build signature payload: timestamp + api_key + recv_window + queryString
	signPayload := timestamp + t.apiKey + recvWindow + queryParams

	// Generate HMAC-SHA256 signature
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(signPayload))
	signature := hex.EncodeToString(h.Sum(nil))

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add Bybit V5 API headers
	req.Header.Set("X-BAPI-API-KEY", t.apiKey)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-SIGN-TYPE", "2")
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	req.Header.Set("Content-Type", "application/json")

	// Reuse trader HTTP client so transport-level timestamp correction is applied.
	httpClient := http.DefaultClient
	if t.client != nil && t.client.HTTPClient != nil {
		httpClient = t.client.HTTPClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Bybit API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API error: %s", result.RetMsg)
	}

	return t.parseTradesResult(result.Result.List)
}

// parseTradesResult parses the execution list result from Bybit API
func (t *BybitTrader) parseTradesResult(list []map[string]interface{}) ([]BybitTrade, error) {
	var trades []BybitTrade

	for _, item := range list {
		symbol, _ := item["symbol"].(string)
		orderID, _ := item["orderId"].(string)
		execID, _ := item["execId"].(string)
		side, _ := item["side"].(string)
		orderType, _ := item["orderType"].(string)
		isMaker, _ := item["isMaker"].(bool)

		execPriceStr, _ := item["execPrice"].(string)
		execQtyStr, _ := item["execQty"].(string)
		execFeeStr, _ := item["execFee"].(string)
		closedSizeStr, _ := item["closedSize"].(string)
		closedPnlStr, _ := item["closedPnl"].(string)
		execTimeStr, _ := item["execTime"].(string)

		execPrice, _ := strconv.ParseFloat(execPriceStr, 64)
		execQty, _ := strconv.ParseFloat(execQtyStr, 64)
		execFee, _ := strconv.ParseFloat(execFeeStr, 64)
		closedSize, _ := strconv.ParseFloat(closedSizeStr, 64)
		closedPnl, _ := strconv.ParseFloat(closedPnlStr, 64)
		execTimeMs, _ := strconv.ParseInt(execTimeStr, 10, 64)
		execTime := time.UnixMilli(execTimeMs).UTC()

		// Determine order action based on side and closedSize
		// If closedSize > 0, it's a close trade
		// Side: Buy = long direction, Sell = short direction
		orderAction := "open_long"
		if closedSize > 0 {
			// This is a close trade
			if strings.ToLower(side) == "sell" {
				orderAction = "close_long" // Selling to close a long
			} else {
				orderAction = "close_short" // Buying to close a short
			}
		} else {
			// This is an open trade
			if strings.ToLower(side) == "buy" {
				orderAction = "open_long"
			} else {
				orderAction = "open_short"
			}
		}

		trade := BybitTrade{
			Symbol:      symbol,
			OrderID:     orderID,
			ExecID:      execID,
			Side:        side,
			ExecPrice:   execPrice,
			ExecQty:     execQty,
			ExecFee:     execFee,
			ExecTime:    execTime,
			IsMaker:     isMaker,
			OrderType:   orderType,
			ClosedSize:  closedSize,
			ClosedPnL:   closedPnl,
			OrderAction: orderAction,
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

func (t *BybitTrader) getTriggerOrderHistoryViaHTTP(startTime time.Time, limit int) ([]BybitTriggerOrder, error) {
	queryParams := fmt.Sprintf("category=linear&orderFilter=StopOrder&startTime=%d&limit=%d", startTime.UnixMilli(), limit)
	url := "https://api.bybit.com/v5/order/history?" + queryParams

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	recvWindow := "10000"
	signPayload := timestamp + t.apiKey + recvWindow + queryParams

	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(signPayload))
	signature := hex.EncodeToString(h.Sum(nil))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-BAPI-API-KEY", t.apiKey)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-SIGN-TYPE", "2")
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	req.Header.Set("Content-Type", "application/json")

	httpClient := http.DefaultClient
	if t.client != nil && t.client.HTTPClient != nil {
		httpClient = t.client.HTTPClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Bybit API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API error: %s", result.RetMsg)
	}

	return parseBybitTriggerOrdersResult(result.Result.List)
}

func parseBybitTriggerOrdersResult(list []map[string]interface{}) ([]BybitTriggerOrder, error) {
	orders := make([]BybitTriggerOrder, 0, len(list))

	for _, item := range list {
		symbol, _ := item["symbol"].(string)
		orderID, _ := item["orderId"].(string)
		side, _ := item["side"].(string)
		orderType, _ := item["orderType"].(string)
		stopOrderType, _ := item["stopOrderType"].(string)
		orderStatus, _ := item["orderStatus"].(string)
		orderLinkID, _ := item["orderLinkId"].(string)

		triggerPrice, _ := parseBybitFloatField(item["triggerPrice"])
		orderPrice, _ := parseBybitFloatField(item["price"])
		quantity, _ := parseBybitFloatField(item["qty"])
		filledQuantity, _ := parseBybitFloatField(item["cumExecQty"])
		avgFillPrice, _ := parseBybitFloatField(item["avgPrice"])
		execFee, _ := parseBybitFloatField(item["cumExecFee"])
		createdTime, _ := parseBybitTimeField(item["createdTime"])
		updatedTime, _ := parseBybitTimeField(item["updatedTime"])

		reduceOnly := parseBybitBoolField(item["reduceOnly"])
		closeOnTrigger := parseBybitBoolField(item["closeOnTrigger"])
		if !reduceOnly && orderLinkID != "" {
			reduceOnly = strings.Contains(strings.ToLower(orderLinkID), "reduce")
		}

		orderAction := inferBybitTriggerOrderAction(side, reduceOnly)
		if orderID == "" || symbol == "" || orderAction == "" {
			continue
		}

		orders = append(orders, BybitTriggerOrder{
			Symbol:         symbol,
			OrderID:        orderID,
			Side:           side,
			OrderType:      orderType,
			StopOrderType:  stopOrderType,
			TriggerPrice:   triggerPrice,
			OrderPrice:     orderPrice,
			Quantity:       quantity,
			FilledQuantity: filledQuantity,
			AvgFillPrice:   avgFillPrice,
			ExecFee:        execFee,
			Status:         normalizeBybitOrderStatus(orderStatus),
			ReduceOnly:     reduceOnly,
			CloseOnTrigger: closeOnTrigger,
			CreatedTime:    createdTime,
			UpdatedTime:    updatedTime,
			OrderAction:    orderAction,
		})
	}

	return orders, nil
}

func parseBybitFloatField(value any) (float64, error) {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		return strconv.ParseFloat(v, 64)
	case float64:
		return v, nil
	case int64:
		return float64(v), nil
	case int:
		return float64(v), nil
	default:
		return 0, nil
	}
}

func parseBybitTimeField(value any) (time.Time, error) {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return time.Time{}, nil
		}
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.UnixMilli(ms).UTC(), nil
	case float64:
		return time.UnixMilli(int64(v)).UTC(), nil
	case int64:
		return time.UnixMilli(v).UTC(), nil
	default:
		return time.Time{}, nil
	}
}

func parseBybitBoolField(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		normalized := strings.ToLower(strings.TrimSpace(v))
		return normalized == "true" || normalized == "1"
	case float64:
		return v != 0
	case int64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

func inferBybitTriggerOrderAction(side string, reduceOnly bool) string {
	if strings.EqualFold(strings.TrimSpace(side), "sell") {
		return "close_long"
	}
	if strings.EqualFold(strings.TrimSpace(side), "buy") {
		return "close_short"
	}
	return ""
}

func normalizeBybitOrderStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "filled":
		return "FILLED"
	case "new", "created", "untriggered", "triggered", "active":
		return "NEW"
	case "cancelled", "cancelledbyuser", "rejected", "deactivated":
		return "CANCELED"
	case "partiallyfilled":
		return "PARTIALLY_FILLED"
	default:
		return strings.ToUpper(strings.TrimSpace(status))
	}
}

func normalizeBybitTriggerOrderType(orderType, stopOrderType string) string {
	normalizedStopType := strings.TrimSpace(stopOrderType)
	if normalizedStopType != "" {
		return normalizedStopType
	}
	normalizedOrderType := strings.TrimSpace(orderType)
	if normalizedOrderType != "" {
		return normalizedOrderType
	}
	return "StopOrder"
}

func (t *BybitTrader) syncTriggerOrdersFromBybit(traderID string, exchangeID string, exchangeType string, st *store.Store, startTime time.Time) (int, error) {
	triggerOrders, err := t.GetTriggerOrderHistory(startTime, 500)
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
		symbol := market.Normalize(triggerOrder.Symbol)
		positionSide := "LONG"
		side := strings.ToUpper(strings.TrimSpace(triggerOrder.Side))
		if strings.Contains(triggerOrder.OrderAction, "short") {
			positionSide = "SHORT"
		}

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
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            normalizeBybitTriggerOrderType(triggerOrder.OrderType, triggerOrder.StopOrderType),
			Quantity:        triggerOrder.Quantity,
			Price:           triggerOrder.OrderPrice,
			StopPrice:       triggerOrder.TriggerPrice,
			Status:          triggerOrder.Status,
			FilledQuantity:  triggerOrder.FilledQuantity,
			AvgFillPrice:    triggerOrder.AvgFillPrice,
			Commission:      triggerOrder.ExecFee,
			CommissionAsset: "USDT",
			ReduceOnly:      true,
			ClosePosition:   triggerOrder.CloseOnTrigger,
			OrderAction:     triggerOrder.OrderAction,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		}
		if triggerOrder.Status == "FILLED" {
			orderRecord.FilledAt = updatedAt
		}

		if err := st.Order().UpsertOrder(orderRecord); err != nil {
			return syncedCount, fmt.Errorf("failed to upsert trigger order %s: %w", triggerOrder.OrderID, err)
		}
		syncedCount++

		if triggerOrder.Status == "FILLED" {
			positions, err := st.Position().GetByExitOrderID(exchangeID, triggerOrder.OrderID)
			if err != nil {
				return syncedCount, fmt.Errorf("failed to load positions for trigger order %s: %w", triggerOrder.OrderID, err)
			}
			for _, pos := range positions {
				if err := st.DealReview().SyncPosition(pos); err != nil {
					logger.Infof("  ⚠️ Failed to resync deal review for trigger order %s / position %d: %v", triggerOrder.OrderID, pos.ID, err)
				}
			}
		}
	}

	return syncedCount, nil
}

// SyncOrdersFromBybit syncs Bybit exchange order history to local database
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("bybit")
func (t *BybitTrader) SyncOrdersFromBybit(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	// Get recent trades (last 24 hours)
	startTime := time.Now().Add(-24 * time.Hour)

	logger.Infof("🔄 Syncing Bybit trades from: %s", startTime.Format(time.RFC3339))

	triggerSyncedCount, err := t.syncTriggerOrdersFromBybit(traderID, exchangeID, exchangeType, st, startTime)
	if err != nil {
		logger.Infof("⚠️  Bybit trigger-order sync failed: %v", err)
	} else if triggerSyncedCount > 0 {
		logger.Infof("📥 Synced %d Bybit trigger orders", triggerSyncedCount)
	}

	// Use GetTrades method to fetch trade records
	trades, err := t.GetTrades(startTime, 1000)
	if err != nil {
		return fmt.Errorf("failed to get trades: %w", err)
	}

	logger.Infof("📥 Received %d trades from Bybit", len(trades))

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
		existing, err := orderStore.GetOrderByExchangeID(exchangeID, trade.ExecID)
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
			ExchangeOrderID: trade.ExecID, // Use ExecID as unique identifier
			Symbol:          symbol,
			Side:            side,
			PositionSide:    "BOTH", // Bybit uses one-way position mode
			Type:            trade.OrderType,
			OrderAction:     trade.OrderAction,
			Quantity:        trade.ExecQty,
			Price:           trade.ExecPrice,
			Status:          "FILLED",
			FilledQuantity:  trade.ExecQty,
			AvgFillPrice:    trade.ExecPrice,
			Commission:      trade.ExecFee,
			FilledAt:        execTimeMs,
			CreatedAt:       execTimeMs,
			UpdatedAt:       execTimeMs,
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync trade %s: %v", trade.ExecID, err)
			continue
		}

		// Create fill record - use UTC time
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			OrderID:         orderRecord.ID,
			ExchangeOrderID: trade.OrderID,
			ExchangeTradeID: trade.ExecID,
			Symbol:          symbol,
			Side:            side,
			Price:           trade.ExecPrice,
			Quantity:        trade.ExecQty,
			QuoteQuantity:   trade.ExecPrice * trade.ExecQty,
			Commission:      trade.ExecFee,
			CommissionAsset: "USDT",
			RealizedPnL:     trade.ClosedPnL,
			IsMaker:         trade.IsMaker,
			CreatedAt:       execTimeMs,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.ExecID, err)
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, trade.OrderAction,
			trade.ExecQty, trade.ExecPrice, trade.ExecFee, trade.ClosedPnL,
			execTimeMs, trade.ExecID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.ExecID, err)
		} else {
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.ExecID, trade.OrderAction, trade.ExecQty)
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s",
			trade.ExecID, symbol, side, trade.ExecQty, trade.ExecPrice, trade.ClosedPnL, trade.ExecFee, trade.OrderAction)
	}

	logger.Infof("✅ Bybit order sync completed: %d new trades synced", syncedCount)
	return nil
}

// StartOrderSync starts background order sync task for Bybit
func (t *BybitTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	go func() {
		if err := t.SyncOrdersFromBybit(traderID, exchangeID, exchangeType, st); err != nil {
			logger.Infof("⚠️  Bybit order sync failed: %v", err)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := t.SyncOrdersFromBybit(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️  Bybit order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Bybit order sync started (interval: %v)", interval)
}
