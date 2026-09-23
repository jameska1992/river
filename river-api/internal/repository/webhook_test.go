package repository

import (
	"testing"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newWebhookRepo(t *testing.T) WebhookRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Webhook{}, &models.WebhookDelivery{}))
	return NewWebhookRepository(db)
}

func TestWebhookRepository_CRUD(t *testing.T) {
	repo := newWebhookRepo(t)

	w := &models.Webhook{Name: "hass", URL: "https://h/1", Secret: "s", Events: `["media.ready"]`, Enabled: true}
	require.NoError(t, repo.Create(w))
	require.NotEqual(t, uuid.Nil, w.ID)

	got, err := repo.FindByID(w.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "hass", got.Name)
	assert.Equal(t, "s", got.Secret)

	// Update.
	got.Name = "home-assistant"
	got.Enabled = false
	require.NoError(t, repo.Update(got))
	after, err := repo.FindByID(w.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "home-assistant", after.Name)
	assert.False(t, after.Enabled)

	// Delete.
	require.NoError(t, repo.Delete(w.ID.String()))
	_, err = repo.FindByID(w.ID.String())
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestWebhookRepository_ListAndListEnabled(t *testing.T) {
	repo := newWebhookRepo(t)
	require.NoError(t, repo.Create(&models.Webhook{Name: "b-hook", URL: "u", Secret: "s", Events: "[]", Enabled: true}))
	require.NoError(t, repo.Create(&models.Webhook{Name: "a-hook", URL: "u", Secret: "s", Events: "[]", Enabled: false}))

	all, err := repo.List()
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, "a-hook", all[0].Name, "ordered by name")

	enabled, err := repo.ListEnabled()
	require.NoError(t, err)
	require.Len(t, enabled, 1)
	assert.Equal(t, "b-hook", enabled[0].Name)
}

func TestWebhookRepository_FindDeleteUnknown(t *testing.T) {
	repo := newWebhookRepo(t)
	_, err := repo.FindByID(uuid.NewString())
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
	assert.ErrorIs(t, repo.Delete(uuid.NewString()), apperrors.ErrNotFound)
}

func TestWebhookRepository_RecordAndListDeliveries(t *testing.T) {
	repo := newWebhookRepo(t)
	w := &models.Webhook{Name: "hook", URL: "u", Secret: "s", Events: "[]", Enabled: true}
	require.NoError(t, repo.Create(w))

	d1 := &models.WebhookDelivery{WebhookID: w.ID, Event: "media.ready.movie", MediaID: "m1", Status: "delivered", Attempts: 1, ResponseCode: 200}
	require.NoError(t, repo.RecordDelivery(d1, ""))
	d2 := &models.WebhookDelivery{WebhookID: w.ID, Event: "media.ready.tvshow", MediaID: "e1", Status: "failed", Attempts: 3}
	require.NoError(t, repo.RecordDelivery(d2, "boom"))

	// The webhook's at-a-glance health fields are stamped from the last delivery.
	after, err := repo.FindByID(w.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "boom", after.LastError)
	require.NotNil(t, after.LastDeliveryAt)

	list, err := repo.ListDeliveries(w.ID.String(), 50)
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Limit is honoured.
	limited, err := repo.ListDeliveries(w.ID.String(), 1)
	require.NoError(t, err)
	assert.Len(t, limited, 1)
}
