package domain

import (
	"context"
	"strings"
	"time"
)

// PromotionApplication is a request to move an employee to a higher grade
// (permohonan naik pangkat). It starts PENDING and is approved or rejected
// exactly once.
type PromotionApplication struct {
	ID              int64          `db:"id" json:"id"`
	EmployeeID      int64          `db:"employee_id" json:"employee_id"`
	CurrentGradeID  int64          `db:"current_grade_id" json:"current_grade_id"`
	ProposedGradeID int64          `db:"proposed_grade_id" json:"proposed_grade_id"`
	Justification   string         `db:"justification" json:"justification"`
	Status          ApprovalStatus `db:"status" json:"status"`
	SubmittedAt     time.Time      `db:"submitted_at" json:"submitted_at"`
	ReviewedBy      *int64         `db:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time     `db:"reviewed_at" json:"reviewed_at,omitempty"`
	ReviewRemarks   string         `db:"review_remarks" json:"review_remarks"`
	EffectiveDate   *time.Time     `db:"effective_date" json:"effective_date,omitempty"`
	CreatedAt       time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at" json:"updated_at"`
}

func (p *PromotionApplication) Validate() error {
	v := NewValidationError()
	if p.EmployeeID <= 0 {
		v.Add("employee_id", "is required")
	}
	if p.CurrentGradeID <= 0 {
		v.Add("current_grade_id", "is required")
	}
	if p.ProposedGradeID <= 0 {
		v.Add("proposed_grade_id", "is required")
	} else if p.ProposedGradeID == p.CurrentGradeID {
		v.Add("proposed_grade_id", "must differ from current_grade_id")
	}
	if strings.TrimSpace(p.Justification) == "" {
		v.Add("justification", "is required")
	}
	if !p.Status.IsValid() {
		v.Add("status", "is invalid")
	}
	return v.Err()
}

// Approve moves a pending application to APPROVED. effectiveDate is the date
// the new grade takes effect.
func (p *PromotionApplication) Approve(reviewerID int64, remarks string, effectiveDate, now time.Time) error {
	if err := checkTransition("promotion application", p.Status, StatusApproved); err != nil {
		return err
	}
	v := reviewValidation(reviewerID, StatusApproved, remarks)
	if effectiveDate.IsZero() {
		v.Add("effective_date", "is required when approving")
	}
	if err := v.Err(); err != nil {
		return err
	}

	p.applyReview(StatusApproved, reviewerID, remarks, now)
	p.EffectiveDate = &effectiveDate
	return nil
}

// Reject moves a pending application to REJECTED. remarks is required.
func (p *PromotionApplication) Reject(reviewerID int64, remarks string, now time.Time) error {
	if err := checkTransition("promotion application", p.Status, StatusRejected); err != nil {
		return err
	}
	if err := reviewValidation(reviewerID, StatusRejected, remarks).Err(); err != nil {
		return err
	}

	p.applyReview(StatusRejected, reviewerID, remarks, now)
	return nil
}

func (p *PromotionApplication) applyReview(status ApprovalStatus, reviewerID int64, remarks string, now time.Time) {
	p.Status = status
	p.ReviewedBy = &reviewerID
	p.ReviewedAt = &now
	p.ReviewRemarks = strings.TrimSpace(remarks)
}

// PromotionFilter narrows List results. Zero-valued fields are not applied.
type PromotionFilter struct {
	EmployeeID int64
	Status     ApprovalStatus
	Pagination
}

// PromotionRepository persists promotion applications. Lookups return
// ErrNotFound when no row matches.
type PromotionRepository interface {
	// Create inserts p and sets p.ID.
	Create(ctx context.Context, p *PromotionApplication) error
	GetByID(ctx context.Context, id int64) (*PromotionApplication, error)
	// List returns one page of matching applications and the total match count.
	List(ctx context.Context, f PromotionFilter) ([]PromotionApplication, int64, error)
	HasPendingByEmployee(ctx context.Context, employeeID int64) (bool, error)
	// UpdateReview persists the review fields of p, but only if the stored row
	// is still PENDING. It returns ErrInvalidStatusTransition otherwise, so two
	// concurrent reviews cannot both succeed.
	UpdateReview(ctx context.Context, p *PromotionApplication) error
}
