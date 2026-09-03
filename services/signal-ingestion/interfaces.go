package main

import (
    "context"

    "github.com/redis/go-redis/v9"
)

// SignalPublisher is used to publish normalized signals to a backend (e.g., Redis).
type SignalPublisher interface {
    Publish(ctx context.Context, channel string, payload []byte) error
}

// RedisPublisher implements SignalPublisher using redis.Client
type RedisPublisher struct{
    client *redis.Client
}

func NewRedisPublisher(c *redis.Client) *RedisPublisher {
    return &RedisPublisher{client: c}
}

func (r *RedisPublisher) Publish(ctx context.Context, channel string, payload []byte) error {
    return r.client.Publish(ctx, channel, payload).Err()
}
