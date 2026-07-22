package handlers

import (
	"net/http"
	"sort"
	"time"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type RecentlyAddedHandler struct {
	movies  *services.MovieService
	tvShows *services.TVShowService
}

func NewRecentlyAddedHandler(movies *services.MovieService, tvShows *services.TVShowService) *RecentlyAddedHandler {
	return &RecentlyAddedHandler{movies: movies, tvShows: tvShows}
}

type recentItem struct {
	ID           string    `json:"id"`
	MediaType    string    `json:"media_type"` // "movie" | "tvshow"
	Title        string    `json:"title"`
	Year         int       `json:"year"`
	Description  string    `json:"description"`
	Genres       string    `json:"genres"`
	Rating       float32   `json:"rating"`
	PosterPath   string    `json:"poster_path"`
	BackdropPath string    `json:"backdrop_path"`
	FilePath     string    `json:"file_path"` // empty for tvshow
	AddedAt      time.Time `json:"added_at"`
}

// List returns the home-screen "Recently Added" rail — newest movies +
// shows interleaved, eight per type.
//
// @Summary      Recently added
// @Tags         recently-added
// @Produce      json
// @Success      200  {array}   handlers.recentItem
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /recently-added [get]
func (h *RecentlyAddedHandler) List(c *gin.Context) {
	const perType = 8

	movies, err := h.movies.ListRecent(perType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch movies"})
		return
	}

	shows, err := h.tvShows.ListRecentShows(perType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch shows"})
		return
	}

	items := make([]recentItem, 0, len(movies)+len(shows))

	for _, m := range movies {
		if m.BackdropPath == "" && m.PosterPath == "" {
			continue
		}
		items = append(items, recentItem{
			ID:           m.ID.String(),
			MediaType:    "movie",
			Title:        m.Title,
			Year:         m.Year,
			Description:  m.Description,
			Genres:       m.Genres,
			Rating:       m.Rating,
			PosterPath:   m.PosterPath,
			BackdropPath: m.BackdropPath,
			FilePath:     m.FilePath,
			AddedAt:      m.CreatedAt,
		})
	}

	for _, s := range shows {
		if s.BackdropPath == "" && s.PosterPath == "" {
			continue
		}
		items = append(items, recentItem{
			ID:           s.ID.String(),
			MediaType:    "tvshow",
			Title:        s.Title,
			Year:         s.Year,
			Description:  s.Description,
			Genres:       s.Genres,
			Rating:       s.Rating,
			PosterPath:   s.PosterPath,
			BackdropPath: s.BackdropPath,
			AddedAt:      s.CreatedAt,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].AddedAt.After(items[j].AddedAt)
	})

	const maxItems = 10
	if len(items) > maxItems {
		items = items[:maxItems]
	}

	c.JSON(http.StatusOK, items)
}
