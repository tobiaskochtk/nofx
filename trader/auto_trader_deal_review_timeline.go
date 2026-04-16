package trader

import (
	"math"
	"nofx/logger"
	"nofx/store"
	"strconv"
	"strings"
	"time"
)

const dealReviewPriceMonitorInterval = 30 * time.Second

func (at *AutoTrader) startDealReviewPriceMonitor() {
	if at.store == nil || strings.TrimSpace(at.userID) == "" {
		return
	}

	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(dealReviewPriceMonitorInterval)
		defer ticker.Stop()

		logger.Infof("📈 Started deal-review price monitor (capture every %v)", dealReviewPriceMonitorInterval)
		at.captureDealReviewPricePoints()

		for {
			select {
			case <-ticker.C:
				at.captureDealReviewPricePoints()
			case <-at.stopMonitorCh:
				logger.Info("⏹ Stopped deal-review price monitor")
				return
			}
		}
	}()
}

func (at *AutoTrader) captureDealReviewPricePoints() {
	if at.store == nil {
		return
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Infof("⚠️ Deal-review price monitor: failed to get positions: %v", err)
		return
	}

	snapshots := buildDealReviewPositionSnapshots(positions)
	if len(snapshots) == 0 {
		return
	}

	if err := at.store.DealReview().CaptureLivePositionPricePoints(at.userID, at.id, snapshots, time.Now().UTC(), "platform"); err != nil {
		logger.Infof("⚠️ Deal-review price monitor: failed to capture position price points: %v", err)
	}
}

func buildDealReviewPositionSnapshots(positions []map[string]interface{}) []store.PositionSnapshot {
	snapshots := make([]store.PositionSnapshot, 0, len(positions))
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}

		side, _ := pos["side"].(string)
		side = strings.TrimSpace(side)

		positionAmt := math.Abs(dealReviewMapFloat(pos, "positionAmt"))
		if positionAmt == 0 {
			continue
		}

		if side == "" {
			if rawAmt := dealReviewMapFloat(pos, "positionAmt"); rawAmt < 0 {
				side = "short"
			} else {
				side = "long"
			}
		}

		snapshots = append(snapshots, store.PositionSnapshot{
			Symbol:           symbol,
			Side:             side,
			PositionAmt:      positionAmt,
			EntryPrice:       dealReviewMapFloat(pos, "entryPrice"),
			MarkPrice:        dealReviewMapFloat(pos, "markPrice"),
			UnrealizedProfit: dealReviewMapFloat(pos, "unRealizedProfit", "unrealizedProfit"),
			Leverage:         dealReviewMapFloat(pos, "leverage"),
			LiquidationPrice: dealReviewMapFloat(pos, "liquidationPrice"),
		})
	}
	return snapshots
}

func dealReviewMapFloat(values map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		switch value := values[key].(type) {
		case float64:
			return value
		case float32:
			return float64(value)
		case int:
			return float64(value)
		case int64:
			return float64(value)
		case int32:
			return float64(value)
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}
