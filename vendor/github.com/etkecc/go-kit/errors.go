package kit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// ErrUnknown is the fallback sentinel for an error you can't classify any better.
var ErrUnknown = errors.New("unknown error")

// maxMatrixErrorBody caps how much of a response body MatrixErrorFrom will read, so a hostile or
// broken server can't turn "decode the error" into an unbounded read that eats all your memory.
const maxMatrixErrorBody = 64 << 10 // 64 KiB, roomy for any real Matrix error JSON

// AggregateError collects errors that happened together and reports them as one. Reach for it
// when a batch of validations or a fan-out of workers each fail their own way and you want the
// whole story, not just whichever one lost the race to return first.
//
// Safe for concurrent use: every method takes the internal lock. The zero value is ready to go.
// Read the underlying slice via Unwrap, but treat it as read-only, it's the original, not a copy.
type AggregateError struct {
	mu     sync.RWMutex
	Errors []error
}

// Error joins every collected message with "; ". Empty aggregate, empty string. Implements error.
func (a *AggregateError) Error() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.Errors) == 0 {
		return ""
	}

	msgs := make([]string, 0, len(a.Errors))
	for _, err := range a.Errors {
		msgs = append(msgs, err.Error())
	}

	return strings.Join(msgs, "; ")
}

// Unwrap returns the underlying slice for errors.Is/As to walk. It's the original, not a copy,
// so don't mutate it, and don't hold onto it across a concurrent Add: the append runs under the
// lock but your copy of the slice header doesn't, so a reallocation leaves you reading a stale
// backing array while the aggregate moves on. Use it and drop it. Nil when the aggregate is empty.
func (a *AggregateError) Unwrap() []error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.Errors) == 0 {
		return nil
	}

	return a.Errors
}

// Is reports whether target matches any error in the bag, running errors.Is on each. So a
// wrapped sentinel buried in one of the collected errors still answers true.
func (a *AggregateError) Is(target error) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, err := range a.Errors {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}

// As finds the first collected error assignable to target, running errors.As on each, so a
// type assertion reaches through the bag and the wrapping alike.
func (a *AggregateError) As(target any) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, err := range a.Errors {
		if errors.As(err, target) {
			return true
		}
	}

	return false
}

// Join drops the non-nil errors into the bag, skips the nils without complaint, and hands back
// the receiver, or nil if the bag is still empty afterward:
//
//	if err := agg.Join(e1, e2, e3); err != nil { ... }
//
// That nil-when-empty return is the whole trick. It's what keeps the line above from propagating
// an empty aggregate that stringifies to nothing and quietly passes every downstream != nil check.
func (a *AggregateError) Join(errs ...error) *AggregateError {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Errors == nil {
		a.Errors = []error{}
	}

	for _, err := range errs {
		if err != nil {
			a.Errors = append(a.Errors, err)
		}
	}

	if len(a.Errors) == 0 {
		return nil
	}

	return a
}

// NewAggregateError builds an aggregate from the given errors, skipping nils, and returns a nil
// *AggregateError if none of them were real, so the line below reads true only when something
// actually went wrong:
//
//	if err := NewAggregateError(e1, e2); err != nil { ... }
func NewAggregateError(errs ...error) *AggregateError {
	agg := &AggregateError{}
	return agg.Join(errs...)
}

// ErrorResponse is an error you can hand straight back to an HTTP client: it carries a status
// code and marshals to {"error": "..."}. StatusCode defaults to 400 and never touches the JSON.
// The wrapped error rides along for errors.Is/As but stays out of the body, so an internal detail
// (a driver error, a file path, a stack-trace-shaped message) never leaks to whoever's on the
// other end of the wire.
//
//nolint:errname // ErrorResponse is a valid name
type ErrorResponse struct {
	StatusCode int    `json:"-"`     // HTTP status code, optional, not serialized
	Err        string `json:"error"` // Error message
	err        error  `json:"-"`     // underlying error, optional, not serialized
}

// Error returns the message. Implements error.
func (e ErrorResponse) Error() string {
	return e.Err
}

// Unwrap hands back the wrapped error so errors.Is/As can walk the chain.
func (e ErrorResponse) Unwrap() error {
	return e.err
}

// NewErrorResponse wraps err as a 400 by default; pass a positive status to override it. A nil
// err becomes "unknown error" so you never ship an empty message to the client. The original
// stays reachable for chain inspection, out of sight of the response body.
func NewErrorResponse(err error, optionalStatusCode ...int) *ErrorResponse {
	statusCode := http.StatusBadRequest
	if len(optionalStatusCode) > 0 && optionalStatusCode[0] > 0 {
		statusCode = optionalStatusCode[0]
	}

	if err == nil {
		return &ErrorResponse{Err: ErrUnknown.Error(), StatusCode: statusCode}
	}

	return &ErrorResponse{Err: err.Error(), StatusCode: statusCode, err: err}
}

// MatrixError is the shape Matrix's Client-Server API speaks errors in: a machine code
// (M_FORBIDDEN, M_UNKNOWN, and friends) and a human message. Implements error.
type MatrixError struct {
	Code string `json:"errcode"`
	Err  string `json:"error"`
}

// Error returns the human-readable message. Implements error.
func (e MatrixError) Error() string {
	return e.Err
}

// NewMatrixError builds one from a standard Matrix errcode and a message.
func NewMatrixError(code, err string) *MatrixError {
	return &MatrixError{Code: code, Err: err}
}

// MatrixErrorFrom decodes a MatrixError out of a JSON body. Nil reader, nil result. When the
// body isn't the JSON you were promised, you get an M_UNKNOWN carrying the raw bytes instead of
// a silent nil, because a server lying about its own error shape is exactly the moment you need
// to see what it actually sent. The read is capped at maxMatrixErrorBody, so a server can't hand
// you a gigabyte labeled "error" and watch you swallow it whole.
func MatrixErrorFrom(r io.Reader) *MatrixError {
	if r == nil {
		return nil
	}

	var matrixErr *MatrixError
	data, _ := io.ReadAll(io.LimitReader(r, maxMatrixErrorBody)) //nolint:errcheck // ignore error as we will return nil
	if err := json.Unmarshal(data, &matrixErr); err != nil {
		return NewMatrixError("M_UNKNOWN", fmt.Sprintf("failed to decode error response %q: %v", string(data), err))
	}

	return matrixErr
}

// IsContextError reports whether err is a context cancellation or a blown deadline, the two you
// usually want to treat alike and not page anyone over.
func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
