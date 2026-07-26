package handlers

import (
	"errors"
	"net/http"
	"testing"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func serviceLogRouter(repo *fakeServiceLogRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewServiceLogHandler(services.NewServiceLogService(repo))
	r := gin.New()
	r.POST("/logs", h.Create)
	r.GET("/admin/logs", h.List)
	return r
}

func TestServiceLogHandler_Create(t *testing.T) {
	repo := &fakeServiceLogRepo{}
	r := serviceLogRouter(repo)

	t.Run("valid is 204 and persists", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/logs", `{"level":"info","service":"river-scan","message":"scan complete"}`)
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Len(t, repo.entries, 1)
	})
	t.Run("missing fields is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/logs", `{"level":"info"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("repo error is 500", func(t *testing.T) {
		bad := serviceLogRouter(&fakeServiceLogRepo{createErr: errors.New("db down")})
		w := doJSON(bad, http.MethodPost, "/logs", `{"level":"info","service":"s","message":"m"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestServiceLogHandler_List(t *testing.T) {
	repo := &fakeServiceLogRepo{}
	// Seed a couple of entries via the handler so List has something to return.
	r := serviceLogRouter(repo)
	doJSON(r, http.MethodPost, "/logs", `{"level":"warn","service":"river-api","message":"a"}`)
	doJSON(r, http.MethodPost, "/logs", `{"level":"error","service":"river-api","message":"b"}`)

	t.Run("returns entries and total", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/admin/logs?level=error&service=river-api&page=1&limit=10", "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"total":2`)
	})
	t.Run("repo error is 500", func(t *testing.T) {
		bad := serviceLogRouter(&fakeServiceLogRepo{listErr: errors.New("db down")})
		w := doJSON(bad, http.MethodGet, "/admin/logs", "")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
	t.Run("unparseable date filter is silently ignored (200)", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/admin/logs?from=not-a-date&to=also-bad", "")
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
