package keygenerator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRandomKeyGenerator(t *testing.T) {
	kg := NewRandomKeyGenerator("abc")
	require.Equal(t, "abc", kg.keyspace)

	defaultKg := NewRandomKeyGenerator("")
	require.Equal(t, "abcdefghijklmnopqrstuvwxyz", defaultKg.keyspace)
}

func TestGenerate(t *testing.T) {
	kg := NewRandomKeyGenerator("abc")

	t.Run("Generate with valid length", func(t *testing.T) {
		key := kg.Generate(10)
		fmt.Print(key)
		require.Len(t, key, 10)
		for _, ch := range key {
			require.Contains(t, "abc", string(ch))
		}
	})

	t.Run("Generate with zero length", func(t *testing.T) {
		key := kg.Generate(0)
		require.Empty(t, key)
	})

	t.Run("Generate with large length", func(t *testing.T) {
		key := kg.Generate(1000)
		require.Len(t, key, 1000)
	})

	t.Run("Check different keys", func(t *testing.T) {
		var oldKey string

		for i := 0; i < 200; i++ {
			key := kg.Generate(50)
			require.NotEqual(t, key, oldKey)

			oldKey = key
		}
	})
}
