package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DealReviewLearnedPatternLiveGuardEffectNoMatch            = "no_match"
	DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified = "matched_not_qualified"
	DealReviewLearnedPatternLiveGuardEffectMonitorOnly        = "monitor_only"
	DealReviewLearnedPatternLiveGuardEffectHardBlocked        = "hard_blocked"
)

type DealReviewLearnedPatternLiveGuardProbe struct {
	CycleNumber        int       `json:"cycle_number,omitempty"`
	DecisionTimestamp  time.Time `json:"decision_timestamp,omitempty"`
	Action             string    `json:"action,omitempty"`
	Symbol             string    `json:"symbol,omitempty"`
	Side               string    `json:"side,omitempty"`
	SelectionBucket    string    `json:"selection_bucket,omitempty"`
	TrendRegime        string    `json:"trend_regime,omitempty"`
	VolatilityRegime   string    `json:"volatility_regime,omitempty"`
	BTCStrengthRegime  string    `json:"btc_strength_regime,omitempty"`
	FundingRegime      string    `json:"funding_regime,omitempty"`
	OIRegime           string    `json:"oi_regime,omitempty"`
	SessionBucket      string    `json:"session_bucket,omitempty"`
	WeekdayBucket      string    `json:"weekday_bucket,omitempty"`
	VenueTier          string    `json:"venue_tier,omitempty"`
	LiquidityTier      string    `json:"liquidity_tier,omitempty"`
	SpreadBucket       string    `json:"spread_bucket,omitempty"`
	SlippageBucket     string    `json:"slippage_bucket,omitempty"`
	DecisionConfidence int       `json:"decision_confidence,omitempty"`
	DecisionReasoning  string    `json:"decision_reasoning,omitempty"`
}

type DealReviewLearnedPatternLiveGuardAssessment struct {
	Enabled          bool                      `json:"enabled"`
	Mode             string                    `json:"mode,omitempty"`
	Effect           string                    `json:"effect,omitempty"`
	Qualified        bool                      `json:"qualified,omitempty"`
	HardBlock        bool                      `json:"hard_block,omitempty"`
	Summary          string                    `json:"summary,omitempty"`
	BlockReason      string                    `json:"block_reason,omitempty"`
	MatchScore       float64                   `json:"match_score,omitempty"`
	MatchedPatternID string                    `json:"matched_pattern_id,omitempty"`
	MatchedPattern   *DealReviewLearnedPattern `json:"matched_pattern,omitempty"`
}

type DealReviewLearnedPatternLiveGuardEventFilter struct {
	Symbol string
	Side   string
	Effect string
	Limit  int
}

type DealReviewLearnedPatternLiveGuardEventSummary struct {
	TotalVisible            int       `json:"total_visible"`
	HardBlockedCount        int       `json:"hard_blocked_count"`
	MonitorOnlyCount        int       `json:"monitor_only_count"`
	MatchedUnqualifiedCount int       `json:"matched_unqualified_count"`
	NoMatchCount            int       `json:"no_match_count"`
	LatestDecisionTimestamp time.Time `json:"latest_decision_timestamp,omitempty"`
}

type DealReviewLearnedPatternLiveGuardEvent struct {
	ID                                   string    `gorm:"primaryKey" json:"id"`
	UserID                               string    `gorm:"column:user_id;not null;index:idx_pattern_live_guard_user_trader" json:"user_id"`
	TraderID                             string    `gorm:"column:trader_id;not null;index:idx_pattern_live_guard_user_trader;index:idx_pattern_live_guard_lookup" json:"trader_id"`
	CycleNumber                          int       `gorm:"column:cycle_number;default:0;index:idx_pattern_live_guard_lookup" json:"cycle_number"`
	DecisionTimestamp                    time.Time `gorm:"column:decision_timestamp;index:idx_pattern_live_guard_lookup" json:"decision_timestamp"`
	Action                               string    `gorm:"column:action;default:''" json:"action"`
	Symbol                               string    `gorm:"column:symbol;not null;index:idx_pattern_live_guard_lookup" json:"symbol"`
	Side                                 string    `gorm:"column:side;not null;index:idx_pattern_live_guard_lookup" json:"side"`
	SelectionBucket                      string    `gorm:"column:selection_bucket;default:''" json:"selection_bucket"`
	TrendRegime                          string    `gorm:"column:trend_regime;default:''" json:"trend_regime"`
	VolatilityRegime                     string    `gorm:"column:volatility_regime;default:''" json:"volatility_regime"`
	OIRegime                             string    `gorm:"column:oi_regime;default:''" json:"oi_regime"`
	PolicyMode                           string    `gorm:"column:policy_mode;default:''" json:"policy_mode"`
	Effect                               string    `gorm:"column:effect;default:'';index:idx_pattern_live_guard_effect" json:"effect"`
	DecisionConfidence                   int       `gorm:"column:decision_confidence;default:0" json:"decision_confidence"`
	MatchScore                           float64   `gorm:"column:match_score;default:0" json:"match_score"`
	MatchedPatternID                     string    `gorm:"column:matched_pattern_id;default:'';index:idx_pattern_live_guard_pattern" json:"matched_pattern_id"`
	MatchedPatternScopeType              string    `gorm:"column:matched_pattern_scope_type;default:''" json:"matched_pattern_scope_type"`
	MatchedPatternClass                  string    `gorm:"column:matched_pattern_class;default:''" json:"matched_pattern_class"`
	MatchedPatternSignature              string    `gorm:"column:matched_pattern_signature;type:text;default:''" json:"matched_pattern_signature"`
	MatchedPatternValidationLabel        string    `gorm:"column:matched_pattern_validation_label;default:''" json:"matched_pattern_validation_label"`
	MatchedPatternRecommendedUse         string    `gorm:"column:matched_pattern_recommended_use;default:''" json:"matched_pattern_recommended_use"`
	MatchedPatternSampleCount            int       `gorm:"column:matched_pattern_sample_count;default:0" json:"matched_pattern_sample_count"`
	MatchedPatternCompositeScore         float64   `gorm:"column:matched_pattern_composite_score;default:0" json:"matched_pattern_composite_score"`
	MatchedPatternConfidenceScore        float64   `gorm:"column:matched_pattern_confidence_score;default:0" json:"matched_pattern_confidence_score"`
	MatchedPatternValidationSupportScore float64   `gorm:"column:matched_pattern_validation_support_score;default:0" json:"matched_pattern_validation_support_score"`
	MatchedPatternFalsePositiveScore     float64   `gorm:"column:matched_pattern_false_positive_score;default:0" json:"matched_pattern_false_positive_score"`
	MatchedPatternDriftScore             float64   `gorm:"column:matched_pattern_drift_score;default:0" json:"matched_pattern_drift_score"`
	Summary                              string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	BlockReason                          string    `gorm:"column:block_reason;type:text;default:''" json:"block_reason"`
	CreatedAt                            time.Time `json:"created_at"`
	UpdatedAt                            time.Time `json:"updated_at"`
}

func (DealReviewLearnedPatternLiveGuardEvent) TableName() string {
	return "deal_review_learned_pattern_live_guard_events"
}

func BuildDealReviewLearnedPatternLiveGuardProbeFromDecisionRecord(record *DecisionRecord, action *DecisionAction) *DealReviewLearnedPatternLiveGuardProbe {
	if record == nil || action == nil || !isOpenDecisionAction(action.Action) {
		return nil
	}

	symbol := strings.ToUpper(strings.TrimSpace(action.Symbol))
	side := normalizeDealReviewSide(decisionActionSide(action.Action))
	if symbol == "" || side == "" {
		return nil
	}

	probe := &DealReviewLearnedPatternLiveGuardProbe{
		CycleNumber:        record.CycleNumber,
		DecisionTimestamp:  decisionActionTimestamp(record, action),
		Action:             strings.ToLower(strings.TrimSpace(action.Action)),
		Symbol:             symbol,
		Side:               side,
		DecisionConfidence: action.Confidence,
		DecisionReasoning:  strings.TrimSpace(action.Reasoning),
	}

	var candidateContext *DealReviewMarketContextSnapshot
	for _, candidate := range record.CandidateDetails {
		if strings.EqualFold(strings.TrimSpace(candidate.Symbol), symbol) {
			probe.SelectionBucket = strings.TrimSpace(candidate.SelectionBucket)
			candidateContext = candidate.MarketContext
			break
		}
	}

	actionContext := buildDealReviewMarketContextFromDecisionRecord(
		record,
		DealReviewStageOpen,
		symbol,
		side,
		probe.DecisionTimestamp,
	)
	context := learnedPatternLiveGuardMergedContext(actionContext, candidateContext)
	if context != nil {
		probe.TrendRegime = normalizeDealReviewSymbolPriorDimension(context.TrendRegime)
		probe.VolatilityRegime = normalizeDealReviewSymbolPriorDimension(context.VolatilityRegime)
		probe.BTCStrengthRegime = normalizeDealReviewSymbolPriorDimension(context.BTCStrengthRegime)
		probe.FundingRegime = normalizeDealReviewSymbolPriorDimension(context.FundingRegime)
		probe.OIRegime = normalizeDealReviewSymbolPriorDimension(context.OIRegime)
		probe.SessionBucket = normalizeDealReviewSymbolPriorDimension(context.SessionBucket)
		probe.WeekdayBucket = normalizeDealReviewSymbolPriorDimension(context.WeekdayBucket)
		probe.VenueTier = normalizeDealReviewSymbolPriorDimension(context.VenueTier)
		probe.LiquidityTier = normalizeDealReviewSymbolPriorDimension(context.LiquidityTier)
		probe.SpreadBucket = normalizeDealReviewSymbolPriorDimension(context.SpreadBucket)
		probe.SlippageBucket = normalizeDealReviewSymbolPriorDimension(context.SlippageBucket)
	}
	if probe.TrendRegime == "" {
		probe.TrendRegime = normalizeDealReviewSymbolPriorDimension(valueOrActionContext(action.MarketContext, candidateContext, "trend"))
	}
	if probe.VolatilityRegime == "" {
		probe.VolatilityRegime = normalizeDealReviewSymbolPriorDimension(valueOrActionContext(action.MarketContext, candidateContext, "volatility"))
	}
	if probe.OIRegime == "" {
		probe.OIRegime = normalizeDealReviewSymbolPriorDimension(valueOrActionContext(action.MarketContext, candidateContext, "oi"))
	}
	return probe
}

func (s *DealReviewStore) EvaluateLearnedPatternLiveGuard(
	userID string,
	traderID string,
	probe DealReviewLearnedPatternLiveGuardProbe,
	cfg LearnedPatternLiveGuardConfig,
) (*DealReviewLearnedPatternLiveGuardAssessment, error) {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	symbol := strings.ToUpper(strings.TrimSpace(probe.Symbol))
	side := normalizeDealReviewSide(probe.Side)
	action := strings.ToLower(strings.TrimSpace(probe.Action))
	if userID == "" || traderID == "" || symbol == "" || side == "" || !isOpenDecisionAction(action) {
		return nil, nil
	}

	effective := normalizeLearnedPatternLiveGuardConfig(cfg)
	if !effective.Enabled {
		return nil, nil
	}
	if _, err := s.RefreshLearnedPatternsIfStale(userID, traderID); err != nil {
		return nil, err
	}

	assessment := &DealReviewLearnedPatternLiveGuardAssessment{
		Enabled: true,
		Mode:    effective.Mode,
	}

	caseRec := buildDealReviewCaseFromLearnedPatternLiveGuardProbe(probe)
	matches, err := s.ListMatchingLearnedPatterns(userID, traderID, caseRec, 20)
	if err != nil {
		return nil, err
	}

	var bestMatch *DealReviewLearnedPattern
	bestMatchScore := 0.0
	var bestEligible *DealReviewLearnedPattern
	bestEligibleScore := 0.0

	for idx := range matches {
		pattern := matches[idx]
		if normalizeDealReviewLearnedPatternClass(pattern.PatternClass) != DealReviewLearnedPatternClassNegativeEdge {
			continue
		}
		score := pattern.MatchScore
		if score <= 0 {
			score = scoreDealReviewLearnedPatternMatch(caseRec, extractLearnedPatternLiveGuardFeatureSet(caseRec), &pattern)
		}
		if bestMatch == nil || score > bestMatchScore {
			current := pattern
			bestMatch = &current
			bestMatchScore = score
		}
		if reasons := dealReviewLearnedPatternLiveDisqualificationReasons(&pattern, effective, score); len(reasons) == 0 {
			if bestEligible == nil || score > bestEligibleScore {
				current := pattern
				bestEligible = &current
				bestEligibleScore = score
			}
		}
	}

	if bestMatch == nil {
		assessment.Effect = DealReviewLearnedPatternLiveGuardEffectNoMatch
		assessment.Summary = fmt.Sprintf(
			"No negative learned pattern matched %s %s strongly enough for live guard review.",
			symbol,
			side,
		)
		return assessment, nil
	}

	if bestEligible != nil {
		assessment.Qualified = true
		assessment.MatchScore = bestEligibleScore
		assessment.MatchedPattern = bestEligible
		assessment.MatchedPatternID = bestEligible.ID
		assessment.Summary = buildDealReviewLearnedPatternLiveQualifiedSummary(bestEligible, bestEligibleScore)
		if effective.Mode == LearnedPatternLiveGuardModeHardBlock {
			assessment.HardBlock = true
			assessment.Effect = DealReviewLearnedPatternLiveGuardEffectHardBlocked
			assessment.BlockReason = assessment.Summary
		} else {
			assessment.Effect = DealReviewLearnedPatternLiveGuardEffectMonitorOnly
		}
		return assessment, nil
	}

	assessment.Effect = DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified
	assessment.MatchScore = bestMatchScore
	assessment.MatchedPattern = bestMatch
	assessment.MatchedPatternID = bestMatch.ID
	assessment.Summary = buildDealReviewLearnedPatternLiveRejectedSummary(
		bestMatch,
		bestMatchScore,
		dealReviewLearnedPatternLiveDisqualificationReasons(bestMatch, effective, bestMatchScore),
	)
	return assessment, nil
}

func (s *DealReviewStore) RecordLearnedPatternLiveGuardEvent(
	userID string,
	traderID string,
	probe *DealReviewLearnedPatternLiveGuardProbe,
	assessment *DealReviewLearnedPatternLiveGuardAssessment,
) error {
	if probe == nil || assessment == nil {
		return nil
	}
	event := &DealReviewLearnedPatternLiveGuardEvent{
		ID:                 uuid.NewString(),
		UserID:             strings.TrimSpace(userID),
		TraderID:           strings.TrimSpace(traderID),
		CycleNumber:        probe.CycleNumber,
		DecisionTimestamp:  probe.DecisionTimestamp.UTC(),
		Action:             strings.TrimSpace(probe.Action),
		Symbol:             strings.ToUpper(strings.TrimSpace(probe.Symbol)),
		Side:               normalizeDealReviewSide(probe.Side),
		SelectionBucket:    strings.TrimSpace(probe.SelectionBucket),
		TrendRegime:        strings.TrimSpace(probe.TrendRegime),
		VolatilityRegime:   strings.TrimSpace(probe.VolatilityRegime),
		OIRegime:           strings.TrimSpace(probe.OIRegime),
		PolicyMode:         strings.TrimSpace(assessment.Mode),
		Effect:             strings.TrimSpace(assessment.Effect),
		DecisionConfidence: probe.DecisionConfidence,
		MatchScore:         assessment.MatchScore,
		Summary:            strings.TrimSpace(assessment.Summary),
		BlockReason:        strings.TrimSpace(assessment.BlockReason),
	}
	if pattern := assessment.MatchedPattern; pattern != nil {
		event.MatchedPatternID = strings.TrimSpace(pattern.ID)
		event.MatchedPatternScopeType = strings.TrimSpace(pattern.ScopeType)
		event.MatchedPatternClass = strings.TrimSpace(pattern.PatternClass)
		event.MatchedPatternSignature = strings.TrimSpace(pattern.PatternSignature)
		event.MatchedPatternValidationLabel = strings.TrimSpace(pattern.ValidationLabel)
		event.MatchedPatternRecommendedUse = strings.TrimSpace(pattern.RecommendedUse)
		event.MatchedPatternSampleCount = pattern.SampleCount
		event.MatchedPatternCompositeScore = pattern.CompositeScore
		event.MatchedPatternConfidenceScore = pattern.ConfidenceScore
		event.MatchedPatternValidationSupportScore = pattern.ValidationSupportScore
		event.MatchedPatternFalsePositiveScore = pattern.FalsePositiveScore
		event.MatchedPatternDriftScore = pattern.DriftScore
	}
	return s.db.Create(event).Error
}

func (s *DealReviewStore) ListLearnedPatternLiveGuardEvents(
	userID string,
	traderID string,
	filter DealReviewLearnedPatternLiveGuardEventFilter,
) ([]DealReviewLearnedPatternLiveGuardEvent, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}
	query := s.db.Model(&DealReviewLearnedPatternLiveGuardEvent{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID))
	if symbol := strings.ToUpper(strings.TrimSpace(filter.Symbol)); symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	if side := normalizeDealReviewSide(filter.Side); side != "" {
		query = query.Where("side = ?", side)
	}
	if effect := strings.TrimSpace(strings.ToLower(filter.Effect)); effect != "" {
		query = query.Where("effect = ?", effect)
	}
	var items []DealReviewLearnedPatternLiveGuardEvent
	if err := query.Order("decision_timestamp DESC, created_at DESC").Limit(filter.Limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func BuildDealReviewLearnedPatternLiveGuardEventSummary(items []DealReviewLearnedPatternLiveGuardEvent) *DealReviewLearnedPatternLiveGuardEventSummary {
	summary := &DealReviewLearnedPatternLiveGuardEventSummary{
		TotalVisible: len(items),
	}
	for _, item := range items {
		switch item.Effect {
		case DealReviewLearnedPatternLiveGuardEffectHardBlocked:
			summary.HardBlockedCount++
		case DealReviewLearnedPatternLiveGuardEffectMonitorOnly:
			summary.MonitorOnlyCount++
		case DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified:
			summary.MatchedUnqualifiedCount++
		case DealReviewLearnedPatternLiveGuardEffectNoMatch:
			summary.NoMatchCount++
		}
		if item.DecisionTimestamp.After(summary.LatestDecisionTimestamp) {
			summary.LatestDecisionTimestamp = item.DecisionTimestamp
		}
	}
	return summary
}

func buildDealReviewCaseFromLearnedPatternLiveGuardProbe(probe DealReviewLearnedPatternLiveGuardProbe) *DealReviewCase {
	return &DealReviewCase{
		Symbol:               strings.ToUpper(strings.TrimSpace(probe.Symbol)),
		Side:                 normalizeDealReviewSide(probe.Side),
		OpenSelectionBucket:  normalizeDealReviewSymbolPriorDimension(probe.SelectionBucket),
		OpenTrendRegime:      normalizeDealReviewSymbolPriorDimension(probe.TrendRegime),
		OpenVolatilityRegime: normalizeDealReviewSymbolPriorDimension(probe.VolatilityRegime),
		OpenBTCStrengthRegime: normalizeDealReviewSymbolPriorDimension(
			probe.BTCStrengthRegime,
		),
		OpenFundingRegime:  normalizeDealReviewSymbolPriorDimension(probe.FundingRegime),
		OpenOIRegime:       normalizeDealReviewSymbolPriorDimension(probe.OIRegime),
		OpenSessionBucket:  normalizeDealReviewSymbolPriorDimension(probe.SessionBucket),
		OpenWeekdayBucket:  normalizeDealReviewSymbolPriorDimension(probe.WeekdayBucket),
		OpenVenueTier:      normalizeDealReviewSymbolPriorDimension(probe.VenueTier),
		OpenLiquidityTier:  normalizeDealReviewSymbolPriorDimension(probe.LiquidityTier),
		OpenSpreadBucket:   normalizeDealReviewSymbolPriorDimension(probe.SpreadBucket),
		OpenSlippageBucket: normalizeDealReviewSymbolPriorDimension(probe.SlippageBucket),
		OpenConfidence:     probe.DecisionConfidence,
	}
}

func extractLearnedPatternLiveGuardFeatureSet(caseRec *DealReviewCase) []string {
	features, _ := extractDealReviewLearnedPatternFeatures(caseRec)
	return features
}

func dealReviewLearnedPatternLiveDisqualificationReasons(
	pattern *DealReviewLearnedPattern,
	cfg LearnedPatternLiveGuardConfig,
	matchScore float64,
) []string {
	if pattern == nil {
		return []string{"missing_pattern"}
	}
	reasons := make([]string, 0, 10)
	appendReason := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range reasons {
			if existing == value {
				return
			}
		}
		reasons = append(reasons, value)
	}

	if normalizeDealReviewLearnedPatternClass(pattern.PatternClass) != DealReviewLearnedPatternClassNegativeEdge {
		appendReason("pattern_not_negative")
	}
	if normalizeDealReviewLearnedPatternValidationLabel(pattern.ValidationLabel) == DealReviewLearnedPatternValidationLabelExpired {
		appendReason("pattern_expired")
	}
	if strings.EqualFold(strings.TrimSpace(pattern.RecommendedUse), DealReviewLearnedPatternRecommendedUseExpiredIgnore) {
		appendReason("recommended_use=expired_do_not_use")
	}
	if cfg.RequireConfirmedLabel || cfg.Mode == LearnedPatternLiveGuardModeHardBlock {
		if !strings.EqualFold(strings.TrimSpace(pattern.ValidationLabel), DealReviewLearnedPatternValidationLabelConfirmed) {
			appendReason("pattern_not_confirmed")
		}
	}
	if pattern.SampleCount < cfg.MinSampleCount {
		appendReason(fmt.Sprintf("sample_count<%d", cfg.MinSampleCount))
	}
	if pattern.CompositeScore < cfg.MinCompositeScore {
		appendReason(fmt.Sprintf("composite<%.2f", cfg.MinCompositeScore))
	}
	if pattern.ConfidenceScore < cfg.MinConfidenceScore {
		appendReason(fmt.Sprintf("confidence<%.2f", cfg.MinConfidenceScore))
	}
	if matchScore < cfg.MinMatchScore {
		appendReason(fmt.Sprintf("match<%.2f", cfg.MinMatchScore))
	}
	if pattern.FalsePositiveScore > cfg.MaxFalsePositiveScore {
		appendReason(fmt.Sprintf("false_positive>%.2f", cfg.MaxFalsePositiveScore))
	}
	if pattern.DriftScore > cfg.MaxDriftScore {
		appendReason(fmt.Sprintf("drift>%.2f", cfg.MaxDriftScore))
	}
	if pattern.ValidationSupportScore < cfg.MinValidationSupportScore {
		appendReason(fmt.Sprintf("validation_support<%.2f", cfg.MinValidationSupportScore))
	}
	return reasons
}

func buildDealReviewLearnedPatternLiveQualifiedSummary(pattern *DealReviewLearnedPattern, matchScore float64) string {
	if pattern == nil {
		return ""
	}
	return fmt.Sprintf(
		"Matched confirmed negative learned pattern %s for %s %s (match %.2f, composite %.2f, confidence %.2f, validation support %.2f, samples %d, false-positive %.2f, drift %.2f).",
		dealReviewLearnedPatternRuntimeKey(pattern),
		strings.ToUpper(strings.TrimSpace(pattern.Symbol)),
		normalizeDealReviewSide(pattern.Side),
		matchScore,
		pattern.CompositeScore,
		pattern.ConfidenceScore,
		pattern.ValidationSupportScore,
		pattern.SampleCount,
		pattern.FalsePositiveScore,
		pattern.DriftScore,
	)
}

func buildDealReviewLearnedPatternLiveRejectedSummary(pattern *DealReviewLearnedPattern, matchScore float64, reasons []string) string {
	if pattern == nil {
		return ""
	}
	summary := fmt.Sprintf(
		"Matched learned pattern %s for %s %s, but live guard kept it non-blocking (match %.2f).",
		dealReviewLearnedPatternRuntimeKey(pattern),
		strings.ToUpper(strings.TrimSpace(pattern.Symbol)),
		normalizeDealReviewSide(pattern.Side),
		matchScore,
	)
	if len(reasons) > 0 {
		summary += " Reasons: " + strings.Join(reasons, ", ") + "."
	}
	return summary
}

func dealReviewLearnedPatternRuntimeKey(pattern *DealReviewLearnedPattern) string {
	if pattern == nil {
		return ""
	}
	scope := normalizeDealReviewLearnedPatternScopeType(pattern.ScopeType)
	if scope == "" {
		scope = DealReviewLearnedPatternScopeTraderLocal
	}
	signature := strings.TrimSpace(pattern.PatternSignature)
	if signature == "" {
		signature = strings.Join(pattern.FeatureSet, " + ")
	}
	if strings.TrimSpace(pattern.Symbol) != "" {
		return fmt.Sprintf("%s:%s:%s:%s", scope, strings.ToUpper(strings.TrimSpace(pattern.Symbol)), normalizeDealReviewSide(pattern.Side), signature)
	}
	return fmt.Sprintf("%s:%s:%s", scope, normalizeDealReviewSide(pattern.Side), signature)
}

func learnedPatternLiveGuardMergedContext(actionContext, candidateContext *DealReviewMarketContextSnapshot) *DealReviewMarketContextSnapshot {
	if actionContext == nil && candidateContext == nil {
		return nil
	}
	if actionContext == nil {
		return candidateContext
	}
	if candidateContext == nil {
		return actionContext
	}
	merged := *candidateContext
	if strings.TrimSpace(actionContext.TrendRegime) != "" {
		merged.TrendRegime = actionContext.TrendRegime
	}
	if strings.TrimSpace(actionContext.VolatilityRegime) != "" {
		merged.VolatilityRegime = actionContext.VolatilityRegime
	}
	if strings.TrimSpace(actionContext.BTCStrengthRegime) != "" {
		merged.BTCStrengthRegime = actionContext.BTCStrengthRegime
	}
	if strings.TrimSpace(actionContext.FundingRegime) != "" {
		merged.FundingRegime = actionContext.FundingRegime
	}
	if strings.TrimSpace(actionContext.OIRegime) != "" {
		merged.OIRegime = actionContext.OIRegime
	}
	if strings.TrimSpace(actionContext.SessionBucket) != "" {
		merged.SessionBucket = actionContext.SessionBucket
	}
	if strings.TrimSpace(actionContext.WeekdayBucket) != "" {
		merged.WeekdayBucket = actionContext.WeekdayBucket
	}
	if strings.TrimSpace(actionContext.VenueTier) != "" {
		merged.VenueTier = actionContext.VenueTier
	}
	if strings.TrimSpace(actionContext.LiquidityTier) != "" {
		merged.LiquidityTier = actionContext.LiquidityTier
	}
	if strings.TrimSpace(actionContext.SpreadBucket) != "" {
		merged.SpreadBucket = actionContext.SpreadBucket
	}
	if strings.TrimSpace(actionContext.SlippageBucket) != "" {
		merged.SlippageBucket = actionContext.SlippageBucket
	}
	return &merged
}

func learnedPatternLiveGuardCoalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
