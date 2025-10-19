package hyperlogger

// DropPayload represents a log payload that was dropped by the asynchronous writer.
// Handlers can inspect the payload, copy it, or retain ownership of the underlying
// buffer by calling Retain. When Retain is used, the returned PayloadLease must be
// released once the handler finishes processing to allow buffer reuse.
type DropPayload interface {
	// Bytes returns a read-only view of the dropped payload.
	Bytes() []byte
	// Size reports the number of bytes contained in the payload.
	Size() int
	// AppendTo appends the payload bytes to the provided destination slice and returns it.
	AppendTo(dst []byte) []byte
	// Retain acquires a lease over the underlying buffer. The returned lease's Release
	// method must be called once the handler no longer needs the payload so the buffer
	// can be reclaimed. Calling Retain more than once returns a no-op lease.
	Retain() PayloadLease
}

// PayloadLease represents ownership of a dropped payload buffer. Call Release when
// finished with the buffer to allow it to be recycled. Release is idempotent.
type PayloadLease interface {
	Bytes() []byte
	Release()
}

// DropPayloadHandler receives advanced drop notifications with ownership semantics.
// Handlers can retain dropped payloads without incurring additional allocations.
type DropPayloadHandler func(DropPayload)
