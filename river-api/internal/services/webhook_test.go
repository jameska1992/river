package services

import (
	"testing"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memWebhookRepo struct {
	hooks      []*models.Webhook
	deliveries []*models.WebhookDelivery
}

func (m *memWebhookRepo) Create(w *models.Webhook) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	m.hooks = append(m.hooks, w)
	return nil
}
func (m *memWebhookRepo) List() ([]models.Webhook, error) {
	out := make([]models.Webhook, len(m.hooks))
	for i, w := range m.hooks {
		out[i] = *w
	}
	return out, nil
}
func (m *memWebhookRepo) ListEnabled() ([]models.Webhook, error) {
	var out []models.Webhook
	for _, w := range m.hooks {
		if w.Enabled {
			out = append(out, *w)
		}
	}
	return out, nil
}
func (m *memWebhookRepo) FindByID(id string) (*models.Webhook, error) {
	for _, w := range m.hooks {
		if w.ID.String() == id {
			return w, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memWebhookRepo) Update(w *models.Webhook) error { return nil }
func (m *memWebhookRepo) Delete(id string) error {
	for i, w := range m.hooks {
		if w.ID.String() == id {
			m.hooks = append(m.hooks[:i], m.hooks[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (m *memWebhookRepo) RecordDelivery(d *models.WebhookDelivery, lastError string) error {
	m.deliveries = append(m.deliveries, d)
	return nil
}
func (m *memWebhookRepo) ListDeliveries(webhookID string, limit int) ([]models.WebhookDelivery, error) {
	var out []models.WebhookDelivery
	for _, d := range m.deliveries {
		if d.WebhookID.String() == webhookID {
			out = append(out, *d)
		}
	}
	return out, nil
}

func TestWebhook_CreateGeneratesSecretAndValidates(t *testing.T) {
	svc := NewWebhookService(&memWebhookRepo{})

	view, secret, err := svc.Create(WebhookInput{Name: "Discord", URL: "https://discord.com/api/webhooks/x", Events: []string{"media.ready"}, Enabled: true})
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, secret, "whsec_")
	assert.Equal(t, []string{"media.ready"}, view.Events)
	assert.True(t, view.Enabled)

	// Bad URL rejected.
	_, _, err = svc.Create(WebhookInput{Name: "x", URL: "not-a-url", Enabled: true})
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)

	// Unknown event kind rejected.
	_, _, err = svc.Create(WebhookInput{Name: "x", URL: "https://h", Events: []string{"media.bogus"}})
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)

	// Missing name rejected.
	_, _, err = svc.Create(WebhookInput{Name: "", URL: "https://h"})
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
}

func TestWebhook_ListActiveIncludesSecret(t *testing.T) {
	repo := &memWebhookRepo{}
	svc := NewWebhookService(repo)
	_, _, _ = svc.Create(WebhookInput{Name: "on", URL: "https://a", Enabled: true})
	_, _, _ = svc.Create(WebhookInput{Name: "off", URL: "https://b", Enabled: false})

	active, err := svc.ListActive()
	require.NoError(t, err)
	require.Len(t, active, 1, "only enabled webhooks are active")
	assert.Equal(t, "on", active[0].Name)
	assert.NotEmpty(t, active[0].Secret, "delivery view carries the HMAC secret")
}

func TestMatchesEvent(t *testing.T) {
	assert.True(t, MatchesEvent(nil, "media.ready"), "empty subscription = all kinds")
	assert.True(t, MatchesEvent([]string{"media.ready"}, "media.ready"))
	assert.False(t, MatchesEvent([]string{"media.ready"}, "media.transcoded"))
}

func TestWebhook_ListMasksSecret(t *testing.T) {
	svc := NewWebhookService(&memWebhookRepo{})
	_, _, err := svc.Create(WebhookInput{Name: "a", URL: "https://a", Enabled: true})
	require.NoError(t, err)

	list, err := svc.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "a", list[0].Name)
}

func TestWebhook_UpdateAndDelete(t *testing.T) {
	svc := NewWebhookService(&memWebhookRepo{})
	view, _, err := svc.Create(WebhookInput{Name: "orig", URL: "https://a", Events: []string{"media.ready"}, Enabled: true})
	require.NoError(t, err)

	updated, err := svc.Update(view.ID, WebhookInput{Name: "renamed", URL: "https://b", Events: []string{"media.transcoded"}, Enabled: false})
	require.NoError(t, err)
	assert.Equal(t, "renamed", updated.Name)
	assert.Equal(t, "https://b", updated.URL)
	assert.Equal(t, []string{"media.transcoded"}, updated.Events)
	assert.False(t, updated.Enabled)

	// Update validates too.
	_, err = svc.Update(view.ID, WebhookInput{Name: "x", URL: "bad-url"})
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)

	// Update of an unknown id surfaces not-found.
	_, err = svc.Update("00000000-0000-0000-0000-000000000000", WebhookInput{Name: "x", URL: "https://h"})
	assert.ErrorIs(t, err, apperrors.ErrNotFound)

	require.NoError(t, svc.Delete(view.ID))
	assert.ErrorIs(t, svc.Delete(view.ID), apperrors.ErrNotFound)
}

func TestWebhook_RecordAndListDeliveries(t *testing.T) {
	repo := &memWebhookRepo{}
	svc := NewWebhookService(repo)
	view, _, err := svc.Create(WebhookInput{Name: "h", URL: "https://a", Enabled: true})
	require.NoError(t, err)

	require.NoError(t, svc.RecordDelivery(RecordDeliveryInput{
		WebhookID: view.ID, Event: "media.ready.movie", MediaID: "m1", Status: "delivered", Attempts: 1, ResponseCode: 200,
	}))
	// A bad webhook id is rejected as invalid input.
	assert.ErrorIs(t, svc.RecordDelivery(RecordDeliveryInput{WebhookID: "not-a-uuid", Status: "failed"}), apperrors.ErrInvalidInput)

	list, err := svc.ListDeliveries(view.ID, 50)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "media.ready.movie", list[0].Event)
}
