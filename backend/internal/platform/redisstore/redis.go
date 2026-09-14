package redisstore

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
}

func Open(ctx context.Context, address, password string, database int) (*Store, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       database,
	})
	store := &Store{client: client}
	if err := store.Ping(ctx); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return store, nil
}

func (s *Store) Name() string {
	return "redis"
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *Store) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *Store) Client() *redis.Client {
	return s.client
}
