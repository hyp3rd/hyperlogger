//go:build grpc

package grpcmw

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/hyp3rd/hyperlogger/internal/constants"
)

// UnaryServerInterceptor enriches the gRPC context with metadata values.
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	cfg := applyOptions(opts...)

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
