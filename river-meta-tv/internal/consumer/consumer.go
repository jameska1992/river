package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
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

const retryCountHeader = "x-retry-count"
const dlqReasonHeader = "x-death-reason"

type Consumer struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
	exchange   string
	queue      string
	retryQueue string
	dlq        string
	maxRetries int
	backoff    time.Duration
}

func New(url, exchange string, maxRetries int, backoff time.Duration) (*Consumer, error) {
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
	retryQueue, dlq, err := declareRetryTopology(ch, q.Name)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set qos: %w", err)
	}
	return &Consumer{
		conn: conn, ch: ch, exchange: exchange, queue: q.Name,
		retryQueue: retryQueue, dlq: dlq, maxRetries: maxRetries, backoff: backoff,
	}, nil
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
			log.Printf("ERROR unmarshal message, dead-lettering: %v", err)
			c.finish(d, c.publishDLQ(d.Body, fmt.Sprintf("unmarshal: %v", err)))
			continue
		}
		if err := handler(event); err != nil {
			c.handleFailure(d, event, err)
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

// declareRetryTopology declares two NEW queues alongside the work queue —
// deliberately without touching the work queue's own declaration, so it
// upgrades cleanly on installs where the work queue already exists:
//
//   - <queue>.retry : no consumer; messages sit here for their per-message
//     TTL, then dead-letter back to the work queue (default exchange, routing
//     key = work queue name). This is the delayed-retry mechanism.
//   - <queue>.dlq   : where messages land after exhausting their retries.
func declareRetryTopology(ch *amqp.Channel, queue string) (retryQueue, dlq string, err error) {
	retryQueue = queue + ".retry"
	dlq = queue + ".dlq"
	if _, err = ch.QueueDeclare(retryQueue, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": queue,
	}); err != nil {
		return "", "", fmt.Errorf("declare retry queue: %w", err)
	}
	if _, err = ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return "", "", fmt.Errorf("declare dlq: %w", err)
	}
	return retryQueue, dlq, nil
}

// handleFailure decides whether to retry the delivery or dead-letter it, then
// republishes accordingly and acks the original. If the republish itself
// fails, the original is nacked-with-requeue so the message is not lost.
func (c *Consumer) handleFailure(d amqp.Delivery, event MediaDiscoveredEvent, cause error) {
	attempts := retryCount(d.Headers)
	if shouldDeadLetter(attempts, c.maxRetries) {
		log.Printf("ERROR event %s failed after %d attempt(s), dead-lettering: %v", event.EventID, attempts+1, cause)
		c.finish(d, c.publishDLQ(d.Body, cause.Error()))
		return
	}
	log.Printf("WARN event %s failed (attempt %d/%d), retrying in %s: %v", event.EventID, attempts+1, c.maxRetries, c.backoff, cause)
	c.finish(d, c.publishRetry(d.Body, attempts+1))
}

// finish acks the delivery when the republish succeeded, or nacks it back
// onto the work queue when it failed (so a broker hiccup during republish
// doesn't drop the message).
func (c *Consumer) finish(d amqp.Delivery, publishErr error) {
	if publishErr != nil {
		log.Printf("ERROR republish failed, requeuing original: %v", publishErr)
		d.Nack(false, true)
		return
	}
	d.Ack(false)
}

func (c *Consumer) publishRetry(body []byte, attempt int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.ch.PublishWithContext(ctx, "", c.retryQueue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		// Per-message TTL: the message waits this long in the retry queue then
		// dead-letters back to the work queue. All retries share the same value,
		// so there's no head-of-line blocking.
		Expiration: strconv.FormatInt(c.backoff.Milliseconds(), 10),
		Headers:    amqp.Table{retryCountHeader: int32(attempt)},
		Body:       body,
	})
}

func (c *Consumer) publishDLQ(body []byte, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.ch.PublishWithContext(ctx, "", c.dlq, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Headers:      amqp.Table{dlqReasonHeader: reason},
		Body:         body,
	})
}

// shouldDeadLetter reports whether a delivery that has already been attempted
// `attempts` times should be dead-lettered rather than retried again.
func shouldDeadLetter(attempts, maxRetries int) bool {
	return attempts+1 >= maxRetries
}

// retryCount reads the retry counter we stamp on republished messages,
// tolerating the various integer types AMQP may decode a header into.
func retryCount(h amqp.Table) int {
	v, ok := h[retryCountHeader]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int32:
		return int(n)
	case int64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
