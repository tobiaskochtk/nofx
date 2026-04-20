package store

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	dealReviewLearnedPatternLiveGuardRollupEventLimit = 12

	DealReviewLearnedPatternLiveGuardAttributionLabelProtective   = "protective"
	DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking = "overblocking"
	DealReviewLearnedPatternLiveGuardAttributionLabelMixed        = "mixed"
	DealReviewLearnedPatternLiveGuardAttributionLabelPending      = "pending"
)

type DealReviewLearnedPatternLiveGuardAttributionRollup struct {
	EventCount                 int       `json:"event_count,omitempty"`
	QualifiedEventCount        int       `json:"qualified_event_count,omitempty"`
	HardBlockedCount           int       `json:"hard_blocked_count,omitempty"`
	MonitorOnlyCount           int       `json:"monitor_only_count,omitempty"`
	MatchedUnqualifiedCount    int       `json:"matched_unqualified_count,omitempty"`
	ResolvedEventCount         int       `json:"resolved_event_count,omitempty"`
	ProtectiveEvidenceCount    int       `json:"protective_evidence_count,omitempty"`
	OverblockingEvidenceCount  int       `json:"overblocking_evidence_count,omitempty"`
	CorrectlyBlockedCount      int       `json:"correctly_blocked_count,omitempty"`
	OverblockedCount           int       `json:"overblocked_count,omitempty"`
	WarningConfirmedCount      int       `json:"warning_confirmed_count,omitempty"`
	WarningNotConfirmedCount   int       `json:"warning_not_confirmed_count,omitempty"`
	ThresholdMissedLossCount   int       `json:"threshold_missed_loss_count,omitempty"`
	ThresholdMissedProfitCount int       `json:"threshold_missed_profit_count,omitempty"`
	PendingCount               int       `json:"pending_count,omitempty"`
	FollowupOpenCount          int       `json:"followup_open_count,omitempty"`
	ProtectiveRate             float64   `json:"protective_rate,omitempty"`
	OverblockingRate           float64   `json:"overblocking_rate,omitempty"`
	ConfidenceScore            float64   `json:"confidence_score,omitempty"`
	AttributionLabel           string    `json:"attribution_label,omitempty"`
	LatestEventAt              time.Time `json:"latest_event_at,omitempty"`
	Summary                    string    `json:"summary,omitempty"`
}

func (s *DealReviewStore) hydrateLearnedPatternLiveGuardAttribution(
	userID string,
	traderID string,
	items []DealReviewLearnedPattern,
	limit int,
) error {
	return s.hydrateLearnedPatternLiveGuardAttributionWithDB(s.db, userID, traderID, items, limit)
}

func (s *DealReviewStore) hydrateLearnedPatternLiveGuardAttributionWithDB(
	db *gorm.DB,
	userID string,
	traderID string,
	items []DealReviewLearnedPattern,
	limit int,
) error {
	if len(items) == 0 {
		return nil
	}
	if limit <= 0 || limit > 24 {
		limit = dealReviewLearnedPatternLiveGuardRollupEventLimit
	}

	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" {
		return nil
	}

	refSet := collectDealReviewLearnedPatternLiveGuardReferenceSet(items)
	if refSet == nil {
		return nil
	}

	cacheByRef, err := s.loadLearnedPatternLiveGuardRollupCaches(
		userID,
		traderID,
		refSet.CanonicalRefs,
		limit,
	)
	if err != nil {
		return err
	}
	eventStats, err := s.loadLearnedPatternLiveGuardEventStats(
		userID,
		traderID,
		refSet.StableRefs,
		refSet.IDRefs,
		refSet.StableToCanonical,
		refSet.IDToCanonical,
	)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	staleCanonicals := make([]string, 0, len(items))
	noEventRefs := make([]string, 0)
	seenStale := make(map[string]struct{}, len(items))
	for idx := range items {
		canonicalRef := dealReviewLearnedPatternLiveGuardCanonicalRef(&items[idx])
		if canonicalRef == "" {
			continue
		}
		stats, hasStats := eventStats[canonicalRef]
		if !hasStats || stats.SourceEventCount <= 0 {
			items[idx].LiveGuardAttribution = nil
			items[idx].LiveGuardAttributionDelta = nil
			if _, ok := cacheByRef[canonicalRef]; ok {
				noEventRefs = append(noEventRefs, canonicalRef)
			}
			continue
		}

		if cached, ok := cacheByRef[canonicalRef]; ok &&
			isDealReviewLearnedPatternLiveGuardRollupCacheFresh(&cached, stats, now, limit) {
			items[idx].LiveGuardAttribution = unmarshalDealReviewLearnedPatternLiveGuardRollup(
				cached.RollupJSON,
			)
			items[idx].LiveGuardAttributionDelta = unmarshalDealReviewLearnedPatternLiveGuardDelta(
				cached.DeltaJSON,
			)
			if items[idx].LiveGuardAttribution != nil {
				continue
			}
		}

		if _, ok := seenStale[canonicalRef]; ok {
			continue
		}
		seenStale[canonicalRef] = struct{}{}
		staleCanonicals = append(staleCanonicals, canonicalRef)
	}

	if len(noEventRefs) > 0 {
		if err := s.deleteLearnedPatternLiveGuardRollupCachesTx(
			db,
			userID,
			traderID,
			dedupeDealReviewLearnedPatternRefs(noEventRefs),
			limit,
		); err != nil {
			return err
		}
	}
	if len(staleCanonicals) == 0 {
		return nil
	}

	recomputed, deltas, cacheRecords, err := s.recomputeLearnedPatternLiveGuardAttribution(
		userID,
		traderID,
		refSet,
		staleCanonicals,
		limit,
		eventStats,
		now,
	)
	if err != nil {
		return err
	}
	if len(cacheRecords) > 0 {
		if err := s.persistLearnedPatternLiveGuardRollupCachesTx(db, cacheRecords); err != nil {
			return err
		}
	}
	for idx := range items {
		canonicalRef := dealReviewLearnedPatternLiveGuardCanonicalRef(&items[idx])
		if canonicalRef == "" {
			continue
		}
		if rollup, ok := recomputed[canonicalRef]; ok {
			items[idx].LiveGuardAttribution = rollup
		}
		if delta, ok := deltas[canonicalRef]; ok {
			items[idx].LiveGuardAttributionDelta = delta
		}
	}
	return nil
}

func (s *DealReviewStore) recomputeLearnedPatternLiveGuardAttribution(
	userID string,
	traderID string,
	refSet *dealReviewLearnedPatternLiveGuardReferenceSet,
	canonicals []string,
	limit int,
	eventStats map[string]dealReviewLearnedPatternLiveGuardEventStats,
	now time.Time,
) (map[string]*DealReviewLearnedPatternLiveGuardAttributionRollup, map[string]*DealReviewLearnedPatternLiveGuardAttributionDelta, []DealReviewLearnedPatternLiveGuardRollupCache, error) {
	rollups := make(map[string]*DealReviewLearnedPatternLiveGuardAttributionRollup, len(canonicals))
	deltas := make(map[string]*DealReviewLearnedPatternLiveGuardAttributionDelta, len(canonicals))
	if refSet == nil || len(canonicals) == 0 {
		return rollups, deltas, nil, nil
	}

	stableRefs, idRefs := buildDealReviewLearnedPatternLiveGuardQueryRefs(refSet, canonicals)
	condition, args := buildDealReviewLearnedPatternLiveGuardReferenceCondition("e", stableRefs, idRefs)
	if condition == "" {
		return rollups, deltas, nil, nil
	}

	refExpr := "CASE WHEN e.matched_pattern_stable_key <> '' THEN e.matched_pattern_stable_key ELSE e.matched_pattern_id END"
	queryLimit := limit * 2
	if queryLimit < limit {
		queryLimit = limit
	}
	query := fmt.Sprintf(
		`SELECT * FROM (
			SELECT e.*, ROW_NUMBER() OVER (
				PARTITION BY %s
				ORDER BY e.decision_timestamp DESC, e.created_at DESC
			) AS rn
			FROM %s e
			WHERE e.user_id = ? AND e.trader_id = ? AND %s
		) ranked
		WHERE rn <= ?
		ORDER BY decision_timestamp DESC, created_at DESC`,
		refExpr,
		DealReviewLearnedPatternLiveGuardEvent{}.TableName(),
		condition,
	)

	params := []any{strings.TrimSpace(userID), strings.TrimSpace(traderID)}
	params = append(params, args...)
	params = append(params, queryLimit)

	var selected []DealReviewLearnedPatternLiveGuardEvent
	if err := s.db.Raw(query, params...).Scan(&selected).Error; err != nil {
		return nil, nil, nil, err
	}
	if len(selected) > 0 {
		if err := s.enrichLearnedPatternLiveGuardEventAttribution(userID, traderID, selected); err != nil {
			return nil, nil, nil, err
		}
	}

	byCanonical := make(map[string][]DealReviewLearnedPatternLiveGuardEvent, len(canonicals))
	for _, item := range selected {
		canonicalRef := dealReviewLearnedPatternLiveGuardCanonicalRefFromEvent(
			&item,
			refSet.StableToCanonical,
			refSet.IDToCanonical,
		)
		if canonicalRef == "" {
			continue
		}
		byCanonical[canonicalRef] = append(byCanonical[canonicalRef], item)
	}

	cacheRecords := make([]DealReviewLearnedPatternLiveGuardRollupCache, 0, len(canonicals))
	for _, canonicalRef := range canonicals {
		rollup := buildDealReviewLearnedPatternLiveGuardAttributionRollup(byCanonical[canonicalRef])
		delta := buildDealReviewLearnedPatternLiveGuardAttributionDelta(byCanonical[canonicalRef], limit)
		rollups[canonicalRef] = rollup
		deltas[canonicalRef] = delta
		if rollup == nil {
			continue
		}
		pattern, ok := refSet.PatternByCanonical[canonicalRef]
		if !ok {
			continue
		}
		record := buildDealReviewLearnedPatternLiveGuardRollupCacheRecord(
			&pattern,
			rollup,
			delta,
			eventStats[canonicalRef],
			limit,
			now,
		)
		if record != nil {
			cacheRecords = append(cacheRecords, *record)
		}
	}
	return rollups, deltas, cacheRecords, nil
}

func buildDealReviewLearnedPatternLiveGuardAttributionRollup(events []DealReviewLearnedPatternLiveGuardEvent) *DealReviewLearnedPatternLiveGuardAttributionRollup {
	if len(events) == 0 {
		return nil
	}

	ordered := append([]DealReviewLearnedPatternLiveGuardEvent(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := dealReviewLearnedPatternLiveGuardEventTime(&ordered[i])
		right := dealReviewLearnedPatternLiveGuardEventTime(&ordered[j])
		if left.Equal(right) {
			return ordered[i].ID > ordered[j].ID
		}
		return left.After(right)
	})

	rollup := &DealReviewLearnedPatternLiveGuardAttributionRollup{
		EventCount: len(ordered),
	}
	for _, event := range ordered {
		if eventTime := dealReviewLearnedPatternLiveGuardEventTime(&event); eventTime.After(rollup.LatestEventAt) {
			rollup.LatestEventAt = eventTime
		}
		switch strings.TrimSpace(event.Effect) {
		case DealReviewLearnedPatternLiveGuardEffectHardBlocked:
			rollup.QualifiedEventCount++
			rollup.HardBlockedCount++
		case DealReviewLearnedPatternLiveGuardEffectMonitorOnly:
			rollup.QualifiedEventCount++
			rollup.MonitorOnlyCount++
		case DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified:
			rollup.MatchedUnqualifiedCount++
		}
		if event.Attribution == nil {
			continue
		}
		switch strings.TrimSpace(event.Attribution.Status) {
		case DealReviewLearnedPatternLiveGuardAttributionStatusCorrectlyBlocked:
			rollup.CorrectlyBlockedCount++
		case DealReviewLearnedPatternLiveGuardAttributionStatusOverblocked:
			rollup.OverblockedCount++
		case DealReviewLearnedPatternLiveGuardAttributionStatusWarningConfirmed:
			rollup.WarningConfirmedCount++
		case DealReviewLearnedPatternLiveGuardAttributionStatusWarningNotConfirmed:
			rollup.WarningNotConfirmedCount++
		case DealReviewLearnedPatternLiveGuardAttributionStatusThresholdMissedLoss:
			rollup.ThresholdMissedLossCount++
		case DealReviewLearnedPatternLiveGuardAttributionStatusThresholdMissedProfit:
			rollup.ThresholdMissedProfitCount++
		case DealReviewLearnedPatternLiveGuardAttributionStatusPending:
			rollup.PendingCount++
		case DealReviewLearnedPatternLiveGuardAttributionStatusFollowupOpen:
			rollup.FollowupOpenCount++
		}
	}

	rollup.ProtectiveEvidenceCount = rollup.CorrectlyBlockedCount +
		rollup.WarningConfirmedCount +
		rollup.ThresholdMissedLossCount
	rollup.OverblockingEvidenceCount = rollup.OverblockedCount +
		rollup.WarningNotConfirmedCount +
		rollup.ThresholdMissedProfitCount
	rollup.ResolvedEventCount = rollup.ProtectiveEvidenceCount + rollup.OverblockingEvidenceCount
	if rollup.ResolvedEventCount > 0 {
		rollup.ProtectiveRate = clampDealReviewUnit(
			float64(rollup.ProtectiveEvidenceCount) / float64(rollup.ResolvedEventCount),
		)
		rollup.OverblockingRate = clampDealReviewUnit(
			float64(rollup.OverblockingEvidenceCount) / float64(rollup.ResolvedEventCount),
		)
		rollup.ConfidenceScore = clampDealReviewUnit(
			maxFloat(rollup.ProtectiveRate, rollup.OverblockingRate) *
				clampDealReviewUnit(float64(rollup.ResolvedEventCount)/4.0),
		)
	}

	switch {
	case rollup.ResolvedEventCount == 0:
		rollup.AttributionLabel = DealReviewLearnedPatternLiveGuardAttributionLabelPending
	case rollup.OverblockingEvidenceCount >= 2 && rollup.OverblockingRate >= 0.60:
		rollup.AttributionLabel = DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking
	case rollup.ProtectiveEvidenceCount >= 2 && rollup.ProtectiveRate >= 0.60:
		rollup.AttributionLabel = DealReviewLearnedPatternLiveGuardAttributionLabelProtective
	case rollup.PendingCount > 0 || rollup.FollowupOpenCount > 0:
		rollup.AttributionLabel = DealReviewLearnedPatternLiveGuardAttributionLabelPending
	default:
		rollup.AttributionLabel = DealReviewLearnedPatternLiveGuardAttributionLabelMixed
	}

	rollup.Summary = buildDealReviewLearnedPatternLiveGuardAttributionRollupSummary(rollup)
	return rollup
}

func buildDealReviewLearnedPatternLiveGuardAttributionRollupSummary(rollup *DealReviewLearnedPatternLiveGuardAttributionRollup) string {
	if rollup == nil {
		return ""
	}

	parts := make([]string, 0, 4)
	if rollup.ResolvedEventCount > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d resolved follow-up outcome(s)", rollup.ResolvedEventCount, rollup.EventCount))
		parts = append(parts, fmt.Sprintf("protective %.0f%%", rollup.ProtectiveRate*100))
		parts = append(parts, fmt.Sprintf("overblocking %.0f%%", rollup.OverblockingRate*100))
	} else {
		parts = append(parts, fmt.Sprintf("%d recent guard event(s) with no resolved follow-up yet", rollup.EventCount))
	}
	if rollup.PendingCount > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", rollup.PendingCount))
	}
	if rollup.FollowupOpenCount > 0 {
		parts = append(parts, fmt.Sprintf("%d follow-up open", rollup.FollowupOpenCount))
	}

	prefix := "Recent live-guard follow-up is mixed:"
	switch strings.TrimSpace(rollup.AttributionLabel) {
	case DealReviewLearnedPatternLiveGuardAttributionLabelProtective:
		prefix = "Recent live-guard follow-up is mostly protective:"
	case DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking:
		prefix = "Recent live-guard follow-up is trending overblocking:"
	case DealReviewLearnedPatternLiveGuardAttributionLabelPending:
		prefix = "Recent live-guard follow-up is still incomplete:"
	}
	return prefix + " " + strings.Join(parts, " · ") + "."
}

func dealReviewLearnedPatternLiveGuardCanonicalRef(pattern *DealReviewLearnedPattern) string {
	if pattern == nil {
		return ""
	}
	if stableKey := strings.TrimSpace(pattern.StableKey); stableKey != "" {
		return stableKey
	}
	return strings.TrimSpace(pattern.ID)
}

func dealReviewLearnedPatternLiveGuardCanonicalRefFromEvent(
	event *DealReviewLearnedPatternLiveGuardEvent,
	stableToCanonical map[string]string,
	idToCanonical map[string]string,
) string {
	if event == nil {
		return ""
	}
	if stableKey := strings.TrimSpace(event.MatchedPatternStableKey); stableKey != "" {
		if canonical := strings.TrimSpace(stableToCanonical[stableKey]); canonical != "" {
			return canonical
		}
	}
	if patternID := strings.TrimSpace(event.MatchedPatternID); patternID != "" {
		if canonical := strings.TrimSpace(idToCanonical[patternID]); canonical != "" {
			return canonical
		}
	}
	return ""
}

func normalizeDealReviewLearnedPatternLiveGuardAttributionLabel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternLiveGuardAttributionLabelProtective:
		return DealReviewLearnedPatternLiveGuardAttributionLabelProtective
	case DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking:
		return DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking
	case DealReviewLearnedPatternLiveGuardAttributionLabelMixed:
		return DealReviewLearnedPatternLiveGuardAttributionLabelMixed
	case DealReviewLearnedPatternLiveGuardAttributionLabelPending:
		return DealReviewLearnedPatternLiveGuardAttributionLabelPending
	default:
		return ""
	}
}
