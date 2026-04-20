package store

import (
	"fmt"
	"strings"
)

const (
	DealReviewLearnedPatternActionHintSuppress        = "suppress"
	DealReviewLearnedPatternActionHintRetire          = "retire"
	DealReviewLearnedPatternActionHintRearm           = "rearm"
	DealReviewLearnedPatternActionHintAcknowledgeLive = "acknowledge_live"

	DealReviewLearnedPatternActionHintPriorityInfo     = "info"
	DealReviewLearnedPatternActionHintPriorityWarning  = "warning"
	DealReviewLearnedPatternActionHintPriorityCritical = "critical"
)

type DealReviewLearnedPatternActionHint struct {
	RecommendedAction string  `json:"recommended_action,omitempty"`
	PriorityLabel     string  `json:"priority_label,omitempty"`
	ReasonCode        string  `json:"reason_code,omitempty"`
	ConfidenceScore   float64 `json:"confidence_score,omitempty"`
	Summary           string  `json:"summary,omitempty"`
	AutoNote          string  `json:"auto_note,omitempty"`
}

func normalizeDealReviewLearnedPatternActionHintAction(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternActionHintSuppress:
		return DealReviewLearnedPatternActionHintSuppress
	case DealReviewLearnedPatternActionHintRetire:
		return DealReviewLearnedPatternActionHintRetire
	case DealReviewLearnedPatternActionHintRearm:
		return DealReviewLearnedPatternActionHintRearm
	case DealReviewLearnedPatternActionHintAcknowledgeLive:
		return DealReviewLearnedPatternActionHintAcknowledgeLive
	default:
		return ""
	}
}

func normalizeDealReviewLearnedPatternActionHintPriority(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternActionHintPriorityInfo:
		return DealReviewLearnedPatternActionHintPriorityInfo
	case DealReviewLearnedPatternActionHintPriorityWarning:
		return DealReviewLearnedPatternActionHintPriorityWarning
	case DealReviewLearnedPatternActionHintPriorityCritical:
		return DealReviewLearnedPatternActionHintPriorityCritical
	default:
		return ""
	}
}

func hydrateDealReviewLearnedPatternActionHints(items []DealReviewLearnedPattern) {
	for idx := range items {
		items[idx].ActionHint = buildDealReviewLearnedPatternActionHint(&items[idx])
	}
	hydrateDealReviewLearnedPatternLiveActionHints(items)
}

func buildDealReviewLearnedPatternActionHint(
	pattern *DealReviewLearnedPattern,
) *DealReviewLearnedPatternActionHint {
	if pattern == nil {
		return nil
	}

	baseUse := baseDealReviewLearnedPatternRecommendedUse(pattern)
	if normalizeDealReviewLearnedPatternRecommendedUse(baseUse) != DealReviewLearnedPatternRecommendedUseMonitoringRule {
		return nil
	}

	manualState := normalizeDealReviewLearnedPatternManualControlState(dealReviewLearnedPatternManualControlState(pattern.ManualControl))
	trend := ""
	if pattern.LiveGuardAttributionDelta != nil {
		trend = normalizeDealReviewLearnedPatternLiveGuardAttributionTrendLabel(
			pattern.LiveGuardAttributionDelta.TrendLabel,
		)
	}
	rollupLabel := ""
	if pattern.LiveGuardAttribution != nil {
		rollupLabel = normalizeDealReviewLearnedPatternLiveGuardAttributionLabel(
			pattern.LiveGuardAttribution.AttributionLabel,
		)
	}
	lifecycleStatus := ""
	expiryScore := 0.0
	rollbackScore := 0.0
	if pattern.Lifecycle != nil {
		lifecycleStatus = normalizeDealReviewLearnedPatternLifecycleStatus(pattern.Lifecycle.Status)
		expiryScore = pattern.Lifecycle.ExpiryScore
		rollbackScore = pattern.Lifecycle.RollbackScore
	}

	recentResolved := 0
	priorResolved := 0
	recentProtective := 0.0
	recentOverblocking := 0.0
	priorProtective := 0.0
	priorOverblocking := 0.0
	deltaConfidence := 0.0
	overblockingDelta := 0.0
	protectiveDelta := 0.0
	if pattern.LiveGuardAttributionDelta != nil {
		recentResolved = pattern.LiveGuardAttributionDelta.RecentResolvedEventCount
		priorResolved = pattern.LiveGuardAttributionDelta.PriorResolvedEventCount
		recentProtective = pattern.LiveGuardAttributionDelta.RecentProtectiveRate
		recentOverblocking = pattern.LiveGuardAttributionDelta.RecentOverblockingRate
		priorProtective = pattern.LiveGuardAttributionDelta.PriorProtectiveRate
		priorOverblocking = pattern.LiveGuardAttributionDelta.PriorOverblockingRate
		deltaConfidence = pattern.LiveGuardAttributionDelta.ConfidenceScore
		overblockingDelta = pattern.LiveGuardAttributionDelta.OverblockingRateDelta
		protectiveDelta = pattern.LiveGuardAttributionDelta.ProtectiveRateDelta
	}

	if manualState == DealReviewLearnedPatternManualControlStateSuppressed ||
		manualState == DealReviewLearnedPatternManualControlStateRetired {
		if (trend == DealReviewLearnedPatternLiveGuardAttributionTrendImproving ||
			rollupLabel == DealReviewLearnedPatternLiveGuardAttributionLabelProtective) &&
			recentResolved >= 2 &&
			recentProtective >= 0.67 &&
			deltaConfidence >= 0.45 &&
			recentOverblocking <= 0.33 {
			summary := fmt.Sprintf(
				"Recent live-guard follow-up recovered enough to justify a re-arm review: protective %.0f%% vs prior %.0f%% across %d recent resolved events.",
				recentProtective*100,
				priorProtective*100,
				recentResolved,
			)
			autoNote := fmt.Sprintf(
				"Suggested re-arm: recent live-guard follow-up improved to protective %.0f%% from %.0f%% across %d recent vs %d prior resolved events.",
				recentProtective*100,
				priorProtective*100,
				recentResolved,
				priorResolved,
			)
			return &DealReviewLearnedPatternActionHint{
				RecommendedAction: DealReviewLearnedPatternActionHintRearm,
				PriorityLabel:     DealReviewLearnedPatternActionHintPriorityWarning,
				ReasonCode:        "delta_recovery_rearm",
				ConfidenceScore:   clampDealReviewUnit(maxFloat(deltaConfidence, recentProtective)),
				Summary:           summary,
				AutoNote:          autoNote,
			}
		}
		return nil
	}

	if lifecycleStatus == DealReviewLearnedPatternLifecycleStatusExpired ||
		expiryScore >= 0.85 ||
		(trend == DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking &&
			recentResolved >= 3 &&
			recentOverblocking >= 0.85 &&
			(rollbackScore >= 0.70 || rollupLabel == DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking)) {
		summary := fmt.Sprintf(
			"Live-rule retirement is recommended: lifecycle %s, expiry %.0f%%, recent overblocking %.0f%% vs prior %.0f%% across %d recent resolved events.",
			blankToValue(lifecycleStatus, "active"),
			expiryScore*100,
			recentOverblocking*100,
			priorOverblocking*100,
			recentResolved,
		)
		autoNote := fmt.Sprintf(
			"Suggested retire: lifecycle %s with expiry %.0f%% and recent overblocking %.0f%% vs %.0f%% across %d recent vs %d prior resolved events.",
			blankToValue(lifecycleStatus, "active"),
			expiryScore*100,
			recentOverblocking*100,
			priorOverblocking*100,
			recentResolved,
			priorResolved,
		)
		return &DealReviewLearnedPatternActionHint{
			RecommendedAction: DealReviewLearnedPatternActionHintRetire,
			PriorityLabel:     DealReviewLearnedPatternActionHintPriorityCritical,
			ReasonCode:        "delta_retire_expired_or_extreme_overblocking",
			ConfidenceScore: clampDealReviewUnit(
				maxFloat(maxFloat(deltaConfidence, recentOverblocking), expiryScore),
			),
			Summary:  summary,
			AutoNote: autoNote,
		}
	}

	if (trend == DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking &&
		recentResolved >= 2 &&
		recentOverblocking >= 0.67) ||
		(trend == DealReviewLearnedPatternLiveGuardAttributionTrendDegrading &&
			recentResolved >= 2 &&
			(overblockingDelta >= 0.25 || protectiveDelta <= -0.25)) ||
		(rollupLabel == DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking &&
			pattern.LiveGuardAttribution != nil &&
			pattern.LiveGuardAttribution.ResolvedEventCount >= 3 &&
			pattern.LiveGuardAttribution.ConfidenceScore >= 0.60) ||
		(lifecycleStatus == DealReviewLearnedPatternLifecycleStatusRollbackWatch &&
			rollbackScore >= 0.65) {
		summary := fmt.Sprintf(
			"Live-rule suppression is recommended: recent overblocking %.0f%% vs prior %.0f%% across %d recent resolved events, rollback %.0f%%.",
			recentOverblocking*100,
			priorOverblocking*100,
			recentResolved,
			rollbackScore*100,
		)
		autoNote := fmt.Sprintf(
			"Suggested suppress: live-guard follow-up worsened to %.0f%% overblocking from %.0f%% across %d recent vs %d prior resolved events (rollback %.0f%%).",
			recentOverblocking*100,
			priorOverblocking*100,
			recentResolved,
			priorResolved,
			rollbackScore*100,
		)
		priority := DealReviewLearnedPatternActionHintPriorityWarning
		if trend == DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking ||
			rollbackScore >= 0.80 {
			priority = DealReviewLearnedPatternActionHintPriorityCritical
		}
		return &DealReviewLearnedPatternActionHint{
			RecommendedAction: DealReviewLearnedPatternActionHintSuppress,
			PriorityLabel:     priority,
			ReasonCode:        "delta_suppress_overblocking_pressure",
			ConfidenceScore: clampDealReviewUnit(
				maxFloat(maxFloat(deltaConfidence, recentOverblocking), rollbackScore),
			),
			Summary:  summary,
			AutoNote: autoNote,
		}
	}

	return nil
}
