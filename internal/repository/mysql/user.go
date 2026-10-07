package mysql

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

const userColumns = "id, username, password_hash, role, employee_id, is_active, created_at, updated_at"

var userKeys = uniqueKeys{
	"uq_users_username": "username already exists",
	"uq_users_employee": "employee already has a user account",
}

type UserRepository struct {
	base
}

var _ domain.UserRepository = (*UserRepository)(nil)

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{base{db: db}}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	ts := now()
	id, err := r.insert(ctx, "user create", userKeys,
		`INSERT INTO users (username, password_hash, role, employee_id, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.Role, u.EmployeeID, u.IsActive, ts, ts)
	if err != nil {
		return err
	}
	u.ID, u.CreatedAt, u.UpdatedAt = id, ts, ts
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	var u domain.User
	if err := r.get(ctx, "user get", &u, "SELECT "+userColumns+" FROM users WHERE id = ?", id); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var u domain.User
	if err := r.get(ctx, "user get by username", &u,
		"SELECT "+userColumns+" FROM users WHERE username = ?", username); err != nil {
		return nil, err
	}
	return &u, nil
}
