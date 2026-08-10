package storage

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFileStorageSetReplacesPreviousCompression(t *testing.T) {
	dir, cleanup := setupTempDir(t)
	t.Cleanup(cleanup)

	plain := NewFileStorage(dir, "none", 0)
	require.NoError(t, plain.Set("key", "old", false))

	compressed := NewFileStorage(dir, "gzip", 0)
	require.NoError(t, compressed.Set("key", "new", false))

	got, err := compressed.Get("key", false)
	require.NoError(t, err)
	require.Equal(t, "new", got)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestFileStorageSetUsesOwnerOnlyPermissions(t *testing.T) {
	dir, cleanup := setupTempDir(t)
	t.Cleanup(cleanup)

	store := NewFileStorage(dir, "none", 0)
	require.NoError(t, store.Set("key", "value", false))

	info, err := os.Stat(dir + "/" + md5Hex("key"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestFileStorageDeleteAfter(t *testing.T) {
	dir, cleanup := setupTempDir(t)
	t.Cleanup(cleanup)

	store := NewFileStorage(dir, "none", 0).(*FileStorage)
	require.NoError(t, store.SetWithDeleteAfter("key", "value", time.Hour))
	require.NoError(t, os.WriteFile(dir+"/"+md5Hex("key")+".expires", []byte(strconv.FormatInt(time.Now().Add(-time.Second).Unix(), 10)), 0600))
	removed, err := store.CleanupExpired()
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	_, err = store.Get("key", false)
	require.ErrorIs(t, err, os.ErrNotExist)
}
