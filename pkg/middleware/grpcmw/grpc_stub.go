package grpcmw

import (
	"context"
	"errors"
)

// Option defines a configuration option for the gRPC middleware.
type Option func(*options)

type options struct{}

// ErrGRPCNotEnabled indicates that the gRPC middleware was built without gRPC support.
var ErrGRPCNotEnabled = errors.New("grpc middleware requires build tag 'grpc'")

// UnaryServerInterceptor returns a stub interceptor when the gRPC build tag is not provided.
//
//nolint:revive // This is a stub function when gRPC is not enabled --- IGNORE ---
func UnaryServerInterceptor(opts ...Option) func(ctx context.Context, req any, info any, handler any) (any, error) {
	return func(ctx context.Context, req any, info any, handler any) (any, error) {
		return nil, ErrGRPCNotEnabled
	}
}

// WithTraceKey is a no-op placeholder without gRPC support.
func WithTraceKey(string) Option { return func(*options) {} }

// WithRequestKey is a no-op placeholder without gRPC support.
func WithRequestKey(string) Option { return func(*options) {} }
