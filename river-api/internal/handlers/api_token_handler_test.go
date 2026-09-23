package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"river-api/internal/middleware"
	"river-api/internal/models"
	"river-api/internal/repository"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAPITokenRouter(t *testing.T, callerID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.APIToken{}))
	h := NewAPITokenHandler(services.NewAPITokenService(repository.NewAPITokenRepository(db)))

	r := gin.New()
	// Mint reads the acting admin from the request claims.
	r.Use(func(c *gin.Context) { c.Set("claims", &middleware.Claims{UserID: callerID, Role: "admin"}) })
	r.GET("/admin/api-tokens", h.List)
	r.POST("/admin/api-tokens", h.Mint)
	r.DELETE("/admin/api-tokens/:id", h.Revoke)
	return r
}

func TestAPITokenHandler_MintListRevoke(t *testing.T) {
	r := newAPITokenRouter(t, uuid.NewString())

	// Mint.
	mw := doJSON(r, http.MethodPost, "/admin/api-tokens", `{"name":"grafana","expires_in_days":30}`)
	require.Equal(t, http.StatusCreated, mw.Code, mw.Body.String())
	var minted map[string]any
	require.NoError(t, json.Unmarshal(mw.Body.Bytes(), &minted))
	assert.Contains(t, minted["token"], "rvat_")
	tok := minted["api_token"].(map[string]any)
	id := tok["id"].(string)
	_, hasHash := tok["token_hash"]
	assert.False(t, hasHash, "view never exposes the hash")

	// List.
	lw := doJSON(r, http.MethodGet, "/admin/api-tokens", "")
	require.Equal(t, http.StatusOK, lw.Code)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(lw.Body.Bytes(), &list))
	require.Len(t, list, 1)

	// Revoke the token (idempotent — revoking again still 204 as the row exists).
	dw := doJSON(r, http.MethodDelete, "/admin/api-tokens/"+id, "")
	assert.Equal(t, http.StatusNoContent, dw.Code)

	// Revoking an unknown id is a 404.
	dw2 := doJSON(r, http.MethodDelete, "/admin/api-tokens/"+uuid.NewString(), "")
	assert.Equal(t, http.StatusNotFound, dw2.Code)
}

func TestAPITokenHandler_MintValidation(t *testing.T) {
	// Missing name → 400.
	r := newAPITokenRouter(t, uuid.NewString())
	w := doJSON(r, http.MethodPost, "/admin/api-tokens", `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPITokenHandler_MintRejectsBadOwner(t *testing.T) {
	// A non-uuid caller id can't own a token.
	r := newAPITokenRouter(t, "not-a-uuid")
	w := doJSON(r, http.MethodPost, "/admin/api-tokens", `{"name":"x"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
