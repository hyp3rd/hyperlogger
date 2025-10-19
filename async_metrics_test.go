package hyperlogger

import (
	"context"
	"testing"
)

func TestRegisterAsyncMetricsHandler(t *testing.T) {
	ClearAsyncMetricsHandlers()
	t.Cleanup(ClearAsyncMetricsHandlers)

	called := false

	RegisterAsyncMetricsHandler(func(ctx context.Context, metrics AsyncMetrics) {
		called = true
	})

	EmitAsyncMetrics(context.Background(), AsyncMetrics{})

	if !called {
		t.Fatalf("expected registered handler to be invoked")
	}
}
