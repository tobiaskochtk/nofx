package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	dealReviewLearnedPatternLiveGuardRollupResolvedCacheTTL   = 30 * time.Minute
	dealReviewLearnedPatternLiveGuardRollupUnresolvedCacheTTL = 2 * time.Minute
)

type DealReviewLearnedPatternLiveGuardRollupCache struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           string    `gorm:"column:user_id;not null;index:idx_pattern_live_guard_rollup_cache_scope_ref_limit,unique;index:idx_pattern_live_guard_rollup_cache_scope" json:"user_id"`
	TraderID         string    `gorm:"column:trader_id;not null;index:idx_pattern_live_guard_rollup_cache_scope_ref_limit,unique;index:idx_pattern_live_guard_rollup_cache_scope" json:"trader_id"`
	PatternRef       string    `gorm:"column:pattern_ref;not null;index:idx_pattern_live_guard_rollup_cache_scope_ref_limit,unique;index:idx_pattern_live_guard_rollup_cache_ref" json:"pattern_ref"`
	PatternID        string    `gorm:"column:pattern_id;default:'';index:idx_pattern_live_guard_rollup_cache_pattern" json:"pattern_id,omitempty"`
	PatternStableKey string    `gorm:"column:pattern_stable_key;default:'';index:idx_pattern_live_guard_rollup_cache_pattern_stable" json:"pattern_stable_key,omitempty"`
	EventLimit       int       `gorm:"column:event_limit;default:0;index:idx_pattern_live_guard_rollup_cache_scope_ref_limit,unique" json:"event_limit"`
	SourceEventCount int       `gorm:"column:source_event_count;default:0" json:"source_event_count"`
	LatestEventAt    time.Time `gorm:"column:latest_event_at;index:idx_pattern_live_guard_rollup_cache_event" json:"latest_event_at,omitempty"`
	RefreshAfter     time.Time `gorm:"column:refresh_after;index:idx_pattern_live_guard_rollup_cache_refresh" json:"refresh_after,omitempty"`
	BuiltAt          time.Time `gorm:"column:built_at;index:idx_pattern_live_guard_rollup_cache_built" json:"built_at,omitempty"`
	RollupJSON       string    `gorm:"column:rollup_json;type:text;default:'{}'" json:"-"`
	DeltaJSON        string    `gorm:"column:delta_json;type:text;default:'{}'" json:"-"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (DealReviewLearnedPatternLiveGuardRollupCache) TableName() string {
	return "deal_review_pattern_live_guard_rollup_cache"
}

type dealReviewLearnedPatternLiveGuardEventStats struct {
	PatternRef       string
	SourceEventCount int
	LatestEventAt    time.Time
}

type dealReviewLearnedPatternLiveGuardReferenceSet struct {
	CanonicalRefs      []string
	StableRefs         []string
	IDRefs             []string
	StableToCanonical  map[string]string
	IDToCanonical      map[string]string
	PatternByCanonical map[string]DealReviewLearnedPattern
}

func collectDealReviewLearnedPatternLiveGuardReferenceSet(items []DealReviewLearnedPattern) *dealReviewLearnedPatternLiveGuardReferenceSet {
	if len(items) == 0 {
		return nil
	}
	refSet := &dealReviewLearnedPatternLiveGuardReferenceSet{
		CanonicalRefs:      make([]string, 0, len(items)),
		StableRefs:         make([]string, 0, len(items)),
		IDRefs:             make([]string, 0, len(items)),
		StableToCanonical:  make(map[string]string, len(items)),
		IDToCanonical:      make(map[string]string, len(items)),
		PatternByCanonical: make(map[string]DealReviewLearnedPattern, len(items)),
	}
	seenCanonical := make(map[string]struct{}, len(items))
	seenStable := make(map[string]struct{}, len(items))
	seenIDs := make(map[string]struct{}, len(items))
	for idx := range items {
		hydrateDealReviewLearnedPattern(&items[idx])
		canonicalRef := dealReviewLearnedPatternLiveGuardCanonicalRef(&items[idx])
		if canonicalRef == "" {
			continue
		}
		refSet.PatternByCanonical[canonicalRef] = items[idx]
		if _, ok := seenCanonical[canonicalRef]; !ok {
			seenCanonical[canonicalRef] = struct{}{}
			refSet.CanonicalRefs = append(refSet.CanonicalRefs, canonicalRef)
		}
		if stable := strings.TrimSpace(items[idx].StableKey); stable != "" {
			refSet.StableToCanonical[stable] = canonicalRef
			if _, ok := seenStable[stable]; !ok {
				seenStable[stable] = struct{}{}
				refSet.StableRefs = append(refSet.StableRefs, stable)
			}
		}
		if id := strings.TrimSpace(items[idx].ID); id != "" {
			refSet.IDToCanonical[id] = canonicalRef
			if _, ok := seenIDs[id]; !ok {
				seenIDs[id] = struct{}{}
				refSet.IDRefs = append(refSet.IDRefs, id)
			}
		}
	}
	if len(refSet.CanonicalRefs) == 0 {
		return nil
	}
	return refSet
}

func buildDealReviewLearnedPatternLiveGuardQueryRefs(
	refSet *dealReviewLearnedPatternLiveGuardReferenceSet,
	canonicals []string,
) ([]string, []string) {
	if refSet == nil || len(canonicals) == 0 {
		return nil, nil
	}
	stableRefs := make([]string, 0, len(canonicals))
	idRefs := make([]string, 0, len(canonicals))
	seenStable := make(map[string]struct{}, len(canonicals))
	seenIDs := make(map[string]struct{}, len(canonicals))
	for _, canonical := range canonicals {
		pattern, ok := refSet.PatternByCanonical[canonical]
		if !ok {
			continue
		}
		if stable := strings.TrimSpace(pattern.StableKey); stable != "" {
			if _, exists := seenStable[stable]; !exists {
				seenStable[stable] = struct{}{}
				stableRefs = append(stableRefs, stable)
			}
		}
		if id := strings.TrimSpace(pattern.ID); id != "" {
			if _, exists := seenIDs[id]; !exists {
				seenIDs[id] = struct{}{}
				idRefs = append(idRefs, id)
			}
		}
	}
	return stableRefs, idRefs
}

func marshalDealReviewLearnedPatternLiveGuardRollup(rollup *DealReviewLearnedPatternLiveGuardAttributionRollup) string {
	if rollup == nil {
		return "{}"
	}
	body, err := json.Marshal(rollup)
	if err != nil {
		return "{}"
	}
	return string(body)
}

func unmarshalDealReviewLearnedPatternLiveGuardRollup(raw string) *DealReviewLearnedPatternLiveGuardAttributionRollup {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return nil
	}
	var rollup DealReviewLearnedPatternLiveGuardAttributionRollup
	if err := json.Unmarshal([]byte(trimmed), &rollup); err != nil {
		return nil
	}
	if rollup.EventCount <= 0 && rollup.Summary == "" && rollup.AttributionLabel == "" {
		return nil
	}
	return &rollup
}

func marshalDealReviewLearnedPatternLiveGuardDelta(
	delta *DealReviewLearnedPatternLiveGuardAttributionDelta,
) string {
	if delta == nil {
		return "{}"
	}
	body, err := json.Marshal(delta)
	if err != nil {
		return "{}"
	}
	return string(body)
}

func unmarshalDealReviewLearnedPatternLiveGuardDelta(
	raw string,
) *DealReviewLearnedPatternLiveGuardAttributionDelta {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return nil
	}
	var delta DealReviewLearnedPatternLiveGuardAttributionDelta
	if err := json.Unmarshal([]byte(trimmed), &delta); err != nil {
		return nil
	}
	if delta.RecentEventCount <= 0 && delta.TrendLabel == "" && delta.Summary == "" {
		return nil
	}
	return &delta
}

func buildDealReviewLearnedPatternLiveGuardReferenceCondition(alias string, stableRefs, idRefs []string) (string, []any) {
	stableColumn := "matched_pattern_stable_key"
	idColumn := "matched_pattern_id"
	if alias != "" {
		stableColumn = alias + "." + stableColumn
		idColumn = alias + "." + idColumn
	}
	switch {
	case len(stableRefs) > 0 && len(idRefs) > 0:
		return fmt.Sprintf("(%s IN ? OR %s IN ?)", stableColumn, idColumn), []any{stableRefs, idRefs}
	case len(stableRefs) > 0:
		return fmt.Sprintf("%s IN ?", stableColumn), []any{stableRefs}
	case len(idRefs) > 0:
		return fmt.Sprintf("%s IN ?", idColumn), []any{idRefs}
	default:
		return "", nil
	}
}

func (s *DealReviewStore) loadLearnedPatternLiveGuardRollupCaches(
	userID string,
	traderID string,
	refs []string,
	limit int,
) (map[string]DealReviewLearnedPatternLiveGuardRollupCache, error) {
	cache := make(map[string]DealReviewLearnedPatternLiveGuardRollupCache)
	if len(refs) == 0 {
		return cache, nil
	}
	var rows []DealReviewLearnedPatternLiveGuardRollupCache
	if err := s.db.Model(&DealReviewLearnedPatternLiveGuardRollupCache{}).
		Where(
			"user_id = ? AND trader_id = ? AND event_limit = ? AND pattern_ref IN ?",
			strings.TrimSpace(userID),
			strings.TrimSpace(traderID),
			limit,
			refs,
		).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		ref := strings.TrimSpace(row.PatternRef)
		if ref == "" {
			continue
		}
		cache[ref] = row
	}
	return cache, nil
}

func (s *DealReviewStore) loadLearnedPatternLiveGuardEventStats(
	userID string,
	traderID string,
	stableRefs []string,
	idRefs []string,
	stableToCanonical map[string]string,
	idToCanonical map[string]string,
) (map[string]dealReviewLearnedPatternLiveGuardEventStats, error) {
	stats := make(map[string]dealReviewLearnedPatternLiveGuardEventStats)
	condition, args := buildDealReviewLearnedPatternLiveGuardReferenceCondition("", stableRefs, idRefs)
	if condition == "" {
		return stats, nil
	}

	type eventStatRow struct {
		PatternRef       string         `gorm:"column:pattern_ref"`
		SourceEventCount int            `gorm:"column:source_event_count"`
		LatestEventAtRaw sql.NullString `gorm:"column:latest_event_at_raw"`
	}

	refExpr := "CASE WHEN matched_pattern_stable_key <> '' THEN matched_pattern_stable_key ELSE matched_pattern_id END"
	query := fmt.Sprintf(
		`SELECT %s AS pattern_ref, COUNT(*) AS source_event_count, CAST(MAX(decision_timestamp) AS TEXT) AS latest_event_at_raw
		FROM %s
		WHERE user_id = ? AND trader_id = ? AND %s
		GROUP BY %s`,
		refExpr,
		DealReviewLearnedPatternLiveGuardEvent{}.TableName(),
		condition,
		refExpr,
	)

	params := []any{strings.TrimSpace(userID), strings.TrimSpace(traderID)}
	params = append(params, args...)

	var rows []eventStatRow
	if err := s.db.Raw(query, params...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		canonicalRef := strings.TrimSpace(row.PatternRef)
		if mapped := strings.TrimSpace(stableToCanonical[canonicalRef]); mapped != "" {
			canonicalRef = mapped
		} else if mapped := strings.TrimSpace(idToCanonical[canonicalRef]); mapped != "" {
			canonicalRef = mapped
		}
		if canonicalRef == "" {
			continue
		}
		stats[canonicalRef] = dealReviewLearnedPatternLiveGuardEventStats{
			PatternRef:       canonicalRef,
			SourceEventCount: row.SourceEventCount,
			LatestEventAt:    parseDealReviewAggregateTime(row.LatestEventAtRaw.String),
		}
	}
	return stats, nil
}

func isDealReviewLearnedPatternLiveGuardRollupCacheFresh(
	cache *DealReviewLearnedPatternLiveGuardRollupCache,
	stats dealReviewLearnedPatternLiveGuardEventStats,
	now time.Time,
	limit int,
) bool {
	if cache == nil {
		return false
	}
	if cache.EventLimit != limit {
		return false
	}
	if cache.SourceEventCount != stats.SourceEventCount {
		return false
	}
	if stats.SourceEventCount <= 0 {
		return false
	}
	if !sameDealReviewLiveGuardRollupTime(cache.LatestEventAt, stats.LatestEventAt) {
		return false
	}
	if cache.RefreshAfter.IsZero() || !cache.RefreshAfter.After(now.UTC()) {
		return false
	}
	return unmarshalDealReviewLearnedPatternLiveGuardRollup(cache.RollupJSON) != nil &&
		unmarshalDealReviewLearnedPatternLiveGuardDelta(cache.DeltaJSON) != nil
}

func buildDealReviewLearnedPatternLiveGuardRollupRefreshAfter(
	rollup *DealReviewLearnedPatternLiveGuardAttributionRollup,
	now time.Time,
) time.Time {
	now = now.UTC()
	if rollup == nil {
		return now.Add(dealReviewLearnedPatternLiveGuardRollupUnresolvedCacheTTL)
	}
	if rollup.PendingCount > 0 || rollup.FollowupOpenCount > 0 {
		return now.Add(dealReviewLearnedPatternLiveGuardRollupUnresolvedCacheTTL)
	}
	return now.Add(dealReviewLearnedPatternLiveGuardRollupResolvedCacheTTL)
}

func sameDealReviewLiveGuardRollupTime(left, right time.Time) bool {
	if left.IsZero() || right.IsZero() {
		return left.IsZero() && right.IsZero()
	}
	return left.UTC().UnixMilli() == right.UTC().UnixMilli()
}

func dedupeDealReviewLearnedPatternRefs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildDealReviewLearnedPatternLiveGuardRollupCacheRecord(
	pattern *DealReviewLearnedPattern,
	rollup *DealReviewLearnedPatternLiveGuardAttributionRollup,
	delta *DealReviewLearnedPatternLiveGuardAttributionDelta,
	stats dealReviewLearnedPatternLiveGuardEventStats,
	limit int,
	now time.Time,
) *DealReviewLearnedPatternLiveGuardRollupCache {
	if pattern == nil || rollup == nil || delta == nil {
		return nil
	}
	patternRef := dealReviewLearnedPatternLiveGuardCanonicalRef(pattern)
	if patternRef == "" {
		return nil
	}
	now = now.UTC()
	return &DealReviewLearnedPatternLiveGuardRollupCache{
		UserID:           strings.TrimSpace(pattern.UserID),
		TraderID:         strings.TrimSpace(pattern.TraderID),
		PatternRef:       patternRef,
		PatternID:        strings.TrimSpace(pattern.ID),
		PatternStableKey: strings.TrimSpace(pattern.StableKey),
		EventLimit:       limit,
		SourceEventCount: stats.SourceEventCount,
		LatestEventAt:    stats.LatestEventAt,
		RefreshAfter:     buildDealReviewLearnedPatternLiveGuardRollupRefreshAfter(rollup, now),
		BuiltAt:          now,
		RollupJSON:       marshalDealReviewLearnedPatternLiveGuardRollup(rollup),
		DeltaJSON:        marshalDealReviewLearnedPatternLiveGuardDelta(delta),
	}
}

func (s *DealReviewStore) persistLearnedPatternLiveGuardRollupCachesTx(
	tx *gorm.DB,
	records []DealReviewLearnedPatternLiveGuardRollupCache,
) error {
	if tx == nil || len(records) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "trader_id"},
			{Name: "pattern_ref"},
			{Name: "event_limit"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"pattern_id",
			"pattern_stable_key",
			"source_event_count",
			"latest_event_at",
			"refresh_after",
			"built_at",
			"rollup_json",
			"delta_json",
			"updated_at",
		}),
	}).Create(&records).Error
}

func (s *DealReviewStore) deleteLearnedPatternLiveGuardRollupCachesTx(
	tx *gorm.DB,
	userID string,
	traderID string,
	refs []string,
	limit int,
) error {
	if tx == nil || len(refs) == 0 {
		return nil
	}
	query := tx.Model(&DealReviewLearnedPatternLiveGuardRollupCache{}).
		Where(
			"user_id = ? AND trader_id = ? AND pattern_ref IN ?",
			strings.TrimSpace(userID),
			strings.TrimSpace(traderID),
			refs,
		)
	if limit > 0 {
		query = query.Where("event_limit = ?", limit)
	}
	return query.Delete(&DealReviewLearnedPatternLiveGuardRollupCache{}).Error
}
