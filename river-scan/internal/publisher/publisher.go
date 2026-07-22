package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type MediaDiscoveredEvent struct {
	EventID       string    `json:"event_id"`
	LibraryID     string    `json:"library_id"`
	LibraryType   string    `json:"library_type"`
	LibraryPath   string    `json:"library_path,omitempty"`
	DirectoryName string    `json:"directory_name"`
	DirectoryPath string    `json:"directory_path"`
	SeasonName    string    `json:"season_name,omitempty"`
	SeasonPath    string    `json:"season_path,omitempty"`
	MediaID       string    `json:"media_id,omitempty"`
	SeasonID      string    `json:"season_id,omitempty"`
	TMDBID        int       `json:"tmdb_id,omitempty"`
	IMDBID        string    `json:"imdb_id,omitempty"`
	Files         []string  `json:"files"`
	DiscoveredAt  time.Time `json:"discovered_at"`
	// PreTranscoded signals that the library owning this content is
	// already in the canonical stream format. Metadata consumers ignore
	// this and enrich the record normally; the video / audio transcoder
	// consumers treat it as a hard skip (the scanner has already
	// registered the media record with source == stream path).
	PreTranscoded bool `json:"pre_transcoded,omitempty"`
}

type Publisher struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	exchange string
}

func New(url, exchange string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange %q: %w", exchange, err)
	}
	return &Publisher{conn: conn, ch: ch, exchange: exchange}, nil
}

func (p *Publisher) Publish(ctx context.Context, event MediaDiscoveredEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	routingKey := "media.discovered." + event.LibraryType
	return p.ch.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (p *Publisher) Close() {
	p.ch.Close()
	p.conn.Close()
}
