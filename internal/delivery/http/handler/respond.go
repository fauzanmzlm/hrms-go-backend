package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/fauzanmzlm/hrms-go-backend/internal/delivery/http/middleware"
	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/response"
)

const maxBodyBytes = 1 << 20

// badRequestError is a malformed request (bad JSON, bad path or query
// parameter), as opposed to a well-formed request that fails validation.
type badRequestError struct {
	msg string
}

func (e *badRequestError) Error() string { return e.msg }

func badRequest(format string, args ...any) error {
	return &badRequestError{msg: fmt.Sprintf(format, args...)}
}

// errorStatus maps an error to its HTTP status and machine-readable code.
func errorStatus(err error) (int, string) {
	var br *badRequestError
	switch {
	case errors.As(err, &br):
		return http.StatusBadRequest, "BAD_REQUEST"
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusUnprocessableEntity, "VALIDATION_FAILED"
	case errors.Is(err, domain.ErrRuleViolation):
		return http.StatusUnprocessableEntity, "RULE_VIOLATION"
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, domain.ErrInvalidStatusTransition):
		return http.StatusConflict, "INVALID_STATUS_TRANSITION"
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, "CONFLICT"
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, "UNAUTHORIZED"
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

// writeError renders err as the standard JSON error body. Details of
// unexpected errors are logged, never sent to the client.
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	status, code := errorStatus(err)
	detail := response.ErrorDetail{
		Code:      code,
		Message:   err.Error(),
		RequestID: chimw.GetReqID(r.Context()),
	}

	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		detail.Message = "validation failed"
		detail.Fields = ve.Fields
	}
	if status == http.StatusInternalServerError {
		log.ErrorContext(r.Context(), "request failed",
			slog.String("error", err.Error()),
			slog.String("request_id", detail.RequestID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)
		detail.Message = "internal server error"
	}
	response.Error(w, status, detail)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return badRequest("request body must not exceed %d bytes", maxErr.Limit)
		case errors.Is(err, io.EOF):
			return badRequest("request body must not be empty")
		default:
			return badRequest("malformed JSON body: %s", err.Error())
		}
	}
	if dec.More() {
		return badRequest("request body must contain a single JSON object")
	}
	return nil
}

func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		return 0, badRequest("path parameter %q must be a positive integer", name)
	}
	return id, nil
}

func queryInt64(r *http.Request, key string) (int64, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return 0, badRequest("query parameter %q must be a non-negative integer", key)
	}
	return v, nil
}

func pagination(r *http.Request) (domain.Pagination, error) {
	page, err := queryInt64(r, "page")
	if err != nil {
		return domain.Pagination{}, err
	}
	size, err := queryInt64(r, "page_size")
	if err != nil {
		return domain.Pagination{}, err
	}
	p := domain.Pagination{Page: int(min(page, 1<<31-1)), PageSize: int(min(size, domain.MaxPageSize))}
	p.Normalize()
	return p, nil
}

func queryStatus(r *http.Request) (domain.ApprovalStatus, error) {
	s := domain.ApprovalStatus(r.URL.Query().Get("status"))
	if s != "" && !s.IsValid() {
		return "", badRequest("query parameter \"status\" must be PENDING, APPROVED or REJECTED")
	}
	return s, nil
}

// Date is a calendar date in request bodies, written as "YYYY-MM-DD".
type Date struct {
	time.Time
}

func (d *Date) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return errors.New("dates must be strings in YYYY-MM-DD format")
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return fmt.Errorf("invalid date %q, want YYYY-MM-DD", s)
	}
	d.Time = t
	return nil
}

// ptr returns nil for a missing or zero date.
func (d *Date) ptr() *time.Time {
	if d == nil || d.IsZero() {
		return nil
	}
	t := d.Time
	return &t
}

func identity(r *http.Request) middleware.Identity {
	id, _ := middleware.IdentityFrom(r.Context())
	return id
}

// ownEmployeeID reports the employee an EMPLOYEE-role caller is limited to.
// restricted is false for every other role.
func ownEmployeeID(id middleware.Identity) (employeeID int64, restricted bool, err error) {
	if id.Role != domain.RoleEmployee {
		return 0, false, nil
	}
	if id.EmployeeID == nil {
		return 0, true, fmt.Errorf("%w: your account is not linked to an employee record", domain.ErrForbidden)
	}
	return *id.EmployeeID, true, nil
}

// ensureCanView rejects an EMPLOYEE-role caller reading another employee's data.
func ensureCanView(id middleware.Identity, employeeID int64) error {
	own, restricted, err := ownEmployeeID(id)
	if err != nil {
		return err
	}
	if restricted && own != employeeID {
		return fmt.Errorf("%w: you can only access your own records", domain.ErrForbidden)
	}
	return nil
}
