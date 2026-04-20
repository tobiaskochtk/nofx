package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	DealReviewLearnedPatternLifecycleStatusActive        = "active"
	DealReviewLearnedPatternLifecycleStatusDegrading     = "degrading"
	DealReviewLearnedPatternLifecycleStatusRollbackWatch = "rollback_watch"
	DealReviewLearnedPatternLifecycleStatusExpired       = "expired"

	dealReviewLearnedPatternLifecycleGuardWindow      = 7 * 24 * time.Hour
	dealReviewLearnedPatternLifecycleLagThreshold     = 72 * time.Hour
	dealReviewLearnedPatternLifecycleRecentSupportMin = 0.58
)

type DealReviewLearnedPatternLifecycle struct {
	Operational                   bool      `json:"operational"`
	Status                        string    `json:"status,omitempty"`
	Summary                       string    `json:"summary,omitempty"`
	ExpiryScore                   float64   `json:"expiry_score,omitempty"`
	RollbackScore                 float64   `json:"rollback_score,omitempty"`
	GuardWindowHours              int       `json:"guard_window_hours,omitempty"`
	RecentGuardEventCount         int       `json:"recent_guard_event_count,omitempty"`
	RecentQualifiedGuardCount     int       `json:"recent_qualified_guard_count,omitempty"`
	RecentHardBlockedCount        int       `json:"recent_hard_blocked_count,omitempty"`
	RecentMonitorOnlyCount        int       `json:"recent_monitor_only_count,omitempty"`
	RecentMatchedUnqualifiedCount int       `json:"recent_matched_unqualified_count,omitempty"`
	RecentGuardBlockRate          float64   `json:"recent_guard_block_rate,omitempty"`
	LastGuardEventAt              time.Time `json:"last_guard_event_at,omitempty"`
	LastObservedToGuardLagHours   float64   `json:"last_observed_to_guard_lag_hours,omitempty"`
}

type dealReviewLearnedPatternLiveGuardPatternStats struct {
	PatternRef                 string         `gorm:"column:pattern_ref"`
	TotalEvents                int            `gorm:"column:total_events"`
	HardBlockedCount           int            `gorm:"column:hard_blocked_count"`
	MonitorOnlyCount           int            `gorm:"column:monitor_only_count"`
	MatchedUnqualifiedCount    int            `gorm:"column:matched_unqualified_count"`
	LatestDecisionTimestampRaw sql.NullString `gorm:"column:latest_decision_timestamp"`
	LatestDecisionTimestamp    time.Time      `gorm:"-"`
}

func normalizeDealReviewLearnedPatternLifecycleStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternLifecycleStatusActive:
		return DealReviewLearnedPatternLifecycleStatusActive
	case DealReviewLearnedPatternLifecycleStatusDegrading:
		return DealReviewLearnedPatternLifecycleStatusDegrading
	case DealReviewLearnedPatternLifecycleStatusRollbackWatch:
		return DealReviewLearnedPatternLifecycleStatusRollbackWatch
	case DealReviewLearnedPatternLifecycleStatusExpired:
		return DealReviewLearnedPatternLifecycleStatusExpired
	default:
		return ""
	}
}

func (s *DealReviewStore) enrichLearnedPatternLifecycle(userID, traderID string, items []DealReviewLearnedPattern) error {
	return s.enrichLearnedPatternLifecycleWithDB(s.db, userID, traderID, items)
}

func (s *DealReviewStore) enrichLearnedPatternLifecycleWithDB(db *gorm.DB, userID, traderID string, items []DealReviewLearnedPattern) error {
	if len(items) == 0 {
		return nil
	}

	refs := make([]string, 0, len(items))
	seenRefs := make(map[string]struct{}, len(items))
	for idx := range items {
		hydrateDealReviewLearnedPattern(&items[idx])
		ref := strings.TrimSpace(items[idx].StableKey)
		if ref == "" {
			ref = strings.TrimSpace(items[idx].ID)
		}
		if ref == "" {
			continue
		}
		if _, ok := seenRefs[ref]; ok {
			continue
		}
		seenRefs[ref] = struct{}{}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return nil
	}

	now := time.Now().UTC()
	statsByPattern, err := s.aggregateLearnedPatternLiveGuardPatternStatsWithDB(
		db,
		strings.TrimSpace(userID),
		strings.TrimSpace(traderID),
		items,
		now.Add(-dealReviewLearnedPatternLifecycleGuardWindow),
	)
	if err != nil {
		return err
	}

	for idx := range items {
		ref := strings.TrimSpace(items[idx].StableKey)
		if ref == "" {
			ref = strings.TrimSpace(items[idx].ID)
		}
		items[idx].Lifecycle = buildDealReviewLearnedPatternLifecycle(
			&items[idx],
			statsByPattern[ref],
			now,
		)
	}
	return nil
}

func (s *DealReviewStore) aggregateLearnedPatternLiveGuardPatternStats(userID, traderID string, patternIDs []string, since time.Time) (map[string]dealReviewLearnedPatternLiveGuardPatternStats, error) {
	items := make([]DealReviewLearnedPattern, 0, len(patternIDs))
	for _, patternID := range patternIDs {
		if strings.TrimSpace(patternID) == "" {
			continue
		}
		items = append(items, DealReviewLearnedPattern{ID: patternID})
	}
	return s.aggregateLearnedPatternLiveGuardPatternStatsWithDB(s.db, userID, traderID, items, since)
}

func (s *DealReviewStore) aggregateLearnedPatternLiveGuardPatternStatsWithDB(db *gorm.DB, userID, traderID string, items []DealReviewLearnedPattern, since time.Time) (map[string]dealReviewLearnedPatternLiveGuardPatternStats, error) {
	out := make(map[string]dealReviewLearnedPatternLiveGuardPatternStats, len(items))
	if db == nil || len(items) == 0 || userID == "" || traderID == "" {
		return out, nil
	}

	stableKeys := make([]string, 0, len(items))
	patternIDs := make([]string, 0, len(items))
	seenStable := make(map[string]struct{}, len(items))
	seenIDs := make(map[string]struct{}, len(items))
	for idx := range items {
		stableKey := strings.TrimSpace(items[idx].StableKey)
		if stableKey != "" {
			if _, ok := seenStable[stableKey]; !ok {
				seenStable[stableKey] = struct{}{}
				stableKeys = append(stableKeys, stableKey)
			}
		}
		patternID := strings.TrimSpace(items[idx].ID)
		if patternID != "" {
			if _, ok := seenIDs[patternID]; !ok {
				seenIDs[patternID] = struct{}{}
				patternIDs = append(patternIDs, patternID)
			}
		}
	}
	if len(stableKeys) == 0 && len(patternIDs) == 0 {
		return out, nil
	}

	query := db.Model(&DealReviewLearnedPatternLiveGuardEvent{}).
		Where("user_id = ? AND trader_id = ?", userID, traderID).
		Select(`
			CASE
				WHEN matched_pattern_stable_key <> '' THEN matched_pattern_stable_key
				ELSE matched_pattern_id
			END AS pattern_ref,
			COUNT(*) AS total_events,
			SUM(CASE WHEN effect = 'hard_blocked' THEN 1 ELSE 0 END) AS hard_blocked_count,
			SUM(CASE WHEN effect = 'monitor_only' THEN 1 ELSE 0 END) AS monitor_only_count,
			SUM(CASE WHEN effect = 'matched_not_qualified' THEN 1 ELSE 0 END) AS matched_unqualified_count,
			CAST(MAX(decision_timestamp) AS TEXT) AS latest_decision_timestamp
		`).
		Group("pattern_ref")
	if !since.IsZero() {
		query = query.Where("decision_timestamp >= ?", since.UTC())
	}
	switch {
	case len(stableKeys) > 0 && len(patternIDs) > 0:
		query = query.Where(
			"((matched_pattern_stable_key <> '' AND matched_pattern_stable_key IN ?) OR ((matched_pattern_stable_key = '' OR matched_pattern_stable_key IS NULL) AND matched_pattern_id IN ?))",
			stableKeys,
			patternIDs,
		)
	case len(stableKeys) > 0:
		query = query.Where("matched_pattern_stable_key <> '' AND matched_pattern_stable_key IN ?", stableKeys)
	case len(patternIDs) > 0:
		query = query.Where("(matched_pattern_stable_key = '' OR matched_pattern_stable_key IS NULL) AND matched_pattern_id IN ?", patternIDs)
	}

	var rows []dealReviewLearnedPatternLiveGuardPatternStats
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.LatestDecisionTimestamp = parseDealReviewAggregateTime(row.LatestDecisionTimestampRaw.String)
		out[strings.TrimSpace(row.PatternRef)] = row
	}
	return out, nil
}

func buildDealReviewLearnedPatternLifecycle(pattern *DealReviewLearnedPattern, stats dealReviewLearnedPatternLiveGuardPatternStats, now time.Time) *DealReviewLearnedPatternLifecycle {
	if pattern == nil {
		return nil
	}

	recommendedUse := effectiveDealReviewLearnedPatternRecommendedUse(pattern)
	validationLabel := normalizeDealReviewLearnedPatternValidationLabel(pattern.ValidationLabel)
	operational := recommendedUse == DealReviewLearnedPatternRecommendedUseMonitoringRule
	currentExpired := recommendedUse == DealReviewLearnedPatternRecommendedUseExpiredIgnore || validationLabel == DealReviewLearnedPatternValidationLabelExpired
	hadLiveGuardActivity := stats.TotalEvents > 0
	if !operational && !currentExpired && !hadLiveGuardActivity {
		return nil
	}

	qualifiedGuardCount := stats.HardBlockedCount + stats.MonitorOnlyCount
	validationPressure := dealReviewLearnedPatternWeaknessBelowMin(
		pattern.ValidationSupportScore,
		DefaultLearnedPatternLiveGuardMinValidationSupportScore,
	)
	recentPressure := 0.0
	if pattern.RecentSampleCount <= 0 {
		recentPressure = 0.55
	} else {
		recentPressure = dealReviewLearnedPatternWeaknessBelowMin(
			pattern.RecentSupportScore,
			dealReviewLearnedPatternLifecycleRecentSupportMin,
		)
	}
	driftPressure := 0.0
	if DefaultLearnedPatternLiveGuardMaxDriftScore > 0 {
		driftPressure = clampDealReviewUnit(pattern.DriftScore / DefaultLearnedPatternLiveGuardMaxDriftScore)
	}
	recencyPressure := clampDealReviewUnit(1 - clampDealReviewUnit(pattern.RecencyWeight))
	lagHours := 0.0
	if !pattern.LastObservedAt.IsZero() &&
		!stats.LatestDecisionTimestamp.IsZero() &&
		stats.LatestDecisionTimestamp.After(pattern.LastObservedAt) {
		lagHours = stats.LatestDecisionTimestamp.Sub(pattern.LastObservedAt).Hours()
	}
	lagPressure := 0.0
	if lagHours > dealReviewLearnedPatternLifecycleLagThreshold.Hours() {
		lagPressure = clampDealReviewUnit(
			(lagHours - dealReviewLearnedPatternLifecycleLagThreshold.Hours()) / 120.0,
		)
	}

	expiryScore := clampDealReviewUnit(
		(validationPressure * 0.28) +
			(recentPressure * 0.24) +
			(driftPressure * 0.22) +
			(recencyPressure * 0.16) +
			(lagPressure * 0.10),
	)
	if validationLabel == DealReviewLearnedPatternValidationLabelDrifting {
		expiryScore = dealReviewPatternLifecycleMaxFloat(expiryScore, 0.60)
	}
	if currentExpired {
		expiryScore = dealReviewPatternLifecycleMaxFloat(expiryScore, 0.95)
	}

	rollbackScore := 0.0
	if qualifiedGuardCount > 0 || hadLiveGuardActivity {
		activityPressure := clampDealReviewUnit(float64(qualifiedGuardCount) / 4.0)
		staleLivePressure := dealReviewPatternLifecycleMaxFloat(expiryScore, validationPressure, recentPressure, lagPressure)
		rollbackScore = clampDealReviewUnit(
			(activityPressure * 0.45) +
				(staleLivePressure * 0.35) +
				(expiryScore * 0.20),
		)
		if !operational {
			rollbackScore = dealReviewPatternLifecycleMaxFloat(rollbackScore, 0.62)
		}
	}

	status := ""
	switch {
	case currentExpired:
		status = DealReviewLearnedPatternLifecycleStatusExpired
	case !operational && hadLiveGuardActivity:
		if rollbackScore >= 0.60 || qualifiedGuardCount >= 3 {
			status = DealReviewLearnedPatternLifecycleStatusRollbackWatch
		} else {
			status = DealReviewLearnedPatternLifecycleStatusDegrading
		}
	case operational && rollbackScore >= 0.72:
		status = DealReviewLearnedPatternLifecycleStatusRollbackWatch
	case operational && (expiryScore >= 0.55 ||
		validationLabel == DealReviewLearnedPatternValidationLabelDrifting ||
		validationLabel == DealReviewLearnedPatternValidationLabelFalsePositive ||
		validationLabel == DealReviewLearnedPatternValidationLabelReverseRisk):
		status = DealReviewLearnedPatternLifecycleStatusDegrading
	case operational:
		status = DealReviewLearnedPatternLifecycleStatusActive
	default:
		return nil
	}

	lifecycle := &DealReviewLearnedPatternLifecycle{
		Operational:                   operational,
		Status:                        status,
		ExpiryScore:                   expiryScore,
		RollbackScore:                 rollbackScore,
		GuardWindowHours:              int(dealReviewLearnedPatternLifecycleGuardWindow.Hours()),
		RecentGuardEventCount:         stats.TotalEvents,
		RecentQualifiedGuardCount:     qualifiedGuardCount,
		RecentHardBlockedCount:        stats.HardBlockedCount,
		RecentMonitorOnlyCount:        stats.MonitorOnlyCount,
		RecentMatchedUnqualifiedCount: stats.MatchedUnqualifiedCount,
		RecentGuardBlockRate:          aggRate(stats.HardBlockedCount, maxDealReviewPatternInt(1, qualifiedGuardCount)),
		LastGuardEventAt:              stats.LatestDecisionTimestamp,
		LastObservedToGuardLagHours:   lagHours,
	}
	lifecycle.Summary = buildDealReviewLearnedPatternLifecycleSummary(pattern, lifecycle)
	return lifecycle
}

func buildDealReviewLearnedPatternLifecycleSummary(pattern *DealReviewLearnedPattern, lifecycle *DealReviewLearnedPatternLifecycle) string {
	if pattern == nil || lifecycle == nil {
		return ""
	}
	recentSupport := fmt.Sprintf(
		"%d/%d recent support",
		pattern.RecentSupportCount,
		pattern.RecentSampleCount,
	)
	guardHits := fmt.Sprintf(
		"%d live-guard hits in the last %dh",
		lifecycle.RecentGuardEventCount,
		lifecycle.GuardWindowHours,
	)
	switch normalizeDealReviewLearnedPatternLifecycleStatus(lifecycle.Status) {
	case DealReviewLearnedPatternLifecycleStatusActive:
		return fmt.Sprintf(
			"Monitoring rule remains healthy: validation %.0f%%, %s, drift %.0f%%, %s.",
			pattern.ValidationSupportScore*100,
			recentSupport,
			pattern.DriftScore*100,
			guardHits,
		)
	case DealReviewLearnedPatternLifecycleStatusDegrading:
		return fmt.Sprintf(
			"Monitoring rule is degrading: expiry pressure %.0f%%, validation %.0f%%, %s, drift %.0f%%, %s.",
			lifecycle.ExpiryScore*100,
			pattern.ValidationSupportScore*100,
			recentSupport,
			pattern.DriftScore*100,
			guardHits,
		)
	case DealReviewLearnedPatternLifecycleStatusRollbackWatch:
		return fmt.Sprintf(
			"Rollback watch: %d qualified live-guard hit(s) in the last %dh while expiry pressure is %.0f%% and the last guard hit is %.0fh newer than the last observed evidence.",
			lifecycle.RecentQualifiedGuardCount,
			lifecycle.GuardWindowHours,
			lifecycle.ExpiryScore*100,
			lifecycle.LastObservedToGuardLagHours,
		)
	case DealReviewLearnedPatternLifecycleStatusExpired:
		return fmt.Sprintf(
			"Live use should be retired: expiry pressure %.0f%%, validation label %s, %s.",
			lifecycle.ExpiryScore*100,
			blankToValue(pattern.ValidationLabel, "unknown"),
			guardHits,
		)
	default:
		return ""
	}
}

func dealReviewLearnedPatternWeaknessBelowMin(value, min float64) float64 {
	if min <= 0 {
		return 0
	}
	if value >= min {
		return 0
	}
	return clampDealReviewUnit((min - value) / min)
}

func dealReviewPatternLifecycleMaxFloat(values ...float64) float64 {
	best := 0.0
	for _, value := range values {
		if value > best {
			best = value
		}
	}
	return best
}

func maxDealReviewPatternInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
