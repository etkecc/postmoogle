package kit

import (
	"encoding/json"
	"fmt"
	"io"
)

// ErrorResponse represents an error response
//
//nolint:errname // ErrorResponse is a valid name
type ErrorResponse struct {
	Err string `json:"error"`
}

// Error returns the error message
func (e ErrorResponse) Error() string {
	return e.Err
}

// NewErrorResponse creates a new error response
func NewErrorResponse(err error) *ErrorResponse {
	if err == nil {
		return &ErrorResponse{Err: "unknown error"}
	}

	return &ErrorResponse{Err: err.Error()}
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
