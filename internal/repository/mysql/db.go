package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Open connects to MySQL and verifies the connection. parseTime and UTC
// location are forced regardless of the DSN because the repositories scan
// DATE/DATETIME columns into time.Time and store all times in UTC.
func Open(ctx context.Context, cfg Config) (*sqlx.DB, error) {
	mc, err := mysql.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse mysql dsn: %w", err)
	}
	mc.ParseTime = true
	mc.Loc = time.UTC

	connector, err := mysql.NewConnector(mc)
	if err != nil {
		return nil, fmt.Errorf("mysql connector: %w", err)
	}
	db := sqlx.NewDb(sql.OpenDB(connector), "mysql")
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}

type txKey struct{}

// Transactor implements domain.Transactor. Repositories pick up the
// transaction from the context, so nested WithinTx calls join the outer one.
type Transactor struct {
	db *sqlx.DB
}

var _ domain.Transactor = (*Transactor)(nil)

func NewTransactor(db *sqlx.DB) *Transactor {
	return &Transactor{db: db}
}

func (t *Transactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*sqlx.Tx); ok {
		return fn(ctx)
	}

	tx, err := t.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rollback tx: %w", rbErr))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// base holds the query helpers shared by every repository.
type base struct {
	db *sqlx.DB
}

func (b base) q(ctx context.Context) sqlx.ExtContext {
	if tx, ok := ctx.Value(txKey{}).(*sqlx.Tx); ok {
		return tx
	}
	return b.db
}

func (b base) get(ctx context.Context, op string, dest any, query string, args ...any) error {
	return mapError(op, sqlx.GetContext(ctx, b.q(ctx), dest, query, args...), nil)
}

func (b base) selectAll(ctx context.Context, op string, dest any, query string, args ...any) error {
	return mapError(op, sqlx.SelectContext(ctx, b.q(ctx), dest, query, args...), nil)
}

func (b base) insert(ctx context.Context, op string, keys uniqueKeys, query string, args ...any) (int64, error) {
	res, err := b.q(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return 0, mapError(op, err, keys)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: last insert id: %w", op, err)
	}
	return id, nil
}

// exec runs a write and returns the number of affected rows.
func (b base) exec(ctx context.Context, op string, keys uniqueKeys, query string, args ...any) (int64, error) {
	res, err := b.q(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return 0, mapError(op, err, keys)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: rows affected: %w", op, err)
	}
	return n, nil
}

// execOne runs a write that must touch exactly one row and returns
// domain.ErrNotFound otherwise. Every update also sets updated_at, so an
// unchanged row still counts as affected.
func (b base) execOne(ctx context.Context, op string, keys uniqueKeys, query string, args ...any) error {
	n, err := b.exec(ctx, op, keys, query, args...)
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}
