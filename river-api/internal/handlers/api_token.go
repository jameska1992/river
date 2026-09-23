package handlers

import (
	"net/http"
	"time"

	"river-api/internal/middleware"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type APITokenHandler struct {
	svc *services.APITokenService
}

func NewAPITokenHandler(svc *services.APITokenService) *APITokenHandler {
	return &APITokenHandler{svc: svc}
}

type mintAPITokenRequest struct {
	Name string `json:"name" binding:"required"`
	// ExpiresInDays optionally expires the token after N days (0 = never).
	ExpiresInDays int      `json:"expires_in_days"`
	Scopes        []string `json:"scopes"`
}

// List returns all API tokens (secret-free). Admin only.
//
// @Summary  List API tokens
// @Tags     admin
// @Produce  json
// @Success  200  {array}  services.APITokenView
// @Security BearerAuth
// @Router   /admin/api-tokens [get]
func (h *APITokenHandler) List(c *gin.Context) {
	toks, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toks)
}

// Mint creates a read-only API token owned by the acting admin and returns the
// plaintext exactly once.
//
// @Summary  Mint an API token
// @Tags     admin
// @Accept   json
// @Produce  json
// @Param    body  body      mintAPITokenRequest  true  "{name, expires_in_days, scopes}"
// @Success  201   {object}  map[string]any  "{token, api_token}"
// @Failure  400   {object}  map[string]string
// @Security BearerAuth
// @Router   /admin/api-tokens [post]
func (h *APITokenHandler) Mint(c *gin.Context) {
	var req mintAPITokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	claims := middleware.GetClaims(c)
	owner, err := uuid.Parse(claims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not resolve token owner"})
		return
	}
	var expiresAt *time.Time
	if req.ExpiresInDays > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresInDays)
		expiresAt = &t
	}
	plaintext, view, err := h.svc.Mint(owner, req.Name, req.Scopes, expiresAt)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": plaintext, "api_token": view})
}

// Revoke disables a token.
//
// @Summary  Revoke an API token
// @Tags     admin
// @Param    id  path  string  true  "API token ID"
// @Success  204
// @Failure  404  {object}  map[string]string
// @Security BearerAuth
// @Router   /admin/api-tokens/{id} [delete]
func (h *APITokenHandler) Revoke(c *gin.Context) {
	if err := h.svc.Revoke(c.Param("id")); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
