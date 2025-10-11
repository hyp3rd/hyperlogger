package hyperlogger

import (
	"bytes"
	"sync"

	"github.com/hyp3rd/ewrap"
)

// Encoder encodes a log entry into bytes suitable for output writers.
// Implementations can reuse the provided buffer to minimize allocations.
type Encoder interface {
	Encode(entry *Entry, cfg *Config, buf *bytes.Buffer) ([]byte, error)
	EstimateSize(entry *Entry) int
}

// EncoderRegistry manages named encoders that can be referenced from configuration.
type EncoderRegistry struct {
	mu       sync.RWMutex
	encoders map[string]Encoder
}

// NewEncoderRegistry creates an empty encoder registry.
func NewEncoderRegistry() *EncoderRegistry {
	return &EncoderRegistry{
		encoders: make(map[string]Encoder),
	}
}

// Register adds an encoder to the registry under the provided name.
func (r *EncoderRegistry) Register(name string, encoder Encoder) error {
	if name == "" {
		return ewrap.New("encoder name cannot be empty")
	}

	if encoder == nil {
		return ewrap.New("encoder cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.encoders[name]; exists {
		return ewrap.New("encoder already registered").WithMetadata("name", name)
	}

	r.encoders[name] = encoder

	return nil
}

// Get retrieves an encoder by name.
func (r *EncoderRegistry) Get(name string) (Encoder, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	enc, ok := r.encoders[name]

	return enc, ok
}

// MustRegister registers an encoder and panics if registration fails.
func (r *EncoderRegistry) MustRegister(name string, encoder Encoder) {
	err := r.Register(name, encoder)
	if err != nil {
		panic(err)
	}
}
