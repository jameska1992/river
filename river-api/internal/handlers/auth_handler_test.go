package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func authRouter(users *fakeUserRepo, refresh *fakeRefreshRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := services.NewAuthService(users, refresh, "test-secret", 15*time.Minute, 7*24*time.Hour, 8*time.Hour)
	h := NewAuthHandler(svc)
	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/refresh", h.Refresh)
	return r
}

func TestAuthHandler_Register_ValidationErrors(t *testing.T) {
	r := authRouter(&fakeUserRepo{}, &fakeRefreshRepo{})

	cases := []struct {
		name string
		body string
	}{
		{"username too short", `{"username":"ab","email":"a@x.com","password":"password1"}`},
		{"invalid email", `{"username":"alice","email":"not-an-email","password":"password1"}`},
		{"password too short", `{"username":"alice","email":"a@x.com","password":"short"}`},
		{"missing fields", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(r, http.MethodPost, "/auth/register", tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestAuthHandler_Register_Success(t *testing.T) {
	users := &fakeUserRepo{}
	r := authRouter(users, &fakeRefreshRepo{})

	w := doJSON(r, http.MethodPost, "/auth/register", `{"username":"alice","email":"alice@x.com","password":"password1"}`)
	require.Equal(t, http.StatusCreated, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "alice", body["username"])
	assert.Equal(t, "admin", body["role"], "the first registered user is an admin")
	assert.NotContains(t, string(w.Body.Bytes()), "password", "password hash must not be serialized")
}

func TestAuthHandler_Register_Conflict(t *testing.T) {
	users := &fakeUserRepo{}
	r := authRouter(users, &fakeRefreshRepo{})

	first := `{"username":"alice","email":"alice@x.com","password":"password1"}`
	require.Equal(t, http.StatusCreated, doJSON(r, http.MethodPost, "/auth/register", first).Code)

	// Same username again → service returns ErrConflict → 409.
	w := doJSON(r, http.MethodPost, "/auth/register", first)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAuthHandler_Login(t *testing.T) {
	users := &fakeUserRepo{}
	r := authRouter(users, &fakeRefreshRepo{})
	require.Equal(t, http.StatusCreated,
		doJSON(r, http.MethodPost, "/auth/register", `{"username":"alice","email":"alice@x.com","password":"password1"}`).Code)

	t.Run("missing fields is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/auth/login", `{"username":"alice"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("wrong password is 401", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/auth/login", `{"username":"alice","password":"wrongpass"}`)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("valid credentials return tokens", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/auth/login", `{"username":"alice","password":"password1"}`)
		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.NotEmpty(t, body["access_token"])
		assert.NotEmpty(t, body["refresh_token"])
		assert.NotEmpty(t, body["stream_token"])
	})
}

func TestAuthHandler_Refresh(t *testing.T) {
	users := &fakeUserRepo{}
	refresh := &fakeRefreshRepo{}
	r := authRouter(users, refresh)
	require.Equal(t, http.StatusCreated,
		doJSON(r, http.MethodPost, "/auth/register", `{"username":"alice","email":"alice@x.com","password":"password1"}`).Code)
	login := doJSON(r, http.MethodPost, "/auth/login", `{"username":"alice","password":"password1"}`)
	var lb map[string]any
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &lb))
	token, _ := lb["refresh_token"].(string)

	t.Run("valid refresh rotates", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/auth/refresh", `{"refresh_token":"`+token+`"}`)
		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.NotEmpty(t, body["refresh_token"])
	})

	t.Run("invalid refresh is 401", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/auth/refresh", `{"refresh_token":"bogus"}`)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
