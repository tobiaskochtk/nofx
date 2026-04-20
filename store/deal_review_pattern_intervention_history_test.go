package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func openDealReviewInterventionTestStore(t *testing.T) *Store {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "deal-review-pattern-interventions.db"))
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

func buildDealReviewInterventionTestPattern() DealReviewLearnedPattern {
	return DealReviewLearnedPattern{
		ID:               "pattern-intervention-1",
		StableKey:        "stable-pattern-intervention-1",
		UserID:           "user-pattern-intervention",
		TraderID:         "trader-pattern-intervention",
		ScopeType:        DealReviewLearnedPatternScopeTraderLocal,
		Symbol:           "RAVEUSDT",
		Side:             "LONG",
		PatternClass:     DealReviewLearnedPatternClassNegativeEdge,
		ValidationLabel:  DealReviewLearnedPatternValidationLabelConfirmed,
		RecommendedUse:   DealReviewLearnedPatternRecommendedUseMonitoringRule,
		PatternSignature: "bucket:breakout|trend:uptrend|vol:high|oi:flat",
		ActionHint: &DealReviewLearnedPatternActionHint{
			RecommendedAction: DealReviewLearnedPatternActionHintSuppress,
			PriorityLabel:     DealReviewLearnedPatternActionHintPriorityWarning,
			ReasonCode:        "delta_suppress_overblocking_pressure",
			ConfidenceScore:   0.74,
			Summary:           "Live-rule suppression is recommended after recent overblocking pressure.",
			AutoNote:          "Suggested suppress: recent live-guard follow-up worsened materially.",
		},
		LiveGuardAttributionDelta: &DealReviewLearnedPatternLiveGuardAttributionDelta{
			TrendLabel:               DealReviewLearnedPatternLiveGuardAttributionTrendDegrading,
			ConfidenceScore:          0.68,
			RecentResolvedEventCount: 3,
			PriorResolvedEventCount:  2,
			RecentOverblockingRate:   0.78,
			PriorOverblockingRate:    0.22,
			RecentProtectiveRate:     0.22,
			PriorProtectiveRate:      0.78,
		},
		Lifecycle: &DealReviewLearnedPatternLifecycle{
			Status:        DealReviewLearnedPatternLifecycleStatusRollbackWatch,
			RollbackScore: 0.72,
			ExpiryScore:   0.44,
			Summary:       "Rollback watch due to live-guard pressure.",
		},
	}
}

func TestSyncLearnedPatternSuggestedInterventions_DedupesAndSupersedes(t *testing.T) {
	root := openDealReviewInterventionTestStore(t)
	pattern := buildDealReviewInterventionTestPattern()

	if err := root.DealReview().syncLearnedPatternSuggestedInterventions(
		pattern.UserID,
		pattern.TraderID,
		[]DealReviewLearnedPattern{pattern},
	); err != nil {
		t.Fatalf("syncLearnedPatternSuggestedInterventions() error = %v", err)
	}

	var rows []DealReviewLearnedPatternIntervention
	if err := root.GormDB().
		Where("user_id = ? AND trader_id = ?", pattern.UserID, pattern.TraderID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		t.Fatalf("query interventions error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 intervention after first sync, got %d", len(rows))
	}
	if rows[0].EventStatus != DealReviewLearnedPatternInterventionStatusOpen {
		t.Fatalf("expected first intervention to remain open, got %q", rows[0].EventStatus)
	}
	if rows[0].SeenCount != 1 {
		t.Fatalf("expected first intervention seen_count=1, got %d", rows[0].SeenCount)
	}

	if err := root.DealReview().syncLearnedPatternSuggestedInterventions(
		pattern.UserID,
		pattern.TraderID,
		[]DealReviewLearnedPattern{pattern},
	); err != nil {
		t.Fatalf("second syncLearnedPatternSuggestedInterventions() error = %v", err)
	}

	rows = nil
	if err := root.GormDB().
		Where("user_id = ? AND trader_id = ?", pattern.UserID, pattern.TraderID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		t.Fatalf("query interventions after second sync error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected deduped single intervention after second sync, got %d", len(rows))
	}
	if rows[0].SeenCount != 2 {
		t.Fatalf("expected deduped suggestion seen_count=2, got %d", rows[0].SeenCount)
	}

	pattern.ActionHint = &DealReviewLearnedPatternActionHint{
		RecommendedAction: DealReviewLearnedPatternActionHintRetire,
		PriorityLabel:     DealReviewLearnedPatternActionHintPriorityCritical,
		ReasonCode:        "delta_retire_expired_or_extreme_overblocking",
		ConfidenceScore:   0.91,
		Summary:           "Live-rule retirement is recommended after newly overblocking follow-up.",
		AutoNote:          "Suggested retire: newly overblocking live-guard follow-up.",
	}
	pattern.LiveGuardAttributionDelta.TrendLabel = DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking
	pattern.LiveGuardAttributionDelta.ConfidenceScore = 0.89
	pattern.LiveGuardAttributionDelta.RecentResolvedEventCount = 4
	pattern.LiveGuardAttributionDelta.RecentOverblockingRate = 0.92
	pattern.Lifecycle.ExpiryScore = 0.88

	if err := root.DealReview().syncLearnedPatternSuggestedInterventions(
		pattern.UserID,
		pattern.TraderID,
		[]DealReviewLearnedPattern{pattern},
	); err != nil {
		t.Fatalf("third syncLearnedPatternSuggestedInterventions() error = %v", err)
	}

	rows = nil
	if err := root.GormDB().
		Where("user_id = ? AND trader_id = ?", pattern.UserID, pattern.TraderID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		t.Fatalf("query interventions after supersede error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 interventions after supersede, got %d", len(rows))
	}
	if rows[0].EventStatus != DealReviewLearnedPatternInterventionStatusSuperseded {
		t.Fatalf("expected first suggestion to become superseded, got %q", rows[0].EventStatus)
	}
	if rows[1].EventStatus != DealReviewLearnedPatternInterventionStatusOpen {
		t.Fatalf("expected latest suggestion to remain open, got %q", rows[1].EventStatus)
	}
	if rows[1].SuggestedAction != DealReviewLearnedPatternActionHintRetire {
		t.Fatalf("expected latest suggested action retire, got %q", rows[1].SuggestedAction)
	}
}

func TestRecordLearnedPatternManualInterventionTx_AcceptsSuggestion(t *testing.T) {
	root := openDealReviewInterventionTestStore(t)
	pattern := buildDealReviewInterventionTestPattern()

	if err := root.DealReview().syncLearnedPatternSuggestedInterventions(
		pattern.UserID,
		pattern.TraderID,
		[]DealReviewLearnedPattern{pattern},
	); err != nil {
		t.Fatalf("syncLearnedPatternSuggestedInterventions() error = %v", err)
	}

	if err := root.GormDB().Transaction(func(tx *gorm.DB) error {
		return root.DealReview().recordLearnedPatternManualInterventionTx(
			tx,
			pattern.UserID,
			pattern.TraderID,
			&pattern,
			DealReviewLearnedPatternManualControlActionSuppress,
			"Accepted suppress recommendation after review.",
			"manual-control-event-1",
		)
	}); err != nil {
		t.Fatalf("recordLearnedPatternManualInterventionTx() error = %v", err)
	}

	var rows []DealReviewLearnedPatternIntervention
	if err := root.GormDB().
		Where("user_id = ? AND trader_id = ?", pattern.UserID, pattern.TraderID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		t.Fatalf("query interventions error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected suggestion + manual action, got %d rows", len(rows))
	}

	var suggestion *DealReviewLearnedPatternIntervention
	var manual *DealReviewLearnedPatternIntervention
	for idx := range rows {
		switch rows[idx].EventType {
		case DealReviewLearnedPatternInterventionEventTypeSuggested:
			suggestion = &rows[idx]
		case DealReviewLearnedPatternInterventionEventTypeManualAction:
			manual = &rows[idx]
		}
	}
	if suggestion == nil || manual == nil {
		t.Fatalf("expected both suggestion and manual intervention rows, got suggestion=%v manual=%v", suggestion != nil, manual != nil)
	}
	if suggestion.EventStatus != DealReviewLearnedPatternInterventionStatusAccepted {
		t.Fatalf("expected suggestion status accepted, got %q", suggestion.EventStatus)
	}
	if suggestion.AppliedAction != DealReviewLearnedPatternManualControlActionSuppress {
		t.Fatalf("expected suggestion applied action suppress, got %q", suggestion.AppliedAction)
	}
	if suggestion.SourceManualControlEventID != "manual-control-event-1" {
		t.Fatalf("expected suggestion source_manual_control_event_id to be set, got %q", suggestion.SourceManualControlEventID)
	}
	if manual.EventStatus != DealReviewLearnedPatternInterventionStatusAccepted {
		t.Fatalf("expected manual intervention accepted, got %q", manual.EventStatus)
	}
	if !manual.AcceptedSuggestion {
		t.Fatalf("expected manual intervention to mark accepted_suggestion")
	}
	if manual.SuggestedAction != DealReviewLearnedPatternManualControlActionSuppress {
		t.Fatalf("expected manual row suggested_action suppress, got %q", manual.SuggestedAction)
	}
	if manual.AppliedAction != DealReviewLearnedPatternManualControlActionSuppress {
		t.Fatalf("expected manual row applied_action suppress, got %q", manual.AppliedAction)
	}
}

func TestSyncLearnedPatternSuggestedInterventions_PersistsDirectLiveActionCandidate(t *testing.T) {
	root := openDealReviewInterventionTestStore(t)
	pattern := buildDealReviewInterventionTestPattern()
	pattern.ActionHint = &DealReviewLearnedPatternActionHint{
		RecommendedAction: DealReviewLearnedPatternActionHintRetire,
		PriorityLabel:     DealReviewLearnedPatternActionHintPriorityCritical,
		ReasonCode:        "delta_retire_expired_or_extreme_overblocking",
		ConfidenceScore:   0.93,
		Summary:           "Retire this rule immediately.",
		AutoNote:          "Suggested retire after newly overblocking guard drift.",
	}
	pattern.LiveGuardAttributionDelta.TrendLabel = DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking
	pattern.LiveGuardAttributionDelta.ConfidenceScore = 0.89
	pattern.LiveGuardAttributionDelta.RecentResolvedEventCount = 4
	pattern.LiveGuardAttributionDelta.RecentOverblockingRate = 0.94
	pattern.LiveGuardAttributionDelta.RecentProtectiveRate = 0.06
	pattern.Lifecycle.Status = DealReviewLearnedPatternLifecycleStatusExpired
	pattern.Lifecycle.ExpiryScore = 0.91

	if err := root.DealReview().syncLearnedPatternSuggestedInterventions(
		pattern.UserID,
		pattern.TraderID,
		[]DealReviewLearnedPattern{pattern},
	); err != nil {
		t.Fatalf("syncLearnedPatternSuggestedInterventions() error = %v", err)
	}

	var rows []DealReviewLearnedPatternIntervention
	if err := root.GormDB().
		Where("user_id = ? AND trader_id = ?", pattern.UserID, pattern.TraderID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		t.Fatalf("query interventions error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 intervention, got %d", len(rows))
	}
	if !rows[0].DirectLiveActionCandidate {
		t.Fatal("expected direct_live_action_candidate=true")
	}
	if rows[0].DirectLiveActionKind != DealReviewLearnedPatternLiveActionKindRollback {
		t.Fatalf("DirectLiveActionKind = %q, want rollback", rows[0].DirectLiveActionKind)
	}
	if rows[0].DirectLiveActionSummary == "" {
		t.Fatal("expected direct live action summary to be populated")
	}
}

func TestRecordLearnedPatternManualInterventionTx_CarriesDirectLiveActionCandidate(t *testing.T) {
	root := openDealReviewInterventionTestStore(t)
	pattern := buildDealReviewInterventionTestPattern()
	pattern.ActionHint = &DealReviewLearnedPatternActionHint{
		RecommendedAction: DealReviewLearnedPatternActionHintRetire,
		PriorityLabel:     DealReviewLearnedPatternActionHintPriorityCritical,
		ReasonCode:        "delta_retire_expired_or_extreme_overblocking",
		ConfidenceScore:   0.93,
		Summary:           "Retire this rule immediately.",
		AutoNote:          "Suggested retire after newly overblocking guard drift.",
	}
	pattern.LiveGuardAttributionDelta.TrendLabel = DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking
	pattern.LiveGuardAttributionDelta.ConfidenceScore = 0.89
	pattern.LiveGuardAttributionDelta.RecentResolvedEventCount = 4
	pattern.LiveGuardAttributionDelta.RecentOverblockingRate = 0.94
	pattern.LiveGuardAttributionDelta.RecentProtectiveRate = 0.06
	pattern.Lifecycle.Status = DealReviewLearnedPatternLifecycleStatusExpired
	pattern.Lifecycle.ExpiryScore = 0.91

	if err := root.DealReview().syncLearnedPatternSuggestedInterventions(
		pattern.UserID,
		pattern.TraderID,
		[]DealReviewLearnedPattern{pattern},
	); err != nil {
		t.Fatalf("syncLearnedPatternSuggestedInterventions() error = %v", err)
	}

	if err := root.GormDB().Transaction(func(tx *gorm.DB) error {
		return root.DealReview().recordLearnedPatternManualInterventionTx(
			tx,
			pattern.UserID,
			pattern.TraderID,
			&pattern,
			DealReviewLearnedPatternManualControlActionRetire,
			"Accepted rollback-grade retire recommendation.",
			"manual-control-event-rollback",
		)
	}); err != nil {
		t.Fatalf("recordLearnedPatternManualInterventionTx() error = %v", err)
	}

	var manual DealReviewLearnedPatternIntervention
	if err := root.GormDB().
		Where("user_id = ? AND trader_id = ? AND event_type = ?", pattern.UserID, pattern.TraderID, DealReviewLearnedPatternInterventionEventTypeManualAction).
		First(&manual).Error; err != nil {
		t.Fatalf("query manual intervention error = %v", err)
	}
	if !manual.DirectLiveActionCandidate {
		t.Fatal("expected manual intervention to keep direct live action flag")
	}
	if manual.DirectLiveActionKind != DealReviewLearnedPatternLiveActionKindRollback {
		t.Fatalf("DirectLiveActionKind = %q, want rollback", manual.DirectLiveActionKind)
	}
	if manual.DirectLiveActionSummary == "" {
		t.Fatal("expected direct live action summary to be populated")
	}
}
