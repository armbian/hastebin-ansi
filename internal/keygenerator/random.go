package keygenerator

import (
	"crypto/rand"
	"math/big"
	"strings"
)

type RandomKeyGenerator struct {
	keyspace string
}

func NewRandomKeyGenerator(keyspace string) *RandomKeyGenerator {
	if keyspace == "" {
		keyspace = "abcdefghijklmnopqrstuvwxyz"
	}
	return &RandomKeyGenerator{
		keyspace: keyspace,
	}
}

func (r *RandomKeyGenerator) Generate(length int) string {
	var out strings.Builder
	defer out.Reset()

	maxRandomIndex := big.NewInt(int64(len(r.keyspace)))

	for i := 0; i < length; i++ {
		index, err := rand.Int(rand.Reader, maxRandomIndex)
		if err != nil {
			continue
		}
		out.WriteByte(r.keyspace[index.Int64()])
	}

	return out.String()
}
