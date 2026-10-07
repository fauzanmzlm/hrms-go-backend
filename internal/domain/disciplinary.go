package domain

import (
	"context"
	"strings"
	"time"
)

// OffenseCategory classifies a disciplinary case (kes tatatertib).
type OffenseCategory string

const (
	// OffenseMinor is a case not pursued with a view to dismissal or demotion.
	OffenseMinor OffenseCategory = "MINOR"
	// OffenseSerious is a case pursued with a view to dismissal or demotion.
	OffenseSerious OffenseCategory = "SERIOUS"
)

func (c OffenseCategory) IsValid() bool {
	return c == OffenseMinor || c == OffenseSerious
}

// PenaltyType is a disciplinary punishment (hukuman tatatertib), ordered from
// least to most severe.
type PenaltyType string

const (
	PenaltyWarning          PenaltyType = "WARNING"
	PenaltyFine             PenaltyType = "FINE"
	PenaltyForfeitEmolument PenaltyType = "FORFEIT_EMOLUMENT"
	PenaltySalaryDeferment  PenaltyType = "SALARY_DEFERMENT"
	PenaltySalaryReduction  PenaltyType = "SALARY_REDUCTION"
	PenaltyDemotion         PenaltyType = "DEMOTION"
	PenaltyDismissal        PenaltyType = "DISMISSAL"
)

func (p PenaltyType) IsValid() bool {
	switch p {
	case PenaltyWarning, PenaltyFine, PenaltyForfeitEmolument, PenaltySalaryDeferment,
		PenaltySalaryReduction, PenaltyDemotion, PenaltyDismissal:
		return true
	}
	return false
}

// AllowedFor reports whether p may be imposed for a case of category c.
// Demotion and dismissal are reserved for SERIOUS cases.
func (p PenaltyType) AllowedFor(c OffenseCategory) bool {
	if p == PenaltyDemotion || p == PenaltyDismissal {
		return c == OffenseSerious
	}
	return p.IsValid() && c.IsValid()
}

// DisciplinaryRecord is a disciplinary case against an employee with a
// proposed penalty. It starts PENDING and is approved or rejected exactly
// once; only APPROVED records count against the employee.
type DisciplinaryRecord struct {
	ID            int64           `db:"id" json:"id"`
	EmployeeID    int64           `db:"employee_id" json:"employee_id"`
	CaseNo        string          `db:"case_no" json:"case_no"`
	Category      OffenseCategory `db:"category" json:"category"`
	Description   string          `db:"description" json:"description"`
	IncidentDate  time.Time       `db:"incident_date" json:"incident_date"`
	Penalty       PenaltyType     `db:"penalty" json:"penalty"`
	Status        ApprovalStatus  `db:"status" json:"status"`
	ReportedBy    int64           `db:"reported_by" json:"reported_by"`
	ReviewedBy    *int64          `db:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time      `db:"reviewed_at" json:"reviewed_at,omitempty"`
	ReviewRemarks string          `db:"review_remarks" json:"review_remarks"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
}

func (d *DisciplinaryRecord) Validate() error {
	v := NewValidationError()
	if d.EmployeeID <= 0 {
		v.Add("employee_id", "is required")
	}
	if strings.TrimSpace(d.CaseNo) == "" {
		v.Add("case_no", "is required")
	}
	if !d.Category.IsValid() {
		v.Add("category", "must be MINOR or SERIOUS")
	}
	if strings.TrimSpace(d.Description) == "" {
		v.Add("description", "is required")
	}
	if d.IncidentDate.IsZero() {
		v.Add("incident_date", "is required")
	}
	if !d.Penalty.IsValid() {
		v.Add("penalty", "is invalid")
	} else if d.Category.IsValid() && !d.Penalty.AllowedFor(d.Category) {
		v.Add("penalty", "is only allowed for SERIOUS cases")
	}
	if !d.Status.IsValid() {
		v.Add("status", "is invalid")
	}
	if d.ReportedBy <= 0 {
		v.Add("reported_by", "is required")
	}
	return v.Err()
}

// Approve moves a pending record to APPROVED, confirming the penalty.
func (d *DisciplinaryRecord) Approve(reviewerID int64, remarks string, now time.Time) error {
	return d.review(StatusApproved, reviewerID, remarks, now)
}

// Reject moves a pending record to REJECTED. remarks is required.
func (d *DisciplinaryRecord) Reject(reviewerID int64, remarks string, now time.Time) error {
	return d.review(StatusRejected, reviewerID, remarks, now)
}

func (d *DisciplinaryRecord) review(status ApprovalStatus, reviewerID int64, remarks string, now time.Time) error {
	if err := checkTransition("disciplinary record", d.Status, status); err != nil {
		return err
	}
	if err := reviewValidation(reviewerID, status, remarks).Err(); err != nil {
		return err
	}

	d.Status = status
	d.ReviewedBy = &reviewerID
	d.ReviewedAt = &now
	d.ReviewRemarks = strings.TrimSpace(remarks)
	return nil
}

// DisciplinaryFilter narrows List results. Zero-valued fields are not applied.
type DisciplinaryFilter struct {
	EmployeeID int64
	Status     ApprovalStatus
	Category   OffenseCategory
	Pagination
}

// DisciplinaryRepository persists disciplinary records. Lookups return
// ErrNotFound when no row matches, and Create returns ErrConflict on a
// duplicate case number.
type DisciplinaryRepository interface {
	// Create inserts d and sets d.ID.
	Create(ctx context.Context, d *DisciplinaryRecord) error
	GetByID(ctx context.Context, id int64) (*DisciplinaryRecord, error)
	// List returns one page of matching records and the total match count.
	List(ctx context.Context, f DisciplinaryFilter) ([]DisciplinaryRecord, int64, error)
	// UpdateReview persists the review fields of d, but only if the stored row
	// is still PENDING. It returns ErrInvalidStatusTransition otherwise.
	UpdateReview(ctx context.Context, d *DisciplinaryRecord) error
	// HasApprovedSince reports whether the employee has an APPROVED record
	// reviewed at or after since.
	HasApprovedSince(ctx context.Context, employeeID int64, since time.Time) (bool, error)
}
