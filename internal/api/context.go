package api

import (
	"context"
	"net/http"

	"github.com/MunifTanjim/pushport/internal/id"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	callerIsAdminKey
)

// WithRequestID assigns each request a server-generated id, echoed as the
// X-Request-Id header. Wrap the top-level mux so SendData/SendError can surface it.
func WithRequestID(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-Id", id)
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func RequestID(r *http.Request) string {
	if id, ok := r.Context().Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func newRequestID() string { return id.New() }

// CallerIsAdmin reports whether the request used the admin token rather
// than an app token. Set by requireAppAuth.
func CallerIsAdmin(r *http.Request) bool {
	isAdmin, ok := r.Context().Value(callerIsAdminKey).(bool)
	return ok && isAdmin
}
