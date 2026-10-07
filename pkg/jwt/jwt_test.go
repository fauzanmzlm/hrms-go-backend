package jwt

import (
	"errors"
	"testing"
	"time"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestIssueAndParse(t *testing.T) {
	m, err := NewManager(secret, "hrms", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	empID := int64(42)
	tok, exp, err := m.Issue(7, "APPROVER", &empID)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(exp) <= 0 {
		t.Errorf("expiry %v is not in the future", exp)
	}

	c, err := m.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != 7 || c.Role != "APPROVER" || c.EmployeeID == nil || *c.EmployeeID != 42 || c.Subject != "7" {
		t.Errorf("claims = %+v", c)
	}
}

func TestParseRejects(t *testing.T) {
	m, _ := NewManager(secret, "hrms", time.Hour)
	tok, _, _ := m.Issue(7, "ADMIN", nil)

	other, _ := NewManager("ffffffffffffffffffffffffffffffff", "hrms", time.Hour)
	otherIssuer, _ := NewManager(secret, "someone-else", time.Hour)
	expired, _ := NewManager(secret, "hrms", time.Hour)
	expired.now = func() time.Time { return time.Now().Add(2 * time.Hour) }

	for name, parser := range map[string]*Manager{
		"wrong secret": other, "wrong issuer": otherIssuer, "expired": expired,
	} {
		if _, err := parser.Parse(tok); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: err = %v, want ErrInvalidToken", name, err)
		}
	}
	if _, err := m.Parse(tok + "x"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("tampered: err = %v", err)
	}
}

func TestNewManagerRejectsShortSecret(t *testing.T) {
	if _, err := NewManager("short", "hrms", time.Hour); err == nil {
		t.Error("expected error for short secret")
	}
}
