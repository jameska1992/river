package services

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
)

// WebhookEventKinds are the lifecycle event kinds a webhook may subscribe to.
// An empty subscription list means "all kinds".
var WebhookEventKinds = []string{"media.transcoded", "media.enriched", "media.ready"}

type WebhookService struct {
	repo repository.WebhookRepository
}

func NewWebhookService(repo repository.WebhookRepository) *WebhookService {
	return &WebhookService{repo: repo}
}

// WebhookView is the admin-facing representation. The HMAC secret is never
// returned — only whether one is set (it always is) is implied.
type WebhookView struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	Events         []string   `json:"events"`
	Enabled        bool       `json:"enabled"`
	LastDeliveryAt *time.Time `json:"last_delivery_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func toWebhookView(w models.Webhook) WebhookView {
	return WebhookView{
		ID: w.ID.String(), Name: w.Name, URL: w.URL, Events: parseScopes(w.Events),
		Enabled: w.Enabled, LastDeliveryAt: w.LastDeliveryAt, LastError: w.LastError,
		CreatedAt: w.CreatedAt,
	}
}

// ActiveWebhook is the delivery-facing shape river-events fetches — it includes
// the HMAC secret so the service can sign deliveries.
type ActiveWebhook struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Secret string   `json:"secret"`
	Events []string `json:"events"`
}

// WebhookInput is the create/update shape.
type WebhookInput struct {
	Name    string
	URL     string
	Events  []string
	Enabled bool
}

// Create validates the input, generates an HMAC secret, and stores the webhook.
// The plaintext secret is returned once so the admin can note it for verifying
// signatures on the receiving end.
func (s *WebhookService) Create(in WebhookInput) (WebhookView, string, error) {
	if err := validateWebhookInput(in); err != nil {
		return WebhookView{}, "", err
	}
	events, err := validateEventKinds(in.Events)
	if err != nil {
		return WebhookView{}, "", err
	}
	secret, err := generateSecret()
	if err != nil {
		return WebhookView{}, "", err
	}
	w := &models.Webhook{
		Name: strings.TrimSpace(in.Name), URL: strings.TrimSpace(in.URL),
		Secret: secret, Events: encodeScopes(events), Enabled: in.Enabled,
	}
	if err := s.repo.Create(w); err != nil {
		return WebhookView{}, "", err
	}
	return toWebhookView(*w), secret, nil
}

func (s *WebhookService) List() ([]WebhookView, error) {
	hooks, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	out := make([]WebhookView, len(hooks))
	for i, w := range hooks {
		out[i] = toWebhookView(w)
	}
	return out, nil
}

// ListActive returns enabled webhooks with their secrets, for river-events.
func (s *WebhookService) ListActive() ([]ActiveWebhook, error) {
	hooks, err := s.repo.ListEnabled()
	if err != nil {
		return nil, err
	}
	out := make([]ActiveWebhook, len(hooks))
	for i, w := range hooks {
		out[i] = ActiveWebhook{ID: w.ID.String(), Name: w.Name, URL: w.URL, Secret: w.Secret, Events: parseScopes(w.Events)}
	}
	return out, nil
}

// Update replaces the mutable fields (not the secret).
func (s *WebhookService) Update(id string, in WebhookInput) (WebhookView, error) {
	w, err := s.repo.FindByID(id)
	if err != nil {
		return WebhookView{}, err
	}
	if err := validateWebhookInput(in); err != nil {
		return WebhookView{}, err
	}
	events, err := validateEventKinds(in.Events)
	if err != nil {
		return WebhookView{}, err
	}
	w.Name = strings.TrimSpace(in.Name)
	w.URL = strings.TrimSpace(in.URL)
	w.Events = encodeScopes(events)
	w.Enabled = in.Enabled
	if err := s.repo.Update(w); err != nil {
		return WebhookView{}, err
	}
	return toWebhookView(*w), nil
}

func (s *WebhookService) Delete(id string) error {
	return s.repo.Delete(id)
}

// RecordDeliveryInput is the outcome river-events reports after a delivery
// attempt-chain finishes (delivered or exhausted).
type RecordDeliveryInput struct {
	WebhookID    string
	Event        string
	MediaID      string
	Status       string // "delivered" | "failed"
	Attempts     int
	ResponseCode int
	Error        string
}

func (s *WebhookService) RecordDelivery(in RecordDeliveryInput) error {
	wid, err := uuid.Parse(in.WebhookID)
	if err != nil {
		return fmt.Errorf("%w: invalid webhook_id", apperrors.ErrInvalidInput)
	}
	d := &models.WebhookDelivery{
		WebhookID: wid, Event: in.Event, MediaID: in.MediaID, Status: in.Status,
		Attempts: in.Attempts, ResponseCode: in.ResponseCode, Error: in.Error,
	}
	return s.repo.RecordDelivery(d, in.Error)
}

func (s *WebhookService) ListDeliveries(webhookID string, limit int) ([]models.WebhookDelivery, error) {
	return s.repo.ListDeliveries(webhookID, limit)
}

func validateWebhookInput(in WebhookInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("%w: name is required", apperrors.ErrInvalidInput)
	}
	u := strings.TrimSpace(in.URL)
	parsed, err := url.Parse(u)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%w: url must be a valid http(s) URL", apperrors.ErrInvalidInput)
	}
	return nil
}

func validateEventKinds(events []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(events))
	for _, e := range events {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if !containsString(WebhookEventKinds, e) {
			return nil, fmt.Errorf("%w: unknown event kind %q", apperrors.ErrInvalidInput, e)
		}
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out, nil
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return "whsec_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// MatchesEvent reports whether a webhook subscribed to `events` should receive
// an event of the given kind. Empty subscription = all kinds.
func MatchesEvent(events []string, kind string) bool {
	if len(events) == 0 {
		return true
	}
	return containsString(events, kind)
}

// (encodeScopes/parseScopes/containsString are shared helpers in service_key.go)
