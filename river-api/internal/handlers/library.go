package handlers

import (
	"net/http"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type LibraryHandler struct {
	svc *services.LibraryService
}

func NewLibraryHandler(svc *services.LibraryService) *LibraryHandler {
	return &LibraryHandler{svc: svc}
}

type libraryRequest struct {
	Name  string             `json:"name" binding:"required"`
	Type  models.LibraryType `json:"type" binding:"required,oneof=movie tvshow music audiobook"`
	Paths string             `json:"paths"`
	// PreTranscoded marks the library as already transcoded — scanning
	// and metadata continue, but the video/audio transcoders skip the
	// event. See models.Library for the full semantics.
	PreTranscoded bool `json:"pre_transcoded"`
}

// List returns every configured library (movie / tvshow / music / audiobook).
//
// @Summary      List libraries
// @Tags         libraries
// @Produce      json
// @Success      200  {array}   models.Library
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /libraries [get]
func (h *LibraryHandler) List(c *gin.Context) {
	libs, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch libraries"})
		return
	}
	c.JSON(http.StatusOK, libs)
}

// Create adds a new library. Admin only.
//
// @Summary      Create library
// @Tags         libraries
// @Accept       json
// @Produce      json
// @Param        body  body      libraryRequest  true  "Library details"
// @Success      201   {object}  models.Library
// @Failure      400   {object}  map[string]string
// @Failure      403   {object}  map[string]string
// @Security     BearerAuth
// @Router       /libraries [post]
func (h *LibraryHandler) Create(c *gin.Context) {
	var req libraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	lib, err := h.svc.Create(services.LibraryInput{
		Name:          req.Name,
		Type:          req.Type,
		Paths:         req.Paths,
		PreTranscoded: req.PreTranscoded,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create library"})
		return
	}
	c.JSON(http.StatusCreated, lib)
}

// Get returns a single library by ID.
//
// @Summary      Get library
// @Tags         libraries
// @Produce      json
// @Param        id   path      string  true  "Library ID"
// @Success      200  {object}  models.Library
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /libraries/{id} [get]
func (h *LibraryHandler) Get(c *gin.Context) {
	lib, err := h.svc.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "library not found"})
		return
	}
	c.JSON(http.StatusOK, lib)
}

// Update modifies a library. Admin only.
//
// @Summary      Update library
// @Tags         libraries
// @Accept       json
// @Produce      json
// @Param        id    path      string          true  "Library ID"
// @Param        body  body      libraryRequest  true  "Library details"
// @Success      200   {object}  models.Library
// @Failure      400   {object}  map[string]string
// @Failure      403   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /libraries/{id} [put]
func (h *LibraryHandler) Update(c *gin.Context) {
	var req libraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	lib, err := h.svc.Update(c.Param("id"), services.LibraryInput{
		Name:          req.Name,
		Type:          req.Type,
		Paths:         req.Paths,
		PreTranscoded: req.PreTranscoded,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, lib)
}

// Delete removes a library. Admin only.
//
// @Summary      Delete library
// @Tags         libraries
// @Param        id  path  string  true  "Library ID"
// @Success      204
// @Failure      403  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /libraries/{id} [delete]
func (h *LibraryHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete library"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
