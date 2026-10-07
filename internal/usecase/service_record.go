package usecase

import (
	"context"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

type ServiceRecordUsecase struct {
	tx        domain.Transactor
	employees domain.EmployeeRepository
	grades    domain.GradeRepository
	records   domain.ServiceRecordRepository
	history   serviceHistory
}

var _ domain.ServiceRecordUsecase = (*ServiceRecordUsecase)(nil)

func NewServiceRecordUsecase(
	tx domain.Transactor,
	employees domain.EmployeeRepository,
	grades domain.GradeRepository,
	records domain.ServiceRecordRepository,
) *ServiceRecordUsecase {
	return &ServiceRecordUsecase{
		tx:        tx,
		employees: employees,
		grades:    grades,
		records:   records,
		history:   serviceHistory{employees: employees, records: records},
	}
}

func normalizeServiceRecord(r *domain.ServiceRecord) {
	trim(&r.Position)
	trim(&r.Department)
	trim(&r.Remarks)
	r.EffectiveFrom = dateOnly(r.EffectiveFrom)
	r.EffectiveTo = dateOnlyPtr(r.EffectiveTo)
}

func (u *ServiceRecordUsecase) Create(ctx context.Context, r *domain.ServiceRecord) error {
	normalizeServiceRecord(r)
	if err := r.Validate(); err != nil {
		return err
	}
	if _, err := u.grades.GetByID(ctx, r.GradeID); err != nil {
		return fieldNotFound(err, "grade_id")
	}

	return u.tx.WithinTx(ctx, func(ctx context.Context) error {
		emp, err := u.employees.GetByID(ctx, r.EmployeeID)
		if err != nil {
			return err
		}
		if r.IsCurrent() {
			return u.history.appendCurrent(ctx, emp, r, "effective_from")
		}
		return u.records.Create(ctx, r)
	})
}

func (u *ServiceRecordUsecase) GetByID(ctx context.Context, id int64) (*domain.ServiceRecord, error) {
	return u.records.GetByID(ctx, id)
}

func (u *ServiceRecordUsecase) ListByEmployee(ctx context.Context, employeeID int64) ([]domain.ServiceRecord, error) {
	if _, err := u.employees.GetByID(ctx, employeeID); err != nil {
		return nil, err
	}
	records, err := u.records.ListByEmployee(ctx, employeeID)
	if records == nil && err == nil {
		records = []domain.ServiceRecord{}
	}
	return records, err
}

// Update edits a record in place. The owning employee cannot change, and
// editing the current record also updates the employee's grade, position and
// department.
func (u *ServiceRecordUsecase) Update(ctx context.Context, r *domain.ServiceRecord) error {
	return u.tx.WithinTx(ctx, func(ctx context.Context) error {
		existing, err := u.records.GetByID(ctx, r.ID)
		if err != nil {
			return err
		}
		r.EmployeeID = existing.EmployeeID
		r.CreatedAt = existing.CreatedAt
		normalizeServiceRecord(r)
		if err := r.Validate(); err != nil {
			return err
		}
		if r.GradeID != existing.GradeID {
			if _, err := u.grades.GetByID(ctx, r.GradeID); err != nil {
				return fieldNotFound(err, "grade_id")
			}
		}
		if err := u.records.Update(ctx, r); err != nil {
			return err
		}
		if !r.IsCurrent() {
			return nil
		}
		emp, err := u.employees.GetByID(ctx, r.EmployeeID)
		if err != nil {
			return err
		}
		return u.history.syncEmployee(ctx, emp, r)
	})
}

func (u *ServiceRecordUsecase) Delete(ctx context.Context, id int64) error {
	return u.records.Delete(ctx, id)
}
