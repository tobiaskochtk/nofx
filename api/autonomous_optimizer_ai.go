package api

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"nofx/decision"
	"nofx/store"
)

const (
	autonomousOptimizerMinClosedDealsForConfigProposal = 3
	autonomousOptimizerMinCyclesForLowTradeReview      = 12
	autonomousOptimizerMinCandidatesForLowTradeReview  = 50
	autonomousOptimizerMaxConfigPatchPaths             = 8
	autonomousOptimizerMaxPromptPatchFields            = 6
	autonomousOptimizerMaxPromptFieldChars             = 6000
	autonomousOptimizerEarlyTighteningMinutes          = 15
	autonomousOptimizerSameSessionGapHours             = 12
)

type autonomousOptimizerWindowBundle struct {
	WindowStart  time.Time
	WindowEnd    time.Time
	Filter       store.DealReviewListFilter
	Summary      *store.DealReviewDatasetSummary
	Cases        []store.DealReviewCaseDetail
	BucketReview *store.TraderBucketReview
	Metadata     autonomousOptimizerRunMetadata
}

type autonomousOptimizerCycleResult struct {
	RunStatus                string
	ConfigStatus             string
	Summary                  string
	Metadata                 map[string]any
	ConfigPatch              map[string]any
	PromptPatch              map[string]any
	Validation               map[string]any
	AppliedStrategyVersionID string
	NextRunAt                time.Time
}

type autonomousOptimizerProposalResult struct {
	ExecutiveSummary   string                               `json:"executive_summary"`
	ProposalType       string                               `json:"proposal_type"`
	Rationale          []string                             `json:"rationale,omitempty"`
	ExpectedEffect     string                               `json:"expected_effect,omitempty"`
	EvidenceStrength   float64                              `json:"evidence_strength,omitempty"`
	Confidence         float64                              `json:"confidence,omitempty"`
	SymbolPriorRefs    []string                             `json:"symbol_prior_references,omitempty"`
	LearnedPatternRefs []string                             `json:"learned_pattern_references,omitempty"`
	ConfigPatch        map[string]any                       `json:"config_patch,omitempty"`
	PromptPatch        map[string]any                       `json:"prompt_patch,omitempty"`
	BacklogItems       []autonomousOptimizerBacklogProposal `json:"backlog_items,omitempty"`
	PauseReason        string                               `json:"pause_reason,omitempty"`
}

type autonomousOptimizerBacklogProposal struct {
	Title              string           `json:"title"`
	Category           string           `json:"category"`
	Description        string           `json:"description"`
	ExpectedImpact     string           `json:"expected_impact"`
	Confidence         float64          `json:"confidence"`
	ImplementationCost float64          `json:"implementation_cost"`
	Urgency            float64          `json:"urgency"`
	RecurrenceCount    int              `json:"recurrence_count"`
	Evidence           []map[string]any `json:"evidence,omitempty"`
	Metadata           map[string]any   `json:"metadata,omitempty"`
}

type autonomousOptimizerCriticResult struct {
	Approved           bool     `json:"approved"`
	Confidence         float64  `json:"confidence"`
	RecommendedAction  string   `json:"recommended_action"`
	Summary            string   `json:"summary"`
	SymbolPriorRefs    []string `json:"symbol_prior_references,omitempty"`
	LearnedPatternRefs []string `json:"learned_pattern_references,omitempty"`
	BlockingIssues     []string `json:"blocking_issues,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
}

type autonomousOptimizerSymbolPriorEvidence struct {
	PriorKey               string   `json:"prior_key"`
	Symbol                 string   `json:"symbol"`
	Side                   string   `json:"side"`
	Status                 string   `json:"status"`
	BehaviorBias           string   `json:"behavior_bias"`
	ValidationLabel        string   `json:"validation_label,omitempty"`
	ValidationAlert        string   `json:"validation_alert,omitempty"`
	RecommendedAction      string   `json:"recommended_action"`
	RegimeSignature        string   `json:"regime_signature"`
	SampleCount            int      `json:"sample_count"`
	WinRate                float64  `json:"win_rate"`
	LossRate               float64  `json:"loss_rate"`
	AvgPnLPct              float64  `json:"avg_pnl_pct"`
	CompositeScore         float64  `json:"composite_score"`
	ContradictionScore     float64  `json:"contradiction_score"`
	DecisionOpenCount      int      `json:"decision_open_count"`
	DecisionCycleCount     int      `json:"decision_cycle_count"`
	AvgDecisionConfidence  float64  `json:"avg_decision_confidence"`
	TrainingSampleCount    int      `json:"training_sample_count"`
	ValidationSampleCount  int      `json:"validation_sample_count"`
	ValidationSupportCount int      `json:"validation_support_count"`
	ValidationSupportScore float64  `json:"validation_support_score"`
	ValidationAvgPnLPct    float64  `json:"validation_avg_pnl_pct"`
	RecentSampleCount      int      `json:"recent_sample_count"`
	RecentSupportCount     int      `json:"recent_support_count"`
	RecentSupportScore     float64  `json:"recent_support_score"`
	RecentAvgPnLPct        float64  `json:"recent_avg_pnl_pct"`
	DriftScore             float64  `json:"drift_score"`
	FalsePositiveScore     float64  `json:"false_positive_score"`
	FalseNegativeScore     float64  `json:"false_negative_score"`
	CurrentWindowMatchType string   `json:"current_window_match_type"`
	ClosedCaseMatchCount   int      `json:"closed_case_match_count"`
	RecentExecutionMatches int      `json:"recent_execution_match_count"`
	OpportunitySymbolHits  int      `json:"opportunity_symbol_match_count"`
	CurrentWindowNetPnL    float64  `json:"current_window_net_pnl"`
	ImplicationType        string   `json:"implication_type"`
	ImplicationSummary     string   `json:"implication_summary"`
	SignalTags             []string `json:"signal_tags,omitempty"`
	EvidenceCaseIDs        []string `json:"evidence_case_ids,omitempty"`
	Summary                string   `json:"summary,omitempty"`
}

type autonomousOptimizerSymbolClusterEvidence struct {
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
	SuggestedUse              string   `json:"suggested_use,omitempty"`
}

type autonomousOptimizerSymbolPriorPayload struct {
	AvailableCount         int                                        `json:"available_count"`
	CandidateCount         int                                        `json:"candidate_count"`
	ValidatedCount         int                                        `json:"validated_count"`
	RelevantCount          int                                        `json:"relevant_count"`
	ConfirmedCount         int                                        `json:"confirmed_count"`
	FalsePositiveCount     int                                        `json:"false_positive_count"`
	FalseNegativeRiskCount int                                        `json:"false_negative_risk_count"`
	DriftingCount          int                                        `json:"drifting_count"`
	ConfigCandidateCount   int                                        `json:"config_candidate_count"`
	PromptOnlyCount        int                                        `json:"prompt_only_count"`
	BacklogCandidateCount  int                                        `json:"backlog_candidate_count"`
	Items                  []autonomousOptimizerSymbolPriorEvidence   `json:"items,omitempty"`
	ClusterRollups         []autonomousOptimizerSymbolClusterEvidence `json:"cluster_rollups,omitempty"`
	Notes                  []string                                   `json:"notes,omitempty"`
}

type autonomousOptimizerLearnedPatternEvidence struct {
	PatternID                   string   `json:"pattern_id"`
	ScopeType                   string   `json:"scope_type"`
	Symbol                      string   `json:"symbol,omitempty"`
	Side                        string   `json:"side"`
	PatternClass                string   `json:"pattern_class"`
	Status                      string   `json:"status,omitempty"`
	ValidationLabel             string   `json:"validation_label,omitempty"`
	RecommendedUse              string   `json:"recommended_use,omitempty"`
	PatternSignature            string   `json:"pattern_signature,omitempty"`
	RegimeSignature             string   `json:"regime_signature,omitempty"`
	PatternOrder                int      `json:"pattern_order,omitempty"`
	FeatureCount                int      `json:"feature_count,omitempty"`
	FeatureSet                  []string `json:"feature_set,omitempty"`
	SampleCount                 int      `json:"sample_count"`
	SupportCount                int      `json:"support_count"`
	ContradictCount             int      `json:"contradict_count"`
	WinRate                     float64  `json:"win_rate"`
	LossRate                    float64  `json:"loss_rate"`
	AvgPnLPct                   float64  `json:"avg_pnl_pct"`
	LiftAvgPnLPct               float64  `json:"lift_avg_pnl_pct"`
	Expectancy                  float64  `json:"expectancy"`
	AvgMFEPct                   float64  `json:"avg_mfe_pct"`
	AvgMAEPct                   float64  `json:"avg_mae_pct"`
	GiveBackRate                float64  `json:"give_back_rate"`
	AvgGiveBackPct              float64  `json:"avg_give_back_pct"`
	ConfidenceScore             float64  `json:"confidence_score"`
	StabilityScore              float64  `json:"stability_score"`
	DriftScore                  float64  `json:"drift_score"`
	CompositeScore              float64  `json:"composite_score"`
	FalsePositiveScore          float64  `json:"false_positive_score"`
	ReverseRiskScore            float64  `json:"reverse_risk_score"`
	TrainingSampleCount         int      `json:"training_sample_count"`
	ValidationSampleCount       int      `json:"validation_sample_count"`
	ValidationSupportCount      int      `json:"validation_support_count"`
	ValidationSupportScore      float64  `json:"validation_support_score"`
	RecentSampleCount           int      `json:"recent_sample_count"`
	RecentSupportCount          int      `json:"recent_support_count"`
	RecentSupportScore          float64  `json:"recent_support_score"`
	CurrentWindowMatchType      string   `json:"current_window_match_type"`
	ClosedCaseMatchCount        int      `json:"closed_case_match_count"`
	RecentExecutionMatchCount   int      `json:"recent_execution_match_count"`
	OpportunitySymbolMatchCount int      `json:"opportunity_symbol_match_count"`
	CurrentWindowNetPnL         float64  `json:"current_window_net_pnl"`
	ImplicationType             string   `json:"implication_type"`
	ImplicationSummary          string   `json:"implication_summary"`
	EvidenceCaseIDs             []string `json:"evidence_case_ids,omitempty"`
	Summary                     string   `json:"summary,omitempty"`
	ValidationAlert             string   `json:"validation_alert,omitempty"`
}

type autonomousOptimizerLearnedPatternPayload struct {
	AvailableCount       int                                         `json:"available_count"`
	RelevantCount        int                                         `json:"relevant_count"`
	PositiveCount        int                                         `json:"positive_count"`
	NegativeCount        int                                         `json:"negative_count"`
	ConfirmedCount       int                                         `json:"confirmed_count"`
	CandidateCount       int                                         `json:"candidate_count"`
	FalsePositiveCount   int                                         `json:"false_positive_count"`
	ReverseRiskCount     int                                         `json:"reverse_risk_count"`
	DriftingCount        int                                         `json:"drifting_count"`
	ExpiredCount         int                                         `json:"expired_count"`
	ConfigCandidateCount int                                         `json:"config_candidate_count"`
	PromptOnlyCount      int                                         `json:"prompt_only_count"`
	ReviewHintCount      int                                         `json:"review_hint_count"`
	MonitorOnlyCount     int                                         `json:"monitor_only_count"`
	DoNotUseCount        int                                         `json:"do_not_use_count"`
	Items                []autonomousOptimizerLearnedPatternEvidence `json:"items,omitempty"`
	TopPositivePatterns  []autonomousOptimizerLearnedPatternEvidence `json:"top_positive_patterns,omitempty"`
	TopNegativePatterns  []autonomousOptimizerLearnedPatternEvidence `json:"top_negative_patterns,omitempty"`
	TopSymbolOverrides   []autonomousOptimizerLearnedPatternEvidence `json:"top_symbol_overrides,omitempty"`
	Notes                []string                                    `json:"notes,omitempty"`
}

type autonomousOptimizerPromptPatch struct {
	Strategy  *autonomousOptimizerStrategyPromptPatch  `json:"strategy,omitempty"`
	Trader    *autonomousOptimizerTraderPromptPatch    `json:"trader,omitempty"`
	Optimizer *autonomousOptimizerOptimizerPromptPatch `json:"optimizer,omitempty"`
}

type autonomousOptimizerStrategyPromptPatch struct {
	CustomPrompt   *string                                 `json:"custom_prompt,omitempty"`
	PromptSections *autonomousOptimizerPromptSectionsPatch `json:"prompt_sections,omitempty"`
}

type autonomousOptimizerPromptSectionsPatch struct {
	RoleDefinition   *string `json:"role_definition,omitempty"`
	TradingFrequency *string `json:"trading_frequency,omitempty"`
	EntryStandards   *string `json:"entry_standards,omitempty"`
	MarketContext    *string `json:"market_context,omitempty"`
	DecisionProcess  *string `json:"decision_process,omitempty"`
	DecisionFormat   *string `json:"decision_format,omitempty"`
}

type autonomousOptimizerTraderPromptPatch struct {
	CustomPrompt         *string `json:"custom_prompt,omitempty"`
	OverrideBasePrompt   *bool   `json:"override_base_prompt,omitempty"`
	SystemPromptTemplate *string `json:"system_prompt_template,omitempty"`
}

type autonomousOptimizerOptimizerPromptPatch struct {
	ProposalInstructions *string `json:"proposal_instructions,omitempty"`
	CriticInstructions   *string `json:"critic_instructions,omitempty"`
}

type autonomousOptimizerPromptValidation struct {
	ChangedFields       []string `json:"changed_fields,omitempty"`
	ChangedFieldCount   int      `json:"changed_field_count,omitempty"`
	RequestedFields     []string `json:"requested_fields,omitempty"`
	RequestedFieldCount int      `json:"requested_field_count,omitempty"`
	DeferredFields      []string `json:"deferred_fields,omitempty"`
	AppliedFieldLimit   int      `json:"applied_field_limit,omitempty"`
	AutoTrimmed         bool     `json:"auto_trimmed,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
	BlockingIssues      []string `json:"blocking_issues,omitempty"`
	EstimatedTokens     int      `json:"estimated_tokens,omitempty"`
	ContextLimit        int      `json:"context_limit,omitempty"`
}

type autonomousOptimizerPromptFieldCandidate struct {
	Label    string
	Priority int
	Apply    func()
}

type autonomousOptimizerCooldownSymbolAgg struct {
	Symbol               string
	ReentryCount         int
	RepeatAfterLossCount int
	PnLSum               float64
	LastGapMinutes       float64
	BlockedOutcomes      autonomousOptimizerCooldownOutcomeAgg
	PostCooldownOutcomes autonomousOptimizerCooldownOutcomeAgg
}

type autonomousOptimizerCooldownRegimeAgg struct {
	TrendRegime          string
	VolatilityRegime     string
	OIRegime             string
	ReentryCount         int
	RepeatAfterLossCount int
	PnLSum               float64
	BlockedOutcomes      autonomousOptimizerCooldownOutcomeAgg
	PostCooldownOutcomes autonomousOptimizerCooldownOutcomeAgg
}

type autonomousOptimizerCooldownOutcomeAgg struct {
	TradeCount int
	WinCount   int
	LossCount  int
	NetPnLPct  float64
}

func autonomousOptimizerPromptFieldPriority(label string) int {
	switch strings.TrimSpace(label) {
	case "strategy.prompt_sections.entry_standards":
		return 10
	case "strategy.prompt_sections.market_context":
		return 20
	case "strategy.prompt_sections.decision_process":
		return 30
	case "strategy.prompt_sections.trading_frequency":
		return 40
	case "strategy.prompt_sections.role_definition":
		return 50
	case "strategy.prompt_sections.decision_format":
		return 60
	case "strategy.custom_prompt":
		return 70
	case "trader.custom_prompt":
		return 80
	case "optimizer.proposal_instructions":
		return 90
	case "optimizer.critic_instructions":
		return 100
	case "trader.system_prompt_template":
		return 110
	case "trader.override_base_prompt":
		return 120
	default:
		return 999
	}
}

func autonomousOptimizerPromptCandidateLabels(candidates []autonomousOptimizerPromptFieldCandidate) []string {
	if len(candidates) == 0 {
		return nil
	}
	labels := make([]string, 0, len(candidates))
	for _, item := range candidates {
		if strings.TrimSpace(item.Label) == "" {
			continue
		}
		labels = append(labels, item.Label)
	}
	return labels
}

func (s *Server) collectAutonomousOptimizerWindowBundle(cfg *store.AutonomousOptimizerConfig, now time.Time) (*autonomousOptimizerWindowBundle, error) {
	window := time.Duration(cfg.ReviewIntervalHours) * time.Hour
	if window <= 0 {
		window = time.Duration(store.AutonomousOptimizerDefaultReviewIntervalHours) * time.Hour
	}
	windowStart := now.Add(-window)
	if !cfg.LastRunAt.IsZero() && cfg.LastRunAt.After(windowStart) {
		windowStart = cfg.LastRunAt
	}

	filter := store.DealReviewListFilter{
		TraderID: cfg.TraderID,
		FromTime: windowStart.UnixMilli(),
		ToTime:   now.UnixMilli(),
		Limit:    200,
		Offset:   0,
	}

	cases, summary, err := s.store.DealReview().ListAnalysisCases(cfg.UserID, filter, 120)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = &store.DealReviewDatasetSummary{}
	}

	bucketReview, err := s.store.Decision().GetBucketReview(cfg.TraderID, window, 50)
	if err != nil {
		return nil, err
	}
	if bucketReview == nil {
		bucketReview = &store.TraderBucketReview{}
	}

	return &autonomousOptimizerWindowBundle{
		WindowStart:  windowStart,
		WindowEnd:    now,
		Filter:       filter,
		Summary:      summary,
		Cases:        cases,
		BucketReview: bucketReview,
		Metadata: autonomousOptimizerRunMetadata{
			WindowStartMs:            windowStart.UnixMilli(),
			WindowEndMs:              now.UnixMilli(),
			ReviewIntervalHours:      cfg.ReviewIntervalHours,
			ClosedDeals:              summary.ClosedDeals,
			OpenDeals:                summary.OpenDeals,
			WinningDeals:             summary.WinningDeals,
			LosingDeals:              summary.LosingDeals,
			NetPnL:                   summary.NetPnL,
			AvgPnL:                   summary.AvgPnL,
			Expectancy:               summary.Expectancy,
			WinRate:                  summary.WinRate,
			ProfitFactor:             summary.ProfitFactor,
			MaxDrawdownPct:           summary.MaxDrawdown,
			DecisionRecordCount:      bucketReview.RecordCount,
			DecisionCandidateCount:   bucketReview.TotalCandidates,
			OpenDecisionCount:        bucketReview.TotalOpenDecisions,
			RejectedCandidateCount:   bucketReview.RejectedCandidateCount,
			CyclesWithCandidates:     bucketReview.CyclesWithCandidates,
			CyclesWithOpenDecisions:  bucketReview.CyclesWithOpenDecisions,
			HoldDecisionCount:        bucketReview.HoldDecisionCount,
			WaitDecisionCount:        bucketReview.WaitDecisionCount,
			DecisionConversionRate:   bucketReview.DecisionConversionRate,
			AvgDecisionConfidence:    bucketReview.AvgDecisionConfidence,
			AvgMFECapturedPct:        summary.AvgMFECapturedPct,
			AvgProfitGivenBackPct:    summary.AvgProfitGivenBackPct,
			AvgExitEfficiencyScore:   summary.AvgExitEfficiencyScore,
			AvgEntryTimingScore:      summary.AvgEntryTimingScore,
			AvgRiskSizingScore:       summary.AvgRiskSizingScore,
			BadEntryDeals:            summary.BadEntryDeals,
			BadExitDeals:             summary.BadExitDeals,
			AvoidableLossDeals:       summary.AvoidableLossDeals,
			StrongEntryWeakExitDeals: summary.StrongEntryWeakExitDeals,
			RejectReasons:            bucketReview.RejectReasons,
			ConfidenceBands:          bucketReview.ConfidenceBands,
			OpportunitySessions:      bucketReview.OpportunitySessions,
			OpportunitySymbols:       bucketReview.OpportunitySymbols,
			ExecutionStatuses:        bucketReview.ExecutionStatuses,
			RecentOpenExecutions:     bucketReview.RecentOpenExecutions,
			RegimeSummaries:          bucketReview.RegimeSummaries,
		},
	}, nil
}

func (s *Server) attachAutonomousOptimizerTelemetry(cfg *store.AutonomousOptimizerConfig, strategyCfg *store.StrategyConfig, bundle *autonomousOptimizerWindowBundle) error {
	if cfg == nil || bundle == nil {
		return nil
	}

	positionIDs := autonomousOptimizerTrailingTelemetryPositionIDs(bundle.Cases)
	firstTrailingUpdates := map[int64]store.DealReviewTrailingUpdateRecord{}
	if len(positionIDs) > 0 {
		updates, err := s.store.DealReview().GetFirstTrailingUpdatesByPosition(cfg.UserID, cfg.TraderID, positionIDs)
		if err != nil {
			return err
		}
		firstTrailingUpdates = updates
	}

	bundle.Metadata.TrailingStopTelemetry = buildAutonomousOptimizerTrailingStopTelemetry(bundle.Cases, firstTrailingUpdates)
	bundle.Metadata.AdaptiveCooldownTelemetry = buildAutonomousOptimizerAdaptiveCooldownTelemetry(strategyCfg, bundle.Cases)
	return nil
}

func autonomousOptimizerTrailingTelemetryPositionIDs(cases []store.DealReviewCaseDetail) []int64 {
	seen := make(map[int64]struct{})
	ids := make([]int64, 0, len(cases))
	for _, detail := range cases {
		caseRec := detail.Case
		if normalizeAutonomousOptimizerCloseReason(caseRec.CloseReason) != "trailing_stop" {
			continue
		}
		if caseRec.PositionID <= 0 {
			continue
		}
		if _, exists := seen[caseRec.PositionID]; exists {
			continue
		}
		seen[caseRec.PositionID] = struct{}{}
		ids = append(ids, caseRec.PositionID)
	}
	return ids
}

func buildAutonomousOptimizerTrailingStopTelemetry(cases []store.DealReviewCaseDetail, firstTrailingUpdates map[int64]store.DealReviewTrailingUpdateRecord) *autonomousOptimizerTrailingStopTelemetry {
	telemetry := &autonomousOptimizerTrailingStopTelemetry{}
	var trailingPnLSum float64
	var stopLossPnLSum float64
	var minutesToFirstSum float64
	var minutesFromFirstToExitSum float64
	var timedTrailingSamples int
	auditItems := make([]autonomousOptimizerTrailingStopUpdateAuditItem, 0)

	for _, detail := range cases {
		caseRec := detail.Case
		if !strings.EqualFold(caseRec.Status, store.DealReviewCaseStatusClosed) {
			continue
		}
		if record, ok := firstTrailingUpdates[caseRec.PositionID]; ok && record.TimestampMs > 0 {
			item := autonomousOptimizerTrailingStopUpdateAuditItem{
				PositionID:                caseRec.PositionID,
				Symbol:                    strings.TrimSpace(caseRec.Symbol),
				Side:                      strings.TrimSpace(caseRec.Side),
				CloseReason:               normalizeAutonomousOptimizerCloseReason(caseRec.CloseReason),
				TrailingMode:              strings.TrimSpace(record.TrailingMode),
				UpdateTimeMs:              record.TimestampMs,
				PreUpdateUnrealizedPnL:    roundAutonomousOptimizerFloat(record.UnrealizedPnL, 4),
				PreUpdateUnrealizedPnLPct: roundAutonomousOptimizerFloat(record.UnrealizedPnLPct, 2),
				StopProfitPct:             roundAutonomousOptimizerFloat(record.StopProfitPct, 2),
				ProtectsBreakeven:         record.ProtectsBreakeven,
				RealizedPnLPct:            roundAutonomousOptimizerFloat(caseRec.RealizedPnLPct, 2),
				PreviousStopPrice:         roundAutonomousOptimizerFloat(record.PreviousStopPrice, 8),
				NewStopPrice:              roundAutonomousOptimizerFloat(record.NewStopPrice, 8),
				TierTriggerProfitPct:      roundAutonomousOptimizerFloat(record.TierTriggerProfitPct, 2),
			}
			if record.TimestampMs > caseRec.EntryTimeMs && caseRec.EntryTimeMs > 0 {
				item.MinutesToFirstUpdate = roundAutonomousOptimizerFloat(float64(record.TimestampMs-caseRec.EntryTimeMs)/60000, 1)
			}
			if caseRec.ExitTimeMs > record.TimestampMs && record.TimestampMs > 0 {
				item.MinutesFromUpdateToExit = roundAutonomousOptimizerFloat(float64(caseRec.ExitTimeMs-record.TimestampMs)/60000, 1)
			}
			telemetry.FirstUpdateAuditCount++
			if record.ProtectsBreakeven {
				telemetry.BreakevenProtectedCount++
			}
			auditItems = append(auditItems, item)
		}

		switch normalizeAutonomousOptimizerCloseReason(caseRec.CloseReason) {
		case "trailing_stop":
			telemetry.TrailingExitCount++
			trailingPnLSum += caseRec.RealizedPnLPct
			switch {
			case caseRec.RealizedPnLPct > 0:
				telemetry.TrailingProfitExitCount++
			case caseRec.RealizedPnLPct < 0:
				telemetry.TrailingLossExitCount++
			}

			updateMs := int64(0)
			if record, ok := firstTrailingUpdates[caseRec.PositionID]; ok && record.TimestampMs > 0 {
				updateMs = record.TimestampMs
			} else if caseRec.ExitEvidence != nil && caseRec.ExitEvidence.TrailingUpdatedAtMs > 0 {
				updateMs = caseRec.ExitEvidence.TrailingUpdatedAtMs
			}
			if updateMs > caseRec.EntryTimeMs && caseRec.ExitTimeMs > updateMs {
				minutesToFirst := float64(updateMs-caseRec.EntryTimeMs) / 60000
				minutesFromFirstToExit := float64(caseRec.ExitTimeMs-updateMs) / 60000
				minutesToFirstSum += minutesToFirst
				minutesFromFirstToExitSum += minutesFromFirstToExit
				timedTrailingSamples++
				if minutesToFirst <= autonomousOptimizerEarlyTighteningMinutes {
					telemetry.EarlyTighteningCount++
					if caseRec.RealizedPnLPct < 0 {
						telemetry.EarlyTighteningLossCount++
					}
				}
			}
		case "stop_loss":
			telemetry.InitialStopLossCount++
			stopLossPnLSum += caseRec.RealizedPnLPct
		}
	}

	if telemetry.TrailingExitCount == 0 && telemetry.InitialStopLossCount == 0 && len(auditItems) == 0 {
		return nil
	}
	if telemetry.TrailingExitCount > 0 {
		telemetry.TrailingExitAvgPnLPct = roundAutonomousOptimizerFloat(trailingPnLSum/float64(telemetry.TrailingExitCount), 2)
	}
	if telemetry.InitialStopLossCount > 0 {
		telemetry.InitialStopLossAvgPnLPct = roundAutonomousOptimizerFloat(stopLossPnLSum/float64(telemetry.InitialStopLossCount), 2)
	}
	if timedTrailingSamples > 0 {
		telemetry.AvgMinutesToFirstUpdate = roundAutonomousOptimizerFloat(minutesToFirstSum/float64(timedTrailingSamples), 1)
		telemetry.AvgMinutesFromFirstUpdateToExit = roundAutonomousOptimizerFloat(minutesFromFirstToExitSum/float64(timedTrailingSamples), 1)
	}
	if len(auditItems) > 0 {
		sort.Slice(auditItems, func(i, j int) bool {
			if auditItems[i].RealizedPnLPct == auditItems[j].RealizedPnLPct {
				if auditItems[i].MinutesToFirstUpdate == auditItems[j].MinutesToFirstUpdate {
					return auditItems[i].UpdateTimeMs > auditItems[j].UpdateTimeMs
				}
				return auditItems[i].MinutesToFirstUpdate < auditItems[j].MinutesToFirstUpdate
			}
			return auditItems[i].RealizedPnLPct < auditItems[j].RealizedPnLPct
		})
		if len(auditItems) > 10 {
			auditItems = auditItems[:10]
		}
		telemetry.SampleUpdates = auditItems
	}
	return telemetry
}

func buildAutonomousOptimizerAdaptiveCooldownTelemetry(strategyCfg *store.StrategyConfig, cases []store.DealReviewCaseDetail) *autonomousOptimizerAdaptiveCooldownTelemetry {
	cfg := store.DefaultAdaptiveReentryGuardConfig()
	if strategyCfg != nil {
		cfg = strategyCfg.RiskControl.EffectiveAdaptiveReentryGuard()
	}

	telemetry := &autonomousOptimizerAdaptiveCooldownTelemetry{
		Config: autonomousOptimizerAdaptiveCooldownConfigSnapshot{
			Enabled:                       cfg.Enabled,
			RequireWeakExecutionRegime:    cfg.RequireWeakExecutionRegime,
			RecentTradeWindow:             cfg.RecentTradeWindow,
			MinRecentTrades:               cfg.MinRecentTrades,
			SameSymbolLossCooldownMinutes: cfg.SameSymbolLossCooldownMinutes,
			PairLossLookbackHours:         cfg.PairLossLookbackHours,
		},
	}

	closedCases := make([]store.DealReviewCase, 0, len(cases))
	for _, detail := range cases {
		caseRec := detail.Case
		if strings.EqualFold(caseRec.Status, store.DealReviewCaseStatusClosed) && caseRec.EntryTimeMs > 0 && caseRec.ExitTimeMs > 0 {
			closedCases = append(closedCases, caseRec)
		}
	}
	if len(closedCases) == 0 {
		return telemetry
	}

	sort.Slice(closedCases, func(i, j int) bool {
		if closedCases[i].EntryTimeMs == closedCases[j].EntryTimeMs {
			return closedCases[i].ExitTimeMs < closedCases[j].ExitTimeMs
		}
		return closedCases[i].EntryTimeMs < closedCases[j].EntryTimeMs
	})

	lastBySymbol := make(map[string]store.DealReviewCase)
	lastLossByRegime := make(map[string]store.DealReviewCase)
	symbolAggs := make(map[string]*autonomousOptimizerCooldownSymbolAgg)
	regimeAggs := make(map[string]*autonomousOptimizerCooldownRegimeAgg)
	cooldownWindow := time.Duration(cfg.SameSymbolLossCooldownMinutes) * time.Minute
	regimeLookback := time.Duration(cfg.PairLossLookbackHours) * time.Hour
	sameSessionWindow := time.Duration(autonomousOptimizerSameSessionGapHours) * time.Hour
	blockedSymbolOutcomes := autonomousOptimizerCooldownOutcomeAgg{}
	postSymbolOutcomes := autonomousOptimizerCooldownOutcomeAgg{}
	blockedRegimeOutcomes := autonomousOptimizerCooldownOutcomeAgg{}
	postRegimeOutcomes := autonomousOptimizerCooldownOutcomeAgg{}

	for _, caseRec := range closedCases {
		symbol := strings.ToUpper(strings.TrimSpace(caseRec.Symbol))
		if symbol != "" {
			if prev, ok := lastBySymbol[symbol]; ok && prev.ExitTimeMs > 0 && caseRec.EntryTimeMs > prev.ExitTimeMs {
				gap := time.Duration(caseRec.EntryTimeMs-prev.ExitTimeMs) * time.Millisecond
				sameSession := prev.OpenSessionBucket != "" &&
					prev.OpenSessionBucket == caseRec.OpenSessionBucket &&
					gap <= sameSessionWindow
				if gap <= cooldownWindow || sameSession {
					telemetry.CooldownCandidateCount++
					agg := symbolAggs[symbol]
					if agg == nil {
						agg = &autonomousOptimizerCooldownSymbolAgg{Symbol: symbol}
						symbolAggs[symbol] = agg
					}
					agg.ReentryCount++
					agg.PnLSum += caseRec.RealizedPnLPct
					agg.LastGapMinutes = roundAutonomousOptimizerFloat(gap.Minutes(), 1)
					if prev.RealizedPnLPct < 0 {
						telemetry.RepeatAfterLossCount++
						agg.RepeatAfterLossCount++
						accumulateAutonomousOptimizerCooldownOutcome(&blockedSymbolOutcomes, caseRec)
						accumulateAutonomousOptimizerCooldownOutcome(&agg.BlockedOutcomes, caseRec)
					}
					if sameSession {
						telemetry.SameSessionReentryCount++
					}
				} else if prev.RealizedPnLPct < 0 {
					accumulateAutonomousOptimizerCooldownOutcome(&postSymbolOutcomes, caseRec)
					if agg := symbolAggs[symbol]; agg != nil {
						accumulateAutonomousOptimizerCooldownOutcome(&agg.PostCooldownOutcomes, caseRec)
					}
				}
			}
			lastBySymbol[symbol] = caseRec
		}

		regimeKey := autonomousOptimizerRegimeKey(caseRec)
		if regimeKey != "" {
			if prevLoss, ok := lastLossByRegime[regimeKey]; ok && prevLoss.ExitTimeMs > 0 && caseRec.EntryTimeMs > prevLoss.ExitTimeMs {
				gap := time.Duration(caseRec.EntryTimeMs-prevLoss.ExitTimeMs) * time.Millisecond
				if gap <= regimeLookback {
					telemetry.RegimeRepeatLossCount++
					agg := regimeAggs[regimeKey]
					if agg == nil {
						agg = &autonomousOptimizerCooldownRegimeAgg{
							TrendRegime:      strings.TrimSpace(caseRec.OpenTrendRegime),
							VolatilityRegime: strings.TrimSpace(caseRec.OpenVolatilityRegime),
							OIRegime:         strings.TrimSpace(caseRec.OpenOIRegime),
						}
						regimeAggs[regimeKey] = agg
					}
					agg.ReentryCount++
					agg.RepeatAfterLossCount++
					agg.PnLSum += caseRec.RealizedPnLPct
					accumulateAutonomousOptimizerCooldownOutcome(&blockedRegimeOutcomes, caseRec)
					accumulateAutonomousOptimizerCooldownOutcome(&agg.BlockedOutcomes, caseRec)
				} else {
					accumulateAutonomousOptimizerCooldownOutcome(&postRegimeOutcomes, caseRec)
					if agg := regimeAggs[regimeKey]; agg != nil {
						accumulateAutonomousOptimizerCooldownOutcome(&agg.PostCooldownOutcomes, caseRec)
					}
				}
			}
			if caseRec.RealizedPnLPct < 0 {
				lastLossByRegime[regimeKey] = caseRec
			}
		}
	}

	telemetry.BlockedSymbolReentryOutcomes = finalizeAutonomousOptimizerCooldownOutcome(blockedSymbolOutcomes)
	telemetry.PostSymbolCooldownOutcomes = finalizeAutonomousOptimizerCooldownOutcome(postSymbolOutcomes)
	telemetry.BlockedRegimeReentryOutcomes = finalizeAutonomousOptimizerCooldownOutcome(blockedRegimeOutcomes)
	telemetry.PostRegimeCooldownOutcomes = finalizeAutonomousOptimizerCooldownOutcome(postRegimeOutcomes)
	telemetry.TopSymbols = buildAutonomousOptimizerCooldownSymbols(symbolAggs, 6)
	telemetry.TopRegimes = buildAutonomousOptimizerCooldownRegimes(regimeAggs, 6)
	return telemetry
}

func buildAutonomousOptimizerCooldownSymbols(items map[string]*autonomousOptimizerCooldownSymbolAgg, limit int) []autonomousOptimizerAdaptiveCooldownSymbol {
	if len(items) == 0 {
		return nil
	}
	result := make([]autonomousOptimizerAdaptiveCooldownSymbol, 0, len(items))
	for _, item := range items {
		if item == nil || item.ReentryCount == 0 {
			continue
		}
		result = append(result, autonomousOptimizerAdaptiveCooldownSymbol{
			Symbol:               item.Symbol,
			ReentryCount:         item.ReentryCount,
			RepeatAfterLossCount: item.RepeatAfterLossCount,
			AvgPnLPct:            roundAutonomousOptimizerFloat(item.PnLSum/float64(item.ReentryCount), 2),
			LastGapMinutes:       item.LastGapMinutes,
			BlockedOutcomes:      finalizeAutonomousOptimizerCooldownOutcome(item.BlockedOutcomes),
			PostCooldownOutcomes: finalizeAutonomousOptimizerCooldownOutcome(item.PostCooldownOutcomes),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].RepeatAfterLossCount == result[j].RepeatAfterLossCount {
			if result[i].ReentryCount == result[j].ReentryCount {
				return result[i].Symbol < result[j].Symbol
			}
			return result[i].ReentryCount > result[j].ReentryCount
		}
		return result[i].RepeatAfterLossCount > result[j].RepeatAfterLossCount
	})
	if limit > 0 && len(result) > limit {
		return result[:limit]
	}
	return result
}

func buildAutonomousOptimizerCooldownRegimes(items map[string]*autonomousOptimizerCooldownRegimeAgg, limit int) []autonomousOptimizerAdaptiveCooldownRegime {
	if len(items) == 0 {
		return nil
	}
	result := make([]autonomousOptimizerAdaptiveCooldownRegime, 0, len(items))
	for _, item := range items {
		if item == nil || item.ReentryCount == 0 {
			continue
		}
		result = append(result, autonomousOptimizerAdaptiveCooldownRegime{
			TrendRegime:          item.TrendRegime,
			VolatilityRegime:     item.VolatilityRegime,
			OIRegime:             item.OIRegime,
			ReentryCount:         item.ReentryCount,
			RepeatAfterLossCount: item.RepeatAfterLossCount,
			AvgPnLPct:            roundAutonomousOptimizerFloat(item.PnLSum/float64(item.ReentryCount), 2),
			BlockedOutcomes:      finalizeAutonomousOptimizerCooldownOutcome(item.BlockedOutcomes),
			PostCooldownOutcomes: finalizeAutonomousOptimizerCooldownOutcome(item.PostCooldownOutcomes),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].RepeatAfterLossCount == result[j].RepeatAfterLossCount {
			if result[i].ReentryCount == result[j].ReentryCount {
				leftKey := result[i].TrendRegime + "|" + result[i].VolatilityRegime + "|" + result[i].OIRegime
				rightKey := result[j].TrendRegime + "|" + result[j].VolatilityRegime + "|" + result[j].OIRegime
				return leftKey < rightKey
			}
			return result[i].ReentryCount > result[j].ReentryCount
		}
		return result[i].RepeatAfterLossCount > result[j].RepeatAfterLossCount
	})
	if limit > 0 && len(result) > limit {
		return result[:limit]
	}
	return result
}

func accumulateAutonomousOptimizerCooldownOutcome(agg *autonomousOptimizerCooldownOutcomeAgg, caseRec store.DealReviewCase) {
	if agg == nil {
		return
	}
	agg.TradeCount++
	if caseRec.RealizedPnLPct > 0 {
		agg.WinCount++
	} else if caseRec.RealizedPnLPct < 0 {
		agg.LossCount++
	}
	agg.NetPnLPct += caseRec.RealizedPnLPct
}

func finalizeAutonomousOptimizerCooldownOutcome(agg autonomousOptimizerCooldownOutcomeAgg) autonomousOptimizerCooldownOutcomeSummary {
	summary := autonomousOptimizerCooldownOutcomeSummary{
		TradeCount: agg.TradeCount,
		WinCount:   agg.WinCount,
		LossCount:  agg.LossCount,
		NetPnLPct:  roundAutonomousOptimizerFloat(agg.NetPnLPct, 2),
	}
	if agg.TradeCount > 0 {
		summary.AvgPnLPct = roundAutonomousOptimizerFloat(agg.NetPnLPct/float64(agg.TradeCount), 2)
	}
	return summary
}

func autonomousOptimizerRegimeKey(caseRec store.DealReviewCase) string {
	parts := []string{
		strings.TrimSpace(caseRec.OpenTrendRegime),
		strings.TrimSpace(caseRec.OpenVolatilityRegime),
		strings.TrimSpace(caseRec.OpenOIRegime),
	}
	if parts[0] == "" && parts[1] == "" && parts[2] == "" {
		return ""
	}
	return strings.Join(parts, "|")
}

func normalizeAutonomousOptimizerCloseReason(reason string) string {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "trailing", "trailing-stop", "trailing_stop":
		return "trailing_stop"
	case "stop", "stop-loss", "stop_loss":
		return "stop_loss"
	default:
		return strings.ToLower(strings.TrimSpace(reason))
	}
}

func roundAutonomousOptimizerFloat(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	if factor == 0 {
		return value
	}
	return math.Round(value*factor) / factor
}

func autonomousOptimizerHasProposalEvidence(bundle *autonomousOptimizerWindowBundle) bool {
	if bundle == nil {
		return false
	}
	if bundle.Metadata.ClosedDeals >= autonomousOptimizerMinClosedDealsForConfigProposal {
		return true
	}
	return bundle.Metadata.DecisionRecordCount >= autonomousOptimizerMinCyclesForLowTradeReview &&
		bundle.Metadata.DecisionCandidateCount >= autonomousOptimizerMinCandidatesForLowTradeReview
}

func autonomousOptimizerCanValidateConfigPatch(bundle *autonomousOptimizerWindowBundle) bool {
	if bundle == nil {
		return false
	}
	return bundle.Metadata.ClosedDeals >= autonomousOptimizerMinClosedDealsForConfigProposal
}

func (s *Server) loadAutonomousOptimizerSymbolBehaviorPriorPayload(cfg *store.AutonomousOptimizerConfig, bundle *autonomousOptimizerWindowBundle) (*autonomousOptimizerSymbolPriorPayload, error) {
	if s == nil || s.store == nil || cfg == nil {
		return &autonomousOptimizerSymbolPriorPayload{}, nil
	}
	if _, err := s.store.DealReview().RefreshSymbolBehaviorPriorsIfStale(cfg.UserID, cfg.TraderID); err != nil {
		return nil, err
	}
	priors, err := s.store.DealReview().ListSymbolBehaviorPriors(
		cfg.UserID,
		cfg.TraderID,
		store.DealReviewSymbolBehaviorPriorFilter{Limit: 100},
	)
	if err != nil {
		return nil, err
	}
	return buildAutonomousOptimizerSymbolBehaviorPriorPayload(priors, bundle), nil
}

func buildAutonomousOptimizerSymbolBehaviorPriorPayload(priors []store.DealReviewSymbolBehaviorPrior, bundle *autonomousOptimizerWindowBundle) *autonomousOptimizerSymbolPriorPayload {
	payload := &autonomousOptimizerSymbolPriorPayload{}
	if len(priors) == 0 {
		return payload
	}
	reportingSummary := store.BuildDealReviewSymbolBehaviorPriorReportingSummary(priors)
	if reportingSummary != nil {
		payload.ClusterRollups = buildAutonomousOptimizerSymbolClusterRollups(reportingSummary.TopSignalClusters)
	}

	caseMatchCounts := map[string]int{}
	caseNetPnL := map[string]float64{}
	recentExecutionCounts := map[string]int{}
	opportunityCounts := map[string]int{}
	strongCandidates := make([]autonomousOptimizerSymbolPriorEvidence, 0, len(priors))

	if bundle != nil {
		for _, detail := range bundle.Cases {
			if !strings.EqualFold(detail.Case.Status, store.DealReviewCaseStatusClosed) {
				continue
			}
			key := buildAutonomousOptimizerSymbolPriorKey(
				detail.Case.Symbol,
				detail.Case.Side,
				detail.Case.OpenSelectionBucket,
				detail.Case.OpenTrendRegime,
				detail.Case.OpenVolatilityRegime,
				detail.Case.OpenOIRegime,
			)
			if key == "" {
				continue
			}
			caseMatchCounts[key]++
			caseNetPnL[key] += detail.Case.RealizedPnL
		}
		if bundle.BucketReview != nil {
			for _, item := range bundle.BucketReview.RecentOpenExecutions {
				key := buildAutonomousOptimizerSymbolPriorRegimeKey(
					item.Symbol,
					item.Side,
					item.TrendRegime,
					item.VolatilityRegime,
					item.OIRegime,
				)
				if key != "" {
					recentExecutionCounts[key]++
				}
			}
			for _, item := range bundle.BucketReview.OpportunitySymbols {
				symbol := strings.ToUpper(strings.TrimSpace(item.Symbol))
				if symbol == "" {
					continue
				}
				opportunityCounts[symbol] += item.CandidateCount
			}
		}
	}

	for _, prior := range priors {
		switch strings.TrimSpace(prior.ValidationLabel) {
		case store.DealReviewSymbolBehaviorValidationLabelConfirmed:
			payload.ConfirmedCount++
		case store.DealReviewSymbolBehaviorValidationLabelFalsePositive:
			payload.FalsePositiveCount++
		case store.DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk:
			payload.FalseNegativeRiskCount++
		case store.DealReviewSymbolBehaviorValidationLabelDrifting:
			payload.DriftingCount++
		}
		switch strings.ToLower(strings.TrimSpace(prior.Status)) {
		case store.DealReviewSymbolBehaviorPriorStatusCandidate:
			payload.CandidateCount++
		case store.DealReviewSymbolBehaviorPriorStatusValidated:
			payload.ValidatedCount++
		}
		healthAlert := isAutonomousOptimizerSymbolPriorHealthAlert(&prior)
		if prior.Status != store.DealReviewSymbolBehaviorPriorStatusCandidate &&
			prior.Status != store.DealReviewSymbolBehaviorPriorStatusValidated &&
			!healthAlert {
			continue
		}

		payload.AvailableCount++
		priorKey := buildAutonomousOptimizerSymbolPriorKey(
			prior.Symbol,
			prior.Side,
			prior.OpenSelectionBucket,
			prior.OpenTrendRegime,
			prior.OpenVolatilityRegime,
			prior.OpenOIRegime,
		)
		matchType := "global_background"
		closedMatches := caseMatchCounts[priorKey]
		recentMatches := recentExecutionCounts[buildAutonomousOptimizerSymbolPriorRegimeKey(
			prior.Symbol,
			prior.Side,
			prior.OpenTrendRegime,
			prior.OpenVolatilityRegime,
			prior.OpenOIRegime,
		)]
		opportunityMatches := opportunityCounts[strings.ToUpper(strings.TrimSpace(prior.Symbol))]
		switch {
		case closedMatches > 0:
			matchType = "closed_case"
		case recentMatches > 0:
			matchType = "recent_open_execution"
		case opportunityMatches > 0:
			matchType = "opportunity_symbol"
		}

		implicationType, implicationSummary := classifyAutonomousOptimizerSymbolPriorImplication(
			&prior,
			matchType,
			closedMatches,
			recentMatches,
		)

		item := autonomousOptimizerSymbolPriorEvidence{
			PriorKey:               priorKey,
			Symbol:                 prior.Symbol,
			Side:                   prior.Side,
			Status:                 prior.Status,
			BehaviorBias:           prior.BehaviorBias,
			ValidationLabel:        prior.ValidationLabel,
			ValidationAlert:        clipDealReviewAIScanText(prior.ValidationAlert, 240),
			RecommendedAction:      prior.RecommendedAction,
			RegimeSignature:        prior.RegimeSignature,
			SampleCount:            prior.SampleCount,
			WinRate:                prior.WinRate,
			LossRate:               prior.LossRate,
			AvgPnLPct:              prior.AvgPnLPct,
			CompositeScore:         prior.CompositeScore,
			ContradictionScore:     prior.ContradictionScore,
			DecisionOpenCount:      prior.DecisionOpenCount,
			DecisionCycleCount:     prior.DecisionCycleCount,
			AvgDecisionConfidence:  prior.AvgDecisionConfidence,
			TrainingSampleCount:    prior.TrainingSampleCount,
			ValidationSampleCount:  prior.ValidationSampleCount,
			ValidationSupportCount: prior.ValidationSupportCount,
			ValidationSupportScore: prior.ValidationSupportScore,
			ValidationAvgPnLPct:    prior.ValidationAvgPnLPct,
			RecentSampleCount:      prior.RecentSampleCount,
			RecentSupportCount:     prior.RecentSupportCount,
			RecentSupportScore:     prior.RecentSupportScore,
			RecentAvgPnLPct:        prior.RecentAvgPnLPct,
			DriftScore:             prior.DriftScore,
			FalsePositiveScore:     prior.FalsePositiveScore,
			FalseNegativeScore:     prior.FalseNegativeScore,
			CurrentWindowMatchType: matchType,
			ClosedCaseMatchCount:   closedMatches,
			RecentExecutionMatches: recentMatches,
			OpportunitySymbolHits:  opportunityMatches,
			CurrentWindowNetPnL:    caseNetPnL[priorKey],
			ImplicationType:        implicationType,
			ImplicationSummary:     implicationSummary,
			SignalTags:             limitAutonomousOptimizerStringSlice(prior.SignalTags, 6),
			EvidenceCaseIDs:        buildAutonomousOptimizerPriorEvidenceCaseIDs(prior, 4),
			Summary:                clipDealReviewAIScanText(prior.Summary, 320),
		}

		isRelevant := closedMatches > 0 || recentMatches > 0 || opportunityMatches > 0
		isStrongBackground := !isRelevant && (prior.ContradictionScore >= 0.60 || prior.CompositeScore >= 0.70)
		if isRelevant || isStrongBackground || healthAlert {
			strongCandidates = append(strongCandidates, item)
		}
	}

	sort.Slice(strongCandidates, func(i, j int) bool {
		left := strongCandidates[i]
		right := strongCandidates[j]
		leftHealthRank := autonomousOptimizerSymbolPriorHealthRank(left.ValidationLabel)
		rightHealthRank := autonomousOptimizerSymbolPriorHealthRank(right.ValidationLabel)
		if leftHealthRank != rightHealthRank {
			return leftHealthRank < rightHealthRank
		}
		leftMatchScore := left.ClosedCaseMatchCount*10 + left.RecentExecutionMatches*5 + minInt(left.OpportunitySymbolHits, 1)
		rightMatchScore := right.ClosedCaseMatchCount*10 + right.RecentExecutionMatches*5 + minInt(right.OpportunitySymbolHits, 1)
		if leftMatchScore != rightMatchScore {
			return leftMatchScore > rightMatchScore
		}
		if left.ImplicationType != right.ImplicationType {
			return autonomousOptimizerSymbolPriorImplicationRank(left.ImplicationType) < autonomousOptimizerSymbolPriorImplicationRank(right.ImplicationType)
		}
		if left.ContradictionScore != right.ContradictionScore {
			return left.ContradictionScore > right.ContradictionScore
		}
		if left.CompositeScore != right.CompositeScore {
			return left.CompositeScore > right.CompositeScore
		}
		return left.SampleCount > right.SampleCount
	})
	if len(strongCandidates) > 8 {
		strongCandidates = strongCandidates[:8]
	}
	payload.Items = strongCandidates
	payload.RelevantCount = len(strongCandidates)
	for _, item := range strongCandidates {
		switch item.ImplicationType {
		case "config_candidate":
			payload.ConfigCandidateCount++
		case "prompt_only":
			payload.PromptOnlyCount++
		case "missing_data_backlog":
			payload.BacklogCandidateCount++
		}
	}
	payload.Notes = buildAutonomousOptimizerSymbolPriorNotes(payload)
	return payload
}

func buildAutonomousOptimizerSymbolClusterRollups(items []store.DealReviewSymbolBehaviorClusterSummary) []autonomousOptimizerSymbolClusterEvidence {
	if len(items) == 0 {
		return nil
	}
	limit := len(items)
	if limit > 6 {
		limit = 6
	}
	out := make([]autonomousOptimizerSymbolClusterEvidence, 0, limit)
	for _, item := range items[:limit] {
		suggestedUse := "background_only"
		switch {
		case item.FalsePositiveCount > 0 || item.DriftingCount > 0:
			suggestedUse = "anti_evidence_or_monitoring"
		case item.NegativeBiasCount > item.PositiveBiasCount && item.AvgContradictionScore >= 0.45:
			suggestedUse = "negative_pattern_cluster"
		case item.PositiveBiasCount > item.NegativeBiasCount && item.ConfirmedCount > 0:
			suggestedUse = "positive_pattern_cluster"
		case item.FalseNegativeRiskCount > 0:
			suggestedUse = "reverse_edge_watch"
		}
		out = append(out, autonomousOptimizerSymbolClusterEvidence{
			ClusterLabel:              item.ClusterLabel,
			PriorCount:                item.PriorCount,
			SymbolCount:               item.SymbolCount,
			SampleCount:               item.SampleCount,
			DecisionOpenCount:         item.DecisionOpenCount,
			ConfirmedCount:            item.ConfirmedCount,
			FalsePositiveCount:        item.FalsePositiveCount,
			FalseNegativeRiskCount:    item.FalseNegativeRiskCount,
			DriftingCount:             item.DriftingCount,
			NegativeBiasCount:         item.NegativeBiasCount,
			PositiveBiasCount:         item.PositiveBiasCount,
			AvgPnLPct:                 item.AvgPnLPct,
			AvgContradictionScore:     item.AvgContradictionScore,
			AvgCompositeScore:         item.AvgCompositeScore,
			AvgValidationSupportScore: item.AvgValidationSupportScore,
			TopSymbols:                limitAutonomousOptimizerStringSlice(item.TopSymbols, 4),
			SuggestedUse:              suggestedUse,
		})
	}
	return out
}

func buildAutonomousOptimizerPriorEvidenceCaseIDs(prior store.DealReviewSymbolBehaviorPrior, limit int) []string {
	if len(prior.Evidence) == 0 || limit <= 0 {
		return nil
	}
	ids := make([]string, 0, minInt(len(prior.Evidence), limit))
	for _, item := range prior.Evidence {
		caseID := strings.TrimSpace(item.CaseID)
		if caseID == "" {
			continue
		}
		ids = append(ids, caseID)
		if len(ids) >= limit {
			break
		}
	}
	return ids
}

func limitAutonomousOptimizerStringSlice(items []string, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func buildAutonomousOptimizerSymbolPriorNotes(payload *autonomousOptimizerSymbolPriorPayload) []string {
	if payload == nil {
		return nil
	}
	notes := make([]string, 0, 3)
	if payload.ConfigCandidateCount > 0 {
		notes = append(notes, fmt.Sprintf("%d symbol prior(s) are strong enough for narrow config-level consideration.", payload.ConfigCandidateCount))
	}
	if payload.PromptOnlyCount > 0 {
		notes = append(notes, fmt.Sprintf("%d symbol prior(s) should influence prompt behavior first, not hard config changes.", payload.PromptOnlyCount))
	}
	if payload.BacklogCandidateCount > 0 {
		notes = append(notes, fmt.Sprintf("%d symbol prior(s) suggest missing data or discriminator gaps and should bias toward backlog creation.", payload.BacklogCandidateCount))
	}
	if payload.FalsePositiveCount > 0 {
		notes = append(notes, fmt.Sprintf("%d prior(s) now look like false positives and should be treated as anti-evidence, not config support.", payload.FalsePositiveCount))
	}
	if payload.FalseNegativeRiskCount > 0 {
		notes = append(notes, fmt.Sprintf("%d prior(s) show reverse-edge risk; prefer narrow scans, prompt experiments, or monitoring over blunt config flips.", payload.FalseNegativeRiskCount))
	}
	if payload.DriftingCount > 0 {
		notes = append(notes, fmt.Sprintf("%d prior(s) are drifting, which weakens confidence in any broad patch tied to them.", payload.DriftingCount))
	}
	if len(payload.ClusterRollups) > 0 {
		top := payload.ClusterRollups[0]
		notes = append(notes, fmt.Sprintf("Top normalized cluster %q spans %d prior(s), %d symbol(s), and %d samples; use it to judge whether the issue is broader than a single symbol.", top.ClusterLabel, top.PriorCount, top.SymbolCount, top.SampleCount))
	}
	return notes
}

func isAutonomousOptimizerSymbolPriorHealthAlert(prior *store.DealReviewSymbolBehaviorPrior) bool {
	if prior == nil {
		return false
	}
	switch strings.TrimSpace(prior.ValidationLabel) {
	case store.DealReviewSymbolBehaviorValidationLabelFalsePositive,
		store.DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk,
		store.DealReviewSymbolBehaviorValidationLabelDrifting:
		return true
	default:
		return false
	}
}

func autonomousOptimizerSymbolPriorHealthRank(value string) int {
	switch strings.TrimSpace(value) {
	case store.DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk:
		return 0
	case store.DealReviewSymbolBehaviorValidationLabelFalsePositive:
		return 1
	case store.DealReviewSymbolBehaviorValidationLabelDrifting:
		return 2
	case store.DealReviewSymbolBehaviorValidationLabelConfirmed:
		return 3
	case store.DealReviewSymbolBehaviorValidationLabelCandidate:
		return 4
	default:
		return 5
	}
}

func autonomousOptimizerSymbolPriorImplicationRank(value string) int {
	switch strings.TrimSpace(value) {
	case "config_candidate":
		return 0
	case "prompt_only":
		return 1
	case "missing_data_backlog":
		return 2
	default:
		return 9
	}
}

func classifyAutonomousOptimizerSymbolPriorImplication(prior *store.DealReviewSymbolBehaviorPrior, matchType string, closedMatches, recentMatches int) (string, string) {
	if prior == nil {
		return "missing_data_backlog", "Prior is unavailable."
	}
	switch strings.TrimSpace(prior.ValidationLabel) {
	case store.DealReviewSymbolBehaviorValidationLabelFalsePositive:
		return "prompt_only", "This prior now looks like false-positive anti-evidence. Use it to avoid repeating the old assumption, not to justify a config patch."
	case store.DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk:
		return "prompt_only", "Recent evidence points toward a reverse-edge risk. Prefer narrow prompt experiments, symbol-specific scans, or monitoring before any config flip."
	case store.DealReviewSymbolBehaviorValidationLabelDrifting:
		return "prompt_only", "This prior is drifting. Favor monitoring or narrow prompt caution instead of hard config changes."
	}
	highEvidence := prior.Status == store.DealReviewSymbolBehaviorPriorStatusValidated ||
		(prior.SampleCount >= 8 &&
			prior.CompositeScore >= 0.60 &&
			(prior.ValidationSampleCount < 2 || prior.ValidationSupportScore >= 0.55) &&
			(prior.DriftScore <= 0 || prior.DriftScore <= 0.55))
	strongContradiction := prior.ContradictionScore >= 0.65
	mediumContradiction := prior.ContradictionScore >= 0.45
	repeatedOpens := prior.DecisionOpenCount >= 2
	hasSignalDetail := len(prior.SignalTags) > 0
	directWindowEvidence := closedMatches > 0 || recentMatches > 0

	switch prior.BehaviorBias {
	case store.DealReviewSymbolBehaviorBiasNegative:
		if highEvidence && strongContradiction && repeatedOpens && directWindowEvidence {
			return "config_candidate", "Strong negative symbol prior with direct window relevance; narrow config or risk-control changes are justified."
		}
		if (mediumContradiction || prior.LossRate >= 0.55) && (repeatedOpens || hasSignalDetail) {
			return "prompt_only", "Negative symbol prior should tighten symbol-aware prompt guidance before broader config mutation."
		}
		return "missing_data_backlog", "Negative pattern exists, but evidence is indirect or underspecified; prefer backlog/data expansion."
	case store.DealReviewSymbolBehaviorBiasPositive:
		if highEvidence && directWindowEvidence && prior.RecommendedAction == "favor_setup" {
			return "config_candidate", "Positive symbol prior is stable enough to consider a narrow config relaxation or weighting improvement."
		}
		if repeatedOpens || hasSignalDetail {
			return "prompt_only", "Positive symbol prior is useful for prompt guidance, but not yet strong enough for a durable config change."
		}
		return "missing_data_backlog", "Positive prior exists, but it needs stronger signal detail before changing behavior."
	default:
		if matchType == "global_background" || !hasSignalDetail {
			return "missing_data_backlog", "Mixed prior is too weak or generic; prefer backlog/data work."
		}
		return "prompt_only", "Mixed prior can inform prompt caution, but it does not justify a config change."
	}
}

func buildAutonomousOptimizerSymbolPriorKey(symbol, side, selectionBucket, trend, volatility, oi string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	side = strings.ToUpper(strings.TrimSpace(side))
	selectionBucket = normalizeAutonomousOptimizerSymbolPriorDimension(selectionBucket)
	trend = normalizeAutonomousOptimizerSymbolPriorDimension(trend)
	volatility = normalizeAutonomousOptimizerSymbolPriorDimension(volatility)
	oi = normalizeAutonomousOptimizerSymbolPriorDimension(oi)
	if symbol == "" || side == "" {
		return ""
	}
	return strings.Join([]string{symbol, side, selectionBucket, trend, volatility, oi}, "|")
}

func buildAutonomousOptimizerSymbolPriorRegimeKey(symbol, side, trend, volatility, oi string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	side = strings.ToUpper(strings.TrimSpace(side))
	trend = normalizeAutonomousOptimizerSymbolPriorDimension(trend)
	volatility = normalizeAutonomousOptimizerSymbolPriorDimension(volatility)
	oi = normalizeAutonomousOptimizerSymbolPriorDimension(oi)
	if symbol == "" || side == "" {
		return ""
	}
	return strings.Join([]string{symbol, side, trend, volatility, oi}, "|")
}

func normalizeAutonomousOptimizerSymbolPriorDimension(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "any"
	}
	return trimmed
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxFloat(values ...float64) float64 {
	best := 0.0
	for _, value := range values {
		if value > best {
			best = value
		}
	}
	return best
}

func blankAutonomousOptimizerText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return "n/a"
}

func (s *Server) loadAutonomousOptimizerLearnedPatternPayload(cfg *store.AutonomousOptimizerConfig, bundle *autonomousOptimizerWindowBundle) (*autonomousOptimizerLearnedPatternPayload, error) {
	if s == nil || s.store == nil || cfg == nil {
		return &autonomousOptimizerLearnedPatternPayload{}, nil
	}
	if _, err := s.store.DealReview().RefreshLearnedPatternsIfStale(cfg.UserID, cfg.TraderID); err != nil {
		return nil, err
	}
	patterns, err := s.store.DealReview().ListLearnedPatterns(
		cfg.UserID,
		cfg.TraderID,
		store.DealReviewLearnedPatternFilter{Limit: 100},
	)
	if err != nil {
		return nil, err
	}
	return buildAutonomousOptimizerLearnedPatternPayload(patterns, bundle), nil
}

func buildAutonomousOptimizerLearnedPatternPayload(patterns []store.DealReviewLearnedPattern, bundle *autonomousOptimizerWindowBundle) *autonomousOptimizerLearnedPatternPayload {
	payload := &autonomousOptimizerLearnedPatternPayload{}
	if len(patterns) == 0 {
		return payload
	}

	reportingSummary := store.BuildDealReviewLearnedPatternReportingSummary(patterns)
	if reportingSummary != nil {
		payload.PositiveCount = reportingSummary.PositiveCount
		payload.NegativeCount = reportingSummary.NegativeCount
		payload.ConfirmedCount = reportingSummary.ConfirmedCount
		payload.CandidateCount = reportingSummary.CandidateCount
		payload.FalsePositiveCount = reportingSummary.FalsePositiveCount
		payload.ReverseRiskCount = reportingSummary.ReverseRiskCount
		payload.DriftingCount = reportingSummary.DriftingCount
		payload.ExpiredCount = reportingSummary.ExpiredCount
		payload.Notes = append(payload.Notes, reportingSummary.Notes...)
	}

	closedCaseMatchCounts := map[string]int{}
	closedCaseNetPnL := map[string]float64{}
	recentExecutionMatchCounts := map[string]int{}
	opportunitySymbolMatchCounts := map[string]int{}

	if bundle != nil {
		for _, detail := range bundle.Cases {
			if !strings.EqualFold(detail.Case.Status, store.DealReviewCaseStatusClosed) {
				continue
			}
			caseRec := detail.Case
			for idx := range patterns {
				matched, _ := store.MatchDealReviewLearnedPattern(&caseRec, &patterns[idx])
				if !matched {
					continue
				}
				closedCaseMatchCounts[patterns[idx].ID]++
				closedCaseNetPnL[patterns[idx].ID] += caseRec.RealizedPnL
			}
		}
		if bundle.BucketReview != nil {
			for _, execution := range bundle.BucketReview.RecentOpenExecutions {
				caseRec := buildAutonomousOptimizerLearnedPatternExecutionCase(execution)
				for idx := range patterns {
					matched, _ := store.MatchDealReviewLearnedPattern(&caseRec, &patterns[idx])
					if matched {
						recentExecutionMatchCounts[patterns[idx].ID]++
					}
				}
			}
			for _, symbolSummary := range bundle.BucketReview.OpportunitySymbols {
				symbol := strings.ToUpper(strings.TrimSpace(symbolSummary.Symbol))
				if symbol == "" {
					continue
				}
				for idx := range patterns {
					if patterns[idx].ScopeType != store.DealReviewLearnedPatternScopeSymbol {
						continue
					}
					if strings.ToUpper(strings.TrimSpace(patterns[idx].Symbol)) != symbol {
						continue
					}
					opportunitySymbolMatchCounts[patterns[idx].ID] += maxInt(symbolSummary.CandidateCount, symbolSummary.OpenDecisionCount)
				}
			}
		}
	}

	lookup := make(map[string]autonomousOptimizerLearnedPatternEvidence, len(patterns))
	strongCandidates := make([]autonomousOptimizerLearnedPatternEvidence, 0, len(patterns))
	for _, pattern := range patterns {
		payload.AvailableCount++
		switch strings.TrimSpace(pattern.RecommendedUse) {
		case store.DealReviewLearnedPatternRecommendedUseConfigCand:
			payload.ConfigCandidateCount++
		case store.DealReviewLearnedPatternRecommendedUsePromptHint:
			payload.PromptOnlyCount++
		case store.DealReviewLearnedPatternRecommendedUseReviewHint:
			payload.ReviewHintCount++
		case store.DealReviewLearnedPatternRecommendedUseMonitorOnly:
			payload.MonitorOnlyCount++
		case store.DealReviewLearnedPatternRecommendedUseExpiredIgnore:
			payload.DoNotUseCount++
		}

		closedMatches := closedCaseMatchCounts[pattern.ID]
		recentMatches := recentExecutionMatchCounts[pattern.ID]
		opportunityMatches := opportunitySymbolMatchCounts[pattern.ID]
		matchType := "global_background"
		switch {
		case closedMatches > 0:
			matchType = "closed_case"
		case recentMatches > 0:
			matchType = "recent_open_execution"
		case opportunityMatches > 0:
			matchType = "opportunity_symbol"
		}
		implicationType, implicationSummary := classifyAutonomousOptimizerLearnedPatternImplication(
			&pattern,
			matchType,
			closedMatches,
			recentMatches,
		)

		item := buildAutonomousOptimizerLearnedPatternEvidence(
			pattern,
			matchType,
			closedMatches,
			recentMatches,
			opportunityMatches,
			closedCaseNetPnL[pattern.ID],
			implicationType,
			implicationSummary,
		)
		lookup[item.PatternID] = item

		isRelevant := closedMatches > 0 || recentMatches > 0 || opportunityMatches > 0
		isHealthAlert := isAutonomousOptimizerLearnedPatternHealthAlert(&pattern)
		isStrongBackground := !isRelevant &&
			(pattern.CompositeScore >= 0.72 ||
				(pattern.ValidationLabel == store.DealReviewLearnedPatternValidationLabelConfirmed &&
					pattern.ConfidenceScore >= 0.60 &&
					pattern.SampleCount >= 4))
		if isRelevant || isHealthAlert || isStrongBackground {
			strongCandidates = append(strongCandidates, item)
		}
	}

	sort.Slice(strongCandidates, func(i, j int) bool {
		left := strongCandidates[i]
		right := strongCandidates[j]
		leftHealthRank := autonomousOptimizerLearnedPatternHealthRank(left.ValidationLabel)
		rightHealthRank := autonomousOptimizerLearnedPatternHealthRank(right.ValidationLabel)
		if leftHealthRank != rightHealthRank {
			return leftHealthRank < rightHealthRank
		}
		leftMatchScore := left.ClosedCaseMatchCount*10 + left.RecentExecutionMatchCount*5 + minInt(left.OpportunitySymbolMatchCount, 1)
		rightMatchScore := right.ClosedCaseMatchCount*10 + right.RecentExecutionMatchCount*5 + minInt(right.OpportunitySymbolMatchCount, 1)
		if leftMatchScore != rightMatchScore {
			return leftMatchScore > rightMatchScore
		}
		if left.ImplicationType != right.ImplicationType {
			return autonomousOptimizerLearnedPatternImplicationRank(left.ImplicationType) < autonomousOptimizerLearnedPatternImplicationRank(right.ImplicationType)
		}
		if left.CompositeScore != right.CompositeScore {
			return left.CompositeScore > right.CompositeScore
		}
		if left.ConfidenceScore != right.ConfidenceScore {
			return left.ConfidenceScore > right.ConfidenceScore
		}
		return left.SampleCount > right.SampleCount
	})
	if len(strongCandidates) > 8 {
		strongCandidates = strongCandidates[:8]
	}
	payload.Items = strongCandidates
	payload.RelevantCount = len(strongCandidates)

	if reportingSummary != nil {
		payload.TopPositivePatterns = buildAutonomousOptimizerLearnedPatternEvidenceList(reportingSummary.TopPositivePatterns, lookup, 4)
		payload.TopNegativePatterns = buildAutonomousOptimizerLearnedPatternEvidenceList(reportingSummary.TopNegativePatterns, lookup, 4)
		payload.TopSymbolOverrides = buildAutonomousOptimizerLearnedPatternEvidenceList(reportingSummary.TopSymbolOverrides, lookup, 4)
	}
	payload.Notes = buildAutonomousOptimizerLearnedPatternNotes(payload)
	return payload
}

func buildAutonomousOptimizerLearnedPatternExecutionCase(item store.TraderOpenExecution) store.DealReviewCase {
	return store.DealReviewCase{
		Symbol:               item.Symbol,
		Side:                 item.Side,
		OpenConfidence:       item.Confidence,
		OpenTrendRegime:      item.TrendRegime,
		OpenVolatilityRegime: item.VolatilityRegime,
		OpenFundingRegime:    item.FundingRegime,
		OpenOIRegime:         item.OIRegime,
		OpenSessionBucket:    item.SessionBucket,
		OpenLiquidityTier:    item.LiquidityTier,
		OpenSpreadBucket:     item.SpreadBucket,
		OpenSlippageBucket:   item.SlippageBucket,
	}
}

func buildAutonomousOptimizerLearnedPatternEvidence(pattern store.DealReviewLearnedPattern, matchType string, closedMatches, recentMatches, opportunityMatches int, currentWindowNetPnL float64, implicationType, implicationSummary string) autonomousOptimizerLearnedPatternEvidence {
	return autonomousOptimizerLearnedPatternEvidence{
		PatternID:                   pattern.ID,
		ScopeType:                   pattern.ScopeType,
		Symbol:                      pattern.Symbol,
		Side:                        pattern.Side,
		PatternClass:                pattern.PatternClass,
		Status:                      pattern.Status,
		ValidationLabel:             pattern.ValidationLabel,
		RecommendedUse:              pattern.RecommendedUse,
		PatternSignature:            pattern.PatternSignature,
		RegimeSignature:             pattern.RegimeSignature,
		PatternOrder:                pattern.PatternOrder,
		FeatureCount:                pattern.FeatureCount,
		FeatureSet:                  limitAutonomousOptimizerStringSlice(pattern.FeatureSet, 8),
		SampleCount:                 pattern.SampleCount,
		SupportCount:                pattern.SupportCount,
		ContradictCount:             pattern.ContradictCount,
		WinRate:                     pattern.WinRate,
		LossRate:                    pattern.LossRate,
		AvgPnLPct:                   pattern.AvgPnLPct,
		LiftAvgPnLPct:               pattern.LiftAvgPnLPct,
		Expectancy:                  pattern.Expectancy,
		AvgMFEPct:                   pattern.AvgMFEPct,
		AvgMAEPct:                   pattern.AvgMAEPct,
		GiveBackRate:                pattern.GiveBackRate,
		AvgGiveBackPct:              pattern.AvgGiveBackPct,
		ConfidenceScore:             pattern.ConfidenceScore,
		StabilityScore:              pattern.StabilityScore,
		DriftScore:                  pattern.DriftScore,
		CompositeScore:              pattern.CompositeScore,
		FalsePositiveScore:          pattern.FalsePositiveScore,
		ReverseRiskScore:            pattern.ReverseRiskScore,
		TrainingSampleCount:         pattern.TrainingSampleCount,
		ValidationSampleCount:       pattern.ValidationSampleCount,
		ValidationSupportCount:      pattern.ValidationSupportCount,
		ValidationSupportScore:      pattern.ValidationSupportScore,
		RecentSampleCount:           pattern.RecentSampleCount,
		RecentSupportCount:          pattern.RecentSupportCount,
		RecentSupportScore:          pattern.RecentSupportScore,
		CurrentWindowMatchType:      matchType,
		ClosedCaseMatchCount:        closedMatches,
		RecentExecutionMatchCount:   recentMatches,
		OpportunitySymbolMatchCount: opportunityMatches,
		CurrentWindowNetPnL:         currentWindowNetPnL,
		ImplicationType:             implicationType,
		ImplicationSummary:          implicationSummary,
		EvidenceCaseIDs:             buildAutonomousOptimizerLearnedPatternEvidenceCaseIDs(pattern, 4),
		Summary:                     clipDealReviewAIScanText(pattern.Summary, 320),
		ValidationAlert:             clipDealReviewAIScanText(pattern.ValidationAlert, 240),
	}
}

func buildAutonomousOptimizerLearnedPatternEvidenceList(items []store.DealReviewLearnedPattern, lookup map[string]autonomousOptimizerLearnedPatternEvidence, limit int) []autonomousOptimizerLearnedPatternEvidence {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	out := make([]autonomousOptimizerLearnedPatternEvidence, 0, minInt(len(items), limit))
	for _, item := range items {
		if built, ok := lookup[item.ID]; ok {
			out = append(out, built)
		} else {
			out = append(out, buildAutonomousOptimizerLearnedPatternEvidence(
				item,
				"global_background",
				0,
				0,
				0,
				0,
				autonomousOptimizerLearnedPatternDefaultImplication(&item),
				clipDealReviewAIScanText(item.Summary, 240),
			))
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}

func buildAutonomousOptimizerLearnedPatternEvidenceCaseIDs(pattern store.DealReviewLearnedPattern, limit int) []string {
	if len(pattern.Evidence) == 0 || limit <= 0 {
		return nil
	}
	out := make([]string, 0, minInt(len(pattern.Evidence), limit))
	for _, item := range pattern.Evidence {
		caseID := strings.TrimSpace(item.CaseID)
		if caseID == "" {
			continue
		}
		out = append(out, caseID)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func buildAutonomousOptimizerLearnedPatternNotes(payload *autonomousOptimizerLearnedPatternPayload) []string {
	if payload == nil {
		return nil
	}
	notes := make([]string, 0, 6)
	if payload.ConfigCandidateCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) are strong enough for narrow config-level consideration.", payload.ConfigCandidateCount))
	}
	if payload.PromptOnlyCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) should bias prompt behavior before hard config changes.", payload.PromptOnlyCount))
	}
	if payload.MonitorOnlyCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) are better treated as monitoring or caution signals for now.", payload.MonitorOnlyCount))
	}
	if payload.FalsePositiveCount > 0 || payload.ReverseRiskCount > 0 {
		notes = append(notes, fmt.Sprintf("%d anti-pattern(s) currently act as false-positive or reverse-risk evidence.", payload.FalsePositiveCount+payload.ReverseRiskCount))
	}
	if payload.DriftingCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) are drifting and should be handled conservatively.", payload.DriftingCount))
	}
	if payload.RelevantCount > 0 {
		notes = append(notes, fmt.Sprintf("%d learned pattern(s) directly matched the current review window or recent executions.", payload.RelevantCount))
	}
	return dedupeSortedStrings(notes)
}

func buildAutonomousOptimizerLearnedPatternBacklogProposals(payload *autonomousOptimizerLearnedPatternPayload) []autonomousOptimizerBacklogProposal {
	if payload == nil || len(payload.Items) == 0 {
		return nil
	}
	proposals := make([]autonomousOptimizerBacklogProposal, 0, 4)
	seen := map[string]struct{}{}
	antiEvidenceCount := 0
	driftCount := 0
	coverageCount := 0

	appendProposal := func(item autonomousOptimizerBacklogProposal) {
		item = sanitizeAutonomousOptimizerBacklogProposal(item)
		if item.Title == "" {
			return
		}
		key := autonomousOptimizerBacklogMergeKey(item.Category, item.Title)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		proposals = append(proposals, item)
	}

	for _, item := range payload.Items {
		if antiEvidenceCount < 2 {
			if proposal, ok := buildAutonomousOptimizerLearnedPatternGuardBacklogProposal(item); ok {
				appendProposal(proposal)
				antiEvidenceCount++
			}
		}
		if driftCount < 1 {
			if proposal, ok := buildAutonomousOptimizerLearnedPatternDriftBacklogProposal(item); ok {
				appendProposal(proposal)
				driftCount++
			}
		}
		if coverageCount < 1 {
			if proposal, ok := buildAutonomousOptimizerLearnedPatternCoverageBacklogProposal(item); ok {
				appendProposal(proposal)
				coverageCount++
			}
		}
		if len(proposals) >= 4 {
			break
		}
	}
	return proposals
}

func buildAutonomousOptimizerLearnedPatternGuardBacklogProposal(item autonomousOptimizerLearnedPatternEvidence) (autonomousOptimizerBacklogProposal, bool) {
	if item.ImplicationType != "anti_evidence" {
		return autonomousOptimizerBacklogProposal{}, false
	}
	directRelevance := item.ClosedCaseMatchCount > 0 || item.RecentExecutionMatchCount > 0
	if !directRelevance && item.OpportunitySymbolMatchCount <= 0 {
		return autonomousOptimizerBacklogProposal{}, false
	}

	category := "missing_prompt_instruction"
	if item.ValidationLabel == store.DealReviewLearnedPatternValidationLabelReverseRisk ||
		item.RecentExecutionMatchCount > 0 {
		category = "missing_risk_control"
	}
	scopeLabel := buildAutonomousOptimizerLearnedPatternScopeLabel(item)
	title := fmt.Sprintf("Encode %s learned anti-pattern guard", scopeLabel)
	description := fmt.Sprintf(
		"%s matched the latest review window as %s (%d closed-case, %d recent execution, %d opportunity matches). The optimizer should have a reusable guard against repeating this setup. Pattern summary: %s",
		scopeLabel,
		item.CurrentWindowMatchType,
		item.ClosedCaseMatchCount,
		item.RecentExecutionMatchCount,
		item.OpportunitySymbolMatchCount,
		blankAutonomousOptimizerText(item.Summary, item.ImplicationSummary),
	)
	expectedImpact := "Reduce repeated exposure to a learned anti-edge by encoding a reusable prompt or risk-control guard."
	confidence := clampAutonomousOptimizerScore(maxFloat(
		item.FalsePositiveScore,
		item.ReverseRiskScore,
		item.CompositeScore,
	))
	urgency := clampAutonomousOptimizerScore(maxFloat(
		confidence,
		autonomousOptimizerLearnedPatternWindowUrgency(item),
	))
	implementationCost := 0.34
	if category == "missing_risk_control" {
		implementationCost = 0.48
	}
	return autonomousOptimizerBacklogProposal{
		Title:              title,
		Category:           category,
		Description:        clipDealReviewAIScanText(description, 900),
		ExpectedImpact:     expectedImpact,
		Confidence:         confidence,
		ImplementationCost: implementationCost,
		Urgency:            urgency,
		RecurrenceCount:    maxInt(1, item.ClosedCaseMatchCount+item.RecentExecutionMatchCount),
		Evidence:           []map[string]any{buildAutonomousOptimizerLearnedPatternEvidenceMap(item)},
		Metadata: map[string]any{
			"source_kind":         "learned_pattern_synthesis",
			"derived_goal":        "encode_guard",
			"pattern_id":          item.PatternID,
			"validation_label":    item.ValidationLabel,
			"implication_type":    item.ImplicationType,
			"current_match_type":  item.CurrentWindowMatchType,
			"feature_set":         item.FeatureSet,
			"evidence_case_ids":   item.EvidenceCaseIDs,
			"scope_type":          item.ScopeType,
			"symbol":              item.Symbol,
			"side":                item.Side,
			"pattern_signature":   item.PatternSignature,
			"regime_signature":    item.RegimeSignature,
			"current_window_pnl":  item.CurrentWindowNetPnL,
			"closed_case_matches": item.ClosedCaseMatchCount,
		},
	}, true
}

func buildAutonomousOptimizerLearnedPatternDriftBacklogProposal(item autonomousOptimizerLearnedPatternEvidence) (autonomousOptimizerBacklogProposal, bool) {
	if item.ValidationLabel != store.DealReviewLearnedPatternValidationLabelDrifting {
		return autonomousOptimizerBacklogProposal{}, false
	}
	if item.CurrentWindowMatchType == "global_background" && item.ClosedCaseMatchCount == 0 && item.RecentExecutionMatchCount == 0 {
		return autonomousOptimizerBacklogProposal{}, false
	}
	scopeLabel := buildAutonomousOptimizerLearnedPatternScopeLabel(item)
	description := fmt.Sprintf(
		"%s is marked as drifting and still touched the latest window (%s). This should create stronger review telemetry so stale patterns expire or de-rank automatically before they bias future optimizer runs. Summary: %s",
		scopeLabel,
		item.CurrentWindowMatchType,
		blankAutonomousOptimizerText(item.ValidationAlert, item.Summary),
	)
	return autonomousOptimizerBacklogProposal{
		Title:              fmt.Sprintf("Add drift expiry monitor for %s learned pattern", scopeLabel),
		Category:           "missing_review_metric",
		Description:        clipDealReviewAIScanText(description, 900),
		ExpectedImpact:     "Reduce stale-pattern reuse by surfacing drift and expiry pressure earlier in optimizer evidence.",
		Confidence:         clampAutonomousOptimizerScore(maxFloat(item.DriftScore, item.CompositeScore)),
		ImplementationCost: 0.28,
		Urgency:            clampAutonomousOptimizerScore(maxFloat(item.DriftScore, autonomousOptimizerLearnedPatternWindowUrgency(item))),
		RecurrenceCount:    maxInt(1, item.ClosedCaseMatchCount+item.RecentExecutionMatchCount),
		Evidence:           []map[string]any{buildAutonomousOptimizerLearnedPatternEvidenceMap(item)},
		Metadata: map[string]any{
			"source_kind":        "learned_pattern_synthesis",
			"derived_goal":       "drift_monitoring",
			"pattern_id":         item.PatternID,
			"validation_label":   item.ValidationLabel,
			"current_match_type": item.CurrentWindowMatchType,
			"scope_type":         item.ScopeType,
			"symbol":             item.Symbol,
			"side":               item.Side,
			"feature_set":        item.FeatureSet,
		},
	}, true
}

func buildAutonomousOptimizerLearnedPatternCoverageBacklogProposal(item autonomousOptimizerLearnedPatternEvidence) (autonomousOptimizerBacklogProposal, bool) {
	if item.PatternClass != store.DealReviewLearnedPatternClassPositiveEdge {
		return autonomousOptimizerBacklogProposal{}, false
	}
	if item.ValidationLabel != store.DealReviewLearnedPatternValidationLabelCandidate &&
		item.ValidationLabel != store.DealReviewLearnedPatternValidationLabelInsufficientEvidence {
		return autonomousOptimizerBacklogProposal{}, false
	}
	if item.SampleCount < 4 || item.CompositeScore < 0.55 {
		return autonomousOptimizerBacklogProposal{}, false
	}
	if item.ValidationSampleCount >= 2 && item.RecentSampleCount >= 2 {
		return autonomousOptimizerBacklogProposal{}, false
	}

	scopeLabel := buildAutonomousOptimizerLearnedPatternScopeLabel(item)
	description := fmt.Sprintf(
		"%s looks directionally useful but still lacks enough holdout/recent coverage (validation=%d, recent=%d). Before this turns into a config candidate, the review stack should widen validation coverage or attach better discriminators. Summary: %s",
		scopeLabel,
		item.ValidationSampleCount,
		item.RecentSampleCount,
		blankAutonomousOptimizerText(item.Summary, item.ImplicationSummary),
	)
	return autonomousOptimizerBacklogProposal{
		Title:              fmt.Sprintf("Expand validation coverage for %s learned pattern", scopeLabel),
		Category:           "missing_review_metric",
		Description:        clipDealReviewAIScanText(description, 900),
		ExpectedImpact:     "Separate durable learned edges from small-sample noise before the optimizer promotes them into live patches.",
		Confidence:         clampAutonomousOptimizerScore(maxFloat(item.CompositeScore, item.ConfidenceScore)),
		ImplementationCost: 0.24,
		Urgency:            clampAutonomousOptimizerScore(maxFloat(item.CompositeScore*0.85, autonomousOptimizerLearnedPatternWindowUrgency(item))),
		RecurrenceCount:    maxInt(1, item.SupportCount),
		Evidence:           []map[string]any{buildAutonomousOptimizerLearnedPatternEvidenceMap(item)},
		Metadata: map[string]any{
			"source_kind":             "learned_pattern_synthesis",
			"derived_goal":            "validation_coverage",
			"pattern_id":              item.PatternID,
			"validation_label":        item.ValidationLabel,
			"scope_type":              item.ScopeType,
			"symbol":                  item.Symbol,
			"side":                    item.Side,
			"feature_set":             item.FeatureSet,
			"validation_sample_count": item.ValidationSampleCount,
			"recent_sample_count":     item.RecentSampleCount,
			"sample_count":            item.SampleCount,
		},
	}, true
}

func buildAutonomousOptimizerLearnedPatternEvidenceMap(item autonomousOptimizerLearnedPatternEvidence) map[string]any {
	return map[string]any{
		"source":                         "learned_pattern",
		"pattern_id":                     item.PatternID,
		"scope_type":                     item.ScopeType,
		"symbol":                         item.Symbol,
		"side":                           item.Side,
		"pattern_class":                  item.PatternClass,
		"validation_label":               item.ValidationLabel,
		"recommended_use":                item.RecommendedUse,
		"implication_type":               item.ImplicationType,
		"current_window_match_type":      item.CurrentWindowMatchType,
		"closed_case_match_count":        item.ClosedCaseMatchCount,
		"recent_execution_match_count":   item.RecentExecutionMatchCount,
		"opportunity_symbol_match_count": item.OpportunitySymbolMatchCount,
		"current_window_net_pnl":         item.CurrentWindowNetPnL,
		"feature_set":                    item.FeatureSet,
		"evidence_case_ids":              item.EvidenceCaseIDs,
		"summary":                        item.Summary,
		"validation_alert":               item.ValidationAlert,
	}
}

func buildAutonomousOptimizerLearnedPatternScopeLabel(item autonomousOptimizerLearnedPatternEvidence) string {
	symbol := strings.ToUpper(strings.TrimSpace(item.Symbol))
	side := strings.ToUpper(strings.TrimSpace(item.Side))
	if symbol != "" {
		return strings.TrimSpace(symbol + " " + side)
	}
	if side != "" {
		return strings.TrimSpace(strings.ToLower(strings.ReplaceAll(item.ScopeType, "_", " ")) + " " + side)
	}
	return strings.ToLower(strings.ReplaceAll(item.ScopeType, "_", " "))
}

func autonomousOptimizerLearnedPatternWindowUrgency(item autonomousOptimizerLearnedPatternEvidence) float64 {
	matchWeight := float64(item.ClosedCaseMatchCount)*0.18 + float64(item.RecentExecutionMatchCount)*0.14
	if item.OpportunitySymbolMatchCount > 0 {
		matchWeight += 0.08
	}
	if item.CurrentWindowNetPnL < 0 {
		matchWeight += 0.10
	}
	return clampAutonomousOptimizerScore(matchWeight)
}

func isAutonomousOptimizerLearnedPatternHealthAlert(pattern *store.DealReviewLearnedPattern) bool {
	if pattern == nil {
		return false
	}
	switch strings.TrimSpace(pattern.ValidationLabel) {
	case store.DealReviewLearnedPatternValidationLabelFalsePositive,
		store.DealReviewLearnedPatternValidationLabelReverseRisk,
		store.DealReviewLearnedPatternValidationLabelDrifting:
		return true
	default:
		return false
	}
}

func autonomousOptimizerLearnedPatternHealthRank(value string) int {
	switch strings.TrimSpace(value) {
	case store.DealReviewLearnedPatternValidationLabelReverseRisk:
		return 0
	case store.DealReviewLearnedPatternValidationLabelFalsePositive:
		return 1
	case store.DealReviewLearnedPatternValidationLabelDrifting:
		return 2
	case store.DealReviewLearnedPatternValidationLabelConfirmed:
		return 3
	case store.DealReviewLearnedPatternValidationLabelCandidate:
		return 4
	case store.DealReviewLearnedPatternValidationLabelInsufficientEvidence:
		return 5
	case store.DealReviewLearnedPatternValidationLabelExpired:
		return 6
	default:
		return 7
	}
}

func autonomousOptimizerLearnedPatternImplicationRank(value string) int {
	switch strings.TrimSpace(value) {
	case "anti_evidence":
		return 0
	case "config_candidate":
		return 1
	case "prompt_only":
		return 2
	case "review_hint":
		return 3
	case "monitor_only":
		return 4
	case "do_not_use":
		return 5
	default:
		return 9
	}
}

func autonomousOptimizerLearnedPatternDefaultImplication(pattern *store.DealReviewLearnedPattern) string {
	if pattern == nil {
		return "review_hint"
	}
	switch strings.TrimSpace(pattern.RecommendedUse) {
	case store.DealReviewLearnedPatternRecommendedUseConfigCand:
		return "config_candidate"
	case store.DealReviewLearnedPatternRecommendedUsePromptHint:
		if pattern.PatternClass == store.DealReviewLearnedPatternClassNegativeEdge {
			return "anti_evidence"
		}
		return "prompt_only"
	case store.DealReviewLearnedPatternRecommendedUseMonitorOnly:
		return "monitor_only"
	case store.DealReviewLearnedPatternRecommendedUseExpiredIgnore:
		return "do_not_use"
	default:
		if pattern.PatternClass == store.DealReviewLearnedPatternClassNegativeEdge {
			return "anti_evidence"
		}
		return "review_hint"
	}
}

func classifyAutonomousOptimizerLearnedPatternImplication(pattern *store.DealReviewLearnedPattern, matchType string, closedMatches, recentMatches int) (string, string) {
	if pattern == nil {
		return "review_hint", "Learned pattern is unavailable."
	}
	switch strings.TrimSpace(pattern.ValidationLabel) {
	case store.DealReviewLearnedPatternValidationLabelFalsePositive:
		return "anti_evidence", "This learned pattern now behaves like a false positive. Use it as anti-evidence and avoid repeating the old assumption."
	case store.DealReviewLearnedPatternValidationLabelReverseRisk:
		return "anti_evidence", "This learned pattern now leans in the opposite direction. Prefer caution, prompt narrowing, or monitoring over broader exposure."
	case store.DealReviewLearnedPatternValidationLabelDrifting:
		return "monitor_only", "This learned pattern is drifting. Keep it visible, but avoid hard live mutation unless newer evidence firms it up."
	case store.DealReviewLearnedPatternValidationLabelExpired:
		return "do_not_use", "This learned pattern has expired and should not justify a live patch."
	}

	directWindowEvidence := closedMatches > 0 || recentMatches > 0
	switch strings.TrimSpace(pattern.RecommendedUse) {
	case store.DealReviewLearnedPatternRecommendedUseConfigCand:
		if directWindowEvidence {
			return "config_candidate", "Confirmed learned edge with direct window relevance; a narrow config patch can be justified."
		}
		return "review_hint", "Strong learned edge exists, but the current window did not hit it directly. Treat it as bounded review context first."
	case store.DealReviewLearnedPatternRecommendedUsePromptHint:
		if pattern.PatternClass == store.DealReviewLearnedPatternClassNegativeEdge {
			return "anti_evidence", "Confirmed negative learned pattern should tighten prompt behavior or block repeating the same setup."
		}
		return "prompt_only", "Pattern is best expressed as prompt guidance before any durable config change."
	case store.DealReviewLearnedPatternRecommendedUseMonitorOnly:
		return "monitor_only", "Pattern should stay visible as a monitoring signal, not as direct live patch support."
	case store.DealReviewLearnedPatternRecommendedUseExpiredIgnore:
		return "do_not_use", "Pattern should be ignored for live optimizer decisions."
	default:
		if pattern.PatternClass == store.DealReviewLearnedPatternClassNegativeEdge {
			return "anti_evidence", "Negative learned pattern should count against repeating the same setup."
		}
		if matchType == "global_background" {
			return "review_hint", "Positive learned pattern exists, but current-window relevance is indirect."
		}
		return "prompt_only", "Pattern supports tighter prompt guidance for the current setup."
	}
}

func buildAutonomousOptimizerReviewPayload(cfg *store.AutonomousOptimizerConfig, traderCfg *store.Trader, strategyRecord *store.Strategy, strategyCfg *store.StrategyConfig, bundle *autonomousOptimizerWindowBundle, recentRuns []*store.AutonomousOptimizerRun, symbolBehaviorPriors *autonomousOptimizerSymbolPriorPayload, learnedPatterns *autonomousOptimizerLearnedPatternPayload) (map[string]any, error) {
	basePayload, err := buildDealReviewAnalysisPayload(traderCfg, strategyRecord, strategyCfg, bundle.Filter, bundle.Cases, bundle.Summary)
	if err != nil {
		return nil, err
	}

	basePayload["autonomous_optimizer"] = map[string]any{
		"config_id":                cfg.ID,
		"review_interval_hours":    cfg.ReviewIntervalHours,
		"auto_apply_config_patch":  cfg.AutoApplyConfigPatch,
		"auto_apply_prompt_patch":  cfg.AutoApplyPromptPatch,
		"auto_rollback_enabled":    cfg.AutoRollbackEnabled,
		"self_pause_enabled":       cfg.SelfPauseEnabled,
		"current_status":           cfg.Status,
		"primary_model_name":       cfg.PrimaryModelName,
		"critic_model_name":        cfg.CriticModelName,
		"seed_source_trader_id":    cfg.SeedSourceTraderID,
		"seed_source_strategy_id":  cfg.SeedSourceStrategyID,
		"baseline_strategy_id":     cfg.BaselineStrategyID,
		"current_seed_strategy_id": cfg.CurrentSeedStrategyID,
	}
	basePayload["review_window"] = map[string]any{
		"start":           bundle.WindowStart.UTC().Format(time.RFC3339),
		"end":             bundle.WindowEnd.UTC().Format(time.RFC3339),
		"hours":           cfg.ReviewIntervalHours,
		"closed_deals":    bundle.Metadata.ClosedDeals,
		"open_deals":      bundle.Metadata.OpenDeals,
		"net_pnl":         bundle.Metadata.NetPnL,
		"win_rate":        bundle.Metadata.WinRate,
		"record_count":    bundle.Metadata.DecisionRecordCount,
		"candidate_count": bundle.Metadata.DecisionCandidateCount,
	}
	basePayload["decision_bucket_review"] = bundle.BucketReview
	basePayload["decision_starvation_metrics"] = buildAutonomousOptimizerStarvationMetrics(bundle.BucketReview)
	basePayload["trailing_stop_telemetry"] = bundle.Metadata.TrailingStopTelemetry
	basePayload["adaptive_cooldown_telemetry"] = bundle.Metadata.AdaptiveCooldownTelemetry
	basePayload["current_prompt_bundle"] = buildAutonomousOptimizerPromptBundle(cfg, traderCfg, strategyCfg)
	basePayload["recent_optimizer_runs"] = summarizeAutonomousOptimizerRuns(recentRuns)
	basePayload["latest_gate_feedback"] = buildAutonomousOptimizerLatestGateFeedback(recentRuns)
	if symbolBehaviorPriors != nil {
		basePayload["symbol_behavior_priors"] = symbolBehaviorPriors
	}
	if learnedPatterns != nil {
		basePayload["learned_patterns"] = learnedPatterns
	}
	return basePayload, nil
}

func buildAutonomousOptimizerProposalPrompt(cfg *store.AutonomousOptimizerConfig, payload map[string]any) (string, string, error) {
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", "", err
	}
	systemPrompt := `You are the autonomous optimizer for an automated crypto trader.
Return JSON only with this exact shape:
{
  "executive_summary": "string",
  "proposal_type": "no_change|config_patch|prompt_patch|config_and_prompt_patch|backlog_only|pause_optimizer",
  "rationale": ["string"],
  "expected_effect": "string",
  "evidence_strength": 0.0,
  "confidence": 0.0,
  "symbol_prior_references": ["prior_key"],
  "learned_pattern_references": ["pattern_id"],
  "config_patch": {},
  "prompt_patch": {
    "strategy": {
      "custom_prompt": "string",
      "prompt_sections": {
        "role_definition": "string",
        "trading_frequency": "string",
        "entry_standards": "string",
        "market_context": "string",
        "decision_process": "string",
        "decision_format": "string"
      }
    },
    "trader": {
      "custom_prompt": "string",
      "override_base_prompt": false,
      "system_prompt_template": "string"
    },
    "optimizer": {
      "proposal_instructions": "string",
      "critic_instructions": "string"
    }
  },
  "backlog_items": [
    {
      "title": "string",
      "category": "missing_indicator|missing_market_data|missing_execution_telemetry|missing_regime_metadata|missing_risk_control|missing_prompt_instruction|missing_review_metric|other_capability_gap",
      "description": "string",
      "expected_effect": "string",
      "confidence": 0.0,
      "implementation_cost": 0.0,
      "urgency": 0.0,
      "recurrence_count": 1
    }
  ],
  "pause_reason": "string"
}
Rules:
- Be conservative. Weak evidence should become "no_change" or "backlog_only", not speculative mutation.
- Use "config_patch" only for actual strategy JSON changes. Do not put prompt changes inside config_patch.
- Use "prompt_patch" only for these allowed surfaces: strategy.custom_prompt, strategy.prompt_sections.role_definition, strategy.prompt_sections.trading_frequency, strategy.prompt_sections.entry_standards, strategy.prompt_sections.market_context, strategy.prompt_sections.decision_process, strategy.prompt_sections.decision_format, trader.custom_prompt, trader.override_base_prompt, trader.system_prompt_template, optimizer.proposal_instructions, optimizer.critic_instructions.
- If the system lacks data or features needed for a stronger recommendation, create backlog_items instead of hallucinating unavailable inputs.
- Prefer prompt_patch over config_patch when the evidence is mostly low-trade / no-trade telemetry rather than realized deal outcomes.
- Use decision_starvation_metrics and recent_optimizer_runs explicitly. Explain whether the main problem is bad executed trades, over-filtered inactivity, or repeated failure of the last optimizer changes.
- Use latest_gate_feedback explicitly when it is present. If the last optimizer run was blocked, do not repeat the same patch unchanged. Either propose a materially narrower subset, switch to backlog_only, or explain why the new evidence is now different enough.
- Use symbol_behavior_priors explicitly when present. If you rely on a prior, cite its exact prior_key in symbol_prior_references.
- Use symbol_behavior_priors.cluster_rollups when present to detect normalized setup patterns that recur across multiple priors or symbols. Distinguish one bad symbol from a broader bad cluster.
- Respect symbol_behavior_priors.items[].implication_type:
  - prompt_only: use prompt_patch or backlog_only, not config_patch, unless separate non-prior evidence independently justifies the config change.
  - config_candidate: a narrow config_patch may be justified, especially when current_window_match_type is closed_case or recent_open_execution.
  - missing_data_backlog: prefer backlog_items for missing indicators, market data, regime metadata, or execution telemetry rather than forcing a live patch.
- Use symbol_behavior_priors.items[].validation_label explicitly:
  - false_positive: treat as anti-evidence. Do not use that prior to justify config changes; instead avoid repeating the old assumption, suggest rollback/monitoring, or create backlog if data is missing.
  - false_negative_risk: recent evidence is leaning the other way. Prefer narrow prompt experiments, targeted scans, or symbol-specific monitoring rather than broad flips.
  - drifting: the prior is weakening. Prefer monitoring or a narrow prompt caution instead of a hard config mutation.
  - confirmed: the prior is still supportive evidence, but only within its symbol/regime scope.
- Use learned_patterns explicitly when present. If you rely on a learned pattern, cite its exact pattern_id in learned_pattern_references.
- Respect learned_patterns.items[].implication_type:
  - anti_evidence: use it against repeating the setup, as a prompt constraint, or as a reason to avoid broad config relaxation.
  - config_candidate: a narrow config_patch may be justified, especially when current_window_match_type is closed_case or recent_open_execution.
  - prompt_only or review_hint: bias prompt behavior or narrow the review narrative before hard config changes.
  - monitor_only or do_not_use: do not turn these into live config patches.
- Treat learned_patterns.items[].validation_label=false_positive, reverse_risk, or drifting as caution or anti-evidence, not as support for aggressive config changes.
- Treat learned_patterns.top_symbol_overrides as stronger evidence for symbol-specific exceptions than generic global intuition.
- Treat current_window_match_type=opportunity_symbol or global_background as weaker evidence than direct closed_case matches.
- If a cluster_rollup shows false_positive_count, false_negative_risk_count, or drifting_count concentration, prefer prompt caution, monitoring, or backlog work over broad config mutation unless direct closed-case evidence is stronger.
- When inactivity is the problem, cite concrete reject reasons, confidence bands, sessions, or symbols from the supplied telemetry.
- Use trailing_stop_telemetry when exit management is the issue. Distinguish profitable protective trailing exits from early loss-making stop tightening before proposing stop logic changes, and use sample_updates for first-tighten timing plus pre-update unrealized PnL context.
- Use adaptive_cooldown_telemetry plus current risk_control settings when repeated same-symbol or same-regime re-entries are the issue. Compare blocked re-entry outcomes versus post-cooldown outcomes before tightening or loosening cooldowns, and prefer narrow cooldown controls over blunt reductions in trade frequency.
- Keep patches minimal and high-signal. Do not modify credentials, exchange bindings, or unrelated trader settings.
- If you touch more than 6 prompt fields, only the highest-priority 6 fields will be auto-applied. Concentrate changes into the most causally important prompt surfaces first.
- Never output markdown or code fences.`
	if cfg != nil {
		if extra := strings.TrimSpace(cfg.ProposalPromptInstructions); extra != "" {
			systemPrompt += "\n\n## Existing Proposal Instructions Overlay\n" + extra
		}
	}
	userPrompt := "Review this autonomous optimizer bundle and propose the next action:\n" + string(body)
	return systemPrompt, userPrompt, nil
}

func buildAutonomousOptimizerCriticPrompt(cfg *store.AutonomousOptimizerConfig, payload map[string]any, proposal *autonomousOptimizerProposalResult) (string, string, error) {
	body, err := json.MarshalIndent(map[string]any{
		"optimizer_payload": payload,
		"proposal":          proposal,
	}, "", "  ")
	if err != nil {
		return "", "", err
	}
	systemPrompt := `You are the safety critic for an autonomous trading-strategy optimizer.
Return JSON only with this exact shape:
{
  "approved": true,
  "confidence": 0.0,
  "recommended_action": "approve_apply|approve_backlog_only|block_apply|pause_optimizer",
  "summary": "string",
  "symbol_prior_references": ["prior_key"],
  "learned_pattern_references": ["pattern_id"],
  "blocking_issues": ["string"],
  "warnings": ["string"]
}
Rules:
- Block proposals that overreach the evidence, attempt unsupported edits, or look too broad for one review window.
- If the proposal is useful only as a backlog note, use "approve_backlog_only".
- Prompt patches may be approved on low-trade evidence when they are narrow and explicitly aimed at inactivity or decision quality.
- Config patches need materially stronger evidence than prompt patches.
- Use recent_optimizer_runs and decision_starvation_metrics to spot repeated loops, over-filtering, or attempts to fix the same issue without new evidence.
- Use symbol_behavior_priors explicitly when present. If the proposal relies on them, cite the prior_key values you considered in symbol_prior_references.
- Use symbol_behavior_priors.cluster_rollups when present to detect whether the proposal is overfitting one symbol or whether a normalized setup pattern actually recurs across multiple priors.
- Block or downgrade proposals that turn prompt_only or missing_data_backlog priors into broad config patches without stronger independent evidence.
- Treat config_candidate priors with direct closed_case or recent_open_execution matches as stronger than priors that only match opportunity_symbol or global_background context.
- Treat validation_label=false_positive as anti-evidence against config patches built on that prior.
- Treat validation_label=false_negative_risk as a reason to prefer monitoring, prompt-only experiments, or backlog notes unless there is separate direct evidence for a narrow change.
- Treat validation_label=drifting as weaker than confirmed priors even when the older historical sample looked strong.
- Use learned_patterns explicitly when present. If the proposal relies on them, cite pattern_id values in learned_pattern_references.
- Treat learned_patterns.items[].implication_type=anti_evidence as a blocker against broad config relaxation or repeated exposure to the same setup.
- Treat learned_patterns.items[].implication_type=config_candidate with direct closed_case or recent_open_execution matches as stronger than background-only pattern matches.
- Treat learned_patterns.items[].validation_label=false_positive, reverse_risk, drifting, or expired as caution, anti-evidence, or do-not-use context rather than support for live mutation.
- Never output markdown or code fences.`
	if cfg != nil {
		if extra := strings.TrimSpace(cfg.CriticPromptInstructions); extra != "" {
			systemPrompt += "\n\n## Existing Critic Instructions Overlay\n" + extra
		}
	}
	userPrompt := "Critique this autonomous optimizer proposal:\n" + string(body)
	return systemPrompt, userPrompt, nil
}

func parseAutonomousOptimizerProposalResponse(response string) (*autonomousOptimizerProposalResult, string, error) {
	cleaned := strings.TrimSpace(response)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	result := &autonomousOptimizerProposalResult{}
	if err := json.Unmarshal([]byte(cleaned), result); err != nil {
		start := strings.Index(cleaned, "{")
		end := strings.LastIndex(cleaned, "}")
		if start >= 0 && end > start {
			cleaned = cleaned[start : end+1]
		}
		if err := json.Unmarshal([]byte(cleaned), result); err != nil {
			return nil, "", err
		}
	}
	result.ProposalType = normalizeAutonomousOptimizerProposalType(result.ProposalType)
	result.ExecutiveSummary = strings.TrimSpace(result.ExecutiveSummary)
	result.ExpectedEffect = strings.TrimSpace(result.ExpectedEffect)
	result.PauseReason = strings.TrimSpace(result.PauseReason)
	result.Rationale = filterNonEmptyStrings(result.Rationale)
	result.SymbolPriorRefs = filterNonEmptyStrings(result.SymbolPriorRefs)
	result.LearnedPatternRefs = filterNonEmptyStrings(result.LearnedPatternRefs)
	result.EvidenceStrength = clampAutonomousOptimizerScore(result.EvidenceStrength)
	result.Confidence = clampAutonomousOptimizerScore(result.Confidence)
	result.ConfigPatch = sanitizeAutonomousOptimizerJSONMap(result.ConfigPatch)
	result.PromptPatch = sanitizeAutonomousOptimizerJSONMap(result.PromptPatch)
	for i := range result.BacklogItems {
		result.BacklogItems[i] = sanitizeAutonomousOptimizerBacklogProposal(result.BacklogItems[i])
	}
	raw, _ := json.Marshal(result)
	return result, string(raw), nil
}

func parseAutonomousOptimizerCriticResponse(response string) (*autonomousOptimizerCriticResult, string, error) {
	cleaned := strings.TrimSpace(response)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	result := &autonomousOptimizerCriticResult{}
	if err := json.Unmarshal([]byte(cleaned), result); err != nil {
		start := strings.Index(cleaned, "{")
		end := strings.LastIndex(cleaned, "}")
		if start >= 0 && end > start {
			cleaned = cleaned[start : end+1]
		}
		if err := json.Unmarshal([]byte(cleaned), result); err != nil {
			return nil, "", err
		}
	}
	result.RecommendedAction = normalizeAutonomousOptimizerCriticAction(result.RecommendedAction)
	result.Summary = strings.TrimSpace(result.Summary)
	result.Confidence = clampAutonomousOptimizerScore(result.Confidence)
	result.SymbolPriorRefs = filterNonEmptyStrings(result.SymbolPriorRefs)
	result.LearnedPatternRefs = filterNonEmptyStrings(result.LearnedPatternRefs)
	result.BlockingIssues = filterNonEmptyStrings(result.BlockingIssues)
	result.Warnings = filterNonEmptyStrings(result.Warnings)
	raw, _ := json.Marshal(result)
	return result, string(raw), nil
}

func decodeAutonomousOptimizerPromptPatch(raw map[string]any) (*autonomousOptimizerPromptPatch, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	patch := &autonomousOptimizerPromptPatch{}
	if err := json.Unmarshal(body, patch); err != nil {
		return nil, err
	}
	return patch, nil
}

func validateAutonomousOptimizerPromptPatch(cfg *store.AutonomousOptimizerConfig, traderCfg *store.Trader, strategyCfg *store.StrategyConfig, modelCfg *store.AIModel, modelName string, patch *autonomousOptimizerPromptPatch) (*store.AutonomousOptimizerConfig, *store.Trader, *store.StrategyConfig, *autonomousOptimizerPromptValidation, error) {
	validation := &autonomousOptimizerPromptValidation{
		AppliedFieldLimit: autonomousOptimizerMaxPromptPatchFields,
	}
	if traderCfg == nil || strategyCfg == nil {
		validation.BlockingIssues = append(validation.BlockingIssues, "Trader or strategy config is unavailable for prompt validation.")
		return nil, nil, nil, validation, nil
	}
	if patch == nil {
		validation.BlockingIssues = append(validation.BlockingIssues, "Prompt patch is empty.")
		return nil, nil, nil, validation, nil
	}

	var mergedOptimizer *store.AutonomousOptimizerConfig
	if cfg != nil {
		clone := *cfg
		mergedOptimizer = &clone
	}
	mergedTrader := *traderCfg
	mergedStrategy, err := strategyCfg.Clone()
	if err != nil {
		return nil, nil, nil, validation, err
	}
	if mergedStrategy == nil {
		mergedStrategy = &store.StrategyConfig{}
	}

	candidates := make([]autonomousOptimizerPromptFieldCandidate, 0, 12)
	addTextCandidate := func(label string, target *string, value *string) {
		if value == nil {
			return
		}
		trimmed := strings.TrimSpace(*value)
		if trimmed == "" {
			validation.Warnings = append(validation.Warnings, fmt.Sprintf("Ignored blank autonomous prompt patch for %s.", label))
			return
		}
		if len([]rune(trimmed)) > autonomousOptimizerMaxPromptFieldChars {
			validation.BlockingIssues = append(validation.BlockingIssues, fmt.Sprintf("%s exceeds the %d character limit.", label, autonomousOptimizerMaxPromptFieldChars))
			return
		}
		if target == nil || *target == trimmed {
			return
		}
		candidates = append(candidates, autonomousOptimizerPromptFieldCandidate{
			Label:    label,
			Priority: autonomousOptimizerPromptFieldPriority(label),
			Apply: func() {
				*target = trimmed
			},
		})
	}

	if patch.Strategy != nil {
		addTextCandidate("strategy.custom_prompt", &mergedStrategy.CustomPrompt, patch.Strategy.CustomPrompt)
		if patch.Strategy.PromptSections != nil {
			addTextCandidate("strategy.prompt_sections.role_definition", &mergedStrategy.PromptSections.RoleDefinition, patch.Strategy.PromptSections.RoleDefinition)
			addTextCandidate("strategy.prompt_sections.trading_frequency", &mergedStrategy.PromptSections.TradingFrequency, patch.Strategy.PromptSections.TradingFrequency)
			addTextCandidate("strategy.prompt_sections.entry_standards", &mergedStrategy.PromptSections.EntryStandards, patch.Strategy.PromptSections.EntryStandards)
			addTextCandidate("strategy.prompt_sections.market_context", &mergedStrategy.PromptSections.MarketContext, patch.Strategy.PromptSections.MarketContext)
			addTextCandidate("strategy.prompt_sections.decision_process", &mergedStrategy.PromptSections.DecisionProcess, patch.Strategy.PromptSections.DecisionProcess)
			addTextCandidate("strategy.prompt_sections.decision_format", &mergedStrategy.PromptSections.DecisionFormat, patch.Strategy.PromptSections.DecisionFormat)
		}
	}
	if patch.Trader != nil {
		addTextCandidate("trader.custom_prompt", &mergedTrader.CustomPrompt, patch.Trader.CustomPrompt)
		if patch.Trader.OverrideBasePrompt != nil {
			if *patch.Trader.OverrideBasePrompt && !mergedTrader.OverrideBasePrompt {
				validation.BlockingIssues = append(validation.BlockingIssues, "Autonomous prompt patches may not enable trader.override_base_prompt automatically.")
			} else if mergedTrader.OverrideBasePrompt != *patch.Trader.OverrideBasePrompt {
				nextValue := *patch.Trader.OverrideBasePrompt
				candidates = append(candidates, autonomousOptimizerPromptFieldCandidate{
					Label:    "trader.override_base_prompt",
					Priority: autonomousOptimizerPromptFieldPriority("trader.override_base_prompt"),
					Apply: func() {
						mergedTrader.OverrideBasePrompt = nextValue
					},
				})
			}
		}
		if patch.Trader.SystemPromptTemplate != nil {
			templateName := strings.TrimSpace(*patch.Trader.SystemPromptTemplate)
			if templateName == "" {
				validation.Warnings = append(validation.Warnings, "Ignored blank autonomous prompt patch for trader.system_prompt_template.")
				templateName = mergedTrader.SystemPromptTemplate
			}
			if templateName != "" && !strings.EqualFold(templateName, "strategy") {
				if _, err := decision.GetPromptTemplate(templateName); err != nil {
					validation.BlockingIssues = append(validation.BlockingIssues, fmt.Sprintf("Unknown system prompt template %q.", templateName))
				}
			}
			if mergedTrader.SystemPromptTemplate != templateName {
				nextTemplateName := templateName
				candidates = append(candidates, autonomousOptimizerPromptFieldCandidate{
					Label:    "trader.system_prompt_template",
					Priority: autonomousOptimizerPromptFieldPriority("trader.system_prompt_template"),
					Apply: func() {
						mergedTrader.SystemPromptTemplate = nextTemplateName
					},
				})
			}
		}
	}
	if patch.Optimizer != nil {
		if mergedOptimizer == nil {
			validation.BlockingIssues = append(validation.BlockingIssues, "Optimizer config is unavailable for optimizer prompt validation.")
		} else {
			addTextCandidate("optimizer.proposal_instructions", &mergedOptimizer.ProposalPromptInstructions, patch.Optimizer.ProposalInstructions)
			addTextCandidate("optimizer.critic_instructions", &mergedOptimizer.CriticPromptInstructions, patch.Optimizer.CriticInstructions)
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority == candidates[j].Priority {
			return candidates[i].Label < candidates[j].Label
		}
		return candidates[i].Priority < candidates[j].Priority
	})
	validation.RequestedFields = autonomousOptimizerPromptCandidateLabels(candidates)
	validation.RequestedFieldCount = len(validation.RequestedFields)
	selectedCandidates := candidates
	if len(selectedCandidates) > autonomousOptimizerMaxPromptPatchFields {
		validation.AutoTrimmed = true
		validation.DeferredFields = autonomousOptimizerPromptCandidateLabels(selectedCandidates[autonomousOptimizerMaxPromptPatchFields:])
		selectedCandidates = selectedCandidates[:autonomousOptimizerMaxPromptPatchFields]
		validation.Warnings = append(validation.Warnings,
			fmt.Sprintf(
				"Prompt patch requested %d fields; automatically applied the highest-priority %d fields and deferred %d for a later optimizer window.",
				validation.RequestedFieldCount,
				len(selectedCandidates),
				len(validation.DeferredFields),
			),
		)
	}
	for _, item := range selectedCandidates {
		if item.Apply != nil {
			item.Apply()
		}
	}
	validation.ChangedFields = autonomousOptimizerPromptCandidateLabels(selectedCandidates)
	validation.ChangedFieldCount = len(validation.ChangedFields)
	if len(validation.ChangedFields) == 0 {
		validation.BlockingIssues = append(validation.BlockingIssues, "Prompt patch does not modify any supported prompt surface.")
	}

	estimate := mergedStrategy.EstimateTokens()
	validation.EstimatedTokens = estimate.Total
	if modelCfg != nil {
		validation.ContextLimit = store.GetContextLimitForClient(modelCfg.Provider, modelName)
	}
	if validation.ContextLimit > 0 {
		usage := float64(validation.EstimatedTokens) / float64(validation.ContextLimit)
		switch {
		case usage > 0.90:
			validation.BlockingIssues = append(validation.BlockingIssues, fmt.Sprintf("Merged prompt would use about %d tokens against a %d token context limit.", validation.EstimatedTokens, validation.ContextLimit))
		case usage > 0.80:
			validation.Warnings = append(validation.Warnings, fmt.Sprintf("Merged prompt is estimated at %d tokens against a %d token context limit.", validation.EstimatedTokens, validation.ContextLimit))
		}
	}

	if warnings, err := validateStrategyConfig(mergedStrategy); err != nil {
		validation.BlockingIssues = append(validation.BlockingIssues, err.Error())
	} else if len(warnings) > 0 {
		validation.Warnings = append(validation.Warnings, warnings...)
	}

	if strings.TrimSpace(mergedStrategy.PromptSections.DecisionFormat) != "" {
		decisionFormat := strings.ToLower(mergedStrategy.PromptSections.DecisionFormat)
		if !strings.Contains(decisionFormat, "xml") && !strings.Contains(decisionFormat, "<decision>") && !strings.Contains(decisionFormat, "machine-parseable") {
			validation.Warnings = append(validation.Warnings, "Decision format guidance no longer references XML tags or machine-parseable output explicitly.")
		}
	}

	return mergedOptimizer, &mergedTrader, mergedStrategy, validation, nil
}

func validateAutonomousOptimizerConfigPatchScope(patch map[string]any) ([]string, []string) {
	if len(patch) == 0 {
		return nil, []string{"Config patch is empty."}
	}
	flat := map[string]string{}
	flattenJSONMap("", patch, flat)
	paths := make([]string, 0, len(flat))
	issues := []string{}
	for path := range flat {
		if strings.TrimSpace(path) == "" {
			continue
		}
		paths = append(paths, path)
		lower := strings.ToLower(path)
		switch {
		case lower == "custom_prompt" || lower == "prompt_sections" || strings.HasPrefix(lower, "prompt_sections."):
			issues = append(issues, fmt.Sprintf("Config patch must not modify prompt field %q; use prompt_patch instead.", path))
		case strings.Contains(lower, "api_key"):
			issues = append(issues, fmt.Sprintf("Config patch may not modify credential-like field %q.", path))
		}
	}
	sort.Strings(paths)
	if len(paths) > autonomousOptimizerMaxConfigPatchPaths {
		issues = append(issues, fmt.Sprintf("Config patch changes %d paths, which exceeds the limit of %d.", len(paths), autonomousOptimizerMaxConfigPatchPaths))
	}
	return paths, dedupeSortedStrings(issues)
}

func buildAutonomousOptimizerPromptBundle(cfg *store.AutonomousOptimizerConfig, traderCfg *store.Trader, strategyCfg *store.StrategyConfig) map[string]any {
	if traderCfg == nil || strategyCfg == nil {
		return map[string]any{}
	}
	bundle := map[string]any{
		"strategy": map[string]any{
			"custom_prompt": strategyCfg.CustomPrompt,
			"prompt_sections": map[string]any{
				"role_definition":   strategyCfg.PromptSections.RoleDefinition,
				"trading_frequency": strategyCfg.PromptSections.TradingFrequency,
				"entry_standards":   strategyCfg.PromptSections.EntryStandards,
				"market_context":    strategyCfg.PromptSections.MarketContext,
				"decision_process":  strategyCfg.PromptSections.DecisionProcess,
				"decision_format":   strategyCfg.PromptSections.DecisionFormat,
			},
		},
		"trader": buildAutonomousOptimizerTraderSnapshot(traderCfg),
	}
	if cfg != nil {
		bundle["optimizer"] = buildAutonomousOptimizerConfigPromptSnapshot(cfg)
	}
	return bundle
}

func buildAutonomousOptimizerTraderSnapshot(traderCfg *store.Trader) map[string]any {
	if traderCfg == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":                     traderCfg.ID,
		"strategy_id":            traderCfg.StrategyID,
		"custom_prompt":          traderCfg.CustomPrompt,
		"override_base_prompt":   traderCfg.OverrideBasePrompt,
		"system_prompt_template": traderCfg.SystemPromptTemplate,
		"scan_interval_minutes":  traderCfg.ScanIntervalMinutes,
		"invert_signals":         traderCfg.InvertSignals,
	}
}

func buildAutonomousOptimizerValidationDatasetFilters(bundle *autonomousOptimizerWindowBundle) map[string]any {
	now := time.Now().UTC()
	from := now.Add(-30 * 24 * time.Hour)
	if bundle != nil && !bundle.WindowEnd.IsZero() {
		now = bundle.WindowEnd.UTC()
		from = now.Add(-30 * 24 * time.Hour)
	}
	return map[string]any{
		"status":    store.DealReviewCaseStatusClosed,
		"from_time": from.UnixMilli(),
		"to_time":   now.UnixMilli(),
	}
}

func buildAutonomousOptimizerStarvationMetrics(review *store.TraderBucketReview) map[string]any {
	if review == nil {
		return map[string]any{}
	}
	return map[string]any{
		"record_count":               review.RecordCount,
		"candidate_count":            review.TotalCandidates,
		"open_decision_count":        review.TotalOpenDecisions,
		"rejected_candidate_count":   review.RejectedCandidateCount,
		"hold_decision_count":        review.HoldDecisionCount,
		"wait_decision_count":        review.WaitDecisionCount,
		"decision_conversion_rate":   review.DecisionConversionRate,
		"avg_decision_confidence":    review.AvgDecisionConfidence,
		"reject_reasons":             review.RejectReasons,
		"confidence_bands":           review.ConfidenceBands,
		"opportunity_sessions":       review.OpportunitySessions,
		"opportunity_symbols":        review.OpportunitySymbols,
		"execution_statuses":         review.ExecutionStatuses,
		"recent_open_executions":     review.RecentOpenExecutions,
		"regime_summaries":           review.RegimeSummaries,
		"cycles_with_candidates":     review.CyclesWithCandidates,
		"cycles_with_open_decisions": review.CyclesWithOpenDecisions,
	}
}

func summarizeAutonomousOptimizerRuns(runs []*store.AutonomousOptimizerRun) []map[string]any {
	result := make([]map[string]any, 0, len(runs))
	for _, item := range runs {
		if item == nil {
			continue
		}
		entry := map[string]any{
			"id":                          item.ID,
			"trigger":                     item.Trigger,
			"status":                      item.Status,
			"summary":                     clipDealReviewAIScanText(item.Summary, 320),
			"started_at":                  item.StartedAt.UTC().Format(time.RFC3339),
			"completed_at":                item.CompletedAt.UTC().Format(time.RFC3339),
			"applied_strategy_version_id": item.AppliedStrategyVersionID,
			"primary_model_name":          item.PrimaryModelName,
			"critic_model_name":           item.CriticModelName,
		}
		validation := parseAutonomousOptimizerJSONObject(item.ValidationJSON)
		if gateReasons := autonomousOptimizerStringSlice(validation["gate_reasons"]); len(gateReasons) > 0 {
			entry["gate_reasons"] = gateReasons
		}
		critic := parseAutonomousOptimizerNestedObject(validation, "critic")
		if len(critic) > 0 {
			if summary := autonomousOptimizerString(critic["summary"]); summary != "" {
				entry["critic_summary"] = clipDealReviewAIScanText(summary, 240)
			}
			if action := autonomousOptimizerString(critic["recommended_action"]); action != "" {
				entry["critic_recommended_action"] = action
			}
			if blockingIssues := autonomousOptimizerStringSlice(critic["blocking_issues"]); len(blockingIssues) > 0 {
				entry["critic_blocking_issues"] = blockingIssues
			}
			if refs := autonomousOptimizerStringSlice(critic["symbol_prior_references"]); len(refs) > 0 {
				entry["critic_symbol_prior_references"] = refs
			}
		}
		promptValidation := parseAutonomousOptimizerNestedObject(validation, "prompt_validation")
		if len(promptValidation) > 0 {
			if count, ok := promptValidation["requested_field_count"].(float64); ok && count > 0 {
				entry["prompt_requested_field_count"] = int(count)
			}
			if count, ok := promptValidation["changed_field_count"].(float64); ok && count > 0 {
				entry["prompt_applied_field_count"] = int(count)
			}
			if deferred := autonomousOptimizerStringSlice(promptValidation["deferred_fields"]); len(deferred) > 0 {
				entry["prompt_deferred_fields"] = deferred
			}
			if trimmed, ok := promptValidation["auto_trimmed"].(bool); ok {
				entry["prompt_auto_trimmed"] = trimmed
			}
		}
		result = append(result, entry)
		if len(result) >= 6 {
			break
		}
	}
	return result
}

func buildAutonomousOptimizerLatestGateFeedback(runs []*store.AutonomousOptimizerRun) map[string]any {
	for _, item := range runs {
		if item == nil {
			continue
		}
		switch item.Status {
		case store.AutonomousOptimizerStatusBlockedByGate, store.AutonomousOptimizerStatusDeferredForNextWindow:
		default:
			continue
		}

		validation := parseAutonomousOptimizerJSONObject(item.ValidationJSON)
		metadata := parseAutonomousOptimizerJSONObject(item.MetadataJSON)
		critic := parseAutonomousOptimizerNestedObject(validation, "critic")
		promptValidation := parseAutonomousOptimizerNestedObject(validation, "prompt_validation")
		proposal := parseAutonomousOptimizerNestedObject(metadata, "proposal")

		feedback := map[string]any{
			"run_id":       item.ID,
			"status":       item.Status,
			"trigger":      item.Trigger,
			"summary":      clipDealReviewAIScanText(item.Summary, 320),
			"started_at":   item.StartedAt.UTC().Format(time.RFC3339),
			"completed_at": item.CompletedAt.UTC().Format(time.RFC3339),
			"gate_reasons": autonomousOptimizerStringSlice(validation["gate_reasons"]),
			"deferred_reasons": autonomousOptimizerStringSlice(
				validation["deferred_reasons"],
			),
			"config_patch_paths": autonomousOptimizerStringSlice(
				validation["config_patch_paths"],
			),
		}
		if len(critic) > 0 {
			feedback["critic"] = map[string]any{
				"approved":           autonomousOptimizerBool(critic["approved"]),
				"recommended_action": autonomousOptimizerString(critic["recommended_action"]),
				"summary": clipDealReviewAIScanText(
					autonomousOptimizerString(critic["summary"]),
					240,
				),
				"blocking_issues":         autonomousOptimizerStringSlice(critic["blocking_issues"]),
				"warnings":                autonomousOptimizerStringSlice(critic["warnings"]),
				"symbol_prior_references": autonomousOptimizerStringSlice(critic["symbol_prior_references"]),
			}
		}
		if len(promptValidation) > 0 {
			promptFeedback := map[string]any{}
			if requested, ok := promptValidation["requested_field_count"].(float64); ok && requested > 0 {
				promptFeedback["requested_field_count"] = int(requested)
			}
			if changed, ok := promptValidation["changed_field_count"].(float64); ok && changed > 0 {
				promptFeedback["changed_field_count"] = int(changed)
			}
			if deferred := autonomousOptimizerStringSlice(promptValidation["deferred_fields"]); len(deferred) > 0 {
				promptFeedback["deferred_fields"] = deferred
			}
			if issues := autonomousOptimizerStringSlice(promptValidation["blocking_issues"]); len(issues) > 0 {
				promptFeedback["blocking_issues"] = issues
			}
			if len(promptFeedback) > 0 {
				feedback["prompt_validation"] = promptFeedback
			}
		}
		if len(proposal) > 0 {
			proposalFeedback := map[string]any{}
			if proposalType := autonomousOptimizerString(proposal["proposal_type"]); proposalType != "" {
				proposalFeedback["proposal_type"] = proposalType
			}
			if expectedEffect := autonomousOptimizerString(proposal["expected_effect"]); expectedEffect != "" {
				proposalFeedback["expected_effect"] = clipDealReviewAIScanText(expectedEffect, 240)
			}
			if rationale := autonomousOptimizerStringSlice(proposal["rationale"]); len(rationale) > 0 {
				proposalFeedback["rationale"] = rationale
			}
			if refs := autonomousOptimizerStringSlice(proposal["symbol_prior_references"]); len(refs) > 0 {
				proposalFeedback["symbol_prior_references"] = refs
			}
			if len(proposalFeedback) > 0 {
				feedback["proposal"] = proposalFeedback
			}
		}
		return feedback
	}
	return map[string]any{}
}

func sanitizeAutonomousOptimizerBacklogProposal(item autonomousOptimizerBacklogProposal) autonomousOptimizerBacklogProposal {
	item.Title = strings.TrimSpace(item.Title)
	item.Category = normalizeAutonomousOptimizerBacklogCategory(item.Category)
	item.Description = strings.TrimSpace(item.Description)
	item.ExpectedImpact = strings.TrimSpace(item.ExpectedImpact)
	item.Confidence = clampAutonomousOptimizerScore(item.Confidence)
	item.ImplementationCost = clampAutonomousOptimizerScore(item.ImplementationCost)
	item.Urgency = clampAutonomousOptimizerScore(item.Urgency)
	item.Evidence = sanitizeAutonomousOptimizerJSONObjectArray(item.Evidence)
	item.Metadata = sanitizeAutonomousOptimizerJSONMap(item.Metadata)
	if item.RecurrenceCount <= 0 {
		item.RecurrenceCount = 1
	}
	return item
}

func normalizeAutonomousOptimizerProposalType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "config_patch", "prompt_patch", "config_and_prompt_patch", "backlog_only", "pause_optimizer":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "no_change"
	}
}

func normalizeAutonomousOptimizerCriticAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "approve_apply", "approve_backlog_only", "block_apply", "pause_optimizer":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "block_apply"
	}
}

func normalizeAutonomousOptimizerBacklogCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "missing_indicator", "missing_market_data", "missing_execution_telemetry", "missing_regime_metadata", "missing_risk_control", "missing_prompt_instruction", "missing_review_metric":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "other_capability_gap"
	}
}

func sanitizeAutonomousOptimizerJSONMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	normalized := map[string]any{}
	if err := json.Unmarshal(body, &normalized); err != nil {
		return map[string]any{}
	}
	return normalized
}

func sanitizeAutonomousOptimizerJSONObjectArray(value []map[string]any) []map[string]any {
	if len(value) == 0 {
		return nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var normalized []map[string]any
	if err := json.Unmarshal(body, &normalized); err != nil {
		return nil
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func clampAutonomousOptimizerScore(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func filterNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return dedupeSortedStrings(out)
}

func dedupeSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}
