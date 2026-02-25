package output

import (
	"testing"
	"time"
)

const benchmarkWriteDelay = 200 * time.Microsecond

func BenchmarkAsyncWriter(b *testing.B) {
	benchmarks := []struct {
		name       string
		strategy   AsyncOverflowStrategy
		writeDelay time.Duration
	}{
		{name: "drop_newest", strategy: AsyncOverflowDropNewest, writeDelay: benchmarkWriteDelay},
		{name: "drop_oldest", strategy: AsyncOverflowDropOldest, writeDelay: benchmarkWriteDelay},
		{name: "block", strategy: AsyncOverflowBlock, writeDelay: benchmarkWriteDelay},
		{name: "handoff", strategy: AsyncOverflowHandoff, writeDelay: benchmarkWriteDelay},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			writer := newMockWriter()
			writer.writeDelay = bm.writeDelay

			async := NewAsyncWriter(writer, AsyncConfig{
				BufferSize:       64,
				OverflowStrategy: bm.strategy,
			})

			defer func() {
				err := async.Close()
				if err != nil {
					b.Fatalf("Close failed: %v", err)
				}
			}()

			payload := []byte("benchmark message")

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_, err := async.Write(payload)
				if err != nil {
					b.Fatalf("Write failed: %v", err)
				}
			}
		})
	}
}
