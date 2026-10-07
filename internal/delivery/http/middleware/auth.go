package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/jwt"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/response"
)

type TokenParser interface {
	Parse(token string) (*jwt.Claims, error)
}

// Identity is the authenticated caller, taken from the access token.
type Identity struct {
	UserID     int64
	Role       domain.Role
	EmployeeID *int64
}

type identityKey struct{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}

// Authenticate requires a valid "Authorization: Bearer <token>" header and
// stores the caller's Identity in the request context.
func Authenticate(tokens TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
				w.Header().Set("WWW-Authenticate", `Bearer`)
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
				return
			}
			claims, err := tokens.Parse(strings.TrimSpace(token))
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
				return
			}
			id := Identity{UserID: claims.UserID, Role: domain.Role(claims.Role), EmployeeID: claims.EmployeeID}
			if info, ok := r.Context().Value(requestInfoKey{}).(*requestInfo); ok {
				info.userID = id.UserID
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

// RequireRoles allows the request only when the caller has one of roles.
// It must run after Authenticate.
func RequireRoles(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFrom(r.Context())
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
				return
			}
			if !slices.Contains(roles, id.Role) {
				writeError(w, r, http.StatusForbidden, "FORBIDDEN", "your role is not allowed to perform this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	response.Error(w, status, response.ErrorDetail{
		Code:      code,
		Message:   msg,
		RequestID: chimw.GetReqID(r.Context()),
	})
}
