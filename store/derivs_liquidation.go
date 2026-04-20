package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultDerivsLiquidationRetention = 6 * time.Hour
	derivsLiquidationPruneInterval    = 30 * time.Minute
)

type DerivsLiquidationEvent struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Venue            string    `gorm:"column:venue;not null;index:idx_derivs_liquidation_symbol_time;index:idx_derivs_liquidation_unique,unique" json:"venue"`
	Symbol           string    `gorm:"column:symbol;not null;index:idx_derivs_liquidation_symbol_time;index:idx_derivs_liquidation_unique,unique" json:"symbol"`
	EventTime        time.Time `gorm:"column:event_time;not null;index:idx_derivs_liquidation_symbol_time,sort:desc" json:"event_time"`
	EventTimestampMs int64     `gorm:"column:event_timestamp_ms;not null;index:idx_derivs_liquidation_symbol_time,sort:desc;index:idx_derivs_liquidation_unique,unique" json:"event_timestamp_ms"`
	Side             string    `gorm:"column:side;not null;index:idx_derivs_liquidation_unique,unique" json:"side"`
	Price            float64   `gorm:"column:price;not null;index:idx_derivs_liquidation_unique,unique" json:"price"`
	Size             float64   `gorm:"column:size;not null;index:idx_derivs_liquidation_unique,unique" json:"size"`
	SourceTopic      string    `gorm:"column:source_topic;default:''" json:"source_topic"`
	CreatedAt        time.Time `json:"created_at"`
}

func (DerivsLiquidationEvent) TableName() string { return "derivs_liquidation_events" }

type DerivsLiquidationStore struct {
	db        *gorm.DB
	pruneMu   sync.Mutex
	lastPrune time.Time
}

func NewDerivsLiquidationStore(db *gorm.DB) *DerivsLiquidationStore {
	return &DerivsLiquidationStore{db: db}
}

func (s *DerivsLiquidationStore) initTables() error {
	return s.db.AutoMigrate(&DerivsLiquidationEvent{})
}

func (s *DerivsLiquidationStore) SaveBatch(events []DerivsLiquidationEvent) error {
	if s == nil || s.db == nil || len(events) == 0 {
		return nil
	}

	normalized := make([]DerivsLiquidationEvent, 0, len(events))
	for _, event := range events {
		event.Venue = strings.ToLower(strings.TrimSpace(event.Venue))
		event.Symbol = strings.ToUpper(strings.TrimSpace(event.Symbol))
		event.Side = strings.ToUpper(strings.TrimSpace(event.Side))
		event.SourceTopic = strings.TrimSpace(event.SourceTopic)
		if event.Symbol == "" || event.Venue == "" || event.Price <= 0 || event.Size <= 0 {
			continue
		}
		if event.EventTimestampMs <= 0 {
			if event.EventTime.IsZero() {
				continue
			}
			event.EventTimestampMs = event.EventTime.UTC().UnixMilli()
		}
		if event.EventTime.IsZero() {
			event.EventTime = time.UnixMilli(event.EventTimestampMs).UTC()
		} else {
			event.EventTime = event.EventTime.UTC()
		}
		normalized = append(normalized, event)
	}
	if len(normalized) == 0 {
		return nil
	}

	if err := s.db.Omit("ID").Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(normalized, 200).Error; err != nil {
		return fmt.Errorf("save derivs liquidation batch: %w", err)
	}
	_ = s.pruneIfDue(time.Now().UTC())
	return nil
}

func (s *DerivsLiquidationStore) ListRecent(symbol string, since time.Time, limit int) ([]DerivsLiquidationEvent, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}

	var events []DerivsLiquidationEvent
	query := s.db.Where("symbol = ?", symbol)
	if !since.IsZero() {
		query = query.Where("event_time >= ?", since.UTC())
	}
	if err := query.Order("event_time DESC").Limit(limit).Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list recent derivs liquidations for %s: %w", symbol, err)
	}

	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, nil
}

func (s *DerivsLiquidationStore) PruneOlderThan(cutoff time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	result := s.db.Where("event_time < ?", cutoff.UTC()).Delete(&DerivsLiquidationEvent{})
	if result.Error != nil {
		return 0, fmt.Errorf("prune derivs liquidations older than %s: %w", cutoff.UTC().Format(time.RFC3339), result.Error)
	}
	return result.RowsAffected, nil
}

func (s *DerivsLiquidationStore) pruneIfDue(now time.Time) error {
	s.pruneMu.Lock()
	if !s.lastPrune.IsZero() && now.Sub(s.lastPrune) < derivsLiquidationPruneInterval {
		s.pruneMu.Unlock()
		return nil
	}
	s.lastPrune = now
	s.pruneMu.Unlock()

	_, err := s.PruneOlderThan(now.Add(-defaultDerivsLiquidationRetention))
	return err
}
