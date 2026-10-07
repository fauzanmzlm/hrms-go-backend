package domain

import (
	"context"
	"slices"
	"strings"
	"time"
)

type Role string

const (
	RoleAdmin     Role = "ADMIN"
	RoleHROfficer Role = "HR_OFFICER"
	RoleApprover  Role = "APPROVER"
	RoleEmployee  Role = "EMPLOYEE"
)

func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleHROfficer, RoleApprover, RoleEmployee:
		return true
	}
	return false
}

// User is a login account. EmployeeID links the account to a personnel
// record and is nil for system accounts.
type User struct {
	ID           int64     `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Role         Role      `db:"role" json:"role"`
	EmployeeID   *int64    `db:"employee_id" json:"employee_id,omitempty"`
	IsActive     bool      `db:"is_active" json:"is_active"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

func (u *User) HasRole(roles ...Role) bool {
	return slices.Contains(roles, u.Role)
}

func (u *User) Validate() error {
	v := NewValidationError()
	if n := len(strings.TrimSpace(u.Username)); n < 3 || n > 50 {
		v.Add("username", "must be 3 to 50 characters")
	}
	if u.PasswordHash == "" {
		v.Add("password", "is required")
	}
	if !u.Role.IsValid() {
		v.Add("role", "is invalid")
	}
	if u.EmployeeID != nil && *u.EmployeeID <= 0 {
		v.Add("employee_id", "is invalid")
	} else if u.EmployeeID == nil && u.Role == RoleEmployee {
		v.Add("employee_id", "is required for the EMPLOYEE role")
	}
	return v.Err()
}

// UserRepository persists login accounts. Lookups return ErrNotFound when no
// row matches, and Create returns ErrConflict on a duplicate username.
type UserRepository interface {
	// Create inserts u and sets u.ID.
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
}
