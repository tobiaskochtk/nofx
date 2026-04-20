package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"nofx/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type autonomousOptimizerConfigRequest struct {
	Enabled                    *bool   `json:"enabled"`
	ReviewIntervalHours        *int    `json:"review_interval_hours"`
	AutoApplyCooldownHours     *int    `json:"auto_apply_cooldown_hours"`
	MaxConsecutiveAutoApplies  *int    `json:"max_consecutive_auto_applies"`
	AutoApplyConfigPatch       *bool   `json:"auto_apply_config_patch"`
	AutoApplyPromptPatch       *bool   `json:"auto_apply_prompt_patch"`
	AutoRollbackEnabled        *bool   `json:"auto_rollback_enabled"`
	SelfPauseEnabled           *bool   `json:"self_pause_enabled"`
	Status                     string  `json:"status"`
	PrimaryModelConfigID       string  `json:"primary_model_config_id"`
	PrimaryModelName           string  `json:"primary_model_name"`
	CriticModelConfigID        string  `json:"critic_model_config_id"`
	CriticModelName            string  `json:"critic_model_name"`
	ProposalPromptInstructions *string `json:"proposal_prompt_instructions"`
	CriticPromptInstructions   *string `json:"critic_prompt_instructions"`
}

type autonomousOptimizerBootstrapRequest struct {
	SourceTraderID string `json:"source_trader_id"`
}

type autonomousOptimizerBacklogRequest struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Category           string   `json:"category"`
	Description        string   `json:"description"`
	ExpectedImpact     string   `json:"expected_impact"`
	Confidence         *float64 `json:"confidence"`
	ImplementationCost *float64 `json:"implementation_cost"`
	Urgency            *float64 `json:"urgency"`
	RecurrenceCount    *int     `json:"recurrence_count"`
	CompositeScore     *float64 `json:"composite_score"`
	Status             string   `json:"status"`
	AIGenerated        *bool    `json:"ai_generated"`
	UserEdited         *bool    `json:"user_edited"`
}

type autonomousOptimizerRunDetailResponse struct {
	Run                  *store.AutonomousOptimizerRun          `json:"run"`
	ConfigPatch          map[string]any                         `json:"config_patch,omitempty"`
	PromptPatch          map[string]any                         `json:"prompt_patch,omitempty"`
	Validation           map[string]any                         `json:"validation,omitempty"`
	Metadata             map[string]any                         `json:"metadata,omitempty"`
	GateReasons          []string                               `json:"gate_reasons,omitempty"`
	StrategyDifferences  []dealReviewJSONDiffEntry              `json:"strategy_differences,omitempty"`
	TraderDifferences    []dealReviewJSONDiffEntry              `json:"trader_differences,omitempty"`
	OptimizerDifferences []dealReviewJSONDiffEntry              `json:"optimizer_differences,omitempty"`
	ReviewWindowStartMS  int64                                  `json:"review_window_start_ms,omitempty"`
	ReviewWindowEndMS    int64                                  `json:"review_window_end_ms,omitempty"`
	LinkedStrategy       *store.DealReviewStrategyVersionDetail `json:"linked_strategy_version,omitempty"`
}

type autonomousOptimizerModelOutcomeEntry struct {
	ModelKey             string         `json:"model_key"`
	ModelLabel           string         `json:"model_label"`
	PrimaryModelConfigID string         `json:"primary_model_config_id,omitempty"`
	PrimaryModelName     string         `json:"primary_model_name,omitempty"`
	PrimaryModelLabel    string         `json:"primary_model_label,omitempty"`
	CriticModelConfigID  string         `json:"critic_model_config_id,omitempty"`
	CriticModelName      string         `json:"critic_model_name,omitempty"`
	CriticModelLabel     string         `json:"critic_model_label,omitempty"`
	TotalRuns            int            `json:"total_runs"`
	ApplyCount           int            `json:"apply_count"`
	ApplyRate            float64        `json:"apply_rate"`
	MonitoringCount      int            `json:"monitoring_count"`
	KeptCount            int            `json:"kept_count"`
	KeptWinCount         int            `json:"kept_win_count"`
	KeptWinRate          float64        `json:"kept_win_rate"`
	RollbackCount        int            `json:"rollback_count"`
	RollbackRate         float64        `json:"rollback_rate"`
	FailedCount          int            `json:"failed_count"`
	StaleRecoveryCount   int            `json:"stale_recovery_count"`
	InsufficientEvidence int            `json:"insufficient_evidence_count"`
	FailureEvidenceGaps  int            `json:"failure_overlap_insufficient_evidence_count"`
	OperationalHealth    float64        `json:"operational_health_score"`
	BacklogItemCount     int            `json:"backlog_item_count"`
	UsefulBacklogCount   int            `json:"useful_backlog_count"`
	DoneBacklogCount     int            `json:"done_backlog_count"`
	RejectedBacklogCount int            `json:"rejected_backlog_count"`
	BacklogUsefulness    float64        `json:"backlog_usefulness"`
	OutcomeScore         float64        `json:"outcome_score"`
	LastUsedAt           time.Time      `json:"last_used_at"`
	StatusCounts         map[string]int `json:"status_counts,omitempty"`
}

type autonomousOptimizerModelOutcomeResponse struct {
	Items []autonomousOptimizerModelOutcomeEntry `json:"items"`
}

func (s *Server) handleTraderAutonomousOptimizerConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	cfg, err := s.store.AutonomousOptimizer().GetOrCreateConfig(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Failed to load autonomous optimizer config", err)
		return
	}
	if err := s.ensureAutonomousOptimizerModelDefaults(userID, cfg); err != nil {
		SafeInternalError(c, "Failed to resolve autonomous optimizer model defaults", err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func (s *Server) handleTraderAutonomousOptimizerUpdateConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	var req autonomousOptimizerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid autonomous optimizer config payload")
		return
	}

	cfg, err := s.store.AutonomousOptimizer().GetOrCreateConfig(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Failed to load autonomous optimizer config", err)
		return
	}

	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
		if cfg.Enabled && cfg.Status == store.AutonomousOptimizerStatusPaused {
			cfg.Status = store.AutonomousOptimizerStatusScheduled
		}
		if !cfg.Enabled {
			cfg.Status = store.AutonomousOptimizerStatusPaused
		}
	}
	if req.ReviewIntervalHours != nil {
		cfg.ReviewIntervalHours = *req.ReviewIntervalHours
		cfg.NextRunAt = time.Now().UTC().Add(time.Duration(store.AutonomousOptimizerDefaultReviewIntervalHours) * time.Hour)
		if cfg.ReviewIntervalHours > 0 {
			cfg.NextRunAt = time.Now().UTC().Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)
		}
	}
	if req.AutoApplyCooldownHours != nil {
		cfg.AutoApplyCooldownHours = *req.AutoApplyCooldownHours
	}
	if req.MaxConsecutiveAutoApplies != nil {
		cfg.MaxConsecutiveAutoApplies = *req.MaxConsecutiveAutoApplies
	}
	if req.AutoApplyConfigPatch != nil {
		cfg.AutoApplyConfigPatch = *req.AutoApplyConfigPatch
	}
	if req.AutoApplyPromptPatch != nil {
		cfg.AutoApplyPromptPatch = *req.AutoApplyPromptPatch
	}
	if req.AutoRollbackEnabled != nil {
		cfg.AutoRollbackEnabled = *req.AutoRollbackEnabled
	}
	if req.SelfPauseEnabled != nil {
		cfg.SelfPauseEnabled = *req.SelfPauseEnabled
	}
	if strings.TrimSpace(req.Status) != "" {
		cfg.Status = req.Status
	}
	if strings.TrimSpace(req.PrimaryModelConfigID) != "" {
		modelCfg, err := s.store.AIModel().Get(userID, strings.TrimSpace(req.PrimaryModelConfigID))
		if err != nil || !modelCfg.Enabled {
			SafeBadRequest(c, "Selected primary model config is not available")
			return
		}
		cfg.PrimaryModelConfigID = modelCfg.ID
	}
	if strings.TrimSpace(req.PrimaryModelName) != "" {
		cfg.PrimaryModelName = strings.TrimSpace(req.PrimaryModelName)
	}
	if strings.TrimSpace(req.CriticModelConfigID) != "" {
		modelCfg, err := s.store.AIModel().Get(userID, strings.TrimSpace(req.CriticModelConfigID))
		if err != nil || !modelCfg.Enabled {
			SafeBadRequest(c, "Selected critic model config is not available")
			return
		}
		cfg.CriticModelConfigID = modelCfg.ID
	}
	if strings.TrimSpace(req.CriticModelName) != "" {
		cfg.CriticModelName = strings.TrimSpace(req.CriticModelName)
	}
	if req.ProposalPromptInstructions != nil {
		cfg.ProposalPromptInstructions = strings.TrimSpace(*req.ProposalPromptInstructions)
	}
	if req.CriticPromptInstructions != nil {
		cfg.CriticPromptInstructions = strings.TrimSpace(*req.CriticPromptInstructions)
	}
	if err := s.ensureAutonomousOptimizerModelDefaults(userID, cfg); err != nil {
		SafeInternalError(c, "Failed to resolve autonomous optimizer model defaults", err)
		return
	}
	if err := s.store.AutonomousOptimizer().SaveConfig(cfg); err != nil {
		SafeInternalError(c, "Failed to save autonomous optimizer config", err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func (s *Server) handleTraderAutonomousOptimizerRuns(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	limit := 20
	if raw := strings.TrimSpace(c.DefaultQuery("limit", "20")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	items, err := s.store.AutonomousOptimizer().ListRuns(userID, traderID, limit)
	if err != nil {
		SafeInternalError(c, "Failed to fetch autonomous optimizer runs", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) handleTraderAutonomousOptimizerRunNow(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	cfg, err := s.store.AutonomousOptimizer().GetOrCreateConfig(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Failed to load autonomous optimizer config", err)
		return
	}
	if err := s.ensureAutonomousOptimizerModelDefaults(userID, cfg); err != nil {
		SafeInternalError(c, "Failed to resolve autonomous optimizer model defaults", err)
		return
	}
	if !cfg.Enabled {
		SafeBadRequest(c, "Autonomous optimizer is disabled for this trader")
		return
	}
	if cfg.Status == store.AutonomousOptimizerStatusRunning {
		c.JSON(http.StatusConflict, APIErrorResponse{
			Error: "Autonomous optimizer is already running for this trader",
		})
		return
	}

	now := time.Now().UTC()
	if err := s.recoverStaleAutonomousOptimizerState(now); err != nil {
		SafeInternalError(c, "Failed to recover autonomous optimizer state before manual run", err)
		return
	}
	if cfg.Status == store.AutonomousOptimizerStatusPaused {
		cfg.Status = store.AutonomousOptimizerStatusScheduled
	}
	cfg.NextRunAt = now.Add(-1 * time.Second)
	if err := s.store.AutonomousOptimizer().SaveConfig(cfg); err != nil {
		SafeInternalError(c, "Failed to schedule immediate autonomous optimizer run", err)
		return
	}
	if err := s.processAutonomousOptimizerConfig(cfg, now, store.AutonomousOptimizerRunTriggerManual); err != nil {
		SafeInternalError(c, "Failed to execute autonomous optimizer run", err)
		return
	}

	run, err := s.store.AutonomousOptimizer().GetRun(userID, traderID, cfg.LastRunID)
	if err != nil {
		SafeInternalError(c, "Manual optimizer run completed but could not be reloaded", err)
		return
	}
	detail, err := s.buildAutonomousOptimizerRunDetailResponse(userID, traderID, run)
	if err != nil {
		SafeInternalError(c, "Manual optimizer run completed but detail serialization failed", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleTraderAutonomousOptimizerModelOutcomes(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	runs, err := s.store.AutonomousOptimizer().ListRuns(userID, traderID, 250)
	if err != nil {
		SafeInternalError(c, "Failed to fetch autonomous optimizer runs", err)
		return
	}
	backlog, err := s.store.AutonomousOptimizer().ListBacklog(userID, traderID, 200)
	if err != nil {
		SafeInternalError(c, "Failed to fetch autonomous optimizer backlog", err)
		return
	}
	models, err := s.store.AIModel().List(userID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch AI model configs for optimizer outcomes", err)
		return
	}

	c.JSON(http.StatusOK, autonomousOptimizerModelOutcomeResponse{
		Items: buildAutonomousOptimizerModelOutcomes(runs, backlog, models),
	})
}

func (s *Server) handleTraderAutonomousOptimizerRunDetail(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	runID := c.Param("runId")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	run, err := s.store.AutonomousOptimizer().GetRun(userID, traderID, runID)
	if err != nil {
		SafeNotFound(c, "Autonomous optimizer run")
		return
	}

	detail, err := s.buildAutonomousOptimizerRunDetailResponse(userID, traderID, run)
	if err != nil {
		SafeInternalError(c, "Failed to build autonomous optimizer run detail", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) buildAutonomousOptimizerRunDetailResponse(userID, traderID string, run *store.AutonomousOptimizerRun) (*autonomousOptimizerRunDetailResponse, error) {
	if run == nil {
		return nil, gorm.ErrRecordNotFound
	}

	detail := &autonomousOptimizerRunDetailResponse{
		Run:         run,
		ConfigPatch: parseAutonomousOptimizerJSONObject(run.ConfigPatchJSON),
		PromptPatch: parseAutonomousOptimizerJSONObject(run.PromptPatchJSON),
		Validation:  parseAutonomousOptimizerJSONObject(run.ValidationJSON),
		Metadata:    parseAutonomousOptimizerJSONObject(run.MetadataJSON),
	}
	detail.GateReasons = autonomousOptimizerStringSliceFromValue(detail.Validation["gate_reasons"])
	if start, ok := autonomousOptimizerInt64FromValue(detail.Metadata["window_start_ms"]); ok {
		detail.ReviewWindowStartMS = start
	}
	if end, ok := autonomousOptimizerInt64FromValue(detail.Metadata["window_end_ms"]); ok {
		detail.ReviewWindowEndMS = end
	}

	applyData := parseAutonomousOptimizerNestedObject(detail.Metadata, "apply")
	previousStrategy := parseAutonomousOptimizerNestedObject(applyData, "previous_strategy_config")
	nextStrategy := parseAutonomousOptimizerNestedObject(applyData, "next_strategy_config")
	if len(previousStrategy) > 0 || len(nextStrategy) > 0 {
		detail.StrategyDifferences = diffJSONMaps(previousStrategy, nextStrategy)
	}
	previousTrader := parseAutonomousOptimizerNestedObject(applyData, "previous_trader")
	nextTrader := parseAutonomousOptimizerNestedObject(applyData, "next_trader")
	if len(previousTrader) > 0 || len(nextTrader) > 0 {
		detail.TraderDifferences = diffJSONMaps(previousTrader, nextTrader)
	}
	previousOptimizer := parseAutonomousOptimizerNestedObject(applyData, "previous_optimizer_config")
	nextOptimizer := parseAutonomousOptimizerNestedObject(applyData, "next_optimizer_config")
	if len(previousOptimizer) > 0 || len(nextOptimizer) > 0 {
		detail.OptimizerDifferences = diffJSONMaps(previousOptimizer, nextOptimizer)
	}

	if strings.TrimSpace(run.AppliedStrategyVersionID) != "" {
		if version, versionErr := s.store.DealReview().GetStrategyVersion(userID, traderID, run.AppliedStrategyVersionID); versionErr == nil {
			detail.LinkedStrategy = version
		}
	}
	return detail, nil
}

func (s *Server) handleTraderAutonomousOptimizerBacklog(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	limit := 50
	if raw := strings.TrimSpace(c.DefaultQuery("limit", "50")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	items, err := s.store.AutonomousOptimizer().ListBacklog(userID, traderID, limit)
	if err != nil {
		SafeInternalError(c, "Failed to fetch autonomous optimizer backlog", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) handleTraderAutonomousOptimizerSaveBacklog(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	var req autonomousOptimizerBacklogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid autonomous optimizer backlog payload")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		SafeBadRequest(c, "Backlog title is required")
		return
	}

	var item *store.AutonomousOptimizerBacklogItem
	itemID := strings.TrimSpace(req.ID)
	if itemID != "" {
		existing, err := s.store.AutonomousOptimizer().GetBacklogItem(userID, traderID, itemID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				SafeNotFound(c, "Autonomous optimizer backlog item")
				return
			}
			SafeInternalError(c, "Failed to load autonomous optimizer backlog item", err)
			return
		}
		item = mergeAutonomousOptimizerBacklogUpdate(existing, req)
	} else {
		item = &store.AutonomousOptimizerBacklogItem{
			ID:             itemID,
			UserID:         userID,
			TraderID:       traderID,
			Title:          strings.TrimSpace(req.Title),
			Category:       strings.TrimSpace(req.Category),
			Description:    strings.TrimSpace(req.Description),
			ExpectedImpact: strings.TrimSpace(req.ExpectedImpact),
			Status:         strings.TrimSpace(req.Status),
			AIGenerated:    true,
			UserEdited:     false,
		}
		if req.Confidence != nil {
			item.Confidence = *req.Confidence
		}
		if req.ImplementationCost != nil {
			item.ImplementationCost = *req.ImplementationCost
		}
		if req.Urgency != nil {
			item.Urgency = *req.Urgency
		}
		if req.RecurrenceCount != nil && *req.RecurrenceCount > 0 {
			item.RecurrenceCount = *req.RecurrenceCount
		}
		if req.AIGenerated != nil {
			item.AIGenerated = *req.AIGenerated
		}
		if req.UserEdited != nil {
			item.UserEdited = *req.UserEdited
		}
	}
	if req.CompositeScore != nil && *req.CompositeScore > 0 {
		item.CompositeScore = *req.CompositeScore
	} else {
		item.CompositeScore = buildAutonomousOptimizerBacklogScore(item)
	}
	if err := s.store.AutonomousOptimizer().SaveBacklogItem(item); err != nil {
		SafeInternalError(c, "Failed to save autonomous optimizer backlog item", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func mergeAutonomousOptimizerBacklogUpdate(existing *store.AutonomousOptimizerBacklogItem, req autonomousOptimizerBacklogRequest) *store.AutonomousOptimizerBacklogItem {
	if existing == nil {
		return nil
	}
	updated := *existing
	updated.Title = strings.TrimSpace(req.Title)
	updated.Category = strings.TrimSpace(req.Category)
	updated.Description = strings.TrimSpace(req.Description)
	updated.ExpectedImpact = strings.TrimSpace(req.ExpectedImpact)
	if strings.TrimSpace(req.Status) != "" {
		updated.Status = strings.TrimSpace(req.Status)
	}
	if req.Confidence != nil {
		updated.Confidence = *req.Confidence
	}
	if req.ImplementationCost != nil {
		updated.ImplementationCost = *req.ImplementationCost
	}
	if req.Urgency != nil {
		updated.Urgency = *req.Urgency
	}
	if req.RecurrenceCount != nil && *req.RecurrenceCount > 0 {
		updated.RecurrenceCount = *req.RecurrenceCount
	}
	updated.UserEdited = true
	if req.UserEdited != nil {
		updated.UserEdited = *req.UserEdited || updated.UserEdited
	}
	return &updated
}

type autonomousOptimizerModelOutcomeAgg struct {
	Entry            autonomousOptimizerModelOutcomeEntry
	BacklogWeightSum float64
	Runs             []*store.AutonomousOptimizerRun
}

type autonomousOptimizerApplyResolution struct {
	Status         string
	ObservedNetPnL float64
	Timestamp      time.Time
}

func buildAutonomousOptimizerModelOutcomes(runs []*store.AutonomousOptimizerRun, backlog []*store.AutonomousOptimizerBacklogItem, models []*store.AIModel) []autonomousOptimizerModelOutcomeEntry {
	if len(runs) == 0 && len(backlog) == 0 {
		return []autonomousOptimizerModelOutcomeEntry{}
	}

	modelLabels := make(map[string]string, len(models))
	for _, item := range models {
		if item == nil {
			continue
		}
		label := strings.TrimSpace(item.CustomModelName)
		if label == "" {
			label = strings.TrimSpace(item.Name)
		}
		if label == "" {
			label = strings.TrimSpace(item.Provider)
		}
		if label == "" {
			label = item.ID
		}
		modelLabels[item.ID] = label
	}

	aggs := map[string]*autonomousOptimizerModelOutcomeAgg{}
	runModelKey := map[string]string{}
	runIndex := map[string]*store.AutonomousOptimizerRun{}
	rootResolutions := map[string]autonomousOptimizerApplyResolution{}

	for _, run := range runs {
		if run == nil {
			continue
		}
		runIndex[run.ID] = run
		modelKey, label, primaryLabel, criticLabel := buildAutonomousOptimizerModelKey(run, modelLabels)
		runModelKey[run.ID] = modelKey
		entry := ensureAutonomousOptimizerModelOutcomeAgg(aggs, modelKey, run, label, primaryLabel, criticLabel)
		entry.Entry.TotalRuns++
		entry.Runs = append(entry.Runs, run)
		if entry.Entry.StatusCounts == nil {
			entry.Entry.StatusCounts = map[string]int{}
		}
		entry.Entry.StatusCounts[run.Status]++
		if run.Status == store.AutonomousOptimizerStatusAutoApplied {
			entry.Entry.ApplyCount++
		}
		if run.Status == store.AutonomousOptimizerStatusFailed {
			entry.Entry.FailedCount++
		}
		if run.Status == store.AutonomousOptimizerStatusInsufficientEvidence {
			entry.Entry.InsufficientEvidence++
		}
		if autonomousOptimizerRunRecoveredStale(run) {
			entry.Entry.StaleRecoveryCount++
		}
		if ts := autonomousOptimizerRunEventTime(run); ts.After(entry.Entry.LastUsedAt) {
			entry.Entry.LastUsedAt = ts
		}
	}

	for _, run := range runs {
		if run == nil {
			continue
		}
		rootID := autonomousOptimizerRootApplyID(run)
		if strings.TrimSpace(rootID) == "" || rootID == run.ID {
			continue
		}
		switch run.Status {
		case store.AutonomousOptimizerStatusMonitoring, store.AutonomousOptimizerStatusKept, store.AutonomousOptimizerStatusRolledBack:
		default:
			continue
		}
		current := rootResolutions[rootID]
		next := autonomousOptimizerApplyResolution{
			Status:         run.Status,
			ObservedNetPnL: autonomousOptimizerObservedNetPnL(run),
			Timestamp:      autonomousOptimizerRunEventTime(run),
		}
		if current.Timestamp.IsZero() || next.Timestamp.After(current.Timestamp) {
			rootResolutions[rootID] = next
		}
	}

	for _, run := range runs {
		if run == nil || run.Status != store.AutonomousOptimizerStatusAutoApplied {
			continue
		}
		modelKey := runModelKey[run.ID]
		entry := ensureAutonomousOptimizerModelOutcomeAgg(aggs, modelKey, run, "", "", "")
		resolution, ok := rootResolutions[run.ID]
		if !ok {
			continue
		}
		switch resolution.Status {
		case store.AutonomousOptimizerStatusMonitoring:
			entry.Entry.MonitoringCount++
		case store.AutonomousOptimizerStatusKept:
			entry.Entry.KeptCount++
			if resolution.ObservedNetPnL > 0 {
				entry.Entry.KeptWinCount++
			}
		case store.AutonomousOptimizerStatusRolledBack:
			entry.Entry.RollbackCount++
		}
	}

	for _, item := range backlog {
		if item == nil {
			continue
		}
		runID := strings.TrimSpace(item.RunID)
		if runID == "" {
			continue
		}
		modelKey := runModelKey[runID]
		if modelKey == "" {
			if run := runIndex[runID]; run != nil {
				modelKey, _, _, _ = buildAutonomousOptimizerModelKey(run, modelLabels)
			}
		}
		if modelKey == "" {
			continue
		}
		run := runIndex[runID]
		entry := ensureAutonomousOptimizerModelOutcomeAgg(aggs, modelKey, run, "", "", "")
		entry.Entry.BacklogItemCount++
		switch item.Status {
		case store.AutonomousOptimizerBacklogStatusConfirmed, store.AutonomousOptimizerBacklogStatusPlanned, store.AutonomousOptimizerBacklogStatusInProgress, store.AutonomousOptimizerBacklogStatusDone:
			entry.Entry.UsefulBacklogCount++
		}
		switch item.Status {
		case store.AutonomousOptimizerBacklogStatusDone:
			entry.Entry.DoneBacklogCount++
		case store.AutonomousOptimizerBacklogStatusRejected:
			entry.Entry.RejectedBacklogCount++
		}
		entry.BacklogWeightSum += autonomousOptimizerBacklogUsefulnessWeight(item.Status)
	}

	items := make([]autonomousOptimizerModelOutcomeEntry, 0, len(aggs))
	for _, agg := range aggs {
		if agg == nil {
			continue
		}
		if agg.Entry.TotalRuns > 0 {
			agg.Entry.ApplyRate = (float64(agg.Entry.ApplyCount) / float64(agg.Entry.TotalRuns)) * 100
		}
		if agg.Entry.ApplyCount > 0 {
			agg.Entry.RollbackRate = (float64(agg.Entry.RollbackCount) / float64(agg.Entry.ApplyCount)) * 100
		}
		if agg.Entry.KeptCount > 0 {
			agg.Entry.KeptWinRate = (float64(agg.Entry.KeptWinCount) / float64(agg.Entry.KeptCount)) * 100
		}
		if agg.Entry.BacklogItemCount > 0 {
			agg.Entry.BacklogUsefulness = (agg.BacklogWeightSum / float64(agg.Entry.BacklogItemCount)) * 100
		}
		agg.Entry.FailureEvidenceGaps = autonomousOptimizerFailureEvidenceGapCount(agg.Runs)
		agg.Entry.OperationalHealth = autonomousOptimizerOperationalHealthScore(agg.Entry)
		agg.Entry.OutcomeScore = autonomousOptimizerOutcomeScore(agg.Entry)
		items = append(items, agg.Entry)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].OutcomeScore != items[j].OutcomeScore {
			return items[i].OutcomeScore > items[j].OutcomeScore
		}
		if !items[i].LastUsedAt.Equal(items[j].LastUsedAt) {
			return items[i].LastUsedAt.After(items[j].LastUsedAt)
		}
		return items[i].ModelLabel < items[j].ModelLabel
	})

	return items
}

func ensureAutonomousOptimizerModelOutcomeAgg(index map[string]*autonomousOptimizerModelOutcomeAgg, modelKey string, run *store.AutonomousOptimizerRun, label, primaryLabel, criticLabel string) *autonomousOptimizerModelOutcomeAgg {
	if existing := index[modelKey]; existing != nil {
		if strings.TrimSpace(label) != "" {
			existing.Entry.ModelLabel = label
		}
		if strings.TrimSpace(primaryLabel) != "" {
			existing.Entry.PrimaryModelLabel = primaryLabel
		}
		if strings.TrimSpace(criticLabel) != "" {
			existing.Entry.CriticModelLabel = criticLabel
		}
		return existing
	}
	entry := autonomousOptimizerModelOutcomeEntry{
		ModelKey:     modelKey,
		ModelLabel:   label,
		StatusCounts: map[string]int{},
	}
	if run != nil {
		entry.PrimaryModelConfigID = run.PrimaryModelConfigID
		entry.PrimaryModelName = run.PrimaryModelName
		entry.CriticModelConfigID = run.CriticModelConfigID
		entry.CriticModelName = run.CriticModelName
	}
	if strings.TrimSpace(primaryLabel) != "" {
		entry.PrimaryModelLabel = primaryLabel
	}
	if strings.TrimSpace(criticLabel) != "" {
		entry.CriticModelLabel = criticLabel
	}
	agg := &autonomousOptimizerModelOutcomeAgg{Entry: entry}
	index[modelKey] = agg
	return agg
}

func buildAutonomousOptimizerModelKey(run *store.AutonomousOptimizerRun, modelLabels map[string]string) (string, string, string, string) {
	if run == nil {
		return "unknown", "Unknown / Unknown", "Unknown", "Unknown"
	}
	primaryLabel := autonomousOptimizerModelDisplayLabel(run.PrimaryModelConfigID, run.PrimaryModelName, modelLabels)
	criticLabel := autonomousOptimizerModelDisplayLabel(run.CriticModelConfigID, run.CriticModelName, modelLabels)
	key := strings.Join([]string{
		strings.TrimSpace(run.PrimaryModelConfigID),
		strings.TrimSpace(run.PrimaryModelName),
		strings.TrimSpace(run.CriticModelConfigID),
		strings.TrimSpace(run.CriticModelName),
	}, "|")
	if strings.Trim(strings.ReplaceAll(key, "|", ""), " ") == "" {
		key = strings.ToLower(strings.TrimSpace(primaryLabel + "|" + criticLabel))
	}
	return key, fmt.Sprintf("%s / %s", primaryLabel, criticLabel), primaryLabel, criticLabel
}

func autonomousOptimizerModelDisplayLabel(configID, modelName string, modelLabels map[string]string) string {
	accountLabel := strings.TrimSpace(modelLabels[configID])
	modelName = strings.TrimSpace(modelName)
	switch {
	case accountLabel != "" && modelName != "":
		return fmt.Sprintf("%s (%s)", accountLabel, modelName)
	case accountLabel != "":
		return accountLabel
	case modelName != "":
		return modelName
	case strings.TrimSpace(configID) != "":
		return configID
	default:
		return "Unknown"
	}
}

func autonomousOptimizerRootApplyID(run *store.AutonomousOptimizerRun) string {
	if run == nil {
		return ""
	}
	if run.Status == store.AutonomousOptimizerStatusAutoApplied {
		return run.ID
	}
	metadata := parseAutonomousOptimizerJSONObject(run.MetadataJSON)
	for _, key := range []string{"monitoring_source_run_id", "rollback_source_run_id", "monitoring_root_run_id"} {
		if value := autonomousOptimizerString(metadata[key]); value != "" {
			return value
		}
	}
	rollbackAnalysis := parseAutonomousOptimizerNestedObject(metadata, "rollback_analysis")
	if value := autonomousOptimizerString(rollbackAnalysis["monitoring_root_run_id"]); value != "" {
		return value
	}
	return ""
}

func autonomousOptimizerObservedNetPnL(run *store.AutonomousOptimizerRun) float64 {
	if run == nil {
		return 0
	}
	metadata := parseAutonomousOptimizerJSONObject(run.MetadataJSON)
	rollbackAnalysis := parseAutonomousOptimizerNestedObject(metadata, "rollback_analysis")
	if value, ok := autonomousOptimizerFloat64FromValue(rollbackAnalysis["observed_net_pnl"]); ok {
		return value
	}
	monitoringChain := parseAutonomousOptimizerNestedObject(metadata, "monitoring_chain")
	if value, ok := autonomousOptimizerFloat64FromValue(monitoringChain["total_net_pnl"]); ok {
		return value
	}
	return 0
}

func autonomousOptimizerRunEventTime(run *store.AutonomousOptimizerRun) time.Time {
	if run == nil {
		return time.Time{}
	}
	for _, candidate := range []time.Time{run.CompletedAt.UTC(), run.UpdatedAt.UTC(), run.StartedAt.UTC(), run.CreatedAt.UTC()} {
		if !candidate.IsZero() {
			return candidate
		}
	}
	return time.Time{}
}

func autonomousOptimizerBacklogUsefulnessWeight(status string) float64 {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case store.AutonomousOptimizerBacklogStatusDone:
		return 1.0
	case store.AutonomousOptimizerBacklogStatusInProgress:
		return 0.85
	case store.AutonomousOptimizerBacklogStatusPlanned:
		return 0.7
	case store.AutonomousOptimizerBacklogStatusConfirmed:
		return 0.55
	case store.AutonomousOptimizerBacklogStatusNew:
		return 0.2
	case store.AutonomousOptimizerBacklogStatusRejected:
		return 0
	default:
		return 0.15
	}
}

func autonomousOptimizerRunRecoveredStale(run *store.AutonomousOptimizerRun) bool {
	if run == nil {
		return false
	}
	metadata := parseAutonomousOptimizerJSONObject(run.MetadataJSON)
	return autonomousOptimizerBool(metadata["recovered_stale_run"])
}

func autonomousOptimizerRunCountsAsFailure(run *store.AutonomousOptimizerRun) bool {
	if run == nil {
		return false
	}
	return run.Status == store.AutonomousOptimizerStatusFailed || autonomousOptimizerRunRecoveredStale(run)
}

func autonomousOptimizerFailureEvidenceGapCount(runs []*store.AutonomousOptimizerRun) int {
	if len(runs) == 0 {
		return 0
	}
	ordered := make([]*store.AutonomousOptimizerRun, 0, len(runs))
	for _, run := range runs {
		if run != nil {
			ordered = append(ordered, run)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		left := autonomousOptimizerRunEventTime(ordered[i])
		right := autonomousOptimizerRunEventTime(ordered[j])
		if left.Equal(right) {
			return ordered[i].ID < ordered[j].ID
		}
		return left.Before(right)
	})
	count := 0
	for i, run := range ordered {
		if !autonomousOptimizerRunCountsAsFailure(run) {
			continue
		}
		overlap := false
		if i > 0 && ordered[i-1] != nil && ordered[i-1].Status == store.AutonomousOptimizerStatusInsufficientEvidence {
			overlap = true
		}
		if i+1 < len(ordered) && ordered[i+1] != nil && ordered[i+1].Status == store.AutonomousOptimizerStatusInsufficientEvidence {
			overlap = true
		}
		if overlap {
			count++
		}
	}
	return count
}

func autonomousOptimizerOperationalHealthScore(entry autonomousOptimizerModelOutcomeEntry) float64 {
	if entry.TotalRuns <= 0 {
		return 0
	}
	totalRuns := float64(entry.TotalRuns)
	failedRate := float64(entry.FailedCount) / totalRuns
	staleRate := float64(entry.StaleRecoveryCount) / totalRuns
	insufficientRate := float64(entry.InsufficientEvidence) / totalRuns
	overlapRate := 0.0
	if entry.FailedCount > 0 {
		overlapRate = float64(entry.FailureEvidenceGaps) / float64(entry.FailedCount)
	}
	score := 100.0 -
		(failedRate * 45.0) -
		(staleRate * 25.0) -
		(insufficientRate * 15.0) -
		(overlapRate * 15.0)
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func autonomousOptimizerOutcomeScore(entry autonomousOptimizerModelOutcomeEntry) float64 {
	applySignal := entry.ApplyRate
	if entry.ApplyCount == 0 && entry.TotalRuns > 0 {
		applySignal = 25
	}
	safetySignal := 100 - entry.RollbackRate
	if entry.ApplyCount == 0 {
		safetySignal = 50
	}
	keptSignal := entry.KeptWinRate
	if entry.KeptCount == 0 {
		keptSignal = 40
	}
	score := (applySignal * 0.20) + (safetySignal * 0.35) + (keptSignal * 0.30) + (entry.BacklogUsefulness * 0.15)
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func (s *Server) handleTraderAutonomousOptimizerBootstrap(c *gin.Context) {
	userID := c.GetString("user_id")
	targetTraderID := c.Param("id")

	targetTrader, err := s.store.Trader().Get(userID, targetTraderID)
	if err != nil {
		SafeNotFound(c, "Target trader")
		return
	}

	var req autonomousOptimizerBootstrapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid autonomous optimizer bootstrap payload")
		return
	}
	sourceTraderID := strings.TrimSpace(req.SourceTraderID)
	if sourceTraderID == "" {
		SafeBadRequest(c, "source_trader_id is required")
		return
	}
	if sourceTraderID == targetTraderID {
		SafeBadRequest(c, "source_trader_id must be different from the target trader")
		return
	}

	sourceTrader, err := s.store.Trader().Get(userID, sourceTraderID)
	if err != nil {
		SafeNotFound(c, "Source trader")
		return
	}

	targetStrategyCfg, targetStrategyRecord, err := s.loadStrategyForTrader(userID, targetTrader)
	if err != nil {
		SafeInternalError(c, "Failed to load target strategy", err)
		return
	}
	_ = targetStrategyCfg

	_, sourceStrategyRecord, err := s.loadStrategyForTrader(userID, sourceTrader)
	if err != nil {
		SafeInternalError(c, "Failed to load source strategy", err)
		return
	}

	cfg, err := s.store.AutonomousOptimizer().GetOrCreateConfig(userID, targetTraderID)
	if err != nil {
		SafeInternalError(c, "Failed to load autonomous optimizer config", err)
		return
	}
	if err := s.ensureAutonomousOptimizerModelDefaults(userID, cfg); err != nil {
		SafeInternalError(c, "Failed to resolve optimizer model defaults", err)
		return
	}

	baselineTraderJSON, err := json.Marshal(map[string]any{
		"id":                     targetTrader.ID,
		"name":                   targetTrader.Name,
		"strategy_id":            targetTrader.StrategyID,
		"custom_prompt":          targetTrader.CustomPrompt,
		"override_base_prompt":   targetTrader.OverrideBasePrompt,
		"system_prompt_template": targetTrader.SystemPromptTemplate,
		"scan_interval_minutes":  targetTrader.ScanIntervalMinutes,
		"invert_signals":         targetTrader.InvertSignals,
		"is_cross_margin":        targetTrader.IsCrossMargin,
	})
	if err != nil {
		SafeInternalError(c, "Failed to capture target trader baseline", err)
		return
	}

	newStrategyID := uuid.NewString()
	seededStrategyName := fmt.Sprintf("%s Autonomous Seed %s", targetTrader.Name, time.Now().UTC().Format("2006-01-02 15:04"))
	if err := s.store.Strategy().Duplicate(userID, sourceStrategyRecord.ID, newStrategyID, seededStrategyName); err != nil {
		SafeInternalError(c, "Failed to duplicate source strategy for bootstrap", err)
		return
	}
	seededStrategy, err := s.store.Strategy().Get(userID, newStrategyID)
	if err != nil {
		SafeInternalError(c, "Failed to load seeded strategy", err)
		return
	}
	seededStrategy.Description = fmt.Sprintf("Autonomous optimizer seed for %s based on [%s]", targetTrader.Name, sourceTrader.Name)
	if err := s.store.Strategy().Update(seededStrategy); err != nil {
		SafeInternalError(c, "Failed to annotate seeded strategy", err)
		return
	}

	wasRunning := false
	if existingMemTrader, memErr := s.traderManager.GetTrader(targetTraderID); memErr == nil {
		status := existingMemTrader.GetStatus()
		if running, ok := status["is_running"].(bool); ok && running {
			wasRunning = true
		}
	}

	targetTrader.StrategyID = seededStrategy.ID
	targetTrader.CustomPrompt = sourceTrader.CustomPrompt
	targetTrader.OverrideBasePrompt = sourceTrader.OverrideBasePrompt
	targetTrader.SystemPromptTemplate = sourceTrader.SystemPromptTemplate
	if sourceTrader.ScanIntervalMinutes > 0 {
		targetTrader.ScanIntervalMinutes = sourceTrader.ScanIntervalMinutes
	}
	targetTrader.InvertSignals = sourceTrader.InvertSignals
	if err := s.store.Trader().Update(targetTrader); err != nil {
		SafeInternalError(c, "Failed to update target trader with seeded strategy", err)
		return
	}

	appliedAt := time.Now().UTC()
	version := &store.DealReviewStrategyVersion{
		UserID:             userID,
		TraderID:           targetTraderID,
		StrategyID:         seededStrategy.ID,
		SourceType:         "autonomous_optimizer_seed",
		Summary:            fmt.Sprintf("Seeded %s from %s for autonomous optimization", targetTrader.Name, sourceTrader.Name),
		ExpectedEffect:     "Bootstrap the low-activity trader from a stronger incumbent base before autonomous review begins.",
		PreviousConfigJSON: targetStrategyRecord.Config,
		NextConfigJSON:     seededStrategy.Config,
		AppliedAt:          appliedAt,
	}
	if err := s.store.DealReview().SaveStrategyVersion(version); err != nil {
		SafeInternalError(c, "Failed to record autonomous optimizer seed version", err)
		return
	}

	cfg.Enabled = true
	cfg.Status = store.AutonomousOptimizerStatusScheduled
	cfg.SeedSourceTraderID = sourceTrader.ID
	cfg.SeedSourceStrategyID = sourceStrategyRecord.ID
	cfg.BaselineStrategyID = targetStrategyRecord.ID
	cfg.BaselineStrategyJSON = targetStrategyRecord.Config
	cfg.BaselineTraderJSON = string(baselineTraderJSON)
	cfg.CurrentSeedStrategyID = seededStrategy.ID
	cfg.SeededAt = appliedAt
	cfg.LastRunAt = appliedAt
	cfg.NextRunAt = appliedAt.Add(time.Duration(cfg.ReviewIntervalHours) * time.Hour)

	run := &store.AutonomousOptimizerRun{
		UserID:                   userID,
		TraderID:                 targetTraderID,
		ConfigID:                 cfg.ID,
		Trigger:                  store.AutonomousOptimizerRunTriggerBootstrap,
		Status:                   store.AutonomousOptimizerStatusAutoApplied,
		Summary:                  fmt.Sprintf("Bootstrap applied: %s now seeded from %s.", targetTrader.Name, sourceTrader.Name),
		PrimaryModelConfigID:     cfg.PrimaryModelConfigID,
		PrimaryModelName:         cfg.PrimaryModelName,
		CriticModelConfigID:      cfg.CriticModelConfigID,
		CriticModelName:          cfg.CriticModelName,
		SourceTraderID:           sourceTrader.ID,
		SourceStrategyID:         sourceStrategyRecord.ID,
		AppliedStrategyVersionID: version.ID,
		MetadataJSON:             fmt.Sprintf(`{"target_trader_id":%q,"seed_strategy_id":%q}`, targetTrader.ID, seededStrategy.ID),
		StartedAt:                appliedAt,
		CompletedAt:              appliedAt,
	}
	if err := s.store.AutonomousOptimizer().SaveRun(run); err != nil {
		SafeInternalError(c, "Failed to record autonomous optimizer bootstrap run", err)
		return
	}
	cfg.LastRunID = run.ID
	if err := s.store.AutonomousOptimizer().SaveConfig(cfg); err != nil {
		SafeInternalError(c, "Failed to save autonomous optimizer config", err)
		return
	}

	if err := s.reloadTraderAfterStrategyChange(userID, targetTraderID, wasRunning, "autonomous optimizer bootstrap"); err != nil {
		SafeInternalError(c, "Failed to reload target trader after bootstrap", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":               "Autonomous optimizer bootstrap applied",
		"target_trader_id":      targetTrader.ID,
		"target_trader_name":    targetTrader.Name,
		"source_trader_id":      sourceTrader.ID,
		"source_trader_name":    sourceTrader.Name,
		"seed_strategy_id":      seededStrategy.ID,
		"optimizer_config_id":   cfg.ID,
		"optimizer_run_id":      run.ID,
		"strategy_version_id":   version.ID,
		"next_review_at":        cfg.NextRunAt,
		"default_primary_model": cfg.PrimaryModelName,
		"default_critic_model":  cfg.CriticModelName,
	})
}

func (s *Server) ensureAutonomousOptimizerModelDefaults(userID string, cfg *store.AutonomousOptimizerConfig) error {
	if cfg == nil {
		return fmt.Errorf("autonomous optimizer config cannot be nil")
	}
	primaryCfg, criticCfg, err := s.resolveAutonomousOptimizerDefaultModels(userID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if strings.TrimSpace(cfg.PrimaryModelConfigID) == "" && primaryCfg != nil {
		cfg.PrimaryModelConfigID = primaryCfg.ID
	}
	if strings.TrimSpace(cfg.CriticModelConfigID) == "" && criticCfg != nil {
		cfg.CriticModelConfigID = criticCfg.ID
	}
	if strings.TrimSpace(cfg.PrimaryModelName) == "" {
		cfg.PrimaryModelName = store.AutonomousOptimizerDefaultModelName
	}
	if strings.TrimSpace(cfg.CriticModelName) == "" {
		cfg.CriticModelName = store.AutonomousOptimizerDefaultModelName
	}
	return s.store.AutonomousOptimizer().SaveConfig(cfg)
}

func (s *Server) resolveAutonomousOptimizerDefaultModels(userID string) (*store.AIModel, *store.AIModel, error) {
	models, err := s.store.AIModel().List(userID)
	if err != nil {
		return nil, nil, err
	}
	var preferred []*store.AIModel
	var fallback []*store.AIModel
	for _, item := range models {
		if item == nil || !item.Enabled || !modelHasUsableCredentials(item) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Provider), "openai") {
			preferred = append(preferred, item)
		} else {
			fallback = append(fallback, item)
		}
	}
	if len(preferred) == 0 {
		preferred = fallback
	}
	if len(preferred) == 0 {
		return nil, nil, gorm.ErrRecordNotFound
	}
	primary := preferred[0]
	critic := preferred[0]
	if len(preferred) > 1 {
		critic = preferred[1]
	}
	return primary, critic, nil
}

func buildAutonomousOptimizerBacklogScore(item *store.AutonomousOptimizerBacklogItem) float64 {
	if item == nil {
		return 0
	}
	recurrence := float64(item.RecurrenceCount)
	if recurrence <= 0 {
		recurrence = 1
	}
	score := (item.Confidence * 0.35) + (item.Urgency * 0.25) + (recurrence * 5)
	if item.ExpectedImpact != "" {
		score += 10
	}
	if item.ImplementationCost > 0 {
		score -= item.ImplementationCost * 0.2
	}
	if score < 0 {
		return 0
	}
	return score
}

func autonomousOptimizerStringSliceFromValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if str, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(str); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func autonomousOptimizerInt64FromValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case float32:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func autonomousOptimizerFloat64FromValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
