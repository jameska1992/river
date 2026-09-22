package handlers

import (
	"net/http"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type ServiceKeyHandler struct {
	svc *services.ServiceKeyService
}

func NewServiceKeyHandler(svc *services.ServiceKeyService) *ServiceKeyHandler {
	return &ServiceKeyHandler{svc: svc}
}

type mintServiceKeyRequest struct {
	Name   string   `json:"name" binding:"required"`
	Scopes []string `json:"scopes"`
}

// seedServiceKeysRequest is the bulk create-if-absent shape used by the deploy
// init step. Scopes are optional — omitted means "derive from the service name".
type seedServiceKeysRequest struct {
	Keys []struct {
		Name   string   `json:"name"`
		Key    string   `json:"key"`
		Scopes []string `json:"scopes"`
	} `json:"keys"`
}

// List returns all service keys (secret-free) for the admin UI.
//
// @Summary      List service keys
// @Tags         admin
// @Produce      json
// @Success      200  {array}   services.ServiceKeyView
// @Security     BearerAuth
// @Router       /admin/service-keys [get]
func (h *ServiceKeyHandler) List(c *gin.Context) {
	keys, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, keys)
}

// Mint creates a new key and returns the plaintext exactly once.
//
// @Summary      Mint a service key
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body      mintServiceKeyRequest  true  "{name, scopes}"
// @Success      201   {object}  map[string]any  "{key, service_key}"
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/service-keys [post]
func (h *ServiceKeyHandler) Mint(c *gin.Context) {
	var req mintServiceKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plaintext, view, err := h.svc.Mint(req.Name, req.Scopes)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	// The plaintext is returned here and never again — the client must show it
	// to the admin to copy into the service's env.
	c.JSON(http.StatusCreated, gin.H{"key": plaintext, "service_key": view})
}

// Revoke disables a key. Idempotent from the caller's view (404 if unknown).
//
// @Summary      Revoke a service key
// @Tags         admin
// @Param        id  path  string  true  "Service key ID"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/service-keys/{id} [delete]
func (h *ServiceKeyHandler) Revoke(c *gin.Context) {
	if err := h.svc.Revoke(c.Param("id")); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// Seed bulk-provisions keys from env (create-if-absent), for the deploy init
// step. Never clobbers an existing key, so it's safe to run on every boot.
//
// @Summary      Seed service keys (set-if-absent)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body      seedServiceKeysRequest  true  "{keys:[{name,key,scopes}]}"
// @Success      200   {object}  map[string]any  "{created}"
// @Failure      400   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/service-keys/seed [post]
func (h *ServiceKeyHandler) Seed(c *gin.Context) {
	var req seedServiceKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created := 0
	for _, k := range req.Keys {
		ok, err := h.svc.Seed(k.Name, k.Key, k.Scopes)
		if err != nil {
			// A malformed entry (bad scope / missing prefix) shouldn't abort
			// the whole batch — report it and continue seeding the rest.
			c.Header("X-Seed-Warning", err.Error())
			continue
		}
		if ok {
			created++
		}
	}
	c.JSON(http.StatusOK, gin.H{"created": created})
}
