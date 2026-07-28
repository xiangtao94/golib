// Package errors defines transport-neutral public service errors.
package errors

import (
	stderrors "errors"
	"net/http"
	"strings"
)

// Error keeps the public contract separate from its private cause. Values are
// immutable: every With method returns a copy.
type Error struct {
	code       string
	reason     string
	message    string
	httpStatus int
	retryable  bool
	details    map[string]any
	cause      error
}

func New(code, message string, httpStatus int) *Error {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "INTERNAL"
	}
	if httpStatus < 400 || httpStatus > 599 {
		httpStatus = http.StatusInternalServerError
	}
	return &Error{
		code:       code,
		reason:     code,
		message:    message,
		httpStatus: httpStatus,
	}
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	return err.message
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

func (err *Error) Code() string {
	if err == nil {
		return ""
	}
	return err.code
}

func (err *Error) Reason() string {
	if err == nil {
		return ""
	}
	return err.reason
}

func (err *Error) Message() string {
	if err == nil {
		return ""
	}
	return err.message
}

func (err *Error) HTTPStatus() int {
	if err == nil {
		return http.StatusInternalServerError
	}
	return err.httpStatus
}

func (err *Error) Retryable() bool {
	return err != nil && err.retryable
}

func (err *Error) Details() map[string]any {
	if err == nil {
		return nil
	}
	return cloneDetails(err.details)
}

func (err *Error) WithReason(reason string) *Error {
	clone := err.clone()
	if value := strings.TrimSpace(reason); value != "" {
		clone.reason = value
	}
	return clone
}

func (err *Error) WithRetryable(retryable bool) *Error {
	clone := err.clone()
	clone.retryable = retryable
	return clone
}

func (err *Error) WithDetails(details map[string]any) *Error {
	clone := err.clone()
	clone.details = cloneDetails(details)
	return clone
}

func (err *Error) Wrap(cause error) *Error {
	clone := err.clone()
	clone.cause = cause
	return clone
}

func (err *Error) clone() *Error {
	if err == nil {
		return New("INTERNAL", "internal server error", http.StatusInternalServerError)
	}
	clone := *err
	clone.details = cloneDetails(err.details)
	return &clone
}

func cloneDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	clone := make(map[string]any, len(details))
	for key, value := range details {
		clone[key] = value
	}
	return clone
}

// From extracts a public Error or returns a safe internal fallback that wraps
// the original cause for logs and errors.Is/errors.As.
func From(err error) *Error {
	var public *Error
	if stderrors.As(err, &public) {
		return public
	}
	return ErrInternal.Wrap(err)
}

var (
	ErrInternal         = New("INTERNAL", "internal server error", http.StatusInternalServerError)
	ErrInvalidArgument  = New("INVALID_ARGUMENT", "invalid request", http.StatusBadRequest)
	ErrUnauthenticated  = New("UNAUTHENTICATED", "authentication required", http.StatusUnauthorized)
	ErrPermissionDenied = New("PERMISSION_DENIED", "permission denied", http.StatusForbidden)
	ErrNotFound         = New("NOT_FOUND", "resource not found", http.StatusNotFound)
	ErrConflict         = New("CONFLICT", "resource conflict", http.StatusConflict)
	ErrDeadlineExceeded = New("DEADLINE_EXCEEDED", "request deadline exceeded", http.StatusGatewayTimeout).
				WithReason("TIMEOUT").
				WithRetryable(true)
	ErrResourceExhausted = New("RESOURCE_EXHAUSTED", "request rate limit exceeded", http.StatusTooManyRequests).
				WithReason("RATE_LIMITED").
				WithRetryable(true)
)
