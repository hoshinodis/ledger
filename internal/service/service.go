// Package service implements the business rules shared by REST, MCP and CLI.
package service

import (
	"context"
	"database/sql"

	"ledger/internal/repository"
)

// Service holds all business operations. REST handlers, MCP tools and CLI
// commands all call into it, so rules live in exactly one place.
type Service struct {
	DB *sql.DB
	// Now returns the canonical timestamp; a variable to allow clock control
	// in tests.
	Now func() string
}

// New creates a Service for db.
func New(db *sql.DB) *Service {
	return &Service{DB: db, Now: repository.Now}
}

// Tx runs fn inside a database transaction, rolling back on error.
func (s *Service) Tx(ctx context.Context, fn func(ctx context.Context, q repository.Querier) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
