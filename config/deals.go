//go:build legacy_sqlite_config
// +build legacy_sqlite_config

package config

import (
	"database/sql"
	"time"
)

// DealRecord models a persisted trade lifecycle.
type DealRecord struct {
	ID                int64           `json:"id"`
	UserID            string          `json:"user_id"`
	TraderID          string          `json:"trader_id"`
	Exchange          string          `json:"exchange"`
	Symbol            string          `json:"symbol"`
	Side              string          `json:"side"`
	Leverage          int             `json:"leverage"`
	PositionSizeUSD   float64         `json:"position_size_usd"`
	Quantity          float64         `json:"quantity"`
	OpenPrice         float64         `json:"open_price"`
	OpenTime          time.Time       `json:"open_time"`
	OpenOrderID       string          `json:"open_order_id"`
	SystemPrompt      string          `json:"system_prompt"`
	UserPrompt        string          `json:"user_prompt"`
	Reasoning         string          `json:"reasoning"`
	CoTTrace          string          `json:"cot_trace"`
	DecisionJSON      string          `json:"decision_json"`
	MarketContextJSON string          `json:"market_context_json"`
	StopLoss          float64         `json:"stop_loss"`
	TakeProfit        float64         `json:"take_profit"`
	ClosePrice        sql.NullFloat64 `json:"close_price,omitempty"`
	CloseTime         sql.NullTime    `json:"close_time,omitempty"`
	CloseOrderID      sql.NullString  `json:"close_order_id,omitempty"`
	RealizedPnL       sql.NullFloat64 `json:"realized_pnl,omitempty"`
	RealizedPnLPct    sql.NullFloat64 `json:"realized_pnl_pct,omitempty"`
	DurationSeconds   sql.NullInt64   `json:"duration_seconds,omitempty"`
	WasStopLoss       sql.NullBool    `json:"was_stop_loss,omitempty"`
	CloseReason       sql.NullString  `json:"close_reason,omitempty"`
	Status            string          `json:"status"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// DealEvent records lifecycle updates for a deal.
type DealEvent struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	TraderID   string    `json:"trader_id"`
	DealID     int64     `json:"deal_id"`
	Type       string    `json:"type"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Quantity   float64   `json:"quantity,omitempty"`
	Percentage float64   `json:"percentage,omitempty"`
	Price      float64   `json:"price,omitempty"`
	OrderID    string    `json:"order_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// TrailingStopLog persists trailing-stop updates for auditability.
type TrailingStopLog struct {
	ID                  int64           `json:"id"`
	UserID              string          `json:"user_id"`
	TraderID            string          `json:"trader_id"`
	DealID              int64           `json:"deal_id"`
	Symbol              string          `json:"symbol"`
	Side                string          `json:"side"`
	Action              string          `json:"action"`
	CurrentPrice        float64         `json:"current_price"`
	EntryPrice          float64         `json:"entry_price"`
	ProfitPct           float64         `json:"profit_pct"`
	TierIndex           int             `json:"tier_index"`
	TierThreshold       sql.NullFloat64 `json:"tier_threshold"`
	OldStopPrice        sql.NullFloat64 `json:"old_stop_price"`
	NewStopPrice        sql.NullFloat64 `json:"new_stop_price"`
	TargetStopProfitPct sql.NullFloat64 `json:"target_stop_profit_pct"`
	PriceChangePct      sql.NullFloat64 `json:"price_change_pct"`
	UpdateThresholdPct  sql.NullFloat64 `json:"update_threshold_pct"`
	ShouldUpdate        sql.NullBool    `json:"should_update"`
	APISuccess          sql.NullBool    `json:"api_success"`
	APIError            sql.NullString  `json:"api_error"`
	SkipReason          sql.NullString  `json:"skip_reason"`
	CreatedAt           time.Time       `json:"created_at"`
}

// GetDB exposes the raw sql.DB (used in some legacy call sites).
func (d *Database) GetDB() *sql.DB {
	return d.db
}

// ListDeals returns empty results placeholder; full implementation omitted in this build.
func (d *Database) ListDeals(userID, traderID, status, symbol, side, from, to, q, pnl string, pnlMin, pnlMax *float64, limit, offset int) ([]*DealRecord, error) {
	return []*DealRecord{}, nil
}

// CountDeals returns 0 as placeholder.
func (d *Database) CountDeals(userID, traderID, status, symbol, side, from, to, q, pnl string, pnlMin, pnlMax *float64) (int64, error) {
	return 0, nil
}

// GetDealByID returns nil placeholder.
func (d *Database) GetDealByID(userID, traderID string, id int64) (*DealRecord, error) {
	return nil, sql.ErrNoRows
}

// ListDealEvents returns empty placeholder.
func (d *Database) ListDealEvents(userID, traderID string, dealID int64) ([]DealEvent, error) {
	return []DealEvent{}, nil
}
