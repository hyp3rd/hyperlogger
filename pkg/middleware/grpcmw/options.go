package grpcmw

// Option defines a configuration option for the gRPC middleware.
type Option func(*options)

type options struct {
	traceKey   string
	requestKey string
}

// WithTraceKey customizes the metadata key used to populate the trace identifier.
func WithTraceKey(name string) Option {
	return func(o *options) {
		if o == nil || name == "" {
			return
		}

		o.traceKey = name
	}
}

// WithRequestKey customizes the metadata key used to populate the request identifier.
func WithRequestKey(name string) Option {
	return func(o *options) {
		if o == nil || name == "" {
			return
		}

		o.requestKey = name
	}
}
