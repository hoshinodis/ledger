// Package repository implements SQLite persistence for ledger.
package repository

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	"ledger/internal/domain"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Querier abstracts over *sql.DB and *sql.Tx so repository methods work
// inside transactions managed by the service layer.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Open opens the SQLite database at path with WAL journaling, foreign keys
// enforced and a busy timeout for concurrent access.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc/sqlite serializes access internally; a single connection keeps
	// writes safe and avoids SQLITE_BUSY under concurrent handlers.
	db.SetMaxOpenConns(1)
	return db, nil
}

// Migrate applies the embedded schema. Safe to run on every startup.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Now returns the canonical timestamp format used across the store
// (RFC3339, UTC).
func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func notFound(err error) error {
	if err == sql.ErrNoRows {
		return domain.ErrNotFound
	}
	return err
}

// orNull maps an empty string to SQL NULL.
func orNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}
