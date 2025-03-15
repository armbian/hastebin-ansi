package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func setupMongoContainer(t *testing.T) (string, int, func()) {
	ctx := context.Background()
	mongoContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mongo:latest",
			ExposedPorts: []string{"27017/tcp"},
			WaitingFor:   wait.ForListeningPort("27017/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)

	// Get container host and port
	endpoint, err := mongoContainer.Endpoint(ctx, "")
	require.NoError(t, err)

	host := strings.Split(endpoint, ":")[0]
	port, err := mongoContainer.MappedPort(ctx, "27017")
	require.NoError(t, err)

	// Connect to MongoDB
	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://" + endpoint))
	require.NoError(t, err)

	db := client.Database("testdb")

	cleanup := func() {
		require.NoError(t, db.Drop(ctx))
		require.NoError(t, client.Disconnect(ctx))
		require.NoError(t, mongoContainer.Terminate(ctx))
	}

	return host, port.Int(), cleanup
}

func TestMongoDBStorage(t *testing.T) {
	host, port, cleanup := setupMongoContainer(t)
	defer cleanup()

	const expiration = 2 // seconds
	store := NewMongoDBStorage(host, port, "", "", "testdb", expiration*time.Second)

	// Test Set
	err := store.Set("testKey", "testValue", false)
	require.NoError(t, err)

	// Test Get before expiration
	val, err := store.Get("testKey", false)
	require.NoError(t, err)
	require.Equal(t, "testValue", val)

	// Test expiration mechanism
	time.Sleep(time.Duration(expiration+1) * time.Second)
	val, err = store.Get("testKey", false)
	require.Equal(t, "", val)
	require.ErrorIs(t, ErrNotFound, err) // Should return error because the key should be expired

	// Test key not existing
	val, err = store.Get("testKey2", false)
	require.Equal(t, "", val)
	require.ErrorIs(t, mongo.ErrNoDocuments, err)
}

func TestMongoDBStorageSkipExpiration(t *testing.T) {
	host, port, cleanup := setupMongoContainer(t)
	defer cleanup()

	const expiration = 2 // seconds
	store := NewMongoDBStorage(host, port, "", "", "testdb", expiration*time.Second)

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
