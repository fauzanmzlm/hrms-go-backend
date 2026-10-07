package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 72
)

var errInvalidCredentials = fmt.Errorf("%w: invalid username or password", domain.ErrUnauthorized)

type AuthUsecase struct {
	users     domain.UserRepository
	employees domain.EmployeeRepository
	hasher    PasswordHasher
	tokens    TokenIssuer
	// dummyHash is compared against when the username is unknown, so a login
	// for a missing user takes as long as one with a wrong password.
	dummyHash string
}

var _ domain.AuthUsecase = (*AuthUsecase)(nil)

func NewAuthUsecase(users domain.UserRepository, employees domain.EmployeeRepository, hasher PasswordHasher, tokens TokenIssuer) (*AuthUsecase, error) {
	dummy, err := hasher.Hash("timing-equaliser-password")
	if err != nil {
		return nil, fmt.Errorf("auth usecase: %w", err)
	}
	return &AuthUsecase{users: users, employees: employees, hasher: hasher, tokens: tokens, dummyHash: dummy}, nil
}

func (a *AuthUsecase) Login(ctx context.Context, username, password string) (*domain.LoginResult, error) {
	u, err := a.users.GetByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, domain.ErrNotFound) {
		_ = a.hasher.Compare(a.dummyHash, password)
		return nil, errInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if err := a.hasher.Compare(u.PasswordHash, password); err != nil || !u.IsActive {
		return nil, errInvalidCredentials
	}

	token, expiresAt, err := a.tokens.Issue(u.ID, string(u.Role), u.EmployeeID)
	if err != nil {
		return nil, err
	}
	return &domain.LoginResult{AccessToken: token, TokenType: "Bearer", ExpiresAt: expiresAt, User: u}, nil
}

func (a *AuthUsecase) CreateUser(ctx context.Context, u *domain.User, password string) error {
	trim(&u.Username)
	if n := len(password); n < minPasswordLength || n > maxPasswordLength {
		return validationErr("password", fmt.Sprintf("must be %d to %d bytes", minPasswordLength, maxPasswordLength))
	}
	if u.EmployeeID != nil {
		if _, err := a.employees.GetByID(ctx, *u.EmployeeID); err != nil {
			return fieldNotFound(err, "employee_id")
		}
	}

	hash, err := a.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	u.PasswordHash = hash
	u.IsActive = true
	if err := u.Validate(); err != nil {
		return err
	}
	return a.users.Create(ctx, u)
}

func (a *AuthUsecase) GetUser(ctx context.Context, id int64) (*domain.User, error) {
	return a.users.GetByID(ctx, id)
}
