package mysql

import (
	"context"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

const employeeColumns = `id, employee_no, ic_number, full_name, email, phone, date_of_birth, gender,
	current_grade_id, position, department, service_status, date_joined, confirmed_at,
	created_at, updated_at`

var employeeKeys = uniqueKeys{
	"uq_employees_employee_no": "employee number already exists",
	"uq_employees_ic_number":   "IC number already exists",
	"uq_employees_email":       "email already exists",
}

type EmployeeRepository struct {
	base
}

var _ domain.EmployeeRepository = (*EmployeeRepository)(nil)

func NewEmployeeRepository(db *sqlx.DB) *EmployeeRepository {
	return &EmployeeRepository{base{db: db}}
}

func (r *EmployeeRepository) Create(ctx context.Context, e *domain.Employee) error {
	ts := now()
	id, err := r.insert(ctx, "employee create", employeeKeys,
		`INSERT INTO employees (employee_no, ic_number, full_name, email, phone, date_of_birth, gender,
			current_grade_id, position, department, service_status, date_joined, confirmed_at,
			created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.EmployeeNo, e.ICNumber, e.FullName, e.Email, e.Phone, e.DateOfBirth, e.Gender,
		e.CurrentGradeID, e.Position, e.Department, e.ServiceStatus, e.DateJoined, e.ConfirmedAt,
		ts, ts)
	if err != nil {
		return err
	}
	e.ID, e.CreatedAt, e.UpdatedAt = id, ts, ts
	return nil
}

func (r *EmployeeRepository) GetByID(ctx context.Context, id int64) (*domain.Employee, error) {
	return r.getOne(ctx, "employee get", "id = ?", id)
}

func (r *EmployeeRepository) GetByEmployeeNo(ctx context.Context, employeeNo string) (*domain.Employee, error) {
	return r.getOne(ctx, "employee get by number", "employee_no = ?", employeeNo)
}

func (r *EmployeeRepository) GetByICNumber(ctx context.Context, icNumber string) (*domain.Employee, error) {
	return r.getOne(ctx, "employee get by ic", "ic_number = ?", icNumber)
}

func (r *EmployeeRepository) getOne(ctx context.Context, op, cond string, arg any) (*domain.Employee, error) {
	var e domain.Employee
	if err := r.get(ctx, op, &e, "SELECT "+employeeColumns+" FROM employees WHERE "+cond, arg); err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EmployeeRepository) List(ctx context.Context, f domain.EmployeeFilter) ([]domain.Employee, int64, error) {
	f.Normalize()

	var conds []string
	var args []any
	if s := strings.TrimSpace(f.Search); s != "" {
		pattern := likeContains(s)
		conds = append(conds, "(employee_no LIKE ? OR ic_number LIKE ? OR full_name LIKE ?)")
		args = append(args, pattern, pattern, pattern)
	}
	if f.Department != "" {
		conds = append(conds, "department = ?")
		args = append(args, f.Department)
	}
	if f.GradeID > 0 {
		conds = append(conds, "current_grade_id = ?")
		args = append(args, f.GradeID)
	}
	if f.Status != "" {
		conds = append(conds, "service_status = ?")
		args = append(args, f.Status)
	}
	where := whereClause(conds)

	var total int64
	if err := r.get(ctx, "employee count", &total, "SELECT COUNT(*) FROM employees"+where, args...); err != nil {
		return nil, 0, err
	}
	var items []domain.Employee
	err := r.selectAll(ctx, "employee list", &items,
		"SELECT "+employeeColumns+" FROM employees"+where+" ORDER BY full_name, id LIMIT ? OFFSET ?",
		append(args, f.Limit(), f.Offset())...)
	return items, total, err
}

func (r *EmployeeRepository) Update(ctx context.Context, e *domain.Employee) error {
	ts := now()
	err := r.execOne(ctx, "employee update", employeeKeys,
		`UPDATE employees SET employee_no = ?, ic_number = ?, full_name = ?, email = ?, phone = ?,
			date_of_birth = ?, gender = ?, current_grade_id = ?, position = ?, department = ?,
			service_status = ?, date_joined = ?, confirmed_at = ?, updated_at = ?
		 WHERE id = ?`,
		e.EmployeeNo, e.ICNumber, e.FullName, e.Email, e.Phone,
		e.DateOfBirth, e.Gender, e.CurrentGradeID, e.Position, e.Department,
		e.ServiceStatus, e.DateJoined, e.ConfirmedAt, ts, e.ID)
	if err != nil {
		return err
	}
	e.UpdatedAt = ts
	return nil
}

func (r *EmployeeRepository) UpdateGrade(ctx context.Context, id, gradeID int64) error {
	return r.execOne(ctx, "employee update grade", nil,
		"UPDATE employees SET current_grade_id = ?, updated_at = ? WHERE id = ?", gradeID, now(), id)
}

func (r *EmployeeRepository) Delete(ctx context.Context, id int64) error {
	return r.execOne(ctx, "employee delete", nil, "DELETE FROM employees WHERE id = ?", id)
}

func whereClause(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conds, " AND ")
}
