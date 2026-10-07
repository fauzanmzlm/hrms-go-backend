package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

// Clock returns the current time. Usecases take one so tests can fix time.
type Clock func() time.Time

func SystemClock() time.Time {
	return time.Now().UTC()
}

type PasswordHasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) error
}

type TokenIssuer interface {
	Issue(userID int64, role string, employeeID *int64) (token string, expiresAt time.Time, err error)
}

// dateOnly truncates t to midnight UTC on its calendar date.
func dateOnly(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func dateOnlyPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := dateOnly(*t)
	return &d
}

func validationErr(field, msg string) error {
	v := domain.NewValidationError()
	v.Add(field, msg)
	return v
}

// fieldNotFound turns ErrNotFound for a referenced entity into a validation
// error on field, so the client sees 422 rather than 404.
func fieldNotFound(err error, field string) error {
	if errors.Is(err, domain.ErrNotFound) {
		return validationErr(field, "does not exist")
	}
	return err
}

func ruleViolation(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrRuleViolation, fmt.Sprintf(format, args...))
}

// loadReviewer resolves the reviewing user. A reviewer that no longer exists
// or is inactive is treated as unauthenticated.
func loadReviewer(ctx context.Context, users domain.UserRepository, id int64) (*domain.User, error) {
	u, err := users.GetByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && !u.IsActive) {
		return nil, fmt.Errorf("%w: reviewer account is not active", domain.ErrUnauthorized)
	}
	return u, err
}

func isSameEmployee(u *domain.User, employeeID int64) bool {
	return u.EmployeeID != nil && *u.EmployeeID == employeeID
}

// serviceHistory maintains the invariant that an employee's current service
// record matches the grade, position and department on the employee row.
type serviceHistory struct {
	employees domain.EmployeeRepository
	records   domain.ServiceRecordRepository
}

// appendCurrent makes r the employee's current record. The previous current
// record, if any, is closed the day before r.EffectiveFrom. dateField names
// the input field reported when r.EffectiveFrom is too early. Callers must
// run it inside a transaction.
func (h serviceHistory) appendCurrent(ctx context.Context, emp *domain.Employee, r *domain.ServiceRecord, dateField string) error {
	prev, err := h.records.GetCurrentByEmployee(ctx, emp.ID)
	switch {
	case err == nil:
		if !r.EffectiveFrom.After(prev.EffectiveFrom) {
			return validationErr(dateField, fmt.Sprintf("must be after %s, the start of the current service record",
				prev.EffectiveFrom.Format(time.DateOnly)))
		}
		if err := h.records.CloseCurrent(ctx, emp.ID, r.EffectiveFrom.AddDate(0, 0, -1)); err != nil {
			return err
		}
	case !errors.Is(err, domain.ErrNotFound):
		return err
	}

	if err := h.records.Create(ctx, r); err != nil {
		return err
	}
	return h.syncEmployee(ctx, emp, r)
}

func (h serviceHistory) syncEmployee(ctx context.Context, emp *domain.Employee, r *domain.ServiceRecord) error {
	emp.CurrentGradeID, emp.Position, emp.Department = r.GradeID, r.Position, r.Department
	return h.employees.Update(ctx, emp)
}

func trim(s *string) {
	*s = strings.TrimSpace(*s)
}
