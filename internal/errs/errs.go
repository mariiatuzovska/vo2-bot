package errs

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Error struct {
	Status int
	Detail string
	Extra  map[string]any
}

func (e *Error) Error() string {
	label := Label(e.Status)
	if e.Detail != "" {
		return fmt.Sprintf("%d %s: %s", e.Status, label, e.Detail)
	}
	return fmt.Sprintf("%d %s", e.Status, label)
}

func (e *Error) With(key string, value any) *Error {
	if e.Extra == nil {
		e.Extra = make(map[string]any, 2)
	}
	e.Extra[key] = value
	return e
}

// Code returns the HTTP status carried by err, unwrapping as necessary.
// Returns 0 if err is not (and does not wrap) an *Error.
func Code(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Status
	}
	return 0
}

// Label maps an HTTP status to a lowercase snake_case identifier derived
// from http.StatusText. Example: 422 → "unprocessable_entity".
func Label(status int) string {
	text := http.StatusText(status)
	if text == "" {
		return "error"
	}
	return strings.ReplaceAll(strings.ToLower(text), " ", "_")
}

func newf(status int, format string, args []any) *Error {
	detail := ""
	if format != "" {
		detail = fmt.Sprintf(format, args...)
	}
	return &Error{Status: status, Detail: detail}
}

func NewBadRequest(format string, args ...any) *Error {
	return newf(http.StatusBadRequest, format, args)
}

func NewUnauthorized(format string, args ...any) *Error {
	return newf(http.StatusUnauthorized, format, args)
}

func NewForbidden(format string, args ...any) *Error {
	return newf(http.StatusForbidden, format, args)
}

func NewNotFound(format string, args ...any) *Error {
	return newf(http.StatusNotFound, format, args)
}

func NewConflict(format string, args ...any) *Error {
	return newf(http.StatusConflict, format, args)
}

func NewUnprocessable(format string, args ...any) *Error {
	return newf(http.StatusUnprocessableEntity, format, args)
}

func NewInternal(format string, args ...any) *Error {
	return newf(http.StatusInternalServerError, format, args)
}
