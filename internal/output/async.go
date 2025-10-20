package output

import (
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/internal/constants"
)

// AsyncConfig configures an AsyncWriter.
type AsyncConfig struct {
	// BufferSize is the size of the message buffer channel.
	BufferSize int
	// WaitTimeout is the maximum time to wait for all logs to be written during Flush.
	WaitTimeout time.Duration
	// ErrorHandler is called when an error occurs during async writing.
	ErrorHandler func(error)
	// OverflowStrategy controls what happens when the buffer is full.
	OverflowStrategy AsyncOverflowStrategy
	// DropHandler is invoked with the dropped payload when overflow strategy discards logs.
	DropHandler func([]byte)
	// DropPayloadHandler receives drop notifications with ownership semantics.
	DropPayloadHandler hyperlogger.DropPayloadHandler
	// MetricsReporter receives periodic metrics about the async writer state.
	MetricsReporter func(AsyncMetrics)
	// RetryEnabled enables retry attempts on write failures.
	RetryEnabled bool
	// MaxRetries defines the number of retry attempts after the initial write.
	MaxRetries int
	// RetryBackoff is the base backoff duration between retries.
	RetryBackoff time.Duration
	// RetryBackoffMultiplier scales the backoff after each retry.
	RetryBackoffMultiplier float64
	// RetryMaxBackoff caps the retry backoff duration.
	RetryMaxBackoff time.Duration
}

// AsyncWriter implements asynchronous writing to an io.Writer,
// buffering writes through a channel to decouple logging from I/O operations.
type AsyncWriter struct {
	out         io.Writer
	config      AsyncConfig
	msgCh       chan *asyncPayload
	stopCh      chan struct{}
	flushCh     chan chan struct{}
	wg          sync.WaitGroup
	closed      bool
	closeMutex  sync.Mutex
	metricsMu   sync.Mutex
	payloadPool *sync.Pool

	enqueuedCount  atomic.Uint64
	processedCount atomic.Uint64
	droppedCount   atomic.Uint64
	writeErrors    atomic.Uint64
	retryCount     atomic.Uint64
	bypassCount    atomic.Uint64
}

// AsyncMetrics provides insight into the internal state of the AsyncWriter.
type AsyncMetrics struct {
	Enqueued   uint64
	Processed  uint64
	Dropped    uint64
	WriteError uint64
	Retried    uint64
	QueueDepth int
	Bypassed   uint64
}

// AsyncOverflowStrategy defines how AsyncWriter behaves when buffer is full.
type AsyncOverflowStrategy int

const (
	// AsyncOverflowDropNewest drops the incoming log entry (default, previous behaviour).
	AsyncOverflowDropNewest AsyncOverflowStrategy = iota
	// AsyncOverflowBlock makes writers block until there is space in the buffer.
	AsyncOverflowBlock
	// AsyncOverflowDropOldest discards the oldest buffered entry to make space for the new one.
	AsyncOverflowDropOldest
	// AsyncOverflowHandoff writes the entry synchronously when the buffer is full.
	AsyncOverflowHandoff
)

const (
	defaultRetryBackoff = 10
)

type asyncPayload struct {
	data    []byte
	storage *[]byte
}

// NewAsyncWriter creates a new AsyncWriter that writes to the given writer asynchronously.
func NewAsyncWriter(out io.Writer, config AsyncConfig) *AsyncWriter {
	// Set defaults for config if needed
	if config.BufferSize <= 0 {
		config.BufferSize = 1024
	}

	if config.WaitTimeout <= 0 {
		config.WaitTimeout = constants.DefaultTimeout
	}

	if config.ErrorHandler == nil {
		config.ErrorHandler = func(error) {}
	}

	if config.DropHandler == nil {
		config.DropHandler = func([]byte) {}
	}

	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}

	if config.RetryBackoff <= 0 {
		config.RetryBackoff = defaultRetryBackoff * time.Millisecond
	}

	if config.RetryBackoffMultiplier <= 1 {
		config.RetryBackoffMultiplier = 2
	}

	if config.RetryMaxBackoff <= 0 {
		config.RetryMaxBackoff = config.RetryBackoff * defaultRetryBackoff
	}

	initialPayloadCap := defaultBufferSize
	if config.BufferSize > initialPayloadCap {
		initialPayloadCap = config.BufferSize
	}

	payloadPool := &sync.Pool{
		New: func() any {
			buf := make([]byte, 0, initialPayloadCap)

			return &buf
		},
	}

	aw := &AsyncWriter{
		out:         out,
		config:      config,
		msgCh:       make(chan *asyncPayload, config.BufferSize),
		stopCh:      make(chan struct{}),
		flushCh:     make(chan chan struct{}, 1),
		payloadPool: payloadPool,
	}

	aw.start()

	return aw
}

// Underlying returns the writer wrapped by the AsyncWriter.
func (w *AsyncWriter) Underlying() io.Writer {
	return w.out
}

// Write implements the io.Writer interface for asynchronous writing.
func (w *AsyncWriter) Write(data []byte) (int, error) {
	w.closeMutex.Lock()
	closed := w.closed
	w.closeMutex.Unlock()

	if closed {
		return 0, ErrWriterClosed
	}

	payload := w.borrowPayload(data)

	//nolint:exhaustive // AsyncOverflowDropNewest is the default behaviour
	switch w.config.OverflowStrategy {
	case AsyncOverflowBlock:
		select {
		case w.msgCh <- payload:
			w.enqueuedCount.Add(1)
			w.reportMetrics()

			return len(data), nil
		case <-w.stopCh:
			w.releasePayload(payload)

			return 0, ErrWriterClosed
		}
	case AsyncOverflowDropOldest:
		if w.tryEnqueue(payload) {
			w.reportMetrics()

			return len(data), nil
		}

		w.discardOldest()

		if w.tryEnqueue(payload) {
			w.reportMetrics()

			return len(data), nil
		}

		w.recordOverflow(payload)

		return 0, ErrBufferFull
	case AsyncOverflowHandoff:
		if w.tryEnqueue(payload) {
			w.reportMetrics()

			return len(data), nil
		}

		return w.writeDirect(payload)
	default:
		if w.tryEnqueue(payload) {
			w.reportMetrics()

			return len(data), nil
		}

		w.recordOverflow(payload)

		return 0, ErrBufferFull
	}
}

// Sync ensures that all buffered logs have been written to the underlying writer.
// This method is called by the Logger's Sync() method, typically before application shutdown.
func (w *AsyncWriter) Sync() error {
	return w.Flush()
}

// Flush waits for all logs to be written.
func (w *AsyncWriter) Flush() error {
	w.closeMutex.Lock()

	if w.closed {
		w.closeMutex.Unlock()

		return ErrWriterClosed
	}

	w.closeMutex.Unlock()

	// Create a channel to signal when flush is complete
	doneCh := make(chan struct{})

	// Send flush signal
	w.flushCh <- doneCh

	// Wait for flush to complete or timeout
	select {
	case <-doneCh:
		return w.syncUnderlying()
	case <-time.After(w.config.WaitTimeout):
		return ErrFlushTimeout
	}
}

// WriteCritical bypasses the internal buffer and writes synchronously.
func (w *AsyncWriter) WriteCritical(data []byte) (int, error) {
	w.closeMutex.Lock()
	closed := w.closed
	w.closeMutex.Unlock()

	if closed {
		return 0, ErrWriterClosed
	}

	payload := w.borrowPayload(data)

	return w.writeDirect(payload)
}

// Close stops the background goroutine and closes the message channel.
func (w *AsyncWriter) Close() error {
	w.closeMutex.Lock()
	defer w.closeMutex.Unlock()

	if w.closed {
		return ErrWriterClosed
	}

	w.closed = true

	// Signal the goroutine to stop
	close(w.stopCh)
	close(w.msgCh)

	// Wait for the goroutine to finish processing remaining messages
	w.wg.Wait()

	err := w.syncUnderlying()
	if err != nil {
		return err
	}

	return w.closeUnderlying()
}

// Metrics returns a snapshot of the current metrics counters.
func (w *AsyncWriter) Metrics() AsyncMetrics {
	return AsyncMetrics{
		Enqueued:   w.enqueuedCount.Load(),
		Processed:  w.processedCount.Load(),
		Dropped:    w.droppedCount.Load(),
		WriteError: w.writeErrors.Load(),
		Retried:    w.retryCount.Load(),
		QueueDepth: len(w.msgCh),
		Bypassed:   w.bypassCount.Load(),
	}
}

// start begins the background writing goroutine.
func (w *AsyncWriter) start() {
	w.wg.Add(1)

	go w.processLogs()
}

// processLogs is the background goroutine that processes log messages.
func (w *AsyncWriter) processLogs() {
	defer w.wg.Done()

	for {
		select {
		case msg, ok := <-w.msgCh:
			if !ok {
				return
			}

			w.writeMessage(msg)
		case doneCh := <-w.flushCh:
			w.handleFlush(doneCh)
		case <-w.stopCh:
			w.drainMessages()

			return
		}
	}
}

// writeMessage writes a single message to the underlying writer.
func (w *AsyncWriter) writeMessage(payload *asyncPayload) {
	if payload == nil {
		return
	}

	err := w.performWrite(payload.data)
	if err != nil {
		w.handleWriteFailure(payload)

		return
	}

	w.releasePayload(payload)
}

// handleFlush handles a flush request by draining pending messages before signaling completion.
func (w *AsyncWriter) handleFlush(doneCh chan struct{}) {
	// Drain any currently queued messages to ensure they are written before completing the flush.
	for {
		select {
		case msg, ok := <-w.msgCh:
			if !ok {
				close(doneCh)

				return
			}

			w.writeMessage(msg)
		default:
			// Channel is empty at this moment; signal that flush is complete.
			close(doneCh)

			return
		}
	}
}

func (w *AsyncWriter) writeDirect(payload *asyncPayload) (int, error) {
	if payload == nil {
		return 0, nil
	}

	err := w.performWrite(payload.data)
	if err != nil {
		w.releasePayload(payload)

		return 0, err
	}

	w.bypassCount.Add(1)
	w.reportMetrics()

	written := len(payload.data)
	w.releasePayload(payload)

	return written, nil
}

func (w *AsyncWriter) performWrite(msg []byte) error {
	attempt := 0
	backoff := w.config.RetryBackoff

	for {
		_, err := w.out.Write(msg)
		if err == nil {
			w.processedCount.Add(1)
			w.reportMetrics()

			return nil
		}

		w.writeErrors.Add(1)

		if w.config.ErrorHandler != nil {
			w.config.ErrorHandler(err)
		}

		if !w.config.RetryEnabled || attempt >= w.config.MaxRetries {
			w.reportMetrics()

			return ewrap.Wrap(err, "writing log message")
		}

		attempt++

		w.retryCount.Add(1)
		time.Sleep(backoff)
		backoff = time.Duration(math.Min(float64(w.config.RetryMaxBackoff), float64(backoff)*w.config.RetryBackoffMultiplier))
	}
}

// drainMessages processes any remaining messages in the channel.
func (w *AsyncWriter) drainMessages() {
	for {
		select {
		case msg, ok := <-w.msgCh:
			if !ok {
				return
			}

			w.writeMessage(msg)
		default:
			return
		}
	}
}

// discardOldest removes the oldest message from the buffer to make space for a new one.
func (w *AsyncWriter) discardOldest() {
	select {
	case payload, ok := <-w.msgCh:
		if ok {
			w.handleDrop(payload)
			w.droppedCount.Add(1)
			w.reportMetrics()
		}
	default:
	}
}

// recordOverflow handles the case when a message cannot be enqueued due to a full buffer.
func (w *AsyncWriter) recordOverflow(payload *asyncPayload) {
	if payload == nil {
		return
	}

	w.handleDrop(payload)
	w.droppedCount.Add(1)

	if w.config.ErrorHandler != nil {
		w.config.ErrorHandler(ErrBufferFull)
	}

	w.reportMetrics()
}

// tryEnqueue attempts to enqueue a message without blocking.
func (w *AsyncWriter) tryEnqueue(payload *asyncPayload) bool {
	select {
	case w.msgCh <- payload:
		w.enqueuedCount.Add(1)

		return true
	default:
		return false
	}
}

func (w *AsyncWriter) handleWriteFailure(payload *asyncPayload) {
	if payload == nil {
		return
	}

	w.handleDrop(payload)
	w.droppedCount.Add(1)
	w.reportMetrics()
}

func (w *AsyncWriter) handleDrop(payload *asyncPayload) {
	if payload == nil {
		return
	}

	if handler := w.config.DropPayloadHandler; handler != nil {
		dropped := newDropPayload(payload, func() {
			w.releasePayload(payload)
		})
		handler(dropped)
		dropped.releaseIfNeeded()

		return
	}

	if w.config.DropHandler != nil {
		w.config.DropHandler(payload.data)
	}

	w.releasePayload(payload)
}

func (w *AsyncWriter) borrowPayload(src []byte) *asyncPayload {
	size := len(src)

	var storage *[]byte

	if pool := w.payloadPool; pool != nil {
		if raw := pool.Get(); raw != nil {
			if candidate, ok := raw.(*[]byte); ok && candidate != nil {
				storage = candidate
			}
		}
	}

	if storage == nil {
		buf := make([]byte, 0, size)
		storage = &buf
	}

	data := *storage
	if cap(data) < size {
		data = make([]byte, size)
	}

	data = data[:size]
	copy(data, src)
	*storage = data

	return &asyncPayload{
		data:    data,
		storage: storage,
	}
}

func (w *AsyncWriter) releasePayload(payload *asyncPayload) {
	if payload == nil || payload.storage == nil {
		return
	}

	buf := *payload.storage
	buf = buf[:0]
	*payload.storage = buf

	if pool := w.payloadPool; pool != nil {
		pool.Put(payload.storage)
	}

	payload.storage = nil
	payload.data = nil
}

func (w *AsyncWriter) reportMetrics() {
	reporter := w.config.MetricsReporter
	if reporter == nil {
		return
	}

	w.metricsMu.Lock()
	defer w.metricsMu.Unlock()

	reporter(w.Metrics())
}

func (w *AsyncWriter) syncUnderlying() error {
	if syncer, ok := w.out.(interface{ Sync() error }); ok {
		err := syncer.Sync()
		if err != nil {
			return ewrap.Wrap(err, "syncing underlying writer")
		}
	}

	return nil
}

func (w *AsyncWriter) closeUnderlying() error {
	if closer, ok := w.out.(io.Closer); ok {
		if f, ok := closer.(*os.File); ok && isStandardStream(f) {
			return nil
		}

		err := closer.Close()
		if err != nil {
			return ewrap.Wrap(err, "closing underlying writer")
		}
	}

	return nil
}

type payloadLease struct {
	bytes   []byte
	release func()
	once    sync.Once
}

func (l *payloadLease) Bytes() []byte {
	return l.bytes
}

func (l *payloadLease) Release() {
	l.once.Do(func() {
		if l.release != nil {
			l.release()
		}
	})
	l.bytes = nil
}

type noOpLease struct {
	bytes []byte
}

func (n *noOpLease) Bytes() []byte {
	return n.bytes
}

func (n *noOpLease) Release() {
	n.bytes = nil
}

type dropPayload struct {
	data     []byte
	release  func()
	once     sync.Once
	retained atomic.Bool
}

func newDropPayload(payload *asyncPayload, release func()) *dropPayload {
	return &dropPayload{
		data:    payload.data,
		release: release,
	}
}

func (p *dropPayload) Bytes() []byte {
	return p.data
}

func (p *dropPayload) Size() int {
	return len(p.data)
}

func (p *dropPayload) AppendTo(dst []byte) []byte {
	return append(dst, p.data...)
}

func (p *dropPayload) Retain() hyperlogger.PayloadLease {
	if !p.retained.CompareAndSwap(false, true) {
		return &noOpLease{bytes: p.data}
	}

	return &payloadLease{
		bytes: p.data,
		release: func() {
			p.callRelease()
		},
	}
}

func (p *dropPayload) releaseIfNeeded() {
	if p.retained.Load() {
		return
	}

	p.callRelease()
}

func (p *dropPayload) callRelease() {
	p.once.Do(func() {
		if p.release != nil {
			p.release()
		}
	})
}

var (
	_ hyperlogger.PayloadLease = (*payloadLease)(nil)
	_ hyperlogger.PayloadLease = (*noOpLease)(nil)
	_ hyperlogger.DropPayload  = (*dropPayload)(nil)
)
