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
	DealReviewSymbolBehaviorPriorStatusObserved  = "observed"
	DealReviewSymbolBehaviorPriorStatusCandidate = "candidate"
	DealReviewSymbolBehaviorPriorStatusValidated = "validated"
	DealReviewSymbolBehaviorPriorStatusRejected  = "rejected"
	DealReviewSymbolBehaviorPriorStatusExpired   = "expired"

	DealReviewSymbolBehaviorBiasPositive = "positive"
	DealReviewSymbolBehaviorBiasNegative = "negative"
	DealReviewSymbolBehaviorBiasMixed    = "mixed"

	dealReviewSymbolBehaviorObservedThreshold     = 3
	dealReviewSymbolBehaviorCandidateThreshold    = 5
	dealReviewSymbolBehaviorHalfLifeDays          = 30.0
	dealReviewSymbolBehaviorRefreshInterval       = 15 * time.Minute
	dealReviewSymbolBehaviorEvidenceLimit         = 5
	dealReviewSymbolBehaviorDecisionEvidenceLimit = 5
	dealReviewSymbolBehaviorSignalTagLimit        = 8
	dealReviewSymbolBehaviorSignalClusterLimit    = 8
	dealReviewSymbolBehaviorValidationMinSamples  = 2
	dealReviewSymbolBehaviorRecentWindowMin       = 2
	dealReviewSymbolBehaviorRecentWindowMax       = 4
)

type DealReviewSymbolBehaviorPrior struct {
	ID                        string                                     `gorm:"primaryKey" json:"id"`
	UserID                    string                                     `gorm:"column:user_id;not null;index:idx_symbol_behavior_priors_user_trader" json:"user_id"`
	TraderID                  string                                     `gorm:"column:trader_id;not null;index:idx_symbol_behavior_priors_user_trader;index:idx_symbol_behavior_priors_lookup" json:"trader_id"`
	Symbol                    string                                     `gorm:"column:symbol;not null;index:idx_symbol_behavior_priors_lookup" json:"symbol"`
	Side                      string                                     `gorm:"column:side;not null;index:idx_symbol_behavior_priors_lookup" json:"side"`
	Status                    string                                     `gorm:"column:status;default:observed;index:idx_symbol_behavior_priors_status" json:"status"`
	BehaviorBias              string                                     `gorm:"column:behavior_bias;default:mixed;index:idx_symbol_behavior_priors_bias" json:"behavior_bias"`
	RecommendedAction         string                                     `gorm:"column:recommended_action;default:observe" json:"recommended_action"`
	OpenSelectionBucket       string                                     `gorm:"column:open_selection_bucket;default:''" json:"open_selection_bucket"`
	OpenTrendRegime           string                                     `gorm:"column:open_trend_regime;default:''" json:"open_trend_regime"`
	OpenVolatilityRegime      string                                     `gorm:"column:open_volatility_regime;default:''" json:"open_volatility_regime"`
	OpenOIRegime              string                                     `gorm:"column:open_oi_regime;default:''" json:"open_oi_regime"`
	RegimeSignature           string                                     `gorm:"column:regime_signature;default:'';index:idx_symbol_behavior_priors_regime" json:"regime_signature"`
	SampleCount               int                                        `gorm:"column:sample_count;default:0" json:"sample_count"`
	WinningDeals              int                                        `gorm:"column:winning_deals;default:0" json:"winning_deals"`
	LosingDeals               int                                        `gorm:"column:losing_deals;default:0" json:"losing_deals"`
	FlatDeals                 int                                        `gorm:"column:flat_deals;default:0" json:"flat_deals"`
	WinRate                   float64                                    `gorm:"column:win_rate;default:0" json:"win_rate"`
	LossRate                  float64                                    `gorm:"column:loss_rate;default:0" json:"loss_rate"`
	NetPnL                    float64                                    `gorm:"column:net_pnl;default:0" json:"net_pnl"`
	AvgPnL                    float64                                    `gorm:"column:avg_pnl;default:0" json:"avg_pnl"`
	AvgPnLPct                 float64                                    `gorm:"column:avg_pnl_pct;default:0" json:"avg_pnl_pct"`
	Expectancy                float64                                    `gorm:"column:expectancy;default:0" json:"expectancy"`
	AvgMFEPct                 float64                                    `gorm:"column:avg_mfe_pct;default:0" json:"avg_mfe_pct"`
	AvgMAEPct                 float64                                    `gorm:"column:avg_mae_pct;default:0" json:"avg_mae_pct"`
	GiveBackRate              float64                                    `gorm:"column:give_back_rate;default:0" json:"give_back_rate"`
	AvgGiveBackPct            float64                                    `gorm:"column:avg_give_back_pct;default:0" json:"avg_give_back_pct"`
	AvgHoldMs                 int64                                      `gorm:"column:avg_hold_ms;default:0" json:"avg_hold_ms"`
	DecisionCycleCount        int                                        `gorm:"column:decision_cycle_count;default:0" json:"decision_cycle_count"`
	DecisionOpenCount         int                                        `gorm:"column:decision_open_count;default:0" json:"decision_open_count"`
	AvgDecisionConfidence     float64                                    `gorm:"column:avg_decision_confidence;default:0" json:"avg_decision_confidence"`
	TrainingSampleCount       int                                        `gorm:"column:training_sample_count;default:0" json:"training_sample_count"`
	ValidationSampleCount     int                                        `gorm:"column:validation_sample_count;default:0" json:"validation_sample_count"`
	ValidationSupportCount    int                                        `gorm:"column:validation_support_count;default:0" json:"validation_support_count"`
	ValidationContradictCount int                                        `gorm:"column:validation_contradict_count;default:0" json:"validation_contradict_count"`
	ValidationAvgPnLPct       float64                                    `gorm:"column:validation_avg_pnl_pct;default:0" json:"validation_avg_pnl_pct"`
	ValidationSupportScore    float64                                    `gorm:"column:validation_support_score;default:0" json:"validation_support_score"`
	RecentSampleCount         int                                        `gorm:"column:recent_sample_count;default:0" json:"recent_sample_count"`
	RecentSupportCount        int                                        `gorm:"column:recent_support_count;default:0" json:"recent_support_count"`
	RecentContradictCount     int                                        `gorm:"column:recent_contradict_count;default:0" json:"recent_contradict_count"`
	RecentAvgPnLPct           float64                                    `gorm:"column:recent_avg_pnl_pct;default:0" json:"recent_avg_pnl_pct"`
	RecentSupportScore        float64                                    `gorm:"column:recent_support_score;default:0" json:"recent_support_score"`
	DriftScore                float64                                    `gorm:"column:drift_score;default:0" json:"drift_score"`
	ContradictionScore        float64                                    `gorm:"column:contradiction_score;default:0;index:idx_symbol_behavior_priors_contradiction" json:"contradiction_score"`
	ConfidenceScore           float64                                    `gorm:"column:confidence_score;default:0" json:"confidence_score"`
	RecencyWeight             float64                                    `gorm:"column:recency_weight;default:0" json:"recency_weight"`
	StabilityScore            float64                                    `gorm:"column:stability_score;default:0" json:"stability_score"`
	CompositeScore            float64                                    `gorm:"column:composite_score;default:0;index:idx_symbol_behavior_priors_score" json:"composite_score"`
	Summary                   string                                     `gorm:"column:summary;type:text;default:''" json:"summary"`
	ValidationSummary         string                                     `gorm:"column:validation_summary;type:text;default:''" json:"validation_summary"`
	SignalClusterKey          string                                     `gorm:"column:signal_cluster_key;type:text;default:''" json:"signal_cluster_key,omitempty"`
	SignalTagsJSON            string                                     `gorm:"column:signal_tags_json;type:text;default:'[]'" json:"-"`
	SignalClustersJSON        string                                     `gorm:"column:signal_clusters_json;type:text;default:'[]'" json:"-"`
	EvidenceJSON              string                                     `gorm:"column:evidence_json;type:text;default:'[]'" json:"-"`
	DecisionEvidenceJSON      string                                     `gorm:"column:decision_evidence_json;type:text;default:'[]'" json:"-"`
	FirstObservedAt           time.Time                                  `gorm:"column:first_observed_at" json:"first_observed_at"`
	LastObservedAt            time.Time                                  `gorm:"column:last_observed_at;index:idx_symbol_behavior_priors_last_observed" json:"last_observed_at"`
	BuiltAt                   time.Time                                  `gorm:"column:built_at;index:idx_symbol_behavior_priors_built" json:"built_at"`
	CreatedAt                 time.Time                                  `json:"created_at"`
	UpdatedAt                 time.Time                                  `json:"updated_at"`
	SignalTags                []string                                   `gorm:"-" json:"signal_tags,omitempty"`
	SignalClusters            []string                                   `gorm:"-" json:"signal_clusters,omitempty"`
	Evidence                  []DealReviewSymbolBehaviorPriorEvidence    `gorm:"-" json:"evidence,omitempty"`
	DecisionEvidence          []DealReviewSymbolBehaviorDecisionEvidence `gorm:"-" json:"decision_evidence,omitempty"`
	MatchScore                float64                                    `gorm:"-" json:"match_score,omitempty"`
	ValidationLabel           string                                     `gorm:"-" json:"validation_label,omitempty"`
	ValidationAlert           string                                     `gorm:"-" json:"validation_alert,omitempty"`
	FalsePositiveScore        float64                                    `gorm:"-" json:"false_positive_score,omitempty"`
	FalseNegativeScore        float64                                    `gorm:"-" json:"false_negative_score,omitempty"`
}

func (DealReviewSymbolBehaviorPrior) TableName() string { return "deal_review_symbol_behavior_priors" }

type DealReviewSymbolBehaviorPriorEvidence struct {
	CaseID          string  `json:"case_id"`
	PositionID      int64   `json:"position_id"`
	Outcome         string  `json:"outcome"`
	RealizedPnL     float64 `json:"realized_pnl"`
	RealizedPnLPct  float64 `json:"realized_pnl_pct"`
	CloseReason     string  `json:"close_reason,omitempty"`
	ExitOrigin      string  `json:"exit_origin,omitempty"`
	OpenCycleNumber int     `json:"open_cycle_number,omitempty"`
	ExitTimeMs      int64   `json:"exit_time_ms"`
}

type DealReviewSymbolBehaviorDecisionEvidence struct {
	CycleNumber      int       `json:"cycle_number"`
	Timestamp        time.Time `json:"timestamp"`
	Action           string    `json:"action"`
	Confidence       int       `json:"confidence"`
	Reasoning        string    `json:"reasoning,omitempty"`
	CandidateSources []string  `json:"candidate_sources,omitempty"`
	SignalTags       []string  `json:"signal_tags,omitempty"`
	SignalClusterKey string    `json:"signal_cluster_key,omitempty"`
	SignalClusters   []string  `json:"signal_clusters,omitempty"`
	TerminalStatus   string    `json:"terminal_status,omitempty"`
}

type DealReviewSymbolBehaviorPriorFilter struct {
	Symbol        string
	Side          string
	Status        string
	SignalCluster string
	Limit         int
}

type DealReviewSymbolBehaviorPriorRefreshResult struct {
	Rebuilt     bool      `json:"rebuilt"`
	ItemCount   int       `json:"item_count"`
	GeneratedAt time.Time `json:"generated_at"`
}

type dealReviewSymbolBehaviorPriorAggregate struct {
	Symbol                string
	Side                  string
	OpenSelectionBucket   string
	OpenTrendRegime       string
	OpenVolatilityRegime  string
	OpenOIRegime          string
	RegimeSignature       string
	Total                 int
	Wins                  int
	Losses                int
	Flats                 int
	NetPnL                float64
	NetPnLPct             float64
	SumMFEPct             float64
	SumMAEPct             float64
	SumGiveBackPct        float64
	GiveBackCount         int
	SumHoldMs             int64
	WeightedCount         float64
	WeightedWins          float64
	WeightedLosses        float64
	WeightedFlats         float64
	RecencyWeightSum      float64
	DecisionCycleNumbers  map[int]struct{}
	DecisionOpenCount     int
	DecisionConfidenceSum float64
	DecisionLastSeenAt    time.Time
	SignalTagCounts       map[string]int
	SignalClusterCounts   map[string]int
	FirstObservedAt       time.Time
	LastObservedAt        time.Time
	ObservedCases         []dealReviewSymbolBehaviorObservedCase
	Evidence              []DealReviewSymbolBehaviorPriorEvidence
	DecisionEvidence      []DealReviewSymbolBehaviorDecisionEvidence
}

func (s *DealReviewStore) RefreshSymbolBehaviorPriorsIfStale(userID, traderID string) (*DealReviewSymbolBehaviorPriorRefreshResult, error) {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" {
		return &DealReviewSymbolBehaviorPriorRefreshResult{Rebuilt: false}, nil
	}

	var latestCaseUpdatedRaw sql.NullString
	if err := s.db.Model(&DealReviewCase{}).
		Where("user_id = ? AND trader_id = ? AND status = ?", userID, traderID, DealReviewCaseStatusClosed).
		Select("CAST(MAX(updated_at) AS TEXT)").Scan(&latestCaseUpdatedRaw).Error; err != nil {
		return nil, err
	}
	latestCaseUpdated := parseDealReviewAggregateTime(latestCaseUpdatedRaw.String)

	var latestDecisionAtRaw sql.NullString
	if err := s.db.Model(&DecisionRecordDB{}).
		Where("trader_id = ? AND candidate_metadata_version > 0 AND decisions IS NOT NULL AND decisions <> '[]' AND LOWER(TRIM(decisions)) <> 'null'", traderID).
		Select("CAST(MAX(timestamp) AS TEXT)").Scan(&latestDecisionAtRaw).Error; err != nil {
		return nil, err
	}
	latestDecisionAt := parseDealReviewAggregateTime(latestDecisionAtRaw.String)

	var latestPriorBuiltRaw sql.NullString
	if err := s.db.Model(&DealReviewSymbolBehaviorPrior{}).
		Where("user_id = ? AND trader_id = ?", userID, traderID).
		Select("CAST(MAX(built_at) AS TEXT)").Scan(&latestPriorBuiltRaw).Error; err != nil {
		return nil, err
	}
	latestPriorBuilt := parseDealReviewAggregateTime(latestPriorBuiltRaw.String)

	needsRefresh := latestPriorBuilt.IsZero()
	if !needsRefresh && !latestCaseUpdated.IsZero() && latestCaseUpdated.After(latestPriorBuilt) {
		needsRefresh = true
	}
	if !needsRefresh && !latestDecisionAt.IsZero() && latestDecisionAt.After(latestPriorBuilt) {
		needsRefresh = true
	}
	if !needsRefresh && time.Since(latestPriorBuilt) >= dealReviewSymbolBehaviorRefreshInterval {
		needsRefresh = true
	}
	if !needsRefresh {
		count, err := s.countSymbolBehaviorPriors(userID, traderID)
		if err != nil {
			return nil, err
		}
		return &DealReviewSymbolBehaviorPriorRefreshResult{
			Rebuilt:     false,
			ItemCount:   count,
			GeneratedAt: latestPriorBuilt,
		}, nil
	}
	return s.RebuildSymbolBehaviorPriors(userID, traderID)
}

func (s *DealReviewStore) ensureSymbolBehaviorPriorColumns() error {
	requiredColumns := []struct {
		fieldName  string
		columnName string
	}{
		{fieldName: "DecisionCycleCount", columnName: "decision_cycle_count"},
		{fieldName: "DecisionOpenCount", columnName: "decision_open_count"},
		{fieldName: "AvgDecisionConfidence", columnName: "avg_decision_confidence"},
		{fieldName: "TrainingSampleCount", columnName: "training_sample_count"},
		{fieldName: "ValidationSampleCount", columnName: "validation_sample_count"},
		{fieldName: "ValidationSupportCount", columnName: "validation_support_count"},
		{fieldName: "ValidationContradictCount", columnName: "validation_contradict_count"},
		{fieldName: "ValidationAvgPnLPct", columnName: "validation_avg_pnl_pct"},
		{fieldName: "ValidationSupportScore", columnName: "validation_support_score"},
		{fieldName: "RecentSampleCount", columnName: "recent_sample_count"},
		{fieldName: "RecentSupportCount", columnName: "recent_support_count"},
		{fieldName: "RecentContradictCount", columnName: "recent_contradict_count"},
		{fieldName: "RecentAvgPnLPct", columnName: "recent_avg_pnl_pct"},
		{fieldName: "RecentSupportScore", columnName: "recent_support_score"},
		{fieldName: "DriftScore", columnName: "drift_score"},
		{fieldName: "ContradictionScore", columnName: "contradiction_score"},
		{fieldName: "ValidationSummary", columnName: "validation_summary"},
		{fieldName: "SignalClusterKey", columnName: "signal_cluster_key"},
		{fieldName: "SignalTagsJSON", columnName: "signal_tags_json"},
		{fieldName: "SignalClustersJSON", columnName: "signal_clusters_json"},
		{fieldName: "DecisionEvidenceJSON", columnName: "decision_evidence_json"},
	}
	for _, column := range requiredColumns {
		if s.db.Migrator().HasColumn(&DealReviewSymbolBehaviorPrior{}, column.fieldName) {
			continue
		}
		if err := s.db.Migrator().AddColumn(&DealReviewSymbolBehaviorPrior{}, column.fieldName); err != nil {
			return fmt.Errorf("failed to add %s column: %w", column.columnName, err)
		}
	}
	return nil
}

func (s *DealReviewStore) RebuildSymbolBehaviorPriors(userID, traderID string) (*DealReviewSymbolBehaviorPriorRefreshResult, error) {
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
	decisionRecords, err := s.listDecisionRecordsForSymbolPriors(traderID)
	if err != nil {
		return nil, err
	}

	priors := buildDealReviewSymbolBehaviorPriors(cases, decisionRecords, now)
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&DealReviewSymbolBehaviorPrior{}).Error; err != nil {
			return err
		}
		if len(priors) == 0 {
			return nil
		}
		for idx := range priors {
			priors[idx].UserID = userID
			priors[idx].TraderID = traderID
			priors[idx].BuiltAt = now
		}
		return dealReviewCreateInBatches(tx, priors)
	}); err != nil {
		return nil, err
	}

	return &DealReviewSymbolBehaviorPriorRefreshResult{
		Rebuilt:     true,
		ItemCount:   len(priors),
		GeneratedAt: now,
	}, nil
}

func (s *DealReviewStore) listDecisionRecordsForSymbolPriors(traderID string) ([]*DecisionRecord, error) {
	traderID = strings.TrimSpace(traderID)
	if traderID == "" {
		return nil, nil
	}
	var rows []DecisionRecordDB
	if err := s.db.Model(&DecisionRecordDB{}).
		Where("trader_id = ? AND candidate_metadata_version > 0 AND decisions IS NOT NULL AND decisions <> '[]' AND LOWER(TRIM(decisions)) <> 'null'", traderID).
		Order("timestamp DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	records := make([]*DecisionRecord, 0, len(rows))
	for idx := range rows {
		records = append(records, rows[idx].toRecord())
	}
	return records, nil
}

func (s *DealReviewStore) ListSymbolBehaviorPriors(userID, traderID string, filter DealReviewSymbolBehaviorPriorFilter) ([]DealReviewSymbolBehaviorPrior, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 25
	}
	query := s.db.Model(&DealReviewSymbolBehaviorPrior{}).
		Where("user_id = ? AND trader_id = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID))

	if symbol := strings.ToUpper(strings.TrimSpace(filter.Symbol)); symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	if side := normalizeDealReviewSide(filter.Side); side != "" {
		query = query.Where("side = ?", side)
	}
	if status := strings.ToLower(strings.TrimSpace(filter.Status)); status != "" {
		query = query.Where("lower(status) = ?", status)
	}
	if cluster := normalizeDealReviewSignalClusterLabel(filter.SignalCluster); cluster != "" {
		pattern := fmt.Sprintf("%%\"%s\"%%", cluster)
		query = query.Where("signal_clusters_json LIKE ?", pattern)
	}

	var items []DealReviewSymbolBehaviorPrior
	if err := query.Order("composite_score DESC, sample_count DESC, last_observed_at DESC").Limit(filter.Limit).Find(&items).Error; err != nil {
		return nil, err
	}
	for idx := range items {
		hydrateDealReviewSymbolBehaviorPrior(&items[idx])
	}
	return items, nil
}

func (s *DealReviewStore) ListMatchingSymbolBehaviorPriors(userID, traderID string, caseRec *DealReviewCase, limit int) ([]DealReviewSymbolBehaviorPrior, error) {
	if caseRec == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	var items []DealReviewSymbolBehaviorPrior
	if err := s.db.Model(&DealReviewSymbolBehaviorPrior{}).
		Where("user_id = ? AND trader_id = ? AND symbol = ? AND side = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID), strings.ToUpper(strings.TrimSpace(caseRec.Symbol)), normalizeDealReviewSide(caseRec.Side)).
		Order("composite_score DESC, sample_count DESC, last_observed_at DESC").
		Limit(50).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for idx := range items {
		hydrateDealReviewSymbolBehaviorPrior(&items[idx])
		items[idx].MatchScore = scoreDealReviewSymbolBehaviorPriorMatch(caseRec, &items[idx])
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].MatchScore == items[j].MatchScore {
			if items[i].CompositeScore == items[j].CompositeScore {
				return items[i].SampleCount > items[j].SampleCount
			}
			return items[i].CompositeScore > items[j].CompositeScore
		}
		return items[i].MatchScore > items[j].MatchScore
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *DealReviewStore) countSymbolBehaviorPriors(userID, traderID string) (int, error) {
	var count int64
	if err := s.db.Model(&DealReviewSymbolBehaviorPrior{}).
		Where("user_id = ? AND trader_id = ?", userID, traderID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func buildDealReviewSymbolBehaviorPriors(cases []DealReviewCase, decisionRecords []*DecisionRecord, now time.Time) []DealReviewSymbolBehaviorPrior {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	aggregates := make(map[string]*dealReviewSymbolBehaviorPriorAggregate)
	for idx := range cases {
		caseRec := &cases[idx]
		if !strings.EqualFold(caseRec.Status, DealReviewCaseStatusClosed) {
			continue
		}
		key, agg := dealReviewSymbolBehaviorPriorAggregateForCase(aggregates, caseRec)
		if key == "" || agg == nil {
			continue
		}
		observedAt := dealReviewSymbolBehaviorObservedAt(caseRec)
		recencyWeight := dealReviewSymbolBehaviorRecencyWeight(now, observedAt)

		agg.Total++
		agg.NetPnL += caseRec.RealizedPnL
		agg.NetPnLPct += caseRec.RealizedPnLPct
		agg.SumMFEPct += math.Max(0, caseRec.MaxFavorableExcursionPct)
		agg.SumMAEPct += math.Abs(caseRec.MaxAdverseExcursionPct)
		agg.SumGiveBackPct += caseRec.ProfitGivenBackPct
		agg.SumHoldMs += caseRec.HoldDurationMs
		agg.WeightedCount += recencyWeight
		agg.RecencyWeightSum += recencyWeight
		if agg.FirstObservedAt.IsZero() || observedAt.Before(agg.FirstObservedAt) {
			agg.FirstObservedAt = observedAt
		}
		if agg.LastObservedAt.IsZero() || observedAt.After(agg.LastObservedAt) {
			agg.LastObservedAt = observedAt
		}

		switch strings.ToLower(strings.TrimSpace(caseRec.Outcome)) {
		case "profit":
			agg.Wins++
			agg.WeightedWins += recencyWeight
		case "loss":
			agg.Losses++
			agg.WeightedLosses += recencyWeight
		default:
			if caseRec.RealizedPnL > 0 {
				agg.Wins++
				agg.WeightedWins += recencyWeight
			} else if caseRec.RealizedPnL < 0 {
				agg.Losses++
				agg.WeightedLosses += recencyWeight
			} else {
				agg.Flats++
				agg.WeightedFlats += recencyWeight
			}
		}

		if caseRec.ProfitGivenBackPct >= 40 {
			agg.GiveBackCount++
		}
		if len(agg.Evidence) < dealReviewSymbolBehaviorEvidenceLimit {
			agg.Evidence = append(agg.Evidence, DealReviewSymbolBehaviorPriorEvidence{
				CaseID:          caseRec.ID,
				PositionID:      caseRec.PositionID,
				Outcome:         strings.TrimSpace(caseRec.Outcome),
				RealizedPnL:     caseRec.RealizedPnL,
				RealizedPnLPct:  caseRec.RealizedPnLPct,
				CloseReason:     strings.TrimSpace(caseRec.CloseReason),
				ExitOrigin:      strings.TrimSpace(caseRec.ExitOrigin),
				OpenCycleNumber: caseRec.OpenCycleNumber,
				ExitTimeMs:      caseRec.ExitTimeMs,
			})
		}
		agg.ObservedCases = append(agg.ObservedCases, dealReviewSymbolBehaviorObservedCase{
			ObservedAt:     observedAt,
			Outcome:        strings.TrimSpace(caseRec.Outcome),
			RealizedPnL:    caseRec.RealizedPnL,
			RealizedPnLPct: caseRec.RealizedPnLPct,
		})
	}
	enrichDealReviewSymbolBehaviorAggregatesFromDecisionRecords(aggregates, decisionRecords)

	priors := make([]DealReviewSymbolBehaviorPrior, 0, len(aggregates))
	for _, agg := range aggregates {
		if agg.Total < dealReviewSymbolBehaviorObservedThreshold {
			continue
		}
		prior := buildDealReviewSymbolBehaviorPriorFromAggregate(agg, now)
		priors = append(priors, prior)
	}
	sort.SliceStable(priors, func(i, j int) bool {
		if priors[i].CompositeScore == priors[j].CompositeScore {
			if priors[i].SampleCount == priors[j].SampleCount {
				return priors[i].LastObservedAt.After(priors[j].LastObservedAt)
			}
			return priors[i].SampleCount > priors[j].SampleCount
		}
		return priors[i].CompositeScore > priors[j].CompositeScore
	})
	return priors
}

func dealReviewSymbolBehaviorPriorAggregateForCase(store map[string]*dealReviewSymbolBehaviorPriorAggregate, caseRec *DealReviewCase) (string, *dealReviewSymbolBehaviorPriorAggregate) {
	if caseRec == nil {
		return "", nil
	}
	symbol := strings.ToUpper(strings.TrimSpace(caseRec.Symbol))
	side := normalizeDealReviewSide(caseRec.Side)
	if symbol == "" || side == "" {
		return "", nil
	}
	selectionBucket := normalizeDealReviewSymbolPriorDimension(caseRec.OpenSelectionBucket)
	trend := normalizeDealReviewSymbolPriorDimension(caseRec.OpenTrendRegime)
	volatility := normalizeDealReviewSymbolPriorDimension(caseRec.OpenVolatilityRegime)
	oi := normalizeDealReviewSymbolPriorDimension(caseRec.OpenOIRegime)
	key, regimeSignature := buildDealReviewSymbolBehaviorPriorKey(symbol, side, selectionBucket, trend, volatility, oi)
	agg, ok := store[key]
	if !ok {
		agg = &dealReviewSymbolBehaviorPriorAggregate{
			Symbol:               symbol,
			Side:                 side,
			OpenSelectionBucket:  selectionBucket,
			OpenTrendRegime:      trend,
			OpenVolatilityRegime: volatility,
			OpenOIRegime:         oi,
			RegimeSignature:      regimeSignature,
			Evidence:             make([]DealReviewSymbolBehaviorPriorEvidence, 0, dealReviewSymbolBehaviorEvidenceLimit),
			DecisionCycleNumbers: make(map[int]struct{}),
			SignalTagCounts:      make(map[string]int),
			SignalClusterCounts:  make(map[string]int),
			ObservedCases:        make([]dealReviewSymbolBehaviorObservedCase, 0, dealReviewSymbolBehaviorEvidenceLimit),
			DecisionEvidence:     make([]DealReviewSymbolBehaviorDecisionEvidence, 0, dealReviewSymbolBehaviorDecisionEvidenceLimit),
		}
		store[key] = agg
	}
	return key, agg
}

func buildDealReviewSymbolBehaviorPriorKey(symbol, side, selectionBucket, trend, volatility, oi string) (string, string) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	side = normalizeDealReviewSide(side)
	selectionBucket = normalizeDealReviewSymbolPriorDimension(selectionBucket)
	trend = normalizeDealReviewSymbolPriorDimension(trend)
	volatility = normalizeDealReviewSymbolPriorDimension(volatility)
	oi = normalizeDealReviewSymbolPriorDimension(oi)
	if symbol == "" || side == "" {
		return "", ""
	}
	regimeSignature := fmt.Sprintf("bucket=%s | trend=%s | volatility=%s | oi=%s", selectionBucket, trend, volatility, oi)
	key := strings.Join([]string{symbol, side, selectionBucket, trend, volatility, oi}, "|")
	return key, regimeSignature
}

func dealReviewSymbolBehaviorPriorKeyForCase(caseRec *DealReviewCase) string {
	if caseRec == nil {
		return ""
	}
	key, _ := buildDealReviewSymbolBehaviorPriorKey(
		caseRec.Symbol,
		caseRec.Side,
		caseRec.OpenSelectionBucket,
		caseRec.OpenTrendRegime,
		caseRec.OpenVolatilityRegime,
		caseRec.OpenOIRegime,
	)
	return key
}

func dealReviewSymbolBehaviorPriorKeyForPrior(prior *DealReviewSymbolBehaviorPrior) string {
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

func buildDealReviewSymbolBehaviorPriorFromAggregate(agg *dealReviewSymbolBehaviorPriorAggregate, now time.Time) DealReviewSymbolBehaviorPrior {
	total := float64(agg.Total)
	winRate := aggRate(agg.Wins, agg.Total)
	lossRate := aggRate(agg.Losses, agg.Total)
	avgPnL := agg.NetPnL / total
	avgPnLPct := agg.NetPnLPct / total
	avgMFE := agg.SumMFEPct / total
	avgMAE := agg.SumMAEPct / total
	avgGiveBackPct := agg.SumGiveBackPct / total
	giveBackRate := aggRate(agg.GiveBackCount, agg.Total)
	avgHoldMs := int64(math.Round(float64(agg.SumHoldMs) / total))

	weightedTotal := math.Max(agg.WeightedCount, 1e-9)
	weightedWinRate := agg.WeightedWins / weightedTotal
	weightedLossRate := agg.WeightedLosses / weightedTotal
	weightedFlatRate := agg.WeightedFlats / weightedTotal
	recencyWeight := clampDealReviewUnit(agg.RecencyWeightSum / total)
	sampleScore := clampDealReviewUnit(float64(agg.Total) / 12.0)
	decisionCycleCount := len(agg.DecisionCycleNumbers)
	avgDecisionConfidence := 0.0
	if agg.DecisionOpenCount > 0 {
		avgDecisionConfidence = agg.DecisionConfidenceSum / float64(agg.DecisionOpenCount)
	}

	bias := DealReviewSymbolBehaviorBiasMixed
	if avgPnLPct >= 0.15 && weightedWinRate >= 0.55 {
		bias = DealReviewSymbolBehaviorBiasPositive
	} else if avgPnLPct <= -0.15 && weightedLossRate >= 0.45 {
		bias = DealReviewSymbolBehaviorBiasNegative
	}

	dominantOutcomeRate := math.Max(weightedWinRate, math.Max(weightedLossRate, weightedFlatRate))
	stability := clampDealReviewUnit((dominantOutcomeRate * 0.65) + (sampleScore * 0.35))
	directionality := clampDealReviewUnit(math.Abs(weightedWinRate - weightedLossRate))
	confidence := clampDealReviewUnit((sampleScore * 0.45) + (recencyWeight * 0.20) + (directionality * 0.35))

	var severity float64
	switch bias {
	case DealReviewSymbolBehaviorBiasNegative:
		severity = clampDealReviewUnit(math.Abs(avgPnLPct) / 4.0)
	case DealReviewSymbolBehaviorBiasPositive:
		severity = clampDealReviewUnit(avgPnLPct / 4.0)
	default:
		severity = clampDealReviewUnit(math.Abs(avgPnLPct) / 6.0)
	}
	negativeOutcomeStrength := clampDealReviewUnit((lossRate * 0.55) + (clampDealReviewUnit(math.Abs(avgPnLPct)/2.5) * 0.45))
	openPressure := clampDealReviewUnit(float64(agg.DecisionOpenCount) / 8.0)
	decisionConfidenceNorm := clampDealReviewUnit(avgDecisionConfidence / 100.0)
	contradiction := 0.0
	if bias == DealReviewSymbolBehaviorBiasNegative && agg.DecisionOpenCount > 0 {
		contradiction = clampDealReviewUnit((negativeOutcomeStrength * 0.50) + (openPressure * 0.30) + (decisionConfidenceNorm * 0.20))
	}
	trainingCases, validationCases, recentCases := splitDealReviewSymbolBehaviorObservedCases(agg.ObservedCases)
	validationMetrics := summarizeDealReviewSymbolBehaviorObservedCases(validationCases, bias)
	recentMetrics := summarizeDealReviewSymbolBehaviorObservedCases(recentCases, bias)
	driftScore := computeDealReviewSymbolBehaviorDriftScore(bias, validationMetrics, recentMetrics)
	composite := clampDealReviewUnit(
		(confidence * 0.35) +
			(stability * 0.22) +
			(recencyWeight * 0.08) +
			(severity * 0.15) +
			(contradiction * 0.10) +
			(validationMetrics.SupportScore * 0.07) +
			(recentMetrics.SupportScore * 0.05) -
			(driftScore * 0.08),
	)

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
	recommendedAction := "observe"
	if bias == DealReviewSymbolBehaviorBiasNegative &&
		(status == DealReviewSymbolBehaviorPriorStatusCandidate || status == DealReviewSymbolBehaviorPriorStatusValidated) &&
		(lossRate >= 0.55 || avgPnLPct <= -0.75 || contradiction >= 0.55 || validationMetrics.SupportScore >= 0.60) {
		recommendedAction = "penalize_setup"
	} else if bias == DealReviewSymbolBehaviorBiasPositive &&
		(status == DealReviewSymbolBehaviorPriorStatusCandidate || status == DealReviewSymbolBehaviorPriorStatusValidated) &&
		(winRate >= 0.60 || avgPnLPct >= 0.75 || validationMetrics.SupportScore >= 0.60) {
		recommendedAction = "favor_setup"
	}
	if status == DealReviewSymbolBehaviorPriorStatusRejected || status == DealReviewSymbolBehaviorPriorStatusExpired {
		recommendedAction = "observe"
	}

	summaryDirection := "mixed"
	if bias == DealReviewSymbolBehaviorBiasNegative {
		summaryDirection = "underperformed"
	} else if bias == DealReviewSymbolBehaviorBiasPositive {
		summaryDirection = "outperformed"
	}

	signalTags := topDealReviewLabelCounts(agg.SignalTagCounts, dealReviewSymbolBehaviorSignalTagLimit)
	signalClusters := topDealReviewLabelCounts(agg.SignalClusterCounts, dealReviewSymbolBehaviorSignalClusterLimit)
	signalClusterKey := buildDealReviewSignalClusterKey(signalClusters)
	summary := fmt.Sprintf(
		"%s %s %s in %s: %d wins / %d losses / %d flat across %d deals, avg %.2f%%, give-back rate %.0f%%.",
		agg.Symbol,
		agg.Side,
		summaryDirection,
		agg.RegimeSignature,
		agg.Wins,
		agg.Losses,
		agg.Flats,
		agg.Total,
		avgPnLPct,
		giveBackRate*100,
	)
	if decisionCycleCount > 0 {
		summary += fmt.Sprintf(" Decision engine still opened this setup %d time(s) across %d cycle(s) at %.0f average confidence.", agg.DecisionOpenCount, decisionCycleCount, avgDecisionConfidence)
	}
	if contradiction > 0 {
		summary += fmt.Sprintf(" Contradiction score %.0f%%.", contradiction*100)
	}
	validationSummary := buildDealReviewSymbolBehaviorValidationSummary(
		status,
		bias,
		len(trainingCases),
		validationMetrics,
		recentMetrics,
		driftScore,
		agg.LastObservedAt,
		now,
	)
	if validationSummary != "" {
		summary += " " + validationSummary
	}
	if signalClusterKey != "" {
		summary += fmt.Sprintf(" Normalized signal cluster: %s.", signalClusterKey)
	}
	if len(signalTags) > 0 {
		summary += fmt.Sprintf(" Frequent signal tags: %s.", strings.Join(signalTags, ", "))
	}

	prior := DealReviewSymbolBehaviorPrior{
		ID:                        uuid.NewString(),
		Symbol:                    agg.Symbol,
		Side:                      agg.Side,
		Status:                    status,
		BehaviorBias:              bias,
		RecommendedAction:         recommendedAction,
		OpenSelectionBucket:       agg.OpenSelectionBucket,
		OpenTrendRegime:           agg.OpenTrendRegime,
		OpenVolatilityRegime:      agg.OpenVolatilityRegime,
		OpenOIRegime:              agg.OpenOIRegime,
		RegimeSignature:           agg.RegimeSignature,
		SampleCount:               agg.Total,
		WinningDeals:              agg.Wins,
		LosingDeals:               agg.Losses,
		FlatDeals:                 agg.Flats,
		WinRate:                   winRate,
		LossRate:                  lossRate,
		NetPnL:                    agg.NetPnL,
		AvgPnL:                    avgPnL,
		AvgPnLPct:                 avgPnLPct,
		Expectancy:                avgPnL,
		AvgMFEPct:                 avgMFE,
		AvgMAEPct:                 avgMAE,
		GiveBackRate:              giveBackRate,
		AvgGiveBackPct:            avgGiveBackPct,
		AvgHoldMs:                 avgHoldMs,
		DecisionCycleCount:        decisionCycleCount,
		DecisionOpenCount:         agg.DecisionOpenCount,
		AvgDecisionConfidence:     avgDecisionConfidence,
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
		DriftScore:                driftScore,
		ContradictionScore:        contradiction,
		ConfidenceScore:           confidence,
		RecencyWeight:             recencyWeight,
		StabilityScore:            stability,
		CompositeScore:            composite,
		Summary:                   summary,
		ValidationSummary:         validationSummary,
		SignalClusterKey:          signalClusterKey,
		FirstObservedAt:           agg.FirstObservedAt,
		LastObservedAt:            agg.LastObservedAt,
		SignalTags:                signalTags,
		SignalClusters:            signalClusters,
		Evidence:                  append([]DealReviewSymbolBehaviorPriorEvidence(nil), agg.Evidence...),
		DecisionEvidence:          append([]DealReviewSymbolBehaviorDecisionEvidence(nil), agg.DecisionEvidence...),
	}
	prior.SignalTagsJSON = marshalDealReviewStringArray(prior.SignalTags)
	prior.SignalClustersJSON = marshalDealReviewStringArray(prior.SignalClusters)
	prior.EvidenceJSON = marshalDealReviewSymbolBehaviorPriorEvidence(prior.Evidence)
	prior.DecisionEvidenceJSON = marshalDealReviewSymbolBehaviorDecisionEvidence(prior.DecisionEvidence)
	return prior
}

func hydrateDealReviewSymbolBehaviorPrior(prior *DealReviewSymbolBehaviorPrior) {
	if prior == nil {
		return
	}
	prior.SignalTags = unmarshalDealReviewStringArray(prior.SignalTagsJSON)
	prior.SignalClusters = unmarshalDealReviewStringArray(prior.SignalClustersJSON)
	trimmed := strings.TrimSpace(prior.EvidenceJSON)
	if trimmed == "" {
		prior.Evidence = nil
	} else {
		var evidence []DealReviewSymbolBehaviorPriorEvidence
		if err := json.Unmarshal([]byte(trimmed), &evidence); err == nil {
			prior.Evidence = evidence
		}
	}
	decisionTrimmed := strings.TrimSpace(prior.DecisionEvidenceJSON)
	if decisionTrimmed == "" {
		prior.DecisionEvidence = nil
	} else {
		var decisionEvidence []DealReviewSymbolBehaviorDecisionEvidence
		if err := json.Unmarshal([]byte(decisionTrimmed), &decisionEvidence); err == nil {
			prior.DecisionEvidence = decisionEvidence
		}
	}
	applyDealReviewSymbolBehaviorPriorValidationReport(prior)
}

func marshalDealReviewSymbolBehaviorPriorEvidence(items []DealReviewSymbolBehaviorPriorEvidence) string {
	if len(items) == 0 {
		return "[]"
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func marshalDealReviewSymbolBehaviorDecisionEvidence(items []DealReviewSymbolBehaviorDecisionEvidence) string {
	if len(items) == 0 {
		return "[]"
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func marshalDealReviewStringArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func unmarshalDealReviewStringArray(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}
	return items
}

func dealReviewSymbolBehaviorObservedAt(caseRec *DealReviewCase) time.Time {
	if caseRec == nil {
		return time.Time{}
	}
	if caseRec.ExitTimeMs > 0 {
		return time.UnixMilli(caseRec.ExitTimeMs).UTC()
	}
	if !caseRec.UpdatedAt.IsZero() {
		return caseRec.UpdatedAt.UTC()
	}
	if caseRec.EntryTimeMs > 0 {
		return time.UnixMilli(caseRec.EntryTimeMs).UTC()
	}
	return time.Time{}
}

func dealReviewSymbolBehaviorRecencyWeight(now, observedAt time.Time) float64 {
	if observedAt.IsZero() {
		return 0
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ageHours := now.Sub(observedAt).Hours()
	if ageHours <= 0 {
		return 1
	}
	ageDays := ageHours / 24.0
	return clampDealReviewUnit(math.Pow(0.5, ageDays/dealReviewSymbolBehaviorHalfLifeDays))
}

func normalizeDealReviewSymbolPriorDimension(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	return value
}

func parseDealReviewAggregateTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func enrichDealReviewSymbolBehaviorAggregatesFromDecisionRecords(aggregates map[string]*dealReviewSymbolBehaviorPriorAggregate, records []*DecisionRecord) {
	if len(aggregates) == 0 || len(records) == 0 {
		return
	}
	for _, record := range records {
		if record == nil {
			continue
		}
		candidateBySymbol := make(map[string]CandidateDetail, len(record.CandidateDetails))
		for _, candidate := range record.CandidateDetails {
			symbol := strings.ToUpper(strings.TrimSpace(candidate.Symbol))
			if symbol == "" {
				continue
			}
			candidateBySymbol[symbol] = candidate
		}
		for _, action := range record.Decisions {
			side := dealReviewSymbolBehaviorOpenActionSide(action.Action)
			if side == "" {
				continue
			}
			symbol := strings.ToUpper(strings.TrimSpace(action.Symbol))
			if symbol == "" {
				continue
			}
			candidate := candidateBySymbol[symbol]
			selectionBucket := normalizeDealReviewSymbolPriorDimension(candidate.SelectionBucket)
			trend := normalizeDealReviewSymbolPriorDimension(valueOrActionContext(action.MarketContext, candidate.MarketContext, "trend"))
			volatility := normalizeDealReviewSymbolPriorDimension(valueOrActionContext(action.MarketContext, candidate.MarketContext, "volatility"))
			oi := normalizeDealReviewSymbolPriorDimension(valueOrActionContext(action.MarketContext, candidate.MarketContext, "oi"))
			key := strings.Join([]string{symbol, side, selectionBucket, trend, volatility, oi}, "|")
			agg, ok := aggregates[key]
			if !ok {
				continue
			}
			agg.DecisionOpenCount++
			agg.DecisionConfidenceSum += float64(action.Confidence)
			agg.DecisionCycleNumbers[record.CycleNumber] = struct{}{}
			if agg.DecisionLastSeenAt.IsZero() || record.Timestamp.After(agg.DecisionLastSeenAt) {
				agg.DecisionLastSeenAt = record.Timestamp
			}
			signalTags := extractDealReviewSignalTags(action.Reasoning)
			signalClusters := extractDealReviewNormalizedSignalClusters(action.Reasoning, candidate, action.MarketContext)
			for _, tag := range signalTags {
				agg.SignalTagCounts[tag]++
			}
			for _, label := range signalClusters {
				agg.SignalClusterCounts[label]++
			}
			if len(agg.DecisionEvidence) < dealReviewSymbolBehaviorDecisionEvidenceLimit {
				evidence := DealReviewSymbolBehaviorDecisionEvidence{
					CycleNumber:      record.CycleNumber,
					Timestamp:        record.Timestamp.UTC(),
					Action:           strings.TrimSpace(action.Action),
					Confidence:       action.Confidence,
					Reasoning:        strings.TrimSpace(action.Reasoning),
					CandidateSources: semanticMemoryUniqueSortedStrings(append([]string{}, candidate.Sources...)),
					SignalTags:       signalTags,
					SignalClusterKey: buildDealReviewSignalClusterKey(signalClusters),
					SignalClusters:   signalClusters,
				}
				if action.Execution != nil {
					evidence.TerminalStatus = strings.TrimSpace(action.Execution.TerminalStatus)
				}
				agg.DecisionEvidence = append(agg.DecisionEvidence, evidence)
			}
		}
	}
}

func dealReviewSymbolBehaviorOpenActionSide(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_long":
		return "LONG"
	case "open_short":
		return "SHORT"
	default:
		return ""
	}
}

func valueOrActionContext(actionContext, candidateContext *DealReviewMarketContextSnapshot, dimension string) string {
	switch dimension {
	case "trend":
		if actionContext != nil && strings.TrimSpace(actionContext.TrendRegime) != "" {
			return actionContext.TrendRegime
		}
		if candidateContext != nil {
			return candidateContext.TrendRegime
		}
	case "volatility":
		if actionContext != nil && strings.TrimSpace(actionContext.VolatilityRegime) != "" {
			return actionContext.VolatilityRegime
		}
		if candidateContext != nil {
			return candidateContext.VolatilityRegime
		}
	case "oi":
		if actionContext != nil && strings.TrimSpace(actionContext.OIRegime) != "" {
			return actionContext.OIRegime
		}
		if candidateContext != nil {
			return candidateContext.OIRegime
		}
	}
	return ""
}

func extractDealReviewSignalTags(reasoning string) []string {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(reasoning)), func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})
	seen := map[string]struct{}{}
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		tag = strings.ReplaceAll(tag, " ", "_")
		tag = strings.Trim(tag, "._-")
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func topDealReviewLabelCounts(counts map[string]int, limit int) []string {
	if len(counts) == 0 {
		return nil
	}
	type pair struct {
		tag   string
		count int
	}
	items := make([]pair, 0, len(counts))
	for tag, count := range counts {
		if strings.TrimSpace(tag) == "" || count <= 0 {
			continue
		}
		items = append(items, pair{tag: tag, count: count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].tag < items[j].tag
		}
		return items[i].count > items[j].count
	})
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	result := make([]string, 0, limit)
	for _, item := range items[:limit] {
		result = append(result, item.tag)
	}
	return result
}

func buildDealReviewSignalClusterKey(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	limit := len(labels)
	if limit > 6 {
		limit = 6
	}
	return strings.Join(labels[:limit], " | ")
}

func scoreDealReviewSymbolBehaviorPriorMatch(caseRec *DealReviewCase, prior *DealReviewSymbolBehaviorPrior) float64 {
	if caseRec == nil || prior == nil {
		return 0
	}
	score := 0.15
	if strings.EqualFold(strings.TrimSpace(caseRec.OpenSelectionBucket), strings.TrimSpace(prior.OpenSelectionBucket)) {
		score += 0.25
	}
	if strings.EqualFold(strings.TrimSpace(caseRec.OpenTrendRegime), strings.TrimSpace(prior.OpenTrendRegime)) {
		score += 0.20
	}
	if strings.EqualFold(strings.TrimSpace(caseRec.OpenVolatilityRegime), strings.TrimSpace(prior.OpenVolatilityRegime)) {
		score += 0.20
	}
	if strings.EqualFold(strings.TrimSpace(caseRec.OpenOIRegime), strings.TrimSpace(prior.OpenOIRegime)) {
		score += 0.10
	}
	score += clampDealReviewUnit(prior.CompositeScore) * 0.08
	score += clampDealReviewUnit(prior.ContradictionScore) * 0.07
	return clampDealReviewUnit(score)
}

func aggRate(value, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) / float64(total)
}
