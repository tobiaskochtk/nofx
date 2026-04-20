package store

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	dealReviewLearnedPatternLifecycleHistoryLimit = 6

	dealReviewLearnedPatternLifecycleSnapshotSourceRebuild        = "rebuild"
	dealReviewLearnedPatternLifecycleSnapshotSourceLiveGuardEvent = "live_guard_event"
)

type DealReviewLearnedPatternLifecycleSnapshot struct {
	ID                            string    `gorm:"primaryKey" json:"id"`
	UserID                        string    `gorm:"column:user_id;not null;index:idx_pattern_lifecycle_snapshot_user_trader,priority:1" json:"user_id"`
	TraderID                      string    `gorm:"column:trader_id;not null;index:idx_pattern_lifecycle_snapshot_user_trader,priority:2;index:idx_pattern_lifecycle_snapshot_pattern_time,priority:1" json:"trader_id"`
	PatternID                     string    `gorm:"column:pattern_id;default:'';index:idx_pattern_lifecycle_snapshot_pattern_id_time,priority:1" json:"pattern_id,omitempty"`
	PatternStableKey              string    `gorm:"column:pattern_stable_key;default:'';index:idx_pattern_lifecycle_snapshot_pattern_time,priority:2" json:"pattern_stable_key,omitempty"`
	PatternSignature              string    `gorm:"column:pattern_signature;type:text;default:''" json:"pattern_signature,omitempty"`
	PatternClass                  string    `gorm:"column:pattern_class;default:''" json:"pattern_class,omitempty"`
	ValidationLabel               string    `gorm:"column:validation_label;default:''" json:"validation_label,omitempty"`
	RecommendedUse                string    `gorm:"column:recommended_use;default:''" json:"recommended_use,omitempty"`
	LifecycleStatus               string    `gorm:"column:lifecycle_status;default:'';index:idx_pattern_lifecycle_snapshot_status" json:"lifecycle_status,omitempty"`
	PreviousStatus                string    `gorm:"column:previous_status;default:''" json:"previous_status,omitempty"`
	StatusChanged                 bool      `gorm:"column:status_changed;default:false" json:"status_changed,omitempty"`
	Operational                   bool      `gorm:"column:operational;default:false" json:"operational"`
	ExpiryScore                   float64   `gorm:"column:expiry_score;default:0" json:"expiry_score,omitempty"`
	RollbackScore                 float64   `gorm:"column:rollback_score;default:0" json:"rollback_score,omitempty"`
	RecentGuardEventCount         int       `gorm:"column:recent_guard_event_count;default:0" json:"recent_guard_event_count,omitempty"`
	RecentQualifiedGuardCount     int       `gorm:"column:recent_qualified_guard_count;default:0" json:"recent_qualified_guard_count,omitempty"`
	RecentHardBlockedCount        int       `gorm:"column:recent_hard_blocked_count;default:0" json:"recent_hard_blocked_count,omitempty"`
	RecentMonitorOnlyCount        int       `gorm:"column:recent_monitor_only_count;default:0" json:"recent_monitor_only_count,omitempty"`
	RecentMatchedUnqualifiedCount int       `gorm:"column:recent_matched_unqualified_count;default:0" json:"recent_matched_unqualified_count,omitempty"`
	RecentGuardBlockRate          float64   `gorm:"column:recent_guard_block_rate;default:0" json:"recent_guard_block_rate,omitempty"`
	LastGuardEventAt              time.Time `gorm:"column:last_guard_event_at" json:"last_guard_event_at,omitempty"`
	LastObservedToGuardLagHours   float64   `gorm:"column:last_observed_to_guard_lag_hours;default:0" json:"last_observed_to_guard_lag_hours,omitempty"`
	Summary                       string    `gorm:"column:summary;type:text;default:''" json:"summary,omitempty"`
	SnapshotSource                string    `gorm:"column:snapshot_source;default:'';index:idx_pattern_lifecycle_snapshot_source" json:"snapshot_source,omitempty"`
	SourceEventID                 string    `gorm:"column:source_event_id;default:'';index:idx_pattern_lifecycle_snapshot_source_event" json:"source_event_id,omitempty"`
	CapturedAt                    time.Time `gorm:"column:captured_at;index:idx_pattern_lifecycle_snapshot_pattern_time,priority:3;index:idx_pattern_lifecycle_snapshot_pattern_id_time,priority:2" json:"captured_at"`
	CreatedAt                     time.Time `json:"created_at"`
	UpdatedAt                     time.Time `json:"updated_at"`
}

func (DealReviewLearnedPatternLifecycleSnapshot) TableName() string {
	return "deal_review_pattern_lifecycle_snapshots"
}

func (s *DealReviewStore) persistLearnedPatternLifecycleSnapshotsTx(
	tx *gorm.DB,
	userID string,
	traderID string,
	patterns []DealReviewLearnedPattern,
	capturedAt time.Time,
	source string,
	sourceEventID string,
) error {
	if tx == nil || len(patterns) == 0 {
		return nil
	}

	working := make([]DealReviewLearnedPattern, len(patterns))
	copy(working, patterns)
	if err := s.hydrateLearnedPatternManualControlsWithDB(tx, userID, traderID, working, 1); err != nil {
		return err
	}
	if err := s.enrichLearnedPatternLifecycleWithDB(tx, userID, traderID, working); err != nil {
		return err
	}

	refs := make([]string, 0, len(working))
	seenRefs := make(map[string]struct{}, len(working))
	for idx := range working {
		ref := dealReviewLearnedPatternLifecycleReference(&working[idx])
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

	latestByRef, err := s.listLatestLearnedPatternLifecycleSnapshotsTx(tx, userID, traderID, refs)
	if err != nil {
		return err
	}

	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	} else {
		capturedAt = capturedAt.UTC()
	}
	source = strings.TrimSpace(source)
	sourceEventID = strings.TrimSpace(sourceEventID)

	rows := make([]DealReviewLearnedPatternLifecycleSnapshot, 0, len(working))
	for idx := range working {
		pattern := working[idx]
		if pattern.Lifecycle == nil {
			continue
		}
		ref := dealReviewLearnedPatternLifecycleReference(&pattern)
		if ref == "" {
			continue
		}
		previousStatus := ""
		statusChanged := false
		if previous, ok := latestByRef[ref]; ok {
			previousStatus = strings.TrimSpace(previous.LifecycleStatus)
			statusChanged = normalizeDealReviewLearnedPatternLifecycleStatus(previousStatus) !=
				normalizeDealReviewLearnedPatternLifecycleStatus(pattern.Lifecycle.Status)
		}
		rows = append(rows, DealReviewLearnedPatternLifecycleSnapshot{
			ID:                            uuid.NewString(),
			UserID:                        strings.TrimSpace(userID),
			TraderID:                      strings.TrimSpace(traderID),
			PatternID:                     strings.TrimSpace(pattern.ID),
			PatternStableKey:              strings.TrimSpace(pattern.StableKey),
			PatternSignature:              strings.TrimSpace(pattern.PatternSignature),
			PatternClass:                  strings.TrimSpace(pattern.PatternClass),
			ValidationLabel:               strings.TrimSpace(pattern.ValidationLabel),
			RecommendedUse:                strings.TrimSpace(pattern.RecommendedUse),
			LifecycleStatus:               strings.TrimSpace(pattern.Lifecycle.Status),
			PreviousStatus:                previousStatus,
			StatusChanged:                 statusChanged,
			Operational:                   pattern.Lifecycle.Operational,
			ExpiryScore:                   pattern.Lifecycle.ExpiryScore,
			RollbackScore:                 pattern.Lifecycle.RollbackScore,
			RecentGuardEventCount:         pattern.Lifecycle.RecentGuardEventCount,
			RecentQualifiedGuardCount:     pattern.Lifecycle.RecentQualifiedGuardCount,
			RecentHardBlockedCount:        pattern.Lifecycle.RecentHardBlockedCount,
			RecentMonitorOnlyCount:        pattern.Lifecycle.RecentMonitorOnlyCount,
			RecentMatchedUnqualifiedCount: pattern.Lifecycle.RecentMatchedUnqualifiedCount,
			RecentGuardBlockRate:          pattern.Lifecycle.RecentGuardBlockRate,
			LastGuardEventAt:              pattern.Lifecycle.LastGuardEventAt,
			LastObservedToGuardLagHours:   pattern.Lifecycle.LastObservedToGuardLagHours,
			Summary:                       strings.TrimSpace(pattern.Lifecycle.Summary),
			SnapshotSource:                source,
			SourceEventID:                 sourceEventID,
			CapturedAt:                    capturedAt,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return dealReviewCreateInBatches(tx, rows)
}

func (s *DealReviewStore) listLatestLearnedPatternLifecycleSnapshotsTx(
	tx *gorm.DB,
	userID string,
	traderID string,
	refs []string,
) (map[string]DealReviewLearnedPatternLifecycleSnapshot, error) {
	out := make(map[string]DealReviewLearnedPatternLifecycleSnapshot, len(refs))
	if tx == nil || len(refs) == 0 {
		return out, nil
	}

	var rows []DealReviewLearnedPatternLifecycleSnapshot
	if err := tx.Model(&DealReviewLearnedPatternLifecycleSnapshot{}).
		Where(
			"user_id = ? AND trader_id = ? AND ((pattern_stable_key <> '' AND pattern_stable_key IN ?) OR ((pattern_stable_key = '' OR pattern_stable_key IS NULL) AND pattern_id IN ?))",
			strings.TrimSpace(userID),
			strings.TrimSpace(traderID),
			refs,
			refs,
		).
		Order("captured_at DESC, created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		ref := strings.TrimSpace(row.PatternStableKey)
		if ref == "" {
			ref = strings.TrimSpace(row.PatternID)
		}
		if ref == "" {
			continue
		}
		if _, ok := out[ref]; ok {
			continue
		}
		out[ref] = row
	}
	return out, nil
}

func dealReviewLearnedPatternLifecycleReference(pattern *DealReviewLearnedPattern) string {
	if pattern == nil {
		return ""
	}
	if strings.TrimSpace(pattern.StableKey) != "" {
		return strings.TrimSpace(pattern.StableKey)
	}
	return strings.TrimSpace(pattern.ID)
}
