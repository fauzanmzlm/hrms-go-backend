package usecase

import (
	"context"
	"strings"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

type GradeUsecase struct {
	grades domain.GradeRepository
}

var _ domain.GradeUsecase = (*GradeUsecase)(nil)

func NewGradeUsecase(grades domain.GradeRepository) *GradeUsecase {
	return &GradeUsecase{grades: grades}
}

func normalizeGrade(g *domain.Grade) {
	g.Code = strings.ToUpper(strings.TrimSpace(g.Code))
	g.Scheme = strings.ToUpper(strings.TrimSpace(g.Scheme))
	trim(&g.Title)
}

func (u *GradeUsecase) Create(ctx context.Context, g *domain.Grade) error {
	normalizeGrade(g)
	if err := g.Validate(); err != nil {
		return err
	}
	return u.grades.Create(ctx, g)
}

func (u *GradeUsecase) GetByID(ctx context.Context, id int64) (*domain.Grade, error) {
	return u.grades.GetByID(ctx, id)
}

func (u *GradeUsecase) List(ctx context.Context) ([]domain.Grade, error) {
	grades, err := u.grades.List(ctx)
	if grades == nil && err == nil {
		grades = []domain.Grade{}
	}
	return grades, err
}

func (u *GradeUsecase) Update(ctx context.Context, g *domain.Grade) error {
	existing, err := u.grades.GetByID(ctx, g.ID)
	if err != nil {
		return err
	}
	normalizeGrade(g)
	if err := g.Validate(); err != nil {
		return err
	}
	g.CreatedAt = existing.CreatedAt
	return u.grades.Update(ctx, g)
}

func (u *GradeUsecase) Delete(ctx context.Context, id int64) error {
	return u.grades.Delete(ctx, id)
}
