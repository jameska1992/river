// Package events defines the versioned media-lifecycle event contract carried
// on the river.lifecycle topic exchange, plus a publisher for it.
//
// This payload is a semi-public API surface (external webhooks / bots consume
// it), so it is versioned: SchemaVersion is bumped on any breaking change and
// consumers must tolerate unknown fields.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// SchemaVersion is the current payload version. Bump on a breaking change.
const SchemaVersion = 1

// Exchange is the dedicated topic exchange for lifecycle events. Kept separate
// from river.media (the ingest/discovery bus) so the outbound contract can
// evolve and be bound to independently.
const Exchange = "river.lifecycle"

// Event kinds. Routing keys are "<kind>.<type>", e.g. "media.transcoded.movie".
const (
	KindTranscoded = "media.transcoded"
	KindEnriched   = "media.enriched"
	KindReady      = "media.ready"
)

// LifecycleEvent is one media-lifecycle signal. MediaID is the entity id at the
// event's natural granularity: the movie/episode/track/chapter that was
// transcoded, or the movie/show/album/audiobook that was enriched. ParentID is
// the *enrichable parent* of a transcoded unit — the show for an episode, the
// album for a track, the audiobook for a chapter (empty for a movie, which is
// its own parent). SeasonID adds episode context for TV. Consumers fetch the
// full record from river-api when they need more than this envelope.
type LifecycleEvent struct {
	SchemaVersion int       `json:"schema_version"`
	Kind          string    `json:"kind"` // KindTranscoded | KindEnriched | KindReady
	Type          string    `json:"type"` // movie | tvshow | music | audiobook
	LibraryID     string    `json:"library_id,omitempty"`
	MediaID       string    `json:"media_id"`
	ParentID      string    `json:"parent_id,omitempty"`
	SeasonID      string    `json:"season_id,omitempty"`
	Title         string    `json:"title,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// RoutingKey is "<kind>.<type>", the topic key this event publishes under.
func (e LifecycleEvent) RoutingKey() string {
	return e.Kind + "." + e.Type
}

// Publisher publishes lifecycle events to the river.lifecycle exchange.
type Publisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewPublisher dials RabbitMQ and declares the lifecycle exchange (idempotent).
func NewPublisher(url string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}
	if err := ch.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange %q: %w", Exchange, err)
	}
	return &Publisher{conn: conn, ch: ch}, nil
}

// Publish stamps the schema version + timestamp (when unset) and publishes the
// event under its routing key. Persistent so events survive a broker restart.
func (p *Publisher) Publish(ctx context.Context, e LifecycleEvent) error {
	if e.SchemaVersion == 0 {
		e.SchemaVersion = SchemaVersion
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return p.ch.PublishWithContext(ctx, Exchange, e.RoutingKey(), false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (p *Publisher) Close() {
	if p.ch != nil {
		p.ch.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}
