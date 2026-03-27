package ws

// memoryPubSub is a no-op implementation for single-instance deployments.
// BroadcastToProject delivers directly to the local hub instead of going
// through a broker.
type memoryPubSub struct{}

func (m *memoryPubSub) IsLocal() bool                                  { return true }
func (m *memoryPubSub) Publish(channel string, payload []byte) error   { return nil }
func (m *memoryPubSub) Subscribe(channel string, handler func([]byte)) func() {
	return func() {}
}
