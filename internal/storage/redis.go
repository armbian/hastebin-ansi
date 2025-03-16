package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type RedisStorage struct {
	client     *redis.Client
	expiration time.Duration
}

func NewRedisStorage(host string, port int, username string, password string, expiration time.Duration) *RedisStorage {
	client := redis.NewClient(&redis.Options{
		Addr:     host + ":" + strconv.Itoa(port),
		Username: username,
		Password: password,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if connection is established with 5s timeout
	if status := client.Ping(ctx); status.Err() != nil {
		logrus.Fatalf("Failed to connect to Redis: %v", status.Err())
	}

	return &RedisStorage{client: client, expiration: expiration}
}

var _ Storage = (*RedisStorage)(nil)

func (s *RedisStorage) Set(key string, value string, skip_expiration bool) error {
	ctx := context.Background() // TODO: Add timeout control

	expiry := s.expiration
	if skip_expiration {
		expiry = 0
	}

	return s.client.Set(ctx, key, value, expiry).Err()
}

func (s *RedisStorage) Get(key string, skip_expiration bool) (string, error) {
	ctx := context.Background() // TODO: Add timeout control

	res, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	// Update expiration
	if !skip_expiration {
		s.client.Expire(ctx, key, s.expiration)
	}

	return res, nil
}

func (s *RedisStorage) Close() error {
	return s.client.Close()
}
