package httpclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"syscall"
)

var (
	// retryableStatuses are the HTTP statuses worth another attempt.
	retryableStatuses = map[int]struct{}{
		http.StatusTooManyRequests:     {},
		http.StatusInternalServerError: {},
		http.StatusBadGateway:          {},
		http.StatusServiceUnavailable:  {},
		http.StatusGatewayTimeout:      {},
	}
	// idempotentMethods are safe to replay per RFC 9110.
	idempotentMethods = map[string]struct{}{
		http.MethodGet:     {},
		http.MethodHead:    {},
		http.MethodOptions: {},
		http.MethodPut:     {},
		http.MethodDelete:  {},
		http.MethodTrace:   {},
	}
)

func isCallerCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// defaultRetryIf is the static classifier: retry on a per-attempt timeout, a retryable
// status, a timeout net.Error, or a refused/reset connection. Caller-context death and
// everything else (including any 4xx other than 429) is terminal. It sees only the error,
// never the request method, so the non-idempotent gate lives in the transport, not here.
func defaultRetryIf(err error) bool {
	if err == nil {
		return false
	}
	if isCallerCtxErr(err) {
		return false
	}
	if errors.Is(err, errAttemptTimeout) {
		return true
	}
	var rse retryableStatusError
	if errors.As(err, &rse) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET)
}
