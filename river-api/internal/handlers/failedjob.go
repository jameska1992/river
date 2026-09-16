package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

// FailedJobHandler serves the operator-facing "failed jobs" surface backed by
// the dead-letter queue: services report parked messages here, and admins list
// / retry / dismiss them. Retry re-publishes the original event by delegating
// to river-scan's /republish endpoint (river-api itself has no RabbitMQ
// dependency — same delegation pattern as requeue-untranscoded).
type FailedJobHandler struct {
	svc     *services.FailedJobService
	scanURL string
	http    *http.Client
}

func NewFailedJobHandler(svc *services.FailedJobService, scanURL string) *FailedJobHandler {
	return &FailedJobHandler{
		svc:     svc,
		scanURL: strings.TrimRight(scanURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

type reportFailedJobReq struct {
	Service    string `json:"service" binding:"required"`
	MediaType  string `json:"media_type"`
	SourcePath string `json:"source_path"`
	Reason     string `json:"reason"`
	Attempts   int    `json:"attempts"`
	RoutingKey string `json:"routing_key"`
	// Event is the raw MediaDiscoveredEvent JSON, replayed on retry.
	Event string `json:"event"`
}

// Report records a dead-lettered job. Called by the services (role=service)
// when a consumer parks a message after exhausting retries.
//
// @Summary      Report a failed (dead-lettered) ingest job
// @Tags         failed-jobs
// @Accept       json
// @Produce      json
// @Param        body  body  reportFailedJobReq  true  "Failed job"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /failed-jobs [post]
func (h *FailedJobHandler) Report(c *gin.Context) {
	var req reportFailedJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := h.svc.Report(services.ReportFailedJobInput{
		Service:    req.Service,
		MediaType:  req.MediaType,
		SourcePath: req.SourcePath,
		Reason:     req.Reason,
		Attempts:   req.Attempts,
		RoutingKey: req.RoutingKey,
		Event:      req.Event,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// List returns failed jobs, most recent first, with optional service /
// media_type filters.
//
// @Summary      List failed ingest jobs
// @Tags         failed-jobs
// @Produce      json
// @Success      200  {object}  object  "{jobs, total}"
// @Security     BearerAuth
// @Router       /admin/failed-jobs [get]
func (h *FailedJobHandler) List(c *gin.Context) {
	page, limit := parsePaginationQuery(c)
	jobs, total, err := h.svc.List(services.ListFailedJobsInput{
		Service:   c.Query("service"),
		MediaType: c.Query("media_type"),
		Page:      page,
		Limit:     limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs, "total": total})
}

// Retry re-publishes the job's original event (via river-scan) and, on
// success, removes it from the failed list. If it fails again it'll be
// re-reported as a fresh failed job.
//
// @Summary      Retry a failed ingest job
// @Tags         failed-jobs
// @Produce      json
// @Param        id  path  string  true  "Failed job ID"
// @Success      202
// @Failure      404  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/failed-jobs/{id}/retry [post]
func (h *FailedJobHandler) Retry(c *gin.Context) {
	id := c.Param("id")
	job, err := h.svc.Get(id)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "failed job not found"})
		return
	}
	if strings.TrimSpace(job.Event) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "job has no stored event to replay"})
		return
	}
	if err := h.republish(job.Event); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("republish failed: %v", err)})
		return
	}
	// Re-queued successfully — drop it from the failed list. A repeat failure
	// will surface as a new entry.
	if err := h.svc.Dismiss(id); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "retry queued"})
}

// Dismiss removes a failed job without retrying it.
//
// @Summary      Dismiss a failed ingest job
// @Tags         failed-jobs
// @Param        id  path  string  true  "Failed job ID"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/failed-jobs/{id} [delete]
func (h *FailedJobHandler) Dismiss(c *gin.Context) {
	if err := h.svc.Dismiss(c.Param("id")); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "failed job not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// republish POSTs the stored event JSON to river-scan, which owns the
// RabbitMQ publisher and routes it by the event's library type.
func (h *FailedJobHandler) republish(event string) error {
	if h.scanURL == "" {
		return errors.New("scan service URL not configured")
	}
	resp, err := h.http.Post(h.scanURL+"/republish", "application/json", bytes.NewReader([]byte(event)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("river-scan returned %d", resp.StatusCode)
	}
	return nil
}
