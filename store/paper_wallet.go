package store

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	PaperWalletLedgerEventSeed       = "seed"
	PaperWalletLedgerEventConfigSync = "config_sync"
)

type PaperWalletState struct {
	ID               string  `gorm:"primaryKey" json:"id"`
	UserID           string  `gorm:"column:user_id;not null;index:idx_paper_wallet_user" json:"user_id"`
	ExchangeID       string  `gorm:"column:exchange_id;not null;uniqueIndex:idx_paper_wallet_exchange" json:"exchange_id"`
	SourceVenue      string  `gorm:"column:source_venue;not null;default:''" json:"source_venue"`
	SessionID        string  `gorm:"column:session_id;not null;default:'';index:idx_paper_wallet_session" json:"session_id"`
	Asset            string  `gorm:"column:asset;not null;default:USDT" json:"asset"`
	StartingBalance  float64 `gorm:"column:starting_balance;default:0" json:"starting_balance"`
	AvailableBalance float64 `gorm:"column:available_balance;default:0" json:"available_balance"`
	UsedMargin       float64 `gorm:"column:used_margin;default:0" json:"used_margin"`
	UnrealizedPnL    float64 `gorm:"column:unrealized_pnl;default:0" json:"unrealized_pnl"`
	RealizedPnL      float64 `gorm:"column:realized_pnl;default:0" json:"realized_pnl"`
	TotalFees        float64 `gorm:"column:total_fees;default:0" json:"total_fees"`
	TotalFunding     float64 `gorm:"column:total_funding;default:0" json:"total_funding"`
	Equity           float64 `gorm:"column:equity;default:0" json:"equity"`
	SeededAt         int64   `gorm:"column:seeded_at;default:0" json:"seeded_at"`
	LastResetAt      int64   `gorm:"column:last_reset_at;default:0" json:"last_reset_at"`
	CreatedAt        int64   `gorm:"column:created_at;default:0" json:"created_at"`
	UpdatedAt        int64   `gorm:"column:updated_at;default:0" json:"updated_at"`
}

func (PaperWalletState) TableName() string {
	return "paper_wallet_states"
}

type PaperWalletLedgerEntry struct {
	ID                    string  `gorm:"primaryKey" json:"id"`
	UserID                string  `gorm:"column:user_id;not null;index:idx_paper_wallet_ledger_user" json:"user_id"`
	ExchangeID            string  `gorm:"column:exchange_id;not null;index:idx_paper_wallet_ledger_exchange" json:"exchange_id"`
	WalletID              string  `gorm:"column:wallet_id;not null;index:idx_paper_wallet_ledger_wallet" json:"wallet_id"`
	SessionID             string  `gorm:"column:session_id;not null;default:'';index:idx_paper_wallet_ledger_session" json:"session_id"`
	EventType             string  `gorm:"column:event_type;not null;index:idx_paper_wallet_ledger_event" json:"event_type"`
	Asset                 string  `gorm:"column:asset;not null;default:USDT" json:"asset"`
	DeltaAvailableBalance float64 `gorm:"column:delta_available_balance;default:0" json:"delta_available_balance"`
	DeltaUsedMargin       float64 `gorm:"column:delta_used_margin;default:0" json:"delta_used_margin"`
	DeltaUnrealizedPnL    float64 `gorm:"column:delta_unrealized_pnl;default:0" json:"delta_unrealized_pnl"`
	DeltaRealizedPnL      float64 `gorm:"column:delta_realized_pnl;default:0" json:"delta_realized_pnl"`
	DeltaFees             float64 `gorm:"column:delta_fees;default:0" json:"delta_fees"`
	DeltaFunding          float64 `gorm:"column:delta_funding;default:0" json:"delta_funding"`
	DeltaEquity           float64 `gorm:"column:delta_equity;default:0" json:"delta_equity"`
	AvailableAfter        float64 `gorm:"column:available_after;default:0" json:"available_after"`
	UsedMarginAfter       float64 `gorm:"column:used_margin_after;default:0" json:"used_margin_after"`
	UnrealizedAfter       float64 `gorm:"column:unrealized_after;default:0" json:"unrealized_after"`
	RealizedAfter         float64 `gorm:"column:realized_after;default:0" json:"realized_after"`
	FeesAfter             float64 `gorm:"column:fees_after;default:0" json:"fees_after"`
	FundingAfter          float64 `gorm:"column:funding_after;default:0" json:"funding_after"`
	EquityAfter           float64 `gorm:"column:equity_after;default:0" json:"equity_after"`
	ReferenceType         string  `gorm:"column:reference_type;default:''" json:"reference_type"`
	ReferenceID           string  `gorm:"column:reference_id;default:''" json:"reference_id"`
	Note                  string  `gorm:"column:note;default:''" json:"note"`
	MetaJSON              string  `gorm:"column:meta_json;type:text;default:''" json:"meta_json"`
	CreatedAt             int64   `gorm:"column:created_at;default:0" json:"created_at"`
}

func (PaperWalletLedgerEntry) TableName() string {
	return "paper_wallet_ledger_entries"
}

type PaperWalletStore struct {
	db *gorm.DB
}

func NewPaperWalletStore(db *gorm.DB) *PaperWalletStore {
	return &PaperWalletStore{db: db}
}

func (s *PaperWalletStore) initTables() error {
	return s.db.AutoMigrate(&PaperWalletState{}, &PaperWalletLedgerEntry{})
}

func (s *PaperWalletStore) GetByExchangeID(userID, exchangeID string) (*PaperWalletState, error) {
	var wallet PaperWalletState
	if err := s.db.Where("user_id = ? AND exchange_id = ?", userID, exchangeID).First(&wallet).Error; err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (s *PaperWalletStore) EnsureForExchange(exchange *Exchange) (*PaperWalletState, error) {
	if exchange == nil {
		return nil, fmt.Errorf("paper wallet requires exchange config")
	}
	if !exchange.IsPaper() {
		return nil, fmt.Errorf("exchange %s is not configured for paper execution", exchange.ID)
	}

	nowMs := time.Now().UTC().UnixMilli()
	asset := normalizePaperAsset(exchange.PaperAsset)
	targetStartingBalance := exchange.PaperInitialBalance

	var wallet PaperWalletState
	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("user_id = ? AND exchange_id = ?", exchange.UserID, exchange.ID).First(&wallet).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			wallet = PaperWalletState{
				ID:               uuid.NewString(),
				UserID:           exchange.UserID,
				ExchangeID:       exchange.ID,
				SourceVenue:      exchange.ExchangeType,
				SessionID:        uuid.NewString(),
				Asset:            asset,
				StartingBalance:  targetStartingBalance,
				AvailableBalance: targetStartingBalance,
				UsedMargin:       0,
				UnrealizedPnL:    0,
				RealizedPnL:      0,
				TotalFees:        0,
				TotalFunding:     0,
				Equity:           targetStartingBalance,
				SeededAt:         nowMs,
				LastResetAt:      nowMs,
				CreatedAt:        nowMs,
				UpdatedAt:        nowMs,
			}
			if err := tx.Create(&wallet).Error; err != nil {
				return err
			}
			ledger := &PaperWalletLedgerEntry{
				ID:                    uuid.NewString(),
				UserID:                wallet.UserID,
				ExchangeID:            wallet.ExchangeID,
				WalletID:              wallet.ID,
				SessionID:             wallet.SessionID,
				EventType:             PaperWalletLedgerEventSeed,
				Asset:                 wallet.Asset,
				DeltaAvailableBalance: wallet.AvailableBalance,
				DeltaEquity:           wallet.Equity,
				AvailableAfter:        wallet.AvailableBalance,
				UsedMarginAfter:       wallet.UsedMargin,
				UnrealizedAfter:       wallet.UnrealizedPnL,
				RealizedAfter:         wallet.RealizedPnL,
				FeesAfter:             wallet.TotalFees,
				FundingAfter:          wallet.TotalFunding,
				EquityAfter:           wallet.Equity,
				ReferenceType:         "exchange",
				ReferenceID:           exchange.ID,
				Note:                  "Paper wallet seeded from exchange configuration.",
				CreatedAt:             nowMs,
			}
			return tx.Create(ledger).Error
		}
		if err != nil {
			return err
		}

		changed := false
		recordFinancialSync := false
		deltaAvailable := 0.0
		deltaEquity := 0.0

		if wallet.SourceVenue != exchange.ExchangeType {
			wallet.SourceVenue = exchange.ExchangeType
			changed = true
		}
		if wallet.Asset != asset {
			wallet.Asset = asset
			changed = true
		}

		if paperWalletCanSyncSeed(wallet) && !floatEquals(wallet.StartingBalance, targetStartingBalance) {
			deltaAvailable = targetStartingBalance - wallet.AvailableBalance
			deltaEquity = targetStartingBalance - wallet.Equity
			wallet.StartingBalance = targetStartingBalance
			wallet.AvailableBalance = targetStartingBalance
			wallet.Equity = targetStartingBalance
			changed = true
			recordFinancialSync = true
		}

		if !changed {
			return nil
		}

		wallet.UpdatedAt = nowMs
		if err := tx.Save(&wallet).Error; err != nil {
			return err
		}

		if !recordFinancialSync {
			return nil
		}

		ledger := &PaperWalletLedgerEntry{
			ID:                    uuid.NewString(),
			UserID:                wallet.UserID,
			ExchangeID:            wallet.ExchangeID,
			WalletID:              wallet.ID,
			SessionID:             wallet.SessionID,
			EventType:             PaperWalletLedgerEventConfigSync,
			Asset:                 wallet.Asset,
			DeltaAvailableBalance: deltaAvailable,
			DeltaEquity:           deltaEquity,
			AvailableAfter:        wallet.AvailableBalance,
			UsedMarginAfter:       wallet.UsedMargin,
			UnrealizedAfter:       wallet.UnrealizedPnL,
			RealizedAfter:         wallet.RealizedPnL,
			FeesAfter:             wallet.TotalFees,
			FundingAfter:          wallet.TotalFunding,
			EquityAfter:           wallet.Equity,
			ReferenceType:         "exchange",
			ReferenceID:           exchange.ID,
			Note:                  "Paper wallet synced to updated exchange seed configuration.",
			CreatedAt:             nowMs,
		}
		return tx.Create(ledger).Error
	})
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func paperWalletCanSyncSeed(wallet PaperWalletState) bool {
	return math.Abs(wallet.UsedMargin) < 1e-9 &&
		math.Abs(wallet.UnrealizedPnL) < 1e-9 &&
		math.Abs(wallet.RealizedPnL) < 1e-9 &&
		math.Abs(wallet.TotalFees) < 1e-9 &&
		math.Abs(wallet.TotalFunding) < 1e-9 &&
		floatEquals(wallet.AvailableBalance, wallet.Equity)
}

func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
