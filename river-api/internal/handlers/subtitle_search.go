package handlers

import (
	"net/http"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type SubtitleSearchHandler struct {
	svc *services.SubtitleSearchService
}

func NewSubtitleSearchHandler(svc *services.SubtitleSearchService) *SubtitleSearchHandler {
	return &SubtitleSearchHandler{svc: svc}
}

// attachSubtitleRequest is the body for downloading + attaching a chosen result.
type attachSubtitleRequest struct {
	URL      string `json:"url" binding:"required"`
	Language string `json:"language"`
	Label    string `json:"label"`
}

// SearchMovie searches the external provider for a movie's subtitles.
//
// @Summary      Search subtitles for a movie
// @Tags         subtitles
// @Produce      json
// @Param        id         path   string  true   "Movie ID"
// @Param        languages  query  string  false  "Comma-separated language codes, e.g. EN,FR"
// @Success      200  {array}   subdl.Subtitle
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/subtitles/search [get]
func (h *SubtitleSearchHandler) SearchMovie(c *gin.Context) {
	results, err := h.svc.SearchMovie(c.Request.Context(), c.Param("id"), c.Query("languages"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

// DownloadMovie downloads a chosen provider result and attaches it to a movie.
//
// @Summary      Attach a provider subtitle to a movie
// @Tags         subtitles
// @Accept       json
// @Produce      json
// @Param        id    path  string                 true  "Movie ID"
// @Param        body  body  attachSubtitleRequest  true  "{url, language, label}"
// @Success      201  {object}  models.Subtitle
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/subtitles/download [post]
func (h *SubtitleSearchHandler) DownloadMovie(c *gin.Context) {
	var req attachSubtitleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sub, err := h.svc.AttachMovie(c.Request.Context(), c.Param("id"), services.SubtitleAttachInput{
		URL: req.URL, Language: req.Language, Label: req.Label,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sub)
}

// SearchEpisode searches the external provider for an episode's subtitles.
//
// @Summary      Search subtitles for an episode
// @Tags         subtitles
// @Produce      json
// @Param        id         path   string  true   "TV show ID"
// @Param        seasonId   path   string  true   "Season ID"
// @Param        episodeId  path   string  true   "Episode ID"
// @Param        languages  query  string  false  "Comma-separated language codes, e.g. EN,FR"
// @Success      200  {array}   subdl.Subtitle
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes/{episodeId}/subtitles/search [get]
func (h *SubtitleSearchHandler) SearchEpisode(c *gin.Context) {
	results, err := h.svc.SearchEpisode(c.Request.Context(), c.Param("episodeId"), c.Query("languages"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

// DownloadEpisode downloads a chosen provider result and attaches it to an episode.
//
// @Summary      Attach a provider subtitle to an episode
// @Tags         subtitles
// @Accept       json
// @Produce      json
// @Param        id         path  string                 true  "TV show ID"
// @Param        seasonId   path  string                 true  "Season ID"
// @Param        episodeId  path  string                 true  "Episode ID"
// @Param        body       body  attachSubtitleRequest  true  "{url, language, label}"
// @Success      201  {object}  models.Subtitle
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes/{episodeId}/subtitles/download [post]
func (h *SubtitleSearchHandler) DownloadEpisode(c *gin.Context) {
	var req attachSubtitleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sub, err := h.svc.AttachEpisode(c.Request.Context(), c.Param("episodeId"), services.SubtitleAttachInput{
		URL: req.URL, Language: req.Language, Label: req.Label,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sub)
}
