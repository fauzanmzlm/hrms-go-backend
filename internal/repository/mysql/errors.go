package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

// MySQL server error numbers.
const (
	errDupEntry            = 1062
	errRowIsReferenced     = 1217
	errNoReferencedRow     = 1216
	errRowIsReferenced2    = 1451
	errNoReferencedRow2    = 1452
	errCheckConstraintFail = 3819
)

// uniqueKeys maps a unique index name to the client-facing conflict message.
type uniqueKeys map[string]string

// mapError translates driver errors into domain errors. Errors with no domain
// meaning are wrapped with op for context.
func mapError(op string, err error, keys uniqueKeys) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}

	var me *mysql.MySQLError
	if errors.As(err, &me) {
		switch me.Number {
		case errDupEntry:
			if msg, ok := keys[duplicateKeyName(me.Message)]; ok {
				return fmt.Errorf("%w: %s", domain.ErrConflict, msg)
			}
			return domain.ErrConflict
		case errRowIsReferenced, errRowIsReferenced2:
			return fmt.Errorf("%w: record is still referenced by other records", domain.ErrConflict)
		case errNoReferencedRow, errNoReferencedRow2:
			return fmt.Errorf("%w: referenced record does not exist", domain.ErrInvalidInput)
		case errCheckConstraintFail:
			return fmt.Errorf("%w: %s", domain.ErrInvalidInput, me.Message)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

// duplicateKeyName extracts the index name from a message such as
// "Duplicate entry 'x' for key 'employees.uq_employees_email'".
func duplicateKeyName(msg string) string {
	const marker = "for key '"
	i := strings.LastIndex(msg, marker)
	if i < 0 {
		return ""
	}
	key := strings.TrimSuffix(msg[i+len(marker):], "'")
	if dot := strings.LastIndexByte(key, '.'); dot >= 0 {
		key = key[dot+1:]
	}
	return key
}

// likeContains builds a LIKE pattern matching s anywhere, with LIKE
// wildcards in s escaped.
func likeContains(s string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s) + "%"
}
