package handlers

import (
	"net/http"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	svc *services.SettingsService
}

func NewSettingsHandler(svc *services.SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// integrationsRequest is the write shape for both update and seed. API
// keys are write-only — an empty key on update means "leave unchanged".
type integrationsRequest struct {
	RadarrURL    string `json:"radarr_url"`
	RadarrAPIKey string `json:"radarr_api_key"`
	SonarrURL    string `json:"sonarr_url"`
	SonarrAPIKey string `json:"sonarr_api_key"`
}

// GetIntegrations returns the Radarr/Sonarr integration settings. Secrets
// are masked — only whether a key is set is exposed.
//
// @Summary      Get integration settings
// @Tags         settings
// @Produce      json
// @Success      200  {object}  services.IntegrationSettings
// @Security     BearerAuth
// @Router       /admin/settings/integrations [get]
func (h *SettingsHandler) GetIntegrations(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Integrations())
}

// UpdateIntegrations sets the Radarr/Sonarr URLs and (optionally) API
// keys. An empty API key leaves the stored key unchanged.
//
// @Summary      Update integration settings
// @Tags         settings
// @Accept       json
// @Produce      json
// @Param        body  body      integrationsRequest  true  "Integration settings"
// @Success      200   {object}  services.IntegrationSettings
// @Failure      400   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/settings/integrations [put]
func (h *SettingsHandler) UpdateIntegrations(c *gin.Context) {
	var req integrationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.UpdateIntegrations(req.RadarrURL, req.RadarrAPIKey, req.SonarrURL, req.SonarrAPIKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save settings"})
		return
	}
	c.JSON(http.StatusOK, h.svc.Integrations())
}

// SeedIntegrations bootstraps integration settings from the supplied
// values, applying each only when that key is not already set. Used by
// the deployment init step to migrate env-var config into the DB on
// first boot; safe to call repeatedly (a no-op once values exist).
//
// @Summary      Seed integration settings (set-if-absent)
// @Tags         settings
// @Accept       json
// @Produce      json
// @Param        body  body      integrationsRequest  true  "Integration settings"
// @Success      200   {object}  services.IntegrationSettings
// @Failure      400   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/settings/integrations/seed [post]
func (h *SettingsHandler) SeedIntegrations(c *gin.Context) {
	var req integrationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SeedIntegrations(req.RadarrURL, req.RadarrAPIKey, req.SonarrURL, req.SonarrAPIKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to seed settings"})
		return
	}
	c.JSON(http.StatusOK, h.svc.Integrations())
}
