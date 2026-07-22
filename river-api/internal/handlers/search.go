package handlers

import (
	"net/http"
	"strings"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	svc *services.SearchService
}

func NewSearchHandler(svc *services.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// Search runs a fuzzy text + genre search across libraries and people.
// At least one of q or genre must be set; an empty query returns
// empty results without hitting the DB.
//
// @Summary      Global search
// @Tags         search
// @Produce      json
// @Param        q      query  string  false  "Free-text query"
// @Param        genre  query  string  false  "Genre filter"
// @Success      200  {object}  map[string]interface{}  "{libraries, people}"
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	genre := strings.TrimSpace(c.Query("genre"))
	if q == "" && genre == "" {
		c.JSON(http.StatusOK, gin.H{"libraries": []any{}, "people": []any{}})
		return
	}
	res, err := h.svc.Search(q, genre)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}
