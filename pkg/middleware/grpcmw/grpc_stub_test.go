//go:build !grpc

package grpcmw

import (
	"context"
	"testing"
)

func TestUnaryServerInterceptorStub(t *testing.T) {
	interceptor := UnaryServerInterceptor()
	_, err := interceptor(context.Background(), nil, nil, nil)
	if err != ErrGRPCNotEnabled {
		t.Fatalf("expected ErrGRPCNotEnabled, received %v", err)
	}
}
