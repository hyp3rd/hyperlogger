//go:build !grpc

package grpcmw

import (
	"context"

	"github.com/hyp3rd/ewrap"
	"google.golang.org/grpc"
)

// ErrGRPCNotEnabled indicates that the gRPC middleware was built without gRPC support.
var ErrGRPCNotEnabled = ewrap.New("grpc middleware requires build tag 'grpc'")

// UnaryServerInterceptor returns a stub interceptor when the gRPC build tag is not provided.
func UnaryServerInterceptor(_ ...Option) grpc.UnaryServerInterceptor {
	return func(_ context.Context, _ any, _ *grpc.UnaryServerInfo, _ grpc.UnaryHandler) (any, error) {
		return nil, ErrGRPCNotEnabled
	}
}
