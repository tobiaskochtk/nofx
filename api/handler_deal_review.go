package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"nofx/logger"
	"nofx/mcp"
	"nofx/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type dealReviewListResponse struct {
	Items   []store.DealReviewCaseListItem  `json:"items"`
	Summary *store.DealReviewDatasetSummary `json:"summary"`
	Total   int64                           `json:"total"`
}

type dealReviewAIScanRequest struct {
	ModelID           string   `json:"model_id"`
	OverrideModelName string   `json:"override_model_name"`
	Symbol            string   `json:"symbol"`
	Side              string   `json:"side"`
	Status            string   `json:"status"`
	Outcome           string   `json:"outcome"`
	FromTime          int64    `json:"from_time"`
	ToTime            int64    `json:"to_time"`
	MinPnL            *float64 `json:"min_pnl"`
	MaxPnL            *float64 `json:"max_pnl"`
}

type remoteModelInfo struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
}

type dealReviewCaseReviewRequest struct {
	Labels      []string `json:"labels"`
	AnalystNote string   `json:"analyst_note"`
}

type dealReviewChallengerLaunchRequest struct {
	Mode        string `json:"mode"`
	ExchangeID  string `json:"exchange_id"`
	WindowHours int    `json:"window_hours"`
}

type dealReviewChallengerResolveRequest struct {
	WinnerTraderID string `json:"winner_trader_id"`
}

type dealReviewJSONDiffEntry struct {
	Path  string `json:"path"`
	Left  string `json:"left"`
	Right string `json:"right"`
}

type dealReviewAIScanCompareResponse struct {
	Left              *store.DealReviewAIScanDetail `json:"left"`
	Right             *store.DealReviewAIScanDetail `json:"right"`
	SameFilters       bool                          `json:"same_filters"`
	FilterDifferences []dealReviewJSONDiffEntry     `json:"filter_differences,omitempty"`
	PatchDifferences  []dealReviewJSONDiffEntry     `json:"patch_differences,omitempty"`
	StrengthOverlap   []string                      `json:"strength_overlap,omitempty"`
	WeaknessOverlap   []string                      `json:"weakness_overlap,omitempty"`
	ImmediateActionsA []string                      `json:"immediate_actions_a,omitempty"`
	ImmediateActionsB []string                      `json:"immediate_actions_b,omitempty"`
}

func (s *Server) handleTraderDealReviewCases(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	filter := parseDealReviewFilter(c)
	filter.TraderID = traderID

	items, summary, total, err := s.store.DealReview().ListCases(userID, filter)
	if err != nil {
		SafeInternalError(c, "Failed to fetch deal review cases", err)
		return
	}

	c.JSON(http.StatusOK, dealReviewListResponse{
		Items:   items,
		Summary: summary,
		Total:   total,
	})
}

func (s *Server) handleTraderDealReviewCaseDetail(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	caseID := c.Param("caseId")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	detail, err := s.store.DealReview().GetCaseDetail(userID, traderID, caseID)
	if err != nil {
		SafeNotFound(c, "Deal review case")
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleTraderDealReviewCaseReview(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	caseID := c.Param("caseId")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	var req dealReviewCaseReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid case review payload")
		return
	}

	detail, err := s.store.DealReview().UpdateCaseReview(userID, traderID, caseID, req.Labels, req.AnalystNote)
	if err != nil {
		SafeInternalError(c, "Failed to update deal review annotations", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleTraderDealReviewAIScans(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	limit := 10
	if raw := strings.TrimSpace(c.DefaultQuery("limit", "10")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	scans, err := s.store.DealReview().ListAIScans(userID, traderID, limit)
	if err != nil {
		SafeInternalError(c, "Failed to fetch deal review AI scans", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": scans})
}

func (s *Server) handleTraderDealReviewAIScanCompare(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	leftID := strings.TrimSpace(c.Query("left_scan_id"))
	rightID := strings.TrimSpace(c.Query("right_scan_id"))
	if leftID == "" || rightID == "" {
		SafeBadRequest(c, "left_scan_id and right_scan_id are required")
		return
	}

	left, err := s.store.DealReview().GetAIScan(userID, traderID, leftID)
	if err != nil {
		SafeNotFound(c, "Left AI scan")
		return
	}
	right, err := s.store.DealReview().GetAIScan(userID, traderID, rightID)
	if err != nil {
		SafeNotFound(c, "Right AI scan")
		return
	}

	response := &dealReviewAIScanCompareResponse{
		Left:              left,
		Right:             right,
		SameFilters:       equalJSONMaps(left.Filters, right.Filters),
		FilterDifferences: diffJSONMaps(left.Filters, right.Filters),
		PatchDifferences:  diffJSONMaps(left.StrategyPatch, right.StrategyPatch),
		StrengthOverlap:   intersectStringSlices(left.Result.Strengths, right.Result.Strengths),
		WeaknessOverlap:   intersectStringSlices(left.Result.Weaknesses, right.Result.Weaknesses),
		ImmediateActionsA: actionTitles(left.Result.ImmediateActions),
		ImmediateActionsB: actionTitles(right.Result.ImmediateActions),
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) handleTraderDealReviewAIScan(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	var req dealReviewAIScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid AI scan request")
		return
	}

	traderCfg, err := s.store.Trader().Get(userID, traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	strategyCfg, strategyRecord, err := s.loadStrategyForTrader(userID, traderCfg)
	if err != nil {
		SafeInternalError(c, "Failed to load strategy", err)
		return
	}

	filter := store.DealReviewListFilter{
		TraderID: traderID,
		Symbol:   req.Symbol,
		Side:     req.Side,
		Status:   req.Status,
		Outcome:  req.Outcome,
		FromTime: req.FromTime,
		ToTime:   req.ToTime,
		MinPnL:   req.MinPnL,
		MaxPnL:   req.MaxPnL,
	}
	cases, summary, err := s.store.DealReview().ListAnalysisCases(userID, filter, 120)
	if err != nil {
		SafeInternalError(c, "Failed to load deal review dataset", err)
		return
	}
	if len(cases) == 0 {
		SafeBadRequest(c, "No deals matched the selected filters")
		return
	}

	modelCfg, modelName, err := s.resolveAIScanModel(userID, req.ModelID, req.OverrideModelName)
	if err != nil {
		SafeBadRequest(c, err.Error())
		return
	}

	promptPayload, err := buildDealReviewAnalysisPayload(traderCfg, strategyRecord, strategyCfg, filter, cases, summary)
	if err != nil {
		SafeInternalError(c, "Failed to build AI scan payload", err)
		return
	}
	systemPrompt, userPrompt, err := buildDealReviewAnalysisPrompt(promptPayload)
	if err != nil {
		SafeInternalError(c, "Failed to build AI prompt", err)
		return
	}

	aiClient := newClientFromModelConfig(modelCfg, modelName)
	response, err := aiClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		SafeInternalError(c, "AI scan failed", err)
		return
	}

	result, rawJSON, err := parseDealReviewAIScanResponse(response)
	if err != nil {
		SafeInternalError(c, "Failed to parse AI scan response", err)
		return
	}

	filterJSON, _ := json.Marshal(filter)
	patchJSON, _ := json.Marshal(result.StrategyPatch)
	scan := &store.DealReviewAIScan{
		UserID:            userID,
		TraderID:          traderID,
		StrategyID:        traderCfg.StrategyID,
		ModelConfigID:     modelCfg.ID,
		Provider:          modelCfg.Provider,
		ModelName:         modelName,
		DatasetCount:      len(cases),
		FilterJSON:        string(filterJSON),
		ResultJSON:        rawJSON,
		StrategyPatchJSON: string(patchJSON),
		Status:            "completed",
		Summary:           result.ExecutiveSummary,
	}
	if err := s.store.DealReview().SaveAIScan(scan); err != nil {
		SafeInternalError(c, "Failed to persist AI scan", err)
		return
	}

	detail, err := s.store.DealReview().GetAIScan(userID, traderID, scan.ID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch saved AI scan", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleTraderDealReviewValidateAIScan(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	scanID := c.Param("scanId")

	traderCfg, err := s.store.Trader().Get(userID, traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	scanDetail, err := s.store.DealReview().GetAIScan(userID, traderID, scanID)
	if err != nil {
		SafeNotFound(c, "AI scan")
		return
	}

	validation, _, _, err := s.validateDealReviewAIScan(userID, traderCfg, scanDetail)
	if err != nil {
		SafeInternalError(c, "Failed to validate AI scan", err)
		return
	}
	if err := s.store.DealReview().UpdateAIScanValidation(userID, traderID, scanID, validation); err != nil {
		SafeInternalError(c, "Failed to persist AI scan validation", err)
		return
	}

	detail, err := s.store.DealReview().GetAIScan(userID, traderID, scanID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch validated AI scan", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleTraderDealReviewCreateChallengerCompare(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	scanID := c.Param("scanId")

	var req dealReviewChallengerLaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid challenger launch payload")
		return
	}

	traderCfg, err := s.store.Trader().Get(userID, traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	scanDetail, err := s.store.DealReview().GetAIScan(userID, traderID, scanID)
	if err != nil {
		SafeNotFound(c, "AI scan")
		return
	}

	validation, mergedConfigJSON, strategyRecord, err := s.validateDealReviewAIScan(userID, traderCfg, scanDetail)
	if err != nil {
		SafeInternalError(c, "Failed to validate AI scan", err)
		return
	}
	if err := s.store.DealReview().UpdateAIScanValidation(userID, traderID, scanID, validation); err != nil {
		SafeInternalError(c, "Failed to persist AI scan validation", err)
		return
	}
	if validation.Status != store.DealReviewAIScanValidationPassed {
		SafeBadRequest(c, buildDealReviewValidationBlockMessage(validation))
		return
	}

	windowHours := normalizeDealReviewCompareWindowHours(req.WindowHours)
	mode := normalizeDealReviewChallengerMode(req.Mode)
	exchangeID := strings.TrimSpace(req.ExchangeID)
	if exchangeID == "" {
		exchangeID = traderCfg.ExchangeID
	}

	exchangeCfg, err := s.store.Exchange().GetByID(userID, exchangeID)
	if err != nil {
		SafeBadRequest(c, "Selected challenger wallet does not exist")
		return
	}
	if !exchangeCfg.Enabled {
		SafeBadRequest(c, "Selected challenger wallet is disabled")
		return
	}
	if err := validateDealReviewChallengerMode(mode, traderCfg.ExchangeID, exchangeCfg); err != nil {
		SafeBadRequest(c, err.Error())
		return
	}

	challengerStrategyID := uuid.NewString()
	challengerTraderID := uuid.NewString()
	compareID := uuid.NewString()
	versionID := uuid.NewString()
	createdAt := time.Now().UTC()

	challengerStrategyName := buildDealReviewChallengerStrategyName(strategyRecord.Name, createdAt)
	if err := s.store.Strategy().Duplicate(userID, strategyRecord.ID, challengerStrategyID, challengerStrategyName); err != nil {
		SafeInternalError(c, "Failed to duplicate strategy for challenger", err)
		return
	}
	challengerStrategy, err := s.store.Strategy().Get(userID, challengerStrategyID)
	if err != nil {
		SafeInternalError(c, "Failed to load challenger strategy", err)
		return
	}
	challengerStrategy.Config = mergedConfigJSON
	if err := s.store.Strategy().Update(challengerStrategy); err != nil {
		SafeInternalError(c, "Failed to save challenger strategy config", err)
		return
	}

	challengerTrader := cloneDealReviewChallengerTrader(traderCfg, challengerTraderID, challengerStrategyID, exchangeCfg.ID, createdAt)
	if err := s.store.Trader().Create(challengerTrader); err != nil {
		SafeInternalError(c, "Failed to create challenger trader", err)
		return
	}

	version := &store.DealReviewStrategyVersion{
		ID:                 versionID,
		UserID:             userID,
		TraderID:           traderID,
		StrategyID:         strategyRecord.ID,
		SourceScanID:       scanID,
		SourceCompareID:    compareID,
		SourceType:         "ai_challenger_candidate",
		Summary:            fmt.Sprintf("Challenger candidate prepared (%s, %dh)", mode, windowHours),
		PreviousConfigJSON: strategyRecord.Config,
		NextConfigJSON:     mergedConfigJSON,
	}
	if err := s.store.DealReview().SaveStrategyVersion(version); err != nil {
		SafeInternalError(c, "Failed to record challenger strategy version", err)
		return
	}

	compare := &store.DealReviewChallengerCompare{
		ID:                      compareID,
		UserID:                  userID,
		TraderID:                traderID,
		IncumbentTraderID:       traderCfg.ID,
		IncumbentStrategyID:     strategyRecord.ID,
		ChallengerTraderID:      challengerTrader.ID,
		ChallengerStrategyID:    challengerStrategy.ID,
		ChallengerExchangeID:    exchangeCfg.ID,
		Mode:                    mode,
		WindowHours:             windowHours,
		SourceScanID:            scanID,
		SourceStrategyVersionID: versionID,
		Status:                  store.DealReviewChallengerStatusStarting,
		Summary:                 fmt.Sprintf("Provisioning challenger on %s for %dh.", mode, windowHours),
		StartedAt:               createdAt,
		EndsAt:                  createdAt.Add(time.Duration(windowHours) * time.Hour),
	}
	appendDealReviewChallengerProtocol(compare, "created", "system", fmt.Sprintf("Created challenger candidate in %s mode for %dh.", mode, windowHours), nil, map[string]any{
		"source_scan_id":         scanID,
		"source_strategy_id":     strategyRecord.ID,
		"challenger_trader_id":   challengerTrader.ID,
		"challenger_strategy_id": challengerStrategy.ID,
		"challenger_exchange_id": exchangeCfg.ID,
		"window_hours":           windowHours,
		"mode":                   mode,
	})
	if err := s.store.DealReview().SaveChallengerCompare(compare); err != nil {
		SafeInternalError(c, "Failed to persist challenger compare", err)
		return
	}

	if err := s.traderManager.LoadUserTradersFromStore(s.store, userID); err != nil {
		compare.Status = store.DealReviewChallengerStatusFailed
		compare.ErrorMessage = err.Error()
		compare.Summary = "Failed to load challenger trader into runtime."
		compare.ResolvedAt = time.Now().UTC()
		appendDealReviewChallengerProtocol(compare, "failed", "system", compare.Summary, nil, map[string]any{"error": err.Error()})
		_ = s.store.DealReview().SaveChallengerCompare(compare)
		SafeInternalError(c, "Failed to load challenger trader", err)
		return
	}
	if err := s.ensureDealReviewTraderRunning(userID, traderCfg.ID); err != nil {
		compare.Status = store.DealReviewChallengerStatusFailed
		compare.ErrorMessage = err.Error()
		compare.Summary = "Failed to start incumbent trader for comparison."
		compare.ResolvedAt = time.Now().UTC()
		appendDealReviewChallengerProtocol(compare, "failed", "system", compare.Summary, nil, map[string]any{"error": err.Error()})
		_ = s.store.DealReview().SaveChallengerCompare(compare)
		SafeInternalError(c, "Failed to start incumbent trader", err)
		return
	}
	if err := s.ensureDealReviewTraderRunning(userID, challengerTrader.ID); err != nil {
		compare.Status = store.DealReviewChallengerStatusFailed
		compare.ErrorMessage = err.Error()
		compare.Summary = "Failed to start challenger trader."
		compare.ResolvedAt = time.Now().UTC()
		appendDealReviewChallengerProtocol(compare, "failed", "system", compare.Summary, nil, map[string]any{"error": err.Error()})
		_ = s.store.DealReview().SaveChallengerCompare(compare)
		SafeInternalError(c, "Failed to start challenger trader", err)
		return
	}

	metrics, _ := s.computeDealReviewChallengerMetrics(compare)
	compare.Status = store.DealReviewChallengerStatusRunning
	compare.Summary = fmt.Sprintf("Challenger running in %s mode until %s.", mode, compare.EndsAt.Local().Format(time.RFC3339))
	if metrics != nil {
		body, _ := json.Marshal(metrics)
		compare.MetricsJSON = string(body)
		compare.LastEvaluatedAt = metrics.EvaluatedAt
	}
	appendDealReviewChallengerProtocol(compare, "running", "system", compare.Summary, metrics, nil)
	if err := s.store.DealReview().SaveChallengerCompare(compare); err != nil {
		SafeInternalError(c, "Failed to finalize challenger compare", err)
		return
	}

	detail, err := s.store.DealReview().GetChallengerCompare(userID, traderID, compareID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch challenger compare", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleTraderDealReviewChallengerCompares(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	limit := 10
	if raw := strings.TrimSpace(c.DefaultQuery("limit", "10")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	if err := s.processDealReviewChallengerComparesForTrader(userID, traderID); err != nil {
		logger.Warnf("⚠️ Failed to refresh deal review challenger compares for trader %s: %v", traderID, err)
	}

	compares, err := s.store.DealReview().ListChallengerCompares(userID, traderID, limit)
	if err != nil {
		SafeInternalError(c, "Failed to fetch challenger compares", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": compares})
}

func (s *Server) handleTraderDealReviewChallengerCompareDetail(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	compareID := c.Param("compareId")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	compare, err := s.loadFreshDealReviewChallengerCompare(userID, traderID, compareID)
	if err != nil {
		SafeNotFound(c, "Challenger compare")
		return
	}

	detail, err := s.store.DealReview().GetChallengerCompare(userID, traderID, compare.ID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch challenger compare", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleTraderDealReviewStopChallengerCompare(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	compareID := c.Param("compareId")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	compare, err := s.loadFreshDealReviewChallengerCompare(userID, traderID, compareID)
	if err != nil {
		SafeNotFound(c, "Challenger compare")
		return
	}
	if !isDealReviewChallengerCompareActive(compare.Status) {
		SafeBadRequest(c, "Only running compares can be stopped")
		return
	}

	metrics, _ := s.computeDealReviewChallengerMetrics(compare)
	if metrics != nil {
		body, _ := json.Marshal(metrics)
		compare.MetricsJSON = string(body)
		compare.LastEvaluatedAt = metrics.EvaluatedAt
	}
	if err := s.stopDealReviewTrader(userID, compare.ChallengerTraderID); err != nil {
		SafeInternalError(c, "Failed to stop challenger trader", err)
		return
	}

	now := time.Now().UTC()
	compare.Status = store.DealReviewChallengerStatusStopped
	compare.WinnerTraderID = ""
	compare.LoserTraderID = ""
	compare.ResolvedAt = now
	compare.Summary = "Comparison stopped manually. Incumbent remains active and challenger was disabled."
	appendDealReviewChallengerProtocol(compare, "stopped", "user", compare.Summary, metrics, map[string]any{
		"stopped_trader_id": compare.ChallengerTraderID,
		"kept_trader_id":    compare.IncumbentTraderID,
	})
	if err := s.store.DealReview().SaveChallengerCompare(compare); err != nil {
		SafeInternalError(c, "Failed to update challenger compare", err)
		return
	}

	detail, err := s.store.DealReview().GetChallengerCompare(userID, traderID, compare.ID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch challenger compare", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleTraderDealReviewResolveChallengerCompare(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	compareID := c.Param("compareId")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	var req dealReviewChallengerResolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid challenger resolve payload")
		return
	}

	compare, err := s.loadFreshDealReviewChallengerCompare(userID, traderID, compareID)
	if err != nil {
		SafeNotFound(c, "Challenger compare")
		return
	}
	if !isDealReviewChallengerCompareActive(compare.Status) {
		SafeBadRequest(c, "Only running compares can be resolved manually")
		return
	}

	winnerTraderID := strings.TrimSpace(req.WinnerTraderID)
	loserTraderID := ""
	winnerRole := ""
	switch winnerTraderID {
	case compare.IncumbentTraderID:
		loserTraderID = compare.ChallengerTraderID
		winnerRole = "incumbent"
	case compare.ChallengerTraderID:
		loserTraderID = compare.IncumbentTraderID
		winnerRole = "challenger"
	default:
		SafeBadRequest(c, "winner_trader_id must match incumbent or challenger trader")
		return
	}

	metrics, _ := s.computeDealReviewChallengerMetrics(compare)
	if metrics != nil {
		body, _ := json.Marshal(metrics)
		compare.MetricsJSON = string(body)
		compare.LastEvaluatedAt = metrics.EvaluatedAt
	}
	if err := s.stopDealReviewTrader(userID, loserTraderID); err != nil {
		SafeInternalError(c, "Failed to stop losing trader", err)
		return
	}

	now := time.Now().UTC()
	compare.Status = store.DealReviewChallengerStatusCompleted
	compare.WinnerTraderID = winnerTraderID
	compare.LoserTraderID = loserTraderID
	compare.ResolvedAt = now
	compare.Summary = fmt.Sprintf("Comparison resolved manually in favor of the %s trader.", winnerRole)
	appendDealReviewChallengerProtocol(compare, "resolved", "user", compare.Summary, metrics, map[string]any{
		"winner_trader_id": winnerTraderID,
		"loser_trader_id":  loserTraderID,
		"resolution":       "manual",
	})
	if err := s.store.DealReview().SaveChallengerCompare(compare); err != nil {
		SafeInternalError(c, "Failed to update challenger compare", err)
		return
	}

	detail, err := s.store.DealReview().GetChallengerCompare(userID, traderID, compare.ID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch challenger compare", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleTraderDealReviewApplyAIScan(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	scanID := c.Param("scanId")
	traderCfg, err := s.store.Trader().Get(userID, traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	scanDetail, err := s.store.DealReview().GetAIScan(userID, traderID, scanID)
	if err != nil {
		SafeNotFound(c, "AI scan")
		return
	}
	if len(scanDetail.StrategyPatch) == 0 {
		SafeBadRequest(c, "This AI scan does not contain a strategy patch")
		return
	}
	validation, _, _, err := s.validateDealReviewAIScan(userID, traderCfg, scanDetail)
	if err != nil {
		SafeInternalError(c, "Failed to validate AI scan", err)
		return
	}
	if err := s.store.DealReview().UpdateAIScanValidation(userID, traderID, scanID, validation); err != nil {
		SafeInternalError(c, "Failed to persist AI scan validation", err)
		return
	}
	if validation.Status != store.DealReviewAIScanValidationPassed {
		SafeBadRequest(c, buildDealReviewValidationBlockMessage(validation))
		return
	}

	strategyCfg, strategyRecord, err := s.loadStrategyForTrader(userID, traderCfg)
	if err != nil {
		SafeInternalError(c, "Failed to load strategy", err)
		return
	}
	_ = strategyCfg

	mergedConfigJSON, err := applyStrategyPatch(strategyRecord.Config, scanDetail.StrategyPatch)
	if err != nil {
		SafeInternalError(c, "Failed to merge strategy patch", err)
		return
	}

	var mergedConfig store.StrategyConfig
	if err := json.Unmarshal([]byte(mergedConfigJSON), &mergedConfig); err != nil {
		SafeInternalError(c, "Merged strategy patch is invalid", err)
		return
	}
	if warnings, err := validateStrategyConfig(&mergedConfig); err != nil {
		SafeBadRequest(c, err.Error())
		return
	} else if len(warnings) > 0 {
		logger.Infof("⚠️ Deal review strategy patch warnings: %s", strings.Join(warnings, "; "))
	}

	version := &store.DealReviewStrategyVersion{
		UserID:             userID,
		TraderID:           traderID,
		StrategyID:         strategyRecord.ID,
		SourceScanID:       scanID,
		SourceType:         "ai_apply",
		Summary:            scanDetail.Result.ExecutiveSummary,
		PreviousConfigJSON: strategyRecord.Config,
		NextConfigJSON:     mergedConfigJSON,
	}
	if err := s.store.DealReview().SaveStrategyVersion(version); err != nil {
		SafeInternalError(c, "Failed to record strategy version", err)
		return
	}

	wasRunning := false
	if existingMemTrader, memErr := s.traderManager.GetTrader(traderID); memErr == nil {
		status := existingMemTrader.GetStatus()
		if running, ok := status["is_running"].(bool); ok && running {
			wasRunning = true
		}
	}

	strategyRecord.Config = mergedConfigJSON
	if err := s.store.Strategy().Update(strategyRecord); err != nil {
		SafeInternalError(c, "Failed to update strategy", err)
		return
	}
	if err := s.store.DealReview().MarkAIScanApplied(userID, traderID, scanID); err != nil {
		SafeInternalError(c, "Failed to mark AI scan as applied", err)
		return
	}

	if err := s.reloadTraderAfterStrategyChange(userID, traderID, wasRunning, "AI patch apply"); err != nil {
		logger.Warnf("⚠️ Failed to reload trader after AI patch apply: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "AI strategy recommendations applied",
		"strategy_id":       strategyRecord.ID,
		"scan_id":           scanID,
		"version_id":        version.ID,
		"executive_summary": scanDetail.Result.ExecutiveSummary,
		"applied_strategy":  strategyRecord.Name,
	})
}

func (s *Server) handleTraderDealReviewAnomalies(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	filter := parseDealReviewFilter(c)
	filter.TraderID = traderID
	anomalies, err := s.store.DealReview().GetAnomalySummary(userID, filter)
	if err != nil {
		SafeInternalError(c, "Failed to compute deal review anomalies", err)
		return
	}
	c.JSON(http.StatusOK, anomalies)
}

func (s *Server) handleTraderDealReviewStrategyVersions(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	limit := 10
	if raw := strings.TrimSpace(c.DefaultQuery("limit", "10")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}
	versions, err := s.store.DealReview().ListStrategyVersions(userID, traderID, limit)
	if err != nil {
		SafeInternalError(c, "Failed to fetch strategy versions", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": versions})
}

func (s *Server) handleTraderDealReviewRollbackStrategyVersion(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	versionID := c.Param("versionId")

	traderCfg, err := s.store.Trader().Get(userID, traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	versionDetail, err := s.store.DealReview().GetStrategyVersion(userID, traderID, versionID)
	if err != nil {
		SafeNotFound(c, "Strategy version")
		return
	}
	if versionDetail.Version.StrategyID == "" {
		SafeBadRequest(c, "Strategy version has no strategy reference")
		return
	}

	strategyRecord, err := s.store.Strategy().Get(userID, versionDetail.Version.StrategyID)
	if err != nil {
		SafeNotFound(c, "Strategy")
		return
	}

	targetConfigJSON, err := marshalPrettyJSONMap(versionDetail.PreviousConfig)
	if err != nil {
		SafeInternalError(c, "Failed to rebuild previous strategy config", err)
		return
	}

	var targetConfig store.StrategyConfig
	if err := json.Unmarshal([]byte(targetConfigJSON), &targetConfig); err != nil {
		SafeBadRequest(c, "Stored previous strategy config is invalid")
		return
	}
	if warnings, err := validateStrategyConfig(&targetConfig); err != nil {
		SafeBadRequest(c, err.Error())
		return
	} else if len(warnings) > 0 {
		logger.Infof("⚠️ Deal review rollback warnings: %s", strings.Join(warnings, "; "))
	}

	wasRunning := false
	if existingMemTrader, memErr := s.traderManager.GetTrader(traderID); memErr == nil {
		status := existingMemTrader.GetStatus()
		if running, ok := status["is_running"].(bool); ok && running {
			wasRunning = true
		}
	}

	rollbackVersion := &store.DealReviewStrategyVersion{
		UserID:             userID,
		TraderID:           traderID,
		StrategyID:         strategyRecord.ID,
		SourceType:         "rollback",
		Summary:            fmt.Sprintf("Rollback to version %s", versionID),
		PreviousConfigJSON: strategyRecord.Config,
		NextConfigJSON:     targetConfigJSON,
	}
	if err := s.store.DealReview().SaveStrategyVersion(rollbackVersion); err != nil {
		SafeInternalError(c, "Failed to record rollback strategy version", err)
		return
	}

	strategyRecord.Config = targetConfigJSON
	if err := s.store.Strategy().Update(strategyRecord); err != nil {
		SafeInternalError(c, "Failed to rollback strategy", err)
		return
	}

	if err := s.reloadTraderAfterStrategyChange(userID, traderID, wasRunning, "strategy rollback"); err != nil {
		logger.Warnf("⚠️ Failed to reload trader after strategy rollback: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Strategy rolled back",
		"strategy_id":   strategyRecord.ID,
		"version_id":    rollbackVersion.ID,
		"restored_from": versionID,
		"trader_id":     traderCfg.ID,
	})
}

func (s *Server) handleModelAvailableModels(c *gin.Context) {
	userID := c.GetString("user_id")
	modelID := c.Param("id")

	modelCfg, err := s.store.AIModel().Get(userID, modelID)
	if err != nil {
		SafeNotFound(c, "AI model config")
		return
	}
	if strings.TrimSpace(string(modelCfg.APIKey)) == "" {
		SafeBadRequest(c, "Selected AI model config has no API key")
		return
	}

	models, err := s.listAvailableModelsForConfig(modelCfg)
	if err != nil {
		SafeInternalError(c, "Failed to fetch available models", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": models})
}

func (s *Server) reloadTraderAfterStrategyChange(userID, traderID string, wasRunning bool, reason string) error {
	s.traderManager.RemoveTrader(traderID)
	if err := s.traderManager.LoadUserTradersFromStore(s.store, userID); err != nil {
		return err
	}
	if !wasRunning {
		return nil
	}
	reloadedTrader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		return err
	}
	go func() {
		if runErr := reloadedTrader.Run(); runErr != nil {
			logger.Infof("❌ Trader %s runtime error after %s: %v", traderID, reason, runErr)
		}
	}()
	return nil
}

func parseDealReviewFilter(c *gin.Context) store.DealReviewListFilter {
	filter := store.DealReviewListFilter{
		Symbol:  c.Query("symbol"),
		Side:    c.Query("side"),
		Status:  c.Query("status"),
		Outcome: c.Query("outcome"),
		Limit:   100,
		Offset:  0,
	}
	if raw := strings.TrimSpace(c.DefaultQuery("limit", "100")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			filter.Limit = parsed
		}
	}
	if raw := strings.TrimSpace(c.DefaultQuery("offset", "0")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			filter.Offset = parsed
		}
	}
	if raw := strings.TrimSpace(c.Query("from_time")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			filter.FromTime = parsed
		}
	}
	if raw := strings.TrimSpace(c.Query("to_time")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			filter.ToTime = parsed
		}
	}
	if raw := strings.TrimSpace(c.Query("min_pnl")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			filter.MinPnL = &parsed
		}
	}
	if raw := strings.TrimSpace(c.Query("max_pnl")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			filter.MaxPnL = &parsed
		}
	}
	return filter
}

func dealReviewFilterFromMap(raw map[string]any) store.DealReviewListFilter {
	if len(raw) == 0 {
		return store.DealReviewListFilter{}
	}
	body, _ := json.Marshal(raw)
	var filter store.DealReviewListFilter
	_ = json.Unmarshal(body, &filter)
	return filter
}

func normalizeDealReviewChallengerMode(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case store.DealReviewChallengerModePaper:
		return store.DealReviewChallengerModePaper
	case store.DealReviewChallengerModeIsolatedLive:
		return store.DealReviewChallengerModeIsolatedLive
	default:
		return store.DealReviewChallengerModeSharedLive
	}
}

func normalizeDealReviewCompareWindowHours(value int) int {
	switch value {
	case 12, 24, 36, 48, 96:
		return value
	default:
		return 24
	}
}

func validateDealReviewChallengerMode(mode, incumbentExchangeID string, exchangeCfg *store.Exchange) error {
	if exchangeCfg == nil {
		return fmt.Errorf("challenger wallet is required")
	}
	switch mode {
	case store.DealReviewChallengerModePaper:
		if !exchangeCfg.Testnet {
			return fmt.Errorf("paper / simulation mode currently requires a testnet exchange account")
		}
	case store.DealReviewChallengerModeIsolatedLive:
		if exchangeCfg.Testnet {
			return fmt.Errorf("isolated live mode requires a live wallet, not a testnet account")
		}
		if strings.TrimSpace(exchangeCfg.ID) == strings.TrimSpace(incumbentExchangeID) {
			return fmt.Errorf("isolated live mode requires a different wallet than the incumbent trader")
		}
	case store.DealReviewChallengerModeSharedLive:
		if exchangeCfg.Testnet {
			return fmt.Errorf("shared live mode requires a live wallet, not a testnet account")
		}
	default:
		return fmt.Errorf("unsupported challenger mode %q", mode)
	}
	return nil
}

func buildDealReviewValidationBlockMessage(validation *store.DealReviewAIScanValidation) string {
	if validation == nil {
		return "AI scan validation is required before this action."
	}
	if len(validation.BlockingIssues) > 0 {
		return strings.Join(validation.BlockingIssues, " ")
	}
	return "AI scan validation must pass before this action."
}

const (
	dealReviewTrainingProfitFactorFloor      = 0.65
	dealReviewHoldoutProfitFactorFloor       = 0.60
	dealReviewRecentProfitFactorFloor        = 0.60
	dealReviewTrainingMaxDrawdownCeiling     = 10.0
	dealReviewHoldoutMaxDrawdownCeiling      = 10.0
	dealReviewRecentMaxDrawdownCeiling       = 10.0
	dealReviewHoldoutWinRateDeltaFloor       = -10.0
	dealReviewHoldoutExpectancyPctDeltaFloor = -0.40
	dealReviewRecentExpectancyPctDeltaFloor  = -0.40
	dealReviewHoldoutNetPnLDeltaFloor        = -0.01
	dealReviewRecentNetPnLDeltaFloor         = -0.01
)

func marshalPrettyJSONMap(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		return pretty.String(), nil
	}
	return string(body), nil
}

func (s *Server) loadStrategyForTrader(userID string, traderCfg *store.Trader) (*store.StrategyConfig, *store.Strategy, error) {
	if traderCfg == nil || traderCfg.StrategyID == "" {
		return nil, nil, fmt.Errorf("trader has no strategy configured")
	}
	strategyRecord, err := s.store.Strategy().Get(userID, traderCfg.StrategyID)
	if err != nil {
		return nil, nil, err
	}
	strategyCfg, err := strategyRecord.ParseConfig()
	if err != nil {
		return nil, nil, err
	}
	return strategyCfg, strategyRecord, nil
}

func summarizeClosedDealReviewCases(cases []store.DealReviewCaseDetail) *store.DealReviewDatasetSummary {
	summary := &store.DealReviewDatasetSummary{}
	var grossProfit float64
	var grossLoss float64
	pnlPctSeries := make([]float64, 0, len(cases))

	for _, item := range cases {
		caseRec := item.Case
		if caseRec.Status != store.DealReviewCaseStatusClosed {
			continue
		}
		summary.TotalDeals++
		summary.ClosedDeals++
		summary.NetPnL += caseRec.RealizedPnL
		summary.AvgPnLPct += caseRec.RealizedPnLPct
		summary.AvgHoldMs += caseRec.HoldDurationMs
		if caseRec.Side == "LONG" {
			summary.LongDeals++
			summary.LongNetPnL += caseRec.RealizedPnL
		} else if caseRec.Side == "SHORT" {
			summary.ShortDeals++
			summary.ShortNetPnL += caseRec.RealizedPnL
		}
		pnlPctSeries = append(pnlPctSeries, caseRec.RealizedPnLPct)
		switch {
		case caseRec.RealizedPnL > 0:
			summary.WinningDeals++
			grossProfit += caseRec.RealizedPnL
		case caseRec.RealizedPnL < 0:
			summary.LosingDeals++
			grossLoss += math.Abs(caseRec.RealizedPnL)
		default:
			summary.FlatDeals++
		}
	}

	if summary.ClosedDeals > 0 {
		summary.AvgPnL = summary.NetPnL / float64(summary.ClosedDeals)
		summary.Expectancy = summary.AvgPnL
		summary.AvgPnLPct = summary.AvgPnLPct / float64(summary.ClosedDeals)
		summary.AvgHoldMs = int64(float64(summary.AvgHoldMs) / float64(summary.ClosedDeals))
		summary.WinRate = (float64(summary.WinningDeals) / float64(summary.ClosedDeals)) * 100
	}
	if grossLoss > 0 {
		summary.ProfitFactor = grossProfit / grossLoss
	} else if grossProfit > 0 {
		summary.ProfitFactor = grossProfit
	}
	summary.MaxDrawdown = calculateDealReviewMaxDrawdownPct(pnlPctSeries)

	return summary
}

func calculateDealReviewMaxDrawdownPct(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}

	const startingEquity = 100.0
	equity := startingEquity
	peak := startingEquity
	var maxDrawdown float64

	for _, pnl := range pnls {
		equity += pnl
		if equity > peak {
			peak = equity
		}
		if peak <= 0 {
			continue
		}
		drawdown := (peak - equity) / peak * 100
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return maxDrawdown
}

func sortClosedDealReviewCasesByExit(cases []store.DealReviewCaseDetail) {
	sort.Slice(cases, func(i, j int) bool {
		left := cases[i].Case.ExitTimeMs
		if left <= 0 {
			left = cases[i].Case.EntryTimeMs
		}
		right := cases[j].Case.ExitTimeMs
		if right <= 0 {
			right = cases[j].Case.EntryTimeMs
		}
		return left < right
	})
}

func dealReviewHoldoutSize(total int) int {
	if total <= 0 {
		return 0
	}
	holdout := total / 4
	if holdout < 3 {
		holdout = 3
	}
	if holdout >= total {
		holdout = total - 1
	}
	if holdout < 0 {
		return 0
	}
	return holdout
}

func dealReviewRecentLiveLikeSize(holdoutTotal int) int {
	if holdoutTotal <= 0 {
		return 0
	}
	recent := holdoutTotal / 2
	if recent < 3 {
		recent = 3
	}
	if recent > 8 {
		recent = 8
	}
	if recent > holdoutTotal {
		recent = holdoutTotal
	}
	return recent
}

type dealReviewReplayPatchProfile struct {
	SupportedPaths        []string
	UnsupportedPaths      []string
	SupportsConfidence    bool
	SupportsMinRiskReward bool
	SupportsMajorSizing   bool
	SupportsAltSizing     bool
	SupportsMinSize       bool
	SupportsExcludedCoins bool
	SupportsStaticCoins   bool
	SupportsMajorLeverage bool
	SupportsAltLeverage   bool
}

func buildDealReviewReplayPatchProfile(
	patch map[string]any,
	originalCfg *store.StrategyConfig,
	mergedCfg *store.StrategyConfig,
) *dealReviewReplayPatchProfile {
	profile := &dealReviewReplayPatchProfile{}
	if len(patch) == 0 {
		return profile
	}
	flat := map[string]string{}
	flattenJSONMap("", patch, flat)
	keys := make([]string, 0, len(flat))
	for key := range flat {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		switch key {
		case "risk_control.min_confidence":
			profile.SupportsConfidence = true
			profile.SupportedPaths = append(profile.SupportedPaths, key)
		case "risk_control.min_risk_reward_ratio":
			profile.SupportsMinRiskReward = true
			profile.SupportedPaths = append(profile.SupportedPaths, key)
		case "risk_control.btc_eth_max_position_value_ratio":
			profile.SupportsMajorSizing = true
			profile.SupportedPaths = append(profile.SupportedPaths, key)
		case "risk_control.altcoin_max_position_value_ratio":
			profile.SupportsAltSizing = true
			profile.SupportedPaths = append(profile.SupportedPaths, key)
		case "risk_control.min_position_size":
			profile.SupportsMinSize = true
			profile.SupportedPaths = append(profile.SupportedPaths, key)
		case "risk_control.btc_eth_max_leverage":
			profile.SupportsMajorLeverage = true
			profile.SupportedPaths = append(profile.SupportedPaths, key)
		case "risk_control.altcoin_max_leverage":
			profile.SupportsAltLeverage = true
			profile.SupportedPaths = append(profile.SupportedPaths, key)
		case "coin_source.excluded_coins":
			profile.SupportsExcludedCoins = true
			profile.SupportedPaths = append(profile.SupportedPaths, key)
		case "coin_source.static_coins", "coin_source.source_type":
			if mergedCfg != nil &&
				strings.EqualFold(strings.TrimSpace(mergedCfg.CoinSource.SourceType), "static") &&
				len(mergedCfg.CoinSource.StaticCoins) > 0 {
				profile.SupportsStaticCoins = true
				profile.SupportedPaths = append(profile.SupportedPaths, key)
				continue
			}
			if originalCfg != nil &&
				strings.EqualFold(strings.TrimSpace(originalCfg.CoinSource.SourceType), "static") &&
				len(originalCfg.CoinSource.StaticCoins) > 0 &&
				key == "coin_source.static_coins" {
				profile.SupportsStaticCoins = true
				profile.SupportedPaths = append(profile.SupportedPaths, key)
				continue
			}
			profile.UnsupportedPaths = append(profile.UnsupportedPaths, key)
		default:
			profile.UnsupportedPaths = append(profile.UnsupportedPaths, key)
		}
	}

	return profile
}

func buildDealReviewSymbolSet(symbols []string) map[string]struct{} {
	result := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		normalized := strings.ToUpper(strings.TrimSpace(symbol))
		if normalized == "" {
			continue
		}
		result[normalized] = struct{}{}
	}
	return result
}

func isDealReviewMajorSymbol(symbol string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(symbol))
	return strings.HasPrefix(normalized, "BTC") || strings.HasPrefix(normalized, "ETH")
}

func calcDealReviewRiskRewardRatio(caseRec store.DealReviewCase) float64 {
	if caseRec.EntryPrice <= 0 || caseRec.OpenStopLoss <= 0 || caseRec.OpenTakeProfit <= 0 {
		return 0
	}

	side := strings.ToUpper(strings.TrimSpace(caseRec.Side))
	var riskPct float64
	var rewardPct float64
	switch side {
	case "SHORT":
		riskPct = ((caseRec.OpenStopLoss - caseRec.EntryPrice) / caseRec.EntryPrice) * 100
		rewardPct = ((caseRec.EntryPrice - caseRec.OpenTakeProfit) / caseRec.EntryPrice) * 100
	default:
		riskPct = ((caseRec.EntryPrice - caseRec.OpenStopLoss) / caseRec.EntryPrice) * 100
		rewardPct = ((caseRec.OpenTakeProfit - caseRec.EntryPrice) / caseRec.EntryPrice) * 100
	}
	if riskPct <= 0 || rewardPct <= 0 {
		return 0
	}
	return rewardPct / riskPct
}

func safeDealReviewRatio(next, current float64) float64 {
	if current <= 0 || next <= 0 {
		return 1
	}
	return next / current
}

func simulateDealReviewReplayCase(
	item store.DealReviewCaseDetail,
	originalCfg *store.StrategyConfig,
	mergedCfg *store.StrategyConfig,
	profile *dealReviewReplayPatchProfile,
) (store.DealReviewCaseDetail, bool) {
	if originalCfg == nil || mergedCfg == nil || profile == nil {
		return item, true
	}

	caseRec := item.Case
	symbol := strings.ToUpper(strings.TrimSpace(caseRec.Symbol))
	if profile.SupportsExcludedCoins {
		excludedSymbols := buildDealReviewSymbolSet(mergedCfg.CoinSource.ExcludedCoins)
		if _, blocked := excludedSymbols[symbol]; blocked {
			return item, false
		}
	}

	if profile.SupportsStaticCoins && strings.EqualFold(strings.TrimSpace(mergedCfg.CoinSource.SourceType), "static") {
		allowedSymbols := buildDealReviewSymbolSet(mergedCfg.CoinSource.StaticCoins)
		if len(allowedSymbols) == 0 {
			return item, true
		}
		if _, allowed := allowedSymbols[symbol]; !allowed {
			return item, false
		}
	}

	if profile.SupportsConfidence && caseRec.OpenConfidence > 0 &&
		caseRec.OpenConfidence < mergedCfg.RiskControl.MinConfidence {
		return item, false
	}

	if profile.SupportsMinRiskReward {
		if ratio := calcDealReviewRiskRewardRatio(caseRec); ratio > 0 &&
			ratio < mergedCfg.RiskControl.MinRiskRewardRatio {
			return item, false
		}
	}

	scale := 1.0
	if isDealReviewMajorSymbol(caseRec.Symbol) {
		if profile.SupportsMajorSizing {
			scale *= safeDealReviewRatio(
				mergedCfg.RiskControl.BTCETHMaxPositionValueRatio,
				originalCfg.RiskControl.BTCETHMaxPositionValueRatio,
			)
		}
		if profile.SupportsMajorLeverage &&
			caseRec.Leverage > 0 &&
			mergedCfg.RiskControl.BTCETHMaxLeverage > 0 &&
			caseRec.Leverage > mergedCfg.RiskControl.BTCETHMaxLeverage {
			scale *= float64(mergedCfg.RiskControl.BTCETHMaxLeverage) / float64(caseRec.Leverage)
		}
	} else {
		if profile.SupportsAltSizing {
			scale *= safeDealReviewRatio(
				mergedCfg.RiskControl.AltcoinMaxPositionValueRatio,
				originalCfg.RiskControl.AltcoinMaxPositionValueRatio,
			)
		}
		if profile.SupportsAltLeverage &&
			caseRec.Leverage > 0 &&
			mergedCfg.RiskControl.AltcoinMaxLeverage > 0 &&
			caseRec.Leverage > mergedCfg.RiskControl.AltcoinMaxLeverage {
			scale *= float64(mergedCfg.RiskControl.AltcoinMaxLeverage) / float64(caseRec.Leverage)
		}
	}

	entryValue := math.Abs(caseRec.EntryQuantity) * caseRec.EntryPrice * scale
	if profile.SupportsMinSize && mergedCfg.RiskControl.MinPositionSize > 0 &&
		entryValue > 0 && entryValue < mergedCfg.RiskControl.MinPositionSize {
		return item, false
	}

	if !math.IsNaN(scale) && !math.IsInf(scale, 0) && scale > 0 && scale != 1 {
		caseRec.RealizedPnL *= scale
		caseRec.Fee *= math.Abs(scale)
	}

	item.Case = caseRec
	return item, true
}

func simulateDealReviewReplayDataset(
	originalCfg *store.StrategyConfig,
	mergedCfg *store.StrategyConfig,
	cases []store.DealReviewCaseDetail,
	profile *dealReviewReplayPatchProfile,
) []store.DealReviewCaseDetail {
	result := make([]store.DealReviewCaseDetail, 0, len(cases))
	for _, item := range cases {
		simulated, include := simulateDealReviewReplayCase(item, originalCfg, mergedCfg, profile)
		if include {
			result = append(result, simulated)
		}
	}
	return result
}

func buildDealReviewReplayValidation(
	originalCfg *store.StrategyConfig,
	mergedCfg *store.StrategyConfig,
	patch map[string]any,
	trainingCases []store.DealReviewCaseDetail,
	holdoutCases []store.DealReviewCaseDetail,
	recentCases []store.DealReviewCaseDetail,
	recentSliceLabel string,
	minRecentClosedDealCount int,
) *store.DealReviewAIScanReplay {
	profile := buildDealReviewReplayPatchProfile(patch, originalCfg, mergedCfg)
	replay := &store.DealReviewAIScanReplay{
		Supported:        len(profile.SupportedPaths) > 0,
		Mode:             "historical_cohort_replay",
		SupportedPaths:   append([]string(nil), profile.SupportedPaths...),
		UnsupportedPaths: append([]string(nil), profile.UnsupportedPaths...),
	}

	if !replay.Supported {
		replay.Notes = append(replay.Notes,
			"Replay is not available for this patch yet because it only changes fields outside the current supported risk/cohort subset.",
		)
		return replay
	}

	if len(profile.UnsupportedPaths) > 0 {
		replay.Notes = append(replay.Notes,
			"Replay coverage is partial. Unsupported patch paths still require live challenger evidence.",
		)
	}
	if profile.SupportsMajorLeverage || profile.SupportsAltLeverage {
		replay.Notes = append(replay.Notes,
			"Leverage replay is conservative: lower leverage caps can downscale historical trades, but higher leverage caps do not upscale them.",
		)
	}
	if profile.SupportsExcludedCoins || profile.SupportsStaticCoins {
		replay.Notes = append(replay.Notes,
			"Symbol-source replay currently supports restrictive filters such as excluded coins and switching into a static coin whitelist.",
		)
	}

	trainingReplayCases := simulateDealReviewReplayDataset(originalCfg, mergedCfg, trainingCases, profile)
	holdoutReplayCases := simulateDealReviewReplayDataset(originalCfg, mergedCfg, holdoutCases, profile)
	recentReplayCases := simulateDealReviewReplayDataset(originalCfg, mergedCfg, recentCases, profile)
	trainingReplaySummary := summarizeClosedDealReviewCases(trainingReplayCases)
	holdoutReplaySummary := summarizeClosedDealReviewCases(holdoutReplayCases)
	recentReplaySummary := summarizeClosedDealReviewCases(recentReplayCases)

	replay.TrainingReplaySummary = trainingReplaySummary
	replay.HoldoutReplaySummary = holdoutReplaySummary
	replay.RecentReplaySummary = recentReplaySummary
	if baseline := summarizeClosedDealReviewCases(trainingCases); baseline != nil {
		replay.TrainingNetPnLDelta = trainingReplaySummary.NetPnL - baseline.NetPnL
		replay.TrainingClosedDealDelta = int(trainingReplaySummary.ClosedDeals - baseline.ClosedDeals)
	}
	if baseline := summarizeClosedDealReviewCases(holdoutCases); baseline != nil {
		replay.HoldoutNetPnLDelta = holdoutReplaySummary.NetPnL - baseline.NetPnL
		replay.HoldoutClosedDealDelta = int(holdoutReplaySummary.ClosedDeals - baseline.ClosedDeals)
	}
	if baseline := summarizeClosedDealReviewCases(recentCases); baseline != nil {
		replay.RecentNetPnLDelta = recentReplaySummary.NetPnL - baseline.NetPnL
		replay.RecentClosedDealDelta = int(recentReplaySummary.ClosedDeals - baseline.ClosedDeals)
	}

	replay.Notes = append(replay.Notes,
		"Replay re-evaluates historical deals only for supported risk/cohort constraints such as confidence thresholds, risk-reward floors, minimum size, and position-value caps.",
	)
	if replay.HoldoutNetPnLDelta < -0.01 {
		replay.BlockingIssues = append(replay.BlockingIssues,
			fmt.Sprintf("Historical replay projects holdout net PnL degradation of %s on the current filtered cohort.", formatSignedFloat(replay.HoldoutNetPnLDelta)),
		)
	}
	if len(recentCases) >= minRecentClosedDealCount && replay.RecentNetPnLDelta < -0.01 {
		label := strings.TrimSpace(recentSliceLabel)
		if label == "" {
			label = fmt.Sprintf("latest %d closed deals", len(recentCases))
		}
		replay.BlockingIssues = append(replay.BlockingIssues,
			fmt.Sprintf("Historical replay projects recent live-like net PnL degradation of %s across %s.", formatSignedFloat(replay.RecentNetPnLDelta), label),
		)
	}
	if len(recentCases) > 0 {
		label := strings.TrimSpace(recentSliceLabel)
		if label == "" {
			label = fmt.Sprintf("latest %d closed deals", len(recentCases))
		}
		replay.Notes = append(replay.Notes,
			fmt.Sprintf("Recent live-like replay is evaluated on %s as the freshest proxy for current market behavior.", label),
		)
	}

	return replay
}

func appendDealReviewValidationBlockingIssue(validation *store.DealReviewAIScanValidation, message string) {
	if validation == nil {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	for _, existing := range validation.BlockingIssues {
		if existing == message {
			return
		}
	}
	validation.BlockingIssues = append(validation.BlockingIssues, message)
}

func appendDealReviewValidationCheck(validation *store.DealReviewAIScanValidation, check store.DealReviewValidationCheck) {
	if validation == nil {
		return
	}
	check.Message = strings.TrimSpace(check.Message)
	validation.Checks = append(validation.Checks, check)
	if !check.Passed && check.Blocking && check.Message != "" {
		appendDealReviewValidationBlockingIssue(validation, check.Message)
	}
}

func addDealReviewValidationCountCheck(
	validation *store.DealReviewAIScanValidation,
	key string,
	label string,
	scope string,
	actual int,
	threshold int,
	message string,
) {
	appendDealReviewValidationCheck(validation, store.DealReviewValidationCheck{
		Key:        key,
		Label:      label,
		Scope:      scope,
		Metric:     "closed_deal_count",
		Source:     "baseline",
		Comparator: "gte",
		Threshold:  float64(threshold),
		Actual:     float64(actual),
		Passed:     actual >= threshold,
		Blocking:   true,
		Message:    message,
	})
}

func addDealReviewValidationThresholdCheck(
	validation *store.DealReviewAIScanValidation,
	key string,
	label string,
	scope string,
	metric string,
	source string,
	comparator string,
	actual float64,
	threshold float64,
	message string,
) {
	passed := false
	switch comparator {
	case "lte":
		passed = actual <= threshold
	default:
		passed = actual >= threshold
	}
	appendDealReviewValidationCheck(validation, store.DealReviewValidationCheck{
		Key:        key,
		Label:      label,
		Scope:      scope,
		Metric:     metric,
		Source:     source,
		Comparator: comparator,
		Threshold:  threshold,
		Actual:     actual,
		Passed:     passed,
		Blocking:   true,
		Message:    message,
	})
}

func addDealReviewValidationDeltaCheck(
	validation *store.DealReviewAIScanValidation,
	key string,
	label string,
	scope string,
	metric string,
	source string,
	actual float64,
	baseline float64,
	threshold float64,
	message string,
) {
	delta := actual - baseline
	appendDealReviewValidationCheck(validation, store.DealReviewValidationCheck{
		Key:        key,
		Label:      label,
		Scope:      scope,
		Metric:     metric,
		Source:     source,
		Comparator: "delta_gte",
		Threshold:  threshold,
		Actual:     actual,
		Baseline:   baseline,
		Delta:      delta,
		Passed:     delta >= threshold,
		Blocking:   true,
		Message:    message,
	})
}

func valueOrZero(summary *store.DealReviewDatasetSummary, selector func(*store.DealReviewDatasetSummary) float64) float64 {
	if summary == nil || selector == nil {
		return 0
	}
	return selector(summary)
}

func applyDealReviewValidationMetricGates(validation *store.DealReviewAIScanValidation) {
	if validation == nil {
		return
	}

	addDealReviewValidationCountCheck(
		validation,
		"dataset_min_closed_deals",
		"Minimum filtered closed deals",
		"dataset",
		validation.ClosedDealCount,
		validation.MinClosedDealCount,
		fmt.Sprintf("At least %d closed deals are required before apply or challenger launch; only %d are available in this filtered dataset.",
			validation.MinClosedDealCount,
			validation.ClosedDealCount,
		),
	)
	addDealReviewValidationCountCheck(
		validation,
		"training_min_closed_deals",
		"Minimum training closed deals",
		"training",
		validation.TrainingClosedDealCount,
		5,
		fmt.Sprintf("Training slice is too small for evidence review; only %d closed deals remain before the holdout window.", validation.TrainingClosedDealCount),
	)
	addDealReviewValidationCountCheck(
		validation,
		"holdout_min_closed_deals",
		"Minimum holdout closed deals",
		"holdout",
		validation.HoldoutClosedDealCount,
		validation.MinHoldoutClosedDealCount,
		fmt.Sprintf("At least %d holdout deals are required; only %d are available after time-based split.",
			validation.MinHoldoutClosedDealCount,
			validation.HoldoutClosedDealCount,
		),
	)

	if validation.Replay == nil || !validation.Replay.Supported {
		validation.Notes = append(validation.Notes,
			"Advanced metric gates are skipped when the patch falls outside current replay coverage. Sample-size and config gates still apply; challenger evidence is required for unsupported paths.",
		)
		return
	}

	trainingProjected := validation.Replay.TrainingReplaySummary
	holdoutProjected := validation.Replay.HoldoutReplaySummary
	recentProjected := validation.Replay.RecentReplaySummary

	if trainingProjected != nil && validation.TrainingClosedDealCount > 0 {
		addDealReviewValidationThresholdCheck(
			validation,
			"training_profit_factor_floor",
			"Training profit factor floor",
			"training",
			"profit_factor",
			"replay",
			"gte",
			trainingProjected.ProfitFactor,
			dealReviewTrainingProfitFactorFloor,
			fmt.Sprintf("Projected training profit factor %.2f is below the floor of %.2f.", trainingProjected.ProfitFactor, dealReviewTrainingProfitFactorFloor),
		)
		addDealReviewValidationThresholdCheck(
			validation,
			"training_max_drawdown_ceiling",
			"Training max drawdown ceiling",
			"training",
			"max_drawdown_pct",
			"replay",
			"lte",
			trainingProjected.MaxDrawdown,
			dealReviewTrainingMaxDrawdownCeiling,
			fmt.Sprintf("Projected training max drawdown %.2f%% exceeds the ceiling of %.2f%%.", trainingProjected.MaxDrawdown, dealReviewTrainingMaxDrawdownCeiling),
		)
	}

	if holdoutProjected != nil && validation.HoldoutClosedDealCount > 0 {
		addDealReviewValidationThresholdCheck(
			validation,
			"holdout_profit_factor_floor",
			"Holdout profit factor floor",
			"holdout",
			"profit_factor",
			"replay",
			"gte",
			holdoutProjected.ProfitFactor,
			dealReviewHoldoutProfitFactorFloor,
			fmt.Sprintf("Projected holdout profit factor %.2f is below the floor of %.2f.", holdoutProjected.ProfitFactor, dealReviewHoldoutProfitFactorFloor),
		)
		addDealReviewValidationThresholdCheck(
			validation,
			"holdout_max_drawdown_ceiling",
			"Holdout max drawdown ceiling",
			"holdout",
			"max_drawdown_pct",
			"replay",
			"lte",
			holdoutProjected.MaxDrawdown,
			dealReviewHoldoutMaxDrawdownCeiling,
			fmt.Sprintf("Projected holdout max drawdown %.2f%% exceeds the ceiling of %.2f%%.", holdoutProjected.MaxDrawdown, dealReviewHoldoutMaxDrawdownCeiling),
		)
		if validation.HoldoutSummary != nil {
			winRateDelta := holdoutProjected.WinRate - validation.HoldoutSummary.WinRate
			addDealReviewValidationDeltaCheck(
				validation,
				"holdout_win_rate_delta",
				"Holdout win-rate delta",
				"holdout",
				"win_rate_delta",
				"replay",
				holdoutProjected.WinRate,
				validation.HoldoutSummary.WinRate,
				dealReviewHoldoutWinRateDeltaFloor,
				fmt.Sprintf("Projected holdout win-rate delta %.2f pts is below the floor of %.2f pts.", winRateDelta, dealReviewHoldoutWinRateDeltaFloor),
			)
			expectancyPctDelta := holdoutProjected.AvgPnLPct - validation.HoldoutSummary.AvgPnLPct
			addDealReviewValidationDeltaCheck(
				validation,
				"holdout_expectancy_pct_delta",
				"Holdout expectancy % delta",
				"holdout",
				"avg_pnl_pct_delta",
				"replay",
				holdoutProjected.AvgPnLPct,
				validation.HoldoutSummary.AvgPnLPct,
				dealReviewHoldoutExpectancyPctDeltaFloor,
				fmt.Sprintf("Projected holdout expectancy delta %.2f pts is below the floor of %.2f pts.", expectancyPctDelta, dealReviewHoldoutExpectancyPctDeltaFloor),
			)
		}
		baselineHoldoutNetPnL := valueOrZero(validation.HoldoutSummary, func(summary *store.DealReviewDatasetSummary) float64 {
			return summary.NetPnL
		})
		netPnLDelta := holdoutProjected.NetPnL - baselineHoldoutNetPnL
		addDealReviewValidationDeltaCheck(
			validation,
			"holdout_net_pnl_delta",
			"No severe holdout net PnL degradation",
			"holdout",
			"net_pnl_delta",
			"replay",
			holdoutProjected.NetPnL,
			baselineHoldoutNetPnL,
			dealReviewHoldoutNetPnLDeltaFloor,
			fmt.Sprintf("Projected holdout net PnL delta %s is below the floor of %s.", formatSignedFloat(netPnLDelta), formatSignedFloat(dealReviewHoldoutNetPnLDeltaFloor)),
		)
	}

	if recentProjected != nil && validation.RecentClosedDealCount >= validation.MinRecentClosedDealCount {
		addDealReviewValidationThresholdCheck(
			validation,
			"recent_profit_factor_floor",
			"Recent live-like profit factor floor",
			"recent",
			"profit_factor",
			"replay",
			"gte",
			recentProjected.ProfitFactor,
			dealReviewRecentProfitFactorFloor,
			fmt.Sprintf("Projected recent live-like profit factor %.2f is below the floor of %.2f.", recentProjected.ProfitFactor, dealReviewRecentProfitFactorFloor),
		)
		addDealReviewValidationThresholdCheck(
			validation,
			"recent_max_drawdown_ceiling",
			"Recent live-like max drawdown ceiling",
			"recent",
			"max_drawdown_pct",
			"replay",
			"lte",
			recentProjected.MaxDrawdown,
			dealReviewRecentMaxDrawdownCeiling,
			fmt.Sprintf("Projected recent live-like max drawdown %.2f%% exceeds the ceiling of %.2f%%.", recentProjected.MaxDrawdown, dealReviewRecentMaxDrawdownCeiling),
		)
		if validation.RecentSummary != nil {
			recentExpectancyPctDelta := recentProjected.AvgPnLPct - validation.RecentSummary.AvgPnLPct
			addDealReviewValidationDeltaCheck(
				validation,
				"recent_expectancy_pct_delta",
				"Recent live-like expectancy % delta",
				"recent",
				"avg_pnl_pct_delta",
				"replay",
				recentProjected.AvgPnLPct,
				validation.RecentSummary.AvgPnLPct,
				dealReviewRecentExpectancyPctDeltaFloor,
				fmt.Sprintf("Projected recent live-like expectancy delta %.2f pts is below the floor of %.2f pts.", recentExpectancyPctDelta, dealReviewRecentExpectancyPctDeltaFloor),
			)
		}
		baselineRecentNetPnL := valueOrZero(validation.RecentSummary, func(summary *store.DealReviewDatasetSummary) float64 {
			return summary.NetPnL
		})
		recentNetPnLDelta := recentProjected.NetPnL - baselineRecentNetPnL
		addDealReviewValidationDeltaCheck(
			validation,
			"recent_net_pnl_delta",
			"Recent live-like net PnL degradation",
			"recent",
			"net_pnl_delta",
			"replay",
			recentProjected.NetPnL,
			baselineRecentNetPnL,
			dealReviewRecentNetPnLDeltaFloor,
			fmt.Sprintf("Projected recent live-like net PnL delta %s is below the floor of %s.", formatSignedFloat(recentNetPnLDelta), formatSignedFloat(dealReviewRecentNetPnLDeltaFloor)),
		)
	}
}

func (s *Server) validateDealReviewAIScan(userID string, traderCfg *store.Trader, scanDetail *store.DealReviewAIScanDetail) (*store.DealReviewAIScanValidation, string, *store.Strategy, error) {
	validation := &store.DealReviewAIScanValidation{
		Status:                    store.DealReviewAIScanValidationFailed,
		ValidatedAt:               time.Now().UTC(),
		MinClosedDealCount:        10,
		MinHoldoutClosedDealCount: 3,
		MinRecentClosedDealCount:  3,
	}

	if scanDetail == nil {
		validation.BlockingIssues = append(validation.BlockingIssues, "AI scan could not be loaded for validation.")
		return validation, "", nil, nil
	}
	if traderCfg == nil {
		validation.BlockingIssues = append(validation.BlockingIssues, "Trader config is unavailable for validation.")
		return validation, "", nil, nil
	}
	if len(scanDetail.StrategyPatch) == 0 {
		validation.BlockingIssues = append(validation.BlockingIssues, "This AI scan does not contain a strategy patch to validate.")
		return validation, "", nil, nil
	}

	originalCfg, strategyRecord, err := s.loadStrategyForTrader(userID, traderCfg)
	if err != nil {
		validation.BlockingIssues = append(validation.BlockingIssues, "Active trader strategy could not be loaded.")
		return validation, "", nil, err
	}

	mergedConfigJSON, err := applyStrategyPatch(strategyRecord.Config, scanDetail.StrategyPatch)
	if err != nil {
		validation.BlockingIssues = append(validation.BlockingIssues, "Strategy patch could not be merged with the incumbent config.")
		return validation, "", strategyRecord, nil
	}

	var mergedConfig store.StrategyConfig
	if err := json.Unmarshal([]byte(mergedConfigJSON), &mergedConfig); err != nil {
		validation.BlockingIssues = append(validation.BlockingIssues, "Merged strategy patch is not valid JSON config.")
		return validation, mergedConfigJSON, strategyRecord, nil
	}
	if warnings, err := validateStrategyConfig(&mergedConfig); err != nil {
		validation.BlockingIssues = append(validation.BlockingIssues, err.Error())
	} else {
		validation.ConfigValid = true
		validation.ConfigWarnings = append(validation.ConfigWarnings, warnings...)
	}

	filter := dealReviewFilterFromMap(scanDetail.Filters)
	filter.TraderID = traderCfg.ID
	cases, _, err := s.store.DealReview().ListAnalysisCases(userID, filter, 200)
	if err != nil {
		return validation, mergedConfigJSON, strategyRecord, err
	}
	validation.DatasetCount = len(cases)

	closedCases := make([]store.DealReviewCaseDetail, 0, len(cases))
	for _, item := range cases {
		if item.Case.Status == store.DealReviewCaseStatusClosed {
			closedCases = append(closedCases, item)
		}
	}
	sortClosedDealReviewCasesByExit(closedCases)

	validation.ClosedDealCount = len(closedCases)
	holdoutSize := dealReviewHoldoutSize(len(closedCases))
	trainingEnd := len(closedCases) - holdoutSize
	if trainingEnd < 0 {
		trainingEnd = 0
	}

	trainingCases := append([]store.DealReviewCaseDetail(nil), closedCases[:trainingEnd]...)
	holdoutCases := append([]store.DealReviewCaseDetail(nil), closedCases[trainingEnd:]...)
	recentSize := dealReviewRecentLiveLikeSize(len(holdoutCases))
	recentStart := len(holdoutCases) - recentSize
	if recentStart < 0 {
		recentStart = 0
	}
	recentCases := append([]store.DealReviewCaseDetail(nil), holdoutCases[recentStart:]...)
	validation.TrainingClosedDealCount = len(trainingCases)
	validation.HoldoutClosedDealCount = len(holdoutCases)
	validation.RecentClosedDealCount = len(recentCases)
	if len(recentCases) > 0 {
		validation.RecentSliceLabel = fmt.Sprintf("latest %d holdout deals", len(recentCases))
	}
	if len(trainingCases) > 0 {
		validation.TrainingSummary = summarizeClosedDealReviewCases(trainingCases)
	}
	if len(holdoutCases) > 0 {
		validation.HoldoutSummary = summarizeClosedDealReviewCases(holdoutCases)
	}
	if len(recentCases) > 0 {
		validation.RecentSummary = summarizeClosedDealReviewCases(recentCases)
	}
	validation.Replay = buildDealReviewReplayValidation(
		originalCfg,
		&mergedConfig,
		scanDetail.StrategyPatch,
		trainingCases,
		holdoutCases,
		recentCases,
		validation.RecentSliceLabel,
		validation.MinRecentClosedDealCount,
	)
	if validation.Replay != nil {
		validation.Notes = append(validation.Notes, validation.Replay.Notes...)
	}
	applyDealReviewValidationMetricGates(validation)

	validation.Notes = append(validation.Notes,
		"This validation combines a config/evidence gate with a limited historical replay where the patch touches supported risk/cohort fields. Final proof still comes from the timed challenger compare.",
		"Recent live-like validation uses the newest subset of holdout deals as a proxy for the market regime the challenger would face first.",
	)
	if len(validation.ConfigWarnings) > 0 {
		validation.Notes = append(validation.Notes, "Strategy config warnings were detected but are not blocking by themselves.")
	}

	if len(validation.BlockingIssues) == 0 && validation.ConfigValid {
		validation.Status = store.DealReviewAIScanValidationPassed
		validation.PromotionReady = true
	}

	return validation, mergedConfigJSON, strategyRecord, nil
}

func (s *Server) resolveAIScanModel(userID, requestedModelID, overrideModelName string) (*store.AIModel, string, error) {
	if requestedModelID != "" {
		modelCfg, err := s.store.AIModel().Get(userID, requestedModelID)
		if err != nil {
			return nil, "", err
		}
		if !modelCfg.Enabled {
			return nil, "", fmt.Errorf("AI model %s is disabled", modelCfg.Name)
		}
		if strings.TrimSpace(string(modelCfg.APIKey)) == "" {
			return nil, "", fmt.Errorf("AI model %s has no API key configured", modelCfg.Name)
		}
		modelName := strings.TrimSpace(overrideModelName)
		if modelName == "" {
			modelName = strings.TrimSpace(modelCfg.CustomModelName)
		}
		if modelName == "" {
			modelName = defaultModelForProvider(modelCfg.Provider)
		}
		return modelCfg, modelName, nil
	}

	models, err := s.store.AIModel().List(userID)
	if err != nil {
		return nil, "", err
	}
	var fallback *store.AIModel
	for _, model := range models {
		if !model.Enabled || strings.TrimSpace(string(model.APIKey)) == "" {
			continue
		}
		if model.Provider == "openai" {
			modelName := strings.TrimSpace(overrideModelName)
			if modelName == "" {
				modelName = strings.TrimSpace(model.CustomModelName)
			}
			if modelName == "" {
				modelName = defaultModelForProvider(model.Provider)
			}
			return model, modelName, nil
		}
		if fallback == nil {
			fallback = model
		}
	}
	if fallback == nil {
		return nil, "", fmt.Errorf("no enabled AI model with credentials found")
	}
	modelName := strings.TrimSpace(overrideModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(fallback.CustomModelName)
	}
	if modelName == "" {
		modelName = defaultModelForProvider(fallback.Provider)
	}
	return fallback, modelName, nil
}

func buildDealReviewChallengerTraderName(name string, createdAt time.Time) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = "Trader"
	}
	return fmt.Sprintf("%s Challenger %s", base, createdAt.Local().Format("01-02 15:04"))
}

func buildDealReviewChallengerStrategyName(name string, createdAt time.Time) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = "Strategy"
	}
	return fmt.Sprintf("%s Challenger %s", base, createdAt.Local().Format("01-02 15:04"))
}

func cloneDealReviewChallengerTrader(incumbent *store.Trader, traderID, strategyID, exchangeID string, createdAt time.Time) *store.Trader {
	return &store.Trader{
		ID:                   traderID,
		UserID:               incumbent.UserID,
		Name:                 buildDealReviewChallengerTraderName(incumbent.Name, createdAt),
		AIModelID:            incumbent.AIModelID,
		ExchangeID:           exchangeID,
		StrategyID:           strategyID,
		InitialBalance:       incumbent.InitialBalance,
		ScanIntervalMinutes:  incumbent.ScanIntervalMinutes,
		IsRunning:            false,
		InvertSignals:        incumbent.InvertSignals,
		IsCrossMargin:        incumbent.IsCrossMargin,
		ShowInCompetition:    false,
		BTCETHLeverage:       incumbent.BTCETHLeverage,
		AltcoinLeverage:      incumbent.AltcoinLeverage,
		TradingSymbols:       incumbent.TradingSymbols,
		UseAI500:             incumbent.UseAI500,
		UseOITop:             incumbent.UseOITop,
		CustomPrompt:         incumbent.CustomPrompt,
		OverrideBasePrompt:   incumbent.OverrideBasePrompt,
		SystemPromptTemplate: incumbent.SystemPromptTemplate,
	}
}

func (s *Server) ensureDealReviewTraderRunning(userID, traderID string) error {
	traderRuntime, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		if loadErr := s.traderManager.LoadUserTradersFromStore(s.store, userID); loadErr != nil {
			return loadErr
		}
		traderRuntime, err = s.traderManager.GetTrader(traderID)
		if err != nil {
			return err
		}
	}
	status := traderRuntime.GetStatus()
	if running, ok := status["is_running"].(bool); ok && running {
		return s.store.Trader().UpdateStatus(userID, traderID, true)
	}
	go func() {
		if runErr := traderRuntime.Run(); runErr != nil {
			logger.Infof("❌ Trader %s runtime error during challenger compare start: %v", traderID, runErr)
		}
	}()
	return s.store.Trader().UpdateStatus(userID, traderID, true)
}

func (s *Server) stopDealReviewTrader(userID, traderID string) error {
	if traderID == "" {
		return nil
	}
	if traderRuntime, err := s.traderManager.GetTrader(traderID); err == nil {
		status := traderRuntime.GetStatus()
		if running, ok := status["is_running"].(bool); ok && running {
			traderRuntime.Stop()
		}
	}
	return s.store.Trader().UpdateStatus(userID, traderID, false)
}

func (s *Server) computeDealReviewChallengerMetrics(compare *store.DealReviewChallengerCompare) (*store.DealReviewChallengerMetrics, error) {
	if compare == nil {
		return nil, fmt.Errorf("challenger compare cannot be nil")
	}
	evaluatedAt := time.Now().UTC()
	if !compare.EndsAt.IsZero() && evaluatedAt.After(compare.EndsAt) {
		evaluatedAt = compare.EndsAt.UTC()
	}
	if !compare.StartedAt.IsZero() && evaluatedAt.Before(compare.StartedAt) {
		evaluatedAt = compare.StartedAt.UTC()
	}
	startMs := compare.StartedAt.UTC().UnixMilli()
	endMs := evaluatedAt.UnixMilli()

	incumbentWindow, err := s.store.DealReview().GetTraderWindowPnL(compare.IncumbentTraderID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	challengerWindow, err := s.store.DealReview().GetTraderWindowPnL(compare.ChallengerTraderID, startMs, endMs)
	if err != nil {
		return nil, err
	}

	return &store.DealReviewChallengerMetrics{
		EvaluatedAt:          evaluatedAt,
		IncumbentPnL:         incumbentWindow.RealizedPnL,
		ChallengerPnL:        challengerWindow.RealizedPnL,
		IncumbentTradeCount:  int(incumbentWindow.ClosedCount),
		ChallengerTradeCount: int(challengerWindow.ClosedCount),
		IncumbentFees:        incumbentWindow.TotalFees,
		ChallengerFees:       challengerWindow.TotalFees,
		IncumbentAvgHoldMs:   int64(incumbentWindow.AvgHoldMs),
		ChallengerAvgHoldMs:  int64(challengerWindow.AvgHoldMs),
	}, nil
}

func buildDealReviewCompareRunningSummary(compare *store.DealReviewChallengerCompare, metrics *store.DealReviewChallengerMetrics) string {
	if compare == nil {
		return ""
	}
	if metrics == nil {
		return fmt.Sprintf("Challenger compare is running until %s.", compare.EndsAt.Local().Format(time.RFC3339))
	}
	return fmt.Sprintf(
		"Running: incumbent %s vs challenger %s through %s.",
		formatSignedFloat(metrics.IncumbentPnL),
		formatSignedFloat(metrics.ChallengerPnL),
		compare.EndsAt.Local().Format(time.RFC3339),
	)
}

func formatSignedFloat(value float64) string {
	if value >= 0 {
		return fmt.Sprintf("+%.2f", value)
	}
	return fmt.Sprintf("%.2f", value)
}

func isDealReviewChallengerCompareActive(status string) bool {
	switch strings.TrimSpace(status) {
	case store.DealReviewChallengerStatusStarting, store.DealReviewChallengerStatusRunning:
		return true
	default:
		return false
	}
}

func parseDealReviewChallengerMetrics(raw string) *store.DealReviewChallengerMetrics {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return nil
	}
	metrics := &store.DealReviewChallengerMetrics{}
	if err := json.Unmarshal([]byte(trimmed), metrics); err != nil {
		return nil
	}
	return metrics
}

func challengerMetricChanged(left, right float64) bool {
	return math.Abs(left-right) > 1e-9
}

func didDealReviewChallengerMetricsChange(previous, next *store.DealReviewChallengerMetrics) bool {
	if previous == nil || next == nil {
		return previous != next
	}
	return challengerMetricChanged(previous.IncumbentPnL, next.IncumbentPnL) ||
		challengerMetricChanged(previous.ChallengerPnL, next.ChallengerPnL) ||
		previous.IncumbentTradeCount != next.IncumbentTradeCount ||
		previous.ChallengerTradeCount != next.ChallengerTradeCount ||
		challengerMetricChanged(previous.IncumbentFees, next.IncumbentFees) ||
		challengerMetricChanged(previous.ChallengerFees, next.ChallengerFees) ||
		previous.IncumbentAvgHoldMs != next.IncumbentAvgHoldMs ||
		previous.ChallengerAvgHoldMs != next.ChallengerAvgHoldMs
}

func appendDealReviewChallengerProtocol(compare *store.DealReviewChallengerCompare, eventType, actor, message string, metrics *store.DealReviewChallengerMetrics, data map[string]any) {
	if compare == nil {
		return
	}
	compare.ProtocolJSON = store.AppendDealReviewChallengerProtocolEvent(compare.ProtocolJSON, store.DealReviewChallengerProtocolEvent{
		Timestamp: time.Now().UTC(),
		Type:      strings.TrimSpace(eventType),
		Actor:     strings.TrimSpace(actor),
		Message:   strings.TrimSpace(message),
		Metrics:   metrics,
		Data:      data,
	})
}

func buildDealReviewChallengerEvaluationMessage(metrics *store.DealReviewChallengerMetrics) string {
	if metrics == nil {
		return "Compare evaluation updated."
	}
	return fmt.Sprintf(
		"Evaluation updated: incumbent %s (%d trades) vs challenger %s (%d trades).",
		formatSignedFloat(metrics.IncumbentPnL),
		metrics.IncumbentTradeCount,
		formatSignedFloat(metrics.ChallengerPnL),
		metrics.ChallengerTradeCount,
	)
}

func (s *Server) loadFreshDealReviewChallengerCompare(userID, traderID, compareID string) (*store.DealReviewChallengerCompare, error) {
	compare, err := s.store.DealReview().GetChallengerCompareRecord(userID, traderID, compareID)
	if err != nil {
		return nil, err
	}
	if isDealReviewChallengerCompareActive(compare.Status) {
		if err := s.syncDealReviewChallengerCompare(compare, time.Now().UTC()); err != nil {
			return nil, err
		}
		return s.store.DealReview().GetChallengerCompareRecord(userID, traderID, compareID)
	}
	return compare, nil
}

func (s *Server) syncDealReviewChallengerCompare(compare *store.DealReviewChallengerCompare, now time.Time) error {
	if compare == nil {
		return nil
	}
	previousMetrics := parseDealReviewChallengerMetrics(compare.MetricsJSON)
	metrics, err := s.computeDealReviewChallengerMetrics(compare)
	if err != nil {
		if strings.TrimSpace(compare.ErrorMessage) != err.Error() {
			appendDealReviewChallengerProtocol(compare, "error", "system", fmt.Sprintf("Compare evaluation failed: %v", err), nil, nil)
		}
		compare.ErrorMessage = err.Error()
		return s.store.DealReview().SaveChallengerCompare(compare)
	}
	metricsJSON, _ := json.Marshal(metrics)
	compare.MetricsJSON = string(metricsJSON)
	compare.LastEvaluatedAt = metrics.EvaluatedAt
	compare.ErrorMessage = ""

	if now.Before(compare.EndsAt) {
		compare.Status = store.DealReviewChallengerStatusRunning
		compare.Summary = buildDealReviewCompareRunningSummary(compare, metrics)
		if didDealReviewChallengerMetricsChange(previousMetrics, metrics) {
			appendDealReviewChallengerProtocol(compare, "evaluation", "system", buildDealReviewChallengerEvaluationMessage(metrics), metrics, nil)
		}
		return s.store.DealReview().SaveChallengerCompare(compare)
	}

	switch {
	case metrics.ChallengerPnL > metrics.IncumbentPnL:
		compare.Status = store.DealReviewChallengerStatusCompleted
		compare.WinnerTraderID = compare.ChallengerTraderID
		compare.LoserTraderID = compare.IncumbentTraderID
		compare.ResolvedAt = now.UTC()
		compare.Summary = fmt.Sprintf("Challenger won on realized PnL: %s vs %s.", formatSignedFloat(metrics.ChallengerPnL), formatSignedFloat(metrics.IncumbentPnL))
		appendDealReviewChallengerProtocol(compare, "resolved", "system", compare.Summary, metrics, map[string]any{
			"winner_trader_id": compare.ChallengerTraderID,
			"loser_trader_id":  compare.IncumbentTraderID,
			"resolution":       "auto_pnl",
		})
		if err := s.stopDealReviewTrader(compare.UserID, compare.IncumbentTraderID); err != nil {
			logger.Warnf("⚠️ Failed to stop losing incumbent trader %s: %v", compare.IncumbentTraderID, err)
		}
	case metrics.IncumbentPnL > metrics.ChallengerPnL:
		compare.Status = store.DealReviewChallengerStatusCompleted
		compare.WinnerTraderID = compare.IncumbentTraderID
		compare.LoserTraderID = compare.ChallengerTraderID
		compare.ResolvedAt = now.UTC()
		compare.Summary = fmt.Sprintf("Incumbent held the lead on realized PnL: %s vs %s.", formatSignedFloat(metrics.IncumbentPnL), formatSignedFloat(metrics.ChallengerPnL))
		appendDealReviewChallengerProtocol(compare, "resolved", "system", compare.Summary, metrics, map[string]any{
			"winner_trader_id": compare.IncumbentTraderID,
			"loser_trader_id":  compare.ChallengerTraderID,
			"resolution":       "auto_pnl",
		})
		if err := s.stopDealReviewTrader(compare.UserID, compare.ChallengerTraderID); err != nil {
			logger.Warnf("⚠️ Failed to stop losing challenger trader %s: %v", compare.ChallengerTraderID, err)
		}
	default:
		compare.Status = store.DealReviewChallengerStatusRunning
		compare.ExtensionCount++
		compare.EndsAt = compare.EndsAt.Add(12 * time.Hour)
		compare.Summary = fmt.Sprintf("Comparison tied on realized PnL (%s vs %s). Window extended by 12h to %s.", formatSignedFloat(metrics.IncumbentPnL), formatSignedFloat(metrics.ChallengerPnL), compare.EndsAt.Local().Format(time.RFC3339))
		appendDealReviewChallengerProtocol(compare, "extended", "system", compare.Summary, metrics, map[string]any{
			"extension_count": compare.ExtensionCount,
			"new_ends_at":     compare.EndsAt.UTC(),
		})
	}

	return s.store.DealReview().SaveChallengerCompare(compare)
}

func (s *Server) ProcessPendingDealReviewChallengerCompares() error {
	compares, err := s.store.DealReview().ListActiveChallengerCompares(100)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for i := range compares {
		if err := s.syncDealReviewChallengerCompare(&compares[i], now); err != nil {
			logger.Warnf("⚠️ Failed to process challenger compare %s: %v", compares[i].ID, err)
		}
	}
	return nil
}

func (s *Server) processDealReviewChallengerComparesForTrader(userID, traderID string) error {
	compares, err := s.store.DealReview().ListActiveChallengerCompares(100)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for i := range compares {
		if compares[i].UserID != userID || compares[i].TraderID != traderID {
			continue
		}
		if err := s.syncDealReviewChallengerCompare(&compares[i], now); err != nil {
			logger.Warnf("⚠️ Failed to process challenger compare %s: %v", compares[i].ID, err)
		}
	}
	return nil
}

func (s *Server) RunDealReviewChallengerCompareSupervisor() {
	if err := s.ProcessPendingDealReviewChallengerCompares(); err != nil {
		logger.Warnf("⚠️ Initial deal review challenger compare sweep failed: %v", err)
	}
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if err := s.ProcessPendingDealReviewChallengerCompares(); err != nil {
			logger.Warnf("⚠️ Deal review challenger compare supervisor failed: %v", err)
		}
	}
}

func newClientFromModelConfig(modelCfg *store.AIModel, modelName string) mcp.AIClient {
	aiClient := mcp.NewAIClientByProvider(modelCfg.Provider)
	if aiClient == nil {
		aiClient = mcp.NewClient()
	}
	apiKey := string(modelCfg.APIKey)
	switch modelCfg.Provider {
	case "claw402":
		aiClient.SetAPIKey(apiKey, "", modelName)
	default:
		aiClient.SetAPIKey(apiKey, modelCfg.CustomAPIURL, modelName)
	}
	return aiClient
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case "openai":
		return "gpt-5.1"
	case "claude":
		return "claude-opus-4-6"
	case "gemini":
		return "gemini-3-pro-preview"
	case "grok":
		return "grok-3-latest"
	case "kimi":
		return "moonshot-v1-auto"
	case "minimax":
		return "MiniMax-M2.7"
	case "qwen":
		return "qwen3-max"
	case "claw402":
		return "glm-5"
	default:
		return "deepseek-chat"
	}
}

func (s *Server) listAvailableModelsForConfig(modelCfg *store.AIModel) ([]remoteModelInfo, error) {
	provider := strings.TrimSpace(modelCfg.Provider)
	if provider != "openai" {
		models := []remoteModelInfo{}
		seen := map[string]bool{}
		for _, candidate := range []string{
			strings.TrimSpace(modelCfg.CustomModelName),
			defaultModelForProvider(provider),
		} {
			if candidate == "" || seen[candidate] {
				continue
			}
			seen[candidate] = true
			models = append(models, remoteModelInfo{
				ID:        candidate,
				Label:     candidate,
				Provider:  provider,
				Available: true,
			})
		}
		return models, nil
	}

	baseURL := resolveModelsURL(provider, modelCfg.CustomAPIURL)
	req, err := http.NewRequest(http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(modelCfg.APIKey)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("model catalog request failed with status %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	items := make([]remoteModelInfo, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if !isLikelyChatModel(id) {
			continue
		}
		items = append(items, remoteModelInfo{
			ID:        id,
			Label:     id,
			Provider:  provider,
			Available: true,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func resolveModelsURL(provider, customURL string) string {
	if provider != "openai" {
		return ""
	}
	base := strings.TrimSpace(customURL)
	if base == "" {
		return "https://api.openai.com/v1/models"
	}
	base = strings.TrimSuffix(base, "#")
	base = strings.TrimSuffix(base, "/")
	base = strings.TrimSuffix(base, "/chat/completions")
	if strings.HasSuffix(base, "/v1") {
		return base + "/models"
	}
	if strings.HasSuffix(base, "/models") {
		return base
	}
	return base + "/models"
}

func isLikelyChatModel(id string) bool {
	lower := strings.ToLower(strings.TrimSpace(id))
	if lower == "" {
		return false
	}
	for _, blocked := range []string{"embedding", "tts", "whisper", "audio", "image", "vision-preview", "moderation"} {
		if strings.Contains(lower, blocked) {
			return false
		}
	}
	for _, allowed := range []string{"gpt-", "o1", "o3", "o4", "codex"} {
		if strings.HasPrefix(lower, allowed) {
			return true
		}
	}
	return false
}

func buildDealReviewAnalysisPayload(traderCfg *store.Trader, strategyRecord *store.Strategy, strategyCfg *store.StrategyConfig, filter store.DealReviewListFilter, cases []store.DealReviewCaseDetail, summary *store.DealReviewDatasetSummary) (map[string]any, error) {
	type sample struct {
		Symbol               string   `json:"symbol"`
		Side                 string   `json:"side"`
		Outcome              string   `json:"outcome"`
		RealizedPnL          float64  `json:"realized_pnl"`
		RealizedPnLPct       float64  `json:"realized_pnl_pct"`
		HoldMinutes          float64  `json:"hold_minutes"`
		OpenSelectionBucket  string   `json:"open_selection_bucket,omitempty"`
		OpenCandidateSources []string `json:"open_candidate_sources,omitempty"`
		OpenReasoning        string   `json:"open_reasoning,omitempty"`
		CloseReasoning       string   `json:"close_reasoning,omitempty"`
		CloseReason          string   `json:"close_reason,omitempty"`
		OpenStopLoss         float64  `json:"open_stop_loss,omitempty"`
		OpenTakeProfit       float64  `json:"open_take_profit,omitempty"`
		OpenConfidence       int      `json:"open_confidence,omitempty"`
		CloseConfidence      int      `json:"close_confidence,omitempty"`
	}

	samples := make([]sample, 0, len(cases))
	for _, item := range cases {
		entry := sample{
			Symbol:               item.Case.Symbol,
			Side:                 item.Case.Side,
			Outcome:              item.Case.Outcome,
			RealizedPnL:          item.Case.RealizedPnL,
			RealizedPnLPct:       item.Case.RealizedPnLPct,
			HoldMinutes:          float64(item.Case.HoldDurationMs) / 60000,
			OpenSelectionBucket:  item.Case.OpenSelectionBucket,
			OpenCandidateSources: item.OpenCandidateSources,
			CloseReason:          item.Case.CloseReason,
			OpenStopLoss:         item.Case.OpenStopLoss,
			OpenTakeProfit:       item.Case.OpenTakeProfit,
			OpenConfidence:       item.Case.OpenConfidence,
			CloseConfidence:      item.Case.CloseConfidence,
		}
		if item.Open != nil && item.Open.Event != nil {
			entry.OpenReasoning = item.Open.Event.Reasoning
		}
		if item.Close != nil && item.Close.Event != nil {
			entry.CloseReasoning = item.Close.Event.Reasoning
		}
		samples = append(samples, entry)
	}

	losers := append([]sample(nil), samples...)
	sort.Slice(losers, func(i, j int) bool { return losers[i].RealizedPnL < losers[j].RealizedPnL })
	if len(losers) > 8 {
		losers = losers[:8]
	}
	winners := append([]sample(nil), samples...)
	sort.Slice(winners, func(i, j int) bool { return winners[i].RealizedPnL > winners[j].RealizedPnL })
	if len(winners) > 5 {
		winners = winners[:5]
	}

	filterMap := map[string]any{
		"symbol":    filter.Symbol,
		"side":      filter.Side,
		"status":    filter.Status,
		"outcome":   filter.Outcome,
		"from_time": filter.FromTime,
		"to_time":   filter.ToTime,
	}
	if filter.MinPnL != nil {
		filterMap["min_pnl"] = *filter.MinPnL
	}
	if filter.MaxPnL != nil {
		filterMap["max_pnl"] = *filter.MaxPnL
	}

	configJSON, err := json.Marshal(strategyCfg)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"trader": map[string]any{
			"id":   traderCfg.ID,
			"name": traderCfg.Name,
		},
		"strategy": map[string]any{
			"id":     strategyRecord.ID,
			"name":   strategyRecord.Name,
			"config": json.RawMessage(configJSON),
		},
		"filters":         filterMap,
		"dataset_summary": summary,
		"top_losers":      losers,
		"top_winners":     winners,
		"recent_cases":    samples[:minInt(len(samples), 12)],
		"case_contexts":   buildDealReviewAIScanCaseContexts(cases, 6),
	}, nil
}

func buildDealReviewAnalysisPrompt(payload map[string]any) (string, string, error) {
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", "", err
	}
	systemPrompt := `You are reviewing deal-level trading outcomes to improve an automated strategy.
Return JSON only with this shape:
{
  "executive_summary": "string",
  "strengths": ["string"],
  "weaknesses": ["string"],
  "patterns": ["string"],
  "immediate_actions": [
    {"title":"string","rationale":"string","expected_impact":"string","risk":"string","config_patch":{}}
  ],
  "experiments": [
    {"title":"string","rationale":"string","expected_impact":"string","risk":"string","config_patch":{}}
  ],
  "strategy_patch": {}
}
Rules:
- Focus on concrete strategy improvements, not generic advice.
- Use "strategy_patch" as a partial JSON patch that can be merged into the current strategy config.
- Keep patches realistic and minimal; do not include fields you do not want to change.
- If evidence is weak, say so explicitly in weaknesses/patterns.
- Never output markdown or code fences.`
	userPrompt := "Analyze this deal-review dataset and propose strategy improvements:\n" + string(body)
	return systemPrompt, userPrompt, nil
}

func buildDealReviewAIScanCaseContexts(cases []store.DealReviewCaseDetail, limit int) []map[string]any {
	if limit <= 0 {
		limit = 6
	}

	selected := selectDealReviewContextCases(cases, limit)
	result := make([]map[string]any, 0, len(selected))
	for _, item := range selected {
		entry := map[string]any{
			"case_id":          item.Case.ID,
			"symbol":           item.Case.Symbol,
			"side":             item.Case.Side,
			"outcome":          item.Case.Outcome,
			"realized_pnl":     item.Case.RealizedPnL,
			"realized_pnl_pct": item.Case.RealizedPnLPct,
			"hold_minutes":     float64(item.Case.HoldDurationMs) / 60000,
			"close_reason":     item.Case.CloseReason,
		}
		if open := buildDealReviewAIScanEventContext(item.Open); len(open) > 0 {
			entry["open_context"] = open
		}
		if closeCtx := buildDealReviewAIScanEventContext(item.Close); len(closeCtx) > 0 {
			entry["close_context"] = closeCtx
		}
		if item.PriceTimeline != nil {
			entry["price_timeline_summary"] = item.PriceTimeline.Summary
		}
		result = append(result, entry)
	}
	return result
}

func selectDealReviewContextCases(cases []store.DealReviewCaseDetail, limit int) []store.DealReviewCaseDetail {
	if len(cases) <= limit {
		return append([]store.DealReviewCaseDetail(nil), cases...)
	}

	losers := append([]store.DealReviewCaseDetail(nil), cases...)
	sort.Slice(losers, func(i, j int) bool {
		return losers[i].Case.RealizedPnL < losers[j].Case.RealizedPnL
	})

	winners := append([]store.DealReviewCaseDetail(nil), cases...)
	sort.Slice(winners, func(i, j int) bool {
		return winners[i].Case.RealizedPnL > winners[j].Case.RealizedPnL
	})

	selected := make([]store.DealReviewCaseDetail, 0, limit)
	seen := make(map[string]struct{}, limit)
	appendUnique := func(items []store.DealReviewCaseDetail, count int) {
		for _, item := range items {
			if len(selected) >= limit || count <= 0 {
				return
			}
			if _, exists := seen[item.Case.ID]; exists {
				continue
			}
			seen[item.Case.ID] = struct{}{}
			selected = append(selected, item)
			count--
		}
	}

	appendUnique(losers, minInt(limit, 4))
	if len(selected) < limit {
		appendUnique(winners, minInt(limit-len(selected), 2))
	}
	if len(selected) < limit {
		appendUnique(cases, limit-len(selected))
	}
	return selected
}

func buildDealReviewAIScanEventContext(detail *store.DealReviewEventDetail) map[string]any {
	if detail == nil || detail.Event == nil {
		return nil
	}

	context := map[string]any{
		"cycle":       detail.Event.DecisionCycleNumber,
		"action":      detail.Event.Action,
		"reasoning":   detail.Event.Reasoning,
		"confidence":  detail.Event.Confidence,
		"price":       detail.Event.Price,
		"quantity":    detail.Event.Quantity,
		"stop_loss":   detail.Event.StopLoss,
		"take_profit": detail.Event.TakeProfit,
	}
	if detail.Event.CloseReason != "" {
		context["close_reason"] = detail.Event.CloseReason
	}
	if len(detail.CandidateSources) > 0 {
		context["candidate_sources"] = detail.CandidateSources
	}
	if detail.Snapshot != nil {
		if detail.Snapshot.AccountState.PositionCount > 0 || detail.Snapshot.AccountState.TotalBalance > 0 || detail.Snapshot.AccountState.AvailableBalance > 0 {
			context["account_state"] = detail.Snapshot.AccountState
		}
		if len(detail.Snapshot.Positions) > 0 {
			context["positions"] = detail.Snapshot.Positions
		}
		if len(detail.Snapshot.CandidateCoins) > 0 {
			context["candidate_coins"] = detail.Snapshot.CandidateCoins
		}
		if len(detail.Snapshot.CandidateDetails) > 0 {
			context["candidate_details"] = detail.Snapshot.CandidateDetails
		}
		if len(detail.Snapshot.ExecutionLog) > 0 {
			context["execution_log"] = detail.Snapshot.ExecutionLog[:minInt(len(detail.Snapshot.ExecutionLog), 12)]
		}
	}

	systemPrompt := ""
	userPrompt := ""
	decisionJSON := ""
	rawResponse := ""
	if detail.Snapshot != nil {
		systemPrompt = detail.Snapshot.SystemPrompt
		userPrompt = detail.Snapshot.UserPrompt
		decisionJSON = detail.Snapshot.DecisionJSON
		rawResponse = detail.Snapshot.RawResponse
	}
	if detail.DecisionRecord != nil {
		if systemPrompt == "" {
			systemPrompt = detail.DecisionRecord.SystemPrompt
		}
		if userPrompt == "" {
			userPrompt = detail.DecisionRecord.InputPrompt
		}
		if decisionJSON == "" {
			decisionJSON = detail.DecisionRecord.DecisionJSON
		}
		if rawResponse == "" {
			rawResponse = detail.DecisionRecord.RawResponse
		}
	}
	if clipped := clipDealReviewAIScanText(systemPrompt, 2500); clipped != "" {
		context["system_prompt"] = clipped
	}
	if clipped := clipDealReviewAIScanText(userPrompt, 6000); clipped != "" {
		context["input_prompt"] = clipped
	}
	if clipped := clipDealReviewAIScanText(decisionJSON, 1500); clipped != "" {
		context["decision_json"] = clipped
	}
	if clipped := clipDealReviewAIScanText(rawResponse, 2500); clipped != "" {
		context["raw_response"] = clipped
	}

	return context
}

func clipDealReviewAIScanText(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return string(runes[:limit]) + "\n...[truncated]"
}

func parseDealReviewAIScanResponse(response string) (*store.DealReviewAIScanResult, string, error) {
	cleaned := strings.TrimSpace(response)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	result := &store.DealReviewAIScanResult{}
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
	if result.StrategyPatch == nil {
		result.StrategyPatch = map[string]any{}
	}
	rawJSON, _ := json.Marshal(result)
	return result, string(rawJSON), nil
}

func applyStrategyPatch(existingConfigJSON string, patch map[string]any) (string, error) {
	if len(patch) == 0 {
		return "", fmt.Errorf("strategy patch is empty")
	}
	existing := map[string]any{}
	if err := json.Unmarshal([]byte(existingConfigJSON), &existing); err != nil {
		return "", err
	}
	merged := mergeJSONMaps(existing, patch)
	body, err := json.Marshal(merged)
	if err != nil {
		return "", err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		return pretty.String(), nil
	}
	return string(body), nil
}

func mergeJSONMaps(dst map[string]any, patch map[string]any) map[string]any {
	for key, value := range patch {
		if existingMap, ok := dst[key].(map[string]any); ok {
			if patchMap, ok := value.(map[string]any); ok {
				dst[key] = mergeJSONMaps(existingMap, patchMap)
				continue
			}
		}
		dst[key] = value
	}
	return dst
}

func equalJSONMaps(left, right map[string]any) bool {
	leftBody, _ := json.Marshal(left)
	rightBody, _ := json.Marshal(right)
	return string(leftBody) == string(rightBody)
}

func diffJSONMaps(left, right map[string]any) []dealReviewJSONDiffEntry {
	leftFlat := map[string]string{}
	rightFlat := map[string]string{}
	flattenJSONMap("", left, leftFlat)
	flattenJSONMap("", right, rightFlat)

	keysMap := map[string]struct{}{}
	for key := range leftFlat {
		keysMap[key] = struct{}{}
	}
	for key := range rightFlat {
		keysMap[key] = struct{}{}
	}
	keys := make([]string, 0, len(keysMap))
	for key := range keysMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	diff := make([]dealReviewJSONDiffEntry, 0)
	for _, key := range keys {
		leftValue := leftFlat[key]
		rightValue := rightFlat[key]
		if leftValue == rightValue {
			continue
		}
		diff = append(diff, dealReviewJSONDiffEntry{
			Path:  key,
			Left:  leftValue,
			Right: rightValue,
		})
	}
	return diff
}

func flattenJSONMap(prefix string, value any, out map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			out[prefix] = "{}"
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flattenJSONMap(next, typed[key], out)
		}
	case []any:
		body, _ := json.Marshal(typed)
		out[prefix] = string(body)
	default:
		body, _ := json.Marshal(typed)
		out[prefix] = string(body)
	}
}

func intersectStringSlices(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	for _, item := range left {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		seen[normalized] = struct{}{}
	}
	result := []string{}
	for _, item := range right {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			result = append(result, normalized)
			delete(seen, normalized)
		}
	}
	sort.Strings(result)
	return result
}

func actionTitles(items []store.DealReviewActionItem) []string {
	if len(items) == 0 {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		result = append(result, title)
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
