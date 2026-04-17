package api

import (
	"encoding/json"
	"fmt"
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
	ChangedFields     []string `json:"changed_fields,omitempty"`
	ChangedFieldCount int      `json:"changed_field_count,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
	BlockingIssues    []string `json:"blocking_issues,omitempty"`
	EstimatedTokens   int      `json:"estimated_tokens,omitempty"`
	ContextLimit      int      `json:"context_limit,omitempty"`
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
		},
	}, nil
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
	basePayload["current_prompt_bundle"] = buildAutonomousOptimizerPromptBundle(cfg, traderCfg, strategyCfg)
	basePayload["recent_optimizer_runs"] = summarizeAutonomousOptimizerRuns(recentRuns)
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
- When inactivity is the problem, cite concrete reject reasons, confidence bands, sessions, or symbols from the supplied telemetry.
- Keep patches minimal and high-signal. Do not modify credentials, exchange bindings, or unrelated trader settings.
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
	validation := &autonomousOptimizerPromptValidation{}
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

	applyText := func(label string, target *string, value *string) {
		if value == nil {
			return
		}
		trimmed := strings.TrimSpace(*value)
		if len([]rune(trimmed)) > autonomousOptimizerMaxPromptFieldChars {
			validation.BlockingIssues = append(validation.BlockingIssues, fmt.Sprintf("%s exceeds the %d character limit.", label, autonomousOptimizerMaxPromptFieldChars))
			return
		}
		*target = trimmed
		validation.ChangedFields = append(validation.ChangedFields, label)
	}

	if patch.Strategy != nil {
		applyText("strategy.custom_prompt", &mergedStrategy.CustomPrompt, patch.Strategy.CustomPrompt)
		if patch.Strategy.PromptSections != nil {
			applyText("strategy.prompt_sections.role_definition", &mergedStrategy.PromptSections.RoleDefinition, patch.Strategy.PromptSections.RoleDefinition)
			applyText("strategy.prompt_sections.trading_frequency", &mergedStrategy.PromptSections.TradingFrequency, patch.Strategy.PromptSections.TradingFrequency)
			applyText("strategy.prompt_sections.entry_standards", &mergedStrategy.PromptSections.EntryStandards, patch.Strategy.PromptSections.EntryStandards)
			applyText("strategy.prompt_sections.market_context", &mergedStrategy.PromptSections.MarketContext, patch.Strategy.PromptSections.MarketContext)
			applyText("strategy.prompt_sections.decision_process", &mergedStrategy.PromptSections.DecisionProcess, patch.Strategy.PromptSections.DecisionProcess)
			applyText("strategy.prompt_sections.decision_format", &mergedStrategy.PromptSections.DecisionFormat, patch.Strategy.PromptSections.DecisionFormat)
		}
	}
	if patch.Trader != nil {
		applyText("trader.custom_prompt", &mergedTrader.CustomPrompt, patch.Trader.CustomPrompt)
		if patch.Trader.OverrideBasePrompt != nil {
			if *patch.Trader.OverrideBasePrompt && !mergedTrader.OverrideBasePrompt {
				validation.BlockingIssues = append(validation.BlockingIssues, "Autonomous prompt patches may not enable trader.override_base_prompt automatically.")
			} else if mergedTrader.OverrideBasePrompt != *patch.Trader.OverrideBasePrompt {
				mergedTrader.OverrideBasePrompt = *patch.Trader.OverrideBasePrompt
				validation.ChangedFields = append(validation.ChangedFields, "trader.override_base_prompt")
			}
		}
		if patch.Trader.SystemPromptTemplate != nil {
			templateName := strings.TrimSpace(*patch.Trader.SystemPromptTemplate)
			if templateName != "" && !strings.EqualFold(templateName, "strategy") {
				if _, err := decision.GetPromptTemplate(templateName); err != nil {
					validation.BlockingIssues = append(validation.BlockingIssues, fmt.Sprintf("Unknown system prompt template %q.", templateName))
				}
			}
			if mergedTrader.SystemPromptTemplate != templateName {
				mergedTrader.SystemPromptTemplate = templateName
				validation.ChangedFields = append(validation.ChangedFields, "trader.system_prompt_template")
			}
		}
	}
	if patch.Optimizer != nil {
		if mergedOptimizer == nil {
			validation.BlockingIssues = append(validation.BlockingIssues, "Optimizer config is unavailable for optimizer prompt validation.")
		} else {
			applyText("optimizer.proposal_instructions", &mergedOptimizer.ProposalPromptInstructions, patch.Optimizer.ProposalInstructions)
			applyText("optimizer.critic_instructions", &mergedOptimizer.CriticPromptInstructions, patch.Optimizer.CriticInstructions)
		}
	}

	validation.ChangedFields = dedupeSortedStrings(validation.ChangedFields)
	validation.ChangedFieldCount = len(validation.ChangedFields)
	if len(validation.ChangedFields) == 0 {
		validation.BlockingIssues = append(validation.BlockingIssues, "Prompt patch does not modify any supported prompt surface.")
	}
	if len(validation.ChangedFields) > autonomousOptimizerMaxPromptPatchFields {
		validation.BlockingIssues = append(validation.BlockingIssues, fmt.Sprintf("Prompt patch touches %d fields, which exceeds the limit of %d.", len(validation.ChangedFields), autonomousOptimizerMaxPromptPatchFields))
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
		"hold_decision_count":        review.HoldDecisionCount,
		"wait_decision_count":        review.WaitDecisionCount,
		"decision_conversion_rate":   review.DecisionConversionRate,
		"avg_decision_confidence":    review.AvgDecisionConfidence,
		"reject_reasons":             review.RejectReasons,
		"confidence_bands":           review.ConfidenceBands,
		"opportunity_sessions":       review.OpportunitySessions,
		"opportunity_symbols":        review.OpportunitySymbols,
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
		result = append(result, map[string]any{
			"id":                          item.ID,
			"trigger":                     item.Trigger,
			"status":                      item.Status,
			"summary":                     clipDealReviewAIScanText(item.Summary, 320),
			"started_at":                  item.StartedAt.UTC().Format(time.RFC3339),
			"completed_at":                item.CompletedAt.UTC().Format(time.RFC3339),
			"applied_strategy_version_id": item.AppliedStrategyVersionID,
			"primary_model_name":          item.PrimaryModelName,
			"critic_model_name":           item.CriticModelName,
		})
		if len(result) >= 6 {
			break
		}
	}
	return result
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
