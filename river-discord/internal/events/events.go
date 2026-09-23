// Package events mirrors the versioned media-lifecycle event contract that
// river-events publishes and delivers over outbound webhooks (the POST body is
// exactly this envelope). river-discord is its own Go module, so the contract
// is duplicated here rather than imported — it must stay in lockstep with
// river-events/internal/events. Unknown fields are tolerated per the contract.
package events

import "time"

// Event kinds. Outbound webhooks fire on these lifecycle kinds; the routing key
// on the wire is "<kind>.<type>" (e.g. "media.ready.movie").
const (
	KindTranscoded = "media.transcoded"
	KindEnriched   = "media.enriched"
	KindReady      = "media.ready"
)

// LifecycleEvent is the webhook payload. MediaID is the entity id at the event's
// natural granularity; ParentID is the enrichable parent of a transcoded unit
// (show for an episode, album for a track, audiobook for a chapter). Consumers
// fetch the full record from river-api when they need more than this envelope.
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

// RoutingKey is "<kind>.<type>", matching the X-River-Event header.
func (e LifecycleEvent) RoutingKey() string {
	return e.Kind + "." + e.Type
}

// IsLifecycleKind reports whether kind is one of the known lifecycle kinds the
// notifier acts on. Unknown kinds are ignored (forward-compatible).
func IsLifecycleKind(kind string) bool {
	switch kind {
	case KindTranscoded, KindEnriched, KindReady:
		return true
	default:
		return false
	}
}
