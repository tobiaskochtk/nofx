package store

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaperTradePosition struct {
	ID            string  `gorm:"primaryKey" json:"id"`
	UserID        string  `gorm:"column:user_id;not null;index:idx_paper_trade_positions_user" json:"user_id"`
	TraderID      string  `gorm:"column:trader_id;not null;index:idx_paper_trade_positions_trader" json:"trader_id"`
	ExchangeID    string  `gorm:"column:exchange_id;not null;index:idx_paper_trade_positions_exchange" json:"exchange_id"`
	SessionID     string  `gorm:"column:session_id;not null;default:'';index:idx_paper_trade_positions_session" json:"session_id"`
	Symbol        string  `gorm:"column:symbol;not null;index:idx_paper_trade_positions_symbol" json:"symbol"`
	Side          string  `gorm:"column:side;not null;index:idx_paper_trade_positions_side" json:"side"`
	Quantity      float64 `gorm:"column:quantity;default:0" json:"quantity"`
	EntryPrice    float64 `gorm:"column:entry_price;default:0" json:"entry_price"`
	MarkPrice     float64 `gorm:"column:mark_price;default:0" json:"mark_price"`
	Leverage      int     `gorm:"column:leverage;default:1" json:"leverage"`
	InitialMargin float64 `gorm:"column:initial_margin;default:0" json:"initial_margin"`
	EntryOrderID  string  `gorm:"column:entry_order_id;default:''" json:"entry_order_id"`
	CreatedAt     int64   `gorm:"column:created_at;default:0" json:"created_at"`
	UpdatedAt     int64   `gorm:"column:updated_at;default:0" json:"updated_at"`
}

func (PaperTradePosition) TableName() string {
	return "paper_trade_positions"
}

type PaperTradeOrder struct {
	ID            string  `gorm:"primaryKey" json:"id"`
	UserID        string  `gorm:"column:user_id;not null;index:idx_paper_trade_orders_user" json:"user_id"`
	TraderID      string  `gorm:"column:trader_id;not null;index:idx_paper_trade_orders_trader" json:"trader_id"`
	ExchangeID    string  `gorm:"column:exchange_id;not null;index:idx_paper_trade_orders_exchange" json:"exchange_id"`
	SessionID     string  `gorm:"column:session_id;not null;default:'';index:idx_paper_trade_orders_session" json:"session_id"`
	PositionID    string  `gorm:"column:position_id;default:'';index:idx_paper_trade_orders_position" json:"position_id"`
	Symbol        string  `gorm:"column:symbol;not null;index:idx_paper_trade_orders_symbol" json:"symbol"`
	Side          string  `gorm:"column:side;not null" json:"side"`
	PositionSide  string  `gorm:"column:position_side;default:''" json:"position_side"`
	Type          string  `gorm:"column:type;not null;default:''" json:"type"`
	TriggerSubtype string `gorm:"column:trigger_subtype;default:''" json:"trigger_subtype"`
	OrderAction   string  `gorm:"column:order_action;default:''" json:"order_action"`
	Quantity      float64 `gorm:"column:quantity;default:0" json:"quantity"`
	Price         float64 `gorm:"column:price;default:0" json:"price"`
	StopPrice     float64 `gorm:"column:stop_price;default:0" json:"stop_price"`
	FilledQuantity float64 `gorm:"column:filled_quantity;default:0" json:"filled_quantity"`
	AvgFillPrice  float64 `gorm:"column:avg_fill_price;default:0" json:"avg_fill_price"`
	Commission    float64 `gorm:"column:commission;default:0" json:"commission"`
	ReduceOnly    bool    `gorm:"column:reduce_only;default:false" json:"reduce_only"`
	ClosePosition bool    `gorm:"column:close_position;default:false" json:"close_position"`
	Status        string  `gorm:"column:status;default:NEW;index:idx_paper_trade_orders_status" json:"status"`
	CreatedAt     int64   `gorm:"column:created_at;default:0" json:"created_at"`
	UpdatedAt     int64   `gorm:"column:updated_at;default:0" json:"updated_at"`
	FilledAt      int64   `gorm:"column:filled_at;default:0" json:"filled_at"`
}

func (PaperTradeOrder) TableName() string {
	return "paper_trade_orders"
}

func (s *PaperWalletStore) initTradeStateTables() error {
	return s.db.AutoMigrate(&PaperTradePosition{}, &PaperTradeOrder{})
}

func (s *PaperWalletStore) SavePaperTradePosition(position *PaperTradePosition) error {
	if position == nil {
		return nil
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(position).Error
}

func (s *PaperWalletStore) DeletePaperTradePosition(id string) error {
	return s.db.Where("id = ?", strings.TrimSpace(id)).Delete(&PaperTradePosition{}).Error
}

func (s *PaperWalletStore) GetPaperTradePositionBySymbol(traderID, exchangeID, symbol, side string) (*PaperTradePosition, error) {
	var position PaperTradePosition
	err := s.db.
		Where("trader_id = ? AND exchange_id = ? AND symbol = ? AND side = ?", traderID, exchangeID, strings.TrimSpace(symbol), strings.ToUpper(strings.TrimSpace(side))).
		First(&position).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &position, nil
}

func (s *PaperWalletStore) ListPaperTradePositions(traderID, exchangeID string) ([]*PaperTradePosition, error) {
	var positions []*PaperTradePosition
	err := s.db.
		Where("trader_id = ? AND exchange_id = ?", traderID, exchangeID).
		Order("created_at ASC").
		Find(&positions).Error
	if err != nil {
		return nil, err
	}
	return positions, nil
}

func (s *PaperWalletStore) ListPaperTradePositionsByExchange(exchangeID string) ([]*PaperTradePosition, error) {
	var positions []*PaperTradePosition
	err := s.db.
		Where("exchange_id = ?", exchangeID).
		Order("created_at ASC").
		Find(&positions).Error
	if err != nil {
		return nil, err
	}
	return positions, nil
}

func (s *PaperWalletStore) SavePaperTradeOrder(order *PaperTradeOrder) error {
	if order == nil {
		return nil
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(order).Error
}

func (s *PaperWalletStore) GetPaperTradeOrder(exchangeID, orderID string) (*PaperTradeOrder, error) {
	var order PaperTradeOrder
	err := s.db.Where("exchange_id = ? AND id = ?", exchangeID, strings.TrimSpace(orderID)).First(&order).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (s *PaperWalletStore) ListOpenPaperTradeOrders(traderID, exchangeID, symbol string) ([]*PaperTradeOrder, error) {
	query := s.db.Where("trader_id = ? AND exchange_id = ? AND status = ?", traderID, exchangeID, "NEW")
	if trimmed := strings.TrimSpace(symbol); trimmed != "" {
		query = query.Where("symbol = ?", trimmed)
	}
	var orders []*PaperTradeOrder
	err := query.Order("created_at ASC").Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}

func (s *PaperWalletStore) CancelPaperTradeOrder(id string, timestampMs int64) error {
	return s.db.Model(&PaperTradeOrder{}).
		Where("id = ?", strings.TrimSpace(id)).
		Updates(map[string]any{
			"status":     "CANCELED",
			"updated_at": timestampMs,
		}).Error
}

type PaperWalletLedgerMutation struct {
	EventType             string
	Asset                 string
	DeltaAvailableBalance float64
	DeltaUsedMargin       float64
	DeltaUnrealizedPnL    float64
	DeltaRealizedPnL      float64
	DeltaFees             float64
	DeltaFunding          float64
	ReferenceType         string
	ReferenceID           string
	Note                  string
	MetaJSON              string
}

func (s *PaperWalletStore) ApplyLedgerMutation(exchangeID string, mutation PaperWalletLedgerMutation) (*PaperWalletState, error) {
	var updatedWallet *PaperWalletState
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var wallet PaperWalletState
		if err := tx.Where("exchange_id = ?", strings.TrimSpace(exchangeID)).First(&wallet).Error; err != nil {
			return err
		}

		wallet.AvailableBalance += mutation.DeltaAvailableBalance
		wallet.UsedMargin += mutation.DeltaUsedMargin
		wallet.UnrealizedPnL += mutation.DeltaUnrealizedPnL
		wallet.RealizedPnL += mutation.DeltaRealizedPnL
		wallet.TotalFees += mutation.DeltaFees
		wallet.TotalFunding += mutation.DeltaFunding
		wallet.Equity = computePaperWalletEquity(&wallet)
		nowMs := currentPaperWalletTimestampMs()
		wallet.UpdatedAt = nowMs

		if wallet.AvailableBalance < 0 {
			return fmt.Errorf("paper wallet available balance fell below zero for exchange %s", exchangeID)
		}
		if wallet.UsedMargin < 0 {
			wallet.UsedMargin = 0
		}

		if err := tx.Save(&wallet).Error; err != nil {
			return err
		}

		ledger := &PaperWalletLedgerEntry{
			ID:                    uuid.NewString(),
			UserID:                wallet.UserID,
			ExchangeID:            wallet.ExchangeID,
			WalletID:              wallet.ID,
			SessionID:             wallet.SessionID,
			EventType:             blankToDefault(strings.TrimSpace(mutation.EventType), PaperWalletLedgerEventConfigSync),
			Asset:                 blankToDefault(normalizePaperAsset(mutation.Asset), wallet.Asset),
			DeltaAvailableBalance: mutation.DeltaAvailableBalance,
			DeltaUsedMargin:       mutation.DeltaUsedMargin,
			DeltaUnrealizedPnL:    mutation.DeltaUnrealizedPnL,
			DeltaRealizedPnL:      mutation.DeltaRealizedPnL,
			DeltaFees:             mutation.DeltaFees,
			DeltaFunding:          mutation.DeltaFunding,
			DeltaEquity:           wallet.Equity - updatedWalletEquityBeforeMutation(&wallet, mutation),
			AvailableAfter:        wallet.AvailableBalance,
			UsedMarginAfter:       wallet.UsedMargin,
			UnrealizedAfter:       wallet.UnrealizedPnL,
			RealizedAfter:         wallet.RealizedPnL,
			FeesAfter:             wallet.TotalFees,
			FundingAfter:          wallet.TotalFunding,
			EquityAfter:           wallet.Equity,
			ReferenceType:         strings.TrimSpace(mutation.ReferenceType),
			ReferenceID:           strings.TrimSpace(mutation.ReferenceID),
			Note:                  strings.TrimSpace(mutation.Note),
			MetaJSON:              strings.TrimSpace(mutation.MetaJSON),
			CreatedAt:             nowMs,
		}
		if err := tx.Create(ledger).Error; err != nil {
			return err
		}

		copyWallet := wallet
		updatedWallet = &copyWallet
		return nil
	})
	return updatedWallet, err
}

func (s *PaperWalletStore) UpdateWalletUnrealizedPnL(exchangeID string, unrealizedPnL float64) (*PaperWalletState, error) {
	var wallet PaperWalletState
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("exchange_id = ?", strings.TrimSpace(exchangeID)).First(&wallet).Error; err != nil {
			return err
		}
		wallet.UnrealizedPnL = unrealizedPnL
		wallet.Equity = computePaperWalletEquity(&wallet)
		wallet.UpdatedAt = currentPaperWalletTimestampMs()
		return tx.Save(&wallet).Error
	})
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func computePaperWalletEquity(wallet *PaperWalletState) float64 {
	if wallet == nil {
		return 0
	}
	return wallet.StartingBalance + wallet.RealizedPnL + wallet.UnrealizedPnL - wallet.TotalFees + wallet.TotalFunding
}

func currentPaperWalletTimestampMs() int64 {
	return nowUTC().UnixMilli()
}

func updatedWalletEquityBeforeMutation(wallet *PaperWalletState, mutation PaperWalletLedgerMutation) float64 {
	if wallet == nil {
		return 0
	}
	return computePaperWalletEquity(&PaperWalletState{
		StartingBalance: wallet.StartingBalance,
		RealizedPnL:     wallet.RealizedPnL - mutation.DeltaRealizedPnL,
		UnrealizedPnL:   wallet.UnrealizedPnL - mutation.DeltaUnrealizedPnL,
		TotalFees:       wallet.TotalFees - mutation.DeltaFees,
		TotalFunding:    wallet.TotalFunding - mutation.DeltaFunding,
	})
}

func blankToDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
