package output

import (
	"testing"
	"time"
)

func BenchmarkAsyncWriter(b *testing.B) {
	benchmarks := []struct {
		name       string
		strategy   AsyncOverflowStrategy
		writeDelay time.Duration
	}{
		{name: "drop_newest", strategy: AsyncOverflowDropNewest, writeDelay: 200 * time.Microsecond},
		{name: "drop_oldest", strategy: AsyncOverflowDropOldest, writeDelay: 200 * time.Microsecond},
		{name: "block", strategy: AsyncOverflowBlock, writeDelay: 200 * time.Microsecond},
		{name: "handoff", strategy: AsyncOverflowHandoff, writeDelay: 200 * time.Microsecond},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			writer := newMockWriter()
			writer.writeDelay = bm.writeDelay

			async := NewAsyncWriter(writer, AsyncConfig{
				BufferSize:       64,
				OverflowStrategy: bm.strategy,
			})
			defer async.Close()

			payload := []byte("benchmark message")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = async.Write(payload)
			}
		})
	}
}
