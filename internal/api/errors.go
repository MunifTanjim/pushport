package api

import "net/http"

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`

	cause      error // logged server-side, never serialized
	retryAfter string
}

func (e *APIError) Error() string {
	if e.cause != nil {
		return e.Code + ": " + e.Message + ": " + e.cause.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *APIError) Unwrap() error { return e.cause }

func (e *APIError) WithMessage(msg string) *APIError  { e.Message = msg; return e }
func (e *APIError) WithCause(err error) *APIError     { e.cause = err; return e }
func (e *APIError) WithRetryAfter(v string) *APIError { e.retryAfter = v; return e }

func newAPIError(code string, status int, msg string) *APIError {
	return &APIError{Code: code, Message: msg, StatusCode: status}
}

func ErrorBadRequest() *APIError {
	return newAPIError("bad_request", http.StatusBadRequest, "bad request")
}
func ErrorUnauthorized() *APIError {
	return newAPIError("unauthorized", http.StatusUnauthorized, "unauthorized")
}
func ErrorForbidden() *APIError {
	return newAPIError("forbidden", http.StatusForbidden, "forbidden")
}
func ErrorNotFound() *APIError {
	return newAPIError("not_found", http.StatusNotFound, "not found")
}
func ErrorUnsupportedMediaType() *APIError {
	return newAPIError("unsupported_media_type", http.StatusUnsupportedMediaType, "unsupported media type")
}
func ErrorPayloadTooLarge() *APIError {
	return newAPIError("payload_too_large", http.StatusRequestEntityTooLarge, "payload too large")
}
func ErrorTooManyRequests() *APIError {
	return newAPIError("too_many_requests", http.StatusTooManyRequests, "rate limited")
}
func ErrorConflict() *APIError {
	return newAPIError("conflict", http.StatusConflict, "conflict")
}
func ErrorGone() *APIError {
	return newAPIError("gone", http.StatusGone, "gone")
}
func ErrorInternalServerError() *APIError {
	return newAPIError("internal_server_error", http.StatusInternalServerError, "internal error")
}
func ErrorBadGateway() *APIError {
	return newAPIError("bad_gateway", http.StatusBadGateway, "bad gateway")
}
func ErrorServiceUnavailable() *APIError {
	return newAPIError("service_unavailable", http.StatusServiceUnavailable, "service unavailable")
}
