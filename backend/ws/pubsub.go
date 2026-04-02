package ws

import (
	"encoding/json"
	"log"
)

const broadcastChannel = "warmdesk:broadcast"

// PubSub is the interface for cross-instance message delivery.
// The memory implementation handles single-instance deployments;
// the Redis implementation enables horizontal scaling.
type PubSub interface {
	// IsLocal returns true when messages should be delivered directly
	// without going through a broker (single-instance mode).
	IsLocal() bool
	// Publish sends a payload to all subscribers of channel.
	Publish(channel string, payload []byte) error
	// Subscribe registers handler for messages on channel.
	// The returned function cancels the subscription.
	Subscribe(channel string, handler func([]byte)) func()
}

var globalPubSub PubSub = &memoryPubSub{}

// InitPubSub sets the global pub/sub backend. Call once at startup.
func InitPubSub(ps PubSub) {
	globalPubSub = ps
}

// broadcastEnvelope wraps a message for cross-instance delivery.
type broadcastEnvelope struct {
	ProjectID uint            `json:"project_id"`
	Data      json.RawMessage `json:"data"`
}

// StartPubSubListener subscribes to the shared broadcast channel and
// delivers incoming messages to local hubs. A no-op in memory mode.
func StartPubSubListener() {
	if globalPubSub.IsLocal() {
		return
	}
	globalPubSub.Subscribe(broadcastChannel, func(payload []byte) {
		var env broadcastEnvelope
		if err := json.Unmarshal(payload, &env); err != nil {
			log.Printf("ws: invalid broadcast envelope: %v", err)
			return
		}
		localBroadcastRaw(env.ProjectID, env.Data)
	})
}
