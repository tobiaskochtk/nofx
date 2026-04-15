package store

import (
	"database/sql"
	"encoding/json"
	"math"
	"path/filepath"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

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
	if !detail.PriceTimeline.Summary.EverInProfit {
		t.Fatal("expected EverInProfit to be true")
	}
	if detail.PriceTimeline.Summary.MaxUnrealizedPnL <= 0 {
		t.Fatalf("MaxUnrealizedPnL = %.4f, want > 0", detail.PriceTimeline.Summary.MaxUnrealizedPnL)
	}
	if detail.PriceTimeline.Summary.MinUnrealizedPnL >= 0 {
		t.Fatalf("MinUnrealizedPnL = %.4f, want < 0", detail.PriceTimeline.Summary.MinUnrealizedPnL)
	}
	if len(detail.PriceTimeline.Points) != 4 {
		t.Fatalf("len(PriceTimeline.Points) = %d, want 4", len(detail.PriceTimeline.Points))
	}
	if detail.PriceTimeline.Points[1].DecisionCycleNumber != 901 {
		t.Fatalf("first cycle point = %d, want 901", detail.PriceTimeline.Points[1].DecisionCycleNumber)
	}
	if detail.PriceTimeline.Points[2].DecisionCycleNumber != 902 {
		t.Fatalf("second cycle point = %d, want 902", detail.PriceTimeline.Points[2].DecisionCycleNumber)
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
			CloseReason:         "manual_exit",
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
	if len(summary.Notes) == 0 {
		t.Fatal("expected anomaly notes to be generated")
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
	version := &DealReviewStrategyVersion{
		UserID:             "user-versions",
		TraderID:           "trader-versions",
		StrategyID:         "strategy-versions",
		SourceScanID:       "scan-versions",
		SourceType:         "ai_apply",
		Summary:            "Tighten exposure after weak bucket scan",
		PreviousConfigJSON: string(prevJSON),
		NextConfigJSON:     string(nextJSON),
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
	if detail.Close == nil || detail.Close.Event == nil || detail.Close.Event.CloseReason != "manual_exit" {
		t.Fatalf("close event = %#v, want close_reason=manual_exit", detail.Close)
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
	if items[0].Case.CloseReason != "manual_exit" {
		t.Fatalf("initial close reason = %q, want manual_exit", items[0].Case.CloseReason)
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
	if detail.Close == nil || detail.Close.Event == nil || detail.Close.Event.DecisionCycleNumber != 802 {
		t.Fatalf("close event = %#v, want decision cycle 802", detail.Close)
	}
	if detail.Close.Event.Reasoning != "loss_cut, momentum_failure, manage_positions_first" {
		t.Fatalf("close reasoning = %q", detail.Close.Event.Reasoning)
	}
}
