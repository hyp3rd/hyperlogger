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
//
//nolint:revive // This is a stub function when gRPC is not enabled.
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return nil, ErrGRPCNotEnabled
	}
}
