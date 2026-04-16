package store

import (
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// HistorySummary comprehensive trading history for AI context
type HistorySummary struct {
	TotalTrades    int     `json:"total_trades"`
	WinRate        float64 `json:"win_rate"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgTradeReturn float64 `json:"avg_trade_return"`

	BestSymbols  []SymbolStats `json:"best_symbols"`
	WorstSymbols []SymbolStats `json:"worst_symbols"`

	LongWinRate  float64 `json:"long_win_rate"`
	ShortWinRate float64 `json:"short_win_rate"`
	LongPnL      float64 `json:"long_pnl"`
	ShortPnL     float64 `json:"short_pnl"`

	AvgHoldingMins float64 `json:"avg_holding_mins"`
	BestHoldRange  string  `json:"best_hold_range"`

	RecentWinRate float64 `json:"recent_win_rate"`
	RecentPnL     float64 `json:"recent_pnl"`

	CurrentStreak int `json:"current_streak"`
	MaxWinStreak  int `json:"max_win_streak"`
	MaxLoseStreak int `json:"max_lose_streak"`
}

// GetHistorySummary generates comprehensive AI context summary
func (s *PositionStore) GetHistorySummary(traderID string) (*HistorySummary, error) {
	summary := &HistorySummary{}

	fullStats, err := s.GetFullStats(traderID)
	if err != nil {
		return nil, err
	}
	summary.TotalTrades = fullStats.TotalTrades
	summary.WinRate = fullStats.WinRate
	summary.TotalPnL = fullStats.TotalPnL
	if fullStats.TotalTrades > 0 {
		summary.AvgTradeReturn = fullStats.TotalPnL / float64(fullStats.TotalTrades)
	}

	symbolStats, _ := s.GetSymbolStats(traderID, 20)
	if len(symbolStats) > 0 {
		for i := 0; i < len(symbolStats) && i < 3; i++ {
			if symbolStats[i].TotalPnL > 0 {
				summary.BestSymbols = append(summary.BestSymbols, symbolStats[i])
			}
		}
		for i := len(symbolStats) - 1; i >= 0 && len(summary.WorstSymbols) < 3; i-- {
			if symbolStats[i].TotalPnL < 0 {
				summary.WorstSymbols = append(summary.WorstSymbols, symbolStats[i])
			}
		}
	}

	dirStats, _ := s.GetDirectionStats(traderID)
	for _, d := range dirStats {
		if d.Side == "LONG" {
			summary.LongWinRate = d.WinRate
			summary.LongPnL = d.TotalPnL
		} else if d.Side == "SHORT" {
			summary.ShortWinRate = d.WinRate
			summary.ShortPnL = d.TotalPnL
		}
	}

	holdStats, _ := s.GetHoldingTimeStats(traderID)
	var bestHoldWinRate float64
	for _, h := range holdStats {
		if h.WinRate > bestHoldWinRate && h.TradeCount >= 3 {
			bestHoldWinRate = h.WinRate
			summary.BestHoldRange = h.Range
		}
	}

	// Calculate average holding time
	var positions []TraderPosition
	s.db.Where("trader_id = ? AND status = ? AND exit_time > 0", traderID, "CLOSED").Find(&positions)
	if len(positions) > 0 {
		var totalMins float64
		for _, pos := range positions {
			if pos.ExitTime > 0 {
				totalMins += float64(pos.ExitTime-pos.EntryTime) / 60000.0 // ms to minutes
			}
		}
		summary.AvgHoldingMins = totalMins / float64(len(positions))
	}

	// Recent 20 trades
	var recent []TraderPosition
	s.db.Where("trader_id = ? AND status = ?", traderID, "CLOSED").
		Order("exit_time DESC").Limit(20).Find(&recent)
	for _, pos := range recent {
		summary.RecentPnL += pos.RealizedPnL
		if pos.RealizedPnL > 0 {
			summary.RecentWinRate++
		}
	}
	if len(recent) > 0 {
		summary.RecentWinRate = summary.RecentWinRate / float64(len(recent)) * 100
	}

	// Calculate streaks
	s.calculateStreaks(traderID, summary)

	return summary, nil
}

// calculateStreaks calculates win/loss streaks
func (s *PositionStore) calculateStreaks(traderID string, summary *HistorySummary) {
	var positions []TraderPosition
	err := s.db.Where("trader_id = ? AND status = ?", traderID, "CLOSED").
		Order("exit_time DESC").
		Find(&positions).Error
	if err != nil || len(positions) == 0 {
		return
	}

	var currentStreak, maxWin, maxLose int
	var prevWin *bool
	isFirst := true

	for _, pos := range positions {
		isWin := pos.RealizedPnL > 0

		if isFirst {
			if isWin {
				currentStreak = 1
			} else {
				currentStreak = -1
			}
			isFirst = false
		}

		if prevWin == nil {
			prevWin = &isWin
		} else if *prevWin == isWin {
			if isWin {
				currentStreak++
				if currentStreak > maxWin {
					maxWin = currentStreak
				}
			} else {
				currentStreak--
				if -currentStreak > maxLose {
					maxLose = -currentStreak
				}
			}
		} else {
			if isWin {
				currentStreak = 1
			} else {
				currentStreak = -1
			}
			*prevWin = isWin
		}
	}

	summary.CurrentStreak = currentStreak
	summary.MaxWinStreak = maxWin
	summary.MaxLoseStreak = maxLose
}

// ClosedPnLRecord represents a closed position record from exchange
// All time fields use int64 millisecond timestamps (UTC)
type ClosedPnLRecord struct {
	Symbol      string
	Side        string
	EntryPrice  float64
	ExitPrice   float64
	Quantity    float64
	RealizedPnL float64
	Fee         float64
	Leverage    int
	EntryTime   int64 // Unix milliseconds UTC
	ExitTime    int64 // Unix milliseconds UTC
	OrderID     string
	CloseType   string
	ExchangeID  string
}

type closedPnLMatchKind int

const (
	closedPnLMatchNone closedPnLMatchKind = iota
	closedPnLMatchExact
	closedPnLMatchContained
)

// CreateFromClosedPnL creates a closed position record from exchange data
func (s *PositionStore) CreateFromClosedPnL(traderID, exchangeID, exchangeType string, record *ClosedPnLRecord) (bool, error) {
	if record.Symbol == "" {
		return false, nil
	}

	side := strings.ToUpper(record.Side)
	if side == "LONG" || side == "BUY" {
		side = "LONG"
	} else if side == "SHORT" || side == "SELL" {
		side = "SHORT"
	} else {
		return false, nil
	}

	if record.Quantity <= 0 || record.ExitPrice <= 0 || record.EntryPrice <= 0 {
		return false, nil
	}

	exchangePositionID := record.ExchangeID
	if exchangePositionID == "" {
		exchangePositionID = fmt.Sprintf("%s_%s_%d_%.8f", record.Symbol, side, record.ExitTime, record.RealizedPnL)
	}

	exists, err := s.ExistsWithExchangePositionID(exchangeID, exchangePositionID)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if matched, matchKind, err := s.findMatchingClosedPnLPosition(traderID, exchangeID, side, record, 0); err != nil {
		return false, err
	} else if matched != nil {
		if matchKind == closedPnLMatchExact {
			if err := s.applyClosedPnLToPosition(matched.ID, record); err != nil {
				return false, err
			}
		}
		return false, nil
	}

	exitTimeMs := record.ExitTime
	entryTimeMs := record.EntryTime

	// Validate timestamps (must be after year 2000 = ~946684800000 ms)
	minValidTime := int64(946684800000) // 2000-01-01 UTC in milliseconds
	if exitTimeMs < minValidTime {
		return false, nil
	}
	if entryTimeMs < minValidTime {
		entryTimeMs = exitTimeMs
	}
	if entryTimeMs > exitTimeMs {
		entryTimeMs = exitTimeMs
	}

	nowMs := time.Now().UTC().UnixMilli()
	pos := &TraderPosition{
		TraderID:           traderID,
		ExchangeID:         exchangeID,
		ExchangeType:       exchangeType,
		ExchangePositionID: exchangePositionID,
		Symbol:             record.Symbol,
		Side:               side,
		Quantity:           record.Quantity,
		EntryQuantity:      record.Quantity,
		EntryPrice:         record.EntryPrice,
		EntryTime:          entryTimeMs,
		ExitPrice:          record.ExitPrice,
		ExitOrderID:        record.OrderID,
		ExitTime:           exitTimeMs,
		RealizedPnL:        record.RealizedPnL,
		Fee:                record.Fee,
		Leverage:           record.Leverage,
		Status:             "CLOSED",
		CloseReason:        record.CloseType,
		Source:             "closed_pnl_sync",
		CreatedAt:          nowMs,
		UpdatedAt:          nowMs,
	}

	err = s.db.Create(pos).Error
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return false, nil
		}
		return false, fmt.Errorf("failed to create position from closed PnL: %w", err)
	}
	s.syncDealReviewClosedByID(pos.ID)

	return true, nil
}

func (s *PositionStore) findMatchingClosedPnLPosition(traderID, exchangeID, side string, record *ClosedPnLRecord, excludePositionID int64) (*TraderPosition, closedPnLMatchKind, error) {
	if record == nil {
		return nil, closedPnLMatchNone, nil
	}

	exitTimeMs := record.ExitTime
	if exitTimeMs == 0 {
		return nil, closedPnLMatchNone, nil
	}
	entryTimeMs := record.EntryTime
	if entryTimeMs == 0 || entryTimeMs > exitTimeMs {
		entryTimeMs = exitTimeMs
	}

	const (
		timeToleranceMs   = int64(2 * 60 * 1000)
		priceTolerance    = 0.000001
		entryTolerance    = 0.000005
		quantityTolerance = 0.0001
	)

	var candidates []TraderPosition
	if err := s.db.Where(
		"trader_id = ? AND exchange_id = ? AND status = ? AND symbol = ? AND side = ? AND id != ? AND entry_time <= ? AND exit_time >= ?",
		traderID,
		exchangeID,
		"CLOSED",
		record.Symbol,
		side,
		excludePositionID,
		exitTimeMs+timeToleranceMs,
		entryTimeMs-timeToleranceMs,
	).Find(&candidates).Error; err != nil {
		return nil, closedPnLMatchNone, fmt.Errorf("failed to query matching closed positions: %w", err)
	}

	for i := range candidates {
		candidate := &candidates[i]
		qty := candidate.EntryQuantity
		if qty == 0 {
			qty = candidate.Quantity
		}
		if math.Abs(qty-record.Quantity) > quantityTolerance {
			continue
		}
		if record.EntryPrice > 0 && candidate.EntryPrice > 0 && math.Abs(candidate.EntryPrice-record.EntryPrice) > priceTolerance*math.Max(1, record.EntryPrice) {
			continue
		}
		if absInt64(candidate.ExitTime-exitTimeMs) > timeToleranceMs {
			continue
		}
		if record.ExitPrice > 0 && math.Abs(candidate.ExitPrice-record.ExitPrice) > priceTolerance*math.Max(1, record.ExitPrice) {
			continue
		}
		return candidate, closedPnLMatchExact, nil
	}

	for i := range candidates {
		candidate := &candidates[i]
		if strings.EqualFold(candidate.Source, "closed_pnl_sync") {
			continue
		}
		qty := candidate.EntryQuantity
		if qty == 0 {
			qty = candidate.Quantity
		}
		if qty+quantityTolerance < record.Quantity {
			continue
		}
		if record.EntryPrice > 0 && candidate.EntryPrice > 0 && math.Abs(candidate.EntryPrice-record.EntryPrice) > entryTolerance*math.Max(1, record.EntryPrice) {
			continue
		}

		candidateEntryTimeMs := candidate.EntryTime
		if candidateEntryTimeMs == 0 || candidateEntryTimeMs > candidate.ExitTime {
			candidateEntryTimeMs = candidate.ExitTime
		}
		if candidateEntryTimeMs == 0 || candidate.ExitTime == 0 {
			continue
		}
		if entryTimeMs < candidateEntryTimeMs-timeToleranceMs {
			continue
		}
		if exitTimeMs > candidate.ExitTime+timeToleranceMs {
			continue
		}

		return candidate, closedPnLMatchContained, nil
	}

	return nil, closedPnLMatchNone, nil
}

func (s *PositionStore) applyClosedPnLToPosition(positionID int64, record *ClosedPnLRecord) error {
	if positionID == 0 || record == nil {
		return nil
	}

	position, err := s.GetByID(positionID)
	if err != nil {
		return fmt.Errorf("failed to load matching position %d: %w", positionID, err)
	}

	updates := map[string]any{
		"realized_pnl": record.RealizedPnL,
		"fee":          record.Fee,
		"updated_at":   time.Now().UTC().UnixMilli(),
	}

	if record.ExitPrice > 0 {
		updates["exit_price"] = record.ExitPrice
	}
	if record.ExitTime > 0 {
		updates["exit_time"] = record.ExitTime
	}
	if record.Leverage > 0 && position.Leverage != record.Leverage {
		updates["leverage"] = record.Leverage
	}
	if orderID := strings.TrimSpace(record.OrderID); orderID != "" && position.ExitOrderID != orderID {
		updates["exit_order_id"] = orderID
	}
	if record.EntryTime > 0 && record.EntryTime < record.ExitTime &&
		(position.EntryTime == 0 || position.EntryTime == position.ExitTime || position.EntryTime > record.EntryTime) {
		updates["entry_time"] = record.EntryTime
	}
	if record.EntryPrice > 0 && position.EntryPrice <= 0 {
		updates["entry_price"] = record.EntryPrice
	}
	if shouldReplaceCloseReason(position.CloseReason, record.CloseType) {
		updates["close_reason"] = record.CloseType
	}

	if err := s.db.Model(&TraderPosition{}).Where("id = ?", positionID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update matching position %d with closed PnL data: %w", positionID, err)
	}
	s.syncDealReviewClosedByID(positionID)
	return nil
}

// CleanupRedundantClosedPnLPositions removes stale closed_pnl_sync rows that are already represented
// by reconstructed sync positions. Exact matches refresh the canonical position before deleting the duplicate.
func (s *PositionStore) CleanupRedundantClosedPnLPositions() (int, int, error) {
	var closedPnLPositions []TraderPosition
	if err := s.db.
		Where("status = ? AND source = ?", "CLOSED", "closed_pnl_sync").
		Order("id ASC").
		Find(&closedPnLPositions).Error; err != nil {
		return 0, 0, fmt.Errorf("failed to load closed PnL positions for cleanup: %w", err)
	}

	removed := 0
	refreshed := 0
	for i := range closedPnLPositions {
		position := &closedPnLPositions[i]
		record := &ClosedPnLRecord{
			Symbol:      position.Symbol,
			Side:        position.Side,
			EntryPrice:  position.EntryPrice,
			ExitPrice:   position.ExitPrice,
			Quantity:    position.EntryQuantity,
			RealizedPnL: position.RealizedPnL,
			Fee:         position.Fee,
			Leverage:    position.Leverage,
			EntryTime:   position.EntryTime,
			ExitTime:    position.ExitTime,
			OrderID:     position.ExitOrderID,
			CloseType:   position.CloseReason,
			ExchangeID:  position.ExchangePositionID,
		}
		if record.Quantity == 0 {
			record.Quantity = position.Quantity
		}

		matched, matchKind, err := s.findMatchingClosedPnLPosition(position.TraderID, position.ExchangeID, position.Side, record, position.ID)
		if err != nil {
			return removed, refreshed, err
		}
		if matched == nil {
			continue
		}
		if matchKind == closedPnLMatchExact {
			if err := s.applyClosedPnLToPosition(matched.ID, record); err != nil {
				return removed, refreshed, err
			}
			refreshed++
		}
		if err := s.deletePositionWithReviewArtifacts(position.ID); err != nil {
			return removed, refreshed, err
		}
		removed++
	}

	return removed, refreshed, nil
}

func (s *PositionStore) deletePositionWithReviewArtifacts(positionID int64) error {
	if positionID == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var caseRec DealReviewCase
		err := tx.Select("id").Where("position_id = ?", positionID).First(&caseRec).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to load deal review case for redundant position %d: %w", positionID, err)
		}

		if err := tx.Where("position_id = ?", positionID).Delete(&DealReviewCyclePointRecord{}).Error; err != nil {
			return fmt.Errorf("failed to delete deal review cycle points for redundant position %d: %w", positionID, err)
		}
		if err := tx.Where("position_id = ?", positionID).Delete(&DealReviewMarketPointRecord{}).Error; err != nil {
			return fmt.Errorf("failed to delete deal review market points for redundant position %d: %w", positionID, err)
		}
		if err := tx.Where("position_id = ?", positionID).Delete(&DealReviewEvent{}).Error; err != nil {
			return fmt.Errorf("failed to delete deal review events for redundant position %d: %w", positionID, err)
		}
		if caseRec.ID != "" {
			if err := tx.Where("deal_id = ?", caseRec.ID).Delete(&DealReviewEvent{}).Error; err != nil {
				return fmt.Errorf("failed to delete deal review events for redundant case %s: %w", caseRec.ID, err)
			}
			if err := tx.Where("case_id = ?", caseRec.ID).Delete(&DealReviewClassifierFeedback{}).Error; err != nil {
				return fmt.Errorf("failed to delete classifier feedback for redundant case %s: %w", caseRec.ID, err)
			}
		}
		if err := tx.Where("position_id = ?", positionID).Delete(&DealReviewCase{}).Error; err != nil {
			return fmt.Errorf("failed to delete deal review case for redundant position %d: %w", positionID, err)
		}
		if err := tx.Delete(&TraderPosition{}, positionID).Error; err != nil {
			return fmt.Errorf("failed to delete redundant position %d: %w", positionID, err)
		}
		return nil
	})
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

// GetLastClosedPositionTime gets the most recent exit time (Unix ms)
func (s *PositionStore) GetLastClosedPositionTime(traderID string) (int64, error) {
	var pos TraderPosition
	err := s.db.Where("trader_id = ? AND status = ? AND exit_time > 0", traderID, "CLOSED").
		Order("exit_time DESC").
		First(&pos).Error

	if err == gorm.ErrRecordNotFound || pos.ExitTime == 0 {
		return time.Now().UTC().Add(-30 * 24 * time.Hour).UnixMilli(), nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get last closed position time: %w", err)
	}

	return pos.ExitTime, nil
}

// SyncClosedPositions syncs closed positions from exchange
func (s *PositionStore) SyncClosedPositions(traderID, exchangeID, exchangeType string, records []ClosedPnLRecord) (int, int, error) {
	created, skipped := 0, 0
	for _, record := range records {
		rec := record
		wasCreated, err := s.CreateFromClosedPnL(traderID, exchangeID, exchangeType, &rec)
		if err != nil {
			return created, skipped, fmt.Errorf("failed to sync position: %w", err)
		}
		if wasCreated {
			created++
		} else {
			skipped++
		}
	}
	return created, skipped, nil
}
