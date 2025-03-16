package storage

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupRedisContainer(t *testing.T) (string, int, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	return host, port.Int(), func() {
		require.NoError(t, container.Terminate(ctx))
	}
}

func TestRedisStorage_SetAndGet(t *testing.T) {
	host, port, close := setupRedisContainer(t)
	defer close()

	expiration := time.Second * 2
	storage := NewRedisStorage(host, port, "", "", expiration)

	key := "testKey"
	value := "testValue"

	err := storage.Set(key, value, false)
	require.NoError(t, err)

	ttl, err := storage.client.TTL(context.Background(), key).Result()
	require.NoError(t, err)

	require.True(t, ttl > 0 && ttl <= time.Duration(expiration))

	time.Sleep(time.Second)

	ttl, err = storage.client.TTL(context.Background(), key).Result()
	require.NoError(t, err)

	require.True(t, ttl > 0 && ttl <= time.Duration(expiration))

	got, err := storage.Get(key, false)
	require.NoError(t, err)
	require.Equal(t, value, got)

	ttlAfterGet, err := storage.client.TTL(context.Background(), key).Result() // TTL should be reset after GET operation
	require.NoError(t, err)
	require.True(t, ttlAfterGet > time.Second)

	keyNoExpire := "testKeyNoExpire"
	err = storage.Set(keyNoExpire, value, true)
	require.NoError(t, err)

	ttlNoExpire, err := storage.client.TTL(context.Background(), keyNoExpire).Result()
	require.NoError(t, err)
	require.Equal(t, -1*time.Nanosecond, ttlNoExpire)

	got, err = storage.Get(keyNoExpire, true)
	require.NoError(t, err)
	require.Equal(t, value, got)

	ttlAfterGetNoExpire, err := storage.client.TTL(context.Background(), keyNoExpire).Result()
	require.NoError(t, err)
	require.Equal(t, -1*time.Nanosecond, ttlAfterGetNoExpire) // -1ns means no expiration

	require.NoError(t, storage.Close())
}

func TestRedisStorage_GetNonExistentKey(t *testing.T) {
	host, port, close := setupRedisContainer(t)
	defer close()

	expiration := time.Second * 2
	storage := NewRedisStorage(host, port, "", "", expiration)

	_, err := storage.Get("nonExistentKey", false)
	require.Error(t, err)
	require.Equal(t, redis.Nil, err)

	require.NoError(t, storage.Close())
}
