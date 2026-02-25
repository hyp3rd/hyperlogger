package hyperlogger

import (
	"context"
	"testing"
)

func TestRegisterAsyncMetricsHandler(t *testing.T) {
	ClearAsyncMetricsHandlers()
	t.Cleanup(ClearAsyncMetricsHandlers)

	called := false

	RegisterAsyncMetricsHandler(func(_ context.Context, _ AsyncMetrics) {
		called = true
	})

	EmitAsyncMetrics(context.Background(), AsyncMetrics{})

	if !called {
		t.Fatal("expected registered handler to be invoked")
	}
}
