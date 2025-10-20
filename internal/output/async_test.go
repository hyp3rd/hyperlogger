package output

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyp3rd/ewrap"
	"github.com/stretchr/testify/require"

	"github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/internal/constants"
)

// mockWriter implements io.Writer with controllable behavior for testing.
type mockWriter struct {
	mu                    sync.Mutex
	writtenData           [][]byte
	writeError            error
	transientError        error
	failuresBeforeSuccess int
	writeDelay            time.Duration
}

func newMockWriter() *mockWriter {
	return &mockWriter{
		writtenData: make([][]byte, 0),
	}
}

func (m *mockWriter) Write(p []byte) (int, error) {
	m.mu.Lock()
	delay := m.writeDelay
	persistentErr := m.writeError
	transientErr := m.transientError

	failures := m.failuresBeforeSuccess
	if failures > 0 {
		m.failuresBeforeSuccess--
	}

	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	if failures > 0 {
		err := transientErr
		if err == nil {
			err = ewrap.New("transient error")
		}

		return 0, err
	}

	if persistentErr != nil {
		return 0, persistentErr
	}

	m.mu.Lock()

	buf := make([]byte, len(p))
	copy(buf, p)
	m.writtenData = append(m.writtenData, buf)
	m.mu.Unlock()

	return len(p), nil
}

func (m *mockWriter) getWrittenData() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.writtenData
}

func TestNewAsyncWriter(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		writer := newMockWriter()

		async := NewAsyncWriter(writer, AsyncConfig{})
		defer async.Close()

		if async.config.BufferSize != 1024 {
			t.Errorf("Expected default buffer size 1024, got %d", async.config.BufferSize)
		}

		if async.config.WaitTimeout != constants.DefaultTimeout {
			t.Errorf("Expected default timeout %v, got %v", constants.DefaultTimeout, async.config.WaitTimeout)
		}

		if async.config.ErrorHandler == nil {
			t.Error("Expected non-nil error handler")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		writer := newMockWriter()
		errCalled := false

		async := NewAsyncWriter(writer, AsyncConfig{
			BufferSize:   100,
			WaitTimeout:  2 * time.Second,
			ErrorHandler: func(err error) { errCalled = true },
		})
		defer async.Close()

		if async.config.BufferSize != 100 {
			t.Errorf("Expected buffer size 100, got %d", async.config.BufferSize)
		}

		if async.config.WaitTimeout != 2*time.Second {
			t.Errorf("Expected timeout 2s, got %v", async.config.WaitTimeout)
		}

		async.config.ErrorHandler(ewrap.New("test"))

		if !errCalled {
			t.Error("Error handler was not called")
		}
	})

	t.Run("handoff strategy", func(t *testing.T) {
		writer := newMockWriter()
		writer.writeDelay = 200 * time.Millisecond

		async := NewAsyncWriter(writer, AsyncConfig{
			BufferSize:       1,
			OverflowStrategy: AsyncOverflowHandoff,
		})
		defer async.Close()

		if _, err := async.Write([]byte("first")); err != nil {
			t.Fatalf("first write failed: %v", err)
		}

		if _, err := async.Write([]byte("second")); err != nil {
			t.Fatalf("handoff write failed: %v", err)
		}

		time.Sleep(250 * time.Millisecond)

		_ = async.Flush()

		data := writer.getWrittenData()
		if len(data) < 2 {
			t.Fatalf("expected at least two writes, got %d", len(data))
		}
	})

	t.Run("write critical bypasses queue", func(t *testing.T) {
		writer := newMockWriter()
		writer.writeDelay = 200 * time.Millisecond

		async := NewAsyncWriter(writer, AsyncConfig{BufferSize: 1})
		defer async.Close()

		if _, err := async.Write([]byte("background")); err != nil {
			t.Fatalf("initial write failed: %v", err)
		}

		if _, err := async.WriteCritical([]byte("critical")); err != nil {
			t.Fatalf("critical write failed: %v", err)
		}

		time.Sleep(250 * time.Millisecond)

		_ = async.Flush()

		written := writer.getWrittenData()
		if len(written) != 2 {
			t.Fatalf("expected two messages written, got %d", len(written))
		}

		metrics := async.Metrics()
		if metrics.Bypassed < 1 {
			t.Fatalf("expected bypassed count to increase")
		}
	})
}

func TestAsyncWriter_Write(t *testing.T) {
	t.Run("successful write", func(t *testing.T) {
		writer := newMockWriter()

		async := NewAsyncWriter(writer, AsyncConfig{})
		defer async.Close()

		data := []byte("test message")

		n, err := async.Write(data)
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}

		if n != len(data) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
		}

		// Wait for async processing
		time.Sleep(50 * time.Millisecond)

		err = async.Flush()
		if err != nil {
			t.Fatalf("Flush error: %v", err)
		}

		written := writer.getWrittenData()
		if len(written) != 1 {
			t.Fatalf("Expected 1 write, got %d", len(written))
		}

		if string(written[0]) != "test message" {
			t.Errorf("Expected 'test message', got '%s'", written[0])
		}
	})

	t.Run("write to closed writer", func(t *testing.T) {
		writer := newMockWriter()
		async := NewAsyncWriter(writer, AsyncConfig{})

		err := async.Close()
		if err != nil {
			t.Fatalf("Close error: %v", err)
		}

		_, err = async.Write([]byte("test"))
		if !errors.Is(err, ErrWriterClosed) {
			t.Errorf("Expected ErrWriterClosed, got %v", err)
		}
	})

	t.Run("buffer full", func(t *testing.T) {
		writer := newMockWriter()
		writer.writeDelay = 100 * time.Millisecond

		errCalled := false
		dropCalled := false

		async := NewAsyncWriter(writer, AsyncConfig{
			BufferSize: 1,
			ErrorHandler: func(err error) {
				if errors.Is(err, ErrBufferFull) {
					errCalled = true
				}
			},
			DropHandler: func([]byte) {
				dropCalled = true
			},
		})
		defer async.Close()

		// First write should succeed
		_, err := async.Write([]byte("first"))
		if err != nil {
			t.Fatalf("First write failed: %v", err)
		}

		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			_, err = async.Write([]byte("second"))
			if errors.Is(err, ErrBufferFull) {
				break
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			time.Sleep(10 * time.Millisecond)
		}

		if !errors.Is(err, ErrBufferFull) {
			t.Fatalf("Expected ErrBufferFull, got %v", err)
		}

		if !errCalled {
			t.Error("Error handler not called for buffer full")
		}

		if !dropCalled {
			t.Error("Drop handler not invoked for overflow")
		}
	})

	t.Run("drop oldest strategy", func(t *testing.T) {
		writer := newMockWriter()
		writer.writeDelay = 100 * time.Millisecond

		var dropped [][]byte

		async := NewAsyncWriter(writer, AsyncConfig{
			BufferSize:       1,
			OverflowStrategy: AsyncOverflowDropOldest,
			DropHandler: func(payload []byte) {
				buf := make([]byte, len(payload))
				copy(buf, payload)
				dropped = append(dropped, buf)
			},
		})
		defer async.Close()

		if _, err := async.Write([]byte("first")); err != nil {
			t.Fatalf("first write failed: %v", err)
		}

		deadline := time.Now().Add(500 * time.Millisecond)
		for len(dropped) == 0 && time.Now().Before(deadline) {
			if _, err := async.Write([]byte("second")); err != nil {
				t.Fatalf("second write should succeed with drop oldest, got %v", err)
			}

			time.Sleep(10 * time.Millisecond)
		}

		if len(dropped) == 0 {
			t.Fatalf("expected dropped payloads, got none")
		}

		writer.mu.Lock()
		writer.writeDelay = 0
		writer.mu.Unlock()

		_ = async.Flush()

		written := writer.getWrittenData()
		if len(written) == 0 || string(written[len(written)-1]) != "second" {
			t.Fatalf("expected latest message to be written, got %v", written)
		}
	})

	t.Run("block strategy", func(t *testing.T) {
		writer := newMockWriter()
		writer.writeDelay = 50 * time.Millisecond

		async := NewAsyncWriter(writer, AsyncConfig{
			BufferSize:       1,
			OverflowStrategy: AsyncOverflowBlock,
		})
		defer async.Close()

		if _, err := async.Write([]byte("first")); err != nil {
			t.Fatalf("first write failed: %v", err)
		}

		done := make(chan error, 1)

		go func() {
			_, err := async.Write([]byte("second"))
			done <- err
		}()

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("write did not complete in time")
		}

		time.Sleep(150 * time.Millisecond)

		_ = async.Flush()

		written := writer.getWrittenData()
		if len(written) < 2 {
			t.Fatalf("expected both messages to be written, got %v", written)
		}
	})

	t.Run("handoff strategy", func(t *testing.T) {
		writer := newMockWriter()
		writer.writeDelay = 200 * time.Millisecond

		async := NewAsyncWriter(writer, AsyncConfig{
			BufferSize:       1,
			OverflowStrategy: AsyncOverflowHandoff,
		})
		defer async.Close()

		if _, err := async.Write([]byte("first")); err != nil {
			t.Fatalf("first write failed: %v", err)
		}

		if _, err := async.Write([]byte("second")); err != nil {
			t.Fatalf("handoff write failed: %v", err)
		}

		time.Sleep(250 * time.Millisecond)

		_ = async.Flush()

		written := writer.getWrittenData()
		if len(written) != 2 {
			t.Fatalf("expected two messages written, got %d", len(written))
		}
	})

	t.Run("write critical bypasses queue", func(t *testing.T) {
		writer := newMockWriter()
		writer.writeDelay = 200 * time.Millisecond

		async := NewAsyncWriter(writer, AsyncConfig{BufferSize: 1})
		defer async.Close()

		if _, err := async.Write([]byte("background")); err != nil {
			t.Fatalf("initial write failed: %v", err)
		}

		if _, err := async.WriteCritical([]byte("critical")); err != nil {
			t.Fatalf("critical write failed: %v", err)
		}

		time.Sleep(250 * time.Millisecond)

		_ = async.Flush()

		written := writer.getWrittenData()
		if len(written) != 2 {
			t.Fatalf("expected two messages written, got %d", len(written))
		}

		metrics := async.Metrics()
		if metrics.Bypassed < 1 {
			t.Fatalf("expected bypassed metrics to increase")
		}
	})
}

func TestAsyncWriter_ReusesPayloadBuffers(t *testing.T) {
	writer := newMockWriter()

	async := NewAsyncWriter(writer, AsyncConfig{BufferSize: 1})
	defer async.Close()

	var newCount atomic.Int32

	async.payloadPool = &sync.Pool{
		New: func() any {
			newCount.Add(1)

			buf := make([]byte, 0, 64)

			return &buf
		},
	}

	require.Equal(t, int32(0), newCount.Load())

	_, err := async.Write([]byte("first"))
	require.NoError(t, err)

	err = async.Flush()
	require.NoError(t, err)

	require.LessOrEqual(t, newCount.Load(), int32(2))

	_, err = async.Write([]byte("second"))
	require.NoError(t, err)

	err = async.Flush()
	require.NoError(t, err)

	require.LessOrEqual(t, newCount.Load(), int32(2))
}

func TestAsyncWriter_DropPayloadRetention(t *testing.T) {
	writer := newMockWriter()
	writer.writeDelay = 200 * time.Millisecond

	var (
		lease    hyperlogger.PayloadLease
		received string
	)

	async := NewAsyncWriter(writer, AsyncConfig{
		BufferSize: 1,
		DropPayloadHandler: func(payload hyperlogger.DropPayload) {
			received = string(payload.Bytes())
			appended := payload.AppendTo(make([]byte, 0, payload.Size()))
			require.Equal(t, received, string(appended))

			lease = payload.Retain()
			require.NotNil(t, lease)

			secondary := payload.Retain()
			require.NotNil(t, secondary)
			secondary.Release()
		},
	})
	defer async.Close()

	_, err := async.Write([]byte("first"))
	require.NoError(t, err)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_, err = async.Write([]byte("second"))
		if errors.Is(err, ErrBufferFull) {
			break
		}

		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	require.ErrorIs(t, err, ErrBufferFull)
	require.Equal(t, "second", received)
	require.NotNil(t, lease)
	require.Equal(t, "second", string(lease.Bytes()))

	lease.Release()
	lease.Release() // idempotent

	writer.mu.Lock()
	writer.writeDelay = 0
	writer.mu.Unlock()

	require.NoError(t, async.Flush())

	_, err = async.Write([]byte("third"))
	require.NoError(t, err)
	require.NoError(t, async.Flush())
}

func TestAsyncWriter_DropPayloadAutoRelease(t *testing.T) {
	writer := newMockWriter()
	writer.writeDelay = 200 * time.Millisecond

	var dropCount atomic.Int32

	async := NewAsyncWriter(writer, AsyncConfig{
		BufferSize: 1,
		DropPayloadHandler: func(payload hyperlogger.DropPayload) {
			dropCount.Add(1)
			require.Equal(t, "second", string(payload.Bytes()))
		},
	})
	defer async.Close()

	_, err := async.Write([]byte("first"))
	require.NoError(t, err)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_, err = async.Write([]byte("second"))
		if errors.Is(err, ErrBufferFull) {
			break
		}

		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	require.ErrorIs(t, err, ErrBufferFull)
	require.Equal(t, int32(1), dropCount.Load())

	writer.mu.Lock()
	writer.writeDelay = 0
	writer.mu.Unlock()
	require.NoError(t, async.Flush())

	_, err = async.Write([]byte("third"))
	require.NoError(t, err)
	require.NoError(t, async.Flush())
	require.Equal(t, int32(1), dropCount.Load())
}

func TestAsyncWriter_Metrics(t *testing.T) {
	writer := newMockWriter()
	writer.writeDelay = 750 * time.Millisecond

	var reported atomic.Pointer[AsyncMetrics]

	async := NewAsyncWriter(writer, AsyncConfig{
		BufferSize:       1,
		OverflowStrategy: AsyncOverflowDropNewest,
		MetricsReporter: func(m AsyncMetrics) {
			snapshot := m
			reported.Store(&snapshot)
		},
	})
	defer async.Close()

	_, err := async.Write([]byte("first"))
	require.NoError(t, err)

	var overflow bool

	deadline := time.Now().Add(2 * time.Second)
	for !overflow && time.Now().Before(deadline) {
		_, err = async.Write([]byte("second"))
		if err == nil {
			time.Sleep(10 * time.Millisecond)

			continue
		}

		if !errors.Is(err, ErrBufferFull) {
			t.Fatalf("expected ErrBufferFull or nil, got %v", err)
		}

		overflow = true
	}

	if !overflow {
		t.Fatal("failed to trigger buffer overflow")
	}

	writer.mu.Lock()
	writer.writeDelay = 0
	writer.mu.Unlock()

	time.Sleep(200 * time.Millisecond)

	snapshot := async.Metrics()
	if snapshot.Enqueued == 0 {
		t.Fatalf("expected enqueued entries to be tracked")
	}

	if snapshot.Dropped == 0 {
		t.Fatalf("expected dropped entries to be tracked")
	}

	if snapshot.Bypassed != 0 {
		t.Fatalf("expected bypassed entries to remain zero")
	}

	if latest := reported.Load(); latest == nil || latest.Dropped == 0 {
		t.Fatalf("expected metrics reporter to receive updates")
	}
}

func TestAsyncWriter_Retry(t *testing.T) {
	writer := newMockWriter()
	writer.transientError = ewrap.New("temporary")
	writer.failuresBeforeSuccess = 2

	async := NewAsyncWriter(writer, AsyncConfig{
		RetryEnabled:           true,
		MaxRetries:             3,
		RetryBackoff:           5 * time.Millisecond,
		RetryBackoffMultiplier: 1.0,
		RetryMaxBackoff:        5 * time.Millisecond,
	})
	defer async.Close()

	_, err := async.Write([]byte("retry message"))
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	metrics := async.Metrics()
	if metrics.Processed == 0 {
		t.Fatalf("expected message to eventually process")
	}

	if metrics.Retried != 2 {
		t.Fatalf("expected 2 retries, got %d", metrics.Retried)
	}

	if metrics.WriteError != 2 {
		t.Fatalf("expected 2 write errors, got %d", metrics.WriteError)
	}

	data := writer.getWrittenData()
	if len(data) != 1 {
		t.Fatalf("expected one successful write, got %d", len(data))
	}
}

func TestAsyncWriter_Flush(t *testing.T) {
	t.Run("successful flush", func(t *testing.T) {
		writer := newMockWriter()

		async := NewAsyncWriter(writer, AsyncConfig{})
		defer async.Close()

		_, err := async.Write([]byte("test"))
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}

		err = async.Flush()
		if err != nil {
			t.Errorf("Flush error: %v", err)
		}
	})

	t.Run("flush timeout", func(t *testing.T) {
		writer := newMockWriter()
		writer.writeDelay = 100 * time.Millisecond

		async := NewAsyncWriter(writer, AsyncConfig{
			WaitTimeout: 1 * time.Nanosecond,
		})

		_, err := async.Write([]byte("test"))
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}

		err = async.Flush()
		if !errors.Is(err, ErrFlushTimeout) {
			t.Errorf("Expected ErrFlushTimeout, got %v", err)
		}

		// Close with a timeout to avoid hanging if the goroutine is stuck
		// Remove the writeDelay to allow the goroutine to finish
		writer.mu.Lock()
		writer.writeDelay = 0
		writer.mu.Unlock()

		// Give some time for the handleFlush to potentially complete
		time.Sleep(10 * time.Millisecond)

		done := make(chan error, 1)

		go func() {
			done <- async.Close()
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Logf("Close error (expected in some cases): %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Close did not return within timeout")
		}
	})
}

func TestAsyncWriter_Sync(t *testing.T) {
	writer := newMockWriter()

	async := NewAsyncWriter(writer, AsyncConfig{})
	defer async.Close()

	_, err := async.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	err = async.Sync()
	if err != nil {
		t.Errorf("Sync error: %v", err)
	}
}

func TestAsyncWriter_Close(t *testing.T) {
	t.Run("successful close", func(t *testing.T) {
		writer := newMockWriter()
		async := NewAsyncWriter(writer, AsyncConfig{})

		_, err := async.Write([]byte("test"))
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}

		err = async.Close()
		if err != nil {
			t.Errorf("Close error: %v", err)
		}
	})

	t.Run("close twice", func(t *testing.T) {
		writer := newMockWriter()
		async := NewAsyncWriter(writer, AsyncConfig{})

		err := async.Close()
		if err != nil {
			t.Fatalf("First close error: %v", err)
		}

		err = async.Close()
		if !errors.Is(err, ErrWriterClosed) {
			t.Errorf("Expected ErrWriterClosed, got %v", err)
		}
	})
}

func TestAsyncWriter_ErrorHandling(t *testing.T) {
	t.Run("write error", func(t *testing.T) {
		writer := newMockWriter()
		writer.writeError = ewrap.New("write failed")

		var (
			gotError error
			errorWg  sync.WaitGroup
		)

		errorWg.Add(1)

		async := NewAsyncWriter(writer, AsyncConfig{
			ErrorHandler: func(err error) {
				gotError = err

				errorWg.Done()
			},
		})
		defer async.Close()

		_, err := async.Write([]byte("test"))
		if err != nil {
			t.Fatalf("Write should not return immediate error: %v", err)
		}

		// Wait for error handler to be called
		errorWg.Wait()

		if gotError == nil || gotError.Error() != "write failed" {
			t.Errorf("Expected 'write failed' error, got %v", gotError)
		}
	})
}

func TestAsyncWriter_ConcurrentWrites(t *testing.T) {
	writer := newMockWriter()

	async := NewAsyncWriter(writer, AsyncConfig{
		BufferSize: 100,
	})
	defer async.Close()

	const (
		goroutines           = 10
		messagesPerGoroutine = 10
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()

			for range messagesPerGoroutine {
				_, err := async.Write([]byte("test"))
				if err != nil {
					t.Errorf("Write error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	err := async.Flush()
	if err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	written := writer.getWrittenData()

	expected := goroutines * messagesPerGoroutine
	if len(written) != expected {
		t.Errorf("Expected %d messages, got %d", expected, len(written))
	}
}
