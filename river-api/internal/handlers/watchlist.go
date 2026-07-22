package handlers

import (
	"net/http"

	"river-api/internal/middleware"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type WatchlistHandler struct {
	svc *services.WatchlistService
}

func NewWatchlistHandler(svc *services.WatchlistService) *WatchlistHandler {
	return &WatchlistHandler{svc: svc}
}

type watchlistAddRequest struct {
	MediaType string `json:"media_type" binding:"required,oneof=movie tvshow audiobook"`
	MediaID   string `json:"media_id"   binding:"required"`
}

// List returns the calling user's watchlist.
//
// @Summary      List watchlist
// @Tags         watchlist
// @Produce      json
// @Success      200  {array}   object
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /watchlist [get]
func (h *WatchlistHandler) List(c *gin.Context) {
	claims := middleware.GetClaims(c)
	entries, err := h.svc.List(claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch watchlist"})
		return
	}
	c.JSON(http.StatusOK, entries)
}

// Add adds a media item to the watchlist.
//
// @Summary      Add to watchlist
// @Tags         watchlist
// @Accept       json
// @Produce      json
// @Param        body  body      watchlistAddRequest  true  "Media reference"
// @Success      201   {object}  models.WatchlistItem
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /watchlist [post]
func (h *WatchlistHandler) Add(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req watchlistAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry, err := h.svc.Add(claims.UserID, req.MediaType, req.MediaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add to watchlist"})
		return
	}
	c.JSON(http.StatusCreated, entry)
}

// Remove deletes a watchlist entry.
//
// @Summary      Remove from watchlist
// @Tags         watchlist
// @Param        id  path  string  true  "Watchlist entry ID"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /watchlist/{id} [delete]
func (h *WatchlistHandler) Remove(c *gin.Context) {
	claims := middleware.GetClaims(c)
	itemID := c.Param("id")
	if err := h.svc.Remove(claims.UserID, itemID); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "failed to remove from watchlist"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
