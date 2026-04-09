package kit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// ErrUnknown is a sentinel error representing an unknown or unclassified error.
// It can be used as a fallback when no more specific error type is available.
var ErrUnknown = errors.New("unknown error")

// AggregateError is a thread-safe container for multiple errors that occurred together.
// It is commonly used to collect errors from concurrent operations or validation checks
// and report them all at once. The zero value is ready to use.
//
// AggregateError is safe for concurrent use; all methods are protected by an internal mutex.
// The Errors field holds the underlying error slice and can be read via Unwrap, but callers
// must not modify it directly.
type AggregateError struct {
	mu     sync.RWMutex
	Errors []error
}

// Error returns the aggregated error messages as a single string by joining all errors
// with a semicolon and space separator. It returns an empty string if no errors are present.
// This method implements the error interface.
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

// Unwrap returns the underlying error slice. The returned slice is the original,
// not a copy; callers must not modify it. Returns nil if the aggregate contains no errors.
// This method is used by the errors package to unwrap error chains.
func (a *AggregateError) Unwrap() []error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.Errors) == 0 {
		return nil
	}

	return a.Errors
}

// Is reports whether the target error matches any error in the aggregate by recursively
// checking each error with errors.Is. This allows callers to check for specific error types
// within the aggregate using standard error chain semantics.
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

// As reports whether the aggregate or any of its underlying errors matches the target type
// by recursively checking each error with errors.As. This allows type assertions on wrapped
// errors to work correctly across the aggregate.
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

// Join adds non-nil errors to the aggregate. Nil errors are silently skipped.
// It returns the receiver if any errors are present after filtering, or nil if the aggregate
// is empty after the operation. This allows callers to idiomatically chain:
//
//	if err := agg.Join(e1, e2, e3); err != nil { ... }
//
// The nil return when the aggregate is still empty is intentional—it prevents accidental
// propagation of empty aggregates.
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

// NewAggregateError creates a new AggregateError and adds the provided errors to it.
// Nil errors are silently skipped. It returns nil if no non-nil errors were provided,
// allowing idiomatically correct error handling:
//
//	if err := NewAggregateError(e1, e2); err != nil { ... }
func NewAggregateError(errs ...error) *AggregateError {
	agg := &AggregateError{}
	return agg.Join(errs...)
}

// ErrorResponse represents a structured HTTP error response. It combines an HTTP status code
// with machine-serializable and human-readable error details.
//
// The Err field is the JSON-serialized error message (key "error" in JSON output).
// StatusCode defaults to http.StatusBadRequest (400) if not explicitly set and is never
// serialized to JSON. The underlying error is preserved for errors.Is/As chain semantics
// via the Unwrap method but is not exposed in JSON output.
//
// ErrorResponse implements the error interface and the errors.Wrapper interface.
//
//nolint:errname // ErrorResponse is a valid name
type ErrorResponse struct {
	StatusCode int    `json:"-"`     // HTTP status code, optional, not serialized
	Err        string `json:"error"` // Error message
	err        error  `json:"-"`     // underlying error, optional, not serialized
}

// Error returns the error message string, implementing the error interface.
func (e ErrorResponse) Error() string {
	return e.Err
}

// Unwrap returns the underlying error, if any. This enables ErrorResponse to participate
// in error chains inspected by errors.Is and errors.As.
func (e ErrorResponse) Unwrap() error {
	return e.err
}

// NewErrorResponse creates a new ErrorResponse from an error. The StatusCode defaults to
// http.StatusBadRequest (400); an optional status code may be provided as a variadic argument
// and is used if positive (> 0), otherwise the default is applied. If err is nil,
// Err is set to "unknown error". The underlying error is preserved for chain inspection
// but is not serialized to JSON.
func NewErrorResponse(err error, optionalStatusCode ...int) *ErrorResponse {
	statusCode := http.StatusBadRequest
	if len(optionalStatusCode) > 0 && optionalStatusCode[0] > 0 {
		statusCode = optionalStatusCode[0]
	}

	if err == nil {
		return &ErrorResponse{Err: "unknown error", StatusCode: statusCode}
	}

	return &ErrorResponse{Err: err.Error(), StatusCode: statusCode, err: err}
}

// MatrixError represents an error response from the Matrix Client-Server API.
// It follows the Matrix specification error format with two fields:
// Code is the machine-readable error code (e.g., "M_FORBIDDEN", "M_UNKNOWN")
// and Err is the human-readable error message.
//
// MatrixError implements the error interface.
type MatrixError struct {
	Code string `json:"errcode"`
	Err  string `json:"error"`
}

// Error returns the human-readable error message, implementing the error interface.
func (e MatrixError) Error() string {
	return e.Err
}

// NewMatrixError creates a new MatrixError with the given error code and message.
// The code should be a standard Matrix error code like "M_FORBIDDEN" or "M_UNKNOWN".
func NewMatrixError(code, err string) *MatrixError {
	return &MatrixError{Code: code, Err: err}
}

// MatrixErrorFrom decodes a MatrixError from an io.Reader containing JSON data.
// It returns nil if r is nil. If JSON decoding fails, it returns a MatrixError
// with code "M_UNKNOWN" and a descriptive error message that includes the raw body,
// allowing debugging of malformed responses.
func MatrixErrorFrom(r io.Reader) *MatrixError {
	if r == nil {
		return nil
	}

	var matrixErr *MatrixError
	data, _ := io.ReadAll(r) //nolint:errcheck // ignore error as we will return nil
	if err := json.Unmarshal(data, &matrixErr); err != nil {
		return NewMatrixError("M_UNKNOWN", fmt.Sprintf("failed to decode error response %q: %v", string(data), err))
	}

	return matrixErr
}
