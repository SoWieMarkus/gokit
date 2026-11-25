// Package apperr holds transport independent business errors.
//
// Services return these errors and the HTTP layer (package httpx) translates
// their Kind into a status code, so no service ever has to know about HTTP.
// Every application defines its concrete errors on top of this package:
//
//	var ErrPostNotFound = apperr.New(apperr.KindNotFound, "post_not_found", "post does not exist")
package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies an error for the transport layer.
type Kind int

const (
	KindInvalid Kind = iota + 1
	KindNotFound
	KindConflict
	KindUnprocessable
	KindUnauthenticated
	KindForbidden
	KindTooManyRequests
	KindBadGateway
	KindUnavailable
)

// Error is a business rule violation with a stable machine readable code.
type Error struct {
	Kind    Kind
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// New creates an error of the given kind.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// Invalid creates a validation error for input that breaks a business rule.
func Invalid(code, message string) *Error {
	return New(KindInvalid, code, message)
}

// Unprocessable creates an error for well formed input the server cannot act on.
func Unprocessable(code, message string) *Error {
	return New(KindUnprocessable, code, message)
}

// KindOf returns the Kind of the first *Error in err's chain, or 0 if there is
// none.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return 0
}
