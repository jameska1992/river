package models

import (
	"time"

	"github.com/google/uuid"
)

// Webhook is an admin-configured outbound HTTP callback fired on media-lifecycle
// events. Deliveries are HMAC-signed with Secret so receivers can verify
// authenticity. Events is a JSON-encoded []string of subscribed event kinds
// (e.g. ["media.ready"]); an empty list means "all kinds".
//
// Secret is a symmetric HMAC key: river-events needs the real value to sign
// deliveries, so it's stored as-is (like the TMDB key) and returned to the
// service via a service-scoped read — the admin API only reports whether it's
// set, never the value.
type Webhook struct {
	Base
	Name           string     `gorm:"not null" json:"name"`
	URL            string     `gorm:"not null" json:"url"`
	Secret         string     `gorm:"not null" json:"-"`
	Events         string     `gorm:"not null;default:'[]'" json:"events"`
	Enabled        bool       `gorm:"not null;default:true" json:"enabled"`
	LastDeliveryAt *time.Time `json:"last_delivery_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
}

// WebhookDelivery records the terminal outcome of one delivery attempt-chain
// (event × webhook), written by river-events for admin observability. The
// durable retry itself is handled by the RabbitMQ per-delivery queue; this is
// the human-facing log, not the retry mechanism.
type WebhookDelivery struct {
	Base
	WebhookID    uuid.UUID `gorm:"type:varchar(36);not null;index" json:"webhook_id"`
	Event        string    `json:"event"` // routing key, e.g. "media.ready.movie"
	MediaID      string    `json:"media_id"`
	Status       string    `json:"status"` // "delivered" | "failed"
	Attempts     int       `json:"attempts"`
	ResponseCode int       `json:"response_code,omitempty"`
	Error        string    `json:"error,omitempty"`
}
