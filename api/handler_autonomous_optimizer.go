package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nofx/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type autonomousOptimizerConfigRequest struct {
	Enabled              *bool  `json:"enabled"`
	ReviewIntervalHours  *int   `json:"review_interval_hours"`
	AutoApplyConfigPatch *bool  `json:"auto_apply_config_patch"`
	AutoApplyPromptPatch *bool  `json:"auto_apply_prompt_patch"`
	AutoRollbackEnabled  *bool  `json:"auto_rollback_enabled"`
	SelfPauseEnabled     *bool  `json:"self_pause_enabled"`
	Status               string `json:"status"`
	PrimaryModelConfigID string `json:"primary_model_config_id"`
	PrimaryModelName     string `json:"primary_model_name"`
	CriticModelConfigID  string `json:"critic_model_config_id"`
	CriticModelName      string `json:"critic_model_name"`
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

	item := &store.AutonomousOptimizerBacklogItem{
		ID:             strings.TrimSpace(req.ID),
		UserID:         userID,
		TraderID:       traderID,
		Title:          strings.TrimSpace(req.Title),
		Category:       strings.TrimSpace(req.Category),
		Description:    strings.TrimSpace(req.Description),
		ExpectedImpact: strings.TrimSpace(req.ExpectedImpact),
		Status:         strings.TrimSpace(req.Status),
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
	if req.CompositeScore != nil {
		item.CompositeScore = *req.CompositeScore
	}
	if req.AIGenerated != nil {
		item.AIGenerated = *req.AIGenerated
	} else {
		item.AIGenerated = true
	}
	if req.UserEdited != nil {
		item.UserEdited = *req.UserEdited
	}
	if item.CompositeScore == 0 {
		item.CompositeScore = buildAutonomousOptimizerBacklogScore(item)
	}
	if err := s.store.AutonomousOptimizer().SaveBacklogItem(item); err != nil {
		SafeInternalError(c, "Failed to save autonomous optimizer backlog item", err)
		return
	}
	c.JSON(http.StatusOK, item)
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
		if item == nil || !item.Enabled || strings.TrimSpace(string(item.APIKey)) == "" {
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
