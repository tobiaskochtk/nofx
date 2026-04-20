package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func TestEvaluateLearnedPatternLiveGuard_HardBlocksConfirmedNegativePattern(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	now := time.Now().UTC()

	pattern := DealReviewLearnedPattern{
		ID:                     "pattern-rave-block",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		ScopeType:              DealReviewLearnedPatternScopeSymbol,
		ScopeKey:               "symbol:RAVEUSDT:LONG",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		Status:                 "validated",
		ValidationLabel:        DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitoringRule,
		PatternSignature:       "bucket:breakout",
		FeatureCount:           1,
		PatternOrder:           1,
		FeatureSetJSON:         `["bucket:breakout"]`,
		SampleCount:            12,
		CompositeScore:         0.93,
		ConfidenceScore:        0.94,
		ValidationSupportScore: 0.88,
		FalsePositiveScore:     0.04,
		DriftScore:             0.05,
		EvidenceJSON:           "[]",
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-24 * time.Hour),
		BuiltAt:                now,
	}
	if err := root.gdb.Create(&pattern).Error; err != nil {
		t.Fatalf("Create(pattern) error = %v", err)
	}

	cfg := DefaultLearnedPatternLiveGuardConfig()
	cfg.Enabled = true
	cfg.Mode = LearnedPatternLiveGuardModeHardBlock

	assessment, err := root.DealReview().EvaluateLearnedPatternLiveGuard(
		"user-live-guard",
		"trader-live-guard",
		DealReviewLearnedPatternLiveGuardProbe{
			Action:          "open_long",
			Symbol:          "RAVEUSDT",
			Side:            "LONG",
			SelectionBucket: "breakout",
		},
		cfg,
	)
	if err != nil {
		t.Fatalf("EvaluateLearnedPatternLiveGuard() error = %v", err)
	}
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if !assessment.HardBlock {
		t.Fatalf("HardBlock = %v, want true", assessment.HardBlock)
	}
	if assessment.Effect != DealReviewLearnedPatternLiveGuardEffectHardBlocked {
		t.Fatalf("Effect = %q, want %q", assessment.Effect, DealReviewLearnedPatternLiveGuardEffectHardBlocked)
	}
	if assessment.MatchedPattern == nil || assessment.MatchedPattern.ID != pattern.ID {
		t.Fatalf("MatchedPattern = %#v, want %q", assessment.MatchedPattern, pattern.ID)
	}
	if assessment.MatchScore < cfg.MinMatchScore {
		t.Fatalf("MatchScore = %.4f, want >= %.4f", assessment.MatchScore, cfg.MinMatchScore)
	}
}

func TestEvaluateLearnedPatternLiveGuard_KeepsUnconfirmedPatternNonBlocking(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	now := time.Now().UTC()

	pattern := DealReviewLearnedPattern{
		ID:                     "pattern-rave-candidate",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		ScopeType:              DealReviewLearnedPatternScopeSymbol,
		ScopeKey:               "symbol:RAVEUSDT:LONG",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		Status:                 "observed",
		ValidationLabel:        DealReviewLearnedPatternValidationLabelCandidate,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitoringRule,
		PatternSignature:       "bucket:breakout",
		FeatureCount:           1,
		PatternOrder:           1,
		FeatureSetJSON:         `["bucket:breakout"]`,
		SampleCount:            20,
		CompositeScore:         0.95,
		ConfidenceScore:        0.96,
		ValidationSupportScore: 0.90,
		FalsePositiveScore:     0.03,
		DriftScore:             0.04,
		EvidenceJSON:           "[]",
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-48 * time.Hour),
		BuiltAt:                now,
	}
	if err := root.gdb.Create(&pattern).Error; err != nil {
		t.Fatalf("Create(pattern) error = %v", err)
	}

	cfg := DefaultLearnedPatternLiveGuardConfig()
	cfg.Enabled = true
	cfg.Mode = LearnedPatternLiveGuardModeHardBlock

	assessment, err := root.DealReview().EvaluateLearnedPatternLiveGuard(
		"user-live-guard",
		"trader-live-guard",
		DealReviewLearnedPatternLiveGuardProbe{
			Action:          "open_long",
			Symbol:          "RAVEUSDT",
			Side:            "LONG",
			SelectionBucket: "breakout",
		},
		cfg,
	)
	if err != nil {
		t.Fatalf("EvaluateLearnedPatternLiveGuard() error = %v", err)
	}
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if assessment.HardBlock {
		t.Fatalf("HardBlock = %v, want false", assessment.HardBlock)
	}
	if assessment.Effect != DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified {
		t.Fatalf("Effect = %q, want %q", assessment.Effect, DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified)
	}
	if !strings.Contains(assessment.Summary, "pattern_not_confirmed") {
		t.Fatalf("Summary = %q, want confirmation-threshold explanation", assessment.Summary)
	}
}

func TestEvaluateLearnedPatternLiveGuard_KeepsPromptHintPatternNonBlocking(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	now := time.Now().UTC()

	pattern := DealReviewLearnedPattern{
		ID:                     "pattern-rave-prompt-only",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		ScopeType:              DealReviewLearnedPatternScopeSymbol,
		ScopeKey:               "symbol:RAVEUSDT:LONG",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		Status:                 "validated",
		ValidationLabel:        DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUsePromptHint,
		PatternSignature:       "bucket:breakout + session:asia",
		FeatureCount:           2,
		PatternOrder:           2,
		FeatureSetJSON:         `["bucket:breakout","session:asia"]`,
		SampleCount:            7,
		CompositeScore:         0.83,
		ConfidenceScore:        0.88,
		ValidationSupportScore: 0.58,
		FalsePositiveScore:     0.04,
		DriftScore:             0.05,
		EvidenceJSON:           "[]",
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-24 * time.Hour),
		BuiltAt:                now,
	}
	if err := root.gdb.Create(&pattern).Error; err != nil {
		t.Fatalf("Create(pattern) error = %v", err)
	}

	cfg := DefaultLearnedPatternLiveGuardConfig()
	cfg.Enabled = true
	cfg.Mode = LearnedPatternLiveGuardModeHardBlock

	assessment, err := root.DealReview().EvaluateLearnedPatternLiveGuard(
		"user-live-guard",
		"trader-live-guard",
		DealReviewLearnedPatternLiveGuardProbe{
			Action:          "open_long",
			Symbol:          "RAVEUSDT",
			Side:            "LONG",
			SelectionBucket: "breakout",
			SessionBucket:   "asia",
		},
		cfg,
	)
	if err != nil {
		t.Fatalf("EvaluateLearnedPatternLiveGuard() error = %v", err)
	}
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if assessment.HardBlock {
		t.Fatalf("HardBlock = %v, want false", assessment.HardBlock)
	}
	if assessment.Effect != DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified {
		t.Fatalf("Effect = %q, want %q", assessment.Effect, DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified)
	}
	if !strings.Contains(assessment.Summary, "recommended_use!=monitoring_rule") {
		t.Fatalf("Summary = %q, want monitoring_rule gate explanation", assessment.Summary)
	}
}

func TestRecordLearnedPatternLiveGuardEvent_FromDecisionRecordProbe(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	record := &DecisionRecord{
		TraderID:         "trader-live-guard",
		CycleNumber:      77,
		Timestamp:        time.Now().UTC(),
		InputPrompt:      buildLearnedPatternLiveGuardPrompt(t, "RAVEUSDT"),
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		CandidateDetails: []CandidateDetail{
			{Symbol: "RAVEUSDT", SelectionBucket: "breakout"},
		},
	}
	action := &DecisionAction{
		Action:     "open_long",
		Symbol:     "RAVEUSDT",
		Confidence: 82,
		Timestamp:  record.Timestamp,
		Reasoning:  "trend_align, oi_rising",
	}
	probe := BuildDealReviewLearnedPatternLiveGuardProbeFromDecisionRecord(record, action)
	if probe == nil {
		t.Fatal("probe is nil")
	}

	now := time.Now().UTC()
	pattern := DealReviewLearnedPattern{
		ID:                     "pattern-rave-event",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		ScopeType:              DealReviewLearnedPatternScopeSymbol,
		ScopeKey:               "symbol:RAVEUSDT:LONG",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		PatternClass:           DealReviewLearnedPatternClassNegativeEdge,
		Status:                 "validated",
		ValidationLabel:        DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitoringRule,
		PatternSignature:       "bucket:breakout",
		FeatureCount:           1,
		PatternOrder:           1,
		FeatureSetJSON:         `["bucket:breakout"]`,
		SampleCount:            10,
		CompositeScore:         0.92,
		ConfidenceScore:        0.93,
		ValidationSupportScore: 0.86,
		FalsePositiveScore:     0.05,
		DriftScore:             0.04,
		EvidenceJSON:           "[]",
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-24 * time.Hour),
		BuiltAt:                now,
	}
	pattern.StableKey = dealReviewLearnedPatternStableKey(&pattern)
	if err := root.gdb.Create(&pattern).Error; err != nil {
		t.Fatalf("Create(pattern) error = %v", err)
	}

	cfg := DefaultLearnedPatternLiveGuardConfig()
	cfg.Enabled = true
	cfg.Mode = LearnedPatternLiveGuardModeHardBlock

	assessment, err := root.DealReview().EvaluateLearnedPatternLiveGuard(
		"user-live-guard",
		"trader-live-guard",
		*probe,
		cfg,
	)
	if err != nil {
		t.Fatalf("EvaluateLearnedPatternLiveGuard() error = %v", err)
	}
	if assessment == nil || !assessment.HardBlock {
		t.Fatalf("assessment = %#v, want hard block", assessment)
	}
	if err := root.DealReview().RecordLearnedPatternLiveGuardEvent(
		"user-live-guard",
		"trader-live-guard",
		probe,
		assessment,
	); err != nil {
		t.Fatalf("RecordLearnedPatternLiveGuardEvent() error = %v", err)
	}

	var events []DealReviewLearnedPatternLiveGuardEvent
	if err := root.gdb.Order("created_at DESC").Find(&events).Error; err != nil {
		t.Fatalf("Find(events) error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].Effect != DealReviewLearnedPatternLiveGuardEffectHardBlocked {
		t.Fatalf("event effect = %q, want %q", events[0].Effect, DealReviewLearnedPatternLiveGuardEffectHardBlocked)
	}
	if events[0].MatchedPatternID != pattern.ID {
		t.Fatalf("event matched pattern id = %q, want %q", events[0].MatchedPatternID, pattern.ID)
	}
	if strings.TrimSpace(events[0].MatchedPatternStableKey) == "" {
		t.Fatal("event matched pattern stable key is empty, want stable key persisted")
	}

	var snapshots []DealReviewLearnedPatternLifecycleSnapshot
	if err := root.gdb.Order("captured_at DESC").Find(&snapshots).Error; err != nil {
		t.Fatalf("Find(lifecycle snapshots) error = %v", err)
	}
	if len(snapshots) == 0 {
		t.Fatal("lifecycle snapshots len = 0, want >= 1")
	}
	if snapshots[0].PatternID != pattern.ID {
		t.Fatalf("snapshot pattern id = %q, want %q", snapshots[0].PatternID, pattern.ID)
	}
	if snapshots[0].PatternStableKey != pattern.StableKey {
		t.Fatalf("snapshot pattern stable key = %q, want %q", snapshots[0].PatternStableKey, pattern.StableKey)
	}
	if snapshots[0].SnapshotSource != dealReviewLearnedPatternLifecycleSnapshotSourceLiveGuardEvent {
		t.Fatalf(
			"snapshot source = %q, want %q",
			snapshots[0].SnapshotSource,
			dealReviewLearnedPatternLifecycleSnapshotSourceLiveGuardEvent,
		)
	}
}

func TestListLearnedPatternLiveGuardEvents_EnrichesResolvedAttribution(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	baseTime := time.Date(2026, time.April, 20, 10, 0, 0, 0, time.UTC)

	events := []DealReviewLearnedPatternLiveGuardEvent{
		{
			ID:                "evt-hard-overblocked",
			UserID:            "user-live-guard",
			TraderID:          "trader-live-guard",
			CycleNumber:       100,
			DecisionTimestamp: baseTime,
			Action:            "open_long",
			Symbol:            "RAVEUSDT",
			Side:              "LONG",
			Effect:            DealReviewLearnedPatternLiveGuardEffectHardBlocked,
			CreatedAt:         baseTime,
			UpdatedAt:         baseTime,
		},
		{
			ID:                "evt-monitor-loss",
			UserID:            "user-live-guard",
			TraderID:          "trader-live-guard",
			CycleNumber:       110,
			DecisionTimestamp: baseTime.Add(1 * time.Hour),
			Action:            "open_short",
			Symbol:            "TAOUSDT",
			Side:              "SHORT",
			Effect:            DealReviewLearnedPatternLiveGuardEffectMonitorOnly,
			CreatedAt:         baseTime.Add(1 * time.Hour),
			UpdatedAt:         baseTime.Add(1 * time.Hour),
		},
	}
	if err := root.gdb.Create(&events).Error; err != nil {
		t.Fatalf("Create(events) error = %v", err)
	}

	cases := []DealReviewCase{
		{
			ID:             "case-rave-followup",
			UserID:         "user-live-guard",
			TraderID:       "trader-live-guard",
			PositionID:     1001,
			Symbol:         "RAVEUSDT",
			Side:           "LONG",
			Status:         DealReviewCaseStatusClosed,
			Outcome:        "profit",
			EntryTimeMs:    baseTime.Add(30 * time.Minute).UnixMilli(),
			ExitTimeMs:     baseTime.Add(55 * time.Minute).UnixMilli(),
			RealizedPnL:    1.25,
			RealizedPnLPct: 2.50,
			CreatedAt:      baseTime.Add(30 * time.Minute),
			UpdatedAt:      baseTime.Add(55 * time.Minute),
		},
		{
			ID:             "case-tao-monitored",
			UserID:         "user-live-guard",
			TraderID:       "trader-live-guard",
			PositionID:     1002,
			Symbol:         "TAOUSDT",
			Side:           "SHORT",
			Status:         DealReviewCaseStatusClosed,
			Outcome:        "loss",
			EntryTimeMs:    baseTime.Add(1*time.Hour + 15*time.Second).UnixMilli(),
			ExitTimeMs:     baseTime.Add(1*time.Hour + 35*time.Minute).UnixMilli(),
			RealizedPnL:    -0.75,
			RealizedPnLPct: -1.20,
			CreatedAt:      baseTime.Add(1*time.Hour + 15*time.Second),
			UpdatedAt:      baseTime.Add(1*time.Hour + 35*time.Minute),
		},
	}
	if err := root.gdb.Create(&cases).Error; err != nil {
		t.Fatalf("Create(cases) error = %v", err)
	}

	items, err := root.DealReview().ListLearnedPatternLiveGuardEvents(
		"user-live-guard",
		"trader-live-guard",
		DealReviewLearnedPatternLiveGuardEventFilter{Limit: 20},
	)
	if err != nil {
		t.Fatalf("ListLearnedPatternLiveGuardEvents() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}

	byID := make(map[string]DealReviewLearnedPatternLiveGuardEvent, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}

	hardEvent := byID["evt-hard-overblocked"]
	if hardEvent.Attribution == nil {
		t.Fatal("hardEvent attribution is nil")
	}
	if hardEvent.Attribution.Status != DealReviewLearnedPatternLiveGuardAttributionStatusOverblocked {
		t.Fatalf(
			"hardEvent attribution status = %q, want %q",
			hardEvent.Attribution.Status,
			DealReviewLearnedPatternLiveGuardAttributionStatusOverblocked,
		)
	}
	if !hardEvent.Attribution.Resolved {
		t.Fatalf("hardEvent attribution resolved = %v, want true", hardEvent.Attribution.Resolved)
	}
	if hardEvent.Attribution.FollowupCaseID != "case-rave-followup" {
		t.Fatalf("hardEvent followup case id = %q, want case-rave-followup", hardEvent.Attribution.FollowupCaseID)
	}

	monitorEvent := byID["evt-monitor-loss"]
	if monitorEvent.Attribution == nil {
		t.Fatal("monitorEvent attribution is nil")
	}
	if monitorEvent.Attribution.Status != DealReviewLearnedPatternLiveGuardAttributionStatusWarningConfirmed {
		t.Fatalf(
			"monitorEvent attribution status = %q, want %q",
			monitorEvent.Attribution.Status,
			DealReviewLearnedPatternLiveGuardAttributionStatusWarningConfirmed,
		)
	}
	if monitorEvent.Attribution.FollowupCaseID != "case-tao-monitored" {
		t.Fatalf("monitorEvent followup case id = %q, want case-tao-monitored", monitorEvent.Attribution.FollowupCaseID)
	}

	summary := BuildDealReviewLearnedPatternLiveGuardEventSummary(items)
	if summary.OverblockedCount != 1 {
		t.Fatalf("summary overblocked = %d, want 1", summary.OverblockedCount)
	}
	if summary.WarningConfirmedCount != 1 {
		t.Fatalf("summary warning_confirmed = %d, want 1", summary.WarningConfirmedCount)
	}
}

func TestListLearnedPatternLiveGuardEvents_AnnotatesPendingAttributionWithNextAttempt(t *testing.T) {
	root := newLearnedPatternLiveGuardTestStore(t)
	baseTime := time.Date(2026, time.April, 20, 12, 0, 0, 0, time.UTC)

	event := DealReviewLearnedPatternLiveGuardEvent{
		ID:                "evt-hard-pending",
		UserID:            "user-live-guard",
		TraderID:          "trader-live-guard",
		CycleNumber:       220,
		DecisionTimestamp: baseTime,
		Action:            "open_long",
		Symbol:            "RAVEUSDT",
		Side:              "LONG",
		Effect:            DealReviewLearnedPatternLiveGuardEffectHardBlocked,
		CreatedAt:         baseTime,
		UpdatedAt:         baseTime,
	}
	if err := root.gdb.Create(&event).Error; err != nil {
		t.Fatalf("Create(event) error = %v", err)
	}

	record := &DecisionRecord{
		TraderID:    "trader-live-guard",
		CycleNumber: 221,
		Timestamp:   baseTime.Add(20 * time.Minute),
		Decisions: []DecisionAction{
			{
				Action:    "open_long",
				Symbol:    "RAVEUSDT",
				Timestamp: baseTime.Add(20 * time.Minute),
				Success:   false,
				Error:     "risk control paused",
			},
		},
	}
	if err := root.Decision().LogDecision(record); err != nil {
		t.Fatalf("LogDecision() error = %v", err)
	}

	items, err := root.DealReview().ListLearnedPatternLiveGuardEvents(
		"user-live-guard",
		"trader-live-guard",
		DealReviewLearnedPatternLiveGuardEventFilter{Limit: 20},
	)
	if err != nil {
		t.Fatalf("ListLearnedPatternLiveGuardEvents() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].Attribution == nil {
		t.Fatal("pending attribution is nil")
	}
	if items[0].Attribution.Status != DealReviewLearnedPatternLiveGuardAttributionStatusPending {
		t.Fatalf(
			"attribution status = %q, want %q",
			items[0].Attribution.Status,
			DealReviewLearnedPatternLiveGuardAttributionStatusPending,
		)
	}
	if items[0].Attribution.NextAttemptCycleNumber != 221 {
		t.Fatalf(
			"next attempt cycle = %d, want 221",
			items[0].Attribution.NextAttemptCycleNumber,
		)
	}
	if items[0].Attribution.NextAttemptTerminalStatus != "blocked_by_risk_control" {
		t.Fatalf(
			"next attempt terminal status = %q, want blocked_by_risk_control",
			items[0].Attribution.NextAttemptTerminalStatus,
		)
	}
	if !strings.Contains(items[0].Attribution.Summary, "blocked_by_risk_control") {
		t.Fatalf("attribution summary = %q, want blocked_by_risk_control", items[0].Attribution.Summary)
	}
}

func newLearnedPatternLiveGuardTestStore(t *testing.T) *Store {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "learned-pattern-live-guard.db"))
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
	return root
}

func buildLearnedPatternLiveGuardPrompt(t *testing.T, symbol string) string {
	t.Helper()
	payload := map[string]any{
		"candidates": []map[string]any{
			{
				"sym":     symbol,
				"px_mark": 1.23,
				"ctx": map[string]any{
					"tf":         "5m",
					"px_type":    "mark",
					"ema_fast":   1.10,
					"macd":       0.22,
					"rsi":        61.0,
					"oi_d1h_pct": 3.2,
				},
				"feat": map[string]any{
					"volatility": map[string]any{
						"regime": "high",
					},
				},
			},
		},
		"positions": []map[string]any{},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(raw)
}
