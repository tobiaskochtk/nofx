package trader

import (
	"fmt"
	"strings"
	"time"

	"nofx/logger"
	"nofx/store"
)

func extractOrderIDFromResult(orderResult map[string]interface{}) string {
	if len(orderResult) == 0 {
		return ""
	}
	switch v := orderResult["orderId"].(type) {
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case string:
		return strings.TrimSpace(v)
	default:
		if v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func normalizeExitIntentSide(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "LONG", "BUY":
		return "LONG"
	case "SHORT", "SELL":
		return "SHORT"
	default:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "long":
			return "LONG"
		case "short":
			return "SHORT"
		default:
			return ""
		}
	}
}

func closeActionForIntentSide(side string) string {
	switch normalizeExitIntentSide(side) {
	case "LONG":
		return "close_long"
	case "SHORT":
		return "close_short"
	default:
		return ""
	}
}

func (at *AutoTrader) findOpenPositionIntentContext(symbol, sideHint string) (normalizedSide string, quantity, entryPrice, markPrice float64) {
	if at == nil || at.trader == nil {
		return normalizeExitIntentSide(sideHint), 0, 0, 0
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		return normalizeExitIntentSide(sideHint), 0, 0, 0
	}

	wantSymbol := strings.ToUpper(strings.TrimSpace(symbol))
	wantSide := normalizeExitIntentSide(sideHint)
	for _, pos := range positions {
		posSymbol, _ := pos["symbol"].(string)
		if strings.ToUpper(strings.TrimSpace(posSymbol)) != wantSymbol {
			continue
		}

		size, _ := pos["positionAmt"].(float64)
		side := normalizeExitIntentSide(sideHint)
		if posSide, _ := pos["side"].(string); normalizeExitIntentSide(posSide) != "" {
			side = normalizeExitIntentSide(posSide)
		} else if size < 0 {
			side = "SHORT"
		} else if size > 0 {
			side = "LONG"
		}
		if wantSide != "" && side != "" && side != wantSide {
			continue
		}

		entryPrice, _ = pos["entryPrice"].(float64)
		markPrice, _ = pos["markPrice"].(float64)
		if size < 0 {
			size = -size
		}
		return side, size, entryPrice, markPrice
	}

	return normalizeExitIntentSide(sideHint), 0, 0, 0
}

func (at *AutoTrader) recordExitIntent(input *store.DealReviewExitIntentInput) {
	if at == nil || at.store == nil || input == nil {
		return
	}
	if err := at.store.DealReview().RecordExitIntent(input); err != nil {
		logger.Warnf("⚠️ Failed to persist exit intent for %s %s: %v", input.Symbol, input.Side, err)
	}
}

func (at *AutoTrader) recordAICloseExitIntent(orderID string, action string, quantity, price, entryPrice float64, reviewInput *store.DealReviewDecisionEventInput) {
	if at == nil || at.store == nil || reviewInput == nil || reviewInput.Stage != store.DealReviewStageClose {
		return
	}
	if strings.TrimSpace(orderID) == "" {
		return
	}

	summary := "Matched AI close order submission."
	if reviewInput.CycleNumber > 0 {
		summary = fmt.Sprintf("Matched AI close order submission from decision cycle %d.", reviewInput.CycleNumber)
	}

	at.recordExitIntent(&store.DealReviewExitIntentInput{
		UserID:              at.userID,
		TraderID:            at.id,
		ExchangeID:          at.exchangeID,
		ExchangeOrderID:     orderID,
		Symbol:              reviewInput.Symbol,
		Side:                reviewInput.Side,
		Action:              action,
		IntentType:          store.DealReviewExitIntentTypeAICloseDecision,
		SourceModule:        "trader.auto_trader_orders",
		Summary:             summary,
		Reasoning:           reviewInput.Reasoning,
		DecisionCycleNumber: reviewInput.CycleNumber,
		Confidence:          reviewInput.Confidence,
		Quantity:            quantity,
		Price:               price,
		EntryPrice:          entryPrice,
		Timestamp:           time.Now().UTC(),
	})
}

func (at *AutoTrader) recordSystemExitIntent(orderResult map[string]interface{}, symbol, side, intentType, sourceModule, summary, reasoning string, quantity, price, entryPrice float64) {
	if at == nil || at.store == nil {
		return
	}
	orderID := extractOrderIDFromResult(orderResult)
	if orderID == "" {
		return
	}

	at.recordExitIntent(&store.DealReviewExitIntentInput{
		UserID:          at.userID,
		TraderID:        at.id,
		ExchangeID:      at.exchangeID,
		ExchangeOrderID: orderID,
		Symbol:          symbol,
		Side:            normalizeExitIntentSide(side),
		Action:          closeActionForIntentSide(side),
		IntentType:      intentType,
		SourceModule:    sourceModule,
		Summary:         strings.TrimSpace(summary),
		Reasoning:       strings.TrimSpace(reasoning),
		Quantity:        quantity,
		Price:           price,
		EntryPrice:      entryPrice,
		Timestamp:       time.Now().UTC(),
	})
}
