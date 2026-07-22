package handlers

import (
	"net/http"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type CreditsHandler struct {
	svc *services.CreditsService
}

func NewCreditsHandler(svc *services.CreditsService) *CreditsHandler {
	return &CreditsHandler{svc: svc}
}

type castEntryRequest struct {
	TmdbID      int    `json:"tmdb_id"`
	Name        string `json:"name" binding:"required"`
	ProfilePath string `json:"profile_path"`
	Biography   string `json:"biography"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
}

type crewEntryRequest struct {
	TmdbID      int    `json:"tmdb_id"`
	Name        string `json:"name" binding:"required"`
	ProfilePath string `json:"profile_path"`
	Biography   string `json:"biography"`
	Job         string `json:"job"`
	Department  string `json:"department"`
}

type creditsRequest struct {
	Cast []castEntryRequest `json:"cast"`
	Crew []crewEntryRequest `json:"crew"`
}

func (r creditsRequest) toCastInputs() []services.CastInput {
	out := make([]services.CastInput, len(r.Cast))
	for i, e := range r.Cast {
		out[i] = services.CastInput{TmdbID: e.TmdbID, Name: e.Name, ProfilePath: e.ProfilePath, Biography: e.Biography, Character: e.Character, Order: e.Order}
	}
	return out
}

func (r creditsRequest) toCrewInputs() []services.CrewInput {
	out := make([]services.CrewInput, len(r.Crew))
	for i, e := range r.Crew {
		out[i] = services.CrewInput{TmdbID: e.TmdbID, Name: e.Name, ProfilePath: e.ProfilePath, Biography: e.Biography, Job: e.Job, Department: e.Department}
	}
	return out
}

// GetPerson returns a person (actor/director/…) plus their credits.
//
// @Summary      Get person
// @Tags         credits
// @Produce      json
// @Param        id  path  string  true  "Person ID"
// @Success      200  {object}  object
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /people/{id} [get]
func (h *CreditsHandler) GetPerson(c *gin.Context) {
	res, err := h.svc.GetPerson(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetMovieCredits returns the cast + crew for a movie.
//
// @Summary      Get movie credits
// @Tags         credits
// @Produce      json
// @Param        id  path  string  true  "Movie ID"
// @Success      200  {object}  object  "{cast, crew}"
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/credits [get]
func (h *CreditsHandler) GetMovieCredits(c *gin.Context) {
	res, err := h.svc.GetMovieCredits(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, res)
}

// SetMovieCredits replaces the cast + crew lists for a movie.
//
// @Summary      Set movie credits
// @Tags         credits
// @Accept       json
// @Produce      json
// @Param        id    path  string          true  "Movie ID"
// @Param        body  body  creditsRequest  true  "{cast[], crew[]}"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/credits [put]
func (h *CreditsHandler) SetMovieCredits(c *gin.Context) {
	var req creditsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetMovieCredits(c.Param("id"), req.toCastInputs(), req.toCrewInputs()); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// GetTVShowCredits returns the cast + crew for a TV show.
//
// @Summary      Get TV show credits
// @Tags         credits
// @Produce      json
// @Param        id  path  string  true  "TV show ID"
// @Success      200  {object}  object  "{cast, crew}"
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/credits [get]
func (h *CreditsHandler) GetTVShowCredits(c *gin.Context) {
	res, err := h.svc.GetTVShowCredits(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, res)
}

// SetTVShowCredits replaces the cast + crew lists for a TV show.
//
// @Summary      Set TV show credits
// @Tags         credits
// @Accept       json
// @Produce      json
// @Param        id    path  string          true  "TV show ID"
// @Param        body  body  creditsRequest  true  "{cast[], crew[]}"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/credits [put]
func (h *CreditsHandler) SetTVShowCredits(c *gin.Context) {
	var req creditsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetTVShowCredits(c.Param("id"), req.toCastInputs(), req.toCrewInputs()); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
