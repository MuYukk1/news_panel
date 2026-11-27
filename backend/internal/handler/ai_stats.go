package handler

import (
	"github.com/gin-gonic/gin"
	"news_panel/internal/ai"
)

func (h *Handler) aiStats(c *gin.Context) {
	var stats ai.PipelineStats
	if reporter, ok := h.evaluator.(interface{ Stats() ai.PipelineStats }); ok {
		stats = reporter.Stats()
	}
	dataResponse(c, stats)
}
