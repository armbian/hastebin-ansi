package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestContainer(t *testing.T) (string, int, func()) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:latest",
		Env:          map[string]string{"POSTGRES_USER": "test", "POSTGRES_PASSWORD": "test", "POSTGRES_DB": "testdb"},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForListeningPort("5432/tcp"),
	}

	container, err := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(context.Background())
	require.NoError(t, err)

	port, err := container.MappedPort(context.Background(), "5432")
	require.NoError(t, err)

	return host, port.Int(), func() {
		require.NoError(t, container.Terminate(context.Background()))
	}
}

func TestPostgresStorage(t *testing.T) {
	host, port, cleanup := setupTestContainer(t)
	defer cleanup()

	store := NewPostgresStorage(host, port, "test", "test", "testdb", 2)

	err := store.Set("key1", "value1", false)
	require.NoError(t, err)

	val, err := store.Get("key1", false)
	require.NoError(t, err)
	require.Equal(t, "value1", val)

	time.Sleep(3 * time.Second)

	val, err = store.Get("key1", false)
	require.NoError(t, err)
	require.Empty(t, val)

	// Test with skip expiration
	err = store.Set("key1", "value1", false)
	require.NoError(t, err)

	val, err = store.Get("key1", true)
	require.NoError(t, err)
	require.Equal(t, "value1", val)

	time.Sleep(2 * time.Second)

	val, err = store.Get("key1", false)
	require.NoError(t, err)
	require.Empty(t, val)
}
