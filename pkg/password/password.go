// Package password hashes and verifies passwords with bcrypt.
package password

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// MaxLength is bcrypt's input limit in bytes.
const MaxLength = 72

var ErrMismatch = errors.New("password does not match")

type Bcrypt struct {
	cost int
}

// NewBcrypt returns a hasher using cost, or bcrypt.DefaultCost when cost is
// outside bcrypt's allowed range.
func NewBcrypt(cost int) *Bcrypt {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &Bcrypt{cost: cost}
}

func (b *Bcrypt) Hash(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), b.cost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func (b *Bcrypt) Compare(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return ErrMismatch
	}
	return nil
}
