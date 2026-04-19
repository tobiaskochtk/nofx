package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DealReviewLearnedPatternScopeTraderLocal = "trader_local"
	DealReviewLearnedPatternScopeSymbol      = "symbol"

	DealReviewLearnedPatternClassPositiveEdge = "positive_edge"
	DealReviewLearnedPatternClassNegativeEdge = "negative_edge"

	DealReviewLearnedPatternValidationLabelConfirmed            = "confirmed"
	DealReviewLearnedPatternValidationLabelCandidate            = "candidate"
	DealReviewLearnedPatternValidationLabelInsufficientEvidence = "insufficient_evidence"
	DealReviewLearnedPatternValidationLabelFalsePositive        = "false_positive"
	DealReviewLearnedPatternValidationLabelReverseRisk          = "reverse_risk"
	DealReviewLearnedPatternValidationLabelDrifting             = "drifting"
	DealReviewLearnedPatternValidationLabelExpired              = "expired"

	DealReviewLearnedPatternRecommendedUseReviewHint    = "review_hint"
	DealReviewLearnedPatternRecommendedUsePromptHint    = "prompt_hint"
	DealReviewLearnedPatternRecommendedUseConfigCand    = "config_candidate"
	DealReviewLearnedPatternRecommendedUseMonitorOnly   = "monitor_only"
	DealReviewLearnedPatternRecommendedUseExpiredIgnore = "expired_do_not_use"

	dealReviewLearnedPatternFeatureLimit      = 16
	dealReviewLearnedPatternEvidenceLimit     = 6
	dealReviewLearnedPatternMaxOrder          = 2
	dealReviewLearnedPatternRefreshInterval   = 30 * time.Minute
	dealReviewLearnedPatternObservedThreshold = 3
)

type DealReviewPatternFeatureRecord struct {
	ID                 int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID             string    `gorm:"column:user_id;not null;index:idx_deal_review_pattern_features_user_trader" json:"user_id"`
	TraderID           string    `gorm:"column:trader_id;not null;index:idx_deal_review_pattern_features_user_trader;index:idx_deal_review_pattern_features_lookup" json:"trader_id"`
	CaseID             string    `gorm:"column:case_id;not null;uniqueIndex:idx_deal_review_pattern_features_case" json:"case_id"`
	PositionID         int64     `gorm:"column:position_id;default:0;index:idx_deal_review_pattern_features_lookup" json:"position_id"`
	Symbol             string    `gorm:"column:symbol;not null;index:idx_deal_review_pattern_features_lookup" json:"symbol"`
	Side               string    `gorm:"column:side;not null;index:idx_deal_review_pattern_features_lookup" json:"side"`
	Outcome            string    `gorm:"column:outcome;default:'';index:idx_deal_review_pattern_features_outcome" json:"outcome"`
	OpenConfidence     int       `gorm:"column:open_confidence;default:0" json:"open_confidence"`
	RealizedPnL        float64   `gorm:"column:realized_pnl;default:0" json:"realized_pnl"`
	RealizedPnLPct     float64   `gorm:"column:realized_pnl_pct;default:0" json:"realized_pnl_pct"`
	MaxMFEPct          float64   `gorm:"column:max_mfe_pct;default:0" json:"max_mfe_pct"`
	MaxMAEPct          float64   `gorm:"column:max_mae_pct;default:0" json:"max_mae_pct"`
	ProfitGivenBackPct float64   `gorm:"column:profit_given_back_pct;default:0" json:"profit_given_back_pct"`
	HoldDurationMs     int64     `gorm:"column:hold_duration_ms;default:0" json:"hold_duration_ms"`
	FeatureCount       int       `gorm:"column:feature_count;default:0" json:"feature_count"`
	FeaturesJSON       string    `gorm:"column:features_json;type:text;default:'[]'" json:"-"`
	QualityFlagsJSON   string    `gorm:"column:quality_flags_json;type:text;default:'[]'" json:"-"`
	ObservedAt         time.Time `gorm:"column:observed_at;index:idx_deal_review_pattern_features_observed" json:"observed_at"`
	BuiltAt            time.Time `gorm:"column:built_at;index:idx_deal_review_pattern_features_built" json:"built_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	Features           []string  `gorm:"-" json:"features,omitempty"`
	QualityFlags       []string  `gorm:"-" json:"quality_flags,omitempty"`
}

func (DealReviewPatternFeatureRecord) TableName() string { return "deal_review_pattern_features" }

type DealReviewLearnedPatternEvidence struct {
	CaseID         string  `json:"case_id"`
	PositionID     int64   `json:"position_id"`
	Symbol         string  `json:"symbol"`
	Side           string  `json:"side"`
	Outcome        string  `json:"outcome"`
	RealizedPnL    float64 `json:"realized_pnl"`
	RealizedPnLPct float64 `json:"realized_pnl_pct"`
	HoldDurationMs int64   `json:"hold_duration_ms"`
	ObservedAt     string  `json:"observed_at,omitempty"`
}

type DealReviewLearnedPattern struct {
	ID                        string                             `gorm:"primaryKey" json:"id"`
	UserID                    string                             `gorm:"column:user_id;not null;index:idx_deal_review_learned_patterns_user_trader" json:"user_id"`
	TraderID                  string                             `gorm:"column:trader_id;not null;index:idx_deal_review_learned_patterns_user_trader;index:idx_deal_review_learned_patterns_lookup" json:"trader_id"`
	ScopeType                 string                             `gorm:"column:scope_type;default:trader_local;index:idx_deal_review_learned_patterns_scope" json:"scope_type"`
	ScopeKey                  string                             `gorm:"column:scope_key;default:'';index:idx_deal_review_learned_patterns_scope" json:"scope_key"`
	Symbol                    string                             `gorm:"column:symbol;default:'';index:idx_deal_review_learned_patterns_lookup" json:"symbol"`
	Side                      string                             `gorm:"column:side;not null;index:idx_deal_review_learned_patterns_lookup" json:"side"`
	PatternClass              string                             `gorm:"column:pattern_class;default:'';index:idx_deal_review_learned_patterns_class" json:"pattern_class"`
	Status                    string                             `gorm:"column:status;default:observed;index:idx_deal_review_learned_patterns_status" json:"status"`
	ValidationLabel           string                             `gorm:"column:validation_label;default:'';index:idx_deal_review_learned_patterns_validation" json:"validation_label"`
	RecommendedUse            string                             `gorm:"column:recommended_use;default:review_hint" json:"recommended_use"`
	PatternSignature          string                             `gorm:"column:pattern_signature;type:text;default:'';index:idx_deal_review_learned_patterns_signature" json:"pattern_signature"`
	RegimeSignature           string                             `gorm:"column:regime_signature;type:text;default:''" json:"regime_signature"`
	PatternOrder              int                                `gorm:"column:pattern_order;default:1" json:"pattern_order"`
	FeatureCount              int                                `gorm:"column:feature_count;default:0" json:"feature_count"`
	FeatureSetJSON            string                             `gorm:"column:feature_set_json;type:text;default:'[]'" json:"-"`
	SampleCount               int                                `gorm:"column:sample_count;default:0" json:"sample_count"`
	WinningDeals              int                                `gorm:"column:winning_deals;default:0" json:"winning_deals"`
	LosingDeals               int                                `gorm:"column:losing_deals;default:0" json:"losing_deals"`
	FlatDeals                 int                                `gorm:"column:flat_deals;default:0" json:"flat_deals"`
	SupportCount              int                                `gorm:"column:support_count;default:0" json:"support_count"`
	ContradictCount           int                                `gorm:"column:contradict_count;default:0" json:"contradict_count"`
	WinRate                   float64                            `gorm:"column:win_rate;default:0" json:"win_rate"`
	LossRate                  float64                            `gorm:"column:loss_rate;default:0" json:"loss_rate"`
	NetPnL                    float64                            `gorm:"column:net_pnl;default:0" json:"net_pnl"`
	AvgPnL                    float64                            `gorm:"column:avg_pnl;default:0" json:"avg_pnl"`
	AvgPnLPct                 float64                            `gorm:"column:avg_pnl_pct;default:0" json:"avg_pnl_pct"`
	Expectancy                float64                            `gorm:"column:expectancy;default:0" json:"expectancy"`
	AvgMFEPct                 float64                            `gorm:"column:avg_mfe_pct;default:0" json:"avg_mfe_pct"`
	AvgMAEPct                 float64                            `gorm:"column:avg_mae_pct;default:0" json:"avg_mae_pct"`
	GiveBackRate              float64                            `gorm:"column:give_back_rate;default:0" json:"give_back_rate"`
	AvgGiveBackPct            float64                            `gorm:"column:avg_give_back_pct;default:0" json:"avg_give_back_pct"`
	AvgHoldMs                 int64                              `gorm:"column:avg_hold_ms;default:0" json:"avg_hold_ms"`
	BaselineWinRate           float64                            `gorm:"column:baseline_win_rate;default:0" json:"baseline_win_rate"`
	BaselineAvgPnLPct         float64                            `gorm:"column:baseline_avg_pnl_pct;default:0" json:"baseline_avg_pnl_pct"`
	LiftWinRate               float64                            `gorm:"column:lift_win_rate;default:0" json:"lift_win_rate"`
	LiftAvgPnLPct             float64                            `gorm:"column:lift_avg_pnl_pct;default:0" json:"lift_avg_pnl_pct"`
	TrainingSampleCount       int                                `gorm:"column:training_sample_count;default:0" json:"training_sample_count"`
	ValidationSampleCount     int                                `gorm:"column:validation_sample_count;default:0" json:"validation_sample_count"`
	ValidationSupportCount    int                                `gorm:"column:validation_support_count;default:0" json:"validation_support_count"`
	ValidationContradictCount int                                `gorm:"column:validation_contradict_count;default:0" json:"validation_contradict_count"`
	ValidationAvgPnLPct       float64                            `gorm:"column:validation_avg_pnl_pct;default:0" json:"validation_avg_pnl_pct"`
	ValidationSupportScore    float64                            `gorm:"column:validation_support_score;default:0" json:"validation_support_score"`
	RecentSampleCount         int                                `gorm:"column:recent_sample_count;default:0" json:"recent_sample_count"`
	RecentSupportCount        int                                `gorm:"column:recent_support_count;default:0" json:"recent_support_count"`
	RecentContradictCount     int                                `gorm:"column:recent_contradict_count;default:0" json:"recent_contradict_count"`
	RecentAvgPnLPct           float64                            `gorm:"column:recent_avg_pnl_pct;default:0" json:"recent_avg_pnl_pct"`
	RecentSupportScore        float64                            `gorm:"column:recent_support_score;default:0" json:"recent_support_score"`
	ConfidenceScore           float64                            `gorm:"column:confidence_score;default:0" json:"confidence_score"`
	StabilityScore            float64                            `gorm:"column:stability_score;default:0" json:"stability_score"`
	DriftScore                float64                            `gorm:"column:drift_score;default:0" json:"drift_score"`
	RecencyWeight             float64                            `gorm:"column:recency_weight;default:0" json:"recency_weight"`
	CompositeScore            float64                            `gorm:"column:composite_score;default:0;index:idx_deal_review_learned_patterns_score" json:"composite_score"`
	FalsePositiveScore        float64                            `gorm:"column:false_positive_score;default:0" json:"false_positive_score"`
	ReverseRiskScore          float64                            `gorm:"column:reverse_risk_score;default:0" json:"reverse_risk_score"`
	Summary                   string                             `gorm:"column:summary;type:text;default:''" json:"summary"`
	ValidationAlert           string                             `gorm:"column:validation_alert;type:text;default:''" json:"validation_alert"`
	EvidenceJSON              string                             `gorm:"column:evidence_json;type:text;default:'[]'" json:"-"`
	FirstObservedAt           time.Time                          `gorm:"column:first_observed_at" json:"first_observed_at"`
	LastObservedAt            time.Time                          `gorm:"column:last_observed_at;index:idx_deal_review_learned_patterns_observed" json:"last_observed_at"`
	BuiltAt                   time.Time                          `gorm:"column:built_at;index:idx_deal_review_learned_patterns_built" json:"built_at"`
	CreatedAt                 time.Time                          `json:"created_at"`
	UpdatedAt                 time.Time                          `json:"updated_at"`
	FeatureSet                []string                           `gorm:"-" json:"feature_set,omitempty"`
	Evidence                  []DealReviewLearnedPatternEvidence `gorm:"-" json:"evidence,omitempty"`
	MatchScore                float64                            `gorm:"-" json:"match_score,omitempty"`
}

func (DealReviewLearnedPattern) TableName() string { return "deal_review_learned_patterns" }

type DealReviewLearnedPatternFilter struct {
	PatternID       string
	Symbol          string
	Side            string
	ScopeType       string
	PatternClass    string
	ValidationLabel string
	Feature         string
	Limit           int
}

type DealReviewLearnedPatternRefreshResult struct {
	Rebuilt      bool      `json:"rebuilt"`
	FeatureCount int       `json:"feature_count"`
	PatternCount int       `json:"pattern_count"`
	GeneratedAt  time.Time `json:"generated_at"`
}

type DealReviewLearnedPatternReportingSummary struct {
	TotalCount          int                        `json:"total_count"`
	PositiveCount       int                        `json:"positive_count"`
	NegativeCount       int                        `json:"negative_count"`
	ConfirmedCount      int                        `json:"confirmed_count"`
	CandidateCount      int                        `json:"candidate_count"`
	FalsePositiveCount  int                        `json:"false_positive_count"`
	ReverseRiskCount    int                        `json:"reverse_risk_count"`
	DriftingCount       int                        `json:"drifting_count"`
	ExpiredCount        int                        `json:"expired_count"`
	ClassCounts         map[string]int             `json:"class_counts,omitempty"`
	LabelCounts         map[string]int             `json:"label_counts,omitempty"`
	TopPositivePatterns []DealReviewLearnedPattern `json:"top_positive_patterns,omitempty"`
	TopNegativePatterns []DealReviewLearnedPattern `json:"top_negative_patterns,omitempty"`
	TopSymbolOverrides  []DealReviewLearnedPattern `json:"top_symbol_overrides,omitempty"`
	Notes               []string                   `json:"notes,omitempty"`
}

type dealReviewLearnedPatternAggregate struct {
	ScopeType       string
	ScopeKey        string
	Symbol          string
	Side            string
	PatternOrder    int
	FeatureSet      []string
	Signature       string
	RegimeSignature string
	Total           int
	Wins            int
	Losses          int
	Flats           int
	NetPnL          float64
	NetPnLPct       float64
	SumMFEPct       float64
	SumMAEPct       float64
	SumGiveBackPct  float64
	GiveBackCount   int
	SumHoldMs       int64
	ObservedCases   []dealReviewSymbolBehaviorObservedCase
	Evidence        []DealReviewLearnedPatternEvidence
	FirstObservedAt time.Time
	LastObservedAt  time.Time
}

type dealReviewLearnedPatternBaseline struct {
	Count        int
	Wins         int
	Losses       int
	Flats        int
	NetPnLPctSum float64
}

func (b dealReviewLearnedPatternBaseline) WinRate() float64 {
	if b.Count <= 0 {
		return 0
	}
	return float64(b.Wins) / float64(b.Count)
}

func (b dealReviewLearnedPatternBaseline) AvgPnLPct() float64 {
	if b.Count <= 0 {
		return 0
	}
	return b.NetPnLPctSum / float64(b.Count)
}

func (s *DealReviewStore) RefreshLearnedPatternsIfStale(userID, traderID string) (*DealReviewLearnedPatternRefreshResult, error) {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" {
		return &DealReviewLearnedPatternRefreshResult{}, nil
	}

	var latestCaseUpdatedRaw sql.NullString
	if err := s.db.Model(&DealReviewCase{}).
		Where("user_id = ? AND trader_id = ? AND status = ?", userID, traderID, DealReviewCaseStatusClosed).
		Select("CAST(MAX(updated_at) AS TEXT)").Scan(&latestCaseUpdatedRaw).Error; err != nil {
		return nil, err
	}
	latestCaseUpdated := parseDealReviewAggregateTime(latestCaseUpdatedRaw.String)

	var latestBuiltRaw sql.NullString
	if err := s.db.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ?", userID, traderID).
		Select("CAST(MAX(built_at) AS TEXT)").Scan(&latestBuiltRaw).Error; err != nil {
		return nil, err
	}
	latestBuilt := parseDealReviewAggregateTime(latestBuiltRaw.String)

	needsRefresh := latestBuilt.IsZero()
	if !needsRefresh && !latestCaseUpdated.IsZero() && latestCaseUpdated.After(latestBuilt) {
		needsRefresh = true
	}
	if !needsRefresh && time.Since(latestBuilt) >= dealReviewLearnedPatternRefreshInterval {
		needsRefresh = true
	}
	if !needsRefresh {
		featureCount, err := s.countLearnedPatternFeatures(userID, traderID)
		if err != nil {
			return nil, err
		}
		patternCount, err := s.countLearnedPatterns(userID, traderID)
		if err != nil {
			return nil, err
		}
		return &DealReviewLearnedPatternRefreshResult{
			Rebuilt:      false,
			FeatureCount: featureCount,
			PatternCount: patternCount,
			GeneratedAt:  latestBuilt,
		}, nil
	}
	return s.RebuildLearnedPatterns(userID, traderID)
}

func (s *DealReviewStore) RebuildLearnedPatterns(userID, traderID string) (*DealReviewLearnedPatternRefreshResult, error) {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	now := time.Now().UTC()

	var cases []DealReviewCase
	if err := s.db.
		Where("user_id = ? AND trader_id = ? AND status = ?", userID, traderID, DealReviewCaseStatusClosed).
		Order("exit_time_ms DESC, updated_at DESC").
		Find(&cases).Error; err != nil {
		return nil, err
	}

	featureRows := make([]DealReviewPatternFeatureRecord, 0, len(cases))
	patterns := make([]DealReviewLearnedPattern, 0)
	if len(cases) > 0 {
		featureRows, patterns = buildDealReviewLearnedPatterns(cases, userID, traderID, now)
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&DealReviewPatternFeatureRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&DealReviewLearnedPattern{}).Error; err != nil {
			return err
		}
		if len(featureRows) > 0 {
			if err := dealReviewCreateInBatches(tx, featureRows); err != nil {
				return err
			}
		}
		if len(patterns) > 0 {
			if err := dealReviewCreateInBatches(tx, patterns); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &DealReviewLearnedPatternRefreshResult{
		Rebuilt:      true,
		FeatureCount: len(featureRows),
		PatternCount: len(patterns),
		GeneratedAt:  now,
	}, nil
}

func (s *DealReviewStore) ListLearnedPatterns(userID, traderID string, filter DealReviewLearnedPatternFilter) ([]DealReviewLearnedPattern, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 24
	}
	query := s.db.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID))

	if patternID := strings.TrimSpace(filter.PatternID); patternID != "" {
		query = query.Where("id = ?", patternID)
	}
	if symbol := strings.ToUpper(strings.TrimSpace(filter.Symbol)); symbol != "" {
		query = query.Where("symbol = ? OR (scope_type = ? AND symbol = '')", symbol, DealReviewLearnedPatternScopeTraderLocal)
	}
	if side := normalizeDealReviewSide(filter.Side); side != "" {
		query = query.Where("side = ?", side)
	}
	if scopeType := normalizeDealReviewLearnedPatternScopeType(filter.ScopeType); scopeType != "" {
		query = query.Where("scope_type = ?", scopeType)
	}
	if class := normalizeDealReviewLearnedPatternClass(filter.PatternClass); class != "" {
		query = query.Where("pattern_class = ?", class)
	}
	if label := normalizeDealReviewLearnedPatternValidationLabel(filter.ValidationLabel); label != "" {
		query = query.Where("validation_label = ?", label)
	}
	if feature := normalizeDealReviewPatternFeatureFilterToken(filter.Feature); feature != "" {
		pattern := fmt.Sprintf("%%\"%s\"%%", feature)
		query = query.Where("feature_set_json LIKE ?", pattern)
	}

	var items []DealReviewLearnedPattern
	if err := query.Order("composite_score DESC, sample_count DESC, last_observed_at DESC").Limit(filter.Limit).Find(&items).Error; err != nil {
		return nil, err
	}
	for idx := range items {
		hydrateDealReviewLearnedPattern(&items[idx])
	}
	return items, nil
}

func (s *DealReviewStore) ListMatchingLearnedPatterns(userID, traderID string, caseRec *DealReviewCase, limit int) ([]DealReviewLearnedPattern, error) {
	if caseRec == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 6
	}
	featureSet, _ := extractDealReviewLearnedPatternFeatures(caseRec)
	if len(featureSet) == 0 {
		return nil, nil
	}
	featureLookup := make(map[string]struct{}, len(featureSet))
	for _, feature := range featureSet {
		featureLookup[feature] = struct{}{}
	}

	var items []DealReviewLearnedPattern
	if err := s.db.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ? AND side = ? AND (scope_type = ? OR (scope_type = ? AND symbol = ?))",
			strings.TrimSpace(userID),
			strings.TrimSpace(traderID),
			normalizeDealReviewSide(caseRec.Side),
			DealReviewLearnedPatternScopeTraderLocal,
			DealReviewLearnedPatternScopeSymbol,
			strings.ToUpper(strings.TrimSpace(caseRec.Symbol)),
		).
		Order("composite_score DESC, sample_count DESC, last_observed_at DESC").
		Limit(200).
		Find(&items).Error; err != nil {
		return nil, err
	}

	matched := make([]DealReviewLearnedPattern, 0, limit)
	for idx := range items {
		hydrateDealReviewLearnedPattern(&items[idx])
		if !dealReviewPatternFeatureSubset(featureLookup, items[idx].FeatureSet) {
			continue
		}
		items[idx].MatchScore = scoreDealReviewLearnedPatternMatch(caseRec, featureSet, &items[idx])
		matched = append(matched, items[idx])
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].MatchScore == matched[j].MatchScore {
			if matched[i].CompositeScore == matched[j].CompositeScore {
				return matched[i].SampleCount > matched[j].SampleCount
			}
			return matched[i].CompositeScore > matched[j].CompositeScore
		}
		return matched[i].MatchScore > matched[j].MatchScore
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

func MatchDealReviewLearnedPattern(caseRec *DealReviewCase, pattern *DealReviewLearnedPattern) (bool, float64) {
	if caseRec == nil || pattern == nil {
		return false, 0
	}
	featureSet, _ := extractDealReviewLearnedPatternFeatures(caseRec)
	if len(featureSet) == 0 {
		return false, 0
	}
	if normalizeDealReviewSide(caseRec.Side) != normalizeDealReviewSide(pattern.Side) {
		return false, 0
	}
	if normalizeDealReviewLearnedPatternScopeType(pattern.ScopeType) == DealReviewLearnedPatternScopeSymbol &&
		strings.ToUpper(strings.TrimSpace(pattern.Symbol)) != strings.ToUpper(strings.TrimSpace(caseRec.Symbol)) {
		return false, 0
	}

	if len(pattern.FeatureSet) == 0 && strings.TrimSpace(pattern.FeatureSetJSON) != "" {
		hydrateDealReviewLearnedPattern(pattern)
	}
	if len(pattern.FeatureSet) == 0 {
		return false, 0
	}

	featureLookup := make(map[string]struct{}, len(featureSet))
	for _, feature := range featureSet {
		featureLookup[feature] = struct{}{}
	}
	if !dealReviewPatternFeatureSubset(featureLookup, pattern.FeatureSet) {
		return false, 0
	}
	return true, scoreDealReviewLearnedPatternMatch(caseRec, featureSet, pattern)
}

func BuildDealReviewLearnedPatternReportingSummary(items []DealReviewLearnedPattern) *DealReviewLearnedPatternReportingSummary {
	summary := &DealReviewLearnedPatternReportingSummary{
		TotalCount:  len(items),
		ClassCounts: map[string]int{},
		LabelCounts: map[string]int{},
	}
	if len(items) == 0 {
		return summary
	}

	positive := make([]DealReviewLearnedPattern, 0, len(items))
	negative := make([]DealReviewLearnedPattern, 0, len(items))
	symbolOverrides := make([]DealReviewLearnedPattern, 0, len(items))
	for idx := range items {
		item := items[idx]
		hydrateDealReviewLearnedPattern(&item)
		classKey := normalizeDealReviewLearnedPatternClass(item.PatternClass)
		labelKey := normalizeDealReviewLearnedPatternValidationLabel(item.ValidationLabel)
		if classKey == "" {
			classKey = "unknown"
		}
		if labelKey == "" {
			labelKey = "unknown"
		}
		summary.ClassCounts[classKey]++
		summary.LabelCounts[labelKey]++
		switch classKey {
		case DealReviewLearnedPatternClassPositiveEdge:
			summary.PositiveCount++
			positive = append(positive, dealReviewLearnedPatternForSummaryCard(item))
		case DealReviewLearnedPatternClassNegativeEdge:
			summary.NegativeCount++
			negative = append(negative, dealReviewLearnedPatternForSummaryCard(item))
		}
		switch labelKey {
		case DealReviewLearnedPatternValidationLabelConfirmed:
			summary.ConfirmedCount++
		case DealReviewLearnedPatternValidationLabelCandidate:
			summary.CandidateCount++
		case DealReviewLearnedPatternValidationLabelFalsePositive:
			summary.FalsePositiveCount++
		case DealReviewLearnedPatternValidationLabelReverseRisk:
			summary.ReverseRiskCount++
		case DealReviewLearnedPatternValidationLabelDrifting:
			summary.DriftingCount++
		case DealReviewLearnedPatternValidationLabelExpired:
			summary.ExpiredCount++
		}
		if item.ScopeType == DealReviewLearnedPatternScopeSymbol &&
			(item.ValidationLabel == DealReviewLearnedPatternValidationLabelConfirmed ||
				item.ValidationLabel == DealReviewLearnedPatternValidationLabelFalsePositive ||
				item.ValidationLabel == DealReviewLearnedPatternValidationLabelReverseRisk) {
			symbolOverrides = append(symbolOverrides, dealReviewLearnedPatternForSummaryCard(item))
		}
	}

	sort.SliceStable(positive, func(i, j int) bool {
		if positive[i].CompositeScore == positive[j].CompositeScore {
			return positive[i].SampleCount > positive[j].SampleCount
		}
		return positive[i].CompositeScore > positive[j].CompositeScore
	})
	sort.SliceStable(negative, func(i, j int) bool {
		if negative[i].CompositeScore == negative[j].CompositeScore {
			return negative[i].SampleCount > negative[j].SampleCount
		}
		return negative[i].CompositeScore > negative[j].CompositeScore
	})
	sort.SliceStable(symbolOverrides, func(i, j int) bool {
		left := math.Abs(symbolOverrides[i].LiftAvgPnLPct)
		right := math.Abs(symbolOverrides[j].LiftAvgPnLPct)
		if left == right {
			return symbolOverrides[i].CompositeScore > symbolOverrides[j].CompositeScore
		}
		return left > right
	})

	summary.TopPositivePatterns = limitDealReviewLearnedPatternSummaryItems(positive, 3)
	summary.TopNegativePatterns = limitDealReviewLearnedPatternSummaryItems(negative, 3)
	summary.TopSymbolOverrides = limitDealReviewLearnedPatternSummaryItems(symbolOverrides, 3)

	notes := make([]string, 0, 4)
	if summary.PositiveCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) currently lean positive under the filtered trader slice.", summary.PositiveCount))
	}
	if summary.NegativeCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) currently behave like anti-edges or caution setups.", summary.NegativeCount))
	}
	if summary.FalsePositiveCount > 0 || summary.ReverseRiskCount > 0 || summary.DriftingCount > 0 {
		notes = append(notes, fmt.Sprintf("%d false-positive, %d reverse-risk, and %d drifting pattern(s) now need caution.", summary.FalsePositiveCount, summary.ReverseRiskCount, summary.DriftingCount))
	}
	if len(summary.TopSymbolOverrides) > 0 {
		top := summary.TopSymbolOverrides[0]
		notes = append(notes, fmt.Sprintf("Top symbol override %q on %s %s deviates by %+.2f%% against the side baseline.", top.PatternSignature, top.Symbol, top.Side, top.LiftAvgPnLPct))
	}
	summary.Notes = notes
	return summary
}

func buildDealReviewLearnedPatterns(cases []DealReviewCase, userID, traderID string, now time.Time) ([]DealReviewPatternFeatureRecord, []DealReviewLearnedPattern) {
	featureRows := make([]DealReviewPatternFeatureRecord, 0, len(cases))
	aggregates := map[string]*dealReviewLearnedPatternAggregate{}
	baselines := map[string]*dealReviewLearnedPatternBaseline{}

	for idx := range cases {
		caseRec := cases[idx]
		if !strings.EqualFold(caseRec.Status, DealReviewCaseStatusClosed) {
			continue
		}
		featureSet, qualityFlags := extractDealReviewLearnedPatternFeatures(&caseRec)
		observedAt := dealReviewSymbolBehaviorObservedAt(&caseRec)
		if observedAt.IsZero() {
			observedAt = now
		}
		featureRows = append(featureRows, DealReviewPatternFeatureRecord{
			UserID:             userID,
			TraderID:           traderID,
			CaseID:             strings.TrimSpace(caseRec.ID),
			PositionID:         caseRec.PositionID,
			Symbol:             strings.ToUpper(strings.TrimSpace(caseRec.Symbol)),
			Side:               normalizeDealReviewSide(caseRec.Side),
			Outcome:            classifyDealOutcome(caseRec.RealizedPnL),
			OpenConfidence:     caseRec.OpenConfidence,
			RealizedPnL:        caseRec.RealizedPnL,
			RealizedPnLPct:     caseRec.RealizedPnLPct,
			MaxMFEPct:          caseRec.MaxFavorableExcursionPct,
			MaxMAEPct:          caseRec.MaxAdverseExcursionPct,
			ProfitGivenBackPct: caseRec.ProfitGivenBackPct,
			HoldDurationMs:     caseRec.HoldDurationMs,
			FeatureCount:       len(featureSet),
			FeaturesJSON:       marshalDealReviewStringArray(featureSet),
			QualityFlagsJSON:   marshalDealReviewStringArray(qualityFlags),
			ObservedAt:         observedAt,
			BuiltAt:            now,
		})

		side := normalizeDealReviewSide(caseRec.Side)
		if side == "" || len(featureSet) == 0 {
			continue
		}
		baseline := baselines[side]
		if baseline == nil {
			baseline = &dealReviewLearnedPatternBaseline{}
			baselines[side] = baseline
		}
		baseline.Count++
		baseline.NetPnLPctSum += caseRec.RealizedPnLPct
		switch classifyDealOutcome(caseRec.RealizedPnL) {
		case "profit":
			baseline.Wins++
		case "loss":
			baseline.Losses++
		default:
			baseline.Flats++
		}

		combos := buildDealReviewLearnedPatternCombos(featureSet)
		for _, scope := range buildDealReviewLearnedPatternScopes(&caseRec) {
			for _, combo := range combos {
				key := strings.Join([]string{scope.scopeType, scope.scopeKey, side, strings.Join(combo, " + ")}, "|")
				agg := aggregates[key]
				if agg == nil {
					agg = &dealReviewLearnedPatternAggregate{
						ScopeType:       scope.scopeType,
						ScopeKey:        scope.scopeKey,
						Symbol:          scope.symbol,
						Side:            side,
						PatternOrder:    len(combo),
						FeatureSet:      append([]string(nil), combo...),
						Signature:       strings.Join(combo, " + "),
						RegimeSignature: buildDealReviewLearnedPatternRegimeSignature(combo),
					}
					aggregates[key] = agg
				}
				agg.Total++
				agg.NetPnL += caseRec.RealizedPnL
				agg.NetPnLPct += caseRec.RealizedPnLPct
				agg.SumMFEPct += caseRec.MaxFavorableExcursionPct
				agg.SumMAEPct += caseRec.MaxAdverseExcursionPct
				agg.SumHoldMs += caseRec.HoldDurationMs
				if caseRec.ProfitGivenBackPct > 0 {
					agg.GiveBackCount++
					agg.SumGiveBackPct += caseRec.ProfitGivenBackPct
				}
				outcome := classifyDealOutcome(caseRec.RealizedPnL)
				switch outcome {
				case "profit":
					agg.Wins++
				case "loss":
					agg.Losses++
				default:
					agg.Flats++
				}
				agg.ObservedCases = append(agg.ObservedCases, dealReviewSymbolBehaviorObservedCase{
					ObservedAt:     observedAt,
					Outcome:        outcome,
					RealizedPnL:    caseRec.RealizedPnL,
					RealizedPnLPct: caseRec.RealizedPnLPct,
				})
				if agg.FirstObservedAt.IsZero() || observedAt.Before(agg.FirstObservedAt) {
					agg.FirstObservedAt = observedAt
				}
				if agg.LastObservedAt.IsZero() || observedAt.After(agg.LastObservedAt) {
					agg.LastObservedAt = observedAt
				}
				if len(agg.Evidence) < dealReviewLearnedPatternEvidenceLimit {
					agg.Evidence = append(agg.Evidence, DealReviewLearnedPatternEvidence{
						CaseID:         strings.TrimSpace(caseRec.ID),
						PositionID:     caseRec.PositionID,
						Symbol:         strings.ToUpper(strings.TrimSpace(caseRec.Symbol)),
						Side:           side,
						Outcome:        outcome,
						RealizedPnL:    caseRec.RealizedPnL,
						RealizedPnLPct: caseRec.RealizedPnLPct,
						HoldDurationMs: caseRec.HoldDurationMs,
						ObservedAt:     observedAt.Format(time.RFC3339),
					})
				}
			}
		}
	}

	patterns := make([]DealReviewLearnedPattern, 0, len(aggregates))
	for _, agg := range aggregates {
		if agg.Total < dealReviewLearnedPatternObservedThreshold {
			continue
		}
		pattern, ok := buildDealReviewLearnedPatternFromAggregate(agg, baselines[agg.Side], now)
		if !ok {
			continue
		}
		pattern.UserID = userID
		pattern.TraderID = traderID
		patterns = append(patterns, pattern)
	}

	sort.SliceStable(patterns, func(i, j int) bool {
		if patterns[i].CompositeScore == patterns[j].CompositeScore {
			return patterns[i].SampleCount > patterns[j].SampleCount
		}
		return patterns[i].CompositeScore > patterns[j].CompositeScore
	})
	return featureRows, patterns
}

func buildDealReviewLearnedPatternFromAggregate(agg *dealReviewLearnedPatternAggregate, baseline *dealReviewLearnedPatternBaseline, now time.Time) (DealReviewLearnedPattern, bool) {
	if agg == nil || agg.Total < dealReviewLearnedPatternObservedThreshold {
		return DealReviewLearnedPattern{}, false
	}
	total := float64(agg.Total)
	winRate := aggRate(agg.Wins, agg.Total)
	lossRate := aggRate(agg.Losses, agg.Total)
	avgPnL := agg.NetPnL / total
	avgPnLPct := agg.NetPnLPct / total

	bias := DealReviewSymbolBehaviorBiasMixed
	switch {
	case winRate >= 0.55 && avgPnLPct > 0.05:
		bias = DealReviewSymbolBehaviorBiasPositive
	case lossRate >= 0.55 && avgPnLPct < -0.05:
		bias = DealReviewSymbolBehaviorBiasNegative
	case avgPnLPct >= 0.30 && agg.Wins >= agg.Losses:
		bias = DealReviewSymbolBehaviorBiasPositive
	case avgPnLPct <= -0.30 && agg.Losses >= agg.Wins:
		bias = DealReviewSymbolBehaviorBiasNegative
	}
	if bias == DealReviewSymbolBehaviorBiasMixed {
		return DealReviewLearnedPattern{}, false
	}

	supportCount := agg.Wins
	contradictCount := agg.Losses
	patternClass := DealReviewLearnedPatternClassPositiveEdge
	if bias == DealReviewSymbolBehaviorBiasNegative {
		supportCount = agg.Losses
		contradictCount = agg.Wins
		patternClass = DealReviewLearnedPatternClassNegativeEdge
	}

	trainingCases, validationCases, recentCases := splitDealReviewSymbolBehaviorObservedCases(agg.ObservedCases)
	validationMetrics := summarizeDealReviewSymbolBehaviorObservedCases(validationCases, bias)
	recentMetrics := summarizeDealReviewSymbolBehaviorObservedCases(recentCases, bias)
	driftScore := computeDealReviewSymbolBehaviorDriftScore(bias, validationMetrics, recentMetrics)
	recencyWeight := dealReviewSymbolBehaviorRecencyWeight(now, agg.LastObservedAt)
	status := classifyDealReviewSymbolBehaviorPriorStatus(
		agg.Total,
		bias,
		recencyWeight,
		validationMetrics,
		recentMetrics,
		driftScore,
		agg.LastObservedAt,
		now,
	)

	sampleScore := clampDealReviewUnit(float64(agg.Total) / 18.0)
	baselineWinRate := 0.0
	baselineAvgPnLPct := 0.0
	if baseline != nil {
		baselineWinRate = baseline.WinRate()
		baselineAvgPnLPct = baseline.AvgPnLPct()
	}
	liftWinRate := winRate - baselineWinRate
	liftAvgPnLPct := avgPnLPct - baselineAvgPnLPct
	liftStrength := computeDealReviewLearnedPatternLiftStrength(bias, liftWinRate, liftAvgPnLPct)

	windowSupport := validationMetrics.SupportScore
	if recentMetrics.SupportScore > windowSupport {
		windowSupport = recentMetrics.SupportScore
	}
	directionality := clampDealReviewUnit(math.Abs(winRate - lossRate))
	confidence := clampDealReviewUnit((sampleScore * 0.35) + (windowSupport * 0.30) + (recencyWeight * 0.15) + (directionality * 0.20))
	stability := clampDealReviewUnit(((1 - driftScore) * 0.60) + (windowSupport * 0.40))
	composite := clampDealReviewUnit((confidence * 0.30) + (stability * 0.20) + (liftStrength * 0.25) + (windowSupport * 0.15) + (sampleScore * 0.10))

	proxy := &DealReviewSymbolBehaviorPrior{
		Status:                    status,
		BehaviorBias:              bias,
		ValidationSampleCount:     validationMetrics.Count,
		ValidationSupportCount:    validationMetrics.SupportCount,
		ValidationContradictCount: validationMetrics.ContradictCount,
		ValidationAvgPnLPct:       validationMetrics.AvgPnLPct,
		ValidationSupportScore:    validationMetrics.SupportScore,
		RecentSampleCount:         recentMetrics.Count,
		RecentSupportCount:        recentMetrics.SupportCount,
		RecentContradictCount:     recentMetrics.ContradictCount,
		RecentAvgPnLPct:           recentMetrics.AvgPnLPct,
		RecentSupportScore:        recentMetrics.SupportScore,
		DriftScore:                driftScore,
	}
	applyDealReviewSymbolBehaviorPriorValidationReport(proxy)
	validationLabel := mapDealReviewLearnedPatternValidationLabel(proxy.ValidationLabel)
	validationAlert := buildDealReviewLearnedPatternValidationAlert(patternClass, validationLabel, proxy.FalsePositiveScore, proxy.FalseNegativeScore, driftScore)
	recommendedUse := buildDealReviewLearnedPatternRecommendedUse(patternClass, validationLabel, composite, agg.Total, math.Abs(liftAvgPnLPct))

	pattern := DealReviewLearnedPattern{
		ID:                        uuid.NewString(),
		ScopeType:                 agg.ScopeType,
		ScopeKey:                  agg.ScopeKey,
		Symbol:                    agg.Symbol,
		Side:                      agg.Side,
		PatternClass:              patternClass,
		Status:                    status,
		ValidationLabel:           validationLabel,
		RecommendedUse:            recommendedUse,
		PatternSignature:          agg.Signature,
		RegimeSignature:           agg.RegimeSignature,
		PatternOrder:              agg.PatternOrder,
		FeatureCount:              len(agg.FeatureSet),
		FeatureSetJSON:            marshalDealReviewStringArray(agg.FeatureSet),
		SampleCount:               agg.Total,
		WinningDeals:              agg.Wins,
		LosingDeals:               agg.Losses,
		FlatDeals:                 agg.Flats,
		SupportCount:              supportCount,
		ContradictCount:           contradictCount,
		WinRate:                   winRate,
		LossRate:                  lossRate,
		NetPnL:                    agg.NetPnL,
		AvgPnL:                    avgPnL,
		AvgPnLPct:                 avgPnLPct,
		Expectancy:                avgPnL,
		AvgMFEPct:                 agg.SumMFEPct / total,
		AvgMAEPct:                 agg.SumMAEPct / total,
		GiveBackRate:              aggRate(agg.GiveBackCount, agg.Total),
		AvgGiveBackPct:            safeAvgDealReviewPattern(agg.SumGiveBackPct, agg.GiveBackCount),
		AvgHoldMs:                 int64(float64(agg.SumHoldMs) / total),
		BaselineWinRate:           baselineWinRate,
		BaselineAvgPnLPct:         baselineAvgPnLPct,
		LiftWinRate:               liftWinRate,
		LiftAvgPnLPct:             liftAvgPnLPct,
		TrainingSampleCount:       len(trainingCases),
		ValidationSampleCount:     validationMetrics.Count,
		ValidationSupportCount:    validationMetrics.SupportCount,
		ValidationContradictCount: validationMetrics.ContradictCount,
		ValidationAvgPnLPct:       validationMetrics.AvgPnLPct,
		ValidationSupportScore:    validationMetrics.SupportScore,
		RecentSampleCount:         recentMetrics.Count,
		RecentSupportCount:        recentMetrics.SupportCount,
		RecentContradictCount:     recentMetrics.ContradictCount,
		RecentAvgPnLPct:           recentMetrics.AvgPnLPct,
		RecentSupportScore:        recentMetrics.SupportScore,
		ConfidenceScore:           confidence,
		StabilityScore:            stability,
		DriftScore:                driftScore,
		RecencyWeight:             recencyWeight,
		CompositeScore:            composite,
		FalsePositiveScore:        proxy.FalsePositiveScore,
		ReverseRiskScore:          proxy.FalseNegativeScore,
		ValidationAlert:           validationAlert,
		EvidenceJSON:              marshalDealReviewLearnedPatternEvidence(agg.Evidence),
		FirstObservedAt:           agg.FirstObservedAt,
		LastObservedAt:            agg.LastObservedAt,
		BuiltAt:                   now,
	}
	pattern.FeatureSet = append([]string(nil), agg.FeatureSet...)
	pattern.Evidence = append([]DealReviewLearnedPatternEvidence(nil), agg.Evidence...)
	pattern.Summary = buildDealReviewLearnedPatternSummaryText(&pattern)
	return pattern, true
}

func (s *DealReviewStore) countLearnedPatternFeatures(userID, traderID string) (int, error) {
	var count int64
	if err := s.db.Model(&DealReviewPatternFeatureRecord{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID)).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *DealReviewStore) countLearnedPatterns(userID, traderID string) (int, error) {
	var count int64
	if err := s.db.Model(&DealReviewLearnedPattern{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID)).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func extractDealReviewLearnedPatternFeatures(caseRec *DealReviewCase) ([]string, []string) {
	if caseRec == nil {
		return nil, nil
	}
	features := make([]string, 0, dealReviewLearnedPatternFeatureLimit)
	qualityFlags := make([]string, 0, 6)
	seen := map[string]struct{}{}
	appendFeature := func(prefix, raw string) {
		value := normalizeDealReviewPatternFeatureToken(raw)
		if value == "" || value == "unknown" {
			return
		}
		feature := prefix + ":" + value
		if _, ok := seen[feature]; ok {
			return
		}
		seen[feature] = struct{}{}
		features = append(features, feature)
	}
	appendFlag := func(flag string) {
		flag = normalizeDealReviewPatternFeatureToken(flag)
		if flag == "" {
			return
		}
		for _, current := range qualityFlags {
			if current == flag {
				return
			}
		}
		qualityFlags = append(qualityFlags, flag)
	}

	appendFeature("bucket", caseRec.OpenSelectionBucket)
	appendFeature("trend", caseRec.OpenTrendRegime)
	appendFeature("vol", caseRec.OpenVolatilityRegime)
	appendFeature("btc", caseRec.OpenBTCStrengthRegime)
	appendFeature("funding", caseRec.OpenFundingRegime)
	appendFeature("oi", caseRec.OpenOIRegime)
	appendFeature("session", caseRec.OpenSessionBucket)
	appendFeature("weekday", caseRec.OpenWeekdayBucket)
	appendFeature("venue", caseRec.OpenVenueTier)
	appendFeature("liq", caseRec.OpenLiquidityTier)
	appendFeature("spread", caseRec.OpenSpreadBucket)
	appendFeature("slip", caseRec.OpenSlippageBucket)

	switch {
	case caseRec.OpenConfidence >= 80:
		appendFeature("conf", "high")
	case caseRec.OpenConfidence >= 65:
		appendFeature("conf", "mid")
	case caseRec.OpenConfidence > 0:
		appendFeature("conf", "low")
	default:
		appendFlag("missing_confidence")
	}

	if riskBucket := bucketDealReviewPlannedRisk(caseRec.PlannedRiskPct); riskBucket != "" {
		appendFeature("risk", riskBucket)
	}
	for _, source := range unmarshalDealReviewStringArray(caseRec.OpenCandidateSourcesJSON) {
		appendFeature("src", source)
	}

	if normalizeDealReviewPatternFeatureToken(caseRec.OpenTrendRegime) == "" {
		appendFlag("missing_trend_regime")
	}
	if normalizeDealReviewPatternFeatureToken(caseRec.OpenVolatilityRegime) == "" {
		appendFlag("missing_volatility_regime")
	}
	if normalizeDealReviewPatternFeatureToken(caseRec.OpenOIRegime) == "" {
		appendFlag("missing_oi_regime")
	}
	if normalizeDealReviewPatternFeatureToken(caseRec.OpenSessionBucket) == "" {
		appendFlag("missing_session_bucket")
	}

	sort.Strings(features)
	if len(features) > dealReviewLearnedPatternFeatureLimit {
		features = features[:dealReviewLearnedPatternFeatureLimit]
	}
	sort.Strings(qualityFlags)
	return features, qualityFlags
}

func buildDealReviewLearnedPatternCombos(features []string) [][]string {
	if len(features) == 0 {
		return nil
	}
	limit := len(features)
	if limit > dealReviewLearnedPatternFeatureLimit {
		limit = dealReviewLearnedPatternFeatureLimit
	}
	trimmed := append([]string(nil), features[:limit]...)
	combos := make([][]string, 0, len(trimmed)+(len(trimmed)*(len(trimmed)-1))/2)
	for i := range trimmed {
		combos = append(combos, []string{trimmed[i]})
	}
	if dealReviewLearnedPatternMaxOrder >= 2 {
		for i := 0; i < len(trimmed); i++ {
			for j := i + 1; j < len(trimmed); j++ {
				combos = append(combos, []string{trimmed[i], trimmed[j]})
			}
		}
	}
	return combos
}

type dealReviewLearnedPatternScope struct {
	scopeType string
	scopeKey  string
	symbol    string
}

func buildDealReviewLearnedPatternScopes(caseRec *DealReviewCase) []dealReviewLearnedPatternScope {
	if caseRec == nil {
		return nil
	}
	symbol := strings.ToUpper(strings.TrimSpace(caseRec.Symbol))
	out := []dealReviewLearnedPatternScope{
		{
			scopeType: DealReviewLearnedPatternScopeTraderLocal,
			scopeKey:  DealReviewLearnedPatternScopeTraderLocal,
			symbol:    "",
		},
	}
	if symbol != "" {
		out = append(out, dealReviewLearnedPatternScope{
			scopeType: DealReviewLearnedPatternScopeSymbol,
			scopeKey:  symbol,
			symbol:    symbol,
		})
	}
	return out
}

func buildDealReviewLearnedPatternRegimeSignature(features []string) string {
	if len(features) == 0 {
		return ""
	}
	selected := make([]string, 0, 4)
	for _, feature := range features {
		switch {
		case strings.HasPrefix(feature, "trend:"),
			strings.HasPrefix(feature, "vol:"),
			strings.HasPrefix(feature, "oi:"),
			strings.HasPrefix(feature, "funding:"),
			strings.HasPrefix(feature, "btc:"),
			strings.HasPrefix(feature, "session:"):
			selected = append(selected, feature)
		}
		if len(selected) >= 4 {
			break
		}
	}
	return strings.Join(selected, " | ")
}

func bucketDealReviewPlannedRisk(value float64) string {
	switch {
	case value <= 0:
		return ""
	case value <= 0.50:
		return "tight"
	case value <= 1.25:
		return "normal"
	case value <= 2.50:
		return "wide"
	default:
		return "aggressive"
	}
}

func normalizeDealReviewPatternFeatureToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return ""
	}
	return out
}

func normalizeDealReviewPatternFeatureFilterToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		prefix := normalizeDealReviewPatternFeatureToken(parts[0])
		suffix := normalizeDealReviewPatternFeatureToken(parts[1])
		if prefix == "" || suffix == "" {
			return ""
		}
		return prefix + ":" + suffix
	}
	normalized := normalizeDealReviewPatternFeatureToken(value)
	if normalized == "" {
		return ""
	}
	if idx := strings.Index(normalized, "_"); idx > 0 && idx < len(normalized)-1 {
		return normalized[:idx] + ":" + normalized[idx+1:]
	}
	return normalized
}

func normalizeDealReviewLearnedPatternScopeType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternScopeTraderLocal:
		return DealReviewLearnedPatternScopeTraderLocal
	case DealReviewLearnedPatternScopeSymbol:
		return DealReviewLearnedPatternScopeSymbol
	default:
		return ""
	}
}

func normalizeDealReviewLearnedPatternClass(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternClassPositiveEdge:
		return DealReviewLearnedPatternClassPositiveEdge
	case DealReviewLearnedPatternClassNegativeEdge:
		return DealReviewLearnedPatternClassNegativeEdge
	default:
		return ""
	}
}

func normalizeDealReviewLearnedPatternValidationLabel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternValidationLabelConfirmed:
		return DealReviewLearnedPatternValidationLabelConfirmed
	case DealReviewLearnedPatternValidationLabelCandidate:
		return DealReviewLearnedPatternValidationLabelCandidate
	case DealReviewLearnedPatternValidationLabelInsufficientEvidence:
		return DealReviewLearnedPatternValidationLabelInsufficientEvidence
	case DealReviewLearnedPatternValidationLabelFalsePositive:
		return DealReviewLearnedPatternValidationLabelFalsePositive
	case DealReviewLearnedPatternValidationLabelReverseRisk:
		return DealReviewLearnedPatternValidationLabelReverseRisk
	case DealReviewLearnedPatternValidationLabelDrifting:
		return DealReviewLearnedPatternValidationLabelDrifting
	case DealReviewLearnedPatternValidationLabelExpired:
		return DealReviewLearnedPatternValidationLabelExpired
	default:
		return ""
	}
}

func computeDealReviewLearnedPatternLiftStrength(bias string, liftWinRate, liftAvgPnLPct float64) float64 {
	switch strings.TrimSpace(bias) {
	case DealReviewSymbolBehaviorBiasPositive:
		return clampDealReviewUnit((clampDealReviewUnit(liftWinRate/0.25) * 0.45) + (clampDealReviewUnit(liftAvgPnLPct/1.5) * 0.55))
	case DealReviewSymbolBehaviorBiasNegative:
		return clampDealReviewUnit((clampDealReviewUnit((-liftWinRate)/0.25) * 0.45) + (clampDealReviewUnit(math.Abs(minFloat(liftAvgPnLPct, 0))/1.5) * 0.55))
	default:
		return 0
	}
}

func mapDealReviewLearnedPatternValidationLabel(value string) string {
	switch strings.TrimSpace(value) {
	case DealReviewSymbolBehaviorValidationLabelConfirmed:
		return DealReviewLearnedPatternValidationLabelConfirmed
	case DealReviewSymbolBehaviorValidationLabelCandidate:
		return DealReviewLearnedPatternValidationLabelCandidate
	case DealReviewSymbolBehaviorValidationLabelFalsePositive:
		return DealReviewLearnedPatternValidationLabelFalsePositive
	case DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk:
		return DealReviewLearnedPatternValidationLabelReverseRisk
	case DealReviewSymbolBehaviorValidationLabelDrifting:
		return DealReviewLearnedPatternValidationLabelDrifting
	case DealReviewSymbolBehaviorValidationLabelInsufficientEvidence:
		return DealReviewLearnedPatternValidationLabelInsufficientEvidence
	default:
		if strings.EqualFold(strings.TrimSpace(value), DealReviewSymbolBehaviorPriorStatusExpired) {
			return DealReviewLearnedPatternValidationLabelExpired
		}
		return DealReviewLearnedPatternValidationLabelCandidate
	}
}

func buildDealReviewLearnedPatternValidationAlert(patternClass, label string, falsePositiveScore, reverseRiskScore, driftScore float64) string {
	switch normalizeDealReviewLearnedPatternValidationLabel(label) {
	case DealReviewLearnedPatternValidationLabelConfirmed:
		return "Holdout and recent windows still support this learned pattern."
	case DealReviewLearnedPatternValidationLabelFalsePositive:
		return fmt.Sprintf("Holdout or recent windows contradicted this %s strongly enough that it now looks like a likely false positive (score %.0f%%).", patternClass, falsePositiveScore*100)
	case DealReviewLearnedPatternValidationLabelReverseRisk:
		return fmt.Sprintf("Recent evidence is leaning toward the opposite direction instead (score %.0f%%), so this pattern now behaves like a reverse-risk setup.", reverseRiskScore*100)
	case DealReviewLearnedPatternValidationLabelDrifting:
		return fmt.Sprintf("Recent evidence is drifting away from the earlier learned edge (drift %.0f%%).", driftScore*100)
	case DealReviewLearnedPatternValidationLabelExpired:
		return "This pattern is too stale to trust without fresh confirmation."
	case DealReviewLearnedPatternValidationLabelInsufficientEvidence:
		return "Not enough holdout evidence yet to judge whether this pattern is real."
	default:
		return "This pattern is directional, but its validation windows are still mixed."
	}
}

func buildDealReviewLearnedPatternRecommendedUse(patternClass, label string, composite float64, sampleCount int, absLiftPnLPct float64) string {
	switch normalizeDealReviewLearnedPatternValidationLabel(label) {
	case DealReviewLearnedPatternValidationLabelExpired:
		return DealReviewLearnedPatternRecommendedUseExpiredIgnore
	case DealReviewLearnedPatternValidationLabelFalsePositive, DealReviewLearnedPatternValidationLabelDrifting:
		return DealReviewLearnedPatternRecommendedUseMonitorOnly
	case DealReviewLearnedPatternValidationLabelReverseRisk:
		return DealReviewLearnedPatternRecommendedUseReviewHint
	case DealReviewLearnedPatternValidationLabelConfirmed:
		if patternClass == DealReviewLearnedPatternClassPositiveEdge &&
			composite >= 0.78 &&
			sampleCount >= 8 &&
			absLiftPnLPct >= 0.45 {
			return DealReviewLearnedPatternRecommendedUseConfigCand
		}
		if patternClass == DealReviewLearnedPatternClassNegativeEdge {
			return DealReviewLearnedPatternRecommendedUsePromptHint
		}
		return DealReviewLearnedPatternRecommendedUseReviewHint
	default:
		return DealReviewLearnedPatternRecommendedUseReviewHint
	}
}

func buildDealReviewLearnedPatternSummaryText(pattern *DealReviewLearnedPattern) string {
	if pattern == nil {
		return ""
	}
	baselineDelta := pattern.LiftAvgPnLPct
	scopeLabel := "trader-local"
	if pattern.ScopeType == DealReviewLearnedPatternScopeSymbol && strings.TrimSpace(pattern.Symbol) != "" {
		scopeLabel = pattern.Symbol
	}
	return fmt.Sprintf(
		"%s on %s %s matched %d/%d supporting cases (avg %+.2f%%, lift %+.2f%% vs baseline). Holdout %d/%d, recent %d/%d, drift %.0f%%.",
		pattern.PatternSignature,
		scopeLabel,
		pattern.Side,
		pattern.SupportCount,
		pattern.SampleCount,
		pattern.AvgPnLPct,
		baselineDelta,
		pattern.ValidationSupportCount,
		pattern.ValidationSampleCount,
		pattern.RecentSupportCount,
		pattern.RecentSampleCount,
		pattern.DriftScore*100,
	)
}

func dealReviewPatternFeatureSubset(lookup map[string]struct{}, features []string) bool {
	if len(features) == 0 {
		return false
	}
	for _, feature := range features {
		if _, ok := lookup[feature]; !ok {
			return false
		}
	}
	return true
}

func scoreDealReviewLearnedPatternMatch(caseRec *DealReviewCase, caseFeatures []string, pattern *DealReviewLearnedPattern) float64 {
	if caseRec == nil || pattern == nil || len(caseFeatures) == 0 || len(pattern.FeatureSet) == 0 {
		return 0
	}
	score := 0.35
	score += clampDealReviewUnit(float64(len(pattern.FeatureSet))/float64(maxInt(len(caseFeatures), 1))) * 0.20
	score += clampDealReviewUnit(pattern.CompositeScore) * 0.20
	score += clampDealReviewUnit(pattern.ConfidenceScore) * 0.15
	score += clampDealReviewUnit(pattern.ValidationSupportScore) * 0.10
	if pattern.ScopeType == DealReviewLearnedPatternScopeSymbol &&
		strings.EqualFold(strings.TrimSpace(caseRec.Symbol), strings.TrimSpace(pattern.Symbol)) {
		score += 0.10
	}
	if pattern.PatternOrder >= 2 {
		score += 0.05
	}
	return clampDealReviewUnit(score)
}

func dealReviewLearnedPatternForSummaryCard(item DealReviewLearnedPattern) DealReviewLearnedPattern {
	item.Evidence = nil
	item.FeatureSet = append([]string(nil), item.FeatureSet...)
	return item
}

func limitDealReviewLearnedPatternSummaryItems(items []DealReviewLearnedPattern, limit int) []DealReviewLearnedPattern {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]DealReviewLearnedPattern, 0, len(items))
	for _, item := range items {
		out = append(out, dealReviewLearnedPatternForSummaryCard(item))
	}
	return out
}

func hydrateDealReviewLearnedPattern(item *DealReviewLearnedPattern) {
	if item == nil {
		return
	}
	item.FeatureSet = unmarshalDealReviewStringArray(item.FeatureSetJSON)
	item.Evidence = unmarshalDealReviewLearnedPatternEvidence(item.EvidenceJSON)
}

func unmarshalDealReviewLearnedPatternEvidence(raw string) []DealReviewLearnedPatternEvidence {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var items []DealReviewLearnedPatternEvidence
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}
	return items
}

func marshalDealReviewLearnedPatternEvidence(items []DealReviewLearnedPatternEvidence) string {
	body, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(body)
}

func safeAvgDealReviewPattern(sum float64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return sum / float64(count)
}
