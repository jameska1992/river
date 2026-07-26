package handlers

import (
	"net/http"
	"testing"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func creditsRouter(repo *fakeCreditsRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewCreditsHandler(services.NewCreditsService(repo))
	r := gin.New()
	r.GET("/people/:id", h.GetPerson)
	r.GET("/movies/:id/credits", h.GetMovieCredits)
	r.PUT("/movies/:id/credits", h.SetMovieCredits)
	r.GET("/tvshows/:id/credits", h.GetTVShowCredits)
	r.PUT("/tvshows/:id/credits", h.SetTVShowCredits)
	return r
}

func TestCreditsHandler_GetPerson(t *testing.T) {
	pid := uuid.New()
	t.Run("found is 200", func(t *testing.T) {
		r := creditsRouter(&fakeCreditsRepo{person: &models.Person{Base: models.Base{ID: pid}, Name: "Boris Karloff"}})
		w := doJSON(r, http.MethodGet, "/people/"+pid.String(), "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Boris Karloff")
	})
	t.Run("non-uuid is 404", func(t *testing.T) {
		r := creditsRouter(&fakeCreditsRepo{})
		w := doJSON(r, http.MethodGet, "/people/not-a-uuid", "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
	t.Run("unknown person is 404", func(t *testing.T) {
		r := creditsRouter(&fakeCreditsRepo{personErr: services.ErrNotFound})
		w := doJSON(r, http.MethodGet, "/people/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestCreditsHandler_GetMovieCredits(t *testing.T) {
	personA := uuid.New()
	repo := &fakeCreditsRepo{movieCast: []models.MovieCast{{
		PersonID: personA, Character: "The Monster",
		Person: models.Person{Base: models.Base{ID: personA}, Name: "Boris Karloff"},
	}}}
	r := creditsRouter(repo)

	t.Run("valid id is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/movies/"+uuid.New().String()+"/credits", "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Boris Karloff")
	})
	t.Run("non-uuid is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/movies/nope/credits", "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestCreditsHandler_SetMovieCredits(t *testing.T) {
	r := creditsRouter(&fakeCreditsRepo{})
	body := `{"cast":[{"tmdb_id":100,"name":"Boris Karloff","character":"The Monster","order":1}],"crew":[{"name":"James Whale","job":"Director"}]}`

	t.Run("valid is 204", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/movies/"+uuid.New().String()+"/credits", body)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
	t.Run("malformed json is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/movies/"+uuid.New().String()+"/credits", `{"cast":`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("non-uuid is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/movies/nope/credits", body)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestCreditsHandler_GetTVShowCredits(t *testing.T) {
	personA := uuid.New()
	repo := &fakeCreditsRepo{tvCast: []models.TVShowCast{{
		PersonID: personA, Character: "Mulder",
		Person: models.Person{Base: models.Base{ID: personA}, Name: "David Duchovny"},
	}}}
	r := creditsRouter(repo)

	t.Run("valid id is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/tvshows/"+uuid.New().String()+"/credits", "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "David Duchovny")
	})
	t.Run("non-uuid is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/tvshows/nope/credits", "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestCreditsHandler_SetTVShowCredits(t *testing.T) {
	r := creditsRouter(&fakeCreditsRepo{})

	t.Run("valid is 204", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/tvshows/"+uuid.New().String()+"/credits", `{"cast":[{"name":"Gillian Anderson","character":"Scully"}]}`)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
	t.Run("non-uuid is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/tvshows/nope/credits", `{}`)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
