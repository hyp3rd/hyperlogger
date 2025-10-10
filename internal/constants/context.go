package constants

// Context key types to avoid collisions when using context.WithValue.
type (
	traceIDKeyType   struct{}
	requestIDKeyType struct{}
)

// Context keys for values that may be stored in a context.Context and extracted
// by the logger.
var (
	// TraceIDKey is the context key for the trace ID.
	//
	//nolint:gochecknoglobals
	TraceIDKey = traceIDKeyType{}
	// RequestIDKey is the context key for the request ID.
	//
	//nolint:gochecknoglobals
	RequestIDKey = requestIDKeyType{}
)

// NamespaceKey is a type for the namespace key to avoid collisions.
type NamespaceKey struct{}
