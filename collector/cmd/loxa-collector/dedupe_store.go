package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type dedupeStore interface {
	SeenBefore(ctx context.Context, value string, window time.Duration) (bool, error)
	Close() error
}

type redisDedupeStore struct {
	client *redis.Client
	prefix string
}

func newRedisDedupeStore(cfg collectorConfig) (dedupeStore, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.dedupeBackend))
	switch backend {
	case "", "memory":
		return nil, nil
	case "redis":
		// continue
	default:
		return nil, fmt.Errorf("unsupported dedupe.backend %q", cfg.dedupeBackend)
	}
	if strings.TrimSpace(cfg.dedupeRedisAddr) == "" {
		return nil, fmt.Errorf("dedupe.redis_addr must be configured when dedupe.backend=redis")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.dedupeRedisAddr,
		Password: cfg.dedupeRedisPassword,
		DB:       cfg.dedupeRedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect redis dedupe store: %w", err)
	}
	return &redisDedupeStore{
		client: client,
		prefix: cfg.dedupeRedisPrefix,
	}, nil
}

func (s *redisDedupeStore) SeenBefore(ctx context.Context, value string, window time.Duration) (bool, error) {
	key := value
	if strings.TrimSpace(s.prefix) != "" {
		key = s.prefix + value
	}
	ok, err := s.client.SetNX(ctx, key, "1", window).Result()
	if err != nil {
		return false, err
	}
	return !ok, nil
}

func (s *redisDedupeStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}
