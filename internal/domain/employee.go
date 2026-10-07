package domain

import (
	"context"
	"net/mail"
	"strings"
	"time"
)

// ServiceStatus is an employee's current employment status.
type ServiceStatus string

const (
	ServiceStatusActive     ServiceStatus = "ACTIVE"
	ServiceStatusSuspended  ServiceStatus = "SUSPENDED"
	ServiceStatusRetired    ServiceStatus = "RETIRED"
	ServiceStatusResigned   ServiceStatus = "RESIGNED"
	ServiceStatusTerminated ServiceStatus = "TERMINATED"
)

func (s ServiceStatus) IsValid() bool {
	switch s {
	case ServiceStatusActive, ServiceStatusSuspended, ServiceStatusRetired,
		ServiceStatusResigned, ServiceStatusTerminated:
		return true
	}
	return false
}

type Gender string

const (
	GenderMale   Gender = "MALE"
	GenderFemale Gender = "FEMALE"
)

func (g Gender) IsValid() bool {
	return g == GenderMale || g == GenderFemale
}

// Employee is a personnel record in the service records module
// (Modul Perkhidmatan).
type Employee struct {
	ID             int64         `db:"id" json:"id"`
	EmployeeNo     string        `db:"employee_no" json:"employee_no"`
	ICNumber       string        `db:"ic_number" json:"ic_number"`
	FullName       string        `db:"full_name" json:"full_name"`
	Email          string        `db:"email" json:"email"`
	Phone          string        `db:"phone" json:"phone"`
	DateOfBirth    time.Time     `db:"date_of_birth" json:"date_of_birth"`
	Gender         Gender        `db:"gender" json:"gender"`
	CurrentGradeID int64         `db:"current_grade_id" json:"current_grade_id"`
	Position       string        `db:"position" json:"position"`
	Department     string        `db:"department" json:"department"`
	ServiceStatus  ServiceStatus `db:"service_status" json:"service_status"`
	DateJoined     time.Time     `db:"date_joined" json:"date_joined"`
	ConfirmedAt    *time.Time    `db:"confirmed_at" json:"confirmed_at,omitempty"`
	CreatedAt      time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at" json:"updated_at"`
}

func (e *Employee) IsActive() bool {
	return e.ServiceStatus == ServiceStatusActive
}

func (e *Employee) Validate() error {
	v := NewValidationError()
	if strings.TrimSpace(e.EmployeeNo) == "" {
		v.Add("employee_no", "is required")
	}
	if !IsValidICNumber(e.ICNumber) {
		v.Add("ic_number", "must be a 12-digit MyKad number")
	}
	if strings.TrimSpace(e.FullName) == "" {
		v.Add("full_name", "is required")
	}
	if addr, err := mail.ParseAddress(e.Email); err != nil || addr.Address != e.Email {
		v.Add("email", "must be a valid email address")
	}
	if e.DateOfBirth.IsZero() {
		v.Add("date_of_birth", "is required")
	}
	if !e.Gender.IsValid() {
		v.Add("gender", "must be MALE or FEMALE")
	}
	if e.CurrentGradeID <= 0 {
		v.Add("current_grade_id", "is required")
	}
	if strings.TrimSpace(e.Position) == "" {
		v.Add("position", "is required")
	}
	if strings.TrimSpace(e.Department) == "" {
		v.Add("department", "is required")
	}
	if !e.ServiceStatus.IsValid() {
		v.Add("service_status", "is invalid")
	}
	if e.DateJoined.IsZero() {
		v.Add("date_joined", "is required")
	} else if !e.DateOfBirth.IsZero() && !e.DateJoined.After(e.DateOfBirth) {
		v.Add("date_joined", "must be after date_of_birth")
	}
	if e.ConfirmedAt != nil && !e.DateJoined.IsZero() && e.ConfirmedAt.Before(e.DateJoined) {
		v.Add("confirmed_at", "must not be before date_joined")
	}
	return v.Err()
}

// NormalizeICNumber strips the dashes and spaces commonly used when writing
// MyKad numbers, e.g. "900101-14-5678" becomes "900101145678".
func NormalizeICNumber(ic string) string {
	return strings.NewReplacer("-", "", " ", "").Replace(ic)
}

// IsValidICNumber reports whether ic is 12 digits whose first six form a
// valid YYMMDD date.
func IsValidICNumber(ic string) bool {
	if len(ic) != 12 {
		return false
	}
	for _, r := range ic {
		if r < '0' || r > '9' {
			return false
		}
	}
	_, err := time.Parse("060102", ic[:6])
	return err == nil
}

// EmployeeFilter narrows List results. Zero-valued fields are not applied.
type EmployeeFilter struct {
	// Search matches against employee number, IC number and full name.
	Search     string
	Department string
	GradeID    int64
	Status     ServiceStatus
	Pagination
}

// EmployeeRepository persists employees. Lookups return ErrNotFound when no
// row matches, and writes return ErrConflict on a duplicate employee number,
// IC number or email.
type EmployeeRepository interface {
	// Create inserts e and sets e.ID.
	Create(ctx context.Context, e *Employee) error
	GetByID(ctx context.Context, id int64) (*Employee, error)
	GetByEmployeeNo(ctx context.Context, employeeNo string) (*Employee, error)
	GetByICNumber(ctx context.Context, icNumber string) (*Employee, error)
	// List returns one page of matching employees and the total match count.
	List(ctx context.Context, f EmployeeFilter) ([]Employee, int64, error)
	Update(ctx context.Context, e *Employee) error
	UpdateGrade(ctx context.Context, id, gradeID int64) error
	Delete(ctx context.Context, id int64) error
}
