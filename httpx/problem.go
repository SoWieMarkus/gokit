package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/SoWieMarkus/gokit/apperr"
)

// Problem is an RFC 9457 problem details response.
type Problem struct {
	Type     string `json:"type"     example:"about:blank"`
	Title    string `json:"title"    example:"Conflict"`
	Status   int    `json:"status"   example:"409"`
	Detail   string `json:"detail"   example:"this name is already taken"`
	Code     string `json:"code"     example:"name_taken"`
	Instance string `json:"instance" example:"/api/v1/items"`
	// RequestID correlates the response with server logs.
	RequestID string       `json:"requestId,omitempty" binding:"optional"`
	Errors    []FieldError `json:"errors,omitempty" binding:"optional"`
}

type FieldError struct {
	Field   string `json:"field"   example:"title"`
	Message string `json:"message" example:"is required"`
}

// requestError is a problem with the request itself (bad JSON, failed
// validation, malformed path parameter) detected by the HTTP layer.
type requestError struct {
	status int
	code   string
	detail string
	fields []FieldError
}

func (e *requestError) Error() string {
	return e.code + ": " + e.detail
}

// BadRequest creates a 400 problem for invalid query or form input.
func BadRequest(code, detail string) error {
	return &requestError{status: http.StatusBadRequest, code: code, detail: detail}
}

// errorMappers translate foreign errors into apperr errors, see MapError.
var errorMappers []func(error) *apperr.Error

// MapError teaches Error to translate errors that are not *apperr.Error, for
// example the error types of a library that must not depend on gokit. fn
// returns nil for errors it does not know. Call it from an init function:
// mappers must not be added while requests are served.
func MapError(fn func(error) *apperr.Error) {
	errorMappers = append(errorMappers, fn)
}

// StatusOf maps an error kind to its HTTP status. Unknown kinds are 500.
func StatusOf(kind apperr.Kind) int {
	switch kind {
	case apperr.KindInvalid:
		return http.StatusBadRequest
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindUnprocessable:
		return http.StatusUnprocessableEntity
	case apperr.KindUnauthenticated:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindTooManyRequests:
		return http.StatusTooManyRequests
	case apperr.KindBadGateway:
		return http.StatusBadGateway
	case apperr.KindUnavailable:
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

// Error translates any error into a problem response. Business errors from
// package apperr map to 4xx by their Kind, unknown errors are logged and
// hidden behind a generic 500 so internals never leak.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	if reqErr, ok := errors.AsType[*requestError](err); ok {
		WriteProblem(w, r, reqErr.status, reqErr.code, reqErr.detail, reqErr.fields)
		return
	}

	for _, mapErr := range errorMappers {
		if appErr := mapErr(err); appErr != nil {
			WriteProblem(w, r, StatusOf(appErr.Kind), appErr.Code, appErr.Message, nil)
			return
		}
	}

	if appErr, ok := errors.AsType[*apperr.Error](err); ok {
		WriteProblem(w, r, StatusOf(appErr.Kind), appErr.Code, appErr.Message, nil)
		return
	}

	slog.ErrorContext(r.Context(), "unhandled error",
		"error", err,
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", middleware.GetReqID(r.Context()),
	)
	WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "something went wrong on our side", nil)
}

// WriteProblem writes a problem response with the given status and code.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string, fields []FieldError) {
	w.Header().Set("Content-Type", "application/problem+json")
	encode(w, status, Problem{
		Type:      "about:blank",
		Title:     http.StatusText(status),
		Status:    status,
		Detail:    detail,
		Code:      code,
		Instance:  r.URL.Path,
		RequestID: middleware.GetReqID(r.Context()),
		Errors:    fields,
	})
}

// NotFound is a chi NotFound handler that answers with a problem response.
func NotFound(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, http.StatusNotFound, "route_not_found", "this route does not exist", nil)
}

// MethodNotAllowed is a chi MethodNotAllowed handler that answers with a
// problem response.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed on this route", nil)
}
