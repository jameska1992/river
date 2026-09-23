package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"river-api/internal/models"
	"river-api/internal/repository"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newWebhookRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Webhook{}, &models.WebhookDelivery{}))
	h := NewWebhookHandler(services.NewWebhookService(repository.NewWebhookRepository(db)))

	r := gin.New()
	r.GET("/admin/webhooks", h.List)
	r.POST("/admin/webhooks", h.Create)
	r.PUT("/admin/webhooks/:id", h.Update)
	r.DELETE("/admin/webhooks/:id", h.Delete)
	r.GET("/admin/webhooks/:id/deliveries", h.ListDeliveries)
	r.GET("/webhooks/active", h.ListActive)
	r.POST("/webhooks/deliveries", h.RecordDelivery)
	return r
}

func createWebhook(t *testing.T, r *gin.Engine, body string) map[string]any {
	t.Helper()
	w := doJSON(r, http.MethodPost, "/admin/webhooks", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func TestWebhookHandler_CreateListUpdateDelete(t *testing.T) {
	r := newWebhookRouter(t)

	out := createWebhook(t, r, `{"name":"hass","url":"https://h/1","events":["media.ready"]}`)
	assert.Contains(t, out["secret"], "whsec_")
	hook := out["webhook"].(map[string]any)
	id := hook["id"].(string)
	assert.Equal(t, true, hook["enabled"], "defaults enabled")

	// List.
	lw := doJSON(r, http.MethodGet, "/admin/webhooks", "")
	require.Equal(t, http.StatusOK, lw.Code)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(lw.Body.Bytes(), &list))
	require.Len(t, list, 1)
	_, hasSecret := list[0]["secret"]
	assert.False(t, hasSecret, "admin list never exposes the secret")

	// Update.
	uw := doJSON(r, http.MethodPut, "/admin/webhooks/"+id, `{"name":"renamed","url":"https://h/2","events":[],"enabled":false}`)
	require.Equal(t, http.StatusOK, uw.Code, uw.Body.String())
	var updated map[string]any
	require.NoError(t, json.Unmarshal(uw.Body.Bytes(), &updated))
	assert.Equal(t, "renamed", updated["name"])
	assert.Equal(t, false, updated["enabled"])

	// Delete → then 404 on a second delete.
	dw := doJSON(r, http.MethodDelete, "/admin/webhooks/"+id, "")
	assert.Equal(t, http.StatusNoContent, dw.Code)
	dw2 := doJSON(r, http.MethodDelete, "/admin/webhooks/"+id, "")
	assert.Equal(t, http.StatusNotFound, dw2.Code)
}

func TestWebhookHandler_CreateValidation(t *testing.T) {
	r := newWebhookRouter(t)
	// Missing required url → 400 (binding).
	w := doJSON(r, http.MethodPost, "/admin/webhooks", `{"name":"x"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	// Bad url → 400 (service validation).
	w = doJSON(r, http.MethodPost, "/admin/webhooks", `{"name":"x","url":"not-a-url"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookHandler_ActiveIncludesSecret(t *testing.T) {
	r := newWebhookRouter(t)
	createWebhook(t, r, `{"name":"on","url":"https://a","enabled":true}`)

	w := doJSON(r, http.MethodGet, "/webhooks/active", "")
	require.Equal(t, http.StatusOK, w.Code)
	var active []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &active))
	require.Len(t, active, 1)
	assert.Contains(t, active[0]["secret"], "whsec_", "service view carries the secret")
}

func TestWebhookHandler_RecordAndListDeliveries(t *testing.T) {
	r := newWebhookRouter(t)
	out := createWebhook(t, r, `{"name":"h","url":"https://a"}`)
	id := out["webhook"].(map[string]any)["id"].(string)

	rec := doJSON(r, http.MethodPost, "/webhooks/deliveries",
		`{"webhook_id":"`+id+`","event":"media.ready.movie","media_id":"m1","status":"delivered","attempts":1,"response_code":200}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	// Bad body (missing webhook_id) → 400.
	bad := doJSON(r, http.MethodPost, "/webhooks/deliveries", `{"status":"delivered"}`)
	assert.Equal(t, http.StatusBadRequest, bad.Code)

	lw := doJSON(r, http.MethodGet, "/admin/webhooks/"+id+"/deliveries", "")
	require.Equal(t, http.StatusOK, lw.Code)
	var deliveries []map[string]any
	require.NoError(t, json.Unmarshal(lw.Body.Bytes(), &deliveries))
	require.Len(t, deliveries, 1)
	assert.Equal(t, "media.ready.movie", deliveries[0]["event"])
}
