package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

const migrationLock = "hrms_schema_migrations"

// Migrate applies every *.up.sql file in fsys that is not yet recorded in
// schema_migrations, in lexical order. A MySQL named lock serialises
// concurrent instances. MySQL DDL auto-commits, so a file that fails halfway
// stays partially applied; migration files must therefore be idempotent
// (CREATE TABLE IF NOT EXISTS, etc.).
func Migrate(ctx context.Context, db *sqlx.DB, fsys fs.FS, log *slog.Logger) error {
	conn, err := db.Connx(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire conn: %w", err)
	}
	defer conn.Close()

	var locked sql.NullInt64
	if err := conn.GetContext(ctx, &locked, "SELECT GET_LOCK(?, 60)", migrationLock); err != nil {
		return fmt.Errorf("migrate: get lock: %w", err)
	}
	if !locked.Valid || locked.Int64 != 1 {
		return fmt.Errorf("migrate: could not acquire lock %q", migrationLock)
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), "SELECT RELEASE_LOCK(?)", migrationLock)
	}()

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    VARCHAR(255) NOT NULL PRIMARY KEY,
		applied_at DATETIME(6)  NOT NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return fmt.Errorf("migrate: create schema_migrations: %w", err)
	}

	var versions []string
	if err := conn.SelectContext(ctx, &versions, "SELECT version FROM schema_migrations"); err != nil {
		return fmt.Errorf("migrate: list applied: %w", err)
	}
	applied := make(map[string]bool, len(versions))
	for _, v := range versions {
		applied[v] = true
	}

	files, err := fs.Glob(fsys, "*.up.sql")
	if err != nil {
		return fmt.Errorf("migrate: list files: %w", err)
	}
	sort.Strings(files)

	for _, name := range files {
		version := strings.TrimSuffix(name, ".up.sql")
		if applied[version] {
			continue
		}
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", name, err)
		}
		for _, stmt := range splitStatements(string(content)) {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migrate: %s: %w", name, err)
			}
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", version, now()); err != nil {
			return fmt.Errorf("migrate: record %s: %w", name, err)
		}
		log.Info("migration applied", slog.String("version", version))
	}
	return nil
}

// splitStatements splits a SQL script on semicolons after removing "--" line
// comments. Statements must not contain semicolons inside string literals.
func splitStatements(script string) []string {
	var b strings.Builder
	for line := range strings.Lines(script) {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		b.WriteString(line)
	}

	var stmts []string
	for stmt := range strings.SplitSeq(b.String(), ";") {
		if s := strings.TrimSpace(stmt); s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}
