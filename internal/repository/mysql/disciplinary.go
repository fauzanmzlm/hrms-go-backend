package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

const disciplinaryColumns = `id, employee_id, case_no, category, description, incident_date, penalty, status,
	reported_by, reviewed_by, reviewed_at, review_remarks, created_at, updated_at`

var disciplinaryKeys = uniqueKeys{
	"uq_disciplinary_records_case_no": "case number already exists",
}

type DisciplinaryRepository struct {
	base
}

var _ domain.DisciplinaryRepository = (*DisciplinaryRepository)(nil)

func NewDisciplinaryRepository(db *sqlx.DB) *DisciplinaryRepository {
	return &DisciplinaryRepository{base{db: db}}
}

func (r *DisciplinaryRepository) Create(ctx context.Context, d *domain.DisciplinaryRecord) error {
	ts := now()
	id, err := r.insert(ctx, "disciplinary create", disciplinaryKeys,
		`INSERT INTO disciplinary_records (employee_id, case_no, category, description, incident_date,
			penalty, status, reported_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.EmployeeID, d.CaseNo, d.Category, d.Description, d.IncidentDate,
		d.Penalty, d.Status, d.ReportedBy, ts, ts)
	if err != nil {
		return err
	}
	d.ID, d.CreatedAt, d.UpdatedAt = id, ts, ts
	return nil
}

func (r *DisciplinaryRepository) GetByID(ctx context.Context, id int64) (*domain.DisciplinaryRecord, error) {
	var d domain.DisciplinaryRecord
	if err := r.get(ctx, "disciplinary get", &d,
		"SELECT "+disciplinaryColumns+" FROM disciplinary_records WHERE id = ?", id); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DisciplinaryRepository) List(ctx context.Context, f domain.DisciplinaryFilter) ([]domain.DisciplinaryRecord, int64, error) {
	f.Normalize()

	var conds []string
	var args []any
	if f.EmployeeID > 0 {
		conds = append(conds, "employee_id = ?")
		args = append(args, f.EmployeeID)
	}
	if f.Status != "" {
		conds = append(conds, "status = ?")
		args = append(args, f.Status)
	}
	if f.Category != "" {
		conds = append(conds, "category = ?")
		args = append(args, f.Category)
	}
	where := whereClause(conds)

	var total int64
	if err := r.get(ctx, "disciplinary count", &total,
		"SELECT COUNT(*) FROM disciplinary_records"+where, args...); err != nil {
		return nil, 0, err
	}
	var items []domain.DisciplinaryRecord
	err := r.selectAll(ctx, "disciplinary list", &items,
		"SELECT "+disciplinaryColumns+" FROM disciplinary_records"+where+
			" ORDER BY incident_date DESC, id DESC LIMIT ? OFFSET ?",
		append(args, f.Limit(), f.Offset())...)
	return items, total, err
}

func (r *DisciplinaryRepository) UpdateReview(ctx context.Context, d *domain.DisciplinaryRecord) error {
	ts := now()
	n, err := r.exec(ctx, "disciplinary update review", nil,
		`UPDATE disciplinary_records
		 SET status = ?, reviewed_by = ?, reviewed_at = ?, review_remarks = ?, updated_at = ?
		 WHERE id = ? AND status = ?`,
		d.Status, d.ReviewedBy, d.ReviewedAt, d.ReviewRemarks, ts,
		d.ID, domain.StatusPending)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: disciplinary record %d is no longer pending", domain.ErrInvalidStatusTransition, d.ID)
	}
	d.UpdatedAt = ts
	return nil
}

func (r *DisciplinaryRepository) HasApprovedSince(ctx context.Context, employeeID int64, since time.Time) (bool, error) {
	var exists bool
	err := r.get(ctx, "disciplinary has approved since", &exists,
		`SELECT EXISTS(SELECT 1 FROM disciplinary_records
		 WHERE employee_id = ? AND status = ? AND reviewed_at >= ?)`,
		employeeID, domain.StatusApproved, since)
	return exists, err
}
