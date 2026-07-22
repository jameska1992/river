package handlers

import (
	"net/http"

	"river-api/internal/repository"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type SubtitleHandler struct {
	svc *services.SubtitleService
}

func NewSubtitleHandler(svc *services.SubtitleService) *SubtitleHandler {
	return &SubtitleHandler{svc: svc}
}

// ListMovieSubtitles returns all subtitle tracks for a movie.
//
// @Summary      List movie subtitles
// @Tags         subtitles
// @Produce      json
// @Param        id  path  string  true  "Movie ID"
// @Success      200  {array}   models.Subtitle
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/subtitles [get]
func (h *SubtitleHandler) ListMovieSubtitles(c *gin.Context) {
	subs, err := h.svc.ListByMedia("movie", c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, subs)
}

// ListEpisodeSubtitles returns all subtitle tracks for an episode.
//
// @Summary      List episode subtitles
// @Tags         subtitles
// @Produce      json
// @Param        id         path  string  true  "TV show ID"
// @Param        seasonId   path  string  true  "Season ID"
// @Param        episodeId  path  string  true  "Episode ID"
// @Success      200  {array}   models.Subtitle
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes/{episodeId}/subtitles [get]
func (h *SubtitleHandler) ListEpisodeSubtitles(c *gin.Context) {
	subs, err := h.svc.ListByMedia("episode", c.Param("episodeId"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, subs)
}

// Stream serves a WebVTT subtitle file.
//
// @Summary      Stream subtitle
// @Tags         subtitles
// @Produce      text/vtt
// @Param        id     path   string  true   "Subtitle ID"
// @Param        token  query  string  false  "Stream JWT"
// @Success      200
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /subtitles/{id}/stream [get]
func (h *SubtitleHandler) Stream(c *gin.Context) {
	sub, err := h.svc.FindByID(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/vtt; charset=utf-8")
	serveMediaFile(c, sub.FilePath)
}

type createSubtitleRequest struct {
	MediaType string `json:"media_type" binding:"required"`
	MediaID   string `json:"media_id"   binding:"required"`
	Language  string `json:"language"   binding:"required"`
	Label     string `json:"label"`
	FilePath  string `json:"file_path"  binding:"required"`
}

// Create registers a subtitle file with a movie or episode.
//
// @Summary      Create subtitle
// @Tags         subtitles
// @Accept       json
// @Produce      json
// @Param        body  body      createSubtitleRequest  true  "Subtitle metadata"
// @Success      201   {object}  models.Subtitle
// @Failure      400   {object}  map[string]string
// @Security     BearerAuth
// @Router       /subtitles [post]
func (h *SubtitleHandler) Create(c *gin.Context) {
	var req createSubtitleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sub, err := h.svc.Create(repository.SubtitleInput{
		MediaType: req.MediaType,
		MediaID:   req.MediaID,
		Language:  req.Language,
		Label:     req.Label,
		FilePath:  req.FilePath,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sub)
}

// Delete removes a subtitle row.
//
// @Summary      Delete subtitle
// @Tags         subtitles
// @Param        id  path  string  true  "Subtitle ID"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /subtitles/{id} [delete]
func (h *SubtitleHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Param("id")); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
