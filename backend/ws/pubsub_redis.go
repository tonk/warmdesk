package ws

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// RedisPubSub enables horizontal scaling by routing broadcast messages
// through a Redis pub/sub channel so all server instances receive them.
type RedisPubSub struct {
	client *redis.Client
}

// NewRedisPubSub creates a RedisPubSub from a Redis URL (e.g. redis://localhost:6379/0).
func NewRedisPubSub(redisURL string) (*RedisPubSub, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	log.Printf("ws: Redis pub/sub connected to %s", opt.Addr)
	return &RedisPubSub{client: c}, nil
}

func (r *RedisPubSub) IsLocal() bool { return false }

func (r *RedisPubSub) Publish(channel string, payload []byte) error {
	return r.client.Publish(context.Background(), channel, payload).Err()
}

func (r *RedisPubSub) Subscribe(channel string, handler func([]byte)) func() {
	ctx, cancel := context.WithCancel(context.Background())
	sub := r.client.Subscribe(ctx, channel)
	go func() {
		defer sub.Close()
		ch := sub.Channel()
		for msg := range ch {
			handler([]byte(msg.Payload))
		}
	}()
	return cancel
}
