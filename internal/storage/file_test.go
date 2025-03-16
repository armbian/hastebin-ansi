package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupTempDir(t *testing.T) (string, func()) {
	dir, err := os.MkdirTemp("", "filestorage_test")
	require.NoError(t, err)
	cleanup := func() { os.RemoveAll(dir) }
	return dir, cleanup
}

func TestFileStorage(t *testing.T) {
	dir, cleanup := setupTempDir(t)
	t.Cleanup(cleanup)

	const expiration = 2 // seconds
	store := NewFileStorage(dir, expiration)

	// Test Set
	err := store.Set("testKey", "testValue", false)
	require.NoError(t, err)

	// Test Get
	val, err := store.Get("testKey", false)
	require.NoError(t, err)
	require.Equal(t, "testValue", val)

	_, err = store.Get("testKey", false)
	require.NoError(t, err)

	_, err = store.Get("testKey2", false)
	require.True(t, os.IsNotExist(err))

	require.NoError(t, store.Close())
}

func TestFileStorageSkipExpiration(t *testing.T) {
	dir, cleanup := setupTempDir(t)
	t.Cleanup(cleanup)

	const expiration = 2 // seconds
	store := NewFileStorage(dir, expiration)

	// Test Set
	err := store.Set("persistentKey", "persistentValue", true)
	require.NoError(t, err)

	// Test Get with skip_expiration
	val, err := store.Get("persistentKey", true)
	require.NoError(t, err)
	require.Equal(t, "persistentValue", val)

	require.NoError(t, store.Close())
}
