package api

import (
	"context"
	"net/http"

	"github.com/annmary-ann/recurin/internal/platform/id"
)

type requestIDKey struct{}

// withRequestID gives every request a fresh req_ ID, returned in the
// Request-Id header and in every error body. Client-supplied IDs are ignored
// so the value is always unique and safe to log.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := id.New[id.Request]().String()
		w.Header().Set("Request-Id", rid)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, rid)))
	})
}

func requestIDFrom(ctx context.Context) string {
	rid, _ := ctx.Value(requestIDKey{}).(string)
	return rid
}

// withNoStore stops caches from keeping API responses, which carry account data.
func withNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
