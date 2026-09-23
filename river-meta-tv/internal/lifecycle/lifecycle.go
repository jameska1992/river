// Package lifecycle publishes media-lifecycle events to the river.lifecycle
// topic exchange (media.transcoded.* / media.enriched.*). river-events joins
// them into media.ready.*. The payload mirrors the versioned contract in the
// river-events service — keep the two in sync when the schema changes.
package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const SchemaVersion = 1

const Exchange = "river.lifecycle"

const (
	KindTranscoded = "media.transcoded"
	KindEnriched   = "media.enriched"
	KindReady      = "media.ready"
)

// Event is the versioned lifecycle payload. See river-events for full docs.
type Event struct {
	SchemaVersion int       `json:"schema_version"`
	Kind          string    `json:"kind"`
	Type          string    `json:"type"`
	LibraryID     string    `json:"library_id,omitempty"`
	MediaID       string    `json:"media_id"`
	ParentID      string    `json:"parent_id,omitempty"`
	SeasonID      string    `json:"season_id,omitempty"`
	Title         string    `json:"title,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type Publisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// New dials RabbitMQ and declares the lifecycle exchange (idempotent).
func New(url string) (*Publisher, error) {
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

// Publish emits one event under "<kind>.<type>". Stamps schema version + time.
func (p *Publisher) Publish(e Event) error {
	e.SchemaVersion = SchemaVersion
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.ch.PublishWithContext(ctx, Exchange, e.Kind+"."+e.Type, false, false, amqp.Publishing{
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
