//go:build grpc

package grpcmw

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/hyp3rd/hyperlogger/internal/constants"
)

func actualOptions(opts ...Option) options {
	cfg := options{}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.traceKey == "" {
		cfg.traceKey = "x-trace-id"
	}

	if cfg.requestKey == "" {
		cfg.requestKey = "x-request-id"
	}

	return cfg
}

// UnaryServerInterceptor enriches the gRPC context with metadata values.
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	cfg := actualOptions(opts...)

	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(cfg.traceKey); len(values) > 0 {
				ctx = context.WithValue(ctx, constants.TraceKey{}, values[0])
			}

			if values := md.Get(cfg.requestKey); len(values) > 0 {
				ctx = context.WithValue(ctx, constants.RequestKey{}, values[0])
			}
		}

		return handler(ctx, req)
	}
}
