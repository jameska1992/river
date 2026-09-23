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
	return nil, nil
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
