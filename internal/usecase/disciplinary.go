package usecase

import (
	"context"
	"fmt"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

type DisciplinaryUsecase struct {
	tx        domain.Transactor
	employees domain.EmployeeRepository
	records   domain.DisciplinaryRepository
	users     domain.UserRepository
	now       Clock
}

var _ domain.DisciplinaryUsecase = (*DisciplinaryUsecase)(nil)

func NewDisciplinaryUsecase(
	tx domain.Transactor,
	employees domain.EmployeeRepository,
	records domain.DisciplinaryRepository,
	users domain.UserRepository,
	now Clock,
) *DisciplinaryUsecase {
	return &DisciplinaryUsecase{tx: tx, employees: employees, records: records, users: users, now: now}
}

// Create opens a PENDING case. d.ReportedBy must be set by the caller to the
// reporting user.
func (u *DisciplinaryUsecase) Create(ctx context.Context, d *domain.DisciplinaryRecord) error {
	trim(&d.CaseNo)
	trim(&d.Description)
	d.IncidentDate = dateOnly(d.IncidentDate)
	d.Status = domain.StatusPending
	d.ReviewedBy, d.ReviewedAt, d.ReviewRemarks = nil, nil, ""

	if err := d.Validate(); err != nil {
		return err
	}
	if d.IncidentDate.After(u.now()) {
		return validationErr("incident_date", "must not be in the future")
	}
	if _, err := u.employees.GetByID(ctx, d.EmployeeID); err != nil {
		return fieldNotFound(err, "employee_id")
	}
	return u.records.Create(ctx, d)
}

// Review approves or rejects a pending case. The reporting officer and the
// employee concerned may not review it. Approving a DISMISSAL also marks the
// employee TERMINATED.
func (u *DisciplinaryUsecase) Review(ctx context.Context, id int64, dec domain.ReviewDecision) (*domain.DisciplinaryRecord, error) {
	if err := dec.Validate(); err != nil {
		return nil, err
	}

	var result *domain.DisciplinaryRecord
	err := u.tx.WithinTx(ctx, func(ctx context.Context) error {
		rec, err := u.records.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if dec.ReviewerID == rec.ReportedBy {
			return fmt.Errorf("%w: the reporting officer cannot review the same case", domain.ErrForbidden)
		}
		reviewer, err := loadReviewer(ctx, u.users, dec.ReviewerID)
		if err != nil {
			return err
		}
		if isSameEmployee(reviewer, rec.EmployeeID) {
			return fmt.Errorf("%w: you cannot review your own disciplinary case", domain.ErrForbidden)
		}

		now := u.now()
		if dec.Status == domain.StatusApproved {
			err = rec.Approve(dec.ReviewerID, dec.Remarks, now)
		} else {
			err = rec.Reject(dec.ReviewerID, dec.Remarks, now)
		}
		if err != nil {
			return err
		}
		if err := u.records.UpdateReview(ctx, rec); err != nil {
			return err
		}

		if rec.Status == domain.StatusApproved && rec.Penalty == domain.PenaltyDismissal {
			emp, err := u.employees.GetByID(ctx, rec.EmployeeID)
			if err != nil {
				return err
			}
			emp.ServiceStatus = domain.ServiceStatusTerminated
			if err := u.employees.Update(ctx, emp); err != nil {
				return err
			}
		}
		result = rec
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (u *DisciplinaryUsecase) Get(ctx context.Context, id int64) (*domain.DisciplinaryRecord, error) {
	return u.records.GetByID(ctx, id)
}

func (u *DisciplinaryUsecase) List(ctx context.Context, f domain.DisciplinaryFilter) (domain.Page[domain.DisciplinaryRecord], error) {
	f.Normalize()
	items, total, err := u.records.List(ctx, f)
	if err != nil {
		return domain.Page[domain.DisciplinaryRecord]{}, err
	}
	return domain.NewPage(items, total, f.Pagination), nil
}
