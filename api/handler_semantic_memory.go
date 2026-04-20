package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nofx/store"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type semanticMemorySimilarityResponse struct {
	QueryDocument *store.SemanticMemoryDocument    `json:"query_document"`
	Items         []*store.SemanticMemorySearchHit `json:"items"`
	Total         int                              `json:"total"`
}

func (s *Server) handleTraderDealReviewSimilarCases(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	caseID := c.Param("caseId")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	result, err := s.store.SemanticMemory().FindSimilarDocumentsBySource(
		userID,
		traderID,
		store.SemanticMemoryDocTypeDealReviewCase,
		caseID,
		parseSemanticMemoryDealReviewFilter(c),
		parseSemanticMemoryLimit(c, 8, 50),
	)
	if err != nil {
		handleSemanticMemorySimilarityError(c, "Deal review case", "Failed to fetch similar deal review cases", err)
		return
	}
	c.JSON(http.StatusOK, semanticMemorySimilarityResponse{
		QueryDocument: result.QueryDocument,
		Items:         result.Items,
		Total:         len(result.Items),
	})
}

func (s *Server) handleTraderAutonomousOptimizerSimilarRuns(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	runID := c.Param("runId")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	result, err := s.store.SemanticMemory().FindSimilarDocumentsBySource(
		userID,
		traderID,
		store.SemanticMemoryDocTypeAutonomousOptimizerRun,
		runID,
		parseSemanticMemoryOptimizerFilter(c),
		parseSemanticMemoryLimit(c, 6, 25),
	)
	if err != nil {
		handleSemanticMemorySimilarityError(c, "Autonomous optimizer run", "Failed to fetch similar autonomous optimizer runs", err)
		return
	}
	c.JSON(http.StatusOK, semanticMemorySimilarityResponse{
		QueryDocument: result.QueryDocument,
		Items:         result.Items,
		Total:         len(result.Items),
	})
}

func (s *Server) handleTraderDealReviewSimilarStrategyVersions(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	versionID := c.Param("versionId")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	result, err := s.store.SemanticMemory().FindSimilarDocumentsBySource(
		userID,
		traderID,
		store.SemanticMemoryDocTypeStrategyVersion,
		versionID,
		store.SemanticMemorySearchFilter{},
		parseSemanticMemoryLimit(c, 6, 25),
	)
	if err != nil {
		handleSemanticMemorySimilarityError(c, "Strategy version", "Failed to fetch similar strategy versions", err)
		return
	}
	c.JSON(http.StatusOK, semanticMemorySimilarityResponse{
		QueryDocument: result.QueryDocument,
		Items:         result.Items,
		Total:         len(result.Items),
	})
}

func parseSemanticMemoryLimit(c *gin.Context, fallback, max int) int {
	limit := fallback
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > max {
		limit = max
	}
	return limit
}

func parseSemanticMemoryDealReviewFilter(c *gin.Context) store.SemanticMemorySearchFilter {
	filter := store.SemanticMemorySearchFilter{
		Symbol:                c.Query("symbol"),
		Side:                  c.Query("side"),
		Outcome:               c.Query("outcome"),
		CloseReason:           c.Query("close_reason"),
		ExitOrigin:            c.Query("exit_origin"),
		ExitReasonQuality:     c.Query("exit_reason_quality"),
		OpenTrendRegime:       c.Query("open_trend_regime"),
		OpenVolatilityRegime:  c.Query("open_volatility_regime"),
		OpenOIRegime:          c.Query("open_oi_regime"),
		CloseTrendRegime:      c.Query("close_trend_regime"),
		CloseVolatilityRegime: c.Query("close_volatility_regime"),
		CloseOIRegime:         c.Query("close_oi_regime"),
	}
	filter.FromTime = parseSemanticMemoryUnixMillis(c.Query("from_time"))
	filter.ToTime = parseSemanticMemoryUnixMillis(c.Query("to_time"))
	return filter
}

func parseSemanticMemoryOptimizerFilter(c *gin.Context) store.SemanticMemorySearchFilter {
	filter := store.SemanticMemorySearchFilter{
		Status:   c.Query("status"),
		Trigger:  c.Query("trigger"),
		Category: c.Query("category"),
	}
	filter.FromTime = parseSemanticMemoryUnixMillis(c.Query("from_time"))
	filter.ToTime = parseSemanticMemoryUnixMillis(c.Query("to_time"))
	return filter
}

func parseSemanticMemoryUnixMillis(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil
	}
	t := time.UnixMilli(value).UTC()
	return &t
}

func handleSemanticMemorySimilarityError(c *gin.Context, notFoundResource, operation string, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		SafeNotFound(c, notFoundResource)
		return
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "not embedded yet"):
		SafeConflict(c, "Semantic memory is not ready for this record yet")
	case strings.Contains(msg, "pgvector"), strings.Contains(msg, "requires postgresql"):
		SafeConflict(c, "Semantic memory retrieval is not available in the current runtime")
	default:
		SafeInternalError(c, operation, err)
	}
}
