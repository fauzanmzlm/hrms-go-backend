package domain

import (
	"context"
	"strings"
	"time"
)

type AppointmentType string

const (
	AppointmentPermanent  AppointmentType = "PERMANENT"
	AppointmentContract   AppointmentType = "CONTRACT"
	AppointmentTemporary  AppointmentType = "TEMPORARY"
	AppointmentSecondment AppointmentType = "SECONDMENT"
	AppointmentPromotion  AppointmentType = "PROMOTION"
)

func (t AppointmentType) IsValid() bool {
	switch t {
	case AppointmentPermanent, AppointmentContract, AppointmentTemporary,
		AppointmentSecondment, AppointmentPromotion:
		return true
	}
	return false
}

// ServiceRecord is one entry in an employee's service history (rekod
// perkhidmatan). EffectiveFrom and EffectiveTo are inclusive dates; a nil
// EffectiveTo marks the employee's current record.
type ServiceRecord struct {
	ID              int64           `db:"id" json:"id"`
	EmployeeID      int64           `db:"employee_id" json:"employee_id"`
	GradeID         int64           `db:"grade_id" json:"grade_id"`
	Position        string          `db:"position" json:"position"`
	Department      string          `db:"department" json:"department"`
	AppointmentType AppointmentType `db:"appointment_type" json:"appointment_type"`
	EffectiveFrom   time.Time       `db:"effective_from" json:"effective_from"`
	EffectiveTo     *time.Time      `db:"effective_to" json:"effective_to,omitempty"`
	Remarks         string          `db:"remarks" json:"remarks"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at" json:"updated_at"`
}

func (r *ServiceRecord) IsCurrent() bool {
	return r.EffectiveTo == nil
}

func (r *ServiceRecord) Validate() error {
	v := NewValidationError()
	if r.EmployeeID <= 0 {
		v.Add("employee_id", "is required")
	}
	if r.GradeID <= 0 {
		v.Add("grade_id", "is required")
	}
	if strings.TrimSpace(r.Position) == "" {
		v.Add("position", "is required")
	}
	if strings.TrimSpace(r.Department) == "" {
		v.Add("department", "is required")
	}
	if !r.AppointmentType.IsValid() {
		v.Add("appointment_type", "is invalid")
	}
	if r.EffectiveFrom.IsZero() {
		v.Add("effective_from", "is required")
	} else if r.EffectiveTo != nil && r.EffectiveTo.Before(r.EffectiveFrom) {
		v.Add("effective_to", "must not be before effective_from")
	}
	return v.Err()
}

// ServiceRecordRepository persists service history. Lookups return
// ErrNotFound when no row matches.
type ServiceRecordRepository interface {
	// Create inserts r and sets r.ID.
	Create(ctx context.Context, r *ServiceRecord) error
	GetByID(ctx context.Context, id int64) (*ServiceRecord, error)
	// ListByEmployee returns the employee's history, newest first.
	ListByEmployee(ctx context.Context, employeeID int64) ([]ServiceRecord, error)
	// GetCurrentByEmployee returns the record whose EffectiveTo is nil.
	GetCurrentByEmployee(ctx context.Context, employeeID int64) (*ServiceRecord, error)
	// CloseCurrent sets EffectiveTo on the employee's current record. It is a
	// no-op when the employee has no current record.
	CloseCurrent(ctx context.Context, employeeID int64, endDate time.Time) error
	Update(ctx context.Context, r *ServiceRecord) error
	Delete(ctx context.Context, id int64) error
}
