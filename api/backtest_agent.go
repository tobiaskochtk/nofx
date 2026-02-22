package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"nofx/backtest"
	"nofx/logger"
	"nofx/market"
	"nofx/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	defaultBacktestLookbackDays = 90
	defaultAgentWaitTimeoutSecs = 120
	maxAgentWaitTimeoutSecs     = 900
)

type backtestCreateRequest struct {
	Config            backtest.BacktestConfig `json:"config" binding:"required"`
	Strategy          agentStrategyInput      `json:"strategy"`
	WaitForCompletion bool                    `json:"wait_for_completion"`
	TimeoutSeconds    int                     `json:"timeout_seconds"`
}

type agentStrategyInput struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Description     string                `json:"description"`
	NaturalLanguage string                `json:"natural_language"`
	Save            *bool                 `json:"save"`
	Config          *store.StrategyConfig `json:"config"`
}

type backtestImproveRequest struct {
	Goal              string `json:"goal"`
	MaxSuggestions    int    `json:"max_suggestions"`
	AutoApply         bool   `json:"auto_apply"`
	SaveAsStrategy    *bool  `json:"save_as_strategy"`
	StrategyName      string `json:"strategy_name"`
	Rerun             bool   `json:"rerun"`
	WaitForCompletion bool   `json:"wait_for_completion"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
}

type strategyImprovementSuggestion struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Reason         string         `json:"reason"`
	ExpectedImpact string         `json:"expected_impact"`
	Patch          map[string]any `json:"patch,omitempty"`
}

type generatedImprovement struct {
	Suggestion strategyImprovementSuggestion
	Apply      func(strategyCfg *store.StrategyConfig, runCfg *backtest.BacktestConfig)
}

func (s *Server) handleBacktestCreate(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}

	var req backtestCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	userID := normalizeUserID(c.GetString("user_id"))
	cfg := req.Config
	cfg.UserID = userID

	strategyID, strategyName, strategyCfg, err := s.prepareAgentStrategyForBacktest(userID, &req.Strategy, &cfg)
	if err != nil {
		SafeBadRequest(c, err.Error())
		return
	}
	if strategyCfg != nil {
		cfg.SetLoadedStrategy(strategyCfg)
	}

	if err := s.prepareBacktestConfig(&cfg, userID); err != nil {
		SafeBadRequest(c, err.Error())
		return
	}

	runner, err := s.backtestManager.Start(context.Background(), cfg)
	if err != nil {
		SafeError(c, http.StatusBadRequest, "Failed to start backtest", err)
		return
	}

	meta := runner.CurrentMetadata()
	resp := gin.H{
		"run_id":        cfg.RunID,
		"strategy_id":   strategyID,
		"strategy_name": strategyName,
		"metadata":      meta,
	}

	if !req.WaitForCompletion {
		c.JSON(http.StatusCreated, resp)
		return
	}

	timeout := normalizeAgentTimeout(req.TimeoutSeconds)
	finalMeta, waitErr := s.waitForBacktestCompletion(cfg.RunID, timeout)
	if finalMeta == nil {
		finalMeta = meta
	}

	result, err := s.loadBacktestResultPayload(cfg.RunID, finalMeta, map[string]bool{}, 0, 0, 0)
	if err != nil {
		SafeInternalError(c, "Load backtest result", err)
		return
	}
	resp["result"] = result

	if errors.Is(waitErr, context.DeadlineExceeded) {
		resp["timed_out"] = true
		resp["timeout_seconds"] = int(timeout.Seconds())
		c.JSON(http.StatusAccepted, resp)
		return
	}
	if waitErr != nil {
		SafeInternalError(c, "Wait backtest completion", waitErr)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleBacktestResult(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}

	userID := normalizeUserID(c.GetString("user_id"))
	runID := strings.TrimSpace(c.Param("id"))
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	meta, err := s.ensureBacktestRunOwnership(runID, userID)
	if writeBacktestAccessError(c, err) {
		return
	}

	include := parseIncludeSet(c.Query("include"))
	equityLimit := queryInt(c, "equity_limit", 500)
	tradeLimit := queryInt(c, "trade_limit", 500)
	decisionLimit := queryInt(c, "decision_limit", 50)

	payload, err := s.loadBacktestResultPayload(runID, meta, include, equityLimit, tradeLimit, decisionLimit)
	if err != nil {
		SafeInternalError(c, "Load backtest results", err)
		return
	}

	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleBacktestImprove(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}

	userID := normalizeUserID(c.GetString("user_id"))
	runID := strings.TrimSpace(c.Param("id"))
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	meta, err := s.ensureBacktestRunOwnership(runID, userID)
	if writeBacktestAccessError(c, err) {
		return
	}

	var req backtestImproveRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			SafeBadRequest(c, "Invalid request parameters")
			return
		}
	}

	if req.Rerun && !req.AutoApply {
		SafeBadRequest(c, "rerun requires auto_apply=true")
		return
	}

	runCfg, err := backtest.LoadConfig(runID)
	if err != nil {
		SafeNotFound(c, "Backtest config")
		return
	}

	baseStrategyCfg, baseStrategyID, baseStrategyName, err := s.loadStrategyConfigForRun(userID, runCfg)
	if err != nil {
		SafeInternalError(c, "Load strategy config for backtest", err)
		return
	}

	var metrics *backtest.Metrics
	metrics, err = s.backtestManager.GetMetrics(runID)
	if err != nil {
		if !(errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist)) {
			SafeInternalError(c, "Load backtest metrics", err)
			return
		}
		metrics = nil
	}

	improvements := generateImprovementPlans(metrics, req.Goal, *baseStrategyCfg, *runCfg)
	maxSuggestions := req.MaxSuggestions
	if maxSuggestions <= 0 {
		maxSuggestions = 3
	}
	if maxSuggestions > 10 {
		maxSuggestions = 10
	}
	if len(improvements) > maxSuggestions {
		improvements = improvements[:maxSuggestions]
	}

	suggestions := make([]strategyImprovementSuggestion, 0, len(improvements))
	for _, item := range improvements {
		suggestions = append(suggestions, item.Suggestion)
	}

	resp := gin.H{
		"run_id":             runID,
		"goal":               strings.TrimSpace(req.Goal),
		"base_strategy_id":   baseStrategyID,
		"base_strategy_name": baseStrategyName,
		"metadata":           meta,
		"metrics":            metrics,
		"suggestions":        suggestions,
	}

	if !req.AutoApply {
		c.JSON(http.StatusOK, resp)
		return
	}

	improvedStrategy := *baseStrategyCfg
	improvedRunCfg := *runCfg
	applied := make([]strategyImprovementSuggestion, 0, len(improvements))
	for _, item := range improvements {
		if item.Apply != nil {
			item.Apply(&improvedStrategy, &improvedRunCfg)
		}
		applied = append(applied, item.Suggestion)
	}

	resp["auto_applied"] = true
	resp["applied_suggestions"] = applied
	resp["improved_config"] = improvedStrategy

	saveAsStrategy := true
	if req.SaveAsStrategy != nil {
		saveAsStrategy = *req.SaveAsStrategy
	}

	var savedStrategy *store.Strategy
	if saveAsStrategy {
		name := strings.TrimSpace(req.StrategyName)
		if name == "" {
			name = fmt.Sprintf("Agent Improved %s", time.Now().UTC().Format("2006-01-02 15:04 UTC"))
		}
		description := strings.TrimSpace(req.Goal)
		if description == "" {
			description = "Auto-improved from backtest " + runID
		}
		savedStrategy, err = s.persistStrategyConfig(userID, name, description, &improvedStrategy)
		if err != nil {
			SafeInternalError(c, "Save improved strategy", err)
			return
		}
		improvedRunCfg.StrategyID = savedStrategy.ID
		resp["saved_strategy"] = gin.H{
			"id":          savedStrategy.ID,
			"name":        savedStrategy.Name,
			"description": savedStrategy.Description,
		}
	} else {
		improvedRunCfg.StrategyID = ""
		improvedRunCfg.SetLoadedStrategy(&improvedStrategy)
	}

	if err := s.applyStrategyToBacktestConfig(&improvedRunCfg, &improvedStrategy); err != nil && len(improvedRunCfg.Symbols) == 0 {
		SafeBadRequest(c, "failed to apply improved strategy: "+err.Error())
		return
	}

	if !req.Rerun {
		c.JSON(http.StatusOK, resp)
		return
	}

	improvedRunCfg.RunID = generateBacktestRunID()
	improvedRunCfg.UserID = userID
	if !saveAsStrategy {
		improvedRunCfg.SetLoadedStrategy(&improvedStrategy)
	}

	if err := s.prepareBacktestConfig(&improvedRunCfg, userID); err != nil {
		SafeBadRequest(c, err.Error())
		return
	}

	runner, err := s.backtestManager.Start(context.Background(), improvedRunCfg)
	if err != nil {
		SafeError(c, http.StatusBadRequest, "Failed to start improved backtest", err)
		return
	}

	rerunMeta := runner.CurrentMetadata()
	rerunResp := gin.H{
		"run_id":   improvedRunCfg.RunID,
		"metadata": rerunMeta,
	}
	resp["rerun"] = rerunResp

	if req.WaitForCompletion {
		timeout := normalizeAgentTimeout(req.TimeoutSeconds)
		finalMeta, waitErr := s.waitForBacktestCompletion(improvedRunCfg.RunID, timeout)
		if finalMeta == nil {
			finalMeta = rerunMeta
		}

		result, err := s.loadBacktestResultPayload(improvedRunCfg.RunID, finalMeta, map[string]bool{}, 0, 0, 0)
		if err != nil {
			SafeInternalError(c, "Load improved backtest result", err)
			return
		}
		rerunResp["result"] = result

		if errors.Is(waitErr, context.DeadlineExceeded) {
			rerunResp["timed_out"] = true
			rerunResp["timeout_seconds"] = int(timeout.Seconds())
			c.JSON(http.StatusAccepted, resp)
			return
		}
		if waitErr != nil {
			SafeInternalError(c, "Wait improved backtest completion", waitErr)
			return
		}
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) prepareBacktestConfig(cfg *backtest.BacktestConfig, userID string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	cfg.UserID = normalizeUserID(userID)
	cfg.RunID = strings.TrimSpace(cfg.RunID)
	if cfg.RunID == "" {
		cfg.RunID = generateBacktestRunID()
	}
	cfg.CustomPrompt = strings.TrimSpace(cfg.CustomPrompt)

	if cfg.EndTS <= 0 {
		cfg.EndTS = time.Now().UTC().Unix()
	}
	if cfg.StartTS <= 0 {
		cfg.StartTS = time.Unix(cfg.EndTS, 0).AddDate(0, 0, -defaultBacktestLookbackDays).Unix()
	}
	if cfg.EndTS <= cfg.StartTS {
		return fmt.Errorf("invalid start_ts/end_ts")
	}

	if cfg.StrategyID != "" {
		if s.store == nil {
			return fmt.Errorf("strategy store unavailable")
		}

		strategy, err := s.store.Strategy().Get(cfg.UserID, strings.TrimSpace(cfg.StrategyID))
		if err != nil {
			return fmt.Errorf("failed to load strategy: %w", err)
		}
		if strategy == nil {
			return fmt.Errorf("strategy not found")
		}

		var strategyConfig store.StrategyConfig
		if err := json.Unmarshal([]byte(strategy.Config), &strategyConfig); err != nil {
			return fmt.Errorf("failed to parse strategy config: %w", err)
		}

		cfg.SetLoadedStrategy(&strategyConfig)
		if err := s.applyStrategyToBacktestConfig(cfg, &strategyConfig); err != nil && len(cfg.Symbols) == 0 {
			return fmt.Errorf("failed to resolve strategy symbols: %w", err)
		}
	}

	if len(cfg.Timeframes) == 0 {
		cfg.Timeframes = strategyTimeframesFromConfig(cfg.ToStrategyConfig())
	}
	if cfg.DecisionTimeframe == "" {
		if len(cfg.Timeframes) > 0 {
			cfg.DecisionTimeframe = cfg.Timeframes[0]
		} else {
			cfg.DecisionTimeframe = "15m"
		}
	}

	if err := s.hydrateBacktestAIConfig(cfg); err != nil {
		return fmt.Errorf("failed to configure AI model: %w", err)
	}

	logger.Infof("📊 Prepared backtest config: run_id=%s symbols=%v strategy_id=%s start=%d end=%d",
		cfg.RunID, cfg.Symbols, cfg.StrategyID, cfg.StartTS, cfg.EndTS)
	return nil
}

func (s *Server) prepareAgentStrategyForBacktest(userID string, input *agentStrategyInput, cfg *backtest.BacktestConfig) (string, string, *store.StrategyConfig, error) {
	if cfg == nil {
		return "", "", nil, fmt.Errorf("config is nil")
	}
	if input == nil {
		return "", "", nil, nil
	}

	strategyID := strings.TrimSpace(input.ID)
	if strategyID != "" {
		cfg.StrategyID = strategyID
		return strategyID, "", nil, nil
	}

	desc := strings.TrimSpace(input.NaturalLanguage)
	if input.Config == nil && desc == "" {
		return "", "", nil, nil
	}

	var strategyCfg store.StrategyConfig
	if input.Config != nil {
		strategyCfg = *input.Config
	} else {
		strategyCfg = inferStrategyConfigFromDescription(desc)
	}

	if err := s.applyStrategyToBacktestConfig(cfg, &strategyCfg); err != nil && len(cfg.Symbols) == 0 {
		return "", "", nil, fmt.Errorf("failed to apply strategy: %w", err)
	}

	save := true
	if input.Save != nil {
		save = *input.Save
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = fmt.Sprintf("Agent Strategy %s", time.Now().UTC().Format("2006-01-02 15:04 UTC"))
	}

	if !save {
		return "", name, &strategyCfg, nil
	}

	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = desc
	}
	if description == "" {
		description = "Agent-created strategy"
	}

	strategy, err := s.persistStrategyConfig(userID, name, description, &strategyCfg)
	if err != nil {
		return "", "", nil, err
	}

	cfg.StrategyID = strategy.ID
	return strategy.ID, strategy.Name, &strategyCfg, nil
}

func (s *Server) persistStrategyConfig(userID, name, description string, strategyCfg *store.StrategyConfig) (*store.Strategy, error) {
	if strategyCfg == nil {
		return nil, fmt.Errorf("strategy config is nil")
	}
	if s.store == nil {
		return nil, fmt.Errorf("strategy store unavailable")
	}

	configJSON, err := json.Marshal(strategyCfg)
	if err != nil {
		return nil, fmt.Errorf("serialize strategy config: %w", err)
	}

	strategy := &store.Strategy{
		ID:          uuid.New().String(),
		UserID:      normalizeUserID(userID),
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		IsActive:    false,
		IsDefault:   false,
		Config:      string(configJSON),
	}
	if strategy.Name == "" {
		strategy.Name = "Agent Strategy"
	}

	if err := s.store.Strategy().Create(strategy); err != nil {
		return nil, fmt.Errorf("create strategy: %w", err)
	}
	return strategy, nil
}

func (s *Server) applyStrategyToBacktestConfig(cfg *backtest.BacktestConfig, strategyCfg *store.StrategyConfig) error {
	if cfg == nil || strategyCfg == nil {
		return nil
	}

	cfg.SetLoadedStrategy(strategyCfg)

	symbols, err := s.resolveStrategyCoins(strategyCfg)
	if err != nil {
		fallback := normalizeSymbols(strategyCfg.CoinSource.StaticCoins)
		if len(fallback) > 0 {
			cfg.Symbols = fallback
		}
	} else if len(symbols) > 0 {
		cfg.Symbols = symbols
	}

	timeframes := strategyTimeframesFromConfig(strategyCfg)
	if len(timeframes) > 0 {
		cfg.Timeframes = timeframes
		if cfg.DecisionTimeframe == "" {
			cfg.DecisionTimeframe = timeframes[0]
		}
	}

	if leverage := strategyCfg.RiskControl.BTCETHMaxLeverage; leverage > 0 {
		cfg.Leverage.BTCETHLeverage = leverage
	}
	if leverage := strategyCfg.RiskControl.AltcoinMaxLeverage; leverage > 0 {
		cfg.Leverage.AltcoinLeverage = leverage
	}

	customPrompt := strings.TrimSpace(strategyCfg.CustomPrompt)
	if customPrompt != "" {
		cfg.CustomPrompt = customPrompt
	}

	return err
}

func strategyTimeframesFromConfig(strategyCfg *store.StrategyConfig) []string {
	if strategyCfg == nil {
		return nil
	}

	tfSet := make(map[string]bool)
	out := make([]string, 0, 4)

	appendTF := func(tf string) {
		if tf == "" {
			return
		}
		norm, err := market.NormalizeTimeframe(tf)
		if err != nil {
			return
		}
		if !tfSet[norm] {
			tfSet[norm] = true
			out = append(out, norm)
		}
	}

	appendTF(strategyCfg.Indicators.Klines.PrimaryTimeframe)
	for _, tf := range strategyCfg.Indicators.Klines.SelectedTimeframes {
		appendTF(tf)
	}
	appendTF(strategyCfg.Indicators.Klines.LongerTimeframe)

	if len(out) == 0 {
		out = []string{"5m", "15m", "1h"}
	}

	return out
}

func (s *Server) loadBacktestResultPayload(runID string, meta *backtest.RunMetadata, include map[string]bool, equityLimit, tradeLimit, decisionLimit int) (gin.H, error) {
	payload := gin.H{
		"run_id":   runID,
		"metadata": meta,
	}

	if status := s.backtestManager.Status(runID); status != nil {
		payload["status"] = status
	} else if meta != nil {
		payload["status"] = statusPayloadFromMetadata(meta)
	}

	metrics, err := s.backtestManager.GetMetrics(runID)
	if err == nil {
		payload["metrics"] = metrics
		payload["metrics_ready"] = true
	} else if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
		payload["metrics"] = nil
		payload["metrics_ready"] = false
	} else {
		return nil, fmt.Errorf("load backtest metrics: %w", err)
	}

	if include["equity"] {
		if equityLimit <= 0 {
			equityLimit = 500
		}
		equity, err := s.backtestManager.LoadEquity(runID, "", equityLimit)
		if err != nil {
			return nil, fmt.Errorf("load equity: %w", err)
		}
		payload["equity"] = equity
	}

	if include["trades"] {
		if tradeLimit <= 0 {
			tradeLimit = 500
		}
		trades, err := s.backtestManager.LoadTrades(runID, tradeLimit)
		if err != nil {
			return nil, fmt.Errorf("load trades: %w", err)
		}
		payload["trades"] = trades
	}

	if include["decisions"] {
		if decisionLimit <= 0 {
			decisionLimit = 50
		}
		decisions, err := backtest.LoadDecisionRecords(runID, decisionLimit, 0)
		if err != nil {
			return nil, fmt.Errorf("load decisions: %w", err)
		}
		payload["decisions"] = decisions
	}

	return payload, nil
}

func statusPayloadFromMetadata(meta *backtest.RunMetadata) backtest.StatusPayload {
	if meta == nil {
		return backtest.StatusPayload{}
	}
	return backtest.StatusPayload{
		RunID:          meta.RunID,
		State:          meta.State,
		ProgressPct:    meta.Summary.ProgressPct,
		ProcessedBars:  meta.Summary.ProcessedBars,
		CurrentTime:    0,
		DecisionCycle:  meta.Summary.ProcessedBars,
		Equity:         meta.Summary.EquityLast,
		UnrealizedPnL:  0,
		RealizedPnL:    0,
		Note:           meta.Summary.LiquidationNote,
		LastError:      meta.LastError,
		LastUpdatedIso: meta.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func parseIncludeSet(raw string) map[string]bool {
	set := make(map[string]bool)
	for _, token := range strings.Split(strings.TrimSpace(raw), ",") {
		key := strings.ToLower(strings.TrimSpace(token))
		if key == "" {
			continue
		}
		if key == "all" {
			set["equity"] = true
			set["trades"] = true
			set["decisions"] = true
			continue
		}
		set[key] = true
	}
	return set
}

func (s *Server) waitForBacktestCompletion(runID string, timeout time.Duration) (*backtest.RunMetadata, error) {
	if timeout <= 0 {
		timeout = normalizeAgentTimeout(0)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastMeta *backtest.RunMetadata
	for {
		meta, err := s.backtestManager.LoadMetadata(runID)
		if err == nil {
			lastMeta = meta
			if isTerminalBacktestState(meta.State) {
				return meta, nil
			}
		}

		if time.Now().After(deadline) {
			if lastMeta != nil {
				return lastMeta, context.DeadlineExceeded
			}
			return nil, context.DeadlineExceeded
		}

		<-ticker.C
	}
}

func isTerminalBacktestState(state backtest.RunState) bool {
	switch state {
	case backtest.RunStateCompleted, backtest.RunStateFailed, backtest.RunStateStopped, backtest.RunStateLiquidated:
		return true
	default:
		return false
	}
}

func normalizeAgentTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = defaultAgentWaitTimeoutSecs
	}
	if seconds > maxAgentWaitTimeoutSecs {
		seconds = maxAgentWaitTimeoutSecs
	}
	return time.Duration(seconds) * time.Second
}

func generateBacktestRunID() string {
	return fmt.Sprintf("bt_%s_%s", time.Now().UTC().Format("20060102_150405"), uuid.NewString()[:8])
}

func (s *Server) loadStrategyConfigForRun(userID string, runCfg *backtest.BacktestConfig) (*store.StrategyConfig, string, string, error) {
	if runCfg == nil {
		return nil, "", "", fmt.Errorf("run config is nil")
	}

	strategyID := strings.TrimSpace(runCfg.StrategyID)
	if strategyID != "" && s.store != nil {
		st, err := s.store.Strategy().Get(userID, strategyID)
		if err == nil && st != nil {
			cfg, err := st.ParseConfig()
			if err != nil {
				return nil, "", "", fmt.Errorf("parse strategy config: %w", err)
			}
			return cfg, st.ID, st.Name, nil
		}
		logger.Infof("⚠️ unable to load strategy %s for run %s, using stored run config fallback", strategyID, runCfg.RunID)
	}

	fallback := runCfg.ToStrategyConfig()
	if fallback == nil {
		return nil, strategyID, "", fmt.Errorf("unable to derive strategy config from run")
	}
	return fallback, strategyID, "", nil
}

func generateImprovementPlans(metrics *backtest.Metrics, goal string, strategyCfg store.StrategyConfig, runCfg backtest.BacktestConfig) []generatedImprovement {
	goalLower := strings.ToLower(strings.TrimSpace(goal))
	items := make([]generatedImprovement, 0, 6)
	seen := make(map[string]bool)

	add := func(item generatedImprovement) {
		if item.Suggestion.ID == "" || seen[item.Suggestion.ID] {
			return
		}
		seen[item.Suggestion.ID] = true
		items = append(items, item)
	}

	if metrics != nil && metrics.MaxDrawdownPct >= 10 {
		currentBTCLev := maxInt(1, strategyCfg.RiskControl.BTCETHMaxLeverage)
		currentAltLev := maxInt(1, strategyCfg.RiskControl.AltcoinMaxLeverage)
		targetBTCLev := clampInt(int(float64(currentBTCLev)*0.75), 1, 20)
		targetAltLev := clampInt(int(float64(currentAltLev)*0.7), 1, 20)
		if targetBTCLev == currentBTCLev && currentBTCLev > 1 {
			targetBTCLev--
		}
		if targetAltLev == currentAltLev && currentAltLev > 1 {
			targetAltLev--
		}

		maxMarginUsage := strategyCfg.RiskControl.MaxMarginUsage
		if maxMarginUsage <= 0 || maxMarginUsage > 1 {
			maxMarginUsage = 0.9
		}
		if maxMarginUsage > 0.75 {
			maxMarginUsage = 0.75
		}

		maxPositions := strategyCfg.RiskControl.MaxPositions
		if maxPositions <= 1 {
			maxPositions = 1
		} else {
			maxPositions--
		}

		add(generatedImprovement{
			Suggestion: strategyImprovementSuggestion{
				ID:             "drawdown_shield",
				Title:          "Reduce drawdown pressure",
				Reason:         fmt.Sprintf("Max drawdown %.2f%% is above the 10%% threshold.", metrics.MaxDrawdownPct),
				ExpectedImpact: "Lower leverage and tighter margin usage should reduce peak-to-trough losses.",
				Patch: map[string]any{
					"risk_control": map[string]any{
						"btc_eth_max_leverage": targetBTCLev,
						"altcoin_max_leverage": targetAltLev,
						"max_margin_usage":     maxMarginUsage,
						"max_positions":        maxPositions,
					},
				},
			},
			Apply: func(cfg *store.StrategyConfig, btCfg *backtest.BacktestConfig) {
				cfg.RiskControl.BTCETHMaxLeverage = targetBTCLev
				cfg.RiskControl.AltcoinMaxLeverage = targetAltLev
				cfg.RiskControl.MaxMarginUsage = maxMarginUsage
				cfg.RiskControl.MaxPositions = maxPositions
				btCfg.Leverage.BTCETHLeverage = targetBTCLev
				btCfg.Leverage.AltcoinLeverage = targetAltLev
			},
		})
	}

	if metrics != nil && metrics.SharpeRatio > 0 && metrics.SharpeRatio < 1.0 {
		targetTimeframes := mergeTimeframes(strategyCfg.Indicators.Klines.SelectedTimeframes, recommendedTimeframes(runCfg.DecisionTimeframe))
		minConfidence := clampInt(maxInt(strategyCfg.RiskControl.MinConfidence, 80), 60, 95)
		minRR := strategyCfg.RiskControl.MinRiskRewardRatio
		if minRR < 2.5 {
			minRR = 2.5
		}

		add(generatedImprovement{
			Suggestion: strategyImprovementSuggestion{
				ID:             "sharpe_boost",
				Title:          "Increase signal quality",
				Reason:         fmt.Sprintf("Sharpe ratio %.2f indicates weak risk-adjusted returns.", metrics.SharpeRatio),
				ExpectedImpact: "Multi-timeframe confirmation and stricter entry confidence should improve Sharpe.",
				Patch: map[string]any{
					"indicators": map[string]any{
						"enable_rsi":             true,
						"enable_macd":            true,
						"selected_timeframes":    targetTimeframes,
						"enable_multi_timeframe": len(targetTimeframes) > 1,
					},
					"risk_control": map[string]any{
						"min_confidence":        minConfidence,
						"min_risk_reward_ratio": minRR,
					},
				},
			},
			Apply: func(cfg *store.StrategyConfig, btCfg *backtest.BacktestConfig) {
				cfg.Indicators.EnableRSI = true
				cfg.Indicators.EnableMACD = true
				cfg.Indicators.Klines.SelectedTimeframes = targetTimeframes
				cfg.Indicators.Klines.PrimaryTimeframe = targetTimeframes[0]
				cfg.Indicators.Klines.LongerTimeframe = targetTimeframes[len(targetTimeframes)-1]
				cfg.Indicators.Klines.EnableMultiTimeframe = len(targetTimeframes) > 1
				cfg.RiskControl.MinConfidence = minConfidence
				cfg.RiskControl.MinRiskRewardRatio = minRR

				btCfg.Timeframes = targetTimeframes
				btCfg.DecisionTimeframe = targetTimeframes[0]
			},
		})
	}

	if metrics != nil && (metrics.Trades > 120 || (metrics.Trades > 60 && metrics.WinRate < 45)) {
		currentCadence := runCfg.DecisionCadenceNBars
		if currentCadence <= 0 {
			currentCadence = 20
		}
		targetCadence := currentCadence + 5
		if targetCadence > 120 {
			targetCadence = 120
		}

		add(generatedImprovement{
			Suggestion: strategyImprovementSuggestion{
				ID:             "trade_frequency_control",
				Title:          "Reduce overtrading",
				Reason:         fmt.Sprintf("Trade count=%d with win rate %.2f%% suggests noisy entries.", metrics.Trades, metrics.WinRate),
				ExpectedImpact: "Longer decision cadence should reduce low-quality churn.",
				Patch: map[string]any{
					"decision_cadence_nbars": targetCadence,
				},
			},
			Apply: func(_ *store.StrategyConfig, btCfg *backtest.BacktestConfig) {
				btCfg.DecisionCadenceNBars = targetCadence
			},
		})
	}

	if metrics != nil && metrics.TotalReturnPct <= 0 {
		enableMomentum := !strategyCfg.Indicators.EnableEMA || !strategyCfg.Indicators.EnableMACD
		minConfidence := clampInt(maxInt(strategyCfg.RiskControl.MinConfidence, 78), 60, 95)

		add(generatedImprovement{
			Suggestion: strategyImprovementSuggestion{
				ID:             "return_recovery",
				Title:          "Improve return profile",
				Reason:         fmt.Sprintf("Total return %.2f%% is non-positive.", metrics.TotalReturnPct),
				ExpectedImpact: "Momentum confirmation and stricter confidence can avoid low-conviction entries.",
				Patch: map[string]any{
					"indicators": map[string]any{
						"enable_ema":  true,
						"enable_macd": true,
					},
					"risk_control": map[string]any{
						"min_confidence": minConfidence,
					},
				},
			},
			Apply: func(cfg *store.StrategyConfig, _ *backtest.BacktestConfig) {
				if enableMomentum {
					cfg.Indicators.EnableEMA = true
					cfg.Indicators.EnableMACD = true
				}
				cfg.RiskControl.MinConfidence = minConfidence
			},
		})
	}

	if containsAny(goalLower, "volatility", "atr") || (metrics != nil && metrics.MaxDrawdownPct >= 8) {
		add(generatedImprovement{
			Suggestion: strategyImprovementSuggestion{
				ID:             "volatility_filter",
				Title:          "Add volatility filter",
				Reason:         "Requested explicitly or implied by unstable equity curve.",
				ExpectedImpact: "ATR/BOLL filters help avoid entries during chaotic market regimes.",
				Patch: map[string]any{
					"indicators": map[string]any{
						"enable_atr":  true,
						"enable_boll": true,
						"atr_periods": []int{14},
					},
				},
			},
			Apply: func(cfg *store.StrategyConfig, _ *backtest.BacktestConfig) {
				cfg.Indicators.EnableATR = true
				cfg.Indicators.EnableBOLL = true
				if len(cfg.Indicators.ATRPeriods) == 0 {
					cfg.Indicators.ATRPeriods = []int{14}
				}
				if len(cfg.Indicators.BOLLPeriods) == 0 {
					cfg.Indicators.BOLLPeriods = []int{20}
				}
			},
		})
	}

	if containsAny(goalLower, "momentum", "trend") {
		add(generatedImprovement{
			Suggestion: strategyImprovementSuggestion{
				ID:             "momentum_confirmation",
				Title:          "Strengthen momentum confirmation",
				Reason:         "Goal emphasizes momentum or trend-following behavior.",
				ExpectedImpact: "EMA + MACD + RSI alignment reduces false breakouts.",
				Patch: map[string]any{
					"indicators": map[string]any{
						"enable_ema":  true,
						"enable_macd": true,
						"enable_rsi":  true,
					},
				},
			},
			Apply: func(cfg *store.StrategyConfig, _ *backtest.BacktestConfig) {
				cfg.Indicators.EnableEMA = true
				cfg.Indicators.EnableMACD = true
				cfg.Indicators.EnableRSI = true
			},
		})
	}

	if len(items) == 0 {
		add(generatedImprovement{
			Suggestion: strategyImprovementSuggestion{
				ID:             "baseline_tuning",
				Title:          "Apply baseline tuning",
				Reason:         "No major red flags detected; use conservative optimization defaults.",
				ExpectedImpact: "Small risk-control and signal-quality adjustments for incremental improvement.",
				Patch: map[string]any{
					"risk_control": map[string]any{
						"min_confidence":        clampInt(maxInt(strategyCfg.RiskControl.MinConfidence, 76), 60, 95),
						"min_risk_reward_ratio": maxFloat(strategyCfg.RiskControl.MinRiskRewardRatio, 2.5),
					},
				},
			},
			Apply: func(cfg *store.StrategyConfig, _ *backtest.BacktestConfig) {
				cfg.RiskControl.MinConfidence = clampInt(maxInt(cfg.RiskControl.MinConfidence, 76), 60, 95)
				cfg.RiskControl.MinRiskRewardRatio = maxFloat(cfg.RiskControl.MinRiskRewardRatio, 2.5)
			},
		})
	}

	return items
}

func inferStrategyConfigFromDescription(description string) store.StrategyConfig {
	desc := strings.TrimSpace(description)
	descLower := strings.ToLower(desc)

	cfg := store.GetDefaultStrategyConfig("en")
	cfg.CoinSource.SourceType = "static"
	cfg.CoinSource.UseAI500 = false
	cfg.CoinSource.UseOITop = false
	cfg.CoinSource.UseOILow = false
	cfg.CoinSource.AI500Limit = 0
	cfg.CoinSource.OITopLimit = 0
	cfg.CoinSource.OILowLimit = 0
	cfg.CoinSource.ExcludedCoins = nil

	symbols := extractSymbolsFromText(desc)
	if len(symbols) == 0 {
		symbols = []string{"BTCUSDT"}
	}
	cfg.CoinSource.StaticCoins = symbols

	// Keep base data channels enabled for strategy context.
	cfg.Indicators.EnableRawKlines = true
	cfg.Indicators.EnableVolume = true
	cfg.Indicators.EnableOI = true
	cfg.Indicators.EnableFundingRate = true

	if containsAny(descLower, "momentum", "trend") {
		cfg.Indicators.EnableEMA = true
		cfg.Indicators.EnableMACD = true
	}
	if containsAny(descLower, "mean reversion", "reversal") {
		cfg.Indicators.EnableRSI = true
		cfg.Indicators.EnableBOLL = true
	}
	if strings.Contains(descLower, "rsi") {
		cfg.Indicators.EnableRSI = true
	}
	if strings.Contains(descLower, "macd") {
		cfg.Indicators.EnableMACD = true
	}
	if strings.Contains(descLower, "ema") {
		cfg.Indicators.EnableEMA = true
	}
	if containsAny(descLower, "atr", "volatility") {
		cfg.Indicators.EnableATR = true
	}
	if containsAny(descLower, "boll", "bollinger") {
		cfg.Indicators.EnableBOLL = true
	}

	if !cfg.Indicators.EnableEMA && !cfg.Indicators.EnableMACD && !cfg.Indicators.EnableRSI && !cfg.Indicators.EnableATR && !cfg.Indicators.EnableBOLL {
		// Minimum sane defaults for signal generation.
		cfg.Indicators.EnableEMA = true
		cfg.Indicators.EnableRSI = true
	}

	timeframes := extractTimeframesFromText(desc)
	if len(timeframes) == 0 {
		if containsAny(descLower, "scalp", "scalping") {
			timeframes = []string{"1m", "5m", "15m"}
		} else if containsAny(descLower, "swing") {
			timeframes = []string{"15m", "1h", "4h"}
		} else {
			timeframes = []string{"5m", "15m", "1h"}
		}
	}
	cfg.Indicators.Klines.SelectedTimeframes = timeframes
	cfg.Indicators.Klines.PrimaryTimeframe = timeframes[0]
	cfg.Indicators.Klines.LongerTimeframe = timeframes[len(timeframes)-1]
	cfg.Indicators.Klines.EnableMultiTimeframe = len(timeframes) > 1

	if leverage := extractLeverageFromText(desc); leverage > 0 {
		cfg.RiskControl.BTCETHMaxLeverage = leverage
		cfg.RiskControl.AltcoinMaxLeverage = clampInt(leverage-2, 1, leverage)
	}

	if containsAny(descLower, "conservative", "low risk") {
		cfg.RiskControl.BTCETHMaxLeverage = clampInt(cfg.RiskControl.BTCETHMaxLeverage, 1, 5)
		cfg.RiskControl.AltcoinMaxLeverage = clampInt(cfg.RiskControl.AltcoinMaxLeverage, 1, 3)
		cfg.RiskControl.MaxMarginUsage = 0.65
		cfg.RiskControl.MinConfidence = clampInt(maxInt(cfg.RiskControl.MinConfidence, 80), 70, 95)
	}
	if containsAny(descLower, "aggressive", "high risk") {
		cfg.RiskControl.BTCETHMaxLeverage = clampInt(maxInt(cfg.RiskControl.BTCETHMaxLeverage, 8), 1, 20)
		cfg.RiskControl.AltcoinMaxLeverage = clampInt(maxInt(cfg.RiskControl.AltcoinMaxLeverage, 6), 1, 20)
		cfg.RiskControl.MaxMarginUsage = maxFloat(cfg.RiskControl.MaxMarginUsage, 0.9)
		cfg.RiskControl.MinConfidence = clampInt(cfg.RiskControl.MinConfidence, 60, 95)
	}

	if desc != "" {
		cfg.CustomPrompt = "Agent strategy intent: " + desc
	}

	return cfg
}

var (
	symbolUSDTRegex  = regexp.MustCompile(`\b[A-Z]{2,12}USDT\b`)
	symbolTokenRegex = regexp.MustCompile(`\b[A-Z]{2,10}\b`)
	timeframeRegex   = regexp.MustCompile(`\b(1m|3m|5m|15m|30m|1h|2h|4h|6h|12h|1d)\b`)
	leverageRegex    = regexp.MustCompile(`\b([1-9]|1[0-9]|20)\s*x\b`)
)

func extractSymbolsFromText(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}

	upper := strings.ToUpper(input)
	seen := make(map[string]bool)
	out := make([]string, 0, 8)

	appendSymbol := func(symbol string) {
		symbol = strings.TrimSpace(strings.ToUpper(symbol))
		if symbol == "" {
			return
		}
		if !strings.HasSuffix(symbol, "USDT") {
			symbol += "USDT"
		}
		symbol = market.Normalize(symbol)
		if !seen[symbol] {
			seen[symbol] = true
			out = append(out, symbol)
		}
	}

	for _, symbol := range symbolUSDTRegex.FindAllString(upper, -1) {
		appendSymbol(symbol)
	}

	known := map[string]string{
		"BTC":  "BTCUSDT",
		"ETH":  "ETHUSDT",
		"SOL":  "SOLUSDT",
		"XRP":  "XRPUSDT",
		"ADA":  "ADAUSDT",
		"BNB":  "BNBUSDT",
		"DOGE": "DOGEUSDT",
		"LINK": "LINKUSDT",
		"DOT":  "DOTUSDT",
		"AVAX": "AVAXUSDT",
		"UNI":  "UNIUSDT",
		"LTC":  "LTCUSDT",
		"BCH":  "BCHUSDT",
		"TRX":  "TRXUSDT",
		"SUI":  "SUIUSDT",
		"ATOM": "ATOMUSDT",
		"NEAR": "NEARUSDT",
		"APT":  "APTUSDT",
		"PEPE": "PEPEUSDT",
		"ARB":  "ARBUSDT",
		"OP":   "OPUSDT",
		"INJ":  "INJUSDT",
		"WIF":  "WIFUSDT",
	}
	for _, token := range symbolTokenRegex.FindAllString(upper, -1) {
		if symbol, ok := known[token]; ok {
			appendSymbol(symbol)
		}
	}

	return out
}

func extractTimeframesFromText(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}

	matches := timeframeRegex.FindAllString(strings.ToLower(input), -1)
	if len(matches) == 0 {
		return nil
	}
	return mergeTimeframes(nil, matches)
}

func extractLeverageFromText(input string) int {
	matches := leverageRegex.FindStringSubmatch(strings.ToLower(input))
	if len(matches) < 2 {
		return 0
	}
	lev, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return clampInt(lev, 1, 20)
}

func recommendedTimeframes(decisionTF string) []string {
	base := []string{"15m", "1h", "4h"}
	if norm, err := market.NormalizeTimeframe(decisionTF); err == nil && norm != "" {
		base = append([]string{norm}, base...)
	}
	return mergeTimeframes(nil, base)
}

func mergeTimeframes(base []string, extras []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(base)+len(extras))

	appendTF := func(tf string) {
		if tf == "" {
			return
		}
		norm, err := market.NormalizeTimeframe(tf)
		if err != nil {
			return
		}
		if !seen[norm] {
			seen[norm] = true
			out = append(out, norm)
		}
	}

	for _, tf := range base {
		appendTF(tf)
	}
	for _, tf := range extras {
		appendTF(tf)
	}

	if len(out) == 0 {
		out = []string{"5m", "15m", "1h"}
	}
	slices.SortStableFunc(out, func(a, b string) int {
		// Keep natural short-to-long timeframe order for readability.
		order := map[string]int{
			"1m": 1, "3m": 2, "5m": 3, "15m": 4, "30m": 5,
			"1h": 6, "2h": 7, "4h": 8, "6h": 9, "12h": 10, "1d": 11,
		}
		return order[a] - order[b]
	})
	return out
}

func normalizeSymbols(symbols []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		norm := market.Normalize(sym)
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	return out
}

func containsAny(input string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(input, needle) {
			return true
		}
	}
	return false
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
