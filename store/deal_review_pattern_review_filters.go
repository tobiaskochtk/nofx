package store

import (
	"sort"
	"strings"
)

const (
	DealReviewLearnedPatternInterventionStateFilterOpen     = "open"
	DealReviewLearnedPatternInterventionStateFilterResolved = "resolved"
	DealReviewLearnedPatternInterventionStateFilterNone     = "none"
)

func normalizeDealReviewLearnedPatternInterventionStateFilter(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternInterventionStateFilterOpen:
		return DealReviewLearnedPatternInterventionStateFilterOpen
	case DealReviewLearnedPatternInterventionStateFilterResolved:
		return DealReviewLearnedPatternInterventionStateFilterResolved
	case DealReviewLearnedPatternInterventionStateFilterNone:
		return DealReviewLearnedPatternInterventionStateFilterNone
	default:
		return ""
	}
}

func shouldApplyDealReviewLearnedPatternPostHydrationFilter(filter DealReviewLearnedPatternFilter) bool {
	return normalizeDealReviewLearnedPatternLiveActionKind(filter.LiveActionKind) != "" ||
		filter.DirectLiveActionCandidate ||
		normalizeDealReviewLearnedPatternInterventionStateFilter(filter.InterventionState) != ""
}

func dealReviewLearnedPatternHasOpenIntervention(pattern *DealReviewLearnedPattern) bool {
	if pattern == nil {
		return false
	}
	for _, event := range pattern.InterventionHistory {
		if normalizeDealReviewLearnedPatternInterventionStatus(event.EventStatus) == DealReviewLearnedPatternInterventionStatusOpen {
			return true
		}
	}
	return false
}

func dealReviewLearnedPatternHasResolvedIntervention(pattern *DealReviewLearnedPattern) bool {
	if pattern == nil {
		return false
	}
	for _, event := range pattern.InterventionHistory {
		switch normalizeDealReviewLearnedPatternInterventionStatus(event.EventStatus) {
		case DealReviewLearnedPatternInterventionStatusAccepted,
			DealReviewLearnedPatternInterventionStatusOverridden,
			DealReviewLearnedPatternInterventionStatusSuperseded,
			DealReviewLearnedPatternInterventionStatusCleared,
			DealReviewLearnedPatternInterventionStatusStandalone:
			return true
		}
	}
	return false
}

func dealReviewLearnedPatternHasDirectLiveActionCandidate(pattern *DealReviewLearnedPattern) bool {
	if pattern == nil {
		return false
	}
	for _, event := range pattern.InterventionHistory {
		if event.DirectLiveActionCandidate {
			return true
		}
	}
	return false
}

func dealReviewLearnedPatternHasOpenDirectLiveActionCandidate(pattern *DealReviewLearnedPattern) bool {
	if pattern == nil {
		return false
	}
	for _, event := range pattern.InterventionHistory {
		if !event.DirectLiveActionCandidate {
			continue
		}
		if normalizeDealReviewLearnedPatternInterventionStatus(event.EventStatus) == DealReviewLearnedPatternInterventionStatusOpen {
			return true
		}
	}
	return false
}

func dealReviewLearnedPatternHasResolvedDirectLiveActionCandidate(pattern *DealReviewLearnedPattern) bool {
	if pattern == nil {
		return false
	}
	for _, event := range pattern.InterventionHistory {
		if !event.DirectLiveActionCandidate {
			continue
		}
		switch normalizeDealReviewLearnedPatternInterventionStatus(event.EventStatus) {
		case DealReviewLearnedPatternInterventionStatusAccepted,
			DealReviewLearnedPatternInterventionStatusOverridden,
			DealReviewLearnedPatternInterventionStatusSuperseded,
			DealReviewLearnedPatternInterventionStatusCleared,
			DealReviewLearnedPatternInterventionStatusStandalone:
			return true
		}
	}
	return false
}

func dealReviewLearnedPatternStrongestLiveActionKind(
	pattern *DealReviewLearnedPattern,
	openOnly bool,
) string {
	if pattern == nil {
		return ""
	}
	best := ""
	for _, event := range pattern.InterventionHistory {
		if !event.DirectLiveActionCandidate {
			continue
		}
		if openOnly &&
			normalizeDealReviewLearnedPatternInterventionStatus(event.EventStatus) != DealReviewLearnedPatternInterventionStatusOpen {
			continue
		}
		kind := normalizeDealReviewLearnedPatternLiveActionKind(event.DirectLiveActionKind)
		if kind == "" {
			continue
		}
		if best == DealReviewLearnedPatternLiveActionKindRollback {
			continue
		}
		best = kind
	}
	if best != "" {
		return best
	}
	if openOnly || pattern.LiveActionHint == nil {
		return ""
	}
	return normalizeDealReviewLearnedPatternLiveActionKind(pattern.LiveActionHint.CandidateKind)
}

func dealReviewLearnedPatternMatchesLiveActionKind(pattern *DealReviewLearnedPattern, kind string) bool {
	kind = normalizeDealReviewLearnedPatternLiveActionKind(kind)
	if kind == "" || pattern == nil {
		return kind == ""
	}
	if normalizeDealReviewLearnedPatternLiveActionKind(
		dealReviewLearnedPatternLiveActionHintKind(pattern.LiveActionHint),
	) == kind {
		return true
	}
	for _, event := range pattern.InterventionHistory {
		if !event.DirectLiveActionCandidate {
			continue
		}
		if normalizeDealReviewLearnedPatternLiveActionKind(event.DirectLiveActionKind) == kind {
			return true
		}
	}
	return false
}

func dealReviewLearnedPatternMatchesInterventionState(pattern *DealReviewLearnedPattern, state string) bool {
	state = normalizeDealReviewLearnedPatternInterventionStateFilter(state)
	switch state {
	case "":
		return true
	case DealReviewLearnedPatternInterventionStateFilterOpen:
		return dealReviewLearnedPatternHasOpenIntervention(pattern)
	case DealReviewLearnedPatternInterventionStateFilterResolved:
		return dealReviewLearnedPatternHasResolvedIntervention(pattern)
	case DealReviewLearnedPatternInterventionStateFilterNone:
		return pattern == nil || len(pattern.InterventionHistory) == 0
	default:
		return true
	}
}

func filterDealReviewLearnedPatternsPostHydration(
	items []DealReviewLearnedPattern,
	filter DealReviewLearnedPatternFilter,
) []DealReviewLearnedPattern {
	if len(items) == 0 {
		return nil
	}
	liveActionKind := normalizeDealReviewLearnedPatternLiveActionKind(filter.LiveActionKind)
	interventionState := normalizeDealReviewLearnedPatternInterventionStateFilter(filter.InterventionState)
	out := make([]DealReviewLearnedPattern, 0, len(items))
	for idx := range items {
		item := items[idx]
		if liveActionKind != "" && !dealReviewLearnedPatternMatchesLiveActionKind(&item, liveActionKind) {
			continue
		}
		if filter.DirectLiveActionCandidate && !dealReviewLearnedPatternHasDirectLiveActionCandidate(&item) {
			continue
		}
		if interventionState != "" && !dealReviewLearnedPatternMatchesInterventionState(&item, interventionState) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func sortDealReviewLearnedPatternsForReview(items []DealReviewLearnedPattern) {
	sort.SliceStable(items, func(i, j int) bool {
		leftOpenDirect := dealReviewLearnedPatternHasOpenDirectLiveActionCandidate(&items[i])
		rightOpenDirect := dealReviewLearnedPatternHasOpenDirectLiveActionCandidate(&items[j])
		if leftOpenDirect != rightOpenDirect {
			return leftOpenDirect
		}

		leftOpenKind := dealReviewLearnedPatternStrongestLiveActionKind(&items[i], true)
		rightOpenKind := dealReviewLearnedPatternStrongestLiveActionKind(&items[j], true)
		if leftOpenKind != rightOpenKind {
			return dealReviewLearnedPatternLiveActionKindSortRank(leftOpenKind) <
				dealReviewLearnedPatternLiveActionKindSortRank(rightOpenKind)
		}

		leftAnyDirect := dealReviewLearnedPatternHasDirectLiveActionCandidate(&items[i])
		rightAnyDirect := dealReviewLearnedPatternHasDirectLiveActionCandidate(&items[j])
		if leftAnyDirect != rightAnyDirect {
			return leftAnyDirect
		}

		leftCurrentKind := dealReviewLearnedPatternStrongestLiveActionKind(&items[i], false)
		rightCurrentKind := dealReviewLearnedPatternStrongestLiveActionKind(&items[j], false)
		if leftCurrentKind != rightCurrentKind {
			return dealReviewLearnedPatternLiveActionKindSortRank(leftCurrentKind) <
				dealReviewLearnedPatternLiveActionKindSortRank(rightCurrentKind)
		}

		leftOpenIntervention := dealReviewLearnedPatternHasOpenIntervention(&items[i])
		rightOpenIntervention := dealReviewLearnedPatternHasOpenIntervention(&items[j])
		if leftOpenIntervention != rightOpenIntervention {
			return leftOpenIntervention
		}

		leftConfidence := dealReviewLearnedPatternReviewUrgencyScore(&items[i])
		rightConfidence := dealReviewLearnedPatternReviewUrgencyScore(&items[j])
		if leftConfidence != rightConfidence {
			return leftConfidence > rightConfidence
		}

		if items[i].CompositeScore != items[j].CompositeScore {
			return items[i].CompositeScore > items[j].CompositeScore
		}
		if items[i].SampleCount != items[j].SampleCount {
			return items[i].SampleCount > items[j].SampleCount
		}
		return items[i].LastObservedAt.After(items[j].LastObservedAt)
	})
}

func dealReviewLearnedPatternLiveActionKindSortRank(kind string) int {
	switch normalizeDealReviewLearnedPatternLiveActionKind(kind) {
	case DealReviewLearnedPatternLiveActionKindRollback:
		return 0
	case DealReviewLearnedPatternLiveActionKindSuppression:
		return 1
	default:
		return 2
	}
}

func dealReviewLearnedPatternReviewUrgencyScore(pattern *DealReviewLearnedPattern) float64 {
	if pattern == nil {
		return 0
	}
	score := 0.0
	if pattern.LiveActionHint != nil {
		score = maxFloat(score, pattern.LiveActionHint.ConfidenceScore)
	}
	if pattern.ActionHint != nil {
		score = maxFloat(score, pattern.ActionHint.ConfidenceScore)
	}
	if pattern.Lifecycle != nil {
		score = maxFloat(score, maxFloat(pattern.Lifecycle.RollbackScore, pattern.Lifecycle.ExpiryScore))
	}
	if pattern.LiveGuardAttributionDelta != nil {
		score = maxFloat(
			score,
			maxFloat(
				pattern.LiveGuardAttributionDelta.ConfidenceScore,
				maxFloat(
					pattern.LiveGuardAttributionDelta.RecentOverblockingRate,
					maxFloat(
						-pattern.LiveGuardAttributionDelta.ProtectiveRateDelta,
						pattern.LiveGuardAttributionDelta.OverblockingRateDelta,
					),
				),
			),
		)
	}
	for _, event := range pattern.InterventionHistory {
		if !event.DirectLiveActionCandidate {
			continue
		}
		score = maxFloat(score, event.DirectLiveActionConfidence)
	}
	return clampDealReviewUnit(score)
}

func dealReviewLearnedPatternLiveActionHintKind(hint *DealReviewLearnedPatternLiveActionHint) string {
	if hint == nil {
		return ""
	}
	return strings.TrimSpace(hint.CandidateKind)
}
