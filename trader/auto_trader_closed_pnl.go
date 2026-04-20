package trader

import (
	"nofx/logger"
	"nofx/store"
	tradertypes "nofx/trader/types"
	"sort"
	"time"
)

const (
	closedPnLSyncInterval = 60 * time.Second
	closedPnLOverlap      = 72 * time.Hour
	closedPnLMaxLookback  = 30 * 24 * time.Hour
	closedPnLFetchLimit   = 500
)

func (at *AutoTrader) startClosedPnLSync() {
	if at.store == nil || at.trader == nil {
		return
	}

	go func() {
		if err := at.SyncClosedPnLHistoryOnce(); err != nil {
			logger.Infof("⚠️ [%s] Initial closed PnL sync failed: %v", at.name, err)
		}

		ticker := time.NewTicker(closedPnLSyncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := at.SyncClosedPnLHistoryOnce(); err != nil {
					logger.Infof("⚠️ [%s] Closed PnL sync failed: %v", at.name, err)
				}
			case <-at.stopMonitorCh:
				return
			}
		}
	}()

	logger.Infof("🔄 [%s] Closed PnL sync enabled (every %v)", at.name, closedPnLSyncInterval)
}

// SyncClosedPnLHistoryOnce imports exchange-reported realized closes into immutable closed-position history.
// This complements order sync so TP/SL/manual/external closes and partial reductions are visible in history.
func (at *AutoTrader) SyncClosedPnLHistoryOnce() error {
	if at.store == nil || at.trader == nil {
		return nil
	}

	lastClosedMs, err := at.store.Position().GetLastClosedPositionTime(at.id)
	if err != nil {
		return err
	}

	startTime := time.Now().UTC().Add(-closedPnLMaxLookback)
	if lastClosedMs > 0 {
		candidate := time.UnixMilli(lastClosedMs).UTC().Add(-closedPnLOverlap)
		if candidate.After(startTime) {
			startTime = candidate
		}
	}

	records, err := at.trader.GetClosedPnL(startTime, closedPnLFetchLimit)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].ExitTime.Before(records[j].ExitTime)
	})

	converted := make([]store.ClosedPnLRecord, 0, len(records))
	for _, record := range records {
		converted = append(converted, convertClosedPnLRecord(record))
	}

	created, skipped, err := at.store.Position().SyncClosedPositions(at.id, at.exchangeID, at.exchange, converted)
	if err != nil {
		return err
	}
	if created > 0 || skipped > 0 {
		logger.Infof("📚 [%s] Closed PnL sync imported %d deals, skipped %d duplicates (fetched %d)", at.name, created, skipped, len(records))
	}
	return nil
}

func convertClosedPnLRecord(record tradertypes.ClosedPnLRecord) store.ClosedPnLRecord {
	return store.ClosedPnLRecord{
		Symbol:      record.Symbol,
		Side:        record.Side,
		EntryPrice:  record.EntryPrice,
		ExitPrice:   record.ExitPrice,
		Quantity:    record.Quantity,
		RealizedPnL: record.RealizedPnL,
		Fee:         record.Fee,
		Leverage:    record.Leverage,
		EntryTime:   record.EntryTime.UTC().UnixMilli(),
		ExitTime:    record.ExitTime.UTC().UnixMilli(),
		OrderID:     record.OrderID,
		CloseType:   record.CloseType,
		ExchangeID:  record.ExchangeID,
	}
}
