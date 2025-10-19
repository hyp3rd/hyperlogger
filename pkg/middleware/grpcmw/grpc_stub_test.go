//go:build !grpc

package grpcmw

import (
	"context"
	"errors"
	"testing"
)

func TestUnaryServerInterceptorStub(t *testing.T) {
	interceptor := UnaryServerInterceptor()

	_, err := interceptor(context.Background(), nil, nil, nil)
	if !errors.Is(err, ErrGRPCNotEnabled) {
		t.Fatalf("expected ErrGRPCNotEnabled, received %v", err)
	}
}
