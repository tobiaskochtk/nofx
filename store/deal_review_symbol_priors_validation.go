package store

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	DealReviewSymbolBehaviorValidationLabelConfirmed            = "confirmed"
	DealReviewSymbolBehaviorValidationLabelCandidate            = "candidate"
	DealReviewSymbolBehaviorValidationLabelInsufficientEvidence = "insufficient_evidence"
	DealReviewSymbolBehaviorValidationLabelDrifting             = "drifting"
	DealReviewSymbolBehaviorValidationLabelFalsePositive        = "false_positive"
	DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk    = "false_negative_risk"
)

type dealReviewSymbolBehaviorObservedCase struct {
	ObservedAt     time.Time
	Outcome        string
	RealizedPnL    float64
	RealizedPnLPct float64
}

type DealReviewSymbolBehaviorPriorReportingSummary struct {
	TotalCount                int                                      `json:"total_count"`
	StatusCounts              map[string]int                           `json:"status_counts,omitempty"`
	LabelCounts               map[string]int                           `json:"label_counts,omitempty"`
	ConfirmedCount            int                                      `json:"confirmed_count"`
	FalsePositiveCount        int                                      `json:"false_positive_count"`
	FalseNegativeRiskCount    int                                      `json:"false_negative_risk_count"`
	DriftingCount             int                                      `json:"drifting_count"`
	InsufficientEvidenceCount int                                      `json:"insufficient_evidence_count"`
	TopFalsePositives         []DealReviewSymbolBehaviorPrior          `json:"top_false_positives,omitempty"`
	TopFalseNegativeRisks     []DealReviewSymbolBehaviorPrior          `json:"top_false_negative_risks,omitempty"`
	TopDrifting               []DealReviewSymbolBehaviorPrior          `json:"top_drifting,omitempty"`
	TopSignalClusters         []DealReviewSymbolBehaviorClusterSummary `json:"top_signal_clusters,omitempty"`
	Notes                     []string                                 `json:"notes,omitempty"`
}

type DealReviewSymbolBehaviorClusterSummary struct {
	ClusterLabel              string   `json:"cluster_label"`
	PriorCount                int      `json:"prior_count"`
	SymbolCount               int      `json:"symbol_count"`
	SampleCount               int      `json:"sample_count"`
	DecisionOpenCount         int      `json:"decision_open_count"`
	ConfirmedCount            int      `json:"confirmed_count"`
	FalsePositiveCount        int      `json:"false_positive_count"`
	FalseNegativeRiskCount    int      `json:"false_negative_risk_count"`
	DriftingCount             int      `json:"drifting_count"`
	NegativeBiasCount         int      `json:"negative_bias_count"`
	PositiveBiasCount         int      `json:"positive_bias_count"`
	AvgPnLPct                 float64  `json:"avg_pnl_pct"`
	AvgContradictionScore     float64  `json:"avg_contradiction_score"`
	AvgCompositeScore         float64  `json:"avg_composite_score"`
	AvgValidationSupportScore float64  `json:"avg_validation_support_score"`
	TopSymbols                []string `json:"top_symbols,omitempty"`
}

type dealReviewSymbolBehaviorClusterAccumulator struct {
	ClusterLabel               string
	PriorCount                 int
	SampleCount                int
	DecisionOpenCount          int
	ConfirmedCount             int
	FalsePositiveCount         int
	FalseNegativeRiskCount     int
	DriftingCount              int
	NegativeBiasCount          int
	PositiveBiasCount          int
	WeightedPnLPctSum          float64
	WeightedContradictionSum   float64
	WeightedCompositeSum       float64
	WeightedValidationScoreSum float64
	Symbols                    map[string]struct{}
	SymbolCounts               map[string]int
}

type dealReviewSymbolBehaviorWindowMetrics struct {
	Count           int
	Wins            int
	Losses          int
	Flats           int
	NetPnL          float64
	AvgPnLPct       float64
	SupportCount    int
	ContradictCount int
	SupportRate     float64
	SupportScore    float64
}

func splitDealReviewSymbolBehaviorObservedCases(items []dealReviewSymbolBehaviorObservedCase) ([]dealReviewSymbolBehaviorObservedCase, []dealReviewSymbolBehaviorObservedCase, []dealReviewSymbolBehaviorObservedCase) {
	if len(items) == 0 {
		return nil, nil, nil
	}
	sortedItems := append([]dealReviewSymbolBehaviorObservedCase(nil), items...)
	sort.SliceStable(sortedItems, func(i, j int) bool {
		if sortedItems[i].ObservedAt.Equal(sortedItems[j].ObservedAt) {
			if sortedItems[i].RealizedPnLPct == sortedItems[j].RealizedPnLPct {
				return sortedItems[i].RealizedPnL < sortedItems[j].RealizedPnL
			}
			return sortedItems[i].RealizedPnLPct < sortedItems[j].RealizedPnLPct
		}
		return sortedItems[i].ObservedAt.Before(sortedItems[j].ObservedAt)
	})

	total := len(sortedItems)
	recentCount := dealReviewSymbolBehaviorRecentWindowCount(total)
	if total >= dealReviewSymbolBehaviorCandidateThreshold+dealReviewSymbolBehaviorObservedThreshold {
		validationCount := dealReviewSymbolBehaviorValidationCount(total)
		maxValidation := total - recentCount - dealReviewSymbolBehaviorObservedThreshold
		if maxValidation >= dealReviewSymbolBehaviorValidationMinSamples {
			if validationCount > maxValidation {
				validationCount = maxValidation
			}
			if validationCount >= dealReviewSymbolBehaviorValidationMinSamples {
				validationStart := total - recentCount - validationCount
				recentStart := total - recentCount
				return append([]dealReviewSymbolBehaviorObservedCase(nil), sortedItems[:validationStart]...),
					append([]dealReviewSymbolBehaviorObservedCase(nil), sortedItems[validationStart:recentStart]...),
					append([]dealReviewSymbolBehaviorObservedCase(nil), sortedItems[recentStart:]...)
			}
		}
	}

	validationCount := dealReviewSymbolBehaviorValidationCount(total)
	maxValidation := total - dealReviewSymbolBehaviorObservedThreshold
	if validationCount > maxValidation {
		validationCount = maxValidation
	}
	if validationCount >= dealReviewSymbolBehaviorValidationMinSamples {
		validationStart := total - validationCount
		validation := append([]dealReviewSymbolBehaviorObservedCase(nil), sortedItems[validationStart:]...)
		return append([]dealReviewSymbolBehaviorObservedCase(nil), sortedItems[:validationStart]...), validation, append([]dealReviewSymbolBehaviorObservedCase(nil), validation...)
	}

	recentStart := total - recentCount
	if recentStart < 0 {
		recentStart = 0
	}
	return append([]dealReviewSymbolBehaviorObservedCase(nil), sortedItems...),
		nil,
		append([]dealReviewSymbolBehaviorObservedCase(nil), sortedItems[recentStart:]...)
}

func dealReviewSymbolBehaviorValidationCount(total int) int {
	if total < dealReviewSymbolBehaviorCandidateThreshold {
		return 0
	}
	count := int(math.Ceil(float64(total) * 0.22))
	if count < dealReviewSymbolBehaviorValidationMinSamples {
		count = dealReviewSymbolBehaviorValidationMinSamples
	}
	return count
}

func dealReviewSymbolBehaviorRecentWindowCount(total int) int {
	if total <= 0 {
		return 0
	}
	count := int(math.Ceil(float64(total) * 0.20))
	if count < dealReviewSymbolBehaviorRecentWindowMin {
		count = dealReviewSymbolBehaviorRecentWindowMin
	}
	if count > dealReviewSymbolBehaviorRecentWindowMax {
		count = dealReviewSymbolBehaviorRecentWindowMax
	}
	if count > total {
		count = total
	}
	return count
}

func summarizeDealReviewSymbolBehaviorObservedCases(items []dealReviewSymbolBehaviorObservedCase, bias string) dealReviewSymbolBehaviorWindowMetrics {
	if len(items) == 0 {
		return dealReviewSymbolBehaviorWindowMetrics{}
	}

	metrics := dealReviewSymbolBehaviorWindowMetrics{Count: len(items)}
	for _, item := range items {
		metrics.NetPnL += item.RealizedPnL
		metrics.AvgPnLPct += item.RealizedPnLPct
		switch classifyDealOutcome(item.RealizedPnL) {
		case "profit":
			metrics.Wins++
		case "loss":
			metrics.Losses++
		default:
			metrics.Flats++
		}
	}
	metrics.AvgPnLPct = metrics.AvgPnLPct / float64(metrics.Count)

	switch bias {
	case DealReviewSymbolBehaviorBiasNegative:
		metrics.SupportCount = metrics.Losses
		metrics.ContradictCount = metrics.Wins
	case DealReviewSymbolBehaviorBiasPositive:
		metrics.SupportCount = metrics.Wins
		metrics.ContradictCount = metrics.Losses
	default:
		metrics.SupportCount = maxInt(metrics.Wins, metrics.Losses)
		metrics.ContradictCount = minInt(metrics.Wins, metrics.Losses)
	}
	metrics.SupportRate = float64(metrics.SupportCount) / float64(metrics.Count)
	directionalPnL := 0.0
	switch bias {
	case DealReviewSymbolBehaviorBiasNegative:
		if metrics.AvgPnLPct < 0 {
			directionalPnL = clampDealReviewUnit(math.Abs(metrics.AvgPnLPct) / 1.5)
		}
	case DealReviewSymbolBehaviorBiasPositive:
		if metrics.AvgPnLPct > 0 {
			directionalPnL = clampDealReviewUnit(metrics.AvgPnLPct / 1.5)
		}
	default:
		directionalPnL = clampDealReviewUnit(math.Abs(metrics.AvgPnLPct) / 2.0)
	}
	metrics.SupportScore = clampDealReviewUnit((metrics.SupportRate * 0.75) + (directionalPnL * 0.25))
	return metrics
}

func computeDealReviewSymbolBehaviorDriftScore(bias string, validation, recent dealReviewSymbolBehaviorWindowMetrics) float64 {
	if validation.Count == 0 && recent.Count == 0 {
		return 0
	}
	score := 0.0
	if validation.Count > 0 && recent.Count > 0 {
		score += math.Abs(validation.SupportScore-recent.SupportScore) * 0.55
		score += clampDealReviewUnit(math.Abs(validation.AvgPnLPct-recent.AvgPnLPct)/2.0) * 0.25
	}
	if recent.Count >= dealReviewSymbolBehaviorRecentWindowMin && dealReviewSymbolBehaviorWindowOpposesBias(bias, recent) {
		score += 0.20
	}
	if validation.Count >= dealReviewSymbolBehaviorValidationMinSamples &&
		validation.SupportScore >= 0.60 &&
		recent.Count >= dealReviewSymbolBehaviorRecentWindowMin &&
		recent.SupportScore <= 0.35 {
		score += 0.20
	}
	return clampDealReviewUnit(score)
}

func classifyDealReviewSymbolBehaviorPriorStatus(
	total int,
	bias string,
	recencyWeight float64,
	validation dealReviewSymbolBehaviorWindowMetrics,
	recent dealReviewSymbolBehaviorWindowMetrics,
	driftScore float64,
	lastObservedAt time.Time,
	now time.Time,
) string {
	status := DealReviewSymbolBehaviorPriorStatusObserved
	if total >= dealReviewSymbolBehaviorCandidateThreshold {
		status = DealReviewSymbolBehaviorPriorStatusCandidate
	}
	if status == DealReviewSymbolBehaviorPriorStatusObserved {
		return status
	}
	if dealReviewSymbolBehaviorPriorLooksExpired(recencyWeight, lastObservedAt, now) {
		return DealReviewSymbolBehaviorPriorStatusExpired
	}
	if bias == DealReviewSymbolBehaviorBiasMixed {
		return status
	}

	validationReady := validation.Count >= dealReviewSymbolBehaviorValidationMinSamples
	recentReady := recent.Count >= dealReviewSymbolBehaviorRecentWindowMin
	validationStrong := validationReady &&
		validation.SupportScore >= 0.58 &&
		validation.SupportCount >= int(math.Ceil(float64(validation.Count)/2.0))
	validationWeak := validationReady &&
		validation.SupportScore <= 0.35 &&
		validation.ContradictCount >= validation.SupportCount
	recentWeak := recentReady &&
		recent.SupportScore <= 0.35 &&
		recent.ContradictCount >= recent.SupportCount

	if validationStrong {
		status = DealReviewSymbolBehaviorPriorStatusValidated
	}
	if validationWeak {
		return DealReviewSymbolBehaviorPriorStatusRejected
	}
	if status == DealReviewSymbolBehaviorPriorStatusValidated && recentWeak && driftScore >= 0.50 {
		return DealReviewSymbolBehaviorPriorStatusExpired
	}
	if status == DealReviewSymbolBehaviorPriorStatusValidated && recentReady && driftScore >= 0.65 {
		return DealReviewSymbolBehaviorPriorStatusExpired
	}
	return status
}

func buildDealReviewSymbolBehaviorValidationSummary(
	status string,
	bias string,
	trainingCount int,
	validation dealReviewSymbolBehaviorWindowMetrics,
	recent dealReviewSymbolBehaviorWindowMetrics,
	driftScore float64,
	lastObservedAt time.Time,
	now time.Time,
) string {
	if trainingCount == 0 && validation.Count == 0 && recent.Count == 0 {
		return ""
	}
	windowPrefix := fmt.Sprintf("Validation windows: %d training", trainingCount)
	if validation.Count > 0 {
		windowPrefix += fmt.Sprintf(" / %d holdout", validation.Count)
	}
	if recent.Count > 0 {
		windowPrefix += fmt.Sprintf(" / %d recent", recent.Count)
	}
	windowPrefix += "."

	if dealReviewSymbolBehaviorPriorLooksExpired(0, lastObservedAt, now) && !lastObservedAt.IsZero() {
		ageDays := int(now.Sub(lastObservedAt).Hours() / 24)
		return fmt.Sprintf("%s Last evidence is %d day(s) old, so this prior is treated as expired.", windowPrefix, ageDays)
	}
	if bias == DealReviewSymbolBehaviorBiasMixed {
		return windowPrefix + " Bias is still mixed, so the prior remains observational only."
	}
	if validation.Count < dealReviewSymbolBehaviorValidationMinSamples {
		return windowPrefix + " Not enough temporal holdout evidence yet."
	}

	switch status {
	case DealReviewSymbolBehaviorPriorStatusValidated:
		return fmt.Sprintf(
			"%s Holdout supported the %s prior in %d/%d cases (avg %+.2f%%). Recent support is %d/%d with drift %.0f%%.",
			windowPrefix,
			bias,
			validation.SupportCount,
			validation.Count,
			validation.AvgPnLPct,
			recent.SupportCount,
			recent.Count,
			driftScore*100,
		)
	case DealReviewSymbolBehaviorPriorStatusRejected:
		return fmt.Sprintf(
			"%s Holdout contradicted the %s prior in %d/%d cases (avg %+.2f%%), so the prior is rejected for now.",
			windowPrefix,
			bias,
			validation.ContradictCount,
			validation.Count,
			validation.AvgPnLPct,
		)
	case DealReviewSymbolBehaviorPriorStatusExpired:
		return fmt.Sprintf(
			"%s Historical holdout was no longer confirmed by the recent window (%d/%d support, avg %+.2f%%, drift %.0f%%), so the prior is expired.",
			windowPrefix,
			recent.SupportCount,
			recent.Count,
			recent.AvgPnLPct,
			driftScore*100,
		)
	default:
		return fmt.Sprintf(
			"%s Holdout support is still mixed at %d/%d supporting cases (avg %+.2f%%).",
			windowPrefix,
			validation.SupportCount,
			validation.Count,
			validation.AvgPnLPct,
		)
	}
}

func dealReviewSymbolBehaviorPriorLooksExpired(recencyWeight float64, lastObservedAt, now time.Time) bool {
	if !lastObservedAt.IsZero() && !now.IsZero() && now.Sub(lastObservedAt) >= 90*24*time.Hour {
		return true
	}
	return recencyWeight > 0 && recencyWeight < 0.12
}

func dealReviewSymbolBehaviorWindowOpposesBias(bias string, metrics dealReviewSymbolBehaviorWindowMetrics) bool {
	switch bias {
	case DealReviewSymbolBehaviorBiasNegative:
		return metrics.ContradictCount > metrics.SupportCount || metrics.AvgPnLPct > 0.10
	case DealReviewSymbolBehaviorBiasPositive:
		return metrics.ContradictCount > metrics.SupportCount || metrics.AvgPnLPct < -0.10
	default:
		return false
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func applyDealReviewSymbolBehaviorPriorValidationReport(prior *DealReviewSymbolBehaviorPrior) {
	if prior == nil {
		return
	}
	falsePositive := computeDealReviewSymbolBehaviorPriorFalsePositiveScore(prior)
	falseNegative := computeDealReviewSymbolBehaviorPriorFalseNegativeScore(prior)
	prior.FalsePositiveScore = falsePositive
	prior.FalseNegativeScore = falseNegative

	switch {
	case prior.Status == DealReviewSymbolBehaviorPriorStatusRejected || falsePositive >= 0.68:
		prior.ValidationLabel = DealReviewSymbolBehaviorValidationLabelFalsePositive
		prior.ValidationAlert = buildDealReviewSymbolBehaviorPriorFalsePositiveAlert(prior, falsePositive)
	case prior.Status == DealReviewSymbolBehaviorPriorStatusExpired || prior.DriftScore >= 0.45:
		if falseNegative >= 0.60 {
			prior.ValidationLabel = DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk
			prior.ValidationAlert = buildDealReviewSymbolBehaviorPriorFalseNegativeAlert(prior, falseNegative)
		} else {
			prior.ValidationLabel = DealReviewSymbolBehaviorValidationLabelDrifting
			prior.ValidationAlert = buildDealReviewSymbolBehaviorPriorDriftAlert(prior)
		}
	case prior.Status == DealReviewSymbolBehaviorPriorStatusValidated:
		prior.ValidationLabel = DealReviewSymbolBehaviorValidationLabelConfirmed
		prior.ValidationAlert = "Holdout and recent windows still support this learned edge."
	case prior.ValidationSampleCount < dealReviewSymbolBehaviorValidationMinSamples:
		prior.ValidationLabel = DealReviewSymbolBehaviorValidationLabelInsufficientEvidence
		prior.ValidationAlert = "Not enough holdout evidence yet to judge whether this learned edge really holds."
	default:
		prior.ValidationLabel = DealReviewSymbolBehaviorValidationLabelCandidate
		prior.ValidationAlert = "This prior is directional, but its validation windows are still mixed."
	}
}

func computeDealReviewSymbolBehaviorPriorFalsePositiveScore(prior *DealReviewSymbolBehaviorPrior) float64 {
	if prior == nil {
		return 0
	}
	score := 0.0
	if prior.ValidationSampleCount > 0 {
		score += (float64(prior.ValidationContradictCount) / float64(prior.ValidationSampleCount)) * 0.55
		score += dealReviewSymbolBehaviorOppositePnLStrength(prior.BehaviorBias, prior.ValidationAvgPnLPct) * 0.25
	}
	if prior.RecentSampleCount > 0 {
		score += (float64(prior.RecentContradictCount) / float64(prior.RecentSampleCount)) * 0.10
		score += dealReviewSymbolBehaviorOppositePnLStrength(prior.BehaviorBias, prior.RecentAvgPnLPct) * 0.10
	}
	if prior.Status == DealReviewSymbolBehaviorPriorStatusRejected {
		score += 0.15
	}
	if prior.Status == DealReviewSymbolBehaviorPriorStatusExpired {
		score += clampDealReviewUnit(prior.DriftScore) * 0.10
	}
	return clampDealReviewUnit(score)
}

func computeDealReviewSymbolBehaviorPriorFalseNegativeScore(prior *DealReviewSymbolBehaviorPrior) float64 {
	if prior == nil {
		return 0
	}
	score := 0.0
	if prior.RecentSampleCount > 0 {
		score += (float64(prior.RecentContradictCount) / float64(prior.RecentSampleCount)) * 0.55
		score += dealReviewSymbolBehaviorOppositePnLStrength(prior.BehaviorBias, prior.RecentAvgPnLPct) * 0.30
	}
	if prior.ValidationSampleCount > 0 {
		score += (float64(prior.ValidationContradictCount) / float64(prior.ValidationSampleCount)) * 0.10
		score += dealReviewSymbolBehaviorOppositePnLStrength(prior.BehaviorBias, prior.ValidationAvgPnLPct) * 0.05
	}
	if prior.Status == DealReviewSymbolBehaviorPriorStatusExpired || prior.Status == DealReviewSymbolBehaviorPriorStatusRejected {
		score += 0.10
	}
	score += clampDealReviewUnit(prior.DriftScore) * 0.10
	return clampDealReviewUnit(score)
}

func dealReviewSymbolBehaviorOppositePnLStrength(bias string, avgPnLPct float64) float64 {
	switch strings.TrimSpace(bias) {
	case DealReviewSymbolBehaviorBiasNegative:
		if avgPnLPct > 0 {
			return clampDealReviewUnit(avgPnLPct / 1.5)
		}
	case DealReviewSymbolBehaviorBiasPositive:
		if avgPnLPct < 0 {
			return clampDealReviewUnit(math.Abs(avgPnLPct) / 1.5)
		}
	}
	return 0
}

func buildDealReviewSymbolBehaviorPriorFalsePositiveAlert(prior *DealReviewSymbolBehaviorPrior, score float64) string {
	if prior == nil {
		return ""
	}
	return fmt.Sprintf(
		"Holdout or recent windows contradicted this prior strongly enough that the original %s edge now looks like a likely false positive (score %.0f%%).",
		prior.BehaviorBias,
		score*100,
	)
}

func buildDealReviewSymbolBehaviorPriorFalseNegativeAlert(prior *DealReviewSymbolBehaviorPrior, score float64) string {
	if prior == nil {
		return ""
	}
	opposite := "opposite"
	switch strings.TrimSpace(prior.BehaviorBias) {
	case DealReviewSymbolBehaviorBiasNegative:
		opposite = "positive"
	case DealReviewSymbolBehaviorBiasPositive:
		opposite = "negative"
	}
	return fmt.Sprintf(
		"Recent evidence is leaning toward the %s edge instead (score %.0f%%), so this prior may now hide a false-negative reverse setup.",
		opposite,
		score*100,
	)
}

func buildDealReviewSymbolBehaviorPriorDriftAlert(prior *DealReviewSymbolBehaviorPrior) string {
	if prior == nil {
		return ""
	}
	return fmt.Sprintf(
		"Recent evidence is drifting away from the earlier learned edge (drift %.0f%%); treat this prior as degraded until new confirmation appears.",
		prior.DriftScore*100,
	)
}

func BuildDealReviewSymbolBehaviorPriorReportingSummary(items []DealReviewSymbolBehaviorPrior) *DealReviewSymbolBehaviorPriorReportingSummary {
	summary := &DealReviewSymbolBehaviorPriorReportingSummary{
		TotalCount:   len(items),
		StatusCounts: map[string]int{},
		LabelCounts:  map[string]int{},
	}
	if len(items) == 0 {
		return summary
	}

	falsePositiveItems := make([]DealReviewSymbolBehaviorPrior, 0, len(items))
	falseNegativeItems := make([]DealReviewSymbolBehaviorPrior, 0, len(items))
	driftingItems := make([]DealReviewSymbolBehaviorPrior, 0, len(items))
	for idx := range items {
		prior := items[idx]
		applyDealReviewSymbolBehaviorPriorValidationReport(&prior)
		statusKey := strings.TrimSpace(prior.Status)
		if statusKey == "" {
			statusKey = "unknown"
		}
		labelKey := strings.TrimSpace(prior.ValidationLabel)
		if labelKey == "" {
			labelKey = "unknown"
		}
		summary.StatusCounts[statusKey]++
		summary.LabelCounts[labelKey]++
		switch prior.ValidationLabel {
		case DealReviewSymbolBehaviorValidationLabelConfirmed:
			summary.ConfirmedCount++
		case DealReviewSymbolBehaviorValidationLabelFalsePositive:
			summary.FalsePositiveCount++
		case DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk:
			summary.FalseNegativeRiskCount++
		case DealReviewSymbolBehaviorValidationLabelDrifting:
			summary.DriftingCount++
		case DealReviewSymbolBehaviorValidationLabelInsufficientEvidence:
			summary.InsufficientEvidenceCount++
		}
		if prior.FalsePositiveScore >= 0.45 {
			falsePositiveItems = append(falsePositiveItems, dealReviewSymbolBehaviorPriorForSummaryCard(prior))
		}
		if prior.ValidationLabel == DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk ||
			(prior.FalseNegativeScore >= 0.45 && prior.FalsePositiveScore < 0.75) {
			falseNegativeItems = append(falseNegativeItems, dealReviewSymbolBehaviorPriorForSummaryCard(prior))
		}
		if prior.DriftScore >= 0.35 {
			driftingItems = append(driftingItems, dealReviewSymbolBehaviorPriorForSummaryCard(prior))
		}
	}

	sort.SliceStable(falsePositiveItems, func(i, j int) bool {
		if falsePositiveItems[i].FalsePositiveScore == falsePositiveItems[j].FalsePositiveScore {
			return falsePositiveItems[i].CompositeScore > falsePositiveItems[j].CompositeScore
		}
		return falsePositiveItems[i].FalsePositiveScore > falsePositiveItems[j].FalsePositiveScore
	})
	sort.SliceStable(falseNegativeItems, func(i, j int) bool {
		if falseNegativeItems[i].FalseNegativeScore == falseNegativeItems[j].FalseNegativeScore {
			return falseNegativeItems[i].DriftScore > falseNegativeItems[j].DriftScore
		}
		return falseNegativeItems[i].FalseNegativeScore > falseNegativeItems[j].FalseNegativeScore
	})
	sort.SliceStable(driftingItems, func(i, j int) bool {
		if driftingItems[i].DriftScore == driftingItems[j].DriftScore {
			return driftingItems[i].RecentSupportScore < driftingItems[j].RecentSupportScore
		}
		return driftingItems[i].DriftScore > driftingItems[j].DriftScore
	})

	summary.TopFalsePositives = limitDealReviewSymbolBehaviorPriorSummaryItems(falsePositiveItems, 3)
	summary.TopFalseNegativeRisks = limitDealReviewSymbolBehaviorPriorSummaryItems(falseNegativeItems, 3)
	summary.TopDrifting = limitDealReviewSymbolBehaviorPriorSummaryItems(driftingItems, 3)
	summary.TopSignalClusters = buildDealReviewSymbolBehaviorClusterSummaries(items, 6)

	notes := make([]string, 0, 3)
	if summary.FalsePositiveCount > 0 {
		notes = append(notes, fmt.Sprintf("%d prior(s) now look like false positives and should not be trusted blindly.", summary.FalsePositiveCount))
	}
	if summary.FalseNegativeRiskCount > 0 {
		notes = append(notes, fmt.Sprintf("%d prior(s) now show reverse-edge risk, which may justify an opposite-direction follow-up scan.", summary.FalseNegativeRiskCount))
	}
	if summary.DriftingCount > 0 {
		notes = append(notes, fmt.Sprintf("%d prior(s) are drifting and need re-validation before they influence strategy changes.", summary.DriftingCount))
	}
	if len(summary.TopSignalClusters) > 0 {
		topCluster := summary.TopSignalClusters[0]
		notes = append(notes, fmt.Sprintf("Top normalized cluster %q spans %d prior(s), %d symbol(s), and %d closed deals.", topCluster.ClusterLabel, topCluster.PriorCount, topCluster.SymbolCount, topCluster.SampleCount))
	}
	summary.Notes = notes
	return summary
}

func limitDealReviewSymbolBehaviorPriorSummaryItems(items []DealReviewSymbolBehaviorPrior, limit int) []DealReviewSymbolBehaviorPrior {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	result := make([]DealReviewSymbolBehaviorPrior, 0, len(items))
	for _, item := range items {
		result = append(result, dealReviewSymbolBehaviorPriorForSummaryCard(item))
	}
	return result
}

func dealReviewSymbolBehaviorPriorForSummaryCard(prior DealReviewSymbolBehaviorPrior) DealReviewSymbolBehaviorPrior {
	prior.Evidence = nil
	prior.DecisionEvidence = nil
	prior.SignalTags = append([]string(nil), prior.SignalTags...)
	prior.SignalClusters = append([]string(nil), prior.SignalClusters...)
	return prior
}

func buildDealReviewSymbolBehaviorClusterSummaries(items []DealReviewSymbolBehaviorPrior, limit int) []DealReviewSymbolBehaviorClusterSummary {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	accumulators := map[string]*dealReviewSymbolBehaviorClusterAccumulator{}
	for idx := range items {
		prior := items[idx]
		applyDealReviewSymbolBehaviorPriorValidationReport(&prior)
		if len(prior.SignalClusters) == 0 {
			continue
		}
		weight := float64(maxInt(prior.SampleCount, 1))
		for _, cluster := range prior.SignalClusters {
			label := normalizeDealReviewSignalClusterLabel(cluster)
			if label == "" {
				continue
			}
			acc := accumulators[label]
			if acc == nil {
				acc = &dealReviewSymbolBehaviorClusterAccumulator{
					ClusterLabel: label,
					Symbols:      map[string]struct{}{},
					SymbolCounts: map[string]int{},
				}
				accumulators[label] = acc
			}
			acc.PriorCount++
			acc.SampleCount += prior.SampleCount
			acc.DecisionOpenCount += prior.DecisionOpenCount
			acc.WeightedPnLPctSum += prior.AvgPnLPct * weight
			acc.WeightedContradictionSum += prior.ContradictionScore * weight
			acc.WeightedCompositeSum += prior.CompositeScore * weight
			acc.WeightedValidationScoreSum += prior.ValidationSupportScore * weight
			symbol := strings.ToUpper(strings.TrimSpace(prior.Symbol))
			if symbol != "" {
				acc.Symbols[symbol] = struct{}{}
				acc.SymbolCounts[symbol] += prior.SampleCount
			}
			switch prior.ValidationLabel {
			case DealReviewSymbolBehaviorValidationLabelConfirmed:
				acc.ConfirmedCount++
			case DealReviewSymbolBehaviorValidationLabelFalsePositive:
				acc.FalsePositiveCount++
			case DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk:
				acc.FalseNegativeRiskCount++
			case DealReviewSymbolBehaviorValidationLabelDrifting:
				acc.DriftingCount++
			}
			switch prior.BehaviorBias {
			case DealReviewSymbolBehaviorBiasNegative:
				acc.NegativeBiasCount++
			case DealReviewSymbolBehaviorBiasPositive:
				acc.PositiveBiasCount++
			}
		}
	}
	if len(accumulators) == 0 {
		return nil
	}

	rollups := make([]DealReviewSymbolBehaviorClusterSummary, 0, len(accumulators))
	for _, acc := range accumulators {
		weight := float64(maxInt(acc.SampleCount, 1))
		rollups = append(rollups, DealReviewSymbolBehaviorClusterSummary{
			ClusterLabel:              acc.ClusterLabel,
			PriorCount:                acc.PriorCount,
			SymbolCount:               len(acc.Symbols),
			SampleCount:               acc.SampleCount,
			DecisionOpenCount:         acc.DecisionOpenCount,
			ConfirmedCount:            acc.ConfirmedCount,
			FalsePositiveCount:        acc.FalsePositiveCount,
			FalseNegativeRiskCount:    acc.FalseNegativeRiskCount,
			DriftingCount:             acc.DriftingCount,
			NegativeBiasCount:         acc.NegativeBiasCount,
			PositiveBiasCount:         acc.PositiveBiasCount,
			AvgPnLPct:                 acc.WeightedPnLPctSum / weight,
			AvgContradictionScore:     acc.WeightedContradictionSum / weight,
			AvgCompositeScore:         acc.WeightedCompositeSum / weight,
			AvgValidationSupportScore: acc.WeightedValidationScoreSum / weight,
			TopSymbols:                dealReviewSymbolBehaviorClusterTopSymbols(acc.SymbolCounts, 4),
		})
	}
	sort.SliceStable(rollups, func(i, j int) bool {
		if rollups[i].SampleCount == rollups[j].SampleCount {
			if rollups[i].PriorCount == rollups[j].PriorCount {
				if rollups[i].AvgContradictionScore == rollups[j].AvgContradictionScore {
					return rollups[i].ClusterLabel < rollups[j].ClusterLabel
				}
				return rollups[i].AvgContradictionScore > rollups[j].AvgContradictionScore
			}
			return rollups[i].PriorCount > rollups[j].PriorCount
		}
		return rollups[i].SampleCount > rollups[j].SampleCount
	})
	if len(rollups) > limit {
		rollups = rollups[:limit]
	}
	return rollups
}

func dealReviewSymbolBehaviorClusterTopSymbols(counts map[string]int, limit int) []string {
	if len(counts) == 0 || limit <= 0 {
		return nil
	}
	type pair struct {
		symbol string
		count  int
	}
	items := make([]pair, 0, len(counts))
	for symbol, count := range counts {
		if strings.TrimSpace(symbol) == "" || count <= 0 {
			continue
		}
		items = append(items, pair{symbol: symbol, count: count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].symbol < items[j].symbol
		}
		return items[i].count > items[j].count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.symbol)
	}
	return out
}
