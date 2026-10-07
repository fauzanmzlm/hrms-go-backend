package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/fauzanmzlm/hrms-go-backend/internal/delivery/http/handler"
	"github.com/fauzanmzlm/hrms-go-backend/internal/delivery/http/middleware"
	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/response"
)

// Pinger reports whether a dependency (the database) is reachable.
type Pinger interface {
	PingContext(ctx context.Context) error
}

type Handlers struct {
	Auth          *handler.AuthHandler
	Grade         *handler.GradeHandler
	Employee      *handler.EmployeeHandler
	ServiceRecord *handler.ServiceRecordHandler
	Promotion     *handler.PromotionHandler
	Disciplinary  *handler.DisciplinaryHandler
}

type RouterDeps struct {
	Logger   *slog.Logger
	Tokens   middleware.TokenParser
	DB       Pinger
	Handlers Handlers
}

// NewRouter builds the HTTP API. Every /api/v1 route except login requires
// a bearer token. EMPLOYEE-role callers are further limited inside the
// handlers to their own records.
func NewRouter(d RouterDeps) http.Handler {
	h := d.Handlers
	var (
		admin       = middleware.RequireRoles(domain.RoleAdmin)
		hrStaff     = middleware.RequireRoles(domain.RoleAdmin, domain.RoleHROfficer)
		approvers   = middleware.RequireRoles(domain.RoleAdmin, domain.RoleApprover)
		staff       = middleware.RequireRoles(domain.RoleAdmin, domain.RoleHROfficer, domain.RoleApprover)
		applicants  = middleware.RequireRoles(domain.RoleAdmin, domain.RoleHROfficer, domain.RoleEmployee)
		requireAuth = middleware.Authenticate(d.Tokens)
	)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(d.Logger))
	r.Use(middleware.Recoverer(d.Logger))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, http.StatusNotFound, response.ErrorDetail{
			Code: "NOT_FOUND", Message: "route not found", RequestID: chimw.GetReqID(r.Context()),
		})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, http.StatusMethodNotAllowed, response.ErrorDetail{
			Code: "METHOD_NOT_ALLOWED", Message: "method not allowed", RequestID: chimw.GetReqID(r.Context()),
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		response.Data(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.DB.PingContext(ctx); err != nil {
			d.Logger.WarnContext(r.Context(), "readiness check failed", slog.String("error", err.Error()))
			response.Error(w, http.StatusServiceUnavailable, response.ErrorDetail{
				Code: "UNAVAILABLE", Message: "database unreachable", RequestID: chimw.GetReqID(r.Context()),
			})
			return
		}
		response.Data(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", h.Auth.Login)

		r.Group(func(r chi.Router) {
			r.Use(requireAuth)

			r.Get("/auth/me", h.Auth.Me)
			r.With(admin).Post("/users", h.Auth.CreateUser)

			r.Route("/grades", func(r chi.Router) {
				r.Get("/", h.Grade.List)
				r.Get("/{id}", h.Grade.Get)
				r.With(hrStaff).Post("/", h.Grade.Create)
				r.With(hrStaff).Put("/{id}", h.Grade.Update)
				r.With(admin).Delete("/{id}", h.Grade.Delete)
			})

			r.Route("/employees", func(r chi.Router) {
				r.With(staff).Get("/", h.Employee.List)
				r.With(hrStaff).Post("/", h.Employee.Create)
				r.Get("/{id}", h.Employee.Get)
				r.With(hrStaff).Put("/{id}", h.Employee.Update)
				r.With(admin).Delete("/{id}", h.Employee.Delete)
				r.Get("/{id}/service-records", h.ServiceRecord.ListByEmployee)
				r.With(hrStaff).Post("/{id}/service-records", h.ServiceRecord.Create)
			})

			r.Route("/service-records", func(r chi.Router) {
				r.Get("/{id}", h.ServiceRecord.Get)
				r.With(hrStaff).Put("/{id}", h.ServiceRecord.Update)
				r.With(hrStaff).Delete("/{id}", h.ServiceRecord.Delete)
			})

			r.Route("/promotions", func(r chi.Router) {
				r.Get("/", h.Promotion.List)
				r.With(applicants).Post("/", h.Promotion.Submit)
				r.Get("/{id}", h.Promotion.Get)
				r.With(approvers).Post("/{id}/review", h.Promotion.Review)
			})

			r.Route("/disciplinary-records", func(r chi.Router) {
				r.Get("/", h.Disciplinary.List)
				r.With(hrStaff).Post("/", h.Disciplinary.Create)
				r.Get("/{id}", h.Disciplinary.Get)
				r.With(approvers).Post("/{id}/review", h.Disciplinary.Review)
			})
		})
	})

	return r
}
