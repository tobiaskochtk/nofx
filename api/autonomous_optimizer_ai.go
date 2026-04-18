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
	ExecutiveSummary string                               `json:"executive_summary"`
	ProposalType     string                               `json:"proposal_type"`
	Rationale        []string                             `json:"rationale,omitempty"`
	ExpectedEffect   string                               `json:"expected_effect,omitempty"`
	EvidenceStrength float64                              `json:"evidence_strength,omitempty"`
	Confidence       float64                              `json:"confidence,omitempty"`
	ConfigPatch      map[string]any                       `json:"config_patch,omitempty"`
	PromptPatch      map[string]any                       `json:"prompt_patch,omitempty"`
	BacklogItems     []autonomousOptimizerBacklogProposal `json:"backlog_items,omitempty"`
	PauseReason      string                               `json:"pause_reason,omitempty"`
}

type autonomousOptimizerBacklogProposal struct {
	Title              string  `json:"title"`
	Category           string  `json:"category"`
	Description        string  `json:"description"`
	ExpectedImpact     string  `json:"expected_impact"`
	Confidence         float64 `json:"confidence"`
	ImplementationCost float64 `json:"implementation_cost"`
	Urgency            float64 `json:"urgency"`
	RecurrenceCount    int     `json:"recurrence_count"`
}

type autonomousOptimizerCriticResult struct {
	Approved          bool     `json:"approved"`
	Confidence        float64  `json:"confidence"`
	RecommendedAction string   `json:"recommended_action"`
	Summary           string   `json:"summary"`
	BlockingIssues    []string `json:"blocking_issues,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
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

func buildAutonomousOptimizerReviewPayload(cfg *store.AutonomousOptimizerConfig, traderCfg *store.Trader, strategyRecord *store.Strategy, strategyCfg *store.StrategyConfig, bundle *autonomousOptimizerWindowBundle, recentRuns []*store.AutonomousOptimizerRun) (map[string]any, error) {
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
  "blocking_issues": ["string"],
  "warnings": ["string"]
}
Rules:
- Block proposals that overreach the evidence, attempt unsupported edits, or look too broad for one review window.
- If the proposal is useful only as a backlog note, use "approve_backlog_only".
- Prompt patches may be approved on low-trade evidence when they are narrow and explicitly aimed at inactivity or decision quality.
- Config patches need materially stronger evidence than prompt patches.
- Use recent_optimizer_runs and decision_starvation_metrics to spot repeated loops, over-filtering, or attempts to fix the same issue without new evidence.
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
				"blocking_issues": autonomousOptimizerStringSlice(critic["blocking_issues"]),
				"warnings":        autonomousOptimizerStringSlice(critic["warnings"]),
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
