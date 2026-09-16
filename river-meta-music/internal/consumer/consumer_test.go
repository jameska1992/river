package consumer

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestShouldDeadLetter(t *testing.T) {
	// maxRetries = 3 → attempts 0 and 1 retry; attempt 2 (the 3rd try) parks.
	cases := []struct {
		attempts, max int
		want          bool
	}{
		{0, 3, false}, // first failure → retry
		{1, 3, false}, // second failure → retry
		{2, 3, true},  // third failure → dead-letter
		{0, 1, true},  // max=1 means no retries at all
		{5, 3, true},  // already past the limit
	}
	for _, c := range cases {
		if got := shouldDeadLetter(c.attempts, c.max); got != c.want {
			t.Errorf("shouldDeadLetter(%d, %d) = %v, want %v", c.attempts, c.max, got, c.want)
		}
	}
}

func TestRetryCount(t *testing.T) {
	if got := retryCount(nil); got != 0 {
		t.Errorf("nil headers => %d, want 0", got)
	}
	if got := retryCount(amqp.Table{}); got != 0 {
		t.Errorf("missing header => %d, want 0", got)
	}
	// AMQP may decode an integer header as int32, int64, or int.
	for _, v := range []any{int32(2), int64(2), int(2)} {
		if got := retryCount(amqp.Table{retryCountHeader: v}); got != 2 {
			t.Errorf("header %T(%v) => %d, want 2", v, v, got)
		}
	}
	// A non-integer header is treated as no count rather than panicking.
	if got := retryCount(amqp.Table{retryCountHeader: "nope"}); got != 0 {
		t.Errorf("string header => %d, want 0", got)
	}
}
