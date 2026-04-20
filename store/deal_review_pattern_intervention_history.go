package store

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DealReviewLearnedPatternInterventionEventTypeSuggested    = "suggested"
	DealReviewLearnedPatternInterventionEventTypeManualAction = "manual_action"

	DealReviewLearnedPatternInterventionStatusOpen       = "open"
	DealReviewLearnedPatternInterventionStatusAccepted   = "accepted"
	DealReviewLearnedPatternInterventionStatusOverridden = "overridden"
	DealReviewLearnedPatternInterventionStatusSuperseded = "superseded"
	DealReviewLearnedPatternInterventionStatusCleared    = "cleared"
	DealReviewLearnedPatternInterventionStatusStandalone = "standalone"

	dealReviewLearnedPatternInterventionHistoryLimit = 6
)

type DealReviewLearnedPatternIntervention struct {
	ID                         string    `gorm:"primaryKey" json:"id"`
	UserID                     string    `gorm:"column:user_id;not null;index:idx_pattern_intervention_user_trader,priority:1" json:"user_id"`
	TraderID                   string    `gorm:"column:trader_id;not null;index:idx_pattern_intervention_user_trader,priority:2;index:idx_pattern_intervention_ref" json:"trader_id"`
	PatternID                  string    `gorm:"column:pattern_id;default:'';index:idx_pattern_intervention_pattern_id" json:"pattern_id,omitempty"`
	PatternStableKey           string    `gorm:"column:pattern_stable_key;default:'';index:idx_pattern_intervention_ref" json:"pattern_stable_key,omitempty"`
	PatternSignature           string    `gorm:"column:pattern_signature;type:text;default:''" json:"pattern_signature,omitempty"`
	PatternClass               string    `gorm:"column:pattern_class;default:''" json:"pattern_class,omitempty"`
	ScopeType                  string    `gorm:"column:scope_type;default:''" json:"scope_type,omitempty"`
	Symbol                     string    `gorm:"column:symbol;default:''" json:"symbol,omitempty"`
	Side                       string    `gorm:"column:side;default:''" json:"side,omitempty"`
	EventType                  string    `gorm:"column:event_type;default:'';index:idx_pattern_intervention_type" json:"event_type"`
	EventStatus                string    `gorm:"column:event_status;default:'';index:idx_pattern_intervention_status" json:"event_status"`
	TriggerFingerprint         string    `gorm:"column:trigger_fingerprint;default:'';index:idx_pattern_intervention_trigger_fingerprint" json:"trigger_fingerprint,omitempty"`
	TriggerReasonCode          string    `gorm:"column:trigger_reason_code;default:''" json:"trigger_reason_code,omitempty"`
	TriggerTrendLabel          string    `gorm:"column:trigger_trend_label;default:''" json:"trigger_trend_label,omitempty"`
	TriggerPriorityLabel       string    `gorm:"column:trigger_priority_label;default:''" json:"trigger_priority_label,omitempty"`
	SuggestedAction            string    `gorm:"column:suggested_action;default:''" json:"suggested_action,omitempty"`
	AppliedAction              string    `gorm:"column:applied_action;default:''" json:"applied_action,omitempty"`
	AcceptedSuggestion         bool      `gorm:"column:accepted_suggestion;default:false" json:"accepted_suggestion"`
	Summary                    string    `gorm:"column:summary;type:text;default:''" json:"summary,omitempty"`
	Note                       string    `gorm:"column:note;type:text;default:''" json:"note,omitempty"`
	ActionHintConfidenceScore  float64   `gorm:"column:action_hint_confidence_score;default:0" json:"action_hint_confidence_score,omitempty"`
	DeltaConfidenceScore       float64   `gorm:"column:delta_confidence_score;default:0" json:"delta_confidence_score,omitempty"`
	RecentResolvedEventCount   int       `gorm:"column:recent_resolved_event_count;default:0" json:"recent_resolved_event_count,omitempty"`
	PriorResolvedEventCount    int       `gorm:"column:prior_resolved_event_count;default:0" json:"prior_resolved_event_count,omitempty"`
	RecentOverblockingRate     float64   `gorm:"column:recent_overblocking_rate;default:0" json:"recent_overblocking_rate,omitempty"`
	PriorOverblockingRate      float64   `gorm:"column:prior_overblocking_rate;default:0" json:"prior_overblocking_rate,omitempty"`
	RecentProtectiveRate       float64   `gorm:"column:recent_protective_rate;default:0" json:"recent_protective_rate,omitempty"`
	PriorProtectiveRate        float64   `gorm:"column:prior_protective_rate;default:0" json:"prior_protective_rate,omitempty"`
	LifecycleStatus            string    `gorm:"column:lifecycle_status;default:''" json:"lifecycle_status,omitempty"`
	RollbackScore              float64   `gorm:"column:rollback_score;default:0" json:"rollback_score,omitempty"`
	ExpiryScore                float64   `gorm:"column:expiry_score;default:0" json:"expiry_score,omitempty"`
	SeenCount                  int       `gorm:"column:seen_count;default:0" json:"seen_count,omitempty"`
	DirectLiveActionCandidate  bool      `gorm:"column:direct_live_action_candidate;default:false" json:"direct_live_action_candidate"`
	DirectLiveActionKind       string    `gorm:"column:direct_live_action_kind;default:''" json:"direct_live_action_kind,omitempty"`
	DirectLiveActionSummary    string    `gorm:"column:direct_live_action_summary;type:text;default:''" json:"direct_live_action_summary,omitempty"`
	DirectLiveActionConfidence float64   `gorm:"column:direct_live_action_confidence;default:0" json:"direct_live_action_confidence,omitempty"`
	FirstSeenAt                time.Time `gorm:"column:first_seen_at;index:idx_pattern_intervention_first_seen" json:"first_seen_at"`
	LastSeenAt                 time.Time `gorm:"column:last_seen_at;index:idx_pattern_intervention_last_seen" json:"last_seen_at"`
	ResolvedAt                 time.Time `gorm:"column:resolved_at;index:idx_pattern_intervention_resolved" json:"resolved_at"`
	SourceManualControlEventID string    `gorm:"column:source_manual_control_event_id;default:''" json:"source_manual_control_event_id,omitempty"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

func (DealReviewLearnedPatternIntervention) TableName() string {
	return "deal_review_learned_pattern_interventions"
}

func normalizeDealReviewLearnedPatternInterventionEventType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternInterventionEventTypeSuggested:
		return DealReviewLearnedPatternInterventionEventTypeSuggested
	case DealReviewLearnedPatternInterventionEventTypeManualAction:
		return DealReviewLearnedPatternInterventionEventTypeManualAction
	default:
		return ""
	}
}

func normalizeDealReviewLearnedPatternInterventionStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternInterventionStatusOpen:
		return DealReviewLearnedPatternInterventionStatusOpen
	case DealReviewLearnedPatternInterventionStatusAccepted:
		return DealReviewLearnedPatternInterventionStatusAccepted
	case DealReviewLearnedPatternInterventionStatusOverridden:
		return DealReviewLearnedPatternInterventionStatusOverridden
	case DealReviewLearnedPatternInterventionStatusSuperseded:
		return DealReviewLearnedPatternInterventionStatusSuperseded
	case DealReviewLearnedPatternInterventionStatusCleared:
		return DealReviewLearnedPatternInterventionStatusCleared
	case DealReviewLearnedPatternInterventionStatusStandalone:
		return DealReviewLearnedPatternInterventionStatusStandalone
	default:
		return ""
	}
}

func dealReviewLearnedPatternInterventionReference(pattern *DealReviewLearnedPattern) string {
	if pattern == nil {
		return ""
	}
	if stableKey := strings.TrimSpace(pattern.StableKey); stableKey != "" {
		return stableKey
	}
	return strings.TrimSpace(pattern.ID)
}

func dealReviewLearnedPatternInterventionRound(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func buildDealReviewLearnedPatternInterventionTriggerFingerprint(pattern *DealReviewLearnedPattern) string {
	if pattern == nil || pattern.ActionHint == nil {
		return ""
	}
	ref := dealReviewLearnedPatternInterventionReference(pattern)
	if ref == "" {
		return ""
	}
	trendLabel := ""
	deltaConfidence := 0.0
	recentResolved := 0
	priorResolved := 0
	recentOverblocking := 0.0
	priorOverblocking := 0.0
	recentProtective := 0.0
	priorProtective := 0.0
	if pattern.LiveGuardAttributionDelta != nil {
		trendLabel = normalizeDealReviewLearnedPatternLiveGuardAttributionTrendLabel(
			pattern.LiveGuardAttributionDelta.TrendLabel,
		)
		deltaConfidence = pattern.LiveGuardAttributionDelta.ConfidenceScore
		recentResolved = pattern.LiveGuardAttributionDelta.RecentResolvedEventCount
		priorResolved = pattern.LiveGuardAttributionDelta.PriorResolvedEventCount
		recentOverblocking = dealReviewLearnedPatternInterventionRound(pattern.LiveGuardAttributionDelta.RecentOverblockingRate)
		priorOverblocking = dealReviewLearnedPatternInterventionRound(pattern.LiveGuardAttributionDelta.PriorOverblockingRate)
		recentProtective = dealReviewLearnedPatternInterventionRound(pattern.LiveGuardAttributionDelta.RecentProtectiveRate)
		priorProtective = dealReviewLearnedPatternInterventionRound(pattern.LiveGuardAttributionDelta.PriorProtectiveRate)
	}
	lifecycleStatus := ""
	rollbackScore := 0.0
	expiryScore := 0.0
	if pattern.Lifecycle != nil {
		lifecycleStatus = normalizeDealReviewLearnedPatternLifecycleStatus(pattern.Lifecycle.Status)
		rollbackScore = dealReviewLearnedPatternInterventionRound(pattern.Lifecycle.RollbackScore)
		expiryScore = dealReviewLearnedPatternInterventionRound(pattern.Lifecycle.ExpiryScore)
	}
	payload := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%d|%d|%.3f|%.3f|%.3f|%.3f|%.3f|%s|%.3f|%.3f|%.3f",
		ref,
		strings.TrimSpace(pattern.ActionHint.RecommendedAction),
		strings.TrimSpace(pattern.ActionHint.PriorityLabel),
		strings.TrimSpace(pattern.ActionHint.ReasonCode),
		trendLabel,
		recentResolved,
		priorResolved,
		recentOverblocking,
		priorOverblocking,
		recentProtective,
		priorProtective,
		deltaConfidence,
		lifecycleStatus,
		rollbackScore,
		expiryScore,
		dealReviewLearnedPatternInterventionRound(pattern.ActionHint.ConfidenceScore),
	)
	sum := sha1.Sum([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func buildDealReviewLearnedPatternInterventionSummary(pattern *DealReviewLearnedPattern) string {
	if pattern == nil {
		return ""
	}
	if pattern.ActionHint != nil && strings.TrimSpace(pattern.ActionHint.Summary) != "" {
		return strings.TrimSpace(pattern.ActionHint.Summary)
	}
	if pattern.LiveGuardAttributionDelta != nil && strings.TrimSpace(pattern.LiveGuardAttributionDelta.Summary) != "" {
		return strings.TrimSpace(pattern.LiveGuardAttributionDelta.Summary)
	}
	if pattern.Lifecycle != nil && strings.TrimSpace(pattern.Lifecycle.Summary) != "" {
		return strings.TrimSpace(pattern.Lifecycle.Summary)
	}
	return strings.TrimSpace(pattern.Summary)
}

func buildDealReviewLearnedPatternSuggestedIntervention(
	userID string,
	traderID string,
	pattern *DealReviewLearnedPattern,
	now time.Time,
) *DealReviewLearnedPatternIntervention {
	if pattern == nil || pattern.ActionHint == nil {
		return nil
	}
	ref := dealReviewLearnedPatternInterventionReference(pattern)
	if ref == "" {
		return nil
	}
	entry := &DealReviewLearnedPatternIntervention{
		ID:                        uuid.NewString(),
		UserID:                    strings.TrimSpace(userID),
		TraderID:                  strings.TrimSpace(traderID),
		PatternID:                 strings.TrimSpace(pattern.ID),
		PatternStableKey:          strings.TrimSpace(pattern.StableKey),
		PatternSignature:          strings.TrimSpace(pattern.PatternSignature),
		PatternClass:              strings.TrimSpace(pattern.PatternClass),
		ScopeType:                 strings.TrimSpace(pattern.ScopeType),
		Symbol:                    strings.TrimSpace(pattern.Symbol),
		Side:                      strings.TrimSpace(pattern.Side),
		EventType:                 DealReviewLearnedPatternInterventionEventTypeSuggested,
		EventStatus:               DealReviewLearnedPatternInterventionStatusOpen,
		TriggerFingerprint:        buildDealReviewLearnedPatternInterventionTriggerFingerprint(pattern),
		TriggerReasonCode:         strings.TrimSpace(pattern.ActionHint.ReasonCode),
		TriggerPriorityLabel:      strings.TrimSpace(pattern.ActionHint.PriorityLabel),
		SuggestedAction:           strings.TrimSpace(pattern.ActionHint.RecommendedAction),
		Summary:                   buildDealReviewLearnedPatternInterventionSummary(pattern),
		Note:                      strings.TrimSpace(pattern.ActionHint.AutoNote),
		ActionHintConfidenceScore: pattern.ActionHint.ConfidenceScore,
		SeenCount:                 1,
		FirstSeenAt:               now,
		LastSeenAt:                now,
	}
	if pattern.LiveActionHint == nil {
		pattern.LiveActionHint = buildDealReviewLearnedPatternLiveActionHint(pattern)
	}
	if pattern.LiveActionHint != nil {
		entry.DirectLiveActionCandidate = true
		entry.DirectLiveActionKind = normalizeDealReviewLearnedPatternLiveActionKind(
			pattern.LiveActionHint.CandidateKind,
		)
		entry.DirectLiveActionSummary = strings.TrimSpace(pattern.LiveActionHint.Summary)
		entry.DirectLiveActionConfidence = pattern.LiveActionHint.ConfidenceScore
	}
	if pattern.LiveGuardAttributionDelta != nil {
		entry.TriggerTrendLabel = strings.TrimSpace(pattern.LiveGuardAttributionDelta.TrendLabel)
		entry.DeltaConfidenceScore = pattern.LiveGuardAttributionDelta.ConfidenceScore
		entry.RecentResolvedEventCount = pattern.LiveGuardAttributionDelta.RecentResolvedEventCount
		entry.PriorResolvedEventCount = pattern.LiveGuardAttributionDelta.PriorResolvedEventCount
		entry.RecentOverblockingRate = pattern.LiveGuardAttributionDelta.RecentOverblockingRate
		entry.PriorOverblockingRate = pattern.LiveGuardAttributionDelta.PriorOverblockingRate
		entry.RecentProtectiveRate = pattern.LiveGuardAttributionDelta.RecentProtectiveRate
		entry.PriorProtectiveRate = pattern.LiveGuardAttributionDelta.PriorProtectiveRate
	}
	if pattern.Lifecycle != nil {
		entry.LifecycleStatus = strings.TrimSpace(pattern.Lifecycle.Status)
		entry.RollbackScore = pattern.Lifecycle.RollbackScore
		entry.ExpiryScore = pattern.Lifecycle.ExpiryScore
	}
	return entry
}

func dealReviewLearnedPatternInterventionCurrentRefs(
	items []DealReviewLearnedPattern,
) (map[string]*DealReviewLearnedPatternIntervention, map[string]*DealReviewLearnedPattern) {
	suggestions := make(map[string]*DealReviewLearnedPatternIntervention, len(items))
	byRef := make(map[string]*DealReviewLearnedPattern, len(items))
	now := time.Now().UTC()
	for idx := range items {
		pattern := &items[idx]
		ref := dealReviewLearnedPatternInterventionReference(pattern)
		if ref == "" {
			continue
		}
		byRef[ref] = pattern
		if suggestion := buildDealReviewLearnedPatternSuggestedIntervention(
			strings.TrimSpace(pattern.UserID),
			strings.TrimSpace(pattern.TraderID),
			pattern,
			now,
		); suggestion != nil {
			suggestions[ref] = suggestion
		}
	}
	return suggestions, byRef
}

func (s *DealReviewStore) syncLearnedPatternSuggestedInterventions(
	userID string,
	traderID string,
	items []DealReviewLearnedPattern,
) error {
	return s.syncLearnedPatternSuggestedInterventionsWithDB(s.db, userID, traderID, items)
}

func (s *DealReviewStore) syncLearnedPatternSuggestedInterventionsWithDB(
	db *gorm.DB,
	userID string,
	traderID string,
	items []DealReviewLearnedPattern,
) error {
	if db == nil || len(items) == 0 {
		return nil
	}
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" {
		return nil
	}

	suggestionsByRef := make(map[string]*DealReviewLearnedPatternIntervention, len(items))
	refs := make([]string, 0, len(items))
	patternIDs := make([]string, 0, len(items))
	seenRefs := make(map[string]struct{}, len(items))
	seenIDs := make(map[string]struct{}, len(items))
	now := time.Now().UTC()
	for idx := range items {
		pattern := &items[idx]
		ref := dealReviewLearnedPatternInterventionReference(pattern)
		if ref == "" {
			continue
		}
		if _, ok := seenRefs[ref]; !ok {
			seenRefs[ref] = struct{}{}
			refs = append(refs, ref)
		}
		if patternID := strings.TrimSpace(pattern.ID); patternID != "" {
			if _, ok := seenIDs[patternID]; !ok {
				seenIDs[patternID] = struct{}{}
				patternIDs = append(patternIDs, patternID)
			}
		}
		if suggestion := buildDealReviewLearnedPatternSuggestedIntervention(userID, traderID, pattern, now); suggestion != nil {
			suggestionsByRef[ref] = suggestion
		}
	}
	if len(refs) == 0 && len(patternIDs) == 0 {
		return nil
	}

	var existing []DealReviewLearnedPatternIntervention
	if err := db.Model(&DealReviewLearnedPatternIntervention{}).
		Where(
			"user_id = ? AND trader_id = ? AND event_type = ? AND event_status = ? AND ((pattern_stable_key <> '' AND pattern_stable_key IN ?) OR ((pattern_stable_key = '' OR pattern_stable_key IS NULL) AND pattern_id IN ?))",
			userID,
			traderID,
			DealReviewLearnedPatternInterventionEventTypeSuggested,
			DealReviewLearnedPatternInterventionStatusOpen,
			refs,
			patternIDs,
		).
		Order("last_seen_at DESC, created_at DESC").
		Find(&existing).Error; err != nil {
		return err
	}

	existingByRef := make(map[string][]DealReviewLearnedPatternIntervention, len(refs))
	for _, item := range existing {
		ref := strings.TrimSpace(item.PatternStableKey)
		if ref == "" {
			ref = strings.TrimSpace(item.PatternID)
		}
		if ref == "" {
			continue
		}
		existingByRef[ref] = append(existingByRef[ref], item)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, ref := range refs {
			current, hasCurrent := suggestionsByRef[ref]
			rows := existingByRef[ref]
			matchedCurrent := false
			for _, row := range rows {
				if hasCurrent && strings.TrimSpace(row.TriggerFingerprint) == strings.TrimSpace(current.TriggerFingerprint) {
					updates := map[string]any{
						"summary":                       current.Summary,
						"note":                          current.Note,
						"suggested_action":              current.SuggestedAction,
						"trigger_reason_code":           current.TriggerReasonCode,
						"trigger_trend_label":           current.TriggerTrendLabel,
						"trigger_priority_label":        current.TriggerPriorityLabel,
						"action_hint_confidence_score":  current.ActionHintConfidenceScore,
						"delta_confidence_score":        current.DeltaConfidenceScore,
						"recent_resolved_event_count":   current.RecentResolvedEventCount,
						"prior_resolved_event_count":    current.PriorResolvedEventCount,
						"recent_overblocking_rate":      current.RecentOverblockingRate,
						"prior_overblocking_rate":       current.PriorOverblockingRate,
						"recent_protective_rate":        current.RecentProtectiveRate,
						"prior_protective_rate":         current.PriorProtectiveRate,
						"lifecycle_status":              current.LifecycleStatus,
						"rollback_score":                current.RollbackScore,
						"expiry_score":                  current.ExpiryScore,
						"last_seen_at":                  now,
						"seen_count":                    row.SeenCount + 1,
						"direct_live_action_candidate":  current.DirectLiveActionCandidate,
						"direct_live_action_kind":       current.DirectLiveActionKind,
						"direct_live_action_summary":    current.DirectLiveActionSummary,
						"direct_live_action_confidence": current.DirectLiveActionConfidence,
					}
					if err := tx.Model(&DealReviewLearnedPatternIntervention{}).
						Where("id = ?", row.ID).
						Updates(updates).Error; err != nil {
						return err
					}
					matchedCurrent = true
					continue
				}
				nextStatus := DealReviewLearnedPatternInterventionStatusCleared
				if hasCurrent {
					nextStatus = DealReviewLearnedPatternInterventionStatusSuperseded
				}
				if err := tx.Model(&DealReviewLearnedPatternIntervention{}).
					Where("id = ?", row.ID).
					Updates(map[string]any{
						"event_status": nextStatus,
						"resolved_at":  now,
					}).Error; err != nil {
					return err
				}
			}
			if hasCurrent && !matchedCurrent {
				if err := tx.Create(current).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *DealReviewStore) hydrateLearnedPatternInterventions(
	userID string,
	traderID string,
	items []DealReviewLearnedPattern,
	historyLimit int,
) error {
	return s.hydrateLearnedPatternInterventionsWithDB(s.db, userID, traderID, items, historyLimit)
}

func (s *DealReviewStore) hydrateLearnedPatternInterventionsWithDB(
	db *gorm.DB,
	userID string,
	traderID string,
	items []DealReviewLearnedPattern,
	historyLimit int,
) error {
	if db == nil || len(items) == 0 {
		return nil
	}
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" {
		return nil
	}
	if historyLimit <= 0 || historyLimit > 20 {
		historyLimit = dealReviewLearnedPatternInterventionHistoryLimit
	}

	refs := make([]string, 0, len(items))
	patternIDs := make([]string, 0, len(items))
	seenRefs := make(map[string]struct{}, len(items))
	seenIDs := make(map[string]struct{}, len(items))
	for idx := range items {
		ref := dealReviewLearnedPatternInterventionReference(&items[idx])
		if ref != "" {
			if _, ok := seenRefs[ref]; !ok {
				seenRefs[ref] = struct{}{}
				refs = append(refs, ref)
			}
		}
		if patternID := strings.TrimSpace(items[idx].ID); patternID != "" {
			if _, ok := seenIDs[patternID]; !ok {
				seenIDs[patternID] = struct{}{}
				patternIDs = append(patternIDs, patternID)
			}
		}
	}
	if len(refs) == 0 && len(patternIDs) == 0 {
		return nil
	}

	var rows []DealReviewLearnedPatternIntervention
	if err := db.Model(&DealReviewLearnedPatternIntervention{}).
		Where(
			"user_id = ? AND trader_id = ? AND ((pattern_stable_key <> '' AND pattern_stable_key IN ?) OR ((pattern_stable_key = '' OR pattern_stable_key IS NULL) AND pattern_id IN ?))",
			userID,
			traderID,
			refs,
			patternIDs,
		).
		Order("last_seen_at DESC, created_at DESC").
		Find(&rows).Error; err != nil {
		return err
	}

	historyByRef := make(map[string][]DealReviewLearnedPatternIntervention, len(refs))
	for _, row := range rows {
		ref := strings.TrimSpace(row.PatternStableKey)
		if ref == "" {
			ref = strings.TrimSpace(row.PatternID)
		}
		if ref == "" || len(historyByRef[ref]) >= historyLimit {
			continue
		}
		historyByRef[ref] = append(historyByRef[ref], row)
	}

	for idx := range items {
		ref := dealReviewLearnedPatternInterventionReference(&items[idx])
		if history, ok := historyByRef[ref]; ok && len(history) > 0 {
			items[idx].InterventionHistory = append(
				[]DealReviewLearnedPatternIntervention(nil),
				history...,
			)
		}
	}
	return nil
}

func buildDealReviewLearnedPatternManualInterventionStatus(
	suggestedAction string,
	appliedAction string,
) string {
	suggestedAction = strings.TrimSpace(suggestedAction)
	appliedAction = strings.TrimSpace(appliedAction)
	switch {
	case suggestedAction == "":
		return DealReviewLearnedPatternInterventionStatusStandalone
	case suggestedAction == appliedAction:
		return DealReviewLearnedPatternInterventionStatusAccepted
	default:
		return DealReviewLearnedPatternInterventionStatusOverridden
	}
}

func buildDealReviewLearnedPatternManualInterventionSummary(
	suggestedAction string,
	appliedAction string,
	pattern *DealReviewLearnedPattern,
) string {
	suggestedAction = strings.TrimSpace(suggestedAction)
	appliedAction = strings.TrimSpace(appliedAction)
	if suggestedAction == "" {
		return fmt.Sprintf(
			"Recorded manual %s intervention without an active system suggestion.",
			blankToValue(appliedAction, "action"),
		)
	}
	if suggestedAction == appliedAction {
		return fmt.Sprintf(
			"Accepted suggested %s intervention for this learned monitoring rule.",
			suggestedAction,
		)
	}
	return fmt.Sprintf(
		"Overrode suggested %s intervention by applying %s instead.",
		suggestedAction,
		blankToValue(appliedAction, "manual action"),
	)
}

func (s *DealReviewStore) recordLearnedPatternManualInterventionTx(
	tx *gorm.DB,
	userID string,
	traderID string,
	pattern *DealReviewLearnedPattern,
	appliedAction string,
	note string,
	controlEventID string,
) error {
	if tx == nil || pattern == nil {
		return nil
	}
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" {
		return nil
	}
	ref := dealReviewLearnedPatternInterventionReference(pattern)
	if ref == "" {
		return nil
	}

	var openSuggestions []DealReviewLearnedPatternIntervention
	if err := tx.Model(&DealReviewLearnedPatternIntervention{}).
		Where(
			"user_id = ? AND trader_id = ? AND event_type = ? AND event_status = ? AND ((pattern_stable_key <> '' AND pattern_stable_key = ?) OR ((pattern_stable_key = '' OR pattern_stable_key IS NULL) AND pattern_id = ?))",
			userID,
			traderID,
			DealReviewLearnedPatternInterventionEventTypeSuggested,
			DealReviewLearnedPatternInterventionStatusOpen,
			ref,
			strings.TrimSpace(pattern.ID),
		).
		Order("last_seen_at DESC, created_at DESC").
		Find(&openSuggestions).Error; err != nil {
		return err
	}

	suggestedAction := ""
	triggerFingerprint := ""
	triggerReasonCode := ""
	triggerPriorityLabel := ""
	triggerTrendLabel := ""
	actionHintConfidence := 0.0
	deltaConfidence := 0.0
	recentResolved := 0
	priorResolved := 0
	recentOverblocking := 0.0
	priorOverblocking := 0.0
	recentProtective := 0.0
	priorProtective := 0.0
	directLiveActionCandidate := false
	directLiveActionKind := ""
	directLiveActionSummary := ""
	directLiveActionConfidence := 0.0
	if len(openSuggestions) > 0 {
		latest := openSuggestions[0]
		suggestedAction = strings.TrimSpace(latest.SuggestedAction)
		triggerFingerprint = strings.TrimSpace(latest.TriggerFingerprint)
		triggerReasonCode = strings.TrimSpace(latest.TriggerReasonCode)
		triggerPriorityLabel = strings.TrimSpace(latest.TriggerPriorityLabel)
		triggerTrendLabel = strings.TrimSpace(latest.TriggerTrendLabel)
		actionHintConfidence = latest.ActionHintConfidenceScore
		deltaConfidence = latest.DeltaConfidenceScore
		recentResolved = latest.RecentResolvedEventCount
		priorResolved = latest.PriorResolvedEventCount
		recentOverblocking = latest.RecentOverblockingRate
		priorOverblocking = latest.PriorOverblockingRate
		recentProtective = latest.RecentProtectiveRate
		priorProtective = latest.PriorProtectiveRate
		directLiveActionCandidate = latest.DirectLiveActionCandidate
		directLiveActionKind = strings.TrimSpace(latest.DirectLiveActionKind)
		directLiveActionSummary = strings.TrimSpace(latest.DirectLiveActionSummary)
		directLiveActionConfidence = latest.DirectLiveActionConfidence
	} else if pattern.ActionHint != nil {
		suggestedAction = strings.TrimSpace(pattern.ActionHint.RecommendedAction)
		triggerFingerprint = buildDealReviewLearnedPatternInterventionTriggerFingerprint(pattern)
		triggerReasonCode = strings.TrimSpace(pattern.ActionHint.ReasonCode)
		triggerPriorityLabel = strings.TrimSpace(pattern.ActionHint.PriorityLabel)
		actionHintConfidence = pattern.ActionHint.ConfidenceScore
		if pattern.LiveGuardAttributionDelta != nil {
			triggerTrendLabel = strings.TrimSpace(pattern.LiveGuardAttributionDelta.TrendLabel)
			deltaConfidence = pattern.LiveGuardAttributionDelta.ConfidenceScore
			recentResolved = pattern.LiveGuardAttributionDelta.RecentResolvedEventCount
			priorResolved = pattern.LiveGuardAttributionDelta.PriorResolvedEventCount
			recentOverblocking = pattern.LiveGuardAttributionDelta.RecentOverblockingRate
			priorOverblocking = pattern.LiveGuardAttributionDelta.PriorOverblockingRate
			recentProtective = pattern.LiveGuardAttributionDelta.RecentProtectiveRate
			priorProtective = pattern.LiveGuardAttributionDelta.PriorProtectiveRate
		}
		if pattern.LiveActionHint == nil {
			pattern.LiveActionHint = buildDealReviewLearnedPatternLiveActionHint(pattern)
		}
		if pattern.LiveActionHint != nil {
			directLiveActionCandidate = true
			directLiveActionKind = normalizeDealReviewLearnedPatternLiveActionKind(
				pattern.LiveActionHint.CandidateKind,
			)
			directLiveActionSummary = strings.TrimSpace(pattern.LiveActionHint.Summary)
			directLiveActionConfidence = pattern.LiveActionHint.ConfidenceScore
		}
	}

	status := buildDealReviewLearnedPatternManualInterventionStatus(suggestedAction, appliedAction)
	now := time.Now().UTC()
	if len(openSuggestions) > 0 {
		for _, suggestion := range openSuggestions {
			if err := tx.Model(&DealReviewLearnedPatternIntervention{}).
				Where("id = ?", suggestion.ID).
				Updates(map[string]any{
					"event_status":                   status,
					"applied_action":                 strings.TrimSpace(appliedAction),
					"accepted_suggestion":            status == DealReviewLearnedPatternInterventionStatusAccepted,
					"note":                           strings.TrimSpace(note),
					"resolved_at":                    now,
					"source_manual_control_event_id": strings.TrimSpace(controlEventID),
				}).Error; err != nil {
				return err
			}
		}
	}

	lifecycleStatus := ""
	rollbackScore := 0.0
	expiryScore := 0.0
	if pattern.Lifecycle != nil {
		lifecycleStatus = strings.TrimSpace(pattern.Lifecycle.Status)
		rollbackScore = pattern.Lifecycle.RollbackScore
		expiryScore = pattern.Lifecycle.ExpiryScore
	}

	entry := &DealReviewLearnedPatternIntervention{
		ID:                         uuid.NewString(),
		UserID:                     userID,
		TraderID:                   traderID,
		PatternID:                  strings.TrimSpace(pattern.ID),
		PatternStableKey:           strings.TrimSpace(pattern.StableKey),
		PatternSignature:           strings.TrimSpace(pattern.PatternSignature),
		PatternClass:               strings.TrimSpace(pattern.PatternClass),
		ScopeType:                  strings.TrimSpace(pattern.ScopeType),
		Symbol:                     strings.TrimSpace(pattern.Symbol),
		Side:                       strings.TrimSpace(pattern.Side),
		EventType:                  DealReviewLearnedPatternInterventionEventTypeManualAction,
		EventStatus:                status,
		TriggerFingerprint:         triggerFingerprint,
		TriggerReasonCode:          triggerReasonCode,
		TriggerTrendLabel:          triggerTrendLabel,
		TriggerPriorityLabel:       triggerPriorityLabel,
		SuggestedAction:            suggestedAction,
		AppliedAction:              strings.TrimSpace(appliedAction),
		AcceptedSuggestion:         status == DealReviewLearnedPatternInterventionStatusAccepted,
		Summary:                    buildDealReviewLearnedPatternManualInterventionSummary(suggestedAction, appliedAction, pattern),
		Note:                       strings.TrimSpace(note),
		ActionHintConfidenceScore:  actionHintConfidence,
		DeltaConfidenceScore:       deltaConfidence,
		RecentResolvedEventCount:   recentResolved,
		PriorResolvedEventCount:    priorResolved,
		RecentOverblockingRate:     recentOverblocking,
		PriorOverblockingRate:      priorOverblocking,
		RecentProtectiveRate:       recentProtective,
		PriorProtectiveRate:        priorProtective,
		LifecycleStatus:            lifecycleStatus,
		RollbackScore:              rollbackScore,
		ExpiryScore:                expiryScore,
		SeenCount:                  1,
		DirectLiveActionCandidate:  directLiveActionCandidate,
		DirectLiveActionKind:       directLiveActionKind,
		DirectLiveActionSummary:    directLiveActionSummary,
		DirectLiveActionConfidence: directLiveActionConfidence,
		FirstSeenAt:                now,
		LastSeenAt:                 now,
		ResolvedAt:                 now,
		SourceManualControlEventID: strings.TrimSpace(controlEventID),
	}
	return tx.Create(entry).Error
}
