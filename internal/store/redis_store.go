package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a distributed implementation of StateStore using Redis
type RedisStore struct {
	client *redis.Client
	ctx    context.Context
	ttl    time.Duration
}

// NewRedisStore creates a new RedisStore
func NewRedisStore(redisAddr string, password string, db int, ttl time.Duration) *RedisStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: password, // no password set
		DB:       db,       // use default DB
	})

	return &RedisStore{
		client: rdb,
		ctx:    context.Background(),
		ttl:    ttl,
	}
}

// Set stores the token and its original value in Redis
func (r *RedisStore) Set(key string, value string) {
	// Best effort set, errors ignored for MVP
	r.client.Set(r.ctx, key, value, r.ttl)
}

// Get retrieves the original value for a given token from Redis
func (r *RedisStore) Get(key string) (string, bool) {
	val, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil || err != nil {
		return "", false
	}
	return val, true
}
