package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DealReviewSymbolBehaviorLiveGuardEffectNoMatch            = "no_match"
	DealReviewSymbolBehaviorLiveGuardEffectMatchedUnqualified = "matched_not_qualified"
	DealReviewSymbolBehaviorLiveGuardEffectMonitorOnly        = "monitor_only"
	DealReviewSymbolBehaviorLiveGuardEffectHardBlocked        = "hard_blocked"

	dealReviewSymbolBehaviorLiveGuardClusterTightenMinPriors        = 3
	dealReviewSymbolBehaviorLiveGuardClusterTightenMinSymbols       = 3
	dealReviewSymbolBehaviorLiveGuardClusterTightenMinSamples       = 18
	dealReviewSymbolBehaviorLiveGuardClusterTightenMinSupportScore  = 0.55
	dealReviewSymbolBehaviorLiveGuardClusterTightenMinContradiction = 0.55
	dealReviewSymbolBehaviorLiveGuardClusterConfidenceBump          = 0.03
	dealReviewSymbolBehaviorLiveGuardClusterMatchBump               = 0.05
	dealReviewSymbolBehaviorLiveGuardClusterContradictionBump       = 0.05
	dealReviewSymbolBehaviorLiveGuardClusterFalsePositiveTighten    = 0.03
	dealReviewSymbolBehaviorLiveGuardClusterDriftTighten            = 0.03
)

type DealReviewSymbolBehaviorLiveGuardProbe struct {
	CycleNumber        int       `json:"cycle_number,omitempty"`
	DecisionTimestamp  time.Time `json:"decision_timestamp,omitempty"`
	Action             string    `json:"action,omitempty"`
	Symbol             string    `json:"symbol,omitempty"`
	Side               string    `json:"side,omitempty"`
	SelectionBucket    string    `json:"selection_bucket,omitempty"`
	TrendRegime        string    `json:"trend_regime,omitempty"`
	VolatilityRegime   string    `json:"volatility_regime,omitempty"`
	OIRegime           string    `json:"oi_regime,omitempty"`
	DecisionConfidence int       `json:"decision_confidence,omitempty"`
	DecisionReasoning  string    `json:"decision_reasoning,omitempty"`
}

type DealReviewSymbolBehaviorLiveGuardAssessment struct {
	Enabled          bool                           `json:"enabled"`
	Mode             string                         `json:"mode,omitempty"`
	Effect           string                         `json:"effect,omitempty"`
	Qualified        bool                           `json:"qualified,omitempty"`
	HardBlock        bool                           `json:"hard_block,omitempty"`
	Summary          string                         `json:"summary,omitempty"`
	BlockReason      string                         `json:"block_reason,omitempty"`
	MatchScore       float64                        `json:"match_score,omitempty"`
	MatchedPriorKey  string                         `json:"matched_prior_key,omitempty"`
	MatchedPrior     *DealReviewSymbolBehaviorPrior `json:"matched_prior,omitempty"`
	SelectionBucket  string                         `json:"selection_bucket,omitempty"`
	TrendRegime      string                         `json:"trend_regime,omitempty"`
	VolatilityRegime string                         `json:"volatility_regime,omitempty"`
	OIRegime         string                         `json:"oi_regime,omitempty"`
}

type dealReviewSymbolBehaviorLiveGuardThresholdProfile struct {
	Config            SymbolBehaviorLiveGuardConfig
	TightenedClusters []string
	Summary           string
}

type DealReviewSymbolBehaviorLiveGuardEventFilter struct {
	Symbol string
	Side   string
	Effect string
	Limit  int
}

type DealReviewSymbolBehaviorLiveGuardEventSummary struct {
	TotalVisible            int       `json:"total_visible"`
	HardBlockedCount        int       `json:"hard_blocked_count"`
	MonitorOnlyCount        int       `json:"monitor_only_count"`
	MatchedUnqualifiedCount int       `json:"matched_unqualified_count"`
	NoMatchCount            int       `json:"no_match_count"`
	LatestDecisionTimestamp time.Time `json:"latest_decision_timestamp,omitempty"`
}

type DealReviewSymbolBehaviorLiveGuardEvent struct {
	ID                        string    `gorm:"primaryKey" json:"id"`
	UserID                    string    `gorm:"column:user_id;not null;index:idx_symbol_behavior_live_guard_user_trader" json:"user_id"`
	TraderID                  string    `gorm:"column:trader_id;not null;index:idx_symbol_behavior_live_guard_user_trader;index:idx_symbol_behavior_live_guard_lookup" json:"trader_id"`
	CycleNumber               int       `gorm:"column:cycle_number;default:0;index:idx_symbol_behavior_live_guard_lookup" json:"cycle_number"`
	DecisionTimestamp         time.Time `gorm:"column:decision_timestamp;index:idx_symbol_behavior_live_guard_lookup" json:"decision_timestamp"`
	Action                    string    `gorm:"column:action;default:''" json:"action"`
	Symbol                    string    `gorm:"column:symbol;not null;index:idx_symbol_behavior_live_guard_lookup" json:"symbol"`
	Side                      string    `gorm:"column:side;not null;index:idx_symbol_behavior_live_guard_lookup" json:"side"`
	SelectionBucket           string    `gorm:"column:selection_bucket;default:''" json:"selection_bucket"`
	TrendRegime               string    `gorm:"column:trend_regime;default:''" json:"trend_regime"`
	VolatilityRegime          string    `gorm:"column:volatility_regime;default:''" json:"volatility_regime"`
	OIRegime                  string    `gorm:"column:oi_regime;default:''" json:"oi_regime"`
	PolicyMode                string    `gorm:"column:policy_mode;default:''" json:"policy_mode"`
	Effect                    string    `gorm:"column:effect;default:'';index:idx_symbol_behavior_live_guard_effect" json:"effect"`
	DecisionConfidence        int       `gorm:"column:decision_confidence;default:0" json:"decision_confidence"`
	MatchScore                float64   `gorm:"column:match_score;default:0" json:"match_score"`
	MatchedPriorID            string    `gorm:"column:matched_prior_id;default:'';index:idx_symbol_behavior_live_guard_prior" json:"matched_prior_id"`
	MatchedPriorKey           string    `gorm:"column:matched_prior_key;default:''" json:"matched_prior_key"`
	MatchedPriorStatus        string    `gorm:"column:matched_prior_status;default:''" json:"matched_prior_status"`
	MatchedPriorValidation    string    `gorm:"column:matched_prior_validation_label;default:''" json:"matched_prior_validation_label"`
	MatchedPriorBias          string    `gorm:"column:matched_prior_bias;default:''" json:"matched_prior_bias"`
	MatchedPriorSampleCount   int       `gorm:"column:matched_prior_sample_count;default:0" json:"matched_prior_sample_count"`
	MatchedPriorConfidence    float64   `gorm:"column:matched_prior_confidence_score;default:0" json:"matched_prior_confidence_score"`
	MatchedPriorDriftScore    float64   `gorm:"column:matched_prior_drift_score;default:0" json:"matched_prior_drift_score"`
	MatchedPriorFalsePositive float64   `gorm:"column:matched_prior_false_positive_score;default:0" json:"matched_prior_false_positive_score"`
	MatchedPriorContradiction float64   `gorm:"column:matched_prior_contradiction_score;default:0" json:"matched_prior_contradiction_score"`
	Summary                   string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	BlockReason               string    `gorm:"column:block_reason;type:text;default:''" json:"block_reason"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

func (DealReviewSymbolBehaviorLiveGuardEvent) TableName() string {
	return "deal_review_symbol_behavior_live_guard_events"
}

func BuildDealReviewSymbolBehaviorLiveGuardProbeFromDecisionRecord(record *DecisionRecord, action *DecisionAction) *DealReviewSymbolBehaviorLiveGuardProbe {
	if record == nil || action == nil || !isOpenDecisionAction(action.Action) {
		return nil
	}

	symbol := strings.ToUpper(strings.TrimSpace(action.Symbol))
	side := normalizeDealReviewSide(decisionActionSide(action.Action))
	if symbol == "" || side == "" {
		return nil
	}

	probe := &DealReviewSymbolBehaviorLiveGuardProbe{
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

	promptContext := buildDealReviewMarketContextFromDecisionRecord(
		record,
		DealReviewStageOpen,
		symbol,
		side,
		probe.DecisionTimestamp,
	)
	context := candidateContext
	if promptContext != nil {
		context = promptContext
	}

	probe.TrendRegime = normalizeDealReviewSymbolPriorDimension(valueOrActionContext(action.MarketContext, context, "trend"))
	probe.VolatilityRegime = normalizeDealReviewSymbolPriorDimension(valueOrActionContext(action.MarketContext, context, "volatility"))
	probe.OIRegime = normalizeDealReviewSymbolPriorDimension(valueOrActionContext(action.MarketContext, context, "oi"))

	return probe
}

func (s *DealReviewStore) EvaluateSymbolBehaviorLiveGuard(
	userID string,
	traderID string,
	probe DealReviewSymbolBehaviorLiveGuardProbe,
	cfg SymbolBehaviorLiveGuardConfig,
) (*DealReviewSymbolBehaviorLiveGuardAssessment, error) {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	symbol := strings.ToUpper(strings.TrimSpace(probe.Symbol))
	side := normalizeDealReviewSide(probe.Side)
	action := strings.ToLower(strings.TrimSpace(probe.Action))
	if userID == "" || traderID == "" || symbol == "" || side == "" || !isOpenDecisionAction(action) {
		return nil, nil
	}

	effective := normalizeSymbolBehaviorLiveGuardConfig(cfg)
	if !effective.Enabled {
		return nil, nil
	}

	assessment := &DealReviewSymbolBehaviorLiveGuardAssessment{
		Enabled:          true,
		Mode:             effective.Mode,
		SelectionBucket:  normalizeDealReviewSymbolPriorDimension(probe.SelectionBucket),
		TrendRegime:      normalizeDealReviewSymbolPriorDimension(probe.TrendRegime),
		VolatilityRegime: normalizeDealReviewSymbolPriorDimension(probe.VolatilityRegime),
		OIRegime:         normalizeDealReviewSymbolPriorDimension(probe.OIRegime),
	}

	priors, err := s.ListSymbolBehaviorPriors(userID, traderID, DealReviewSymbolBehaviorPriorFilter{
		Symbol: symbol,
		Side:   side,
		Limit:  20,
	})
	if err != nil {
		return nil, err
	}
	clusterSummaryIndex, err := s.listSymbolBehaviorLiveGuardClusterSummaryIndex(userID, traderID, side)
	if err != nil {
		return nil, err
	}

	caseRec := &DealReviewCase{
		Symbol:               symbol,
		Side:                 side,
		OpenSelectionBucket:  assessment.SelectionBucket,
		OpenTrendRegime:      assessment.TrendRegime,
		OpenVolatilityRegime: assessment.VolatilityRegime,
		OpenOIRegime:         assessment.OIRegime,
	}

	var bestMatch *DealReviewSymbolBehaviorPrior
	bestMatchScore := 0.0
	bestMatchProfile := dealReviewSymbolBehaviorLiveGuardThresholdProfile{Config: effective}
	var bestEligible *DealReviewSymbolBehaviorPrior
	bestEligibleScore := 0.0
	bestEligibleProfile := dealReviewSymbolBehaviorLiveGuardThresholdProfile{Config: effective}

	for idx := range priors {
		prior := priors[idx]
		if !strings.EqualFold(strings.TrimSpace(prior.BehaviorBias), DealReviewSymbolBehaviorBiasNegative) {
			continue
		}
		score := scoreDealReviewSymbolBehaviorPriorMatch(caseRec, &prior)
		profile := buildDealReviewSymbolBehaviorLiveGuardThresholdProfile(&prior, effective, clusterSummaryIndex)
		if bestMatch == nil || score > bestMatchScore {
			current := prior
			bestMatch = &current
			bestMatchScore = score
			bestMatchProfile = profile
		}
		if reasons := dealReviewSymbolBehaviorLiveDisqualificationReasons(&prior, profile.Config, score); len(reasons) == 0 {
			if bestEligible == nil || score > bestEligibleScore {
				current := prior
				bestEligible = &current
				bestEligibleScore = score
				bestEligibleProfile = profile
			}
		}
	}

	if bestMatch == nil {
		assessment.Effect = DealReviewSymbolBehaviorLiveGuardEffectNoMatch
		assessment.Summary = fmt.Sprintf(
			"No negative learned symbol prior matched %s %s strongly enough for live guard review.",
			symbol,
			side,
		)
		return assessment, nil
	}

	if bestEligible != nil {
		assessment.Qualified = true
		assessment.MatchScore = bestEligibleScore
		assessment.MatchedPrior = bestEligible
		assessment.MatchedPriorKey = dealReviewSymbolBehaviorPriorRuntimeKey(bestEligible)
		assessment.Summary = buildDealReviewSymbolBehaviorLiveQualifiedSummary(bestEligible, bestEligibleScore, bestEligibleProfile)
		if effective.Mode == SymbolBehaviorLiveGuardModeHardBlock {
			assessment.HardBlock = true
			assessment.Effect = DealReviewSymbolBehaviorLiveGuardEffectHardBlocked
			assessment.BlockReason = assessment.Summary
		} else {
			assessment.Effect = DealReviewSymbolBehaviorLiveGuardEffectMonitorOnly
		}
		return assessment, nil
	}

	assessment.Effect = DealReviewSymbolBehaviorLiveGuardEffectMatchedUnqualified
	assessment.MatchScore = bestMatchScore
	assessment.MatchedPrior = bestMatch
	assessment.MatchedPriorKey = dealReviewSymbolBehaviorPriorRuntimeKey(bestMatch)
	assessment.Summary = buildDealReviewSymbolBehaviorLiveRejectedSummary(
		bestMatch,
		bestMatchScore,
		bestMatchProfile,
		dealReviewSymbolBehaviorLiveDisqualificationReasons(bestMatch, bestMatchProfile.Config, bestMatchScore),
	)
	return assessment, nil
}

func (s *DealReviewStore) listSymbolBehaviorLiveGuardClusterSummaryIndex(
	userID string,
	traderID string,
	side string,
) (map[string]DealReviewSymbolBehaviorClusterSummary, error) {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	side = normalizeDealReviewSide(side)
	if userID == "" || traderID == "" || side == "" {
		return map[string]DealReviewSymbolBehaviorClusterSummary{}, nil
	}

	var items []DealReviewSymbolBehaviorPrior
	if err := s.db.
		Model(&DealReviewSymbolBehaviorPrior{}).
		Where("user_id = ? AND trader_id = ? AND side = ?", userID, traderID, side).
		Order("composite_score DESC, sample_count DESC, last_observed_at DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	for idx := range items {
		hydrateDealReviewSymbolBehaviorPrior(&items[idx])
	}

	rollups := buildDealReviewSymbolBehaviorClusterSummaries(items, maxInt(len(items)*4, 64))
	index := make(map[string]DealReviewSymbolBehaviorClusterSummary, len(rollups))
	for _, item := range rollups {
		label := normalizeDealReviewSignalClusterLabel(item.ClusterLabel)
		if label == "" {
			continue
		}
		index[label] = item
	}
	return index, nil
}

func buildDealReviewSymbolBehaviorLiveGuardThresholdProfile(
	prior *DealReviewSymbolBehaviorPrior,
	baseCfg SymbolBehaviorLiveGuardConfig,
	clusterSummaryIndex map[string]DealReviewSymbolBehaviorClusterSummary,
) dealReviewSymbolBehaviorLiveGuardThresholdProfile {
	profile := dealReviewSymbolBehaviorLiveGuardThresholdProfile{
		Config: baseCfg,
	}
	if prior == nil || len(prior.SignalClusters) == 0 || len(clusterSummaryIndex) == 0 {
		return profile
	}

	tightenedClusters := make([]string, 0, len(prior.SignalClusters))
	for _, rawCluster := range prior.SignalClusters {
		cluster := normalizeDealReviewSignalClusterLabel(rawCluster)
		if cluster == "" {
			continue
		}
		summary, ok := clusterSummaryIndex[cluster]
		if !ok || !isDealReviewSymbolBehaviorLiveGuardMatureCluster(summary) {
			continue
		}
		alreadyIncluded := false
		for _, existing := range tightenedClusters {
			if existing == cluster {
				alreadyIncluded = true
				break
			}
		}
		if !alreadyIncluded {
			tightenedClusters = append(tightenedClusters, cluster)
		}
	}
	if len(tightenedClusters) == 0 {
		return profile
	}

	profile.TightenedClusters = tightenedClusters
	profile.Config.MinConfidenceScore = minFloat(0.98, baseCfg.MinConfidenceScore+dealReviewSymbolBehaviorLiveGuardClusterConfidenceBump)
	profile.Config.MinMatchScore = minFloat(0.98, baseCfg.MinMatchScore+dealReviewSymbolBehaviorLiveGuardClusterMatchBump)
	profile.Config.MinContradictionScore = minFloat(0.98, baseCfg.MinContradictionScore+dealReviewSymbolBehaviorLiveGuardClusterContradictionBump)
	profile.Config.MaxFalsePositiveScore = minFloat(baseCfg.MaxFalsePositiveScore, maxFloat(0.05, baseCfg.MaxFalsePositiveScore-dealReviewSymbolBehaviorLiveGuardClusterFalsePositiveTighten))
	profile.Config.MaxDriftScore = minFloat(baseCfg.MaxDriftScore, maxFloat(0.05, baseCfg.MaxDriftScore-dealReviewSymbolBehaviorLiveGuardClusterDriftTighten))
	profile.Summary = fmt.Sprintf(
		"Mature cluster tightening active for %s: confidence>=%.2f, match>=%.2f, contradiction>=%.2f, false-positive<=%.2f, drift<=%.2f.",
		strings.Join(limitDealReviewSymbolBehaviorLiveGuardClusterLabels(tightenedClusters, 3), ", "),
		profile.Config.MinConfidenceScore,
		profile.Config.MinMatchScore,
		profile.Config.MinContradictionScore,
		profile.Config.MaxFalsePositiveScore,
		profile.Config.MaxDriftScore,
	)
	return profile
}

func isDealReviewSymbolBehaviorLiveGuardMatureCluster(summary DealReviewSymbolBehaviorClusterSummary) bool {
	if summary.PriorCount < dealReviewSymbolBehaviorLiveGuardClusterTightenMinPriors {
		return false
	}
	if summary.SymbolCount < dealReviewSymbolBehaviorLiveGuardClusterTightenMinSymbols {
		return false
	}
	if summary.SampleCount < dealReviewSymbolBehaviorLiveGuardClusterTightenMinSamples {
		return false
	}
	if summary.AvgValidationSupportScore < dealReviewSymbolBehaviorLiveGuardClusterTightenMinSupportScore {
		return false
	}
	if summary.AvgContradictionScore < dealReviewSymbolBehaviorLiveGuardClusterTightenMinContradiction {
		return false
	}
	if summary.DriftingCount*2 > summary.PriorCount {
		return false
	}
	return summary.FalsePositiveCount > 0 ||
		summary.NegativeBiasCount > 0 && summary.NegativeBiasCount >= summary.PositiveBiasCount
}

func limitDealReviewSymbolBehaviorLiveGuardClusterLabels(items []string, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		label := normalizeDealReviewSignalClusterLabel(item)
		if label == "" {
			continue
		}
		out = append(out, label)
	}
	return out
}

func (s *DealReviewStore) RecordSymbolBehaviorLiveGuardEvent(
	userID string,
	traderID string,
	probe *DealReviewSymbolBehaviorLiveGuardProbe,
	assessment *DealReviewSymbolBehaviorLiveGuardAssessment,
) error {
	if probe == nil || assessment == nil || !assessment.Enabled {
		return nil
	}

	event := &DealReviewSymbolBehaviorLiveGuardEvent{
		ID:                 uuid.NewString(),
		UserID:             strings.TrimSpace(userID),
		TraderID:           strings.TrimSpace(traderID),
		CycleNumber:        probe.CycleNumber,
		DecisionTimestamp:  probe.DecisionTimestamp.UTC(),
		Action:             strings.ToLower(strings.TrimSpace(probe.Action)),
		Symbol:             strings.ToUpper(strings.TrimSpace(probe.Symbol)),
		Side:               normalizeDealReviewSide(probe.Side),
		SelectionBucket:    normalizeDealReviewSymbolPriorDimension(probe.SelectionBucket),
		TrendRegime:        normalizeDealReviewSymbolPriorDimension(probe.TrendRegime),
		VolatilityRegime:   normalizeDealReviewSymbolPriorDimension(probe.VolatilityRegime),
		OIRegime:           normalizeDealReviewSymbolPriorDimension(probe.OIRegime),
		PolicyMode:         strings.TrimSpace(assessment.Mode),
		Effect:             strings.TrimSpace(assessment.Effect),
		DecisionConfidence: probe.DecisionConfidence,
		MatchScore:         assessment.MatchScore,
		Summary:            strings.TrimSpace(assessment.Summary),
		BlockReason:        strings.TrimSpace(assessment.BlockReason),
	}
	if event.DecisionTimestamp.IsZero() {
		event.DecisionTimestamp = time.Now().UTC()
	}
	if assessment.MatchedPrior != nil {
		event.MatchedPriorID = strings.TrimSpace(assessment.MatchedPrior.ID)
		event.MatchedPriorKey = strings.TrimSpace(assessment.MatchedPriorKey)
		event.MatchedPriorStatus = strings.TrimSpace(assessment.MatchedPrior.Status)
		event.MatchedPriorValidation = strings.TrimSpace(assessment.MatchedPrior.ValidationLabel)
		event.MatchedPriorBias = strings.TrimSpace(assessment.MatchedPrior.BehaviorBias)
		event.MatchedPriorSampleCount = assessment.MatchedPrior.SampleCount
		event.MatchedPriorConfidence = assessment.MatchedPrior.ConfidenceScore
		event.MatchedPriorDriftScore = assessment.MatchedPrior.DriftScore
		event.MatchedPriorFalsePositive = assessment.MatchedPrior.FalsePositiveScore
		event.MatchedPriorContradiction = assessment.MatchedPrior.ContradictionScore
	}
	return s.db.Create(event).Error
}

func (s *DealReviewStore) ListSymbolBehaviorLiveGuardEvents(
	userID string,
	traderID string,
	filter DealReviewSymbolBehaviorLiveGuardEventFilter,
) ([]DealReviewSymbolBehaviorLiveGuardEvent, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}

	query := s.db.Model(&DealReviewSymbolBehaviorLiveGuardEvent{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID))

	if symbol := strings.ToUpper(strings.TrimSpace(filter.Symbol)); symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	if side := normalizeDealReviewSide(filter.Side); side != "" {
		query = query.Where("side = ?", side)
	}
	if effect := strings.ToLower(strings.TrimSpace(filter.Effect)); effect != "" {
		query = query.Where("lower(effect) = ?", effect)
	}

	var items []DealReviewSymbolBehaviorLiveGuardEvent
	if err := query.
		Order("decision_timestamp DESC, created_at DESC").
		Limit(filter.Limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func BuildDealReviewSymbolBehaviorLiveGuardEventSummary(items []DealReviewSymbolBehaviorLiveGuardEvent) *DealReviewSymbolBehaviorLiveGuardEventSummary {
	summary := &DealReviewSymbolBehaviorLiveGuardEventSummary{
		TotalVisible: len(items),
	}
	for _, item := range items {
		switch strings.TrimSpace(item.Effect) {
		case DealReviewSymbolBehaviorLiveGuardEffectHardBlocked:
			summary.HardBlockedCount++
		case DealReviewSymbolBehaviorLiveGuardEffectMonitorOnly:
			summary.MonitorOnlyCount++
		case DealReviewSymbolBehaviorLiveGuardEffectMatchedUnqualified:
			summary.MatchedUnqualifiedCount++
		case DealReviewSymbolBehaviorLiveGuardEffectNoMatch:
			summary.NoMatchCount++
		}
		if summary.LatestDecisionTimestamp.IsZero() || item.DecisionTimestamp.After(summary.LatestDecisionTimestamp) {
			summary.LatestDecisionTimestamp = item.DecisionTimestamp
		}
	}
	return summary
}

func dealReviewSymbolBehaviorLiveDisqualificationReasons(
	prior *DealReviewSymbolBehaviorPrior,
	cfg SymbolBehaviorLiveGuardConfig,
	matchScore float64,
) []string {
	if prior == nil {
		return []string{"missing_prior"}
	}
	reasons := make([]string, 0, 8)
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

	if !strings.EqualFold(strings.TrimSpace(prior.BehaviorBias), DealReviewSymbolBehaviorBiasNegative) {
		appendReason("prior_not_negative")
	}
	if cfg.RequireConfirmedLabel || cfg.Mode == SymbolBehaviorLiveGuardModeHardBlock {
		if !strings.EqualFold(strings.TrimSpace(prior.ValidationLabel), DealReviewSymbolBehaviorValidationLabelConfirmed) {
			appendReason("prior_not_confirmed")
		}
	}
	if prior.SampleCount < cfg.MinSampleCount {
		appendReason(fmt.Sprintf("sample_count<%d", cfg.MinSampleCount))
	}
	if prior.ConfidenceScore < cfg.MinConfidenceScore {
		appendReason(fmt.Sprintf("confidence<%.2f", cfg.MinConfidenceScore))
	}
	if matchScore < cfg.MinMatchScore {
		appendReason(fmt.Sprintf("match<%.2f", cfg.MinMatchScore))
	}
	if prior.FalsePositiveScore > cfg.MaxFalsePositiveScore {
		appendReason(fmt.Sprintf("false_positive>%.2f", cfg.MaxFalsePositiveScore))
	}
	if prior.DriftScore > cfg.MaxDriftScore {
		appendReason(fmt.Sprintf("drift>%.2f", cfg.MaxDriftScore))
	}
	if prior.ContradictionScore < cfg.MinContradictionScore {
		appendReason(fmt.Sprintf("contradiction<%.2f", cfg.MinContradictionScore))
	}
	return reasons
}

func buildDealReviewSymbolBehaviorLiveQualifiedSummary(
	prior *DealReviewSymbolBehaviorPrior,
	matchScore float64,
	profile dealReviewSymbolBehaviorLiveGuardThresholdProfile,
) string {
	if prior == nil {
		return ""
	}
	summary := fmt.Sprintf(
		"Matched confirmed negative symbol prior %s for %s %s (match %.2f, confidence %.2f, samples %d, false-positive %.2f, drift %.2f).",
		dealReviewSymbolBehaviorPriorRuntimeKey(prior),
		strings.ToUpper(strings.TrimSpace(prior.Symbol)),
		normalizeDealReviewSide(prior.Side),
		matchScore,
		prior.ConfidenceScore,
		prior.SampleCount,
		prior.FalsePositiveScore,
		prior.DriftScore,
	)
	if strings.TrimSpace(profile.Summary) != "" {
		summary += " " + strings.TrimSpace(profile.Summary)
	}
	return summary
}

func buildDealReviewSymbolBehaviorLiveRejectedSummary(
	prior *DealReviewSymbolBehaviorPrior,
	matchScore float64,
	profile dealReviewSymbolBehaviorLiveGuardThresholdProfile,
	reasons []string,
) string {
	if prior == nil {
		return ""
	}
	summary := fmt.Sprintf(
		"Matched symbol prior %s for %s %s, but live guard kept it non-blocking (match %.2f).",
		dealReviewSymbolBehaviorPriorRuntimeKey(prior),
		strings.ToUpper(strings.TrimSpace(prior.Symbol)),
		normalizeDealReviewSide(prior.Side),
		matchScore,
	)
	if strings.TrimSpace(profile.Summary) != "" {
		summary += " " + strings.TrimSpace(profile.Summary)
	}
	if len(reasons) > 0 {
		summary += " Reasons: " + strings.Join(reasons, ", ") + "."
	}
	return summary
}

func dealReviewSymbolBehaviorPriorRuntimeKey(prior *DealReviewSymbolBehaviorPrior) string {
	if prior == nil {
		return ""
	}
	key, _ := buildDealReviewSymbolBehaviorPriorKey(
		prior.Symbol,
		prior.Side,
		prior.OpenSelectionBucket,
		prior.OpenTrendRegime,
		prior.OpenVolatilityRegime,
		prior.OpenOIRegime,
	)
	return key
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
