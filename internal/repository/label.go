package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"ledger/internal/domain"
)

// GetLabelByName returns the label with name in a project, or domain.ErrNotFound.
func GetLabelByName(ctx context.Context, q Querier, projectID int64, name string) (*domain.Label, error) {
	l := &domain.Label{}
	err := q.QueryRowContext(ctx, `
		SELECT id, project_id, name, color FROM labels
		WHERE project_id = ? AND name = ?`, projectID, name).
		Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get label by name: %w", err)
	}
	return l, nil
}

// EnsureLabel returns the existing label with name, creating it if missing.
func EnsureLabel(ctx context.Context, q Querier, projectID int64, name string) (*domain.Label, error) {
	if l, err := GetLabelByName(ctx, q, projectID, name); err == nil {
		return l, nil
	} else if err != domain.ErrNotFound {
		return nil, err
	}
	id, err := CreateLabel(ctx, q, &domain.Label{ProjectID: projectID, Name: name})
	if err != nil {
		// Another writer may have created it concurrently.
		if strings.Contains(err.Error(), "already exists") {
			return GetLabelByName(ctx, q, projectID, name)
		}
		return nil, err
	}
	return &domain.Label{ID: id, ProjectID: projectID, Name: name}, nil
}
