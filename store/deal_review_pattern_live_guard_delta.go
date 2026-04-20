package store

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	DealReviewLearnedPatternLiveGuardAttributionTrendImproving            = "improving"
	DealReviewLearnedPatternLiveGuardAttributionTrendStable               = "stable"
	DealReviewLearnedPatternLiveGuardAttributionTrendDegrading            = "degrading"
	DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking    = "newly_overblocking"
	DealReviewLearnedPatternLiveGuardAttributionTrendInsufficientEvidence = "insufficient_evidence"

	dealReviewLearnedPatternLiveGuardAttributionDeltaMinResolved   = 2
	dealReviewLearnedPatternLiveGuardAttributionDeltaShiftMinor    = 0.15
	dealReviewLearnedPatternLiveGuardAttributionDeltaShiftMaterial = 0.25
)

type DealReviewLearnedPatternLiveGuardAttributionDelta struct {
	WindowEventLimit         int     `json:"window_event_limit,omitempty"`
	RecentEventCount         int     `json:"recent_event_count,omitempty"`
	PriorEventCount          int     `json:"prior_event_count,omitempty"`
	RecentResolvedEventCount int     `json:"recent_resolved_event_count,omitempty"`
	PriorResolvedEventCount  int     `json:"prior_resolved_event_count,omitempty"`
	RecentProtectiveRate     float64 `json:"recent_protective_rate,omitempty"`
	PriorProtectiveRate      float64 `json:"prior_protective_rate,omitempty"`
	RecentOverblockingRate   float64 `json:"recent_overblocking_rate,omitempty"`
	PriorOverblockingRate    float64 `json:"prior_overblocking_rate,omitempty"`
	ProtectiveRateDelta      float64 `json:"protective_rate_delta,omitempty"`
	OverblockingRateDelta    float64 `json:"overblocking_rate_delta,omitempty"`
	RecentAttributionLabel   string  `json:"recent_attribution_label,omitempty"`
	PriorAttributionLabel    string  `json:"prior_attribution_label,omitempty"`
	TrendLabel               string  `json:"trend_label,omitempty"`
	ConfidenceScore          float64 `json:"confidence_score,omitempty"`
	Summary                  string  `json:"summary,omitempty"`
}

func buildDealReviewLearnedPatternLiveGuardAttributionDelta(
	events []DealReviewLearnedPatternLiveGuardEvent,
	limit int,
) *DealReviewLearnedPatternLiveGuardAttributionDelta {
	if len(events) == 0 || limit <= 0 {
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

	recentCount := minInt(len(ordered), limit)
	if recentCount <= 0 {
		return nil
	}
	recentRollup := buildDealReviewLearnedPatternLiveGuardAttributionRollup(ordered[:recentCount])
	if recentRollup == nil {
		return nil
	}

	var priorRollup *DealReviewLearnedPatternLiveGuardAttributionRollup
	if len(ordered) > recentCount {
		priorEnd := minInt(len(ordered), recentCount+limit)
		priorRollup = buildDealReviewLearnedPatternLiveGuardAttributionRollup(ordered[recentCount:priorEnd])
	}

	delta := &DealReviewLearnedPatternLiveGuardAttributionDelta{
		WindowEventLimit:         limit,
		RecentEventCount:         recentRollup.EventCount,
		RecentResolvedEventCount: recentRollup.ResolvedEventCount,
		RecentProtectiveRate:     recentRollup.ProtectiveRate,
		RecentOverblockingRate:   recentRollup.OverblockingRate,
		RecentAttributionLabel:   recentRollup.AttributionLabel,
	}
	if priorRollup != nil {
		delta.PriorEventCount = priorRollup.EventCount
		delta.PriorResolvedEventCount = priorRollup.ResolvedEventCount
		delta.PriorProtectiveRate = priorRollup.ProtectiveRate
		delta.PriorOverblockingRate = priorRollup.OverblockingRate
		delta.PriorAttributionLabel = priorRollup.AttributionLabel
	}
	delta.ProtectiveRateDelta = recentRollup.ProtectiveRate - delta.PriorProtectiveRate
	delta.OverblockingRateDelta = recentRollup.OverblockingRate - delta.PriorOverblockingRate
	delta.ConfidenceScore = buildDealReviewLearnedPatternLiveGuardAttributionDeltaConfidence(delta)
	delta.TrendLabel = buildDealReviewLearnedPatternLiveGuardAttributionDeltaTrendLabel(delta)
	delta.Summary = buildDealReviewLearnedPatternLiveGuardAttributionDeltaSummary(delta)
	return delta
}

func buildDealReviewLearnedPatternLiveGuardAttributionDeltaConfidence(
	delta *DealReviewLearnedPatternLiveGuardAttributionDelta,
) float64 {
	if delta == nil {
		return 0
	}
	evidenceScale := clampDealReviewUnit(
		float64(delta.RecentResolvedEventCount+delta.PriorResolvedEventCount) /
			float64(maxInt(delta.WindowEventLimit*2, 1)),
	)
	changeScale := maxFloat(
		math.Abs(delta.ProtectiveRateDelta),
		math.Abs(delta.OverblockingRateDelta),
	)
	if delta.PriorResolvedEventCount == 0 {
		changeScale = math.Max(changeScale, math.Max(delta.RecentProtectiveRate, delta.RecentOverblockingRate))
	}
	return clampDealReviewUnit((changeScale * 0.65) + (evidenceScale * 0.35))
}

func buildDealReviewLearnedPatternLiveGuardAttributionDeltaTrendLabel(
	delta *DealReviewLearnedPatternLiveGuardAttributionDelta,
) string {
	if delta == nil {
		return ""
	}

	recentLabel := normalizeDealReviewLearnedPatternLiveGuardAttributionLabel(delta.RecentAttributionLabel)
	priorLabel := normalizeDealReviewLearnedPatternLiveGuardAttributionLabel(delta.PriorAttributionLabel)

	if delta.RecentResolvedEventCount < 1 {
		return DealReviewLearnedPatternLiveGuardAttributionTrendInsufficientEvidence
	}
	if recentLabel == DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking &&
		delta.RecentResolvedEventCount >= dealReviewLearnedPatternLiveGuardAttributionDeltaMinResolved &&
		(delta.PriorResolvedEventCount < dealReviewLearnedPatternLiveGuardAttributionDeltaMinResolved ||
			(priorLabel != DealReviewLearnedPatternLiveGuardAttributionLabelOverblocking &&
				delta.OverblockingRateDelta >= dealReviewLearnedPatternLiveGuardAttributionDeltaShiftMaterial)) {
		return DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking
	}
	if delta.RecentResolvedEventCount < dealReviewLearnedPatternLiveGuardAttributionDeltaMinResolved ||
		delta.PriorResolvedEventCount < dealReviewLearnedPatternLiveGuardAttributionDeltaMinResolved {
		return DealReviewLearnedPatternLiveGuardAttributionTrendInsufficientEvidence
	}

	switch {
	case delta.OverblockingRateDelta >= dealReviewLearnedPatternLiveGuardAttributionDeltaShiftMinor ||
		delta.ProtectiveRateDelta <= -dealReviewLearnedPatternLiveGuardAttributionDeltaShiftMinor:
		return DealReviewLearnedPatternLiveGuardAttributionTrendDegrading
	case delta.OverblockingRateDelta <= -dealReviewLearnedPatternLiveGuardAttributionDeltaShiftMinor ||
		delta.ProtectiveRateDelta >= dealReviewLearnedPatternLiveGuardAttributionDeltaShiftMinor:
		return DealReviewLearnedPatternLiveGuardAttributionTrendImproving
	default:
		return DealReviewLearnedPatternLiveGuardAttributionTrendStable
	}
}

func buildDealReviewLearnedPatternLiveGuardAttributionDeltaSummary(
	delta *DealReviewLearnedPatternLiveGuardAttributionDelta,
) string {
	if delta == nil {
		return ""
	}

	recentText := fmt.Sprintf(
		"recent overblocking %.0f%% / protective %.0f%% (%d/%d resolved)",
		delta.RecentOverblockingRate*100,
		delta.RecentProtectiveRate*100,
		delta.RecentResolvedEventCount,
		delta.RecentEventCount,
	)
	priorText := "no prior comparison window yet"
	if delta.PriorEventCount > 0 {
		priorText = fmt.Sprintf(
			"prior overblocking %.0f%% / protective %.0f%% (%d/%d resolved)",
			delta.PriorOverblockingRate*100,
			delta.PriorProtectiveRate*100,
			delta.PriorResolvedEventCount,
			delta.PriorEventCount,
		)
	}

	switch normalizeDealReviewLearnedPatternLiveGuardAttributionTrendLabel(delta.TrendLabel) {
	case DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking:
		return fmt.Sprintf(
			"Recent live-guard follow-up newly looks overblocking: %s vs %s.",
			recentText,
			priorText,
		)
	case DealReviewLearnedPatternLiveGuardAttributionTrendDegrading:
		return fmt.Sprintf(
			"Recent live-guard follow-up is degrading vs the prior window: %s vs %s.",
			recentText,
			priorText,
		)
	case DealReviewLearnedPatternLiveGuardAttributionTrendImproving:
		return fmt.Sprintf(
			"Recent live-guard follow-up improved vs the prior window: %s vs %s.",
			recentText,
			priorText,
		)
	case DealReviewLearnedPatternLiveGuardAttributionTrendStable:
		return fmt.Sprintf(
			"Recent live-guard follow-up is broadly stable vs the prior window: %s vs %s.",
			recentText,
			priorText,
		)
	default:
		return fmt.Sprintf(
			"Recent live-guard follow-up does not yet have enough resolved evidence for a reliable trend call: %s vs %s.",
			recentText,
			priorText,
		)
	}
}

func normalizeDealReviewLearnedPatternLiveGuardAttributionTrendLabel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternLiveGuardAttributionTrendImproving:
		return DealReviewLearnedPatternLiveGuardAttributionTrendImproving
	case DealReviewLearnedPatternLiveGuardAttributionTrendStable:
		return DealReviewLearnedPatternLiveGuardAttributionTrendStable
	case DealReviewLearnedPatternLiveGuardAttributionTrendDegrading:
		return DealReviewLearnedPatternLiveGuardAttributionTrendDegrading
	case DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking:
		return DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking
	case DealReviewLearnedPatternLiveGuardAttributionTrendInsufficientEvidence:
		return DealReviewLearnedPatternLiveGuardAttributionTrendInsufficientEvidence
	default:
		return ""
	}
}
