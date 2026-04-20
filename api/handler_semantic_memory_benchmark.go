package api

import (
	"net/http"
	"strings"

	"nofx/store"

	"github.com/gin-gonic/gin"
)

type semanticMemoryBenchmarkRunRequest struct {
	DocTypes         []string `json:"doc_types"`
	SamplePerDocType int      `json:"sample_per_doc_type"`
	TopK             int      `json:"top_k"`
}

func (s *Server) handleTraderSemanticMemoryBenchmarks(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}
	items, err := s.store.SemanticMemory().ListBenchmarkRuns(userID, traderID, 8)
	if err != nil {
		SafeInternalError(c, "Failed to fetch semantic memory benchmark runs", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) handleTraderSemanticMemoryRunBenchmark(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	var req semanticMemoryBenchmarkRunRequest
	if err := c.ShouldBindJSON(&req); err != nil && strings.TrimSpace(err.Error()) != "EOF" {
		SafeBadRequest(c, "Invalid semantic memory benchmark payload")
		return
	}

	run, err := s.store.SemanticMemory().RunBenchmark(userID, traderID, store.SemanticMemoryBenchmarkConfig{
		DocTypes:         req.DocTypes,
		SamplePerDocType: req.SamplePerDocType,
		TopK:             req.TopK,
	})
	if err != nil {
		msg := strings.ToLower(strings.TrimSpace(err.Error()))
		switch {
		case strings.Contains(msg, "pgvector"), strings.Contains(msg, "requires postgresql"):
			SafeConflict(c, "Semantic memory benchmarks are not available in the current runtime")
		default:
			SafeInternalError(c, "Failed to run semantic memory benchmark", err)
		}
		return
	}
	c.JSON(http.StatusOK, run)
}
