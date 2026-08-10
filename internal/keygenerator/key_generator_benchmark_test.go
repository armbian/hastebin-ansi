package keygenerator

import "testing"

func BenchmarkKeyGeneration(b *testing.B) {
	benchmarks := []struct {
		name      string
		generator KeyGenerator
	}{
		{name: "random", generator: NewRandomKeyGenerator("")},
		{name: "phonetic", generator: NewPhoneticKeyGenerator()},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if got := benchmark.generator.Generate(10); len(got) != 10 {
					b.Fatalf("generated key length = %d, want 10", len(got))
				}
			}
		})
	}
}
