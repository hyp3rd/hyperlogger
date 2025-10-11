package httpmw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyp3rd/hyperlogger/internal/constants"
)

func TestContextMiddleware(t *testing.T) {
	middleware := ContextMiddleware(WithIDGenerator(func() string { return "generated" }))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID, _ := r.Context().Value(constants.TraceKey{}).(string)
		requestID, _ := r.Context().Value(constants.RequestKey{}).(string)

		require.NotEmpty(t, traceID)
		require.NotEmpty(t, requestID)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(rr, req)
}

func TestContextMiddlewareHeaders(t *testing.T) {
	middleware := ContextMiddleware(WithGenerateMissingIDs(false))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID, _ := r.Context().Value(constants.TraceKey{}).(string)
		requestID, _ := r.Context().Value(constants.RequestKey{}).(string)

		require.Equal(t, "trace", traceID)
		require.Equal(t, "req", requestID)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "trace")
	req.Header.Set("X-Request-ID", "req")

	handler.ServeHTTP(rr, req)
}
