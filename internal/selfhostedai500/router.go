package selfhostedai500

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func NewRouter(service *Service, authToken string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, service.GetHealth())
	})

	api := router.Group("/api")
	api.Use(authMiddleware(authToken))

	api.GET("/ai500/list", func(c *gin.Context) {
		coins := service.GetAI500List()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"coins": coins,
				"count": len(coins),
			},
		})
	})

	api.GET("/ai500/:symbol", func(c *gin.Context) {
		detail, ok := service.GetAI500DetailResponse(c.Param("symbol"))
		if !ok {
			stats := service.GetAI500Stats()
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"code":    http.StatusNotFound,
				"error":   "symbol not found",
				"data": gin.H{
					"symbol":       normalizePair(c.Param("symbol")),
					"active_count": stats["active_count"],
				},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"code":    0,
			"data":    detail,
		})
	})

	api.GET("/ai500/stats", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"code":    0,
			"data":    service.GetAI500Stats(),
		})
	})

	api.GET("/debug/score/:symbol", func(c *gin.Context) {
		debug, ok := service.GetScoreDebug(c.Param("symbol"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"code":    http.StatusNotFound,
				"error":   "symbol not found",
				"data": gin.H{
					"symbol": normalizePair(c.Param("symbol")),
				},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"code":    0,
			"data":    debug,
		})
	})

	api.GET("/debug/rankings", func(c *gin.Context) {
		kind := normalizeDebugRankingKind(c.DefaultQuery("kind", "ai500"))
		if kind == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"code":    http.StatusBadRequest,
				"error":   "unsupported ranking kind",
				"data": gin.H{
					"requested_kind": c.Query("kind"),
					"supported_kinds": []string{
						"ai500",
						"oi",
						"price",
						"netflow",
					},
				},
			})
			return
		}

		payload, err := service.GetRankingDebug(
			kind,
			c.DefaultQuery("duration", "1h"),
			c.DefaultQuery("durations", c.DefaultQuery("duration", "1h")),
			parseLimit(c.Query("limit"), 10),
			c.DefaultQuery("type", "institution"),
			c.DefaultQuery("trade", "future"),
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"code":    http.StatusBadRequest,
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"code":    0,
			"data":    payload,
		})
	})

	api.GET("/oi/top-ranking", func(c *gin.Context) {
		duration := c.DefaultQuery("duration", "1h")
		limit := parseLimit(c.Query("limit"), 20)
		positions := service.GetOIRanking(duration, limit, "top")
		exchangeLabel := service.ExchangeLabel()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"code":    0,
			"data": gin.H{
				"positions":        positions,
				"count":            len(positions),
				"exchange":         exchangeLabel,
				"time_range":       duration,
				"time_range_param": duration,
				"rank_type":        "top",
				"limit":            limit,
			},
		})
	})

	api.GET("/oi/low-ranking", func(c *gin.Context) {
		duration := c.DefaultQuery("duration", "1h")
		limit := parseLimit(c.Query("limit"), 20)
		positions := service.GetOIRanking(duration, limit, "low")
		exchangeLabel := service.ExchangeLabel()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"code":    0,
			"data": gin.H{
				"positions":        positions,
				"count":            len(positions),
				"exchange":         exchangeLabel,
				"time_range":       duration,
				"time_range_param": duration,
				"rank_type":        "low",
				"limit":            limit,
			},
		})
	})

	api.GET("/price/ranking", func(c *gin.Context) {
		durations := c.DefaultQuery("duration", "1h")
		limit := parseLimit(c.Query("limit"), 10)
		payload := service.GetPriceRanking(durations, limit)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"durations": splitDurations(durations),
				"limit":     limit,
				"data":      payload,
			},
		})
	})

	api.GET("/netflow/top-ranking", func(c *gin.Context) {
		duration := c.DefaultQuery("duration", "1h")
		limit := parseLimit(c.Query("limit"), 10)
		flowType := normalizeFlowType(c.DefaultQuery("type", "institution"))
		trade := normalizeTradeType(c.DefaultQuery("trade", "future"))
		positions := service.GetNetflowRanking(duration, limit, "top", flowType, trade)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"netflows":   positions,
				"count":      len(positions),
				"type":       flowType,
				"trade":      trade,
				"time_range": duration,
				"rank_type":  "top",
				"limit":      limit,
			},
		})
	})

	api.GET("/netflow/low-ranking", func(c *gin.Context) {
		duration := c.DefaultQuery("duration", "1h")
		limit := parseLimit(c.Query("limit"), 10)
		flowType := normalizeFlowType(c.DefaultQuery("type", "institution"))
		trade := normalizeTradeType(c.DefaultQuery("trade", "future"))
		positions := service.GetNetflowRanking(duration, limit, "low", flowType, trade)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"netflows":   positions,
				"count":      len(positions),
				"type":       flowType,
				"trade":      trade,
				"time_range": duration,
				"rank_type":  "low",
				"limit":      limit,
			},
		})
	})

	api.GET("/coin/:symbol", func(c *gin.Context) {
		include := c.DefaultQuery("include", "netflow,oi,price")
		coin, ok := service.GetCoin(c.Param("symbol"), include)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"code":    http.StatusNotFound,
				"error":   "symbol not found",
				"data": gin.H{
					"symbol":              normalizePair(c.Param("symbol")),
					"requested_includes":  parseRequestedIncludes(include),
					"recognized_includes": parseSupportedCoinIncludeNames(include),
					"supported_includes":  supportedCoinIncludeNames(),
				},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"code":    0,
			"data":    coin,
		})
	})

	return router
}

func authMiddleware(authToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authToken == "" {
			c.Next()
			return
		}
		if c.Query("auth") == authToken {
			c.Next()
			return
		}
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(header), "bearer ") && strings.TrimSpace(header[7:]) == authToken {
			c.Next()
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		c.Abort()
	}
}

func parseLimit(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func normalizeFlowType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "personal":
		return "personal"
	default:
		return "institution"
	}
}

func normalizeTradeType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "spot":
		return "spot"
	case "futures", "future":
		return "future"
	default:
		return "future"
	}
}

func normalizeDebugRankingKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ai500":
		return "ai500"
	case "oi":
		return "oi"
	case "price":
		return "price"
	case "netflow":
		return "netflow"
	default:
		return ""
	}
}

func parseRequestedIncludes(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(strings.ToLower(part))
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}
