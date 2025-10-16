//go:build grpc

package grpcmw

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/hyp3rd/hyperlogger/internal/constants"
)

func TestUnaryServerInterceptorMetadataExtraction(t *testing.T) {
	t.Parallel()

	traceID := "trace-123"
	requestID := "request-456"

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constants.TraceHeader, traceID,
		constants.RequestHeader, requestID,
	))

	interceptor := UnaryServerInterceptor()

	var capturedTrace, capturedRequest string

	handler := func(ctx context.Context, req any) (any, error) {
		traceValue, _ := ctx.Value(constants.TraceKey{}).(string)
		requestValue, _ := ctx.Value(constants.RequestKey{}).(string)

		capturedTrace = traceValue
		capturedRequest = requestValue

		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	require.NoError(t, err)
	require.Equal(t, traceID, capturedTrace)
	require.Equal(t, requestID, capturedRequest)
}

func TestUnaryServerInterceptorCustomKeys(t *testing.T) {
	t.Parallel()

	traceKey := "x-trace"
	requestKey := "x-request"

	traceID := "custom-trace"
	requestID := "custom-request"

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		traceKey, traceID,
		requestKey, requestID,
	))

	interceptor := UnaryServerInterceptor(
		WithTraceKey(traceKey),
		WithRequestKey(requestKey),
	)

	handler := func(ctx context.Context, req any) (any, error) {
		require.Equal(t, traceID, ctx.Value(constants.TraceKey{}))
		require.Equal(t, requestID, ctx.Value(constants.RequestKey{}))

		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	require.NoError(t, err)
}
