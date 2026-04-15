package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// handleTraderAI500BucketReview returns the per-trader AI500 bucket review window.
func (s *Server) handleTraderAI500BucketReview(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	traderID := c.Param("id")
	if traderID == "" {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	if _, err := s.store.Trader().Get(userID, traderID); err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	hours := 24
	if raw := c.Query("hours"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			SafeBadRequest(c, "Invalid hours parameter")
			return
		}
		if parsed > 168 {
			parsed = 168
		}
		hours = parsed
	}

	cycles := 20
	if raw := c.Query("cycles"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			SafeBadRequest(c, "Invalid cycles parameter")
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		cycles = parsed
	}

	review, err := s.store.Decision().GetBucketReview(traderID, time.Duration(hours)*time.Hour, cycles)
	if err != nil {
		SafeInternalError(c, "Get AI500 bucket review", err)
		return
	}

	c.JSON(http.StatusOK, review)
}
