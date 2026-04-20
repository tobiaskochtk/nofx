package store

import (
	"fmt"
	"strings"
)

const (
	DealReviewLearnedPatternLiveActionKindSuppression = "suppression"
	DealReviewLearnedPatternLiveActionKindRollback    = "rollback"
)

type DealReviewLearnedPatternLiveActionHint struct {
	CandidateKind     string  `json:"candidate_kind,omitempty"`
	RecommendedAction string  `json:"recommended_action,omitempty"`
	PriorityLabel     string  `json:"priority_label,omitempty"`
	ConfidenceScore   float64 `json:"confidence_score,omitempty"`
	Summary           string  `json:"summary,omitempty"`
}

func hydrateDealReviewLearnedPatternLiveActionHints(items []DealReviewLearnedPattern) {
	for idx := range items {
		items[idx].LiveActionHint = buildDealReviewLearnedPatternLiveActionHint(&items[idx])
	}
}

func buildDealReviewLearnedPatternLiveActionHint(
	pattern *DealReviewLearnedPattern,
) *DealReviewLearnedPatternLiveActionHint {
	if pattern == nil || pattern.ActionHint == nil {
		return nil
	}
	baseUse := baseDealReviewLearnedPatternRecommendedUse(pattern)
	if normalizeDealReviewLearnedPatternRecommendedUse(baseUse) != DealReviewLearnedPatternRecommendedUseMonitoringRule {
		return nil
	}

	action := normalizeDealReviewLearnedPatternActionHintAction(pattern.ActionHint.RecommendedAction)
	priority := normalizeDealReviewLearnedPatternActionHintPriority(pattern.ActionHint.PriorityLabel)
	if priority != DealReviewLearnedPatternActionHintPriorityCritical {
		return nil
	}

	trend := ""
	recentResolved := 0
	priorResolved := 0
	recentOverblocking := 0.0
	priorOverblocking := 0.0
	recentProtective := 0.0
	priorProtective := 0.0
	deltaConfidence := 0.0
	if pattern.LiveGuardAttributionDelta != nil {
		trend = normalizeDealReviewLearnedPatternLiveGuardAttributionTrendLabel(
			pattern.LiveGuardAttributionDelta.TrendLabel,
		)
		recentResolved = pattern.LiveGuardAttributionDelta.RecentResolvedEventCount
		priorResolved = pattern.LiveGuardAttributionDelta.PriorResolvedEventCount
		recentOverblocking = pattern.LiveGuardAttributionDelta.RecentOverblockingRate
		priorOverblocking = pattern.LiveGuardAttributionDelta.PriorOverblockingRate
		recentProtective = pattern.LiveGuardAttributionDelta.RecentProtectiveRate
		priorProtective = pattern.LiveGuardAttributionDelta.PriorProtectiveRate
		deltaConfidence = pattern.LiveGuardAttributionDelta.ConfidenceScore
	}

	lifecycleStatus := ""
	rollbackScore := 0.0
	expiryScore := 0.0
	if pattern.Lifecycle != nil {
		lifecycleStatus = normalizeDealReviewLearnedPatternLifecycleStatus(pattern.Lifecycle.Status)
		rollbackScore = pattern.Lifecycle.RollbackScore
		expiryScore = pattern.Lifecycle.ExpiryScore
	}

	confidence := clampDealReviewUnit(
		maxFloat(
			maxFloat(pattern.ActionHint.ConfidenceScore, deltaConfidence),
			maxFloat(recentOverblocking, maxFloat(rollbackScore, expiryScore)),
		),
	)

	if confidence < 0.72 {
		return nil
	}

	switch action {
	case DealReviewLearnedPatternActionHintRetire:
		if trend != DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking &&
			lifecycleStatus != DealReviewLearnedPatternLifecycleStatusExpired &&
			expiryScore < 0.85 {
			return nil
		}
		summary := fmt.Sprintf(
			"Direct rollback candidate: retire this live rule now. Recent overblocking is %.0f%% vs %.0f%% prior across %d recent vs %d prior resolved guard events, with lifecycle %s and expiry %.0f%%.",
			recentOverblocking*100,
			priorOverblocking*100,
			recentResolved,
			priorResolved,
			blankToValue(lifecycleStatus, "active"),
			expiryScore*100,
		)
		return &DealReviewLearnedPatternLiveActionHint{
			CandidateKind:     DealReviewLearnedPatternLiveActionKindRollback,
			RecommendedAction: action,
			PriorityLabel:     priority,
			ConfidenceScore:   confidence,
			Summary:           summary,
		}
	case DealReviewLearnedPatternActionHintSuppress:
		if recentResolved < 2 {
			return nil
		}
		if trend != DealReviewLearnedPatternLiveGuardAttributionTrendDegrading &&
			trend != DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking &&
			rollbackScore < 0.80 {
			return nil
		}
		kind := DealReviewLearnedPatternLiveActionKindSuppression
		summary := fmt.Sprintf(
			"Direct suppression candidate: recent overblocking climbed to %.0f%% from %.0f%% across %d recent vs %d prior resolved guard events, with rollback pressure %.0f%%.",
			recentOverblocking*100,
			priorOverblocking*100,
			recentResolved,
			priorResolved,
			rollbackScore*100,
		)
		if trend == DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking ||
			rollbackScore >= 0.82 ||
			(recentOverblocking >= 0.80 && recentProtective <= 0.20) {
			kind = DealReviewLearnedPatternLiveActionKindRollback
			summary = fmt.Sprintf(
				"Direct rollback candidate: suppress this live rule immediately while it is revalidated. Recent overblocking is %.0f%% vs %.0f%% prior and protective follow-up fell to %.0f%% from %.0f%% across %d recent vs %d prior resolved guard events.",
				recentOverblocking*100,
				priorOverblocking*100,
				recentProtective*100,
				priorProtective*100,
				recentResolved,
				priorResolved,
			)
		}
		return &DealReviewLearnedPatternLiveActionHint{
			CandidateKind:     kind,
			RecommendedAction: action,
			PriorityLabel:     priority,
			ConfidenceScore:   confidence,
			Summary:           summary,
		}
	default:
		return nil
	}
}

func normalizeDealReviewLearnedPatternLiveActionKind(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternLiveActionKindSuppression:
		return DealReviewLearnedPatternLiveActionKindSuppression
	case DealReviewLearnedPatternLiveActionKindRollback:
		return DealReviewLearnedPatternLiveActionKindRollback
	default:
		return ""
	}
}
