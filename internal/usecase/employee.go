package usecase

import (
	"context"
	"strings"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

type EmployeeUsecase struct {
	employees domain.EmployeeRepository
	grades    domain.GradeRepository
}

var _ domain.EmployeeUsecase = (*EmployeeUsecase)(nil)

func NewEmployeeUsecase(employees domain.EmployeeRepository, grades domain.GradeRepository) *EmployeeUsecase {
	return &EmployeeUsecase{employees: employees, grades: grades}
}

func normalizeEmployee(e *domain.Employee) {
	trim(&e.EmployeeNo)
	trim(&e.FullName)
	trim(&e.Phone)
	trim(&e.Position)
	trim(&e.Department)
	e.ICNumber = domain.NormalizeICNumber(e.ICNumber)
	e.Email = strings.ToLower(strings.TrimSpace(e.Email))
	if e.ServiceStatus == "" {
		e.ServiceStatus = domain.ServiceStatusActive
	}
	e.DateOfBirth = dateOnly(e.DateOfBirth)
	e.DateJoined = dateOnly(e.DateJoined)
	e.ConfirmedAt = dateOnlyPtr(e.ConfirmedAt)
}

func (u *EmployeeUsecase) Create(ctx context.Context, e *domain.Employee) error {
	normalizeEmployee(e)
	if err := e.Validate(); err != nil {
		return err
	}
	if _, err := u.grades.GetByID(ctx, e.CurrentGradeID); err != nil {
		return fieldNotFound(err, "current_grade_id")
	}
	return u.employees.Create(ctx, e)
}

func (u *EmployeeUsecase) GetByID(ctx context.Context, id int64) (*domain.Employee, error) {
	return u.employees.GetByID(ctx, id)
}

func (u *EmployeeUsecase) List(ctx context.Context, f domain.EmployeeFilter) (domain.Page[domain.Employee], error) {
	f.Normalize()
	items, total, err := u.employees.List(ctx, f)
	if err != nil {
		return domain.Page[domain.Employee]{}, err
	}
	return domain.NewPage(items, total, f.Pagination), nil
}

// Update replaces the employee's personal and placement details. The grade
// is kept as stored: it only changes through service records and approved
// promotions, so the service history stays the source of truth. An omitted
// service status keeps the stored one rather than defaulting to ACTIVE.
func (u *EmployeeUsecase) Update(ctx context.Context, e *domain.Employee) error {
	existing, err := u.employees.GetByID(ctx, e.ID)
	if err != nil {
		return err
	}
	if e.ServiceStatus == "" {
		e.ServiceStatus = existing.ServiceStatus
	}
	normalizeEmployee(e)
	e.CurrentGradeID = existing.CurrentGradeID
	e.CreatedAt = existing.CreatedAt
	if err := e.Validate(); err != nil {
		return err
	}
	return u.employees.Update(ctx, e)
}

func (u *EmployeeUsecase) Delete(ctx context.Context, id int64) error {
	return u.employees.Delete(ctx, id)
}
