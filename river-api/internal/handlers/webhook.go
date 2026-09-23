package handlers

import (
	"net/http"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	svc *services.WebhookService
}

func NewWebhookHandler(svc *services.WebhookService) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

type webhookRequest struct {
	Name    string   `json:"name" binding:"required"`
	URL     string   `json:"url" binding:"required"`
	Events  []string `json:"events"`
	Enabled *bool    `json:"enabled"`
}

func (r webhookRequest) enabled() bool {
	// Default new/updated webhooks to enabled unless explicitly disabled.
	return r.Enabled == nil || *r.Enabled
}

// List returns all webhooks (secret-free). Admin only.
//
// @Summary  List webhooks
// @Tags     admin
// @Produce  json
// @Success  200  {array}  services.WebhookView
// @Security BearerAuth
// @Router   /admin/webhooks [get]
func (h *WebhookHandler) List(c *gin.Context) {
	hooks, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, hooks)
}

// Create adds a webhook and returns its HMAC secret exactly once.
//
// @Summary  Create webhook
// @Tags     admin
// @Accept   json
// @Produce  json
// @Param    body  body      webhookRequest  true  "{name,url,events,enabled}"
// @Success  201   {object}  map[string]any  "{secret, webhook}"
// @Failure  400   {object}  map[string]string
// @Security BearerAuth
// @Router   /admin/webhooks [post]
func (h *WebhookHandler) Create(c *gin.Context) {
	var req webhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	view, secret, err := h.svc.Create(services.WebhookInput{Name: req.Name, URL: req.URL, Events: req.Events, Enabled: req.enabled()})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"secret": secret, "webhook": view})
}

// Update edits a webhook (name/url/events/enabled). Secret is unchanged.
//
// @Summary  Update webhook
// @Tags     admin
// @Accept   json
// @Produce  json
// @Param    id    path  string          true  "Webhook ID"
// @Param    body  body  webhookRequest  true  "fields"
// @Success  200   {object}  services.WebhookView
// @Failure  400   {object}  map[string]string
// @Failure  404   {object}  map[string]string
// @Security BearerAuth
// @Router   /admin/webhooks/{id} [put]
func (h *WebhookHandler) Update(c *gin.Context) {
	var req webhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	view, err := h.svc.Update(c.Param("id"), services.WebhookInput{Name: req.Name, URL: req.URL, Events: req.Events, Enabled: req.enabled()})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

// Delete removes a webhook.
//
// @Summary  Delete webhook
// @Tags     admin
// @Param    id  path  string  true  "Webhook ID"
// @Success  204
// @Failure  404  {object}  map[string]string
// @Security BearerAuth
// @Router   /admin/webhooks/{id} [delete]
func (h *WebhookHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Param("id")); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ListDeliveries returns recent delivery outcomes for a webhook. Admin only.
//
// @Summary  List webhook deliveries
// @Tags     admin
// @Produce  json
// @Param    id  path  string  true  "Webhook ID"
// @Success  200  {array}  models.WebhookDelivery
// @Security BearerAuth
// @Router   /admin/webhooks/{id}/deliveries [get]
func (h *WebhookHandler) ListDeliveries(c *gin.Context) {
	deliveries, err := h.svc.ListDeliveries(c.Param("id"), 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, deliveries)
}

// ListActive returns enabled webhooks WITH their secrets. Service-facing: only
// river-events (or an admin) calls it, to know where to deliver and how to sign.
//
// @Summary  List active webhooks (with secrets)
// @Tags     webhooks
// @Produce  json
// @Success  200  {array}  services.ActiveWebhook
// @Security BearerAuth
// @Router   /webhooks/active [get]
func (h *WebhookHandler) ListActive(c *gin.Context) {
	hooks, err := h.svc.ListActive()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, hooks)
}

type recordDeliveryRequest struct {
	WebhookID    string `json:"webhook_id" binding:"required"`
	Event        string `json:"event"`
	MediaID      string `json:"media_id"`
	Status       string `json:"status" binding:"required"`
	Attempts     int    `json:"attempts"`
	ResponseCode int    `json:"response_code"`
	Error        string `json:"error"`
}

// RecordDelivery persists a delivery outcome reported by river-events.
//
// @Summary  Record a webhook delivery outcome
// @Tags     webhooks
// @Accept   json
// @Param    body  body  recordDeliveryRequest  true  "outcome"
// @Success  204
// @Failure  400  {object}  map[string]string
// @Security BearerAuth
// @Router   /webhooks/deliveries [post]
func (h *WebhookHandler) RecordDelivery(c *gin.Context) {
	var req recordDeliveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.RecordDelivery(services.RecordDeliveryInput{
		WebhookID: req.WebhookID, Event: req.Event, MediaID: req.MediaID,
		Status: req.Status, Attempts: req.Attempts, ResponseCode: req.ResponseCode, Error: req.Error,
	}); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
