package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupMemcachedContainer(t *testing.T) (string, int, func()) {
	ctx := context.Background()
	memcachedContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "memcached:latest",
			ExposedPorts: []string{"11211/tcp"},
			WaitingFor:   wait.ForListeningPort("11211/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)

	endpoint, err := memcachedContainer.Endpoint(ctx, "")
	require.NoError(t, err)

	host := strings.Split(endpoint, ":")[0]
	port, err := memcachedContainer.MappedPort(ctx, "11211")
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, memcachedContainer.Terminate(ctx))
	}

	return host, port.Int(), cleanup
}

func TestMemcachedStorage(t *testing.T) {
	host, port, cleanup := setupMemcachedContainer(t)
	t.Cleanup(cleanup)

	const expiration = 2 // seconds
	store := NewMemcachedStorage(host, port, expiration)

	// Test Set
	err := store.Set("testKey", "testValue", false)
	require.NoError(t, err)

	// Test Get before expiration
	val, err := store.Get("testKey", false)
	require.NoError(t, err)
	require.Equal(t, "testValue", val)

	// Test if expiration is updated after Get
	time.Sleep(1 * time.Second)
	_, err = store.Get("testKey", false)
	require.NoError(t, err) // Should still exist

	time.Sleep(1 * time.Second) // Should reset expiration
	_, err = store.Get("testKey", false)
	require.NoError(t, err) // Should still exist due to refresh

	_, err = store.Get("testKey2", false)
	require.ErrorIs(t, memcache.ErrCacheMiss, err) // Should not exist
}

func TestMemcachedStorageSkipExpiration(t *testing.T) {
	host, port, cleanup := setupMemcachedContainer(t)
	t.Cleanup(cleanup)

	const expiration = 2 // seconds
	store := NewMemcachedStorage(host, port, expiration)

	// Test Set
	err := store.Set("persistentKey", "persistentValue", true)
	require.NoError(t, err)

	// Test Get with skip_expiration
	val, err := store.Get("persistentKey", true)
	require.NoError(t, err)
	require.Equal(t, "persistentValue", val)

	// Wait past expiration but skip expiration should still work
	time.Sleep(time.Duration(expiration+1) * time.Second)
	val, err = store.Get("persistentKey", true)
	require.NoError(t, err)
	require.Equal(t, "persistentValue", val)
}
