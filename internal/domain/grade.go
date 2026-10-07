package domain

import (
	"context"
	"strings"
	"time"
)

// Grade is a service grade (gred), e.g. N41 or DG44 in scheme N or DG.
type Grade struct {
	ID        int64     `db:"id" json:"id"`
	Code      string    `db:"code" json:"code"`
	Scheme    string    `db:"scheme" json:"scheme"`
	Level     int       `db:"level" json:"level"`
	Title     string    `db:"title" json:"title"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (g *Grade) Validate() error {
	v := NewValidationError()
	if strings.TrimSpace(g.Code) == "" {
		v.Add("code", "is required")
	} else if len(g.Code) > 10 {
		v.Add("code", "must be at most 10 characters")
	}
	if strings.TrimSpace(g.Scheme) == "" {
		v.Add("scheme", "is required")
	}
	if g.Level <= 0 {
		v.Add("level", "must be greater than 0")
	}
	if strings.TrimSpace(g.Title) == "" {
		v.Add("title", "is required")
	}
	return v.Err()
}

// IsHigherThan reports whether g ranks above other.
func (g *Grade) IsHigherThan(other *Grade) bool {
	return g.Level > other.Level
}

// GradeRepository persists grades. Lookups return ErrNotFound when no row
// matches, and writes return ErrConflict on a duplicate code.
type GradeRepository interface {
	// Create inserts g and sets g.ID.
	Create(ctx context.Context, g *Grade) error
	GetByID(ctx context.Context, id int64) (*Grade, error)
	GetByCode(ctx context.Context, code string) (*Grade, error)
	// List returns all grades ordered by scheme and level.
	List(ctx context.Context) ([]Grade, error)
	Update(ctx context.Context, g *Grade) error
	Delete(ctx context.Context, id int64) error
}
