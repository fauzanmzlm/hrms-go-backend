package mysql

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

const promotionColumns = `id, employee_id, current_grade_id, proposed_grade_id, justification, status,
	submitted_at, reviewed_by, reviewed_at, review_remarks, effective_date, created_at, updated_at`

var promotionKeys = uniqueKeys{
	"uq_promotion_applications_pending": "employee already has a pending promotion application",
}

type PromotionRepository struct {
	base
}

var _ domain.PromotionRepository = (*PromotionRepository)(nil)

func NewPromotionRepository(db *sqlx.DB) *PromotionRepository {
	return &PromotionRepository{base{db: db}}
}

func (r *PromotionRepository) Create(ctx context.Context, p *domain.PromotionApplication) error {
	ts := now()
	id, err := r.insert(ctx, "promotion create", promotionKeys,
		`INSERT INTO promotion_applications (employee_id, current_grade_id, proposed_grade_id,
			justification, status, submitted_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.EmployeeID, p.CurrentGradeID, p.ProposedGradeID,
		p.Justification, p.Status, p.SubmittedAt, ts, ts)
	if err != nil {
		return err
	}
	p.ID, p.CreatedAt, p.UpdatedAt = id, ts, ts
	return nil
}

func (r *PromotionRepository) GetByID(ctx context.Context, id int64) (*domain.PromotionApplication, error) {
	var p domain.PromotionApplication
	if err := r.get(ctx, "promotion get", &p,
		"SELECT "+promotionColumns+" FROM promotion_applications WHERE id = ?", id); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PromotionRepository) List(ctx context.Context, f domain.PromotionFilter) ([]domain.PromotionApplication, int64, error) {
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
	where := whereClause(conds)

	var total int64
	if err := r.get(ctx, "promotion count", &total,
		"SELECT COUNT(*) FROM promotion_applications"+where, args...); err != nil {
		return nil, 0, err
	}
	var items []domain.PromotionApplication
	err := r.selectAll(ctx, "promotion list", &items,
		"SELECT "+promotionColumns+" FROM promotion_applications"+where+
			" ORDER BY submitted_at DESC, id DESC LIMIT ? OFFSET ?",
		append(args, f.Limit(), f.Offset())...)
	return items, total, err
}

func (r *PromotionRepository) HasPendingByEmployee(ctx context.Context, employeeID int64) (bool, error) {
	var exists bool
	err := r.get(ctx, "promotion has pending", &exists,
		"SELECT EXISTS(SELECT 1 FROM promotion_applications WHERE employee_id = ? AND status = ?)",
		employeeID, domain.StatusPending)
	return exists, err
}

func (r *PromotionRepository) UpdateReview(ctx context.Context, p *domain.PromotionApplication) error {
	ts := now()
	n, err := r.exec(ctx, "promotion update review", nil,
		`UPDATE promotion_applications
		 SET status = ?, reviewed_by = ?, reviewed_at = ?, review_remarks = ?, effective_date = ?, updated_at = ?
		 WHERE id = ? AND status = ?`,
		p.Status, p.ReviewedBy, p.ReviewedAt, p.ReviewRemarks, p.EffectiveDate, ts,
		p.ID, domain.StatusPending)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: promotion application %d is no longer pending", domain.ErrInvalidStatusTransition, p.ID)
	}
	p.UpdatedAt = ts
	return nil
}
