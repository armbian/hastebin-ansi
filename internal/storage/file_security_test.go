package storage

import (
	"os"
	"testing"

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
