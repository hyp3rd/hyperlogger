package output

import "testing"

func BenchmarkAsyncWriter(b *testing.B) {
	benchmarks := []struct {
		name     string
		strategy AsyncOverflowStrategy
	}{
		{name: "drop_newest", strategy: AsyncOverflowDropNewest},
		{name: "drop_oldest", strategy: AsyncOverflowDropOldest},
		{name: "block", strategy: AsyncOverflowBlock},
		{name: "handoff", strategy: AsyncOverflowHandoff},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			writer := newMockWriter()
			async := NewAsyncWriter(writer, AsyncConfig{
				BufferSize:       256,
				OverflowStrategy: bm.strategy,
			})
			defer async.Close()

			payload := []byte("benchmark message")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = async.Write(payload)
			}
		})
	}
}
