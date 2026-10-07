package domain

import (
	"context"
	"time"
)

// LoginResult is returned by a successful login.
type LoginResult struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	User        *User     `json:"user"`
}

type AuthUsecase interface {
	// Login returns ErrUnauthorized for an unknown user, wrong password or
	// inactive account, without revealing which.
	Login(ctx context.Context, username, password string) (*LoginResult, error)
	// CreateUser hashes password, stores it on u and inserts u as an active
	// account.
	CreateUser(ctx context.Context, u *User, password string) error
	GetUser(ctx context.Context, id int64) (*User, error)
}

type GradeUsecase interface {
	Create(ctx context.Context, g *Grade) error
	GetByID(ctx context.Context, id int64) (*Grade, error)
	List(ctx context.Context) ([]Grade, error)
	Update(ctx context.Context, g *Grade) error
	Delete(ctx context.Context, id int64) error
}

type EmployeeUsecase interface {
	Create(ctx context.Context, e *Employee) error
	GetByID(ctx context.Context, id int64) (*Employee, error)
	List(ctx context.Context, f EmployeeFilter) (Page[Employee], error)
	Update(ctx context.Context, e *Employee) error
	Delete(ctx context.Context, id int64) error
}

type ServiceRecordUsecase interface {
	// Create inserts r. When r is current, the employee's previous current
	// record is closed the day before r.EffectiveFrom.
	Create(ctx context.Context, r *ServiceRecord) error
	GetByID(ctx context.Context, id int64) (*ServiceRecord, error)
	ListByEmployee(ctx context.Context, employeeID int64) ([]ServiceRecord, error)
	Update(ctx context.Context, r *ServiceRecord) error
	Delete(ctx context.Context, id int64) error
}

// SubmitPromotionInput is a new promotion application. The current grade is
// taken from the employee record.
type SubmitPromotionInput struct {
	EmployeeID      int64  `json:"employee_id"`
	ProposedGradeID int64  `json:"proposed_grade_id"`
	Justification   string `json:"justification"`
}

type PromotionUsecase interface {
	Submit(ctx context.Context, in SubmitPromotionInput) (*PromotionApplication, error)
	// Review approves or rejects a pending application. On approval the
	// employee's grade is updated and a PROMOTION service record starting at
	// effectiveDate is created in the same transaction. effectiveDate is
	// ignored when rejecting.
	Review(ctx context.Context, id int64, d ReviewDecision, effectiveDate time.Time) (*PromotionApplication, error)
	Get(ctx context.Context, id int64) (*PromotionApplication, error)
	List(ctx context.Context, f PromotionFilter) (Page[PromotionApplication], error)
}

type DisciplinaryUsecase interface {
	Create(ctx context.Context, d *DisciplinaryRecord) error
	Review(ctx context.Context, id int64, dec ReviewDecision) (*DisciplinaryRecord, error)
	Get(ctx context.Context, id int64) (*DisciplinaryRecord, error)
	List(ctx context.Context, f DisciplinaryFilter) (Page[DisciplinaryRecord], error)
}
