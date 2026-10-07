package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
)

const requestIDHeader = "X-Request-Id"

// RequestID assigns each request an ID, reusing a well-formed incoming
// X-Request-Id so IDs can be traced across services, and echoes it in the
// response. The ID is stored under chi's key so chimw.GetReqID works. Unlike
// chi's own middleware it does not embed the server hostname, which would
// otherwise be exposed to clients in error bodies.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if !validRequestID(id) {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), chimw.RequestIDKey, id)))
	})
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// validRequestID accepts 1-64 characters from [A-Za-z0-9._-], which keeps
// client-supplied IDs safe to log and echo.
func validRequestID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}
