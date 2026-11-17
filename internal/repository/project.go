package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"ledger/internal/domain"
)

// CreateProject inserts a new project and returns its generated id.
func CreateProject(ctx context.Context, q Querier, p *domain.Project) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO projects (name, description, context, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		p.Name, p.Description, p.Context, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert project: %w", err)
	}
	return res.LastInsertId()
}

// GetProject returns the project with id, or domain.ErrNotFound.
func GetProject(ctx context.Context, q Querier, id int64) (*domain.Project, error) {
	p := &domain.Project{}
	err := q.QueryRowContext(ctx, `
		SELECT id, name, description, context, created_at, updated_at
		FROM projects WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Description, &p.Context, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	return p, nil
}

// ListProjects returns all projects ordered by most recently updated.
func ListProjects(ctx context.Context, q Querier) ([]domain.Project, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, name, description, context, created_at, updated_at
		FROM projects ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var out []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Context, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateProject persists all mutable fields of p and stamps updatedAt.
func UpdateProject(ctx context.Context, q Querier, p *domain.Project, updatedAt string) error {
	_, err := q.ExecContext(ctx, `
		UPDATE projects
		SET name = ?, description = ?, context = ?, updated_at = ?
		WHERE id = ?`,
		p.Name, p.Description, p.Context, updatedAt, p.ID)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

// CreateLabel inserts a label into a project.
func CreateLabel(ctx context.Context, q Querier, l *domain.Label) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO labels (project_id, name, color) VALUES (?, ?, ?)`,
		l.ProjectID, l.Name, l.Color)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, fmt.Errorf("%w: label %q already exists in project %d", domain.ErrConflict, l.Name, l.ProjectID)
		}
		return 0, fmt.Errorf("insert label: %w", err)
	}
	return res.LastInsertId()
}

// ListLabels returns all labels of a project ordered by name.
func ListLabels(ctx context.Context, q Querier, projectID int64) ([]domain.Label, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, project_id, name, color FROM labels
		WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()

	var out []domain.Label
	for rows.Next() {
		var l domain.Label
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color); err != nil {
			return nil, fmt.Errorf("scan label: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetLabel returns one label by id, or domain.ErrNotFound.
func GetLabel(ctx context.Context, q Querier, id int64) (*domain.Label, error) {
	l := &domain.Label{}
	err := q.QueryRowContext(ctx, `
		SELECT id, project_id, name, color FROM labels WHERE id = ?`, id).
		Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get label: %w", err)
	}
	return l, nil
}

// DeleteLabel removes a label and cascades its issue links.
func DeleteLabel(ctx context.Context, q Querier, id int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM labels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete label: %w", err)
	}
	return nil
}
