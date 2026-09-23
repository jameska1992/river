// Package webhooks implements durable, per-delivery outbound webhook dispatch.
//
// The lifecycle consumer fans a matching event out into one Delivery message per
// subscribed webhook on the river.webhooks work queue. A separate delivery
// consumer processes each Delivery independently — HMAC-signs and POSTs it, and
// on failure retries via the queue's own retry/DLQ. Because each (event ×
// webhook) is its own message, a failing endpoint never causes re-delivery to
// the healthy ones, and deliveries survive a restart (persistent messages).
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"river-events/internal/events"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	queueName        = "river.webhooks"
	retryCountHeader = "x-retry-count"
)

// Delivery is one webhook delivery task: the target + the event payload.
type Delivery struct {
	WebhookID string                `json:"webhook_id"`
	URL       string                `json:"url"`
	Secret    string                `json:"secret"`
	Event     events.LifecycleEvent `json:"event"`
}

// --- Publisher: fan-out onto the delivery queue ---

type Publisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewPublisher declares the delivery work queue + retry/DLQ and returns a
// publisher for fan-out.
func NewPublisher(url string) (*Publisher, error) {
	conn, ch, err := dialAndDeclare(url)
	if err != nil {
		return nil, err
	}
	return &Publisher{conn: conn, ch: ch}, nil
}

// Publish enqueues one delivery task (persistent).
func (p *Publisher) Publish(d Delivery) error {
	body, err := json.Marshal(d)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
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

// --- Consumer: process deliveries with retry/DLQ ---

// OutcomeReporter records a terminal delivery outcome (best-effort).
type OutcomeReporter func(d Delivery, status string, attempts, code int, errMsg string)

type Consumer struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
	http       *http.Client
	maxRetries int
	backoff    time.Duration
	report     OutcomeReporter
}

func NewConsumer(url string, maxRetries int, backoff time.Duration, report OutcomeReporter) (*Consumer, error) {
	conn, ch, err := dialAndDeclare(url)
	if err != nil {
		return nil, err
	}
	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set qos: %w", err)
	}
	return &Consumer{
		conn: conn, ch: ch, http: &http.Client{Timeout: 15 * time.Second},
		maxRetries: maxRetries, backoff: backoff, report: report,
	}, nil
}

// Consume delivers each task; on HTTP failure it retries with backoff up to
// maxRetries, then reports "failed" and drops to the DLQ. On success it reports
// "delivered". Blocks until the delivery channel closes.
func (c *Consumer) Consume() error {
	deliveries, err := c.ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}
	for msg := range deliveries {
		var d Delivery
		if err := json.Unmarshal(msg.Body, &d); err != nil {
			log.Printf("ERROR webhook delivery unmarshal, dead-lettering: %v", err)
			c.finish(msg, c.publishDLQ(msg.Body))
			continue
		}
		attempts := retryCount(msg.Headers) + 1
		code, err := c.deliver(d)
		if err == nil {
			c.reportOutcome(d, "delivered", attempts, code, "")
			msg.Ack(false)
			continue
		}
		if attempts >= c.maxRetries {
			log.Printf("ERROR webhook %s failed after %d attempts, dead-lettering: %v", d.WebhookID, attempts, err)
			c.reportOutcome(d, "failed", attempts, code, err.Error())
			c.finish(msg, c.publishDLQ(msg.Body))
			continue
		}
		log.Printf("WARN webhook %s delivery failed (attempt %d/%d), retrying: %v", d.WebhookID, attempts, c.maxRetries, err)
		c.finish(msg, c.publishRetry(msg.Body, attempts))
	}
	return nil
}

// deliver HMAC-signs the event payload and POSTs it. Returns the response code
// and an error for any non-2xx or transport failure.
func (c *Consumer) deliver(d Delivery) (int, error) {
	body, err := json.Marshal(d.Event)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, d.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-River-Event", d.Event.RoutingKey())
	req.Header.Set("X-River-Signature", "sha256="+sign(d.Secret, body))
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("non-2xx response: %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func (c *Consumer) reportOutcome(d Delivery, status string, attempts, code int, errMsg string) {
	if c.report != nil {
		c.report(d, status, attempts, code, errMsg)
	}
}

func (c *Consumer) finish(msg amqp.Delivery, publishErr error) {
	if publishErr != nil {
		log.Printf("ERROR webhook republish failed, requeuing: %v", publishErr)
		msg.Nack(false, true)
		return
	}
	msg.Ack(false)
}

func (c *Consumer) publishRetry(body []byte, attempt int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.ch.PublishWithContext(ctx, "", queueName+".retry", false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Expiration:   strconv.FormatInt(c.backoff.Milliseconds(), 10),
		Headers:      amqp.Table{retryCountHeader: int32(attempt)},
		Body:         body,
	})
}

func (c *Consumer) publishDLQ(body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.ch.PublishWithContext(ctx, "", queueName+".dlq", false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (c *Consumer) Close() {
	c.ch.Close()
	c.conn.Close()
}

// --- shared topology + helpers ---

// dialAndDeclare declares the work queue plus its .retry (TTL → work queue) and
// .dlq queues. Idempotent; safe for both publisher and consumer to call.
func dialAndDeclare(url string) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("open channel: %w", err)
	}
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("declare queue: %w", err)
	}
	if _, err := ch.QueueDeclare(queueName+".retry", true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": queueName,
	}); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("declare retry queue: %w", err)
	}
	if _, err := ch.QueueDeclare(queueName+".dlq", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("declare dlq: %w", err)
	}
	return conn, ch, nil
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
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
