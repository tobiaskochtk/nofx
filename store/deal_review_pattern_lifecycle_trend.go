package store

import (
	"fmt"
	"strings"
	"time"
)

const dealReviewLearnedPatternLifecycleTrendLimit = 24

type DealReviewLearnedPatternLifecycleTrend struct {
	SnapshotCount              int       `json:"snapshot_count,omitempty"`
	StatusChangeCount          int       `json:"status_change_count,omitempty"`
	FirstCapturedAt            time.Time `json:"first_captured_at,omitempty"`
	LastCapturedAt             time.Time `json:"last_captured_at,omitempty"`
	LastStatusChangeAt         time.Time `json:"last_status_change_at,omitempty"`
	SpanHours                  float64   `json:"span_hours,omitempty"`
	LatestStatusDurationHours  float64   `json:"latest_status_duration_hours,omitempty"`
	ActiveHours                float64   `json:"active_hours,omitempty"`
	DegradingHours             float64   `json:"degrading_hours,omitempty"`
	RollbackWatchHours         float64   `json:"rollback_watch_hours,omitempty"`
	ExpiredHours               float64   `json:"expired_hours,omitempty"`
	ActiveShare                float64   `json:"active_share,omitempty"`
	DegradingShare             float64   `json:"degrading_share,omitempty"`
	RollbackWatchShare         float64   `json:"rollback_watch_share,omitempty"`
	ExpiredShare               float64   `json:"expired_share,omitempty"`
	StaleGuardSnapshotCount    int       `json:"stale_guard_snapshot_count,omitempty"`
	StaleGuardSnapshotShare    float64   `json:"stale_guard_snapshot_share,omitempty"`
	AvgObservedToGuardLagHours float64   `json:"avg_observed_to_guard_lag_hours,omitempty"`
	MaxObservedToGuardLagHours float64   `json:"max_observed_to_guard_lag_hours,omitempty"`
	Fragile                    bool      `json:"fragile,omitempty"`
	Summary                    string    `json:"summary,omitempty"`
}

func (s *DealReviewStore) hydrateLearnedPatternLifecycleHistory(
	userID string,
	traderID string,
	items []DealReviewLearnedPattern,
	limit int,
) error {
	if len(items) == 0 {
		return nil
	}
	if limit <= 0 || limit > 12 {
		limit = dealReviewLearnedPatternLifecycleHistoryLimit
	}

	refs := make([]string, 0, len(items))
	seenRefs := make(map[string]struct{}, len(items))
	for idx := range items {
		hydrateDealReviewLearnedPattern(&items[idx])
		ref := dealReviewLearnedPatternLifecycleReference(&items[idx])
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

	var rows []DealReviewLearnedPatternLifecycleSnapshot
	if err := s.db.Model(&DealReviewLearnedPatternLifecycleSnapshot{}).
		Where(
			"user_id = ? AND trader_id = ? AND ((pattern_stable_key <> '' AND pattern_stable_key IN ?) OR ((pattern_stable_key = '' OR pattern_stable_key IS NULL) AND pattern_id IN ?))",
			strings.TrimSpace(userID),
			strings.TrimSpace(traderID),
			refs,
			refs,
		).
		Order("captured_at DESC, created_at DESC").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	queryLimit := maxDealReviewPatternInt(limit, dealReviewLearnedPatternLifecycleTrendLimit)
	byRef := make(map[string][]DealReviewLearnedPatternLifecycleSnapshot, len(refs))
	for _, row := range rows {
		ref := strings.TrimSpace(row.PatternStableKey)
		if ref == "" {
			ref = strings.TrimSpace(row.PatternID)
		}
		if ref == "" || len(byRef[ref]) >= queryLimit {
			continue
		}
		byRef[ref] = append(byRef[ref], row)
	}

	for idx := range items {
		ref := dealReviewLearnedPatternLifecycleReference(&items[idx])
		if ref == "" {
			continue
		}
		history := byRef[ref]
		if len(history) == 0 {
			continue
		}
		if len(history) > 0 {
			visible := history
			if len(visible) > limit {
				visible = visible[:limit]
			}
			items[idx].LifecycleHistory = append(
				[]DealReviewLearnedPatternLifecycleSnapshot(nil),
				visible...,
			)
		}
		items[idx].LifecycleTrend = buildDealReviewLearnedPatternLifecycleTrend(history)
	}
	return nil
}

func buildDealReviewLearnedPatternLifecycleTrend(history []DealReviewLearnedPatternLifecycleSnapshot) *DealReviewLearnedPatternLifecycleTrend {
	if len(history) == 0 {
		return nil
	}

	ordered := make([]DealReviewLearnedPatternLifecycleSnapshot, 0, len(history))
	for idx := len(history) - 1; idx >= 0; idx-- {
		ordered = append(ordered, history[idx])
	}

	first := ordered[0]
	last := ordered[len(ordered)-1]
	trend := &DealReviewLearnedPatternLifecycleTrend{
		SnapshotCount:   len(ordered),
		FirstCapturedAt: first.CapturedAt,
		LastCapturedAt:  last.CapturedAt,
	}

	var totalLag float64
	var lagCount int
	for idx, snapshot := range ordered {
		if snapshot.StatusChanged {
			trend.StatusChangeCount++
			trend.LastStatusChangeAt = snapshot.CapturedAt
		}
		if snapshot.LastObservedToGuardLagHours > 0 {
			totalLag += snapshot.LastObservedToGuardLagHours
			lagCount++
			if snapshot.LastObservedToGuardLagHours > trend.MaxObservedToGuardLagHours {
				trend.MaxObservedToGuardLagHours = snapshot.LastObservedToGuardLagHours
			}
			if snapshot.RecentGuardEventCount > 0 &&
				snapshot.LastObservedToGuardLagHours >= dealReviewLearnedPatternLifecycleLagThreshold.Hours() {
				trend.StaleGuardSnapshotCount++
			}
		}

		if idx == len(ordered)-1 {
			continue
		}
		next := ordered[idx+1]
		segmentHours := next.CapturedAt.Sub(snapshot.CapturedAt).Hours()
		if segmentHours <= 0 {
			continue
		}
		switch normalizeDealReviewLearnedPatternLifecycleStatus(snapshot.LifecycleStatus) {
		case DealReviewLearnedPatternLifecycleStatusActive:
			trend.ActiveHours += segmentHours
		case DealReviewLearnedPatternLifecycleStatusDegrading:
			trend.DegradingHours += segmentHours
		case DealReviewLearnedPatternLifecycleStatusRollbackWatch:
			trend.RollbackWatchHours += segmentHours
		case DealReviewLearnedPatternLifecycleStatusExpired:
			trend.ExpiredHours += segmentHours
		}
	}

	trend.SpanHours = trend.LastCapturedAt.Sub(trend.FirstCapturedAt).Hours()
	if trend.SpanHours < 0 {
		trend.SpanHours = 0
	}
	if trend.SpanHours > 0 {
		trend.ActiveShare = clampDealReviewUnit(trend.ActiveHours / trend.SpanHours)
		trend.DegradingShare = clampDealReviewUnit(trend.DegradingHours / trend.SpanHours)
		trend.RollbackWatchShare = clampDealReviewUnit(trend.RollbackWatchHours / trend.SpanHours)
		trend.ExpiredShare = clampDealReviewUnit(trend.ExpiredHours / trend.SpanHours)
	}
	if lagCount > 0 {
		trend.AvgObservedToGuardLagHours = totalLag / float64(lagCount)
	}
	if trend.SnapshotCount > 0 {
		trend.StaleGuardSnapshotShare = clampDealReviewUnit(
			float64(trend.StaleGuardSnapshotCount) / float64(trend.SnapshotCount),
		)
	}
	if !trend.LastStatusChangeAt.IsZero() && !trend.LastCapturedAt.IsZero() &&
		!trend.LastCapturedAt.Before(trend.LastStatusChangeAt) {
		trend.LatestStatusDurationHours = trend.LastCapturedAt.Sub(trend.LastStatusChangeAt).Hours()
	} else if !trend.FirstCapturedAt.IsZero() && !trend.LastCapturedAt.IsZero() {
		trend.LatestStatusDurationHours = trend.LastCapturedAt.Sub(trend.FirstCapturedAt).Hours()
	}

	trend.Fragile = trend.RollbackWatchShare >= 0.35 ||
		(trend.DegradingShare+trend.RollbackWatchShare) >= 0.55 ||
		trend.StaleGuardSnapshotShare >= 0.40
	trend.Summary = buildDealReviewLearnedPatternLifecycleTrendSummary(trend, normalizeDealReviewLearnedPatternLifecycleStatus(last.LifecycleStatus))
	return trend
}

func buildDealReviewLearnedPatternLifecycleTrendSummary(trend *DealReviewLearnedPatternLifecycleTrend, latestStatus string) string {
	if trend == nil {
		return ""
	}
	parts := make([]string, 0, 6)
	if trend.SpanHours > 0 {
		parts = append(parts, fmt.Sprintf("window %.0fh", trend.SpanHours))
	}
	statusMix := make([]string, 0, 4)
	if trend.ActiveHours > 0 {
		statusMix = append(statusMix, fmt.Sprintf("active %.0fh", trend.ActiveHours))
	}
	if trend.DegradingHours > 0 {
		statusMix = append(statusMix, fmt.Sprintf("degrading %.0fh", trend.DegradingHours))
	}
	if trend.RollbackWatchHours > 0 {
		statusMix = append(statusMix, fmt.Sprintf("rollback %.0fh", trend.RollbackWatchHours))
	}
	if trend.ExpiredHours > 0 {
		statusMix = append(statusMix, fmt.Sprintf("expired %.0fh", trend.ExpiredHours))
	}
	if len(statusMix) > 0 {
		parts = append(parts, strings.Join(statusMix, ", "))
	}
	if trend.StatusChangeCount > 0 {
		parts = append(parts, fmt.Sprintf("%d status change(s)", trend.StatusChangeCount))
	}
	if trend.StaleGuardSnapshotCount > 0 {
		parts = append(parts, fmt.Sprintf("%d stale guard snapshot(s)", trend.StaleGuardSnapshotCount))
	}
	if trend.AvgObservedToGuardLagHours > 0 {
		parts = append(parts, fmt.Sprintf("avg lag %.0fh", trend.AvgObservedToGuardLagHours))
	}
	if trend.LatestStatusDurationHours > 0 && latestStatus != "" {
		parts = append(parts, fmt.Sprintf("latest %s for %.0fh", latestStatus, trend.LatestStatusDurationHours))
	}
	if len(parts) == 0 {
		return ""
	}
	prefix := "Lifecycle trend:"
	if trend.Fragile {
		prefix = "Lifecycle trend is fragile:"
	}
	return prefix + " " + strings.Join(parts, " · ") + "."
}
