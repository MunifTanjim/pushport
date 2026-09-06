package api

import "net/http"

// WithCORS wraps next with cross-origin handling for the admin console,
// which is hosted on a different origin (Cloudflare Pages) than the API. Only
// origins in allowed receive CORS headers; the matched Origin is echoed back
// (never "*"). Auth is a Bearer header (no cookies), so
// Access-Control-Allow-Credentials is intentionally never set.
func WithCORS(allowed []string, next http.Handler) http.Handler {
	if len(allowed) == 0 {
		return next
	}
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The CORS headers depend on the request Origin, so any cache must key on
		// it — set Vary unconditionally, even when the origin isn't allowlisted.
		w.Header().Add("Vary", "Origin")

		origin := r.Header.Get("Origin")
		_, ok := set[origin]
		if ok {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "600")
		}

		if ok && r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
