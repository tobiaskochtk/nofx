package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"nofx/store"

	"github.com/gin-gonic/gin"
)

type semanticMemorySearchRequest struct {
	Query          string   `json:"query"`
	DocTypes       []string `json:"doc_types"`
	ModelID        string   `json:"model_id"`
	EmbeddingModel string   `json:"embedding_model"`
	Limit          int      `json:"limit"`

	Symbol                string `json:"symbol"`
	Side                  string `json:"side"`
	Outcome               string `json:"outcome"`
	CloseReason           string `json:"close_reason"`
	ExitOrigin            string `json:"exit_origin"`
	ExitReasonQuality     string `json:"exit_reason_quality"`
	Status                string `json:"status"`
	Trigger               string `json:"trigger"`
	Category              string `json:"category"`
	OpenTrendRegime       string `json:"open_trend_regime"`
	OpenVolatilityRegime  string `json:"open_volatility_regime"`
	OpenOIRegime          string `json:"open_oi_regime"`
	CloseTrendRegime      string `json:"close_trend_regime"`
	CloseVolatilityRegime string `json:"close_volatility_regime"`
	CloseOIRegime         string `json:"close_oi_regime"`
	FromTime              int64  `json:"from_time"`
	ToTime                int64  `json:"to_time"`
}

type semanticMemorySearchPresetRequest struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

func (s *Server) handleTraderSemanticMemoryStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	status, err := s.store.SemanticMemory().GetCorpusStatus(userID, traderID, 8)
	if err != nil {
		SafeInternalError(c, "Failed to fetch semantic memory status", err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (s *Server) handleTraderSemanticMemorySearch(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	var req semanticMemorySearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid semantic memory search payload")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		SafeBadRequest(c, "Search query is required")
		return
	}

	modelCfg, err := resolveSemanticMemorySearchModelConfig(s.store, userID, strings.TrimSpace(req.ModelID))
	if err != nil {
		SafeBadRequest(c, err.Error())
		return
	}
	result, err := s.store.SemanticMemory().SearchDocumentsByQuery(
		userID,
		traderID,
		req.Query,
		req.DocTypes,
		semanticMemorySearchFilterFromRequest(req),
		req.Limit,
		modelCfg,
		strings.TrimSpace(req.EmbeddingModel),
	)
	if err != nil {
		msg := strings.ToLower(strings.TrimSpace(err.Error()))
		switch {
		case strings.Contains(msg, "query is required"), strings.Contains(msg, "model config"), strings.Contains(msg, "supports only"):
			SafeBadRequest(c, err.Error())
		case strings.Contains(msg, "pgvector"), strings.Contains(msg, "requires postgresql"):
			SafeConflict(c, "Semantic memory search is not available in the current runtime")
		default:
			SafeInternalError(c, "Failed to run semantic memory search", err)
		}
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleTraderSemanticMemoryPresets(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	items, err := s.store.SemanticMemory().ListSearchPresets(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch semantic memory presets", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) handleTraderSemanticMemorySavePreset(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	var req semanticMemorySearchPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid semantic memory preset payload")
		return
	}
	config := req.Config
	if config == nil {
		config = map[string]any{}
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		SafeBadRequest(c, "Semantic memory preset config must be valid JSON")
		return
	}
	preset := &store.SemanticMemorySearchPreset{
		ID:         strings.TrimSpace(req.ID),
		UserID:     userID,
		TraderID:   traderID,
		Name:       strings.TrimSpace(req.Name),
		ConfigJSON: string(configJSON),
	}
	if err := s.store.SemanticMemory().SaveSearchPreset(preset); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "cannot be empty") {
			SafeBadRequest(c, err.Error())
			return
		}
		SafeInternalError(c, "Failed to save semantic memory preset", err)
		return
	}
	items, err := s.store.SemanticMemory().ListSearchPresets(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch semantic memory presets", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) handleTraderSemanticMemoryDeletePreset(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	presetID := strings.TrimSpace(c.Param("presetId"))
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	if presetID == "" {
		SafeBadRequest(c, "Semantic memory preset id is required")
		return
	}
	if err := s.store.SemanticMemory().DeleteSearchPreset(userID, traderID, presetID); err != nil {
		SafeInternalError(c, "Failed to delete semantic memory preset", err)
		return
	}
	items, err := s.store.SemanticMemory().ListSearchPresets(userID, traderID)
	if err != nil {
		SafeInternalError(c, "Failed to fetch semantic memory presets", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func resolveSemanticMemorySearchModelConfig(root *store.Store, userID, explicitModelID string) (*store.AIModel, error) {
	if root == nil {
		return nil, errSemanticMemorySearchModel("Semantic memory search store is unavailable")
	}
	if explicitModelID != "" {
		modelCfg, err := root.AIModel().Get(userID, explicitModelID)
		if err != nil {
			return nil, errSemanticMemorySearchModel("Selected embedding model config is not available")
		}
		if !modelCfg.Enabled || strings.TrimSpace(modelCfg.Provider) != "openai" || strings.TrimSpace(string(modelCfg.APIKey)) == "" {
			return nil, errSemanticMemorySearchModel("Selected embedding model config must be an enabled OpenAI account with an API key")
		}
		return modelCfg, nil
	}
	modelCfg, err := root.AIModel().GetFirstEnabledByProvider(userID, "openai")
	if err != nil {
		return nil, errSemanticMemorySearchModel("No enabled OpenAI account is available for semantic memory search")
	}
	if strings.TrimSpace(string(modelCfg.APIKey)) == "" {
		return nil, errSemanticMemorySearchModel("The selected OpenAI account has no API key configured")
	}
	return modelCfg, nil
}

func semanticMemorySearchFilterFromRequest(req semanticMemorySearchRequest) store.SemanticMemorySearchFilter {
	filter := store.SemanticMemorySearchFilter{
		Symbol:                req.Symbol,
		Side:                  req.Side,
		Outcome:               req.Outcome,
		CloseReason:           req.CloseReason,
		ExitOrigin:            req.ExitOrigin,
		ExitReasonQuality:     req.ExitReasonQuality,
		Status:                req.Status,
		Trigger:               req.Trigger,
		Category:              req.Category,
		OpenTrendRegime:       req.OpenTrendRegime,
		OpenVolatilityRegime:  req.OpenVolatilityRegime,
		OpenOIRegime:          req.OpenOIRegime,
		CloseTrendRegime:      req.CloseTrendRegime,
		CloseVolatilityRegime: req.CloseVolatilityRegime,
		CloseOIRegime:         req.CloseOIRegime,
	}
	if req.FromTime > 0 {
		from := time.UnixMilli(req.FromTime).UTC()
		filter.FromTime = &from
	}
	if req.ToTime > 0 {
		to := time.UnixMilli(req.ToTime).UTC()
		filter.ToTime = &to
	}
	return filter
}

type semanticMemorySearchModelError string

func (e semanticMemorySearchModelError) Error() string { return string(e) }

func errSemanticMemorySearchModel(msg string) error { return semanticMemorySearchModelError(msg) }
