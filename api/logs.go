package api

import (
	"net/http"
	"nofx/trader/binance"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleRiskAPICallLogs(c *gin.Context) {
	limit := 30
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 30 {
		limit = 30
	}

	items := binance.GetRiskAPICallStats(limit)
	c.JSON(http.StatusOK, gin.H{
		"generated_at": time.Now().Format(time.RFC3339),
		"limit":        limit,
		"items":        items,
	})
}
