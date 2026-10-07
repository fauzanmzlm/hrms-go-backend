package mysql

import (
	"errors"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

func TestMapErrorDuplicateKey(t *testing.T) {
	err := mapError("op", &mysql.MySQLError{
		Number:  errDupEntry,
		Message: "Duplicate entry 'uq_employees_email' for key 'employees.uq_employees_ic_number'",
	}, employeeKeys)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	if want := "resource already exists: IC number already exists"; err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
}

func TestMapErrorForeignKeys(t *testing.T) {
	if err := mapError("op", &mysql.MySQLError{Number: errRowIsReferenced2}, nil); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("1451 err = %v, want ErrConflict", err)
	}
	if err := mapError("op", &mysql.MySQLError{Number: errNoReferencedRow2}, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("1452 err = %v, want ErrInvalidInput", err)
	}
}

func TestLikeContains(t *testing.T) {
	if got, want := likeContains(`50%_a\b`), `%50\%\_a\\b%`; got != want {
		t.Errorf("likeContains = %q, want %q", got, want)
	}
}

func TestSplitStatementsOnInitMigration(t *testing.T) {
	script, err := os.ReadFile("../../../migrations/0001_init.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	stmts := splitStatements(string(script))
	if len(stmts) != 6 {
		t.Fatalf("want 6 CREATE TABLE statements, got %d", len(stmts))
	}
	for _, s := range stmts {
		if len(s) < 12 || s[:12] != "CREATE TABLE" {
			t.Errorf("unexpected statement start: %.40q", s)
		}
	}
}
