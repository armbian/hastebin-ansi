package storage

import (
	"strings"
	"testing"
	"time"
)

const benchmarkDocumentSize = 64 << 10

func BenchmarkFileStorageSet(b *testing.B) {
	payload := strings.Repeat("haste benchmark payload\n", benchmarkDocumentSize/len("haste benchmark payload\n"))

	for _, compression := range []string{"none", "gzip", "zstd"} {
		b.Run(compression, func(b *testing.B) {
			store := NewFileStorage(b.TempDir(), compression, 0)
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := store.Set("benchmark", payload, false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkFileStorageGet(b *testing.B) {
	payload := strings.Repeat("haste benchmark payload\n", benchmarkDocumentSize/len("haste benchmark payload\n"))

	for _, compression := range []string{"none", "gzip", "zstd"} {
		b.Run(compression, func(b *testing.B) {
			store := NewFileStorage(b.TempDir(), compression, time.Hour)
			if err := store.Set("benchmark", payload, false); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if _, err := store.Get("benchmark", false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
