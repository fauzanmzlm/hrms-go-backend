package jwt

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// MinSecretLength is the minimum HMAC secret length accepted by NewManager.
const MinSecretLength = 32

var ErrInvalidToken = errors.New("invalid or expired token")

// Claims are the application claims carried in an access token.
type Claims struct {
	UserID     int64  `json:"uid"`
	Role       string `json:"role"`
	EmployeeID *int64 `json:"eid,omitempty"`
	gojwt.RegisteredClaims
}

// Manager issues and verifies HS256-signed access tokens.
type Manager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

func NewManager(secret, issuer string, ttl time.Duration) (*Manager, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("jwt secret must be at least %d bytes", MinSecretLength)
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be positive")
	}
	return &Manager{secret: []byte(secret), issuer: issuer, ttl: ttl, now: time.Now}, nil
}

func (m *Manager) Issue(userID int64, role string, employeeID *int64) (string, time.Time, error) {
	issuedAt := m.now().UTC()
	expiresAt := issuedAt.Add(m.ttl)
	claims := Claims{
		UserID:     userID,
		Role:       role,
		EmployeeID: employeeID,
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			Issuer:    m.issuer,
			IssuedAt:  gojwt.NewNumericDate(issuedAt),
			NotBefore: gojwt.NewNumericDate(issuedAt),
			ExpiresAt: gojwt.NewNumericDate(expiresAt),
		},
	}
	token, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return token, expiresAt, nil
}

// Parse verifies the signature, algorithm, issuer and expiry of token. Every
// failure is reported as ErrInvalidToken.
func (m *Manager) Parse(token string) (*Claims, error) {
	var claims Claims
	_, err := gojwt.ParseWithClaims(token, &claims,
		func(*gojwt.Token) (any, error) { return m.secret, nil },
		gojwt.WithValidMethods([]string{gojwt.SigningMethodHS256.Alg()}),
		gojwt.WithIssuer(m.issuer),
		gojwt.WithExpirationRequired(),
		gojwt.WithTimeFunc(m.now),
	)
	if err != nil || claims.UserID <= 0 {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}
