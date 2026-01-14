package nepse

import (
	"fmt"
	"net/http"
)

// NepseError is the error type returned by all NEPSE API operations.
// Use errors.Is with sentinel errors (e.g., ErrNotFound) to check error categories,
// or errors.As to extract the full error details.
type NepseError struct {
	Type    ErrorType // Category of error
	Message string    // Human-readable description
	Err     error     // Underlying error, if any
}

// ErrorType categorizes NEPSE errors for programmatic handling.
type ErrorType string

const (
	ErrorTypeInvalidClientRequest  ErrorType = "invalid_client_request"
	ErrorTypeInvalidServerResponse ErrorType = "invalid_server_response"
	ErrorTypeTokenExpired          ErrorType = "token_expired"
	ErrorTypeNetworkError          ErrorType = "network_error"
	ErrorTypeUnauthorized          ErrorType = "unauthorized"
	ErrorTypeNotFound              ErrorType = "not_found"
	ErrorTypeRateLimit             ErrorType = "rate_limit"
	ErrorTypeInternal              ErrorType = "internal_error"
)

// Sentinel errors for use with [errors.Is].
// Matching is based on ErrorType, not identity.
var (
	ErrInvalidClientRequest  = &NepseError{Type: ErrorTypeInvalidClientRequest}
	ErrInvalidServerResponse = &NepseError{Type: ErrorTypeInvalidServerResponse}
	ErrTokenExpired          = &NepseError{Type: ErrorTypeTokenExpired}
	ErrNetworkError          = &NepseError{Type: ErrorTypeNetworkError}
	ErrUnauthorized          = &NepseError{Type: ErrorTypeUnauthorized}
	ErrNotFound              = &NepseError{Type: ErrorTypeNotFound}
	ErrRateLimit             = &NepseError{Type: ErrorTypeRateLimit}
	ErrInternal              = &NepseError{Type: ErrorTypeInternal}
)

// Error implements the error interface.
func (e *NepseError) Error() string {
	if e.Message == "" && e.Err == nil {
		return fmt.Sprintf("nepse: %s", e.Type)
	}
	if e.Err != nil {
		return fmt.Sprintf("nepse: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("nepse: %s", e.Message)
}

// Unwrap returns the underlying error.
func (e *NepseError) Unwrap() error {
	return e.Err
}

// Is reports whether e matches target by comparing ErrorType fields.
func (e *NepseError) Is(target error) bool {
	t, ok := target.(*NepseError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// NewNepseError constructs an error with the given type, message, and optional wrapped error.
func NewNepseError(errorType ErrorType, message string, err error) *NepseError {
	return &NepseError{
		Type:    errorType,
		Message: message,
		Err:     err,
	}
}

// NewInvalidClientRequestError returns an error for malformed requests (HTTP 400).
func NewInvalidClientRequestError(message string) *NepseError {
	return NewNepseError(ErrorTypeInvalidClientRequest, message, nil)
}

// NewInvalidServerResponseError returns an error for unexpected server responses (5xx).
func NewInvalidServerResponseError(message string) *NepseError {
	return NewNepseError(ErrorTypeInvalidServerResponse, message, nil)
}

// NewTokenExpiredError returns an error indicating the access token has expired.
func NewTokenExpiredError() *NepseError {
	return NewNepseError(ErrorTypeTokenExpired, "access token expired", nil)
}

// NewNetworkError wraps a low-level network failure (DNS, connection, timeout).
func NewNetworkError(err error) *NepseError {
	return NewNepseError(ErrorTypeNetworkError, "network request failed", err)
}

// NewUnauthorizedError returns an error for forbidden access (HTTP 403).
// If message is empty, a default message is used.
func NewUnauthorizedError(message string) *NepseError {
	if message == "" {
		message = "access forbidden"
	}
	return NewNepseError(ErrorTypeUnauthorized, message, nil)
}

// NewNotFoundError returns an error for missing resources (HTTP 404).
// If resource is empty, a generic message is used.
func NewNotFoundError(resource string) *NepseError {
	message := "resource not found"
	if resource != "" {
		message = resource + " not found"
	}
	return NewNepseError(ErrorTypeNotFound, message, nil)
}

// NewRateLimitError returns an error when API rate limits are exceeded (HTTP 429).
func NewRateLimitError() *NepseError {
	return NewNepseError(ErrorTypeRateLimit, "rate limit exceeded", nil)
}

// NewInternalError wraps unexpected internal failures.
func NewInternalError(message string, err error) *NepseError {
	return NewNepseError(ErrorTypeInternal, message, err)
}

// MapHTTPStatusToError converts an HTTP status code to the appropriate NepseError.
func MapHTTPStatusToError(statusCode int, message string) *NepseError {
	switch statusCode {
	case http.StatusBadRequest:
		return NewInvalidClientRequestError(message)
	case http.StatusUnauthorized:
		return NewTokenExpiredError()
	case http.StatusForbidden:
		return NewUnauthorizedError(message)
	case http.StatusNotFound:
		return NewNotFoundError("resource")
	case http.StatusTooManyRequests:
		return NewRateLimitError()
	case http.StatusBadGateway:
		return NewInvalidServerResponseError(message)
	case http.StatusServiceUnavailable:
		return NewInvalidServerResponseError("service unavailable")
	case http.StatusGatewayTimeout:
		return NewInvalidServerResponseError("gateway timeout")
	default:
		if statusCode >= 500 {
			return NewInvalidServerResponseError(fmt.Sprintf("server error: %d", statusCode))
		}
		return NewInternalError(fmt.Sprintf("unexpected status code: %d", statusCode), nil)
	}
}

// IsRetryable reports whether the operation that caused this error may succeed on retry.
// Token expiration, network errors, server errors, and rate limits are considered retryable.
func (e *NepseError) IsRetryable() bool {
	switch e.Type {
	case ErrorTypeTokenExpired, ErrorTypeNetworkError, ErrorTypeInvalidServerResponse, ErrorTypeRateLimit:
		return true
	default:
		return false
	}
}
