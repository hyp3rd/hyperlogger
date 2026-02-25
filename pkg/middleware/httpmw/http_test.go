package httpmw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hyp3rd/hyperlogger/internal/constants"
)

func TestContextMiddleware(t *testing.T) {
	middleware := ContextMiddleware(WithIDGenerator(func() string { return "generated" }))

	handler := middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		traceID, traceOK := r.Context().Value(constants.TraceKey{}).(string)
		requestID, requestOK := r.Context().Value(constants.RequestKey{}).(string)

		assert.True(t, traceOK)
		assert.True(t, requestOK)
		assert.NotEmpty(t, traceID)
		assert.NotEmpty(t, requestID)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(rr, req)
}

func TestContextMiddlewareHeaders(t *testing.T) {
	middleware := ContextMiddleware(WithGenerateMissingIDs(false))

	handler := middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		traceID, traceOK := r.Context().Value(constants.TraceKey{}).(string)
		requestID, requestOK := r.Context().Value(constants.RequestKey{}).(string)

		assert.True(t, traceOK)
		assert.True(t, requestOK)
		assert.Equal(t, "trace", traceID)
		assert.Equal(t, "req", requestID)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(constants.TraceHeader, "trace")
	req.Header.Set(constants.RequestHeader, "req")

	handler.ServeHTTP(rr, req)
}
