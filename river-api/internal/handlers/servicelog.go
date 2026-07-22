package handlers

import (
	"net/http"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type ServiceLogHandler struct {
	svc *services.ServiceLogService
}

func NewServiceLogHandler(svc *services.ServiceLogService) *ServiceLogHandler {
	return &ServiceLogHandler{svc: svc}
}

type createLogReq struct {
	Level   string `json:"level"   binding:"required"`
	Service string `json:"service" binding:"required"`
	Message string `json:"message" binding:"required"`
}

// Create accepts a structured log entry from a sibling service (river-scan,
// river-video-trans, etc.) so admins see it in the unified log feed.
//
// @Summary      Append service log entry
// @Tags         logs
// @Accept       json
// @Produce      json
// @Param        body  body      createLogReq  true  "Log entry"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /logs [post]
func (h *ServiceLogHandler) Create(c *gin.Context) {
	var req createLogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.Create(services.CreateLogInput{
		Level:   req.Level,
		Service: req.Service,
		Message: req.Message,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// List returns paginated log entries with optional filters.
//
// @Summary      List service log entries
// @Tags         logs
// @Produce      json
// @Param        level    query  string  false  "Filter by level (info|warn|error)"
// @Param        service  query  string  false  "Filter by service name"
// @Param        from     query  string  false  "ISO8601 lower bound"
// @Param        to       query  string  false  "ISO8601 upper bound"
// @Param        page     query  int     false  "Page number"
// @Param        limit    query  int     false  "Page size"
// @Success      200  {object}  map[string]interface{}  "{logs, total}"
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/logs [get]
func (h *ServiceLogHandler) List(c *gin.Context) {
	page, limit := parsePaginationQuery(c)
	entries, total, err := h.svc.List(services.ListLogsInput{
		Level:   c.Query("level"),
		Service: c.Query("service"),
		From:    c.Query("from"),
		To:      c.Query("to"),
		Page:    page,
		Limit:   limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": entries, "total": total})
}
