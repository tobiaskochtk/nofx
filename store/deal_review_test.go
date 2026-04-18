package store

import (
	"database/sql"
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func TestEnsureCaseForPositionTxRefreshesExistingCase(t *testing.T) {
	root := newPositionHistoryTestStore(t, "deal-review-conflict.db")

	trader := &Trader{
		ID:             "trader-deal-review-conflict",
		UserID:         "user-deal-review-conflict",
		Name:           "Deal Review Conflict Trader",
		AIModelID:      "model-deal-review-conflict",
		ExchangeID:     "exchange-deal-review-conflict",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	now := time.Now().UTC()
	position := &TraderPosition{
		TraderID:           trader.ID,
		ExchangeID:         trader.ExchangeID,
		ExchangeType:       "bybit",
		ExchangePositionID: "position-deal-review-conflict",
		Symbol:             "BTCUSDT",
		Side:               "LONG",
		Quantity:           1.25,
		EntryQuantity:      1.25,
		EntryPrice:         64250,
		EntryTime:          now.Add(-20 * time.Minute).UnixMilli(),
		EntryOrderID:       "live-entry-order",
		CreatedAt:          now.Add(-20 * time.Minute).UnixMilli(),
		UpdatedAt:          now.Add(-20 * time.Minute).UnixMilli(),
	}
	if err := root.gdb.Create(position).Error; err != nil {
		t.Fatalf("Create(position) error = %v", err)
	}

	existing := &DealReviewCase{
		ID:           "case-existing",
		UserID:       trader.UserID,
		TraderID:     trader.ID,
		PositionID:   position.ID,
		ExchangeID:   "stale-exchange",
		ExchangeType: "stale-type",
		Symbol:       "STALEUSDT",
		Side:         "SHORT",
		Status:       DealReviewCaseStatusOpen,
		Outcome:      "open",
		EntryOrderID: "stale-entry-order",
	}
	if err := root.gdb.Create(existing).Error; err != nil {
		t.Fatalf("Create(existing case) error = %v", err)
	}

	var createdCase *DealReviewCase
	if err := root.gdb.Transaction(func(tx *gorm.DB) error {
		var err error
		createdCase, err = root.DealReview().ensureCaseForPositionTx(tx, position)
		return err
	}); err != nil {
		t.Fatalf("ensureCaseForPositionTx() error = %v", err)
	}
	if createdCase == nil {
		t.Fatal("expected deal review case to be returned")
	}
	if createdCase.ID != "case-existing" {
		t.Fatalf("createdCase.ID = %q, want case-existing", createdCase.ID)
	}

	var cases []DealReviewCase
	if err := root.gdb.Where("position_id = ?", position.ID).Find(&cases).Error; err != nil {
		t.Fatalf("Find(cases) error = %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("len(cases) = %d, want 1", len(cases))
	}

	caseRec := cases[0]
	if caseRec.ID != "case-existing" {
		t.Fatalf("caseRec.ID = %q, want case-existing", caseRec.ID)
	}
	if caseRec.ExchangeID != position.ExchangeID {
		t.Fatalf("caseRec.ExchangeID = %q, want %q", caseRec.ExchangeID, position.ExchangeID)
	}
	if caseRec.ExchangeType != position.ExchangeType {
		t.Fatalf("caseRec.ExchangeType = %q, want %q", caseRec.ExchangeType, position.ExchangeType)
	}
	if caseRec.Symbol != position.Symbol {
		t.Fatalf("caseRec.Symbol = %q, want %q", caseRec.Symbol, position.Symbol)
	}
	if caseRec.Side != normalizeDealReviewSide(position.Side) {
		t.Fatalf("caseRec.Side = %q, want %q", caseRec.Side, normalizeDealReviewSide(position.Side))
	}
	if caseRec.EntryOrderID != position.EntryOrderID {
		t.Fatalf("caseRec.EntryOrderID = %q, want %q", caseRec.EntryOrderID, position.EntryOrderID)
	}
}

func TestDealReviewBackfillsDecisionContextFromHistoricalRecords(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-1",
		UserID:         "user-1",
		Name:           "Historical Review Trader",
		AIModelID:      "model-1",
		ExchangeID:     "exchange-1",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-2 * time.Hour).UnixMilli()
	exitTime := time.UnixMilli(entryTime).Add(45 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:            trader.ID,
		CycleNumber:         101,
		Timestamp:           time.UnixMilli(entryTime).Add(-1 * time.Minute).UTC(),
		SystemPrompt:        "system-open-prompt",
		InputPrompt:         "market-context-open-prompt",
		DecisionJSON:        `{"decisions":[{"sym":"ETHUSDT","action":"ENTER","side":"long"}]}`,
		RawResponse:         "<reasoning>Momentum and breadth aligned</reasoning><decision>{\"decisions\":[{\"sym\":\"ETHUSDT\",\"action\":\"ENTER\",\"side\":\"long\"}]}</decision>",
		AIRequestDurationMs: 1450,
		CandidateMetaVer:    DecisionCandidateMetadataVersion,
		AccountState: AccountSnapshot{
			TotalBalance:          1200,
			AvailableBalance:      900,
			TotalUnrealizedProfit: 12,
			PositionCount:         1,
			MarginUsedPct:         18,
			InitialBalance:        1000,
		},
		Positions: []PositionSnapshot{
			{
				Symbol:           "BTCUSDT",
				Side:             "LONG",
				PositionAmt:      0.03,
				EntryPrice:       64000,
				MarkPrice:        64250,
				UnrealizedProfit: 7.5,
				Leverage:         3,
				LiquidationPrice: 51000,
			},
		},
		CandidateCoins: []string{"ETHUSDT", "SOLUSDT"},
		CandidateDetails: []CandidateDetail{
			{Symbol: "ETHUSDT", SelectionBucket: "primary", Sources: []string{"ai500", "oi_top"}},
		},
		ExecutionLog: []string{"selected ETHUSDT", "risk check passed"},
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "ETHUSDT",
				Quantity:   1.25,
				Leverage:   5,
				Price:      2500,
				StopLoss:   2425,
				TakeProfit: 2680,
				Confidence: 83,
				Reasoning:  "Momentum and breadth aligned",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	closeRecord := &DecisionRecord{
		TraderID:            trader.ID,
		CycleNumber:         102,
		Timestamp:           time.UnixMilli(exitTime).Add(-2 * time.Minute).UTC(),
		SystemPrompt:        "system-close-prompt",
		InputPrompt:         "market-context-close-prompt",
		DecisionJSON:        `{"decisions":[{"sym":"ETHUSDT","action":"EXIT","side":"long"}]}`,
		RawResponse:         "<reasoning>Take-profit target hit</reasoning><decision>{\"decisions\":[{\"sym\":\"ETHUSDT\",\"action\":\"EXIT\",\"side\":\"long\"}]}</decision>",
		AIRequestDurationMs: 980,
		ExecutionLog: []string{
			"tp band touched",
		},
		Decisions: []DecisionAction{
			{
				Action:     "close_long",
				Symbol:     "ETHUSDT",
				Quantity:   1.25,
				Price:      2680,
				Confidence: 79,
				Reasoning:  "Take-profit target hit",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(closeRecord); err != nil {
		t.Fatalf("Decision().LogDecision(close) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "binance",
		Symbol:        "ETHUSDT",
		Side:          "LONG",
		EntryQuantity: 1.25,
		Quantity:      1.25,
		EntryPrice:    2500,
		EntryOrderID:  "entry-1",
		EntryTime:     entryTime,
		Leverage:      5,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 2680, "exit-1", exitTime, 180, 4, "take_profit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, summary, total, err := root.DealReview().ListCases("user-1", DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if summary == nil || summary.ClosedDeals != 1 {
		t.Fatalf("summary.ClosedDeals = %v, want 1", summary)
	}

	detail, err := root.DealReview().GetCaseDetail("user-1", trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.Open == nil || detail.Open.Event == nil {
		t.Fatal("expected open event detail to be present")
	}
	if detail.Close == nil || detail.Close.Event == nil {
		t.Fatal("expected close event detail to be present")
	}
	if detail.Open.Event.DecisionCycleNumber != 101 {
		t.Fatalf("open cycle = %d, want 101", detail.Open.Event.DecisionCycleNumber)
	}
	if detail.Close.Event.DecisionCycleNumber != 102 {
		t.Fatalf("close cycle = %d, want 102", detail.Close.Event.DecisionCycleNumber)
	}
	if detail.Open.Event.Reasoning != "Momentum and breadth aligned" {
		t.Fatalf("open reasoning = %q", detail.Open.Event.Reasoning)
	}
	if detail.Close.Event.Reasoning != "Take-profit target hit" {
		t.Fatalf("close reasoning = %q", detail.Close.Event.Reasoning)
	}
	if detail.Open.Event.SelectionBucket != "primary" {
		t.Fatalf("open selection bucket = %q, want primary", detail.Open.Event.SelectionBucket)
	}
	if len(detail.Open.CandidateSources) != 2 {
		t.Fatalf("open candidate sources len = %d, want 2", len(detail.Open.CandidateSources))
	}
	if detail.Open.Snapshot == nil || len(detail.Open.Snapshot.ExecutionLog) != 2 {
		t.Fatalf("expected open snapshot execution log to be backfilled, got %#v", detail.Open.Snapshot)
	}
	if detail.Open.Snapshot == nil || detail.Open.Snapshot.SystemPrompt != "system-open-prompt" {
		t.Fatalf("open snapshot system prompt = %#v, want system-open-prompt", detail.Open.Snapshot)
	}
	if detail.Open.Snapshot == nil || detail.Open.Snapshot.UserPrompt != "market-context-open-prompt" {
		t.Fatalf("open snapshot user prompt = %#v, want market-context-open-prompt", detail.Open.Snapshot)
	}
	if detail.Open.Snapshot == nil || detail.Open.Snapshot.AIRequestMs != 1450 {
		t.Fatalf("open snapshot ai_request_duration_ms = %#v, want 1450", detail.Open.Snapshot)
	}
	if detail.Open.DecisionRecord == nil || detail.Open.DecisionRecord.AccountState.TotalBalance != 1200 {
		t.Fatalf("open decision record account state = %#v, want total balance 1200", detail.Open.DecisionRecord)
	}
	if detail.Close.Snapshot == nil || detail.Close.Snapshot.UserPrompt != "market-context-close-prompt" {
		t.Fatalf("close snapshot user prompt = %#v, want market-context-close-prompt", detail.Close.Snapshot)
	}
}

func TestDealReviewPersistsAndFiltersMarketContextRegimes(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-regimes.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-regime-1",
		UserID:         "user-regime-1",
		Name:           "Regime Review Trader",
		AIModelID:      "model-regime-1",
		ExchangeID:     "exchange-regime-1",
		InitialBalance: 1500,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryDecisionTime := time.Date(2026, time.April, 15, 2, 15, 0, 0, time.UTC)
	exitDecisionTime := time.Date(2026, time.April, 15, 14, 20, 0, 0, time.UTC)
	entryTime := entryDecisionTime.Add(1 * time.Minute).UnixMilli()
	exitTime := exitDecisionTime.Add(20 * time.Minute).UnixMilli()

	openPromptPayload, _ := json.Marshal(map[string]any{
		"candidates": []map[string]any{
			{
				"sym":     "ETHUSDT",
				"px_mark": 2500.0,
				"ctx": map[string]any{
					"tf":         "15m",
					"px_type":    "mark",
					"ema_fast":   2492.0,
					"macd":       0.08,
					"rsi":        61.0,
					"chg_1h":     1.6,
					"chg_4h":     3.2,
					"oi_d1h_pct": 6.4,
					"fund_bps":   5.7,
					"basis_pct":  0.12,
				},
				"execution_quality": map[string]any{
					"spread_bps":          2.2,
					"liq_score":           0.86,
					"slippage_est_25usd":  0.9,
					"slippage_est_100usd": 3.1,
				},
				"venue_tradability": map[string]any{
					"supported":       true,
					"book":            "healthy",
					"price_source":    "mark",
					"min_notional_ok": true,
				},
				"relative_strength": map[string]any{
					"vs_btc_1h":  1.8,
					"vs_btc_4h":  1.2,
					"vs_btc_ctx": 1.4,
					"state":      "outperform",
				},
				"feature_availability": map[string]any{
					"freshness": "fresh",
				},
				"feat": map[string]any{
					"volatility": map[string]any{
						"regime": "expansion",
					},
				},
			},
		},
	})

	closePromptPayload, _ := json.Marshal(map[string]any{
		"positions": []map[string]any{
			{
				"sym":     "ETHUSDT",
				"side":    "LONG",
				"px_mark": 2638.0,
				"ctx": map[string]any{
					"tf":         "15m",
					"px_type":    "mark",
					"ema_fast":   2645.0,
					"macd":       -0.03,
					"rsi":        46.0,
					"chg_1h":     -0.5,
					"chg_4h":     0.4,
					"oi_d1h_pct": -3.8,
					"fund_bps":   -6.2,
					"basis_pct":  -0.09,
				},
				"execution_quality": map[string]any{
					"spread_bps":          9.4,
					"liq_score":           0.48,
					"slippage_est_25usd":  3.6,
					"slippage_est_100usd": 8.7,
				},
				"venue_tradability": map[string]any{
					"supported":    true,
					"book":         "thin",
					"price_source": "mark",
				},
				"relative_strength": map[string]any{
					"vs_btc_1h":  -1.6,
					"vs_btc_4h":  -1.1,
					"vs_btc_ctx": -1.3,
					"state":      "lagging",
				},
				"feature_availability": map[string]any{
					"freshness": "fresh",
				},
				"feat": map[string]any{
					"volatility": map[string]any{
						"regime": "compression",
					},
				},
			},
		},
	})

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      501,
		Timestamp:        entryDecisionTime,
		InputPrompt:      string(openPromptPayload),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		CandidateCoins:   []string{"ETHUSDT"},
		CandidateDetails: []CandidateDetail{{Symbol: "ETHUSDT", SelectionBucket: "momentum", Sources: []string{"ai500"}}},
		Decisions: []DecisionAction{{
			Action:     "open_long",
			Symbol:     "ETHUSDT",
			Quantity:   1.5,
			Leverage:   5,
			Price:      2500,
			StopLoss:   2440,
			TakeProfit: 2680,
			Confidence: 84,
			Reasoning:  "Momentum held through Asia session.",
		}},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	closeRecord := &DecisionRecord{
		TraderID:    trader.ID,
		CycleNumber: 502,
		Timestamp:   exitDecisionTime,
		InputPrompt: string(closePromptPayload),
		Decisions: []DecisionAction{{
			Action:     "close_long",
			Symbol:     "ETHUSDT",
			Quantity:   1.5,
			Price:      2638,
			Confidence: 73,
			Reasoning:  "Momentum faded into US session.",
		}},
		Success: true,
	}
	if err := root.Decision().LogDecision(closeRecord); err != nil {
		t.Fatalf("Decision().LogDecision(close) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "ETHUSDT",
		Side:          "LONG",
		EntryQuantity: 1.5,
		Quantity:      1.5,
		EntryPrice:    2500,
		EntryOrderID:  "entry-regime-1",
		EntryTime:     entryTime,
		Leverage:      5,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 2638, "exit-regime-1", exitTime, 207, 6, "take_profit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("ListCases() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	caseRec := items[0].Case
	if caseRec.OpenTrendRegime != "uptrend" {
		t.Fatalf("OpenTrendRegime = %q, want uptrend", caseRec.OpenTrendRegime)
	}
	if caseRec.OpenVolatilityRegime != "high_vol" {
		t.Fatalf("OpenVolatilityRegime = %q, want high_vol", caseRec.OpenVolatilityRegime)
	}
	if caseRec.OpenFundingRegime != "extreme_longs" {
		t.Fatalf("OpenFundingRegime = %q, want extreme_longs", caseRec.OpenFundingRegime)
	}
	if caseRec.OpenSessionBucket != "asia" {
		t.Fatalf("OpenSessionBucket = %q, want asia", caseRec.OpenSessionBucket)
	}
	if caseRec.CloseVolatilityRegime != "low_vol" {
		t.Fatalf("CloseVolatilityRegime = %q, want low_vol", caseRec.CloseVolatilityRegime)
	}
	if caseRec.CloseFundingRegime != "extreme_shorts" {
		t.Fatalf("CloseFundingRegime = %q, want extreme_shorts", caseRec.CloseFundingRegime)
	}
	if caseRec.CloseSessionBucket != "us" {
		t.Fatalf("CloseSessionBucket = %q, want us", caseRec.CloseSessionBucket)
	}

	filtered, _, totalFiltered, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{
		TraderID:              trader.ID,
		OpenTrendRegime:       "uptrend",
		OpenVolatilityRegime:  "high_vol",
		OpenSessionBucket:     "asia",
		OpenBTCStrengthRegime: "outperform",
	})
	if err != nil {
		t.Fatalf("ListCases(regime filter) error = %v", err)
	}
	if totalFiltered != 1 || len(filtered) != 1 {
		t.Fatalf("filtered total = %d len = %d, want 1", totalFiltered, len(filtered))
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, caseRec.ID)
	if err != nil {
		t.Fatalf("GetCaseDetail() error = %v", err)
	}
	if detail.Open == nil || detail.Open.Snapshot == nil || detail.Open.Snapshot.MarketContext == nil {
		t.Fatalf("expected open market context snapshot, got %#v", detail.Open)
	}
	if detail.Open.Snapshot.MarketContext.TrendRegime != "uptrend" {
		t.Fatalf("open market context trend = %q, want uptrend", detail.Open.Snapshot.MarketContext.TrendRegime)
	}
	if detail.Close == nil || detail.Close.Snapshot == nil || detail.Close.Snapshot.MarketContext == nil {
		t.Fatalf("expected close market context snapshot, got %#v", detail.Close)
	}
	if detail.Close.Snapshot.MarketContext.VenueTier != "thin_book" {
		t.Fatalf("close venue tier = %q, want thin_book", detail.Close.Snapshot.MarketContext.VenueTier)
	}
}

func TestBackfillEventDecisionArtifactsPersistsPromptBundle(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-event-backfill.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	record := &DecisionRecord{
		TraderID:            "trader-event-backfill",
		CycleNumber:         444,
		Timestamp:           time.Now().UTC().Add(-2 * time.Minute),
		SystemPrompt:        "system-prompt-backfill",
		InputPrompt:         "input-prompt-backfill",
		DecisionJSON:        `{"decisions":[{"action":"open_long","symbol":"RAVEUSDT"}]}`,
		RawResponse:         `{"decision":"open_long"}`,
		AIRequestDurationMs: 712,
		AccountState: AccountSnapshot{
			TotalBalance:     1337,
			AvailableBalance: 1001,
			PositionCount:    2,
			InitialBalance:   1000,
		},
		Positions: []PositionSnapshot{
			{
				Symbol:      "BTCUSDT",
				Side:        "LONG",
				PositionAmt: 0.02,
				EntryPrice:  64000,
				MarkPrice:   64120,
			},
		},
		ExecutionLog: []string{"selected RAVEUSDT", "submitted open_long"},
		Success:      true,
	}
	if err := root.Decision().LogDecision(record); err != nil {
		t.Fatalf("Decision().LogDecision() error = %v", err)
	}

	event := &DealReviewEvent{
		ID:                  "event-backfill-1",
		UserID:              "user-event-backfill",
		TraderID:            "trader-event-backfill",
		Stage:               DealReviewStageOpen,
		Source:              DealReviewEventSourceSync,
		Status:              DealReviewEventStatusLinked,
		DecisionCycleNumber: 444,
		DecisionTimestamp:   time.Now().UTC().Add(-90 * time.Second),
		Symbol:              "RAVEUSDT",
		Side:                "LONG",
		Action:              "open_long",
		SnapshotJSON:        "{}",
	}
	if err := root.gdb.Create(event).Error; err != nil {
		t.Fatalf("Create(event) error = %v", err)
	}

	if err := root.DealReview().BackfillEventDecisionArtifacts(); err != nil {
		t.Fatalf("BackfillEventDecisionArtifacts() error = %v", err)
	}

	var refreshed DealReviewEvent
	if err := root.gdb.Where("id = ?", event.ID).First(&refreshed).Error; err != nil {
		t.Fatalf("First(refreshed event) error = %v", err)
	}

	var snapshot DealReviewEventSnapshot
	if err := json.Unmarshal([]byte(refreshed.SnapshotJSON), &snapshot); err != nil {
		t.Fatalf("json.Unmarshal(snapshot) error = %v", err)
	}
	if snapshot.SystemPrompt != "system-prompt-backfill" {
		t.Fatalf("snapshot.SystemPrompt = %q, want system-prompt-backfill", snapshot.SystemPrompt)
	}
	if snapshot.UserPrompt != "input-prompt-backfill" {
		t.Fatalf("snapshot.UserPrompt = %q, want input-prompt-backfill", snapshot.UserPrompt)
	}
	if snapshot.AIRequestMs != 712 {
		t.Fatalf("snapshot.AIRequestMs = %d, want 712", snapshot.AIRequestMs)
	}
	if snapshot.AccountState.TotalBalance != 1337 {
		t.Fatalf("snapshot.AccountState.TotalBalance = %.2f, want 1337", snapshot.AccountState.TotalBalance)
	}
	if len(snapshot.Positions) != 1 {
		t.Fatalf("len(snapshot.Positions) = %d, want 1", len(snapshot.Positions))
	}
}

func TestDealReviewPriceTimelineCaptureAndBackfill(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-price-timeline.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-price-timeline",
		UserID:         "user-price-timeline",
		Name:           "Price Timeline Trader",
		AIModelID:      "model-price-timeline",
		ExchangeID:     "exchange-price-timeline",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-25 * time.Minute).UnixMilli()
	cycleOne := time.UnixMilli(entryTime).Add(5 * time.Minute).UTC()
	cycleTwo := time.UnixMilli(entryTime).Add(11 * time.Minute).UTC()
	exitTime := time.UnixMilli(entryTime).Add(18 * time.Minute).UnixMilli()

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "RAVEUSDT",
		Side:          "LONG",
		EntryQuantity: 3,
		Quantity:      3,
		EntryPrice:    4.20,
		EntryOrderID:  "entry-rave-timeline",
		EntryTime:     entryTime,
		Leverage:      4,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	recordOne := &DecisionRecord{
		TraderID:    trader.ID,
		CycleNumber: 901,
		Timestamp:   cycleOne,
		Positions: []PositionSnapshot{{
			Symbol:           "RAVEUSDT",
			Side:             "LONG",
			PositionAmt:      3,
			EntryPrice:       4.20,
			MarkPrice:        4.29,
			UnrealizedProfit: 0.27,
			Leverage:         4,
		}},
		Success: true,
	}
	if err := root.Decision().LogDecision(recordOne); err != nil {
		t.Fatalf("Decision().LogDecision(recordOne) error = %v", err)
	}
	if err := root.DealReview().CaptureDecisionCyclePricePoints(trader.UserID, recordOne); err != nil {
		t.Fatalf("CaptureDecisionCyclePricePoints(recordOne) error = %v", err)
	}

	recordTwo := &DecisionRecord{
		TraderID:    trader.ID,
		CycleNumber: 902,
		Timestamp:   cycleTwo,
		Positions: []PositionSnapshot{{
			Symbol:           "RAVEUSDT",
			Side:             "LONG",
			PositionAmt:      3,
			EntryPrice:       4.20,
			MarkPrice:        4.12,
			UnrealizedProfit: -0.24,
			Leverage:         4,
		}},
		Success: true,
	}
	if err := root.Decision().LogDecision(recordTwo); err != nil {
		t.Fatalf("Decision().LogDecision(recordTwo) error = %v", err)
	}
	if err := root.DealReview().CaptureLivePositionPricePoints(trader.UserID, trader.ID, []PositionSnapshot{{
		Symbol:           "RAVEUSDT",
		Side:             "LONG",
		PositionAmt:      3,
		EntryPrice:       4.20,
		MarkPrice:        4.24,
		UnrealizedProfit: 0.12,
		Leverage:         4,
	}}, cycleOne.Add(2*time.Minute), "platform"); err != nil {
		t.Fatalf("CaptureLivePositionPricePoints(1) error = %v", err)
	}
	if err := root.DealReview().CaptureLivePositionPricePoints(trader.UserID, trader.ID, []PositionSnapshot{{
		Symbol:           "RAVEUSDT",
		Side:             "LONG",
		PositionAmt:      3,
		EntryPrice:       4.20,
		MarkPrice:        4.18,
		UnrealizedProfit: -0.06,
		Leverage:         4,
	}}, cycleTwo.Add(2*time.Minute), "platform"); err != nil {
		t.Fatalf("CaptureLivePositionPricePoints(2) error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 4.26, "exit-rave-timeline", exitTime, 0.18, 0.03, "take_profit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	if err := root.DealReview().BackfillDecisionCyclePricePoints(); err != nil {
		t.Fatalf("BackfillDecisionCyclePricePoints() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.PriceTimeline == nil {
		t.Fatal("expected price timeline to be present")
	}
	if detail.PriceTimeline.Summary.CycleSamples != 2 {
		t.Fatalf("CycleSamples = %d, want 2", detail.PriceTimeline.Summary.CycleSamples)
	}
	if detail.PriceTimeline.Summary.PlatformSamples != 2 {
		t.Fatalf("PlatformSamples = %d, want 2", detail.PriceTimeline.Summary.PlatformSamples)
	}
	if !detail.PriceTimeline.Summary.EverInProfit {
		t.Fatal("expected EverInProfit to be true")
	}
	if detail.PriceTimeline.Summary.MaxUnrealizedPnL <= 0 {
		t.Fatalf("MaxUnrealizedPnL = %.4f, want > 0", detail.PriceTimeline.Summary.MaxUnrealizedPnL)
	}
	if detail.PriceTimeline.Summary.MinUnrealizedPnL >= 0 {
		t.Fatalf("MinUnrealizedPnL = %.4f, want < 0", detail.PriceTimeline.Summary.MinUnrealizedPnL)
	}
	if len(detail.PriceTimeline.Points) != 6 {
		t.Fatalf("len(PriceTimeline.Points) = %d, want 6", len(detail.PriceTimeline.Points))
	}
	if detail.PriceTimeline.Points[1].DecisionCycleNumber != 901 {
		t.Fatalf("first cycle point = %d, want 901", detail.PriceTimeline.Points[1].DecisionCycleNumber)
	}
	if detail.PriceTimeline.Points[2].Source != "platform" {
		t.Fatalf("second point source = %q, want platform", detail.PriceTimeline.Points[2].Source)
	}
	if detail.PriceTimeline.Points[3].DecisionCycleNumber != 902 {
		t.Fatalf("second cycle point = %d, want 902", detail.PriceTimeline.Points[3].DecisionCycleNumber)
	}
	if detail.PriceTimeline.Points[4].Source != "platform" {
		t.Fatalf("fifth point source = %q, want platform", detail.PriceTimeline.Points[4].Source)
	}
}

func TestDealReviewCaseAnnotationsAndAnomalies(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-annotations.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-annotations",
		UserID:         "user-annotations",
		Name:           "Annotation Trader",
		AIModelID:      "model-annotations",
		ExchangeID:     "exchange-annotations",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	cases := []DealReviewCase{
		{
			ID:                  "case-1",
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          1,
			Symbol:              "BTCUSDT",
			Side:                "LONG",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             "loss",
			EntryTimeMs:         time.Now().UTC().Add(-4 * time.Hour).UnixMilli(),
			ExitTimeMs:          time.Now().UTC().Add(-3 * time.Hour).UnixMilli(),
			RealizedPnL:         -45,
			HoldDurationMs:      int64((55 * time.Minute) / time.Millisecond),
			OpenSelectionBucket: "breakout",
			CloseReason:         "stop_loss",
		},
		{
			ID:                  "case-2",
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          2,
			Symbol:              "BTCUSDT",
			Side:                "LONG",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             "loss",
			EntryTimeMs:         time.Now().UTC().Add(-3 * time.Hour).UnixMilli(),
			ExitTimeMs:          time.Now().UTC().Add(-2 * time.Hour).UnixMilli(),
			RealizedPnL:         -35,
			HoldDurationMs:      int64((42 * time.Minute) / time.Millisecond),
			OpenSelectionBucket: "breakout",
			CloseReason:         "stop_loss",
		},
		{
			ID:                  "case-3",
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          3,
			Symbol:              "BTCUSDT",
			Side:                "SHORT",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             "loss",
			EntryTimeMs:         time.Now().UTC().Add(-2 * time.Hour).UnixMilli(),
			ExitTimeMs:          time.Now().UTC().Add(-90 * time.Minute).UnixMilli(),
			RealizedPnL:         -20,
			HoldDurationMs:      int64((35 * time.Minute) / time.Millisecond),
			OpenSelectionBucket: "breakout",
			CloseReason:         "unknown",
			ExitReasonQuality:   DealReviewExitReasonQualityLowConfidence,
			ExitOrigin:          DealReviewExitOriginSyncedCloseFill,
		},
		{
			ID:                  "case-4",
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          4,
			Symbol:              "SOLUSDT",
			Side:                "LONG",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             "profit",
			EntryTimeMs:         time.Now().UTC().Add(-70 * time.Minute).UnixMilli(),
			ExitTimeMs:          time.Now().UTC().Add(-40 * time.Minute).UnixMilli(),
			RealizedPnL:         60,
			HoldDurationMs:      int64((30 * time.Minute) / time.Millisecond),
			OpenSelectionBucket: "mean_revert",
			CloseReason:         "take_profit",
		},
	}
	for i := range cases {
		if err := root.gdb.Create(&cases[i]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", i, err)
		}
	}

	detail, err := root.DealReview().UpdateCaseReview(
		trader.UserID,
		trader.ID,
		"case-1",
		[]string{"avoidable loss", "good entry", "avoidable loss", " "},
		"Exit rule was overridden too early.",
	)
	if err != nil {
		t.Fatalf("UpdateCaseReview() error = %v", err)
	}
	if detail.Case.AnalystNote != "Exit rule was overridden too early." {
		t.Fatalf("analyst note = %q", detail.Case.AnalystNote)
	}
	if len(detail.Labels) != 2 || detail.Labels[0] != "avoidable loss" || detail.Labels[1] != "good entry" {
		t.Fatalf("labels = %#v, want normalized unique labels", detail.Labels)
	}

	summary, err := root.DealReview().GetAnomalySummary(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("GetAnomalySummary() error = %v", err)
	}
	if summary.ClosedDeals != 4 {
		t.Fatalf("ClosedDeals = %d, want 4", summary.ClosedDeals)
	}
	if len(summary.WorstSymbols) == 0 || summary.WorstSymbols[0].Symbol != "BTCUSDT" {
		t.Fatalf("WorstSymbols = %#v, want BTCUSDT first", summary.WorstSymbols)
	}
	if len(summary.OvertradedSymbols) == 0 || summary.OvertradedSymbols[0].Symbol != "BTCUSDT" {
		t.Fatalf("OvertradedSymbols = %#v, want BTCUSDT first", summary.OvertradedSymbols)
	}
	if len(summary.WeakBuckets) == 0 || summary.WeakBuckets[0].Bucket != "breakout" {
		t.Fatalf("WeakBuckets = %#v, want breakout first", summary.WeakBuckets)
	}
	if len(summary.WeakCloseReasons) == 0 || summary.WeakCloseReasons[0].Reason != "stop_loss" {
		t.Fatalf("WeakCloseReasons = %#v, want stop_loss first", summary.WeakCloseReasons)
	}
	if len(summary.ExitUncertainty) == 0 || summary.ExitUncertainty[0].CloseReason != "unknown" {
		t.Fatalf("ExitUncertainty = %#v, want unknown cohort", summary.ExitUncertainty)
	}
	if len(summary.Notes) == 0 {
		t.Fatal("expected anomaly notes to be generated")
	}

	filtered, _, _, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{
		TraderID:          trader.ID,
		ExitReasonQuality: DealReviewExitReasonQualityLowConfidence,
		Limit:             10,
	})
	if err != nil {
		t.Fatalf("ListCases() with exit_reason_quality filter error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].Case.ID != "case-3" {
		t.Fatalf("filtered low-confidence cases = %#v, want case-3 only", filtered)
	}
}

func TestDealReviewHeuristicClassifierAssistAndFeedback(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-classifier.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-classifier",
		UserID:         "user-classifier",
		Name:           "Classifier Trader",
		AIModelID:      "model-classifier",
		ExchangeID:     "exchange-classifier",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	makeLabels := func(values ...string) string {
		body, _ := json.Marshal(values)
		return string(body)
	}

	now := time.Now().UTC()
	cases := []DealReviewCase{
		{
			ID:                  "case-h1",
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          1,
			Symbol:              "BTCUSDT",
			Side:                "LONG",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             "loss",
			EntryTimeMs:         now.Add(-8 * time.Hour).UnixMilli(),
			ExitTimeMs:          now.Add(-7*time.Hour - 20*time.Minute).UnixMilli(),
			OpenSelectionBucket: "breakout",
			CloseReason:         "stop_loss",
			RealizedPnL:         -25,
			LabelsJSON:          makeLabels("avoidable loss"),
		},
		{
			ID:                  "case-h2",
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          2,
			Symbol:              "BTCUSDT",
			Side:                "LONG",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             "loss",
			EntryTimeMs:         now.Add(-7 * time.Hour).UnixMilli(),
			ExitTimeMs:          now.Add(-6*time.Hour - 10*time.Minute).UnixMilli(),
			OpenSelectionBucket: "breakout",
			CloseReason:         "stop_loss",
			RealizedPnL:         -18,
			LabelsJSON:          makeLabels("avoidable loss"),
		},
		{
			ID:                  "case-h3",
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          3,
			Symbol:              "BTCUSDT",
			Side:                "LONG",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             "loss",
			EntryTimeMs:         now.Add(-6 * time.Hour).UnixMilli(),
			ExitTimeMs:          now.Add(-5*time.Hour - 40*time.Minute).UnixMilli(),
			OpenSelectionBucket: "breakout",
			CloseReason:         "stop_loss",
			RealizedPnL:         -22,
			LabelsJSON:          makeLabels("bad trade"),
		},
		{
			ID:                  "case-h4",
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          4,
			Symbol:              "BTCUSDT",
			Side:                "LONG",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             "loss",
			EntryTimeMs:         now.Add(-5 * time.Hour).UnixMilli(),
			ExitTimeMs:          now.Add(-4*time.Hour - 45*time.Minute).UnixMilli(),
			OpenSelectionBucket: "breakout",
			CloseReason:         "stop_loss",
			RealizedPnL:         -21,
			LabelsJSON:          makeLabels("bad trade"),
		},
		{
			ID:                  "case-target",
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          5,
			Symbol:              "BTCUSDT",
			Side:                "LONG",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             "loss",
			EntryTimeMs:         now.Add(-90 * time.Minute).UnixMilli(),
			ExitTimeMs:          now.Add(-40 * time.Minute).UnixMilli(),
			OpenSelectionBucket: "breakout",
			CloseReason:         "stop_loss",
			RealizedPnL:         -16,
		},
	}
	for i := range cases {
		if err := root.gdb.Create(&cases[i]).Error; err != nil {
			t.Fatalf("Create(case %d) error = %v", i, err)
		}
	}

	items, _, _, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("ListCases() error = %v", err)
	}

	var targetItem *DealReviewCaseListItem
	for i := range items {
		if items[i].Case.ID == "case-target" {
			targetItem = &items[i]
			break
		}
	}
	if targetItem == nil {
		t.Fatal("target case not found in list")
	}
	if targetItem.ClassifierAssist == nil || len(targetItem.ClassifierAssist.Suggestions) < 2 {
		t.Fatalf("classifier assist = %#v, want at least 2 suggestions", targetItem.ClassifierAssist)
	}

	var avoidableLoss *DealReviewClassifierSuggestion
	var badTrade *DealReviewClassifierSuggestion
	for i := range targetItem.ClassifierAssist.Suggestions {
		suggestion := &targetItem.ClassifierAssist.Suggestions[i]
		switch suggestion.Label {
		case "avoidable loss":
			avoidableLoss = suggestion
		case "bad trade":
			badTrade = suggestion
		}
	}
	if avoidableLoss == nil || badTrade == nil {
		t.Fatalf("suggestions = %#v, want avoidable loss and bad trade", targetItem.ClassifierAssist.Suggestions)
	}

	detail, err := root.DealReview().ApplyClassifierFeedback(
		trader.UserID,
		trader.ID,
		"case-target",
		DealReviewClassifierHeuristic,
		avoidableLoss.SuggestionKey,
		avoidableLoss.Label,
		avoidableLoss.IssueType,
		DealReviewClassifierVerdictAccepted,
		avoidableLoss.Rationale,
		true,
	)
	if err != nil {
		t.Fatalf("ApplyClassifierFeedback(accept) error = %v", err)
	}
	if len(detail.Labels) == 0 || detail.Labels[0] != "avoidable loss" {
		t.Fatalf("labels after accept = %#v, want avoidable loss applied", detail.Labels)
	}

	detail, err = root.DealReview().ApplyClassifierFeedback(
		trader.UserID,
		trader.ID,
		"case-target",
		DealReviewClassifierHeuristic,
		badTrade.SuggestionKey,
		badTrade.Label,
		badTrade.IssueType,
		DealReviewClassifierVerdictRejected,
		badTrade.Rationale,
		false,
	)
	if err != nil {
		t.Fatalf("ApplyClassifierFeedback(reject) error = %v", err)
	}
	if detail.ClassifierAssist != nil {
		for _, suggestion := range detail.ClassifierAssist.Suggestions {
			if suggestion.Label == "bad trade" {
				t.Fatalf("bad trade suggestion still present after rejection: %#v", detail.ClassifierAssist.Suggestions)
			}
		}
	}
}

func TestDealReviewFilterPresetRoundTripAndCaseFilters(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-presets.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	filterBody, _ := json.Marshal(map[string]any{
		"symbol":                "BTCUSDT",
		"status":                "CLOSED",
		"date_range":            "30d",
		"open_selection_bucket": "momentum",
		"review_queue":          "unlabeled_losses",
	})
	preset := &DealReviewFilterPreset{
		UserID:     "user-presets",
		TraderID:   "trader-presets",
		Name:       "Momentum loss queue",
		FilterJSON: string(filterBody),
	}
	if err := root.DealReview().SaveFilterPreset(preset); err != nil {
		t.Fatalf("SaveFilterPreset() error = %v", err)
	}

	presets, err := root.DealReview().ListFilterPresets("user-presets", "trader-presets")
	if err != nil {
		t.Fatalf("ListFilterPresets() error = %v", err)
	}
	if len(presets) != 1 {
		t.Fatalf("len(presets) = %d, want 1", len(presets))
	}
	if presets[0].Preset.Name != "Momentum loss queue" {
		t.Fatalf("preset name = %q, want Momentum loss queue", presets[0].Preset.Name)
	}
	if presets[0].Filters["open_selection_bucket"] != "momentum" {
		t.Fatalf("preset filters = %#v, want open_selection_bucket momentum", presets[0].Filters)
	}

	caseA := DealReviewCase{
		ID:                  "case-filter-a",
		UserID:              "user-presets",
		TraderID:            "trader-presets",
		PositionID:          11,
		Symbol:              "BTCUSDT",
		Side:                "LONG",
		Status:              DealReviewCaseStatusClosed,
		Outcome:             "loss",
		EntryTimeMs:         time.Now().UTC().Add(-2 * time.Hour).UnixMilli(),
		ExitTimeMs:          time.Now().UTC().Add(-90 * time.Minute).UnixMilli(),
		EntryPrice:          100,
		ExitPrice:           98,
		EntryQuantity:       1,
		ExitQuantity:        1,
		OpenSelectionBucket: "momentum",
		CloseReason:         "trailing_stop",
		RealizedPnL:         -2,
		RealizedPnLPct:      -2,
	}
	caseB := DealReviewCase{
		ID:                  "case-filter-b",
		UserID:              "user-presets",
		TraderID:            "trader-presets",
		PositionID:          12,
		Symbol:              "ETHUSDT",
		Side:                "SHORT",
		Status:              DealReviewCaseStatusClosed,
		Outcome:             "profit",
		EntryTimeMs:         time.Now().UTC().Add(-70 * time.Minute).UnixMilli(),
		ExitTimeMs:          time.Now().UTC().Add(-20 * time.Minute).UnixMilli(),
		EntryPrice:          200,
		ExitPrice:           190,
		EntryQuantity:       1,
		ExitQuantity:        1,
		OpenSelectionBucket: "mean_revert",
		CloseReason:         "take_profit",
		RealizedPnL:         10,
		RealizedPnLPct:      5,
	}
	if err := root.gdb.Create(&caseA).Error; err != nil {
		t.Fatalf("Create(caseA) error = %v", err)
	}
	if err := root.gdb.Create(&caseB).Error; err != nil {
		t.Fatalf("Create(caseB) error = %v", err)
	}
	point := DealReviewCyclePointRecord{
		UserID:              "user-presets",
		TraderID:            "trader-presets",
		DealID:              caseA.ID,
		PositionID:          caseA.PositionID,
		Symbol:              caseA.Symbol,
		Side:                caseA.Side,
		TimestampMs:         caseA.EntryTimeMs + 30_000,
		DecisionCycleNumber: 99,
		MarkPrice:           101,
		EntryPrice:          100,
		Quantity:            1,
		UnrealizedPnL:       1,
		UnrealizedPnLPct:    1,
		InProfit:            true,
	}
	if err := root.gdb.Create(&point).Error; err != nil {
		t.Fatalf("Create(point) error = %v", err)
	}

	items, _, _, err := root.DealReview().ListCases("user-presets", DealReviewListFilter{
		TraderID:            "trader-presets",
		OpenSelectionBucket: "momentum",
	})
	if err != nil {
		t.Fatalf("ListCases(bucket) error = %v", err)
	}
	if len(items) != 1 || items[0].Case.ID != "case-filter-a" {
		t.Fatalf("bucket filtered items = %#v, want only case-filter-a", items)
	}
	if items[0].PriceTimelineSummary == nil || !items[0].PriceTimelineSummary.EverInProfit {
		t.Fatalf("timeline summary = %#v, want ever_in_profit true", items[0].PriceTimelineSummary)
	}

	items, _, _, err = root.DealReview().ListCases("user-presets", DealReviewListFilter{
		TraderID:    "trader-presets",
		CloseReason: "take_profit",
	})
	if err != nil {
		t.Fatalf("ListCases(close_reason) error = %v", err)
	}
	if len(items) != 1 || items[0].Case.ID != "case-filter-b" {
		t.Fatalf("close reason filtered items = %#v, want only case-filter-b", items)
	}
}

func TestDealReviewStrategyVersionRoundTrip(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-versions.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	prevJSON, _ := json.Marshal(map[string]any{
		"execution": map[string]any{"max_positions": 3},
	})
	nextJSON, _ := json.Marshal(map[string]any{
		"execution": map[string]any{"max_positions": 2},
		"risk":      map[string]any{"stop_loss_pct": 1.5},
	})
	targetCohortJSON, _ := json.Marshal(map[string]any{
		"symbol":    "TONUSDT",
		"status":    "CLOSED",
		"outcome":   "loss",
		"from_time": 123,
	})
	version := &DealReviewStrategyVersion{
		UserID:             "user-versions",
		TraderID:           "trader-versions",
		StrategyID:         "strategy-versions",
		SourceScanID:       "scan-versions",
		SourceType:         "ai_apply",
		Summary:            "Tighten exposure after weak bucket scan",
		ExpectedEffect:     "Reduce losses on TON longs",
		TargetCohortJSON:   string(targetCohortJSON),
		PreviousConfigJSON: string(prevJSON),
		NextConfigJSON:     string(nextJSON),
		AppliedAt:          time.Now().UTC().Add(-2 * time.Hour),
	}
	if err := root.DealReview().SaveStrategyVersion(version); err != nil {
		t.Fatalf("SaveStrategyVersion() error = %v", err)
	}

	detail, err := root.DealReview().GetStrategyVersion("user-versions", "trader-versions", version.ID)
	if err != nil {
		t.Fatalf("GetStrategyVersion() error = %v", err)
	}
	if detail.Version.SourceScanID != "scan-versions" {
		t.Fatalf("SourceScanID = %q, want scan-versions", detail.Version.SourceScanID)
	}
	if detail.Version.ExpectedEffect != "Reduce losses on TON longs" {
		t.Fatalf("ExpectedEffect = %q, want stored expected effect", detail.Version.ExpectedEffect)
	}
	if detail.TargetCohort["symbol"] != "TONUSDT" {
		t.Fatalf("target cohort = %#v, want symbol TONUSDT", detail.TargetCohort)
	}
	if _, exists := detail.TargetCohort["status"]; exists {
		t.Fatalf("target cohort = %#v, did not expect analysis-only status filter to persist", detail.TargetCohort)
	}
	execution, ok := detail.NextConfig["execution"].(map[string]any)
	if !ok || execution["max_positions"].(float64) != 2 {
		t.Fatalf("next execution config = %#v, want max_positions=2", detail.NextConfig["execution"])
	}
	risk, ok := detail.NextConfig["risk"].(map[string]any)
	if !ok || risk["stop_loss_pct"].(float64) != 1.5 {
		t.Fatalf("next risk config = %#v, want stop_loss_pct=1.5", detail.NextConfig["risk"])
	}

	versions, err := root.DealReview().ListStrategyVersions("user-versions", "trader-versions", 10)
	if err != nil {
		t.Fatalf("ListStrategyVersions() error = %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("len(versions) = %d, want 1", len(versions))
	}
}

func TestDealReviewStrategyVersionBuildsAttributionAndWarnings(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "version-attribution.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	appliedAt := time.Now().UTC().Add(-2 * time.Hour)
	targetCohortJSON, _ := json.Marshal(map[string]any{"symbol": "TONUSDT"})
	version := &DealReviewStrategyVersion{
		UserID:           "user-attr",
		TraderID:         "trader-attr",
		StrategyID:       "strategy-attr",
		SourceType:       "ai_apply",
		Summary:          "Tighten TON long entries",
		ExpectedEffect:   "Reduce weak TON long losses",
		TargetCohortJSON: string(targetCohortJSON),
		AppliedAt:        appliedAt,
	}
	if err := root.DealReview().SaveStrategyVersion(version); err != nil {
		t.Fatalf("SaveStrategyVersion() error = %v", err)
	}

	beforeTON := DealReviewCase{
		ID:          "case-before-ton",
		UserID:      "user-attr",
		TraderID:    "trader-attr",
		PositionID:  1,
		Symbol:      "TONUSDT",
		Side:        "LONG",
		Status:      DealReviewCaseStatusClosed,
		ExitTimeMs:  appliedAt.Add(-1 * time.Hour).UnixMilli(),
		RealizedPnL: 1.2,
	}
	beforeTONB := DealReviewCase{
		ID:          "case-before-ton-b",
		UserID:      "user-attr",
		TraderID:    "trader-attr",
		PositionID:  7,
		Symbol:      "TONUSDT",
		Side:        "LONG",
		Status:      DealReviewCaseStatusClosed,
		ExitTimeMs:  appliedAt.Add(-80 * time.Minute).UnixMilli(),
		RealizedPnL: 0.8,
	}
	beforeTONC := DealReviewCase{
		ID:          "case-before-ton-c",
		UserID:      "user-attr",
		TraderID:    "trader-attr",
		PositionID:  8,
		Symbol:      "TONUSDT",
		Side:        "LONG",
		Status:      DealReviewCaseStatusClosed,
		ExitTimeMs:  appliedAt.Add(-20 * time.Minute).UnixMilli(),
		RealizedPnL: 0.6,
	}
	beforeBTC := DealReviewCase{
		ID:          "case-before-btc",
		UserID:      "user-attr",
		TraderID:    "trader-attr",
		PositionID:  2,
		Symbol:      "BTCUSDT",
		Side:        "LONG",
		Status:      DealReviewCaseStatusClosed,
		ExitTimeMs:  appliedAt.Add(-30 * time.Minute).UnixMilli(),
		RealizedPnL: 0.4,
	}
	afterTONA := DealReviewCase{
		ID:          "case-after-ton-a",
		UserID:      "user-attr",
		TraderID:    "trader-attr",
		PositionID:  3,
		Symbol:      "TONUSDT",
		Side:        "LONG",
		Status:      DealReviewCaseStatusClosed,
		ExitTimeMs:  appliedAt.Add(30 * time.Minute).UnixMilli(),
		RealizedPnL: -0.9,
	}
	afterTONB := DealReviewCase{
		ID:          "case-after-ton-b",
		UserID:      "user-attr",
		TraderID:    "trader-attr",
		PositionID:  4,
		Symbol:      "TONUSDT",
		Side:        "LONG",
		Status:      DealReviewCaseStatusClosed,
		ExitTimeMs:  appliedAt.Add(70 * time.Minute).UnixMilli(),
		RealizedPnL: -0.7,
	}
	afterTONC := DealReviewCase{
		ID:          "case-after-ton-c",
		UserID:      "user-attr",
		TraderID:    "trader-attr",
		PositionID:  9,
		Symbol:      "TONUSDT",
		Side:        "LONG",
		Status:      DealReviewCaseStatusClosed,
		ExitTimeMs:  appliedAt.Add(100 * time.Minute).UnixMilli(),
		RealizedPnL: -0.5,
	}
	afterBTCA := DealReviewCase{
		ID:          "case-after-btc-a",
		UserID:      "user-attr",
		TraderID:    "trader-attr",
		PositionID:  5,
		Symbol:      "BTCUSDT",
		Side:        "LONG",
		Status:      DealReviewCaseStatusClosed,
		ExitTimeMs:  appliedAt.Add(80 * time.Minute).UnixMilli(),
		RealizedPnL: 0.2,
	}
	afterBTCB := DealReviewCase{
		ID:          "case-after-btc-b",
		UserID:      "user-attr",
		TraderID:    "trader-attr",
		PositionID:  6,
		Symbol:      "BTCUSDT",
		Side:        "LONG",
		Status:      DealReviewCaseStatusClosed,
		ExitTimeMs:  appliedAt.Add(90 * time.Minute).UnixMilli(),
		RealizedPnL: 0.1,
	}

	if err := gdb.Create(&beforeTON).Error; err != nil {
		t.Fatalf("Create(beforeTON) error = %v", err)
	}
	if err := gdb.Create(&beforeTONB).Error; err != nil {
		t.Fatalf("Create(beforeTONB) error = %v", err)
	}
	if err := gdb.Create(&beforeTONC).Error; err != nil {
		t.Fatalf("Create(beforeTONC) error = %v", err)
	}
	if err := gdb.Create(&beforeBTC).Error; err != nil {
		t.Fatalf("Create(beforeBTC) error = %v", err)
	}
	if err := gdb.Create(&afterTONA).Error; err != nil {
		t.Fatalf("Create(afterTONA) error = %v", err)
	}
	if err := gdb.Create(&afterTONB).Error; err != nil {
		t.Fatalf("Create(afterTONB) error = %v", err)
	}
	if err := gdb.Create(&afterTONC).Error; err != nil {
		t.Fatalf("Create(afterTONC) error = %v", err)
	}
	if err := gdb.Create(&afterBTCA).Error; err != nil {
		t.Fatalf("Create(afterBTCA) error = %v", err)
	}
	if err := gdb.Create(&afterBTCB).Error; err != nil {
		t.Fatalf("Create(afterBTCB) error = %v", err)
	}

	detail, err := root.DealReview().GetStrategyVersion("user-attr", "trader-attr", version.ID)
	if err != nil {
		t.Fatalf("GetStrategyVersion() error = %v", err)
	}
	if detail.Attribution == nil {
		t.Fatalf("Attribution = nil, want populated attribution")
	}
	if detail.Attribution.TargetAfterSummary == nil || detail.Attribution.TargetAfterSummary.NetPnL >= detail.Attribution.TargetBeforeSummary.NetPnL {
		t.Fatalf("target attribution = %#v, want weaker target cohort after apply", detail.Attribution)
	}
	if !detail.Attribution.RollbackSuggested {
		t.Fatalf("RollbackSuggested = false, want rollback suggestion after weaker target cohort")
	}
	if len(detail.Attribution.Warnings) == 0 {
		t.Fatalf("Warnings = %#v, want attribution warning", detail.Attribution.Warnings)
	}
}

func TestCreateFromClosedPnLDeduplicatesMatchingClosedPosition(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed-pnl-dedupe.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	exitTime := time.Now().UTC().Add(-5 * time.Minute).UnixMilli()
	existing := &TraderPosition{
		TraderID:      "trader-dedupe",
		ExchangeID:    "exchange-dedupe",
		ExchangeType:  "bybit",
		Symbol:        "TONUSDT",
		Side:          "LONG",
		Quantity:      16,
		EntryQuantity: 16,
		EntryPrice:    1.4937,
		ExitPrice:     1.4652,
		EntryTime:     exitTime - int64((80*time.Minute)/time.Millisecond),
		ExitTime:      exitTime,
		RealizedPnL:   -0.48,
		Fee:           0.02,
		Status:        DealReviewCaseStatusClosed,
		CloseReason:   "sync",
		Source:        "sync",
		CreatedAt:     time.Now().UTC().UnixMilli(),
		UpdatedAt:     time.Now().UTC().UnixMilli(),
	}
	if err := root.gdb.Create(existing).Error; err != nil {
		t.Fatalf("Create(existing position) error = %v", err)
	}

	record := &ClosedPnLRecord{
		Symbol:      "TONUSDT",
		Side:        "long",
		EntryPrice:  1.4937,
		ExitPrice:   1.4652,
		Quantity:    16,
		RealizedPnL: -0.4852,
		Fee:         0.0283,
		EntryTime:   existing.EntryTime,
		ExitTime:    exitTime,
		OrderID:     "bybit-order-1",
		ExchangeID:  "closed-pnl-record-1",
		CloseType:   "unknown",
	}

	created, err := root.Position().CreateFromClosedPnL("trader-dedupe", "exchange-dedupe", "bybit", record)
	if err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}
	if created {
		t.Fatal("expected matching closed position to be deduplicated")
	}

	var count int64
	if err := root.gdb.Model(&TraderPosition{}).Where("trader_id = ?", "trader-dedupe").Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestCreateFromClosedPnLBackfillsOpenDecisionContext(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed-pnl-open-context.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-closed-pnl-context",
		UserID:         "user-closed-pnl-context",
		Name:           "Closed PnL Context Trader",
		AIModelID:      "model-closed-pnl-context",
		ExchangeID:     "exchange-closed-pnl-context",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-20 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-3 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      501,
		Timestamp:        time.UnixMilli(entryTime).Add(-45 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		CandidateCoins:   []string{"RAVEUSDT", "TONUSDT"},
		CandidateDetails: []CandidateDetail{
			{Symbol: "RAVEUSDT", SelectionBucket: "oi_breakout", Sources: []string{"oi_leader_context", "trend_align"}},
		},
		ExecutionLog: []string{"selected RAVEUSDT", "submitted open_long"},
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "RAVEUSDT",
				Quantity:   3,
				Leverage:   5,
				Price:      4.22223,
				StopLoss:   4.08,
				TakeProfit: 4.46,
				Confidence: 74,
				Reasoning:  "Breakout aligned with OI expansion",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	record := &ClosedPnLRecord{
		Symbol:      "RAVEUSDT",
		Side:        "LONG",
		EntryPrice:  4.22223,
		ExitPrice:   4.30395,
		Quantity:    3,
		RealizedPnL: 0.2170236,
		Fee:         0.0281364,
		Leverage:    5,
		EntryTime:   entryTime,
		ExitTime:    exitTime,
		OrderID:     "exit-rave-1",
		ExchangeID:  "closed-pnl-rave-1",
		CloseType:   "unknown",
	}

	created, err := root.Position().CreateFromClosedPnL(trader.ID, trader.ExchangeID, "bybit", record)
	if err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}
	if !created {
		t.Fatal("expected closed PnL record to create a new closed position")
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.Open == nil || detail.Open.Event == nil {
		t.Fatal("expected open event detail to be linked")
	}
	if detail.Close == nil || detail.Close.Event == nil {
		t.Fatal("expected close event detail to be linked")
	}
	if detail.Open.Event.DecisionCycleNumber != 501 {
		t.Fatalf("open decision cycle = %d, want 501", detail.Open.Event.DecisionCycleNumber)
	}
	if detail.Open.Event.Reasoning != "Breakout aligned with OI expansion" {
		t.Fatalf("open reasoning = %q", detail.Open.Event.Reasoning)
	}
	if detail.Open.Event.StopLoss != 4.08 {
		t.Fatalf("open stop loss = %v, want 4.08", detail.Open.Event.StopLoss)
	}
	if detail.Open.Event.TakeProfit != 4.46 {
		t.Fatalf("open take profit = %v, want 4.46", detail.Open.Event.TakeProfit)
	}
	if len(detail.Open.CandidateSources) != 2 {
		t.Fatalf("candidate sources len = %d, want 2", len(detail.Open.CandidateSources))
	}
	if detail.Open.Snapshot == nil || len(detail.Open.Snapshot.ExecutionLog) != 2 {
		t.Fatalf("expected open snapshot execution log, got %#v", detail.Open.Snapshot)
	}
}

func TestCreateFromClosedPnLBackfillsTimelineWithoutPollutingOlderOverlappingCase(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed-pnl-timeline-overlap.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-closed-pnl-timeline",
		UserID:         "user-closed-pnl-timeline",
		Name:           "Closed PnL Timeline Trader",
		AIModelID:      "model-closed-pnl-timeline",
		ExchangeID:     "exchange-closed-pnl-timeline",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	oldEntryTime := time.Now().UTC().Add(-3 * time.Hour).UnixMilli()
	oldOpen := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "RAVEUSDT",
		Side:          "LONG",
		EntryQuantity: 17,
		Quantity:      17,
		EntryPrice:    7.366863,
		EntryOrderID:  "entry-rave-old-open",
		EntryTime:     oldEntryTime,
		Leverage:      5,
		CreatedAt:     oldEntryTime,
		UpdatedAt:     oldEntryTime,
	}
	if err := root.Position().Create(oldOpen); err != nil {
		t.Fatalf("Position().Create(oldOpen) error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-40 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-5 * time.Minute).UnixMilli()
	cycleOne := time.UnixMilli(entryTime).Add(6 * time.Minute).UTC()
	cycleTwo := time.UnixMilli(entryTime).Add(13 * time.Minute).UTC()

	recordOne := &DecisionRecord{
		TraderID:    trader.ID,
		CycleNumber: 901,
		Timestamp:   cycleOne,
		Positions: []PositionSnapshot{{
			Symbol:           "RAVEUSDT",
			Side:             "LONG",
			PositionAmt:      1,
			EntryPrice:       9.83876,
			MarkPrice:        9.85391,
			UnrealizedProfit: 0.01515,
			Leverage:         5,
		}},
		Success: true,
	}
	if err := root.Decision().LogDecision(recordOne); err != nil {
		t.Fatalf("Decision().LogDecision(recordOne) error = %v", err)
	}

	recordTwo := &DecisionRecord{
		TraderID:    trader.ID,
		CycleNumber: 902,
		Timestamp:   cycleTwo,
		Positions: []PositionSnapshot{{
			Symbol:           "RAVEUSDT",
			Side:             "LONG",
			PositionAmt:      1,
			EntryPrice:       9.83876,
			MarkPrice:        9.88888,
			UnrealizedProfit: 0.05012,
			Leverage:         5,
		}},
		Success: true,
	}
	if err := root.Decision().LogDecision(recordTwo); err != nil {
		t.Fatalf("Decision().LogDecision(recordTwo) error = %v", err)
	}

	created, err := root.Position().CreateFromClosedPnL(trader.ID, trader.ExchangeID, "bybit", &ClosedPnLRecord{
		Symbol:      "RAVEUSDT",
		Side:        "LONG",
		EntryPrice:  9.83876,
		ExitPrice:   10.06093,
		Quantity:    1,
		RealizedPnL: 0.22217,
		Fee:         0.02,
		Leverage:    5,
		EntryTime:   entryTime,
		ExitTime:    exitTime,
		OrderID:     "exit-rave-timeline-overlap",
		ExchangeID:  "closed-pnl-rave-timeline-overlap",
		CloseType:   "take_profit",
	})
	if err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}
	if !created {
		t.Fatal("expected closed PnL record to create a new closed position")
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("total/items = %d/%d, want 2/2", total, len(items))
	}

	var closedCaseID, oldCaseID string
	for _, item := range items {
		switch {
		case math.Abs(item.Case.EntryPrice-9.83876) < 0.000001:
			closedCaseID = item.Case.ID
		case math.Abs(item.Case.EntryPrice-7.366863) < 0.000001:
			oldCaseID = item.Case.ID
		}
	}
	if closedCaseID == "" || oldCaseID == "" {
		t.Fatalf("failed to identify expected cases: %#v", items)
	}

	closedDetail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, closedCaseID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail(closed) error = %v", err)
	}
	if closedDetail.PriceTimeline == nil {
		t.Fatal("expected closed deal to have a price timeline")
	}
	if closedDetail.PriceTimeline.Summary.CycleSamples != 2 {
		t.Fatalf("closed CycleSamples = %d, want 2", closedDetail.PriceTimeline.Summary.CycleSamples)
	}

	if err := root.DealReview().BackfillDecisionCyclePricePoints(); err != nil {
		t.Fatalf("BackfillDecisionCyclePricePoints() error = %v", err)
	}

	oldDetail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, oldCaseID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail(old) error = %v", err)
	}
	if oldDetail.PriceTimeline == nil {
		t.Fatal("expected old overlapping case to still return a timeline payload")
	}
	if oldDetail.PriceTimeline.Summary.CycleSamples != 0 {
		t.Fatalf("old overlapping case CycleSamples = %d, want 0", oldDetail.PriceTimeline.Summary.CycleSamples)
	}
}

func TestCreateFromClosedPnLInfersManualExitFromCloseFill(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed-pnl-manual-exit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-manual-exit",
		UserID:         "user-manual-exit",
		Name:           "Manual Exit Trader",
		AIModelID:      "model-manual-exit",
		ExchangeID:     "exchange-manual-exit",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-25 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-3 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      601,
		Timestamp:        time.UnixMilli(entryTime).Add(-30 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		CandidateDetails: []CandidateDetail{
			{Symbol: "RAVEUSDT", SelectionBucket: "breakout", Sources: []string{"trend_align", "oi_dislocation"}},
		},
		ExecutionLog: []string{"selected RAVEUSDT", "submitted open_long"},
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "RAVEUSDT",
				Quantity:   3,
				Leverage:   3,
				Price:      4.22223,
				StopLoss:   3.674,
				TakeProfit: 4.643,
				Confidence: 70,
				Reasoning:  "ai500, trend_align, oi_dislocation",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	order := &TraderOrder{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		ExchangeOrderID: "close-trade-rave-1",
		Symbol:          "RAVEUSDT",
		Side:            "SELL",
		PositionSide:    "BOTH",
		Type:            "Market",
		OrderAction:     "close_long",
		Quantity:        3,
		Price:           4.30395,
		Status:          "FILLED",
		FilledQuantity:  3,
		AvgFillPrice:    4.30395,
		Commission:      0.0281364,
		CreatedAt:       exitTime,
		UpdatedAt:       exitTime,
		FilledAt:        exitTime,
	}
	if err := root.Order().CreateOrder(order); err != nil {
		t.Fatalf("Order().CreateOrder() error = %v", err)
	}
	fill := &TraderFill{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		OrderID:         order.ID,
		ExchangeOrderID: "exit-rave-manual-1",
		ExchangeTradeID: "close-trade-rave-1",
		Symbol:          "RAVEUSDT",
		Side:            "SELL",
		Price:           4.30395,
		Quantity:        3,
		QuoteQuantity:   12.91185,
		Commission:      0.0281364,
		CommissionAsset: "USDT",
		CreatedAt:       exitTime,
	}
	if err := root.Order().CreateFill(fill); err != nil {
		t.Fatalf("Order().CreateFill() error = %v", err)
	}

	record := &ClosedPnLRecord{
		Symbol:      "RAVEUSDT",
		Side:        "LONG",
		EntryPrice:  4.22223,
		ExitPrice:   4.30395,
		Quantity:    3,
		RealizedPnL: 0.2170236,
		Fee:         0.0281364,
		Leverage:    3,
		EntryTime:   entryTime,
		ExitTime:    exitTime,
		OrderID:     "exit-rave-manual-1",
		ExchangeID:  "closed-pnl-manual-1",
		CloseType:   "unknown",
	}
	if _, err := root.Position().CreateFromClosedPnL(trader.ID, trader.ExchangeID, "bybit", record); err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.Case.CloseReason != "manual_exit" {
		t.Fatalf("close reason = %q, want manual_exit", detail.Case.CloseReason)
	}
	if detail.Case.ExitOrigin != DealReviewExitOriginSyncedMarketOrder {
		t.Fatalf("exit origin = %q, want %q", detail.Case.ExitOrigin, DealReviewExitOriginSyncedMarketOrder)
	}
	if detail.Case.ExitReasonQuality != DealReviewExitReasonQualityHighConfidence {
		t.Fatalf("exit reason quality = %q, want %q", detail.Case.ExitReasonQuality, DealReviewExitReasonQualityHighConfidence)
	}
	if !strings.Contains(detail.Case.ExitEvidenceSummary, "Market close order") {
		t.Fatalf("exit evidence summary = %q, want Market close order evidence", detail.Case.ExitEvidenceSummary)
	}
	if detail.Close == nil || detail.Close.Event == nil || detail.Close.Event.CloseReason != "manual_exit" {
		t.Fatalf("close event = %#v, want close_reason=manual_exit", detail.Close)
	}
	if detail.Close.Event.ExitOrigin != DealReviewExitOriginSyncedMarketOrder {
		t.Fatalf("close event exit origin = %q, want %q", detail.Close.Event.ExitOrigin, DealReviewExitOriginSyncedMarketOrder)
	}
	if detail.Close.Event.ExitEvidence == nil || detail.Close.Event.ExitEvidence.OrderType != "Market" {
		t.Fatalf("close event exit evidence = %#v, want Market order evidence", detail.Close.Event.ExitEvidence)
	}
}

func TestCreateFromClosedPnLInfersTakeProfitFromTargets(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed-pnl-take-profit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-take-profit",
		UserID:         "user-take-profit",
		Name:           "Take Profit Trader",
		AIModelID:      "model-take-profit",
		ExchangeID:     "exchange-take-profit",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-40 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-5 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      701,
		Timestamp:        time.UnixMilli(entryTime).Add(-1 * time.Minute).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "RAVEUSDT",
				Quantity:   6,
				Leverage:   3,
				Price:      2.93919,
				StopLoss:   2.725,
				TakeProfit: 3.124,
				Confidence: 70,
				Reasoning:  "trend_align, profit target nearby",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	order := &TraderOrder{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		ExchangeOrderID: "close-trade-rave-2",
		Symbol:          "RAVEUSDT",
		Side:            "SELL",
		PositionSide:    "BOTH",
		Type:            "Market",
		OrderAction:     "close_long",
		Quantity:        6,
		Price:           3.12085,
		Status:          "FILLED",
		FilledQuantity:  6,
		AvgFillPrice:    3.12085,
		Commission:      0.03999627,
		CreatedAt:       exitTime,
		UpdatedAt:       exitTime,
		FilledAt:        exitTime,
	}
	if err := root.Order().CreateOrder(order); err != nil {
		t.Fatalf("Order().CreateOrder() error = %v", err)
	}
	fill := &TraderFill{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		OrderID:         order.ID,
		ExchangeOrderID: "exit-rave-tp-1",
		ExchangeTradeID: "close-trade-rave-2",
		Symbol:          "RAVEUSDT",
		Side:            "SELL",
		Price:           3.12085,
		Quantity:        6,
		QuoteQuantity:   18.7251,
		Commission:      0.03999627,
		CommissionAsset: "USDT",
		CreatedAt:       exitTime,
	}
	if err := root.Order().CreateFill(fill); err != nil {
		t.Fatalf("Order().CreateFill() error = %v", err)
	}

	record := &ClosedPnLRecord{
		Symbol:      "RAVEUSDT",
		Side:        "LONG",
		EntryPrice:  2.93919,
		ExitPrice:   3.12085,
		Quantity:    6,
		RealizedPnL: 1.04996373,
		Fee:         0.03999627,
		Leverage:    3,
		EntryTime:   entryTime,
		ExitTime:    exitTime,
		OrderID:     "exit-rave-tp-1",
		ExchangeID:  "closed-pnl-tp-1",
		CloseType:   "unknown",
	}
	if _, err := root.Position().CreateFromClosedPnL(trader.ID, trader.ExchangeID, "bybit", record); err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "take_profit" {
		t.Fatalf("close reason = %q, want take_profit", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginTargetProximity {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginTargetProximity)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityHighConfidence {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityHighConfidence)
	}
	if !strings.Contains(items[0].Case.ExitEvidenceSummary, "take profit target") {
		t.Fatalf("exit evidence summary = %q, want take profit target evidence", items[0].Case.ExitEvidenceSummary)
	}
}

func TestCreateFromClosedPnLInfersStopLossFromMatchedTriggerOrder(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed-pnl-trigger-stop.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-trigger-stop",
		UserID:         "user-trigger-stop",
		Name:           "Trigger Stop Trader",
		AIModelID:      "model-trigger-stop",
		ExchangeID:     "exchange-trigger-stop",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-55 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-6 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      751,
		Timestamp:        time.UnixMilli(entryTime).Add(-1 * time.Minute).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "TONUSDT",
				Quantity:   16,
				Leverage:   5,
				Price:      1.4937,
				Confidence: 80,
				Reasoning:  "momentum aligned",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	triggerOrder := &TraderOrder{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		ExchangeOrderID: "trigger-stop-ton-1",
		Symbol:          "TONUSDT",
		Side:            "SELL",
		PositionSide:    "LONG",
		Type:            "StopLoss",
		Status:          "FILLED",
		Quantity:        16,
		StopPrice:       1.503,
		AvgFillPrice:    1.4652,
		FilledQuantity:  16,
		ReduceOnly:      true,
		OrderAction:     "close_long",
		CreatedAt:       exitTime,
		UpdatedAt:       exitTime,
		FilledAt:        exitTime,
	}
	if err := root.Order().UpsertOrder(triggerOrder); err != nil {
		t.Fatalf("Order().UpsertOrder() error = %v", err)
	}

	record := &ClosedPnLRecord{
		Symbol:      "TONUSDT",
		Side:        "LONG",
		EntryPrice:  1.4937,
		ExitPrice:   1.4652,
		Quantity:    16,
		RealizedPnL: -0.48527651,
		Fee:         0.02838651,
		Leverage:    5,
		EntryTime:   entryTime,
		ExitTime:    exitTime,
		OrderID:     "trigger-stop-ton-1",
		ExchangeID:  "closed-pnl-trigger-stop-1",
		CloseType:   "unknown",
	}
	if _, err := root.Position().CreateFromClosedPnL(trader.ID, trader.ExchangeID, "bybit", record); err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "stop_loss" {
		t.Fatalf("close reason = %q, want stop_loss", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginSyncedTriggerOrder {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginSyncedTriggerOrder)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityExplicit {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityExplicit)
	}
	if !strings.Contains(items[0].Case.ExitEvidenceSummary, "StopLoss") {
		t.Fatalf("exit evidence summary = %q, want StopLoss trigger evidence", items[0].Case.ExitEvidenceSummary)
	}
}

func TestCreateFromClosedPnLInfersTakeProfitFromGenericStopOrderTriggerPrice(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed-pnl-trigger-take-profit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-trigger-tp",
		UserID:         "user-trigger-tp",
		Name:           "Trigger TP Trader",
		AIModelID:      "model-trigger-tp",
		ExchangeID:     "exchange-trigger-tp",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-35 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-4 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      761,
		Timestamp:        time.UnixMilli(entryTime).Add(-30 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "RAVEUSDT",
				Quantity:   4,
				Leverage:   3,
				Price:      3.44184,
				StopLoss:   3.035,
				TakeProfit: 3.395,
				Confidence: 70,
				Reasoning:  "trend_align, breakout follow-through",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	triggerOrder := &TraderOrder{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		ExchangeOrderID: "trigger-stop-rave-1",
		Symbol:          "RAVEUSDT",
		Side:            "SELL",
		PositionSide:    "LONG",
		Type:            "Stop",
		Status:          "FILLED",
		Quantity:        4,
		StopPrice:       3.395,
		AvgFillPrice:    3.39214,
		FilledQuantity:  4,
		ReduceOnly:      true,
		OrderAction:     "close_long",
		CreatedAt:       exitTime,
		UpdatedAt:       exitTime,
		FilledAt:        exitTime,
	}
	if err := root.Order().UpsertOrder(triggerOrder); err != nil {
		t.Fatalf("Order().UpsertOrder() error = %v", err)
	}

	record := &ClosedPnLRecord{
		Symbol:      "RAVEUSDT",
		Side:        "LONG",
		EntryPrice:  3.44184,
		ExitPrice:   3.39214,
		Quantity:    4,
		RealizedPnL: -0.22886952,
		Fee:         0.03006952,
		Leverage:    3,
		EntryTime:   entryTime,
		ExitTime:    exitTime,
		OrderID:     "trigger-stop-rave-1",
		ExchangeID:  "closed-pnl-trigger-tp-1",
		CloseType:   "unknown",
	}
	if _, err := root.Position().CreateFromClosedPnL(trader.ID, trader.ExchangeID, "bybit", record); err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "take_profit" {
		t.Fatalf("close reason = %q, want take_profit", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginSyncedTriggerOrder {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginSyncedTriggerOrder)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityHighConfidence {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityHighConfidence)
	}
	if !strings.Contains(items[0].Case.ExitEvidenceSummary, "stored take profit target") {
		t.Fatalf("exit evidence summary = %q, want stored take profit target evidence", items[0].Case.ExitEvidenceSummary)
	}
}

func TestSyncPositionReclassifiesManualExitWhenTriggerOrderArrives(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-trigger-reclassify.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-trigger-reclassify",
		UserID:         "user-trigger-reclassify",
		Name:           "Trigger Reclassify Trader",
		AIModelID:      "model-trigger-reclassify",
		ExchangeID:     "exchange-trigger-reclassify",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-40 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-5 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      771,
		Timestamp:        time.UnixMilli(entryTime).Add(-45 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "RAVEUSDT",
				Quantity:   4,
				Leverage:   3,
				Price:      3.44184,
				StopLoss:   3.035,
				TakeProfit: 3.395,
				Confidence: 72,
				Reasoning:  "trend_align, breakout follow-through",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "RAVEUSDT",
		Side:          "LONG",
		EntryQuantity: 4,
		Quantity:      4,
		EntryPrice:    3.44184,
		EntryOrderID:  "entry-rave-reclassify-1",
		EntryTime:     entryTime,
		Leverage:      3,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 3.30, "trigger-stop-rave-reclassify-1", exitTime, -0.56, 0.03, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases(initial) error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("initial total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "unknown" {
		t.Fatalf("initial close reason = %q, want unknown", items[0].Case.CloseReason)
	}

	triggerOrder := &TraderOrder{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		ExchangeOrderID: "trigger-stop-rave-reclassify-1",
		Symbol:          "RAVEUSDT",
		Side:            "SELL",
		PositionSide:    "LONG",
		Type:            "Stop",
		Status:          "FILLED",
		Quantity:        4,
		StopPrice:       3.395,
		AvgFillPrice:    3.30,
		FilledQuantity:  4,
		ReduceOnly:      true,
		OrderAction:     "close_long",
		CreatedAt:       exitTime,
		UpdatedAt:       exitTime,
		FilledAt:        exitTime,
	}
	if err := root.Order().UpsertOrder(triggerOrder); err != nil {
		t.Fatalf("Order().UpsertOrder() error = %v", err)
	}

	closedPosition, err := root.Position().GetByID(position.ID)
	if err != nil {
		t.Fatalf("Position().GetByID() error = %v", err)
	}
	if err := root.DealReview().SyncPosition(closedPosition); err != nil {
		t.Fatalf("DealReview().SyncPosition() error = %v", err)
	}

	items, _, total, err = root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases(resync) error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("resync total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "take_profit" {
		t.Fatalf("resynced close reason = %q, want take_profit", items[0].Case.CloseReason)
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.Close == nil || detail.Close.Event == nil {
		t.Fatal("expected close event detail to be present")
	}
	if detail.Close.Event.CloseReason != "take_profit" {
		t.Fatalf("close event reason = %q, want take_profit", detail.Close.Event.CloseReason)
	}

	refreshedPosition, err := root.Position().GetByID(position.ID)
	if err != nil {
		t.Fatalf("Position().GetByID(refresh) error = %v", err)
	}
	if refreshedPosition.CloseReason != "take_profit" {
		t.Fatalf("position close reason = %q, want take_profit", refreshedPosition.CloseReason)
	}
}

func TestSyncPositionClassifiesGenericTriggerOrderAsTrailingStopWhenStopHasTightened(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-trigger-unknown.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-trigger-unknown",
		UserID:         "user-trigger-unknown",
		Name:           "Trigger Unknown Trader",
		AIModelID:      "model-trigger-unknown",
		ExchangeID:     "exchange-trigger-unknown",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-50 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-4 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      901,
		Timestamp:        time.UnixMilli(entryTime).Add(-30 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "SPACEUSDT",
				Quantity:   5,
				Leverage:   3,
				Price:      3.4,
				StopLoss:   3.0,
				TakeProfit: 3.9,
				Confidence: 71,
				Reasoning:  "momentum continuation",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "SPACEUSDT",
		Side:          "LONG",
		EntryQuantity: 5,
		Quantity:      5,
		EntryPrice:    3.4,
		EntryOrderID:  "entry-space-1",
		EntryTime:     entryTime,
		Leverage:      3,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}
	if err := root.Position().ClosePositionWithAccurateData(position.ID, 3.22, "trigger-space-generic-1", exitTime, -0.8, 0.04, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	triggerOrder := &TraderOrder{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		ExchangeOrderID: "trigger-space-generic-1",
		ClientOrderID:   "space-stop-generic",
		Symbol:          "SPACEUSDT",
		Side:            "SELL",
		PositionSide:    "LONG",
		Type:            "Stop",
		VenueOrderType:  "Market",
		TriggerSubtype:  "Stop",
		TriggerSource:   "LastPrice",
		Status:          "FILLED",
		Quantity:        5,
		StopPrice:       3.22,
		AvgFillPrice:    3.22,
		FilledQuantity:  5,
		ReduceOnly:      true,
		OrderAction:     "close_long",
		CreatedAt:       exitTime,
		UpdatedAt:       exitTime,
		FilledAt:        exitTime,
	}
	if err := root.Order().UpsertOrder(triggerOrder); err != nil {
		t.Fatalf("Order().UpsertOrder() error = %v", err)
	}

	closedPosition, err := root.Position().GetByID(position.ID)
	if err != nil {
		t.Fatalf("Position().GetByID() error = %v", err)
	}
	if err := root.DealReview().SyncPosition(closedPosition); err != nil {
		t.Fatalf("DealReview().SyncPosition() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "trailing_stop" {
		t.Fatalf("close reason = %q, want trailing_stop", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginTrailingEngine {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginTrailingEngine)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityHighConfidence {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityHighConfidence)
	}
	if items[0].Case.ExitEvidence == nil {
		t.Fatal("expected exit evidence")
	}
	if items[0].Case.ExitEvidence.TriggerSubtype != "Stop" {
		t.Fatalf("trigger subtype = %q, want Stop", items[0].Case.ExitEvidence.TriggerSubtype)
	}
	if items[0].Case.ExitEvidence.TriggerSource != "LastPrice" {
		t.Fatalf("trigger source = %q, want LastPrice", items[0].Case.ExitEvidence.TriggerSource)
	}
	if items[0].Case.ExitEvidence.ExchangeOrderID != "trigger-space-generic-1" {
		t.Fatalf("exchange order id = %q, want trigger-space-generic-1", items[0].Case.ExitEvidence.ExchangeOrderID)
	}

	refreshedPosition, err := root.Position().GetByID(position.ID)
	if err != nil {
		t.Fatalf("Position().GetByID(refresh) error = %v", err)
	}
	if refreshedPosition.CloseReason != "trailing_stop" {
		t.Fatalf("position close reason = %q, want trailing_stop", refreshedPosition.CloseReason)
	}
}

func TestSyncPositionClassifiesGenericTriggerOrderAsStopLossOnLossSideOfEntry(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-trigger-stop-loss.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-trigger-stop-loss",
		UserID:         "user-trigger-stop-loss",
		Name:           "Trigger Stop Loss Trader",
		AIModelID:      "model-trigger-stop-loss",
		ExchangeID:     "exchange-trigger-stop-loss",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-50 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-4 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      902,
		Timestamp:        time.UnixMilli(entryTime).Add(-30 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "IPUSDT",
				Quantity:   5,
				Leverage:   3,
				Price:      3.4,
				StopLoss:   3.0,
				TakeProfit: 3.9,
				Confidence: 71,
				Reasoning:  "momentum continuation",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "IPUSDT",
		Side:          "LONG",
		EntryQuantity: 5,
		Quantity:      5,
		EntryPrice:    3.4,
		EntryOrderID:  "entry-ip-1",
		EntryTime:     entryTime,
		Leverage:      3,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}
	if err := root.Position().ClosePositionWithAccurateData(position.ID, 2.98, "trigger-ip-generic-1", exitTime, -0.9, 0.04, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	triggerOrder := &TraderOrder{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		ExchangeOrderID: "trigger-ip-generic-1",
		ClientOrderID:   "ip-stop-generic",
		Symbol:          "IPUSDT",
		Side:            "SELL",
		PositionSide:    "LONG",
		Type:            "Stop",
		VenueOrderType:  "Market",
		TriggerSubtype:  "Stop",
		TriggerSource:   "LastPrice",
		Status:          "FILLED",
		Quantity:        5,
		StopPrice:       2.98,
		AvgFillPrice:    2.98,
		FilledQuantity:  5,
		ReduceOnly:      true,
		OrderAction:     "close_long",
		CreatedAt:       exitTime,
		UpdatedAt:       exitTime,
		FilledAt:        exitTime,
	}
	if err := root.Order().UpsertOrder(triggerOrder); err != nil {
		t.Fatalf("Order().UpsertOrder() error = %v", err)
	}

	closedPosition, err := root.Position().GetByID(position.ID)
	if err != nil {
		t.Fatalf("Position().GetByID() error = %v", err)
	}
	if err := root.DealReview().SyncPosition(closedPosition); err != nil {
		t.Fatalf("DealReview().SyncPosition() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "stop_loss" {
		t.Fatalf("close reason = %q, want stop_loss", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginSyncedTriggerOrder {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginSyncedTriggerOrder)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityHighConfidence {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityHighConfidence)
	}
}

func TestSyncPositionFallsBackToExchangeSyncUnknownWithoutOrderOrFill(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-sync-unknown.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-sync-unknown",
		UserID:         "user-sync-unknown",
		Name:           "Sync Unknown Trader",
		AIModelID:      "model-sync-unknown",
		ExchangeID:     "exchange-sync-unknown",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-35 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-2 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      902,
		Timestamp:        time.UnixMilli(entryTime).Add(-15 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_short",
				Symbol:     "TONUSDT",
				Quantity:   7,
				Leverage:   2,
				Price:      1.55,
				StopLoss:   1.61,
				TakeProfit: 1.47,
				Confidence: 67,
				Reasoning:  "mean reversion fade",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "TONUSDT",
		Side:          "SHORT",
		EntryQuantity: 7,
		Quantity:      7,
		EntryPrice:    1.55,
		EntryOrderID:  "entry-ton-1",
		EntryTime:     entryTime,
		Leverage:      2,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}
	if err := root.Position().ClosePositionWithAccurateData(position.ID, 1.53, "missing-exit-order-1", exitTime, 0.25, 0.02, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "unknown" {
		t.Fatalf("close reason = %q, want unknown", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginExchangeSyncUnknown {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginExchangeSyncUnknown)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityLowConfidence {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityLowConfidence)
	}
	if !strings.Contains(items[0].Case.ExitEvidenceSummary, "no linked order") {
		t.Fatalf("exit evidence summary = %q, want exchange-sync-unknown summary", items[0].Case.ExitEvidenceSummary)
	}

	refreshedPosition, err := root.Position().GetByID(position.ID)
	if err != nil {
		t.Fatalf("Position().GetByID(refresh) error = %v", err)
	}
	if refreshedPosition.CloseReason != "unknown" {
		t.Fatalf("position close reason = %q, want unknown", refreshedPosition.CloseReason)
	}
}

func TestSyncPositionClassifiesTrailingStopFromPersistedTrailingUpdate(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-trailing-exit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-trailing-exit",
		UserID:         "user-trailing-exit",
		Name:           "Trailing Exit Trader",
		AIModelID:      "model-trailing-exit",
		ExchangeID:     "exchange-trailing-exit",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-40 * time.Minute).UnixMilli()
	trailingUpdateTime := time.Now().UTC().Add(-6 * time.Minute)
	exitTime := time.Now().UTC().Add(-5 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      881,
		Timestamp:        time.UnixMilli(entryTime).Add(-30 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "AVAXUSDT",
				Quantity:   2,
				Leverage:   4,
				Price:      20.0,
				StopLoss:   19.2,
				TakeProfit: 21.5,
				Confidence: 78,
				Reasoning:  "trend continuation setup",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "AVAXUSDT",
		Side:          "LONG",
		EntryQuantity: 2,
		Quantity:      2,
		EntryPrice:    20.0,
		EntryOrderID:  "entry-avax-trailing-1",
		EntryTime:     entryTime,
		Leverage:      4,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	if err := root.DealReview().RecordTrailingStopUpdate(&DealReviewTrailingUpdateInput{
		UserID:               trader.UserID,
		TraderID:             trader.ID,
		ExchangeID:           trader.ExchangeID,
		Symbol:               "AVAXUSDT",
		Side:                 "LONG",
		Timestamp:            trailingUpdateTime,
		PreviousStopPrice:    20.4,
		NewStopPrice:         20.95,
		TakeProfitPrice:      21.5,
		Quantity:             2,
		EntryPrice:           20.0,
		MarkPrice:            21.1,
		Leverage:             4,
		ProfitPct:            22,
		UnrealizedPnL:        2.2,
		UnrealizedPnLPct:     5.5,
		StopProfitPct:        4,
		ProtectsBreakeven:    true,
		TierIndex:            1,
		TierTriggerProfitPct: 10,
		TrailingMode:         TrailingStopModeLockProfit,
		LockProfitPct:        4,
	}); err != nil {
		t.Fatalf("RecordTrailingStopUpdate() error = %v", err)
	}

	triggerOrder := &TraderOrder{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		ExchangeOrderID: "exit-avax-trailing-1",
		Symbol:          "AVAXUSDT",
		Side:            "SELL",
		PositionSide:    "LONG",
		Type:            "Stop",
		Status:          "FILLED",
		Quantity:        2,
		StopPrice:       20.95,
		AvgFillPrice:    20.95,
		FilledQuantity:  2,
		ReduceOnly:      true,
		OrderAction:     "close_long",
		CreatedAt:       exitTime,
		UpdatedAt:       exitTime,
		FilledAt:        exitTime,
	}
	if err := root.Order().UpsertOrder(triggerOrder); err != nil {
		t.Fatalf("Order().UpsertOrder() error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 20.95, "exit-avax-trailing-1", exitTime, 1.9, 0.05, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "trailing_stop" {
		t.Fatalf("close reason = %q, want trailing_stop", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginTrailingEngine {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginTrailingEngine)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityExplicit {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityExplicit)
	}
	if !strings.Contains(items[0].Case.ExitEvidenceSummary, "Matched close against trailing-stop update to 20.95000000") {
		t.Fatalf("exit evidence summary = %q, want trailing update evidence", items[0].Case.ExitEvidenceSummary)
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.Close == nil || detail.Close.Event == nil {
		t.Fatal("expected close event detail to be present")
	}
	if detail.Close.Event.CloseReason != "trailing_stop" {
		t.Fatalf("close event reason = %q, want trailing_stop", detail.Close.Event.CloseReason)
	}
	if detail.Case.ExitEvidence == nil {
		t.Fatal("expected case exit evidence to be present")
	}
	if detail.Case.ExitEvidence.TrailingUpdateID == 0 {
		t.Fatalf("trailing update id = %d, want non-zero", detail.Case.ExitEvidence.TrailingUpdateID)
	}
	if detail.Case.ExitEvidence.PreviousStopPrice != 20.4 {
		t.Fatalf("previous stop price = %.2f, want 20.40", detail.Case.ExitEvidence.PreviousStopPrice)
	}
	if detail.Case.ExitEvidence.NewStopPrice != 20.95 {
		t.Fatalf("new stop price = %.2f, want 20.95", detail.Case.ExitEvidence.NewStopPrice)
	}
	if detail.Case.ExitEvidence.TrailingMode != TrailingStopModeLockProfit {
		t.Fatalf("trailing mode = %q, want %q", detail.Case.ExitEvidence.TrailingMode, TrailingStopModeLockProfit)
	}
	if detail.Case.ExitEvidence.TrailingUnrealizedPnL != 2.2 {
		t.Fatalf("trailing unrealized pnl = %.2f, want 2.20", detail.Case.ExitEvidence.TrailingUnrealizedPnL)
	}
	if detail.Case.ExitEvidence.TrailingUnrealizedPnLPct != 5.5 {
		t.Fatalf("trailing unrealized pnl pct = %.2f, want 5.50", detail.Case.ExitEvidence.TrailingUnrealizedPnLPct)
	}
	if detail.Case.ExitEvidence.TrailingStopProfitPct != 4 {
		t.Fatalf("trailing stop profit pct = %.2f, want 4.00", detail.Case.ExitEvidence.TrailingStopProfitPct)
	}
	if !detail.Case.ExitEvidence.TrailingProtectsBreakeven {
		t.Fatal("expected trailing exit evidence to report breakeven protection")
	}
	if detail.Close.Event.ExitEvidence == nil || detail.Close.Event.ExitEvidence.TrailingUpdatedAtMs != trailingUpdateTime.UnixMilli() {
		t.Fatalf("close event exit evidence = %#v, want trailing update timestamp %d", detail.Close.Event.ExitEvidence, trailingUpdateTime.UnixMilli())
	}

	refreshedPosition, err := root.Position().GetByID(position.ID)
	if err != nil {
		t.Fatalf("Position().GetByID() error = %v", err)
	}
	if refreshedPosition.CloseReason != "trailing_stop" {
		t.Fatalf("position close reason = %q, want trailing_stop", refreshedPosition.CloseReason)
	}
}

func TestSyncPositionClassifiesTrailingStopFromMovedStopOrderWithoutPersistedUpdate(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-moved-stop-exit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-moved-stop-exit",
		UserID:         "user-moved-stop-exit",
		Name:           "Moved Stop Exit Trader",
		AIModelID:      "model-moved-stop-exit",
		ExchangeID:     "exchange-moved-stop-exit",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-15 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-2 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      882,
		Timestamp:        time.UnixMilli(entryTime).Add(-20 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "SUIUSDT",
				Quantity:   50,
				Leverage:   3,
				Price:      1.0047,
				StopLoss:   0.9986,
				TakeProfit: 1.0147,
				Confidence: 74,
				Reasoning:  "continuation setup",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "SUIUSDT",
		Side:          "LONG",
		EntryQuantity: 50,
		Quantity:      50,
		EntryPrice:    1.0047,
		EntryOrderID:  "entry-sui-moved-stop-1",
		EntryTime:     entryTime,
		Leverage:      3,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	triggerOrder := &TraderOrder{
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeType:    "bybit",
		ExchangeOrderID: "exit-sui-moved-stop-1",
		Symbol:          "SUIUSDT",
		Side:            "SELL",
		PositionSide:    "LONG",
		Type:            "Stop",
		Status:          "FILLED",
		Quantity:        50,
		StopPrice:       1.0053,
		AvgFillPrice:    1.0052,
		FilledQuantity:  50,
		ReduceOnly:      true,
		OrderAction:     "close_long",
		CreatedAt:       exitTime,
		UpdatedAt:       exitTime,
		FilledAt:        exitTime,
	}
	if err := root.Order().UpsertOrder(triggerOrder); err != nil {
		t.Fatalf("Order().UpsertOrder() error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 1.0052, "exit-sui-moved-stop-1", exitTime, 0.03, 0.02, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "trailing_stop" {
		t.Fatalf("close reason = %q, want trailing_stop", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginTrailingEngine {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginTrailingEngine)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityHighConfidence {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityHighConfidence)
	}
	if !strings.Contains(items[0].Case.ExitEvidenceSummary, "stored stop moved favorably from 0.99860000") {
		t.Fatalf("exit evidence summary = %q, want moved-stop trailing evidence", items[0].Case.ExitEvidenceSummary)
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.Case.ExitEvidence == nil {
		t.Fatal("expected case exit evidence to be present")
	}
	if detail.Case.ExitEvidence.MatchedBy != "matched_moved_stop_order" {
		t.Fatalf("matched by = %q, want matched_moved_stop_order", detail.Case.ExitEvidence.MatchedBy)
	}
	if detail.Case.ExitEvidence.PreviousStopPrice != 0.9986 {
		t.Fatalf("previous stop price = %.4f, want 0.9986", detail.Case.ExitEvidence.PreviousStopPrice)
	}
	if detail.Case.ExitEvidence.NewStopPrice != 1.0053 {
		t.Fatalf("new stop price = %.4f, want 1.0053", detail.Case.ExitEvidence.NewStopPrice)
	}
	if detail.Case.ExitEvidence.TrailingUpdateID != 0 {
		t.Fatalf("trailing update id = %d, want 0 without persisted update", detail.Case.ExitEvidence.TrailingUpdateID)
	}

	refreshedPosition, err := root.Position().GetByID(position.ID)
	if err != nil {
		t.Fatalf("Position().GetByID() error = %v", err)
	}
	if refreshedPosition.CloseReason != "trailing_stop" {
		t.Fatalf("position close reason = %q, want trailing_stop", refreshedPosition.CloseReason)
	}
}

func TestSyncPositionUsesManualUICloseIntent(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-manual-ui-exit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-manual-ui-exit",
		UserID:         "user-manual-ui-exit",
		Name:           "Manual UI Exit Trader",
		AIModelID:      "model-manual-ui-exit",
		ExchangeID:     "exchange-manual-ui-exit",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-25 * time.Minute).UnixMilli()
	exitIntentTime := time.Now().UTC().Add(-4 * time.Second)
	exitTime := time.Now().UTC().Add(-2 * time.Second).UnixMilli()

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "OPUSDT",
		Side:          "LONG",
		EntryQuantity: 6,
		Quantity:      6,
		EntryPrice:    2.12,
		EntryOrderID:  "entry-op-ui-1",
		EntryTime:     entryTime,
		Leverage:      3,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	if err := root.DealReview().RecordExitIntent(&DealReviewExitIntentInput{
		UserID:          trader.UserID,
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeOrderID: "manual-ui-op-1",
		Symbol:          "OPUSDT",
		Side:            "LONG",
		Action:          "close_long",
		IntentType:      DealReviewExitIntentTypeManualUIClose,
		SourceModule:    "api.handleClosePosition",
		Summary:         "Matched manual UI close request.",
		Reasoning:       "User requested a direct close through the API/UI.",
		Quantity:        6,
		EntryPrice:      2.12,
		Timestamp:       exitIntentTime,
	}); err != nil {
		t.Fatalf("RecordExitIntent() error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 2.08, "manual-ui-op-1", exitTime, -0.72, 0.04, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "manual_exit" {
		t.Fatalf("close reason = %q, want manual_exit", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginManualUIClose {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginManualUIClose)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityExplicit {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityExplicit)
	}
	if !strings.Contains(items[0].Case.ExitEvidenceSummary, "Matched manual UI close request.") {
		t.Fatalf("exit evidence summary = %q, want manual UI summary", items[0].Case.ExitEvidenceSummary)
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.Case.ExitEvidence == nil {
		t.Fatal("expected case exit evidence")
	}
	if detail.Case.ExitEvidence.IntentType != DealReviewExitIntentTypeManualUIClose {
		t.Fatalf("intent type = %q, want %q", detail.Case.ExitEvidence.IntentType, DealReviewExitIntentTypeManualUIClose)
	}
	if detail.Case.ExitEvidence.IntentSourceModule != "api.handleClosePosition" {
		t.Fatalf("intent source module = %q", detail.Case.ExitEvidence.IntentSourceModule)
	}
	if detail.Case.ExitEvidence.IntentCreatedAtMs != exitIntentTime.UnixMilli() {
		t.Fatalf("intent created at = %d, want %d", detail.Case.ExitEvidence.IntentCreatedAtMs, exitIntentTime.UnixMilli())
	}
}

func TestSyncPositionUsesDrawdownGuardExitIntent(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-drawdown-exit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-drawdown-exit",
		UserID:         "user-drawdown-exit",
		Name:           "Drawdown Exit Trader",
		AIModelID:      "model-drawdown-exit",
		ExchangeID:     "exchange-drawdown-exit",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-50 * time.Minute).UnixMilli()
	exitIntentTime := time.Now().UTC().Add(-12 * time.Second)
	exitTime := time.Now().UTC().Add(-5 * time.Second).UnixMilli()

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "TIAUSDT",
		Side:          "SHORT",
		EntryQuantity: 3,
		Quantity:      3,
		EntryPrice:    8.4,
		EntryOrderID:  "entry-tia-risk-1",
		EntryTime:     entryTime,
		Leverage:      5,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	if err := root.DealReview().RecordExitIntent(&DealReviewExitIntentInput{
		UserID:          trader.UserID,
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeOrderID: "risk-drawdown-tia-1",
		Symbol:          "TIAUSDT",
		Side:            "SHORT",
		Action:          "close_short",
		IntentType:      DealReviewExitIntentTypeDrawdownGuard,
		SourceModule:    "trader.auto_trader_risk",
		Summary:         "Matched drawdown-guard exit at current profit 7.20% after peak 14.80% with 51.35% drawdown.",
		Reasoning:       "Matched drawdown-guard exit at current profit 7.20% after peak 14.80% with 51.35% drawdown.",
		Quantity:        3,
		EntryPrice:      8.4,
		Timestamp:       exitIntentTime,
	}); err != nil {
		t.Fatalf("RecordExitIntent() error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 8.28, "risk-drawdown-tia-1", exitTime, 1.8, 0.03, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "manual_exit" {
		t.Fatalf("close reason = %q, want manual_exit", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginRiskGuard {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginRiskGuard)
	}
	if items[0].Case.ExitReasonQuality != DealReviewExitReasonQualityExplicit {
		t.Fatalf("exit reason quality = %q, want %q", items[0].Case.ExitReasonQuality, DealReviewExitReasonQualityExplicit)
	}
	if !strings.Contains(items[0].Case.ExitEvidenceSummary, "drawdown-guard exit") {
		t.Fatalf("exit evidence summary = %q, want drawdown-guard summary", items[0].Case.ExitEvidenceSummary)
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.Case.ExitEvidence == nil {
		t.Fatal("expected case exit evidence")
	}
	if detail.Case.ExitEvidence.IntentType != DealReviewExitIntentTypeDrawdownGuard {
		t.Fatalf("intent type = %q, want %q", detail.Case.ExitEvidence.IntentType, DealReviewExitIntentTypeDrawdownGuard)
	}
	if detail.Case.ExitEvidence.IntentSourceModule != "trader.auto_trader_risk" {
		t.Fatalf("intent source module = %q", detail.Case.ExitEvidence.IntentSourceModule)
	}
}

func TestSyncPositionUsesGridDecisionExitIntent(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-grid-decision-exit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-grid-decision-exit",
		UserID:         "user-grid-decision-exit",
		Name:           "Grid Decision Exit Trader",
		AIModelID:      "model-grid-decision-exit",
		ExchangeID:     "exchange-grid-decision-exit",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-22 * time.Minute).UnixMilli()
	exitIntentTime := time.Now().UTC().Add(-4 * time.Minute)
	exitTime := time.Now().UTC().Add(-3 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      930,
		Timestamp:        time.UnixMilli(entryTime).Add(-20 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "DOGEUSDT",
				Quantity:   200,
				Leverage:   2,
				Price:      0.121,
				StopLoss:   0.118,
				TakeProfit: 0.127,
				Confidence: 68,
				Reasoning:  "grid accumulation",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "DOGEUSDT",
		Side:          "LONG",
		EntryQuantity: 200,
		Quantity:      200,
		EntryPrice:    0.121,
		EntryOrderID:  "entry-doge-grid-1",
		EntryTime:     entryTime,
		Leverage:      2,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	if err := root.DealReview().RecordExitIntent(&DealReviewExitIntentInput{
		UserID:          trader.UserID,
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeOrderID: "grid-ai-close-doge-1",
		Symbol:          "DOGEUSDT",
		Side:            "LONG",
		Action:          "close_long",
		IntentType:      DealReviewExitIntentTypeGridDecision,
		SourceModule:    "trader.auto_trader_grid",
		Summary:         "Matched grid AI close decision.",
		Reasoning:       "Reduce directional exposure after grid regime shifted bearish.",
		Quantity:        200,
		EntryPrice:      0.121,
		Timestamp:       exitIntentTime,
	}); err != nil {
		t.Fatalf("RecordExitIntent() error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 0.123, "grid-ai-close-doge-1", exitTime, 0.4, 0.02, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "ai_exit" {
		t.Fatalf("close reason = %q, want ai_exit", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginAIDecision {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginAIDecision)
	}
	if items[0].Case.ExitEvidence == nil || items[0].Case.ExitEvidence.IntentType != DealReviewExitIntentTypeGridDecision {
		t.Fatalf("exit evidence = %#v, want grid decision intent", items[0].Case.ExitEvidence)
	}
}

func TestSyncPositionUsesGridLevelStopLossIntent(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-grid-stoploss-exit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-grid-stoploss-exit",
		UserID:         "user-grid-stoploss-exit",
		Name:           "Grid Stop Loss Exit Trader",
		AIModelID:      "model-grid-stoploss-exit",
		ExchangeID:     "exchange-grid-stoploss-exit",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-28 * time.Minute).UnixMilli()
	exitIntentTime := time.Now().UTC().Add(-5 * time.Minute)
	exitTime := time.Now().UTC().Add(-4 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      931,
		Timestamp:        time.UnixMilli(entryTime).Add(-20 * time.Second).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_short",
				Symbol:     "TIAUSDT",
				Quantity:   4,
				Leverage:   3,
				Price:      8.4,
				StopLoss:   8.62,
				TakeProfit: 8.1,
				Confidence: 65,
				Reasoning:  "grid fade",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	position := &TraderPosition{
		TraderID:      trader.ID,
		ExchangeID:    trader.ExchangeID,
		ExchangeType:  "bybit",
		Symbol:        "TIAUSDT",
		Side:          "SHORT",
		EntryQuantity: 4,
		Quantity:      4,
		EntryPrice:    8.4,
		EntryOrderID:  "entry-tia-grid-1",
		EntryTime:     entryTime,
		Leverage:      3,
		CreatedAt:     entryTime,
		UpdatedAt:     entryTime,
	}
	if err := root.Position().Create(position); err != nil {
		t.Fatalf("Position().Create() error = %v", err)
	}

	if err := root.DealReview().RecordExitIntent(&DealReviewExitIntentInput{
		UserID:          trader.UserID,
		TraderID:        trader.ID,
		ExchangeID:      trader.ExchangeID,
		ExchangeOrderID: "grid-stoploss-tia-1",
		Symbol:          "TIAUSDT",
		Side:            "SHORT",
		Action:          "close_short",
		IntentType:      DealReviewExitIntentTypeGridLevelStop,
		SourceModule:    "trader.auto_trader_grid_orders",
		Summary:         "Matched grid level stop-loss for level 2 after 3.40% loss.",
		Reasoning:       "Grid stop-loss checker closed the short level after price exceeded the configured loss threshold.",
		Quantity:        4,
		EntryPrice:      8.4,
		Timestamp:       exitIntentTime,
	}); err != nil {
		t.Fatalf("RecordExitIntent() error = %v", err)
	}

	if err := root.Position().ClosePositionWithAccurateData(position.ID, 8.55, "grid-stoploss-tia-1", exitTime, -0.6, 0.03, "manual_exit"); err != nil {
		t.Fatalf("ClosePositionWithAccurateData() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].Case.CloseReason != "stop_loss" {
		t.Fatalf("close reason = %q, want stop_loss", items[0].Case.CloseReason)
	}
	if items[0].Case.ExitOrigin != DealReviewExitOriginRiskGuard {
		t.Fatalf("exit origin = %q, want %q", items[0].Case.ExitOrigin, DealReviewExitOriginRiskGuard)
	}
	if items[0].Case.ExitEvidence == nil || items[0].Case.ExitEvidence.IntentType != DealReviewExitIntentTypeGridLevelStop {
		t.Fatalf("exit evidence = %#v, want grid stop-loss intent", items[0].Case.ExitEvidence)
	}
}

func TestCreateFromClosedPnLInfersAIExitFromMatchedCloseDecision(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed-pnl-ai-exit.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "trader-ai-exit",
		UserID:         "user-ai-exit",
		Name:           "AI Exit Trader",
		AIModelID:      "model-ai-exit",
		ExchangeID:     "exchange-ai-exit",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	entryTime := time.Now().UTC().Add(-50 * time.Minute).UnixMilli()
	exitTime := time.Now().UTC().Add(-8 * time.Minute).UnixMilli()

	openRecord := &DecisionRecord{
		TraderID:         trader.ID,
		CycleNumber:      801,
		Timestamp:        time.UnixMilli(entryTime).Add(-1 * time.Minute).UTC(),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Decisions: []DecisionAction{
			{
				Action:     "open_long",
				Symbol:     "TONUSDT",
				Quantity:   16,
				Leverage:   5,
				Price:      1.4937,
				StopLoss:   1.44,
				TakeProfit: 1.58,
				Confidence: 83,
				Reasoning:  "momentum and breadth aligned",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(openRecord); err != nil {
		t.Fatalf("Decision().LogDecision(open) error = %v", err)
	}

	closeRecord := &DecisionRecord{
		TraderID:    trader.ID,
		CycleNumber: 802,
		Timestamp:   time.UnixMilli(exitTime).Add(-20 * time.Second).UTC(),
		Decisions: []DecisionAction{
			{
				Action:     "close_long",
				Symbol:     "TONUSDT",
				Price:      1.466,
				Confidence: 93,
				Reasoning:  "loss_cut, momentum_failure, manage_positions_first",
			},
		},
		Success: true,
	}
	if err := root.Decision().LogDecision(closeRecord); err != nil {
		t.Fatalf("Decision().LogDecision(close) error = %v", err)
	}

	record := &ClosedPnLRecord{
		Symbol:      "TONUSDT",
		Side:        "LONG",
		EntryPrice:  1.4937,
		ExitPrice:   1.466,
		Quantity:    16,
		RealizedPnL: -0.48527651,
		Fee:         0.02838651,
		Leverage:    5,
		EntryTime:   entryTime,
		ExitTime:    exitTime,
		OrderID:     "exit-ton-ai-1",
		ExchangeID:  "closed-pnl-ai-exit-1",
		CloseType:   "unknown",
	}
	if _, err := root.Position().CreateFromClosedPnL(trader.ID, trader.ExchangeID, "bybit", record); err != nil {
		t.Fatalf("CreateFromClosedPnL() error = %v", err)
	}

	items, _, total, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{TraderID: trader.ID})
	if err != nil {
		t.Fatalf("DealReview().ListCases() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}

	detail, err := root.DealReview().GetCaseDetail(trader.UserID, trader.ID, items[0].Case.ID)
	if err != nil {
		t.Fatalf("DealReview().GetCaseDetail() error = %v", err)
	}
	if detail.Case.CloseReason != "ai_exit" {
		t.Fatalf("close reason = %q, want ai_exit", detail.Case.CloseReason)
	}
	if detail.Case.ExitOrigin != DealReviewExitOriginAIDecision {
		t.Fatalf("exit origin = %q, want %q", detail.Case.ExitOrigin, DealReviewExitOriginAIDecision)
	}
	if detail.Case.ExitReasonQuality != DealReviewExitReasonQualityExplicit {
		t.Fatalf("exit reason quality = %q, want %q", detail.Case.ExitReasonQuality, DealReviewExitReasonQualityExplicit)
	}
	if !strings.Contains(detail.Case.ExitEvidenceSummary, "AI close decision cycle 802") {
		t.Fatalf("exit evidence summary = %q, want AI decision evidence", detail.Case.ExitEvidenceSummary)
	}
	if detail.Close == nil || detail.Close.Event == nil || detail.Close.Event.DecisionCycleNumber != 802 {
		t.Fatalf("close event = %#v, want decision cycle 802", detail.Close)
	}
	if detail.Close.Event.Reasoning != "loss_cut, momentum_failure, manage_positions_first" {
		t.Fatalf("close reasoning = %q", detail.Close.Event.Reasoning)
	}
	if detail.Close.Event.ExitEvidence == nil || detail.Close.Event.ExitEvidence.DecisionCycleNumber != 802 {
		t.Fatalf("close event exit evidence = %#v, want decision cycle 802", detail.Close.Event.ExitEvidence)
	}
}

func TestDealReviewBackfillQualityMetricsAndAnomalies(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-quality.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root := &Store{gdb: gdb, db: sqlDB}
	if err := root.initTables(); err != nil {
		t.Fatalf("initTables() error = %v", err)
	}

	trader := &Trader{
		ID:             "quality-trader",
		UserID:         "quality-user",
		Name:           "Quality Trader",
		AIModelID:      "model-1",
		ExchangeID:     "exchange-1",
		InitialBalance: 1000,
	}
	if err := root.Trader().Create(trader); err != nil {
		t.Fatalf("Trader().Create() error = %v", err)
	}

	now := time.Now().UTC()
	makeCase := func(id string, positionID int64, symbol string, realizedPnL float64, leverage int, stopLoss float64, entryOffset time.Duration, hold time.Duration, exitPrice float64, closeReason string) DealReviewCase {
		entryTime := now.Add(entryOffset).UnixMilli()
		exitTime := now.Add(entryOffset).Add(hold).UnixMilli()
		return DealReviewCase{
			ID:                  id,
			UserID:              trader.UserID,
			TraderID:            trader.ID,
			PositionID:          positionID,
			ExchangeID:          trader.ExchangeID,
			ExchangeType:        "bybit",
			AIModelID:           trader.AIModelID,
			StrategyID:          "strategy-1",
			Symbol:              symbol,
			Side:                "LONG",
			Status:              DealReviewCaseStatusClosed,
			Outcome:             classifyDealOutcome(realizedPnL),
			EntryTimeMs:         entryTime,
			ExitTimeMs:          exitTime,
			EntryPrice:          100,
			ExitPrice:           exitPrice,
			EntryQuantity:       1,
			ExitQuantity:        1,
			Leverage:            leverage,
			OpenStopLoss:        stopLoss,
			RealizedPnL:         realizedPnL,
			RealizedPnLPct:      calculateDealPnLPct(100, 1, leverage, realizedPnL),
			HoldDurationMs:      exitTime - entryTime,
			CloseReason:         closeReason,
			OpenSelectionBucket: "primary",
		}
	}

	cases := []DealReviewCase{
		makeCase("case-1", 1, "ETHUSDT", 1.5, 2, 99, 0, 30*time.Minute, 101.5, "take_profit"),
		makeCase("case-2", 2, "ETHUSDT", -1, 2, 99, 1*time.Hour, 30*time.Minute, 99, "manual_exit"),
		makeCase("case-3", 3, "SOLUSDT", 0.5, 1, 95, 2*time.Hour, 30*time.Minute, 100.5, "manual_exit"),
		makeCase("case-4", 4, "XRPUSDT", -1, 10, 99, 3*time.Hour, 5*time.Minute, 99, "stop_loss"),
	}
	for _, caseRec := range cases {
		if err := root.gdb.Create(&caseRec).Error; err != nil {
			t.Fatalf("Create(case %s) error = %v", caseRec.ID, err)
		}
	}

	entryTimes := map[int64]int64{}
	for _, caseRec := range cases {
		entryTimes[caseRec.PositionID] = caseRec.EntryTimeMs
	}
	makePoint := func(positionID int64, symbol string, offset time.Duration, markPrice, upnl float64) DealReviewMarketPointRecord {
		return DealReviewMarketPointRecord{
			UserID:           trader.UserID,
			TraderID:         trader.ID,
			PositionID:       positionID,
			Symbol:           symbol,
			Side:             "LONG",
			Source:           "platform",
			TimestampMs:      entryTimes[positionID] + offset.Milliseconds(),
			MarkPrice:        markPrice,
			EntryPrice:       100,
			Quantity:         1,
			UnrealizedPnL:    upnl,
			UnrealizedPnLPct: markPrice - 100,
			InProfit:         upnl > 0,
		}
	}

	points := []DealReviewMarketPointRecord{
		makePoint(1, "ETHUSDT", 5*time.Minute, 102, 2),
		makePoint(1, "ETHUSDT", 20*time.Minute, 101.2, 1.2),
		makePoint(2, "ETHUSDT", 5*time.Minute, 102, 2),
		makePoint(2, "ETHUSDT", 20*time.Minute, 99.5, -0.5),
		makePoint(3, "SOLUSDT", 5*time.Minute, 97, -3),
		makePoint(3, "SOLUSDT", 20*time.Minute, 100.6, 0.6),
		makePoint(4, "XRPUSDT", 2*time.Minute, 99.3, -0.7),
	}
	for _, point := range points {
		if err := root.gdb.Create(&point).Error; err != nil {
			t.Fatalf("Create(point %#v) error = %v", point, err)
		}
	}

	if err := root.DealReview().BackfillQualityMetrics(); err != nil {
		t.Fatalf("BackfillQualityMetrics() error = %v", err)
	}

	var avoidable DealReviewCase
	if err := root.gdb.Where("id = ?", "case-2").First(&avoidable).Error; err != nil {
		t.Fatalf("load case-2 error = %v", err)
	}
	if math.Abs(avoidable.MaxFavorableExcursion-2) > 0.0001 {
		t.Fatalf("case-2 max favorable = %v, want 2", avoidable.MaxFavorableExcursion)
	}
	if math.Abs(avoidable.ProfitGivenBack-3) > 0.0001 {
		t.Fatalf("case-2 profit given back = %v, want 3", avoidable.ProfitGivenBack)
	}
	if avoidable.TimeToFirstProfitMs <= 0 {
		t.Fatalf("case-2 time to first profit = %d, want positive", avoidable.TimeToFirstProfitMs)
	}
	if avoidable.ExitEfficiencyScore >= 35 {
		t.Fatalf("case-2 exit efficiency = %v, want weak exit score", avoidable.ExitEfficiencyScore)
	}

	_, summary, _, err := root.DealReview().ListCases(trader.UserID, DealReviewListFilter{
		TraderID: trader.ID,
		Status:   DealReviewCaseStatusClosed,
		Limit:    20,
	})
	if err != nil {
		t.Fatalf("ListCases() error = %v", err)
	}
	if summary.BadExitDeals == 0 {
		t.Fatalf("summary.BadExitDeals = %d, want > 0", summary.BadExitDeals)
	}
	if summary.AvoidableLossDeals == 0 {
		t.Fatalf("summary.AvoidableLossDeals = %d, want > 0", summary.AvoidableLossDeals)
	}
	if summary.StrongEntryWeakExitDeals == 0 {
		t.Fatalf("summary.StrongEntryWeakExitDeals = %d, want > 0", summary.StrongEntryWeakExitDeals)
	}
	if summary.WeakEntryLuckyExitDeals == 0 {
		t.Fatalf("summary.WeakEntryLuckyExitDeals = %d, want > 0", summary.WeakEntryLuckyExitDeals)
	}

	anomalies, err := root.DealReview().GetAnomalySummary(trader.UserID, DealReviewListFilter{
		TraderID: trader.ID,
		Status:   DealReviewCaseStatusClosed,
	})
	if err != nil {
		t.Fatalf("GetAnomalySummary() error = %v", err)
	}
	if len(anomalies.ProfitGiveBackHotspots) == 0 || anomalies.ProfitGiveBackHotspots[0].Symbol != "ETHUSDT" {
		t.Fatalf("profit give-back hotspots = %#v, want ETHUSDT hotspot", anomalies.ProfitGiveBackHotspots)
	}
	if len(anomalies.EarlyStopOutHotspots) == 0 || anomalies.EarlyStopOutHotspots[0].Symbol != "XRPUSDT" {
		t.Fatalf("early stop-out hotspots = %#v, want XRPUSDT hotspot", anomalies.EarlyStopOutHotspots)
	}
	if len(anomalies.OversizedLossHotspots) == 0 || anomalies.OversizedLossHotspots[0].Symbol != "XRPUSDT" {
		t.Fatalf("oversized loss hotspots = %#v, want XRPUSDT hotspot", anomalies.OversizedLossHotspots)
	}
	if len(anomalies.CloseReasonQuality) == 0 {
		t.Fatalf("close reason quality = %#v, want entries", anomalies.CloseReasonQuality)
	}
}
