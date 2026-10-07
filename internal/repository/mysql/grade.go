package mysql

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

const gradeColumns = "id, code, scheme, level, title, created_at, updated_at"

var gradeKeys = uniqueKeys{"uq_grades_code": "grade code already exists"}

type GradeRepository struct {
	base
}

var _ domain.GradeRepository = (*GradeRepository)(nil)

func NewGradeRepository(db *sqlx.DB) *GradeRepository {
	return &GradeRepository{base{db: db}}
}

func (r *GradeRepository) Create(ctx context.Context, g *domain.Grade) error {
	ts := now()
	id, err := r.insert(ctx, "grade create", gradeKeys,
		`INSERT INTO grades (code, scheme, level, title, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		g.Code, g.Scheme, g.Level, g.Title, ts, ts)
	if err != nil {
		return err
	}
	g.ID, g.CreatedAt, g.UpdatedAt = id, ts, ts
	return nil
}

func (r *GradeRepository) GetByID(ctx context.Context, id int64) (*domain.Grade, error) {
	var g domain.Grade
	if err := r.get(ctx, "grade get", &g, "SELECT "+gradeColumns+" FROM grades WHERE id = ?", id); err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GradeRepository) GetByCode(ctx context.Context, code string) (*domain.Grade, error) {
	var g domain.Grade
	if err := r.get(ctx, "grade get by code", &g, "SELECT "+gradeColumns+" FROM grades WHERE code = ?", code); err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GradeRepository) List(ctx context.Context) ([]domain.Grade, error) {
	var grades []domain.Grade
	err := r.selectAll(ctx, "grade list", &grades, "SELECT "+gradeColumns+" FROM grades ORDER BY scheme, level, code")
	return grades, err
}

func (r *GradeRepository) Update(ctx context.Context, g *domain.Grade) error {
	ts := now()
	err := r.execOne(ctx, "grade update", gradeKeys,
		`UPDATE grades SET code = ?, scheme = ?, level = ?, title = ?, updated_at = ? WHERE id = ?`,
		g.Code, g.Scheme, g.Level, g.Title, ts, g.ID)
	if err != nil {
		return err
	}
	g.UpdatedAt = ts
	return nil
}

func (r *GradeRepository) Delete(ctx context.Context, id int64) error {
	return r.execOne(ctx, "grade delete", nil, "DELETE FROM grades WHERE id = ?", id)
}
