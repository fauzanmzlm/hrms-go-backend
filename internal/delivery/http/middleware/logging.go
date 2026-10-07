package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// requestInfo lets inner middleware report details back to RequestLogger,
// which only sees the outer request's context.
type requestInfo struct {
	userID int64
}

type requestInfoKey struct{}

// RequestLogger logs one structured line per request. 5xx responses log at
// ERROR and 4xx at WARN.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			info := &requestInfo{}
			r = r.WithContext(context.WithValue(r.Context(), requestInfoKey{}, info))
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}
				level := slog.LevelInfo
				switch {
				case status >= 500:
					level = slog.LevelError
				case status >= 400:
					level = slog.LevelWarn
				}
				attrs := []slog.Attr{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
					slog.String("request_id", chimw.GetReqID(r.Context())),
					slog.String("remote_addr", r.RemoteAddr),
				}
				if info.userID != 0 {
					attrs = append(attrs, slog.Int64("user_id", info.userID))
				}
				log.LogAttrs(r.Context(), level, "http request", attrs...)
			}()
			next.ServeHTTP(ww, r)
		})
	}
}

// Recoverer turns a handler panic into a JSON 500 and logs the stack trace.
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				log.ErrorContext(r.Context(), "panic recovered",
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
					slog.String("request_id", chimw.GetReqID(r.Context())),
				)
				writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			}()
			next.ServeHTTP(w, r)
		})
	}
}
