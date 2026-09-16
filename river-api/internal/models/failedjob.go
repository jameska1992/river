package models

// FailedJob records an ingest event that a consumer gave up on after
// exhausting its retries (i.e. it was dead-lettered). It's the operator-facing
// counterpart to the RabbitMQ DLQ: reported by the services over HTTP when
// they park a message, listed/retried/dismissed by admins. Event holds the
// original MediaDiscoveredEvent JSON so a retry can re-publish it verbatim.
type FailedJob struct {
	Base
	Service    string `gorm:"not null;index" json:"service"`
	MediaType  string `gorm:"index" json:"media_type"`
	SourcePath string `json:"source_path"`
	Reason     string `json:"reason"`
	Attempts   int    `json:"attempts"`
	RoutingKey string `json:"routing_key"`
	// Event is the raw MediaDiscoveredEvent JSON, replayed on retry. Not
	// exposed in API responses — the display fields above cover the UI.
	Event string `gorm:"type:text" json:"-"`
}
