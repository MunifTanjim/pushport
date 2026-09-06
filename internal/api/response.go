package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// response is the uniform envelope for every JSON response: a top-level
// request_id plus exactly one of data or error.
type response struct {
	RequestID string    `json:"request_id,omitempty"`
	Data      any       `json:"data,omitempty"`
	Error     *APIError `json:"error,omitempty"`
}

func (res response) send(w http.ResponseWriter, r *http.Request, status int) {
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	res.RequestID = RequestID(r)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(res); err != nil {
		slog.Error("failed to encode response", "request_id", res.RequestID, "err", err)
	}
}

// SendData writes a success envelope: {"request_id":..., "data":...}. A 204
// status writes no body (the id rides on the X-Request-Id header).
func SendData(w http.ResponseWriter, r *http.Request, status int, data any) {
	response{Data: data}.send(w, r, status)
}

// SendError writes an error envelope. Any error is coerced to *APIError (a plain
// error becomes a 500). A Retry-After hint on the error is emitted as a header,
// and 5xx errors are logged with the request id and cause.
func SendError(w http.ResponseWriter, r *http.Request, err error) {
	var e *APIError
	if !errors.As(err, &e) {
		e = ErrorInternalServerError().WithCause(err)
	}
	if e.StatusCode == 0 {
		e.StatusCode = http.StatusInternalServerError
	}
	if e.retryAfter != "" {
		w.Header().Set("Retry-After", e.retryAfter)
	}
	if e.StatusCode >= http.StatusInternalServerError {
		slog.Error("request error",
			"request_id", RequestID(r),
			"method", r.Method,
			"path", r.URL.Path,
			"code", e.Code,
			"status", e.StatusCode,
			"cause", e.cause,
		)
	}
	response{Error: e}.send(w, r, e.StatusCode)
}

const defaultMaxRequestBodyBytes = 1 << 20 // 1 MiB

// ReadRequestBodyJSON enforces a JSON content type and decodes the request body
// into payload (which must be a pointer). The body is capped with
// http.MaxBytesReader — at defaultMaxRequestBodyBytes, or the optional maxBytes
// override — so an oversized body can't exhaust memory. It returns a typed
// *APIError on failure: 415 for a non-JSON content type, 413 for an oversized
// body, 400 for a missing or malformed body.
func ReadRequestBodyJSON[T any](w http.ResponseWriter, r *http.Request, payload T, maxBytes ...int64) error {
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		return ErrorUnsupportedMediaType()
	}
	limit := int64(defaultMaxRequestBodyBytes)
	if len(maxBytes) > 0 && maxBytes[0] > 0 {
		limit = maxBytes[0]
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	err := json.NewDecoder(r.Body).Decode(payload)
	if err == nil {
		return nil
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return ErrorPayloadTooLarge()
	}
	if err == io.EOF {
		return ErrorBadRequest().WithMessage("missing body").WithCause(err)
	}
	return ErrorBadRequest().WithMessage("invalid body").WithCause(err)
}
