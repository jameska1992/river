package handlers

import (
	"net/http"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type InsightsHandler struct {
	svc *services.InsightsService
}

func NewInsightsHandler(svc *services.InsightsService) *InsightsHandler {
	return &InsightsHandler{svc: svc}
}

// GetWatch returns watch analytics (watch time, top titles, completion rate,
// activity-over-time, per-user breakdown) for a time window.
//
// @Summary      Watch analytics
// @Tags         admin
// @Produce      json
// @Param        window  query     string  false  "Time window: '7d', '30d' (default), '90d', or 'all'"
// @Success      200     {object}  services.WatchInsights
// @Failure      500     {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/insights/watch [get]
func (h *InsightsHandler) GetWatch(c *gin.Context) {
	data, err := h.svc.WatchInsights(c.Query("window"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetLibrary returns per-library counts, on-disk storage usage, and the
// untranscoded/failed-job tallies for the storage-health view.
//
// @Summary      Library & storage health
// @Tags         admin
// @Produce      json
// @Success      200  {object}  services.LibraryInsights
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/insights/library [get]
func (h *InsightsHandler) GetLibrary(c *gin.Context) {
	data, err := h.svc.LibraryInsights()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}
