package http

import (
	"fmt"
	httpstd "net/http"
)

// APIError represents a structured API error that can be returned to clients.
// It implements the error interface and carries an HTTP status code, a
// machine-readable code, and a human-readable message. Validation errors
// include field-level Details.
type APIError struct {
	Status  int          `json:"-"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// WithMessage returns a copy of the error with a custom message. The original
// sentinel is never mutated. Preserves any existing Details.
func (e *APIError) WithMessage(msg string) *APIError {
	return &APIError{Status: e.Status, Code: e.Code, Message: msg, Details: e.Details}
}

// WithDetails returns a copy of the error with field-level validation details.
// The original sentinel is never mutated.
func (e *APIError) WithDetails(details []FieldError) *APIError {
	return &APIError{Status: e.Status, Code: e.Code, Message: e.Message, Details: details}
}

// Common API errors. Use WithMessage to customize the message for a specific
// context while preserving the status code and error code.
var (
	ErrBadRequest          = &APIError{Status: httpstd.StatusBadRequest, Code: "bad_request", Message: "Bad request"}
	ErrUnauthorized        = &APIError{Status: httpstd.StatusUnauthorized, Code: "unauthorized", Message: "Unauthorized"}
	ErrForbidden           = &APIError{Status: httpstd.StatusForbidden, Code: "forbidden", Message: "Forbidden"}
	ErrNotFound            = &APIError{Status: httpstd.StatusNotFound, Code: "not_found", Message: "Not found"}
	ErrConflict            = &APIError{Status: httpstd.StatusConflict, Code: "conflict", Message: "Conflict"}
	ErrUnprocessableEntity = &APIError{Status: httpstd.StatusUnprocessableEntity, Code: "unprocessable_entity", Message: "Unprocessable entity"}
	ErrTooManyRequests     = &APIError{Status: httpstd.StatusTooManyRequests, Code: "too_many_requests", Message: "Too many requests"}
	ErrInternal            = &APIError{Status: httpstd.StatusInternalServerError, Code: "internal_error", Message: "Internal server error"}
	ErrValidation          = &APIError{Status: httpstd.StatusUnprocessableEntity, Code: "validation_error", Message: "Validation failed"}
)

// NewError creates a custom APIError with the given status, code, and message.
func NewError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}
