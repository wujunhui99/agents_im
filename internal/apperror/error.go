package apperror

import (
	"errors"
	"net/http"
)

type Code string

const (
	CodeOK              Code = "OK"
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeUnauthenticated Code = "UNAUTHENTICATED"
	CodeNotFound        Code = "NOT_FOUND"
	CodeAlreadyExists   Code = "ALREADY_EXISTS"
	CodeInternal        Code = "INTERNAL"
)

type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func InvalidArgument(message string) *Error {
	return New(CodeInvalidArgument, message)
}

func Unauthenticated(message string) *Error {
	return New(CodeUnauthenticated, message)
}

func NotFound(message string) *Error {
	return New(CodeNotFound, message)
}

func AlreadyExists(message string) *Error {
	return New(CodeAlreadyExists, message)
}

func Internal(message string) *Error {
	return New(CodeInternal, message)
}

func From(err error) *Error {
	if err == nil {
		return nil
	}

	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}

	return Internal("internal server error")
}

func HTTPStatus(err error) int {
	appErr := From(err)
	if appErr == nil {
		return http.StatusOK
	}

	switch appErr.Code {
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAlreadyExists:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
