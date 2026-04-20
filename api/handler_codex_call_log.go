package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"nofx/store"
)

func (s *Server) handleGetCodexCallLogs(c *gin.Context) {
	userID := c.GetString("user_id")
	limit := store.CodexCallLogRetention
	if raw := strings.TrimSpace(c.DefaultQuery("limit", strconv.Itoa(store.CodexCallLogRetention))); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= store.CodexCallLogRetention {
			limit = parsed
		}
	}

	traderID := strings.TrimSpace(c.Query("trader_id"))
	items, err := s.store.CodexCallLog().ListRecentForUser(userID, traderID, limit)
	if err != nil {
		SafeInternalError(c, "Failed to load Codex call logs", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"limit": limit,
	})
}
