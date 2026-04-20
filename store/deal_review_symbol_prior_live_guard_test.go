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

func TestEvaluateSymbolBehaviorLiveGuard_HardBlocksConfirmedNegativePrior(t *testing.T) {
	root := newSymbolBehaviorLiveGuardTestStore(t)
	now := time.Now().UTC()

	prior := DealReviewSymbolBehaviorPrior{
		ID:                     "prior-rave-block",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		Status:                 DealReviewSymbolBehaviorPriorStatusValidated,
		BehaviorBias:           DealReviewSymbolBehaviorBiasNegative,
		OpenSelectionBucket:    "breakout",
		OpenTrendRegime:        "uptrend",
		OpenVolatilityRegime:   "high",
		OpenOIRegime:           "rising",
		SampleCount:            12,
		ConfidenceScore:        0.94,
		ContradictionScore:     0.93,
		DriftScore:             0.08,
		ValidationSampleCount:  3,
		ValidationSupportCount: 3,
		RecentSampleCount:      2,
		RecentSupportCount:     2,
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-24 * time.Hour),
		BuiltAt:                now,
		SignalTagsJSON:         "[]",
		EvidenceJSON:           "[]",
		DecisionEvidenceJSON:   "[]",
	}
	if err := root.gdb.Create(&prior).Error; err != nil {
		t.Fatalf("Create(prior) error = %v", err)
	}

	cfg := DefaultSymbolBehaviorLiveGuardConfig()
	cfg.Enabled = true
	cfg.Mode = SymbolBehaviorLiveGuardModeHardBlock

	assessment, err := root.DealReview().EvaluateSymbolBehaviorLiveGuard(
		"user-live-guard",
		"trader-live-guard",
		DealReviewSymbolBehaviorLiveGuardProbe{
			Action:           "open_long",
			Symbol:           "RAVEUSDT",
			Side:             "LONG",
			SelectionBucket:  "breakout",
			TrendRegime:      "uptrend",
			VolatilityRegime: "high",
			OIRegime:         "rising",
		},
		cfg,
	)
	if err != nil {
		t.Fatalf("EvaluateSymbolBehaviorLiveGuard() error = %v", err)
	}
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if !assessment.HardBlock {
		t.Fatalf("HardBlock = %v, want true", assessment.HardBlock)
	}
	if assessment.Effect != DealReviewSymbolBehaviorLiveGuardEffectHardBlocked {
		t.Fatalf("Effect = %q, want %q", assessment.Effect, DealReviewSymbolBehaviorLiveGuardEffectHardBlocked)
	}
	if assessment.MatchedPrior == nil || assessment.MatchedPrior.ID != prior.ID {
		t.Fatalf("MatchedPrior = %#v, want %q", assessment.MatchedPrior, prior.ID)
	}
	if assessment.MatchScore < cfg.MinMatchScore {
		t.Fatalf("MatchScore = %.4f, want >= %.4f", assessment.MatchScore, cfg.MinMatchScore)
	}
}

func TestEvaluateSymbolBehaviorLiveGuard_KeepsWeakMatchNonBlocking(t *testing.T) {
	root := newSymbolBehaviorLiveGuardTestStore(t)
	now := time.Now().UTC()

	prior := DealReviewSymbolBehaviorPrior{
		ID:                     "prior-rave-weak-match",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		Status:                 DealReviewSymbolBehaviorPriorStatusValidated,
		BehaviorBias:           DealReviewSymbolBehaviorBiasNegative,
		OpenSelectionBucket:    "breakout",
		OpenTrendRegime:        "uptrend",
		OpenVolatilityRegime:   "high",
		OpenOIRegime:           "rising",
		SampleCount:            12,
		ConfidenceScore:        0.96,
		ContradictionScore:     0.95,
		DriftScore:             0.05,
		ValidationSampleCount:  3,
		ValidationSupportCount: 3,
		RecentSampleCount:      2,
		RecentSupportCount:     2,
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-24 * time.Hour),
		BuiltAt:                now,
		SignalTagsJSON:         "[]",
		EvidenceJSON:           "[]",
		DecisionEvidenceJSON:   "[]",
	}
	if err := root.gdb.Create(&prior).Error; err != nil {
		t.Fatalf("Create(prior) error = %v", err)
	}

	cfg := DefaultSymbolBehaviorLiveGuardConfig()
	cfg.Enabled = true
	cfg.Mode = SymbolBehaviorLiveGuardModeHardBlock

	assessment, err := root.DealReview().EvaluateSymbolBehaviorLiveGuard(
		"user-live-guard",
		"trader-live-guard",
		DealReviewSymbolBehaviorLiveGuardProbe{
			Action:           "open_long",
			Symbol:           "RAVEUSDT",
			Side:             "LONG",
			SelectionBucket:  "breakout",
			TrendRegime:      "downtrend",
			VolatilityRegime: "low",
			OIRegime:         "falling",
		},
		cfg,
	)
	if err != nil {
		t.Fatalf("EvaluateSymbolBehaviorLiveGuard() error = %v", err)
	}
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if assessment.HardBlock {
		t.Fatalf("HardBlock = %v, want false", assessment.HardBlock)
	}
	if assessment.Effect != DealReviewSymbolBehaviorLiveGuardEffectMatchedUnqualified {
		t.Fatalf("Effect = %q, want %q", assessment.Effect, DealReviewSymbolBehaviorLiveGuardEffectMatchedUnqualified)
	}
	if !strings.Contains(assessment.Summary, "match<") {
		t.Fatalf("Summary = %q, want match threshold explanation", assessment.Summary)
	}
}

func TestRecordSymbolBehaviorLiveGuardEvent_FromDecisionRecordProbe(t *testing.T) {
	root := newSymbolBehaviorLiveGuardTestStore(t)
	record := &DecisionRecord{
		TraderID:         "trader-live-guard",
		CycleNumber:      77,
		Timestamp:        time.Now().UTC(),
		InputPrompt:      buildSymbolBehaviorLiveGuardPrompt(t, "RAVEUSDT"),
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
	probe := BuildDealReviewSymbolBehaviorLiveGuardProbeFromDecisionRecord(record, action)
	if probe == nil {
		t.Fatal("probe is nil")
	}

	now := time.Now().UTC()
	prior := DealReviewSymbolBehaviorPrior{
		ID:                     "prior-rave-event",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		Status:                 DealReviewSymbolBehaviorPriorStatusValidated,
		BehaviorBias:           DealReviewSymbolBehaviorBiasNegative,
		OpenSelectionBucket:    probe.SelectionBucket,
		OpenTrendRegime:        probe.TrendRegime,
		OpenVolatilityRegime:   probe.VolatilityRegime,
		OpenOIRegime:           probe.OIRegime,
		SampleCount:            10,
		ConfidenceScore:        0.92,
		ContradictionScore:     0.90,
		DriftScore:             0.07,
		ValidationSampleCount:  3,
		ValidationSupportCount: 3,
		RecentSampleCount:      2,
		RecentSupportCount:     2,
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-24 * time.Hour),
		BuiltAt:                now,
		SignalTagsJSON:         "[]",
		EvidenceJSON:           "[]",
		DecisionEvidenceJSON:   "[]",
	}
	if err := root.gdb.Create(&prior).Error; err != nil {
		t.Fatalf("Create(prior) error = %v", err)
	}

	cfg := DefaultSymbolBehaviorLiveGuardConfig()
	cfg.Enabled = true
	cfg.Mode = SymbolBehaviorLiveGuardModeHardBlock

	assessment, err := root.DealReview().EvaluateSymbolBehaviorLiveGuard("user-live-guard", "trader-live-guard", *probe, cfg)
	if err != nil {
		t.Fatalf("EvaluateSymbolBehaviorLiveGuard() error = %v", err)
	}
	if assessment == nil || !assessment.HardBlock {
		t.Fatalf("assessment = %#v, want hard block", assessment)
	}
	if err := root.DealReview().RecordSymbolBehaviorLiveGuardEvent("user-live-guard", "trader-live-guard", probe, assessment); err != nil {
		t.Fatalf("RecordSymbolBehaviorLiveGuardEvent() error = %v", err)
	}

	var events []DealReviewSymbolBehaviorLiveGuardEvent
	if err := root.gdb.Order("created_at DESC").Find(&events).Error; err != nil {
		t.Fatalf("Find(events) error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].Effect != DealReviewSymbolBehaviorLiveGuardEffectHardBlocked {
		t.Fatalf("event effect = %q, want %q", events[0].Effect, DealReviewSymbolBehaviorLiveGuardEffectHardBlocked)
	}
	if events[0].MatchedPriorID != prior.ID {
		t.Fatalf("event matched prior id = %q, want %q", events[0].MatchedPriorID, prior.ID)
	}
}

func TestEvaluateSymbolBehaviorLiveGuard_MatureClusterTighteningKeepsBorderlinePriorNonBlocking(t *testing.T) {
	root := newSymbolBehaviorLiveGuardTestStore(t)
	now := time.Now().UTC()

	insertPrior := func(prior DealReviewSymbolBehaviorPrior) {
		t.Helper()
		if err := root.gdb.Create(&prior).Error; err != nil {
			t.Fatalf("Create(prior %s) error = %v", prior.ID, err)
		}
	}

	insertPrior(DealReviewSymbolBehaviorPrior{
		ID:                     "prior-rave-cluster-tighten",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		Status:                 DealReviewSymbolBehaviorPriorStatusValidated,
		BehaviorBias:           DealReviewSymbolBehaviorBiasNegative,
		ValidationLabel:        DealReviewSymbolBehaviorValidationLabelConfirmed,
		OpenSelectionBucket:    "breakout",
		OpenTrendRegime:        "uptrend",
		OpenVolatilityRegime:   "high",
		OpenOIRegime:           "rising",
		SampleCount:            10,
		ConfidenceScore:        0.91,
		ContradictionScore:     0.82,
		FalsePositiveScore:     0.14,
		DriftScore:             0.18,
		ValidationSampleCount:  3,
		ValidationSupportCount: 3,
		ValidationSupportScore: 0.70,
		RecentSampleCount:      2,
		RecentSupportCount:     2,
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-24 * time.Hour),
		BuiltAt:                now,
		SignalClusterKey:       "bucket_breakout",
		SignalClustersJSON:     `["bucket_breakout","src_ai500"]`,
		SignalTagsJSON:         "[]",
		EvidenceJSON:           "[]",
		DecisionEvidenceJSON:   "[]",
	})

	insertPrior(DealReviewSymbolBehaviorPrior{
		ID:                     "prior-ton-cluster-tighten",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		Symbol:                 "TONUSDT",
		Side:                   "LONG",
		Status:                 DealReviewSymbolBehaviorPriorStatusValidated,
		BehaviorBias:           DealReviewSymbolBehaviorBiasNegative,
		ValidationLabel:        DealReviewSymbolBehaviorValidationLabelFalsePositive,
		SampleCount:            7,
		DecisionOpenCount:      3,
		AvgPnLPct:              -1.2,
		ConfidenceScore:        0.95,
		ContradictionScore:     0.79,
		ValidationSampleCount:  3,
		ValidationSupportCount: 3,
		ValidationSupportScore: 0.67,
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-36 * time.Hour),
		BuiltAt:                now,
		SignalClusterKey:       "bucket_breakout",
		SignalClustersJSON:     `["bucket_breakout"]`,
		SignalTagsJSON:         "[]",
		EvidenceJSON:           "[]",
		DecisionEvidenceJSON:   "[]",
	})

	insertPrior(DealReviewSymbolBehaviorPrior{
		ID:                     "prior-link-cluster-tighten",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		Symbol:                 "LINKUSDT",
		Side:                   "LONG",
		Status:                 DealReviewSymbolBehaviorPriorStatusValidated,
		BehaviorBias:           DealReviewSymbolBehaviorBiasNegative,
		ValidationLabel:        DealReviewSymbolBehaviorValidationLabelConfirmed,
		SampleCount:            6,
		DecisionOpenCount:      2,
		AvgPnLPct:              -0.8,
		ConfidenceScore:        0.93,
		ContradictionScore:     0.74,
		ValidationSampleCount:  3,
		ValidationSupportCount: 3,
		ValidationSupportScore: 0.60,
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-48 * time.Hour),
		BuiltAt:                now,
		SignalClusterKey:       "bucket_breakout",
		SignalClustersJSON:     `["bucket_breakout"]`,
		SignalTagsJSON:         "[]",
		EvidenceJSON:           "[]",
		DecisionEvidenceJSON:   "[]",
	})

	cfg := DefaultSymbolBehaviorLiveGuardConfig()
	cfg.Enabled = true
	cfg.Mode = SymbolBehaviorLiveGuardModeHardBlock

	assessment, err := root.DealReview().EvaluateSymbolBehaviorLiveGuard(
		"user-live-guard",
		"trader-live-guard",
		DealReviewSymbolBehaviorLiveGuardProbe{
			Action:           "open_long",
			Symbol:           "RAVEUSDT",
			Side:             "LONG",
			SelectionBucket:  "breakout",
			TrendRegime:      "uptrend",
			VolatilityRegime: "high",
			OIRegime:         "rising",
		},
		cfg,
	)
	if err != nil {
		t.Fatalf("EvaluateSymbolBehaviorLiveGuard() error = %v", err)
	}
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if assessment.HardBlock {
		t.Fatalf("HardBlock = %v, want false once mature cluster tightening applies", assessment.HardBlock)
	}
	if assessment.Effect != DealReviewSymbolBehaviorLiveGuardEffectMatchedUnqualified {
		t.Fatalf("Effect = %q, want %q", assessment.Effect, DealReviewSymbolBehaviorLiveGuardEffectMatchedUnqualified)
	}
	if !strings.Contains(assessment.Summary, "Mature cluster tightening active") {
		t.Fatalf("Summary = %q, want cluster tightening note", assessment.Summary)
	}
	if !strings.Contains(assessment.Summary, "confidence<0.93") {
		t.Fatalf("Summary = %q, want tightened confidence threshold reason", assessment.Summary)
	}
}

func newSymbolBehaviorLiveGuardTestStore(t *testing.T) *Store {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "symbol-behavior-live-guard.db"))
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

func buildSymbolBehaviorLiveGuardPrompt(t *testing.T, symbol string) string {
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
