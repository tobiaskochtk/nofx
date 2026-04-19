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
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitorOnly,
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
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitorOnly,
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
		RecommendedUse:         DealReviewLearnedPatternRecommendedUseMonitorOnly,
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
