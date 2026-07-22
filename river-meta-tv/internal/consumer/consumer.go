package consumer

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type MediaDiscoveredEvent struct {
	EventID       string    `json:"event_id"`
	LibraryID     string    `json:"library_id"`
	LibraryType   string    `json:"library_type"`
	DirectoryName string    `json:"directory_name"`
	DirectoryPath string    `json:"directory_path"`
	SeasonName    string    `json:"season_name"`
	SeasonPath    string    `json:"season_path"`
	MediaID       string    `json:"media_id,omitempty"`
	SeasonID      string    `json:"season_id,omitempty"`
	TMDBID        int       `json:"tmdb_id,omitempty"`
	IMDBID        string    `json:"imdb_id,omitempty"`
	Files         []string  `json:"files"`
	DiscoveredAt  time.Time `json:"discovered_at"`
}

type Consumer struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	exchange string
	queue    string
}

func New(url, exchange string) (*Consumer, error) {
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
		return nil, fmt.Errorf("declare exchange: %w", err)
	}
	q, err := ch.QueueDeclare("river.meta.tvshow", true, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}
	if err := ch.QueueBind(q.Name, "media.discovered.tvshow", exchange, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("bind queue: %w", err)
	}
	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set qos: %w", err)
	}
	return &Consumer{conn: conn, ch: ch, exchange: exchange, queue: q.Name}, nil
}

// Consume starts consuming messages and calls handler for each one.
// Messages are acked on success and nacked (no requeue) on error.
// Blocks until the delivery channel is closed.
func (c *Consumer) Consume(handler func(MediaDiscoveredEvent) error) error {
	deliveries, err := c.ch.Consume(c.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}
	for d := range deliveries {
		var event MediaDiscoveredEvent
		if err := json.Unmarshal(d.Body, &event); err != nil {
			log.Printf("ERROR unmarshal message: %v", err)
			d.Nack(false, false)
			continue
		}
		if err := handler(event); err != nil {
			log.Printf("ERROR processing event %s: %v", event.EventID, err)
			d.Nack(false, false)
			continue
		}
		d.Ack(false)
	}
	return nil
}

func (c *Consumer) Close() {
	c.ch.Close()
	c.conn.Close()
}
