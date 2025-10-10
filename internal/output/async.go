package output

import (
	"io"
	"sync"
	"time"

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
}

// AsyncWriter implements asynchronous writing to an io.Writer,
// buffering writes through a channel to decouple logging from I/O operations.
type AsyncWriter struct {
	out        io.Writer
	config     AsyncConfig
	msgCh      chan []byte
	stopCh     chan struct{}
	flushCh    chan chan struct{}
	wg         sync.WaitGroup
	closed     bool
	closeMutex sync.Mutex
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

	aw := &AsyncWriter{
		out:     out,
		config:  config,
		msgCh:   make(chan []byte, config.BufferSize),
		stopCh:  make(chan struct{}),
		flushCh: make(chan chan struct{}, 1),
	}

	aw.start()

	return aw
}

// Write implements the io.Writer interface for asynchronous writing.
func (w *AsyncWriter) Write(data []byte) (int, error) {
	w.closeMutex.Lock()
	closed := w.closed
	w.closeMutex.Unlock()

	if closed {
		return 0, ErrWriterClosed
	}

	// Copy the buffer since the original might be reused
	buf := make([]byte, len(data))
	copy(buf, data)

	// Try to send the message to the channel
	select {
	case w.msgCh <- buf:
		return len(data), nil
	default:
		// Channel is full, either block or return an error
		// Depending on application needs, you might want to block here
		// or handle differently
		if w.config.ErrorHandler != nil {
			w.config.ErrorHandler(ErrBufferFull)
		}

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
		return nil
	case <-time.After(w.config.WaitTimeout):
		return ErrFlushTimeout
	}
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

	return nil
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
func (w *AsyncWriter) writeMessage(msg []byte) {
	_, err := w.out.Write(msg)
	if err != nil && w.config.ErrorHandler != nil {
		w.config.ErrorHandler(err)
	}
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
