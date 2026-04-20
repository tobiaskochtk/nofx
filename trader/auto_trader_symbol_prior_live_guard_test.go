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

func TestEnforceSymbolBehaviorLiveGuard_BlocksBeforeExecution(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "auto-trader-symbol-live-guard.db"))
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
	if err := gdb.AutoMigrate(&store.DealReviewSymbolBehaviorPrior{}, &store.DealReviewSymbolBehaviorLiveGuardEvent{}); err != nil {
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
		InputPrompt:      buildTraderSymbolBehaviorLiveGuardPrompt(t, "RAVEUSDT"),
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
	probe := store.BuildDealReviewSymbolBehaviorLiveGuardProbeFromDecisionRecord(record, action)
	if probe == nil {
		t.Fatal("probe is nil")
	}

	now := time.Now().UTC()
	prior := store.DealReviewSymbolBehaviorPrior{
		ID:                     "prior-rave-live",
		UserID:                 "user-live-guard",
		TraderID:               "trader-live-guard",
		Symbol:                 "RAVEUSDT",
		Side:                   "LONG",
		Status:                 store.DealReviewSymbolBehaviorPriorStatusValidated,
		BehaviorBias:           store.DealReviewSymbolBehaviorBiasNegative,
		OpenSelectionBucket:    probe.SelectionBucket,
		OpenTrendRegime:        probe.TrendRegime,
		OpenVolatilityRegime:   probe.VolatilityRegime,
		OpenOIRegime:           probe.OIRegime,
		SampleCount:            11,
		ConfidenceScore:        0.93,
		ContradictionScore:     0.91,
		DriftScore:             0.06,
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
	if err := gdb.Create(&prior).Error; err != nil {
		t.Fatalf("Create(prior) error = %v", err)
	}

	at := &AutoTrader{
		id:     "trader-live-guard",
		name:   "Live Guard Trader",
		userID: "user-live-guard",
		store:  root,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				RiskControl: store.RiskControlConfig{
					SymbolBehaviorLiveGuard: store.SymbolBehaviorLiveGuardConfig{
						Enabled:               true,
						Mode:                  store.SymbolBehaviorLiveGuardModeHardBlock,
						RequireConfirmedLabel: true,
						MinConfidenceScore:    0.90,
						MinSampleCount:        8,
						MinMatchScore:         0.75,
						MaxFalsePositiveScore: 0.15,
						MaxDriftScore:         0.20,
						MinContradictionScore: 0.80,
					},
				},
			},
		},
	}

	err = at.enforceSymbolBehaviorLiveGuard(record, action)
	if err == nil {
		t.Fatal("expected guard to block")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "symbol prior guard blocked") {
		t.Fatalf("error = %q, want symbol prior guard block", err.Error())
	}
	if !containsString(action.RejectReasons, "symbol_behavior_prior_block") {
		t.Fatalf("RejectReasons = %#v, want symbol_behavior_prior_block", action.RejectReasons)
	}
	if len(record.ExecutionLog) == 0 {
		t.Fatal("expected execution log entry")
	}
	if !strings.Contains(strings.ToLower(strings.Join(record.ExecutionLog, " ")), "symbol prior block") {
		t.Fatalf("ExecutionLog = %#v, want symbol prior block entry", record.ExecutionLog)
	}
}

func buildTraderSymbolBehaviorLiveGuardPrompt(t *testing.T, symbol string) string {
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

func containsString(items []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	for _, item := range items {
		if strings.TrimSpace(strings.ToLower(item)) == target {
			return true
		}
	}
	return false
}
