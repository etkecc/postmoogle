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

// ErrUnknown represents an unknown error
var ErrUnknown = errors.New("unknown error")

// AggregateError represents an aggregate of multiple errors
type AggregateError struct {
	mu     sync.RWMutex
	Errors []error
}

// Error returns the aggregated error messages
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

// Unwrap returns the underlying errors
// WARNING: This method returns original slice. Do not modify it.
func (a *AggregateError) Unwrap() []error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.Errors) == 0 {
		return nil
	}

	return a.Errors
}

// Is checks if the target error is in the aggregate
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

// As checks if the target error type is in the aggregate
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

// Join adds errors to the aggregate and returns nil if no errors were added
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

// NewAggregateError creates a new aggregate error
func NewAggregateError(errs ...error) *AggregateError {
	agg := &AggregateError{}
	return agg.Join(errs...)
}

// ErrorResponse represents an error response
//
//nolint:errname // ErrorResponse is a valid name
type ErrorResponse struct {
	StatusCode int    `json:"-"`     // HTTP status code, optional, not serialized
	Err        string `json:"error"` // Error message
	err        error  `json:"-"`     // underlying error, optional, not serialized
}

// Error returns the error message
func (e ErrorResponse) Error() string {
	return e.Err
}

// Unwrap returns the underlying error
func (e ErrorResponse) Unwrap() error {
	return e.err
}

// NewErrorResponse creates a new error response
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

// MatrixError represents an error response from the Matrix API
type MatrixError struct {
	Code string `json:"errcode"`
	Err  string `json:"error"`
}

// Error returns the error message
func (e MatrixError) Error() string {
	return e.Err
}

// NewMatrixError creates a new Matrix error
func NewMatrixError(code, err string) *MatrixError {
	return &MatrixError{Code: code, Err: err}
}

// MatrixErrorFrom creates a new Matrix error from io.Reader
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
