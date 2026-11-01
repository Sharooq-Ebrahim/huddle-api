package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	CodeValidation   ErrorCode = "VALIDATION_ERROR"
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"
	CodeForbidden    ErrorCode = "FORBIDDEN"
	CodeNotFound     ErrorCode = "NOT_FOUND"
	CodeConflict     ErrorCode = "CONFLICT"
	CodeRateLimited  ErrorCode = "RATE_LIMITED"
	CodeInternal     ErrorCode = "INTERNAL_ERROR"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}


type AppError struct {
	Code       ErrorCode    `json:"code"`
	Message    string       `json:"message"`
	Details    []FieldError `json:"details,omitempty"`
	HTTPStatus int          `json:"-"`
	Err        error        `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}



func Validation(message string, details []FieldError) *AppError {
	return &AppError{
		Code:       CodeValidation,
		Message:    message,
		Details:    details,
		HTTPStatus: http.StatusBadRequest,
	}
}

func Unauthorized(message string) *AppError {
	return &AppError{
		Code:       CodeUnauthorized,
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
	}
}

func Forbidden(message string) *AppError {
	return &AppError{
		Code:       CodeForbidden,
		Message:    message,
		HTTPStatus: http.StatusForbidden,
	}
}

func NotFound(message string) *AppError {
	return &AppError{
		Code:       CodeNotFound,
		Message:    message,
		HTTPStatus: http.StatusNotFound,
	}
}

func Conflict(message string) *AppError {
	return &AppError{
		Code:       CodeConflict,
		Message:    message,
		HTTPStatus: http.StatusConflict,
	}
}

func RateLimited(message string) *AppError {
	return &AppError{
		Code:       CodeRateLimited,
		Message:    message,
		HTTPStatus: http.StatusTooManyRequests,
	}
}

func Internal(err error) *AppError {
	return &AppError{
		Code:       CodeInternal,
		Message:    "An unexpected error occurred",
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
	}
}


type ResponseEnvelope struct {
	Error *AppError `json:"error"`
}

func ToResponse(err error) (int, ResponseEnvelope) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus, ResponseEnvelope{Error: appErr}
	}

	return http.StatusInternalServerError, ResponseEnvelope{
		Error: Internal(err),
	}
}

