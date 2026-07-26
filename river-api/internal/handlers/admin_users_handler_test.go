package handlers

import (
	"net/http"
	"testing"
	"time"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func adminUsersRouter(users *fakeUserRepo, callerID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	authSvc := services.NewAuthService(users, &fakeRefreshRepo{}, "test-secret", 15*time.Minute, 7*24*time.Hour, 8*time.Hour)
	progressSvc := services.NewProgressService(&fakeProgressRepo{}, &fakeMovieRepo{}, &fakeEpisodeRepo{}, &fakeSeasonRepo{}, &fakeShowRepo{}, users, &fakeAudiobookRepo{}, &fakeChapterRepo{}, &fakeDismissedRepo{})
	h := NewAdminUsersHandler(authSvc, progressSvc)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", callerID) })
	r.GET("/admin/users", h.ListUsers)
	r.POST("/admin/users", h.CreateUser)
	r.PUT("/admin/users/:id", h.UpdateUser)
	r.POST("/admin/users/:id/set-password", h.SetPassword)
	r.DELETE("/admin/users/:id", h.DeleteUser)
	r.GET("/admin/users/:id/activity", h.GetActivity)
	return r
}

func TestAdminUsersHandler_ListUsers(t *testing.T) {
	users := &fakeUserRepo{users: []*models.User{{Base: models.Base{ID: uuid.New()}, Username: "alice"}}}
	w := doJSON(adminUsersRouter(users, ""), http.MethodGet, "/admin/users", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUsersHandler_CreateUser(t *testing.T) {
	r := adminUsersRouter(&fakeUserRepo{}, "")

	t.Run("valid is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/admin/users", `{"username":"carol","email":"carol@example.com","password":"password1","role":"user"}`)
		assert.Equal(t, http.StatusCreated, w.Code)
	})
	t.Run("short password is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/admin/users", `{"username":"carol","email":"carol@example.com","password":"short","role":"user"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("bad role is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/admin/users", `{"username":"carol","email":"carol@example.com","password":"password1","role":"wizard"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAdminUsersHandler_UpdateUser(t *testing.T) {
	u := &models.User{Base: models.Base{ID: uuid.New()}, Username: "grace", Role: "user"}
	r := adminUsersRouter(&fakeUserRepo{users: []*models.User{u}}, "")

	t.Run("valid is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/admin/users/"+u.ID.String(), `{"username":"grace2","email":"g2@example.com","role":"admin"}`)
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("missing fields is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/admin/users/"+u.ID.String(), `{"username":"g"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("unknown user is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/admin/users/"+uuid.New().String(), `{"username":"grace2","email":"g2@example.com","role":"admin"}`)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAdminUsersHandler_SetPassword(t *testing.T) {
	u := &models.User{Base: models.Base{ID: uuid.New()}, Username: "ivan"}
	r := adminUsersRouter(&fakeUserRepo{users: []*models.User{u}}, "")

	t.Run("valid is 204", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/admin/users/"+u.ID.String()+"/set-password", `{"password":"new-password"}`)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
	t.Run("short password is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/admin/users/"+u.ID.String()+"/set-password", `{"password":"x"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAdminUsersHandler_DeleteUser(t *testing.T) {
	u := &models.User{Base: models.Base{ID: uuid.New()}, Username: "judy"}

	t.Run("deletes another user (204)", func(t *testing.T) {
		users := &fakeUserRepo{users: []*models.User{u}}
		r := adminUsersRouter(users, "someone-else")
		w := doJSON(r, http.MethodDelete, "/admin/users/"+u.ID.String(), "")
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
	t.Run("cannot delete self (400)", func(t *testing.T) {
		users := &fakeUserRepo{users: []*models.User{u}}
		r := adminUsersRouter(users, u.ID.String())
		w := doJSON(r, http.MethodDelete, "/admin/users/"+u.ID.String(), "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAdminUsersHandler_GetActivity(t *testing.T) {
	r := adminUsersRouter(&fakeUserRepo{}, "")
	w := doJSON(r, http.MethodGet, "/admin/users/"+uuid.New().String()+"/activity", "")
	require.Equal(t, http.StatusOK, w.Code) // empty activity is still a 200 with []
}
