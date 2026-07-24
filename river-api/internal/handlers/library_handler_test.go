package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLibraryRepo is an in-memory repository.LibraryRepository so the
// handler tests can exercise the full handler -> service -> repo stack
// (request binding + serviceStatus error mapping) without a database.
type fakeLibraryRepo struct{ libs []*models.Library }

func (f *fakeLibraryRepo) FindAll() ([]models.Library, error) {
	out := make([]models.Library, 0, len(f.libs))
	for _, l := range f.libs {
		out = append(out, *l)
	}
	return out, nil
}
func (f *fakeLibraryRepo) FindByID(id string) (*models.Library, error) {
	for _, l := range f.libs {
		if l.ID.String() == id {
			return l, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeLibraryRepo) Create(l *models.Library) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	f.libs = append(f.libs, l)
	return nil
}
func (f *fakeLibraryRepo) Save(l *models.Library) error { return nil }
func (f *fakeLibraryRepo) Delete(id string) error {
	for i, l := range f.libs {
		if l.ID.String() == id {
			f.libs = append(f.libs[:i], f.libs[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

func libraryRouter(repo *fakeLibraryRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewLibraryHandler(services.NewLibraryService(repo))
	r := gin.New()
	r.GET("/libraries", h.List)
	r.POST("/libraries", h.Create)
	r.GET("/libraries/:id", h.Get)
	r.PUT("/libraries/:id", h.Update)
	r.DELETE("/libraries/:id", h.Delete)
	return r
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLibraryHandler_Create_ValidationErrors(t *testing.T) {
	r := libraryRouter(&fakeLibraryRepo{})

	t.Run("missing name and type", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/libraries", `{}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid media type", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/libraries", `{"name":"X","type":"podcast"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code, "type must be one of the allowed media types")
	})
}

func TestLibraryHandler_Create_Success(t *testing.T) {
	repo := &fakeLibraryRepo{}
	r := libraryRouter(repo)

	w := doJSON(r, http.MethodPost, "/libraries", `{"name":"Movies","type":"movie"}`)
	require.Equal(t, http.StatusCreated, w.Code)

	var got models.Library
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Movies", got.Name)
	assert.Equal(t, "[]", got.Paths, "empty paths should default to a JSON empty array")
	assert.Len(t, repo.libs, 1)
}

func TestLibraryHandler_Get(t *testing.T) {
	existing := &models.Library{Base: models.Base{ID: uuid.New()}, Name: "Movies", Type: "movie", Paths: "[]"}
	r := libraryRouter(&fakeLibraryRepo{libs: []*models.Library{existing}})

	t.Run("found returns 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/libraries/"+existing.ID.String(), "")
		require.Equal(t, http.StatusOK, w.Code)
		var got models.Library
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		assert.Equal(t, "Movies", got.Name)
	})

	t.Run("missing maps ErrNotFound to 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/libraries/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestLibraryHandler_List(t *testing.T) {
	repo := &fakeLibraryRepo{libs: []*models.Library{
		{Base: models.Base{ID: uuid.New()}, Name: "Movies", Type: "movie"},
		{Base: models.Base{ID: uuid.New()}, Name: "Shows", Type: "tvshow"},
	}}
	r := libraryRouter(repo)

	w := doJSON(r, http.MethodGet, "/libraries", "")
	require.Equal(t, http.StatusOK, w.Code)
	var got []models.Library
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 2)
}

func TestLibraryHandler_Update_NotFound(t *testing.T) {
	r := libraryRouter(&fakeLibraryRepo{})
	w := doJSON(r, http.MethodPut, "/libraries/"+uuid.New().String(), `{"name":"X","type":"movie"}`)
	assert.Equal(t, http.StatusNotFound, w.Code, "updating a missing library should map ErrNotFound to 404")
}
