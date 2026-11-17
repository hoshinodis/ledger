package repository

import (
	"context"
	"fmt"

	"ledger/internal/domain"
)

// CreateComment inserts a comment on an issue and returns its id.
func CreateComment(ctx context.Context, q Querier, c *domain.Comment) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO comments (issue_id, author, session_id, body, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		c.IssueID, c.Author, c.SessionID, c.Body, c.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert comment: %w", err)
	}
	return res.LastInsertId()
}

// ListCommentsByIssue returns all comments of an issue, oldest first.
func ListCommentsByIssue(ctx context.Context, q Querier, issueID int64) ([]domain.Comment, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, issue_id, author, COALESCE(session_id, ''), body, created_at
		FROM comments WHERE issue_id = ? ORDER BY created_at, id`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var out []domain.Comment
	for rows.Next() {
		var c domain.Comment
		if err := rows.Scan(&c.ID, &c.IssueID, &c.Author, &c.SessionID, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListRecentCommentsByProject returns the newest comments across all issues
// of a project, newest first.
func ListRecentCommentsByProject(ctx context.Context, q Querier, projectID int64, limit int) ([]domain.Comment, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT c.id, c.issue_id, c.author, COALESCE(c.session_id, ''), c.body, c.created_at
		FROM comments c JOIN issues i ON i.id = c.issue_id
		WHERE i.project_id = ?
		ORDER BY c.created_at DESC, c.id DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent comments: %w", err)
	}
	defer rows.Close()

	var out []domain.Comment
	for rows.Next() {
		var c domain.Comment
		if err := rows.Scan(&c.ID, &c.IssueID, &c.Author, &c.SessionID, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
