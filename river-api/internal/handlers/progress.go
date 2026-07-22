package handlers

import (
	"net/http"
	"strconv"

	"river-api/internal/middleware"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type ProgressHandler struct {
	svc *services.ProgressService
}

func NewProgressHandler(svc *services.ProgressService) *ProgressHandler {
	return &ProgressHandler{svc: svc}
}

type progressRequest struct {
	MediaType string  `json:"media_type" binding:"required,oneof=movie episode chapter"`
	MediaID   string  `json:"media_id" binding:"required"`
	Position  float64 `json:"position" binding:"min=0"`
	Duration  float64 `json:"duration" binding:"min=0"`
}

// Get returns the calling user's watch progress for one media item.
//
// @Summary      Get watch progress
// @Tags         progress
// @Produce      json
// @Param        media_type  query  string  true  "movie | episode | chapter"
// @Param        media_id    query  string  true  "Media ID"
// @Success      200  {object}  models.WatchProgress
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress [get]
func (h *ProgressHandler) Get(c *gin.Context) {
	claims := middleware.GetClaims(c)
	mediaType := c.Query("media_type")
	mediaID := c.Query("media_id")
	if mediaType == "" || mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "media_type and media_id are required"})
		return
	}
	p, err := h.svc.Get(claims.UserID, mediaType, mediaID)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// GetAll returns all progress records of a given media type for the
// calling user.
//
// @Summary      List watch progress
// @Tags         progress
// @Produce      json
// @Param        media_type  query  string  true  "movie | episode | chapter"
// @Success      200  {array}   models.WatchProgress
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/all [get]
func (h *ProgressHandler) GetAll(c *gin.Context) {
	claims := middleware.GetClaims(c)
	mediaType := c.Query("media_type")
	if mediaType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "media_type is required"})
		return
	}
	items, err := h.svc.GetAllByType(claims.UserID, mediaType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch progress"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// ContinueWatching returns the user's in-progress media list ordered
// for the home-screen rail.
//
// @Summary      Continue Watching list
// @Tags         progress
// @Produce      json
// @Success      200  {array}   object
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/continue-watching [get]
func (h *ProgressHandler) ContinueWatching(c *gin.Context) {
	claims := middleware.GetClaims(c)
	items, err := h.svc.ContinueWatching(claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch continue watching"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// NextUp returns the "next episode to start" for shows the user has
// recently completed an episode of, ordered by recency, capped at
// limit (default 16, max 50).
//
// @Summary      Next Up list
// @Tags         progress
// @Produce      json
// @Param        limit  query  int  false  "1..50, default 16"
// @Success      200  {array}   object
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/next-up [get]
func (h *ProgressHandler) NextUp(c *gin.Context) {
	claims := middleware.GetClaims(c)
	limit := 16
	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			if n > 50 {
				n = 50
			}
			limit = n
		}
	}
	items, err := h.svc.NextUp(claims.UserID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch next up"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// DismissNextUp hides a specific episode from the Next Up rail. Idem-
// potent — repeated calls return 204 with no error.
//
// @Summary      Dismiss a Next Up entry
// @Tags         progress
// @Param        episode_id  path  string  true  "Episode ID"
// @Success      204
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/next-up/{episode_id}/dismiss [post]
func (h *ProgressHandler) DismissNextUp(c *gin.Context) {
	claims := middleware.GetClaims(c)
	episodeID := c.Param("episode_id")
	if err := h.svc.DismissNextUp(claims.UserID, episodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to dismiss"})
		return
	}
	c.Status(http.StatusNoContent)
}

// UndismissNextUp reverses a dismissal. Returns 404 if the episode
// wasn't dismissed — lets the client tell "undo done" from "nothing to
// undo."
//
// @Summary      Undo a Next Up dismissal
// @Tags         progress
// @Param        episode_id  path  string  true  "Episode ID"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/next-up/{episode_id}/dismiss [delete]
func (h *ProgressHandler) UndismissNextUp(c *gin.Context) {
	claims := middleware.GetClaims(c)
	episodeID := c.Param("episode_id")
	if err := h.svc.UndismissNextUp(claims.UserID, episodeID); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "not dismissed"})
		return
	}
	c.Status(http.StatusNoContent)
}

// NextEpisode returns the next unwatched episode for a show, or the
// first episode if nothing has been watched.
//
// @Summary      Next episode
// @Tags         tvshows
// @Produce      json
// @Param        id  path  string  true  "TV show ID"
// @Success      200  {object}  map[string]string  "{season_id, episode_id}"
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/next-episode [get]
func (h *ProgressHandler) NextEpisode(c *gin.Context) {
	showID := c.Param("id")
	claims := middleware.GetClaims(c)
	result, err := h.svc.NextEpisode(claims.UserID, showID)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "no episodes found"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ActiveSessions returns currently-active playback sessions across all
// users (admin-only dashboard).
//
// @Summary      Active sessions
// @Tags         admin
// @Produce      json
// @Success      200  {array}   object
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/active-sessions [get]
func (h *ProgressHandler) ActiveSessions(c *gin.Context) {
	items, err := h.svc.ActiveSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch active sessions"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// SetCompleted explicitly marks a media item watched or unwatched,
// bypassing the position/duration heuristic that the WS stream uses.
//
// @Summary      Set watch progress completed flag
// @Tags         progress
// @Accept       json
// @Produce      json
// @Param        body  body  completedRequest  true  "Completed payload"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/completed [put]
func (h *ProgressHandler) SetCompleted(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req completedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetCompleted(claims.UserID, req.MediaType, req.MediaID, req.Completed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update progress"})
		return
	}
	c.Status(http.StatusNoContent)
}

type completedRequest struct {
	MediaType string `json:"media_type" binding:"required,oneof=movie episode chapter"`
	MediaID   string `json:"media_id" binding:"required"`
	Completed bool   `json:"completed"`
}

type showCompletedRequest struct {
	ShowID    string `json:"show_id" binding:"required"`
	Completed bool   `json:"completed"`
}

// SetShowCompleted marks every episode of a show as watched/unwatched in
// one call. Used by the library UI's per-show watched toggle.
//
// @Summary      Set show watched (cascades to all episodes)
// @Tags         progress
// @Accept       json
// @Produce      json
// @Param        body  body  showCompletedRequest  true  "Payload"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/show-completed [put]
func (h *ProgressHandler) SetShowCompleted(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req showCompletedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetShowCompleted(claims.UserID, req.ShowID, req.Completed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update show progress"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ShowStates returns per-show {total, completed} episode counts for the
// caller, optionally filtered to a single library. One round-trip lets
// the library UI render the watched indicator on every card without N
// per-show progress lookups.
//
// @Summary      Per-show watch states
// @Tags         progress
// @Produce      json
// @Param        library_id  query  string  false  "Limit to a single library"
// @Success      200  {array}  services.ShowWatchState
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/show-states [get]
func (h *ProgressHandler) ShowStates(c *gin.Context) {
	claims := middleware.GetClaims(c)
	states, err := h.svc.ShowWatchStates(claims.UserID, c.Query("library_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch show states"})
		return
	}
	c.JSON(http.StatusOK, states)
}

// ShowState returns the watched-state summary for a single show.
//
// @Summary      Watch state for a single show
// @Tags         progress
// @Produce      json
// @Param        show_id  query  string  true  "TV show ID"
// @Success      200  {object}  services.ShowWatchState
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/show-state [get]
func (h *ProgressHandler) ShowState(c *gin.Context) {
	claims := middleware.GetClaims(c)
	showID := c.Query("show_id")
	if showID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "show_id is required"})
		return
	}
	state, err := h.svc.GetShowWatchState(claims.UserID, showID)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "failed to fetch show state"})
		return
	}
	c.JSON(http.StatusOK, state)
}

// Delete removes a single progress record.
//
// @Summary      Delete watch progress
// @Tags         progress
// @Param        media_type  query  string  true  "movie | episode | chapter"
// @Param        media_id    query  string  true  "Media ID"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress [delete]
func (h *ProgressHandler) Delete(c *gin.Context) {
	claims := middleware.GetClaims(c)
	mediaType := c.Query("media_type")
	mediaID := c.Query("media_id")
	if mediaType == "" || mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "media_type and media_id are required"})
		return
	}
	if err := h.svc.Delete(claims.UserID, mediaType, mediaID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete progress"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
