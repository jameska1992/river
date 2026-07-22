package handlers

import (
	"net/http"

	"river-api/internal/repository"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type AudioTrackHandler struct {
	svc *services.AudioTrackService
}

func NewAudioTrackHandler(svc *services.AudioTrackService) *AudioTrackHandler {
	return &AudioTrackHandler{svc: svc}
}

// ListMovieAudioTracks returns the standalone audio tracks for a movie.
//
// @Summary      List movie audio tracks
// @Tags         audio-tracks
// @Produce      json
// @Param        id  path  string  true  "Movie ID"
// @Success      200  {array}   models.AudioTrack
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/audio-tracks [get]
func (h *AudioTrackHandler) ListMovieAudioTracks(c *gin.Context) {
	tracks, err := h.svc.ListByMedia("movie", c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tracks)
}

// ListEpisodeAudioTracks returns the standalone audio tracks for an episode.
//
// @Summary      List episode audio tracks
// @Tags         audio-tracks
// @Produce      json
// @Param        id         path  string  true  "TV show ID"
// @Param        seasonId   path  string  true  "Season ID"
// @Param        episodeId  path  string  true  "Episode ID"
// @Success      200  {array}   models.AudioTrack
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes/{episodeId}/audio-tracks [get]
func (h *AudioTrackHandler) ListEpisodeAudioTracks(c *gin.Context) {
	tracks, err := h.svc.ListByMedia("episode", c.Param("episodeId"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tracks)
}

// Stream serves a standalone audio-track file.
//
// @Summary      Stream audio track
// @Tags         audio-tracks
// @Produce      audio/mp4
// @Param        id     path   string  true   "Audio track ID"
// @Param        token  query  string  false  "Stream JWT"
// @Success      200
// @Success      206
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audio-tracks/{id}/stream [get]
func (h *AudioTrackHandler) Stream(c *gin.Context) {
	track, err := h.svc.FindByID(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	serveMediaFile(c, track.FilePath)
}

type createAudioTrackRequest struct {
	MediaType   string `json:"media_type"   binding:"required"`
	MediaID     string `json:"media_id"     binding:"required"`
	Language    string `json:"language"     binding:"required"`
	Label       string `json:"label"`
	StreamIndex int    `json:"stream_index"`
	FilePath    string `json:"file_path"    binding:"required"`
}

// Create registers a standalone audio-track file with a movie or episode.
//
// @Summary      Create audio track
// @Tags         audio-tracks
// @Accept       json
// @Produce      json
// @Param        body  body      createAudioTrackRequest  true  "Audio track metadata"
// @Success      201   {object}  models.AudioTrack
// @Failure      400   {object}  map[string]string
// @Security     BearerAuth
// @Router       /audio-tracks [post]
func (h *AudioTrackHandler) Create(c *gin.Context) {
	var req createAudioTrackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	track, err := h.svc.Create(repository.AudioTrackInput{
		MediaType:   req.MediaType,
		MediaID:     req.MediaID,
		Language:    req.Language,
		Label:       req.Label,
		StreamIndex: req.StreamIndex,
		FilePath:    req.FilePath,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, track)
}

// Delete removes an audio-track row.
//
// @Summary      Delete audio track
// @Tags         audio-tracks
// @Param        id  path  string  true  "Audio track ID"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audio-tracks/{id} [delete]
func (h *AudioTrackHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Param("id")); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
