// Package consumer wires river-events to the river.lifecycle exchange: it
// consumes media.transcoded.* and media.enriched.* with the same retry/DLQ
// safety net the other services use.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"river-events/internal/events"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	retryCountHeader = "x-retry-count"
	dlqReasonHeader  = "x-death-reason"
	// queueName is the work queue; .retry and .dlq are derived from it.
	queueName = "river.events"
)

// DeadLetterReporter is called when a message is parked in the DLQ, so the
// caller can persist it (e.g. river-api failed-jobs). nil skips reporting.
type DeadLetterReporter func(mediaType, sourcePath, reason, routingKey string, attempts int, event []byte)

type Consumer struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
	queue      string
	retryQueue string
	dlq        string
	maxRetries int
	backoff    time.Duration
	reporter   DeadLetterReporter
}

// New connects, declares the lifecycle exchange, the work queue bound to the
// transcoded + enriched routing keys, and the retry/DLQ topology.
func New(url string, maxRetries int, backoff time.Duration, reporter DeadLetterReporter) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}
	if err := ch.ExchangeDeclare(events.Exchange, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}
	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}
	// Bind transcode + enrich (for the join) and ready (so ready events, which
	// the join publishes, are fanned out to subscribed webhooks).
	for _, key := range []string{events.KindTranscoded + ".#", events.KindEnriched + ".#", events.KindReady + ".#"} {
		if err := ch.QueueBind(q.Name, key, events.Exchange, false, nil); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("bind queue to %s: %w", key, err)
		}
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
		conn: conn, ch: ch, queue: q.Name,
		retryQueue: retryQueue, dlq: dlq, maxRetries: maxRetries, backoff: backoff,
		reporter: reporter,
	}, nil
}

// declareRetryTopology declares <queue>.retry (TTL → dead-letters back to the
// work queue) and <queue>.dlq, without touching the work queue declaration.
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

// Consume calls handler for each event; acks on success, retries with backoff
// on error up to maxRetries, then parks in the DLQ. Blocks until the delivery
// channel closes.
func (c *Consumer) Consume(handler func(events.LifecycleEvent) error) error {
	deliveries, err := c.ch.Consume(c.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}
	for d := range deliveries {
		var event events.LifecycleEvent
		if err := json.Unmarshal(d.Body, &event); err != nil {
			log.Printf("ERROR unmarshal message, dead-lettering: %v", err)
			reason := fmt.Sprintf("unmarshal: %v", err)
			c.report("", "", reason, d.RoutingKey, 0, d.Body)
			c.finish(d, c.publishDLQ(d.Body, reason))
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

func (c *Consumer) handleFailure(d amqp.Delivery, event events.LifecycleEvent, cause error) {
	attempts := retryCount(d.Headers)
	if shouldDeadLetter(attempts, c.maxRetries) {
		log.Printf("ERROR event %s/%s failed after %d attempt(s), dead-lettering: %v", event.Kind, event.MediaID, attempts+1, cause)
		c.report(event.Type, event.MediaID, cause.Error(), d.RoutingKey, attempts+1, d.Body)
		c.finish(d, c.publishDLQ(d.Body, cause.Error()))
		return
	}
	log.Printf("WARN event %s/%s failed (attempt %d/%d), retrying in %s: %v", event.Kind, event.MediaID, attempts+1, c.maxRetries, c.backoff, cause)
	c.finish(d, c.publishRetry(d.Body, attempts+1))
}

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
		Expiration:   strconv.FormatInt(c.backoff.Milliseconds(), 10),
		Headers:      amqp.Table{retryCountHeader: int32(attempt)},
		Body:         body,
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

func (c *Consumer) Close() {
	c.ch.Close()
	c.conn.Close()
}

func (c *Consumer) report(mediaType, sourcePath, reason, routingKey string, attempts int, event []byte) {
	if c.reporter != nil {
		c.reporter(mediaType, sourcePath, reason, routingKey, attempts, event)
	}
}

func shouldDeadLetter(attempts, maxRetries int) bool {
	return attempts+1 >= maxRetries
}

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
