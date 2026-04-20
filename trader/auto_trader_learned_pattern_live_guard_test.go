package trader

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nofx/store"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func TestEnforceLearnedPatternLiveGuard_BlocksBeforeExecution(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "auto-trader-learned-pattern-live-guard.db"))
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
	if err := gdb.AutoMigrate(
		&store.DealReviewCase{},
		&store.DealReviewPatternFeatureRecord{},
		&store.DealReviewLearnedPattern{},
		&store.DealReviewLearnedPatternLiveGuardEvent{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	root, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("NewFromGorm() error = %v", err)
	}

	record := &store.DecisionRecord{
		TraderID:         "trader-live-guard",
		CycleNumber:      91,
		Timestamp:        time.Now().UTC(),
		CandidateMetaVer: store.DecisionCandidateMetadataVersion,
		InputPrompt:      buildTraderLearnedPatternLiveGuardPrompt(t, "RAVEUSDT"),
		CandidateDetails: []store.CandidateDetail{
			{Symbol: "RAVEUSDT", SelectionBucket: "breakout"},
		},
	}
	action := &store.DecisionAction{
		Action:     "open_long",
		Symbol:     "RAVEUSDT",
		Confidence: 84,
		Timestamp:  record.Timestamp,
		Reasoning:  "trend_align, oi_rising",
	}

	now := time.Now().UTC()
	pattern := store.DealReviewLearnedPattern{
		ID:                     "pattern-rave-live",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		ScopeType:              store.DealReviewLearnedPatternScopeSymbol,
		ScopeKey:               "symbol:RAVEUSDT:LONG",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		PatternClass:           store.DealReviewLearnedPatternClassNegativeEdge,
		Status:                 "validated",
		ValidationLabel:        store.DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:         store.DealReviewLearnedPatternRecommendedUseMonitorOnly,
		PatternSignature:       "bucket:breakout",
		FeatureCount:           1,
		PatternOrder:           1,
		FeatureSetJSON:         `["bucket:breakout"]`,
		SampleCount:            11,
		CompositeScore:         0.94,
		ConfidenceScore:        0.93,
		ValidationSupportScore: 0.87,
		FalsePositiveScore:     0.05,
		DriftScore:             0.06,
		EvidenceJSON:           "[]",
		LastObservedAt:         now,
		FirstObservedAt:        now.Add(-24 * time.Hour),
		BuiltAt:                now,
	}
	if err := gdb.Create(&pattern).Error; err != nil {
		t.Fatalf("Create(pattern) error = %v", err)
	}

	at := &AutoTrader{
		id:     "trader-live-guard",
		name:   "Live Guard Trader",
		userID: "user-live-guard",
		store:  root,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				RiskControl: store.RiskControlConfig{
					LearnedPatternLiveGuard: store.LearnedPatternLiveGuardConfig{
						Enabled:                   true,
						Mode:                      store.LearnedPatternLiveGuardModeHardBlock,
						RequireConfirmedLabel:     true,
						MinCompositeScore:         0.85,
						MinConfidenceScore:        0.90,
						MinSampleCount:            8,
						MinMatchScore:             0.80,
						MaxFalsePositiveScore:     0.10,
						MaxDriftScore:             0.15,
						MinValidationSupportScore: 0.60,
					},
				},
			},
		},
	}

	err = at.enforceLearnedPatternLiveGuard(record, action)
	if err == nil {
		t.Fatal("expected guard to block")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "learned-pattern guard blocked") {
		t.Fatalf("error = %q, want learned-pattern guard block", err.Error())
	}
	if !containsString(action.RejectReasons, "learned_pattern_block") {
		t.Fatalf("RejectReasons = %#v, want learned_pattern_block", action.RejectReasons)
	}
	if len(record.ExecutionLog) == 0 {
		t.Fatal("expected execution log entry")
	}
	if !strings.Contains(strings.ToLower(strings.Join(record.ExecutionLog, " ")), "learned pattern block") {
		t.Fatalf("ExecutionLog = %#v, want learned pattern block entry", record.ExecutionLog)
	}
}

func buildTraderLearnedPatternLiveGuardPrompt(t *testing.T, symbol string) string {
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
