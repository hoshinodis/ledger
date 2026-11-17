package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"ledger/internal/domain"
)

// IssueFilter narrows ListIssues queries. Zero values mean "no filter".
type IssueFilter struct {
	ProjectID         int64
	Statuses          []domain.Status
	Priority          domain.Priority
	AssigneeSessionID string   // exact match when non-empty
	UnassignedOnly    bool     // assignee_session_id IS NULL
	LabelName         string
	Query             string   // LIKE match on title and body
	Limit             int      // <= 0 means no limit
	OrderBy           string   // defaults to updated_at DESC
}

// CreateIssue inserts an issue and returns its generated id.
func CreateIssue(ctx context.Context, q Querier, i *domain.Issue) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO issues (project_id, title, body, status, priority, assignee_session_id, created_at, updated_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		i.ProjectID, i.Title, i.Body, i.Status, i.Priority, orNull(i.AssigneeSessionID), i.CreatedAt, i.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert issue: %w", err)
	}
	return res.LastInsertId()
}

// GetIssue returns the issue with id (without labels), or domain.ErrNotFound.
func GetIssue(ctx context.Context, q Querier, id int64) (*domain.Issue, error) {
	i := &domain.Issue{}
	var assignee, completed sql.NullString
	err := q.QueryRowContext(ctx, `
		SELECT id, project_id, title, body, status, priority, assignee_session_id,
		       created_at, updated_at, completed_at
		FROM issues WHERE id = ?`, id).
		Scan(&i.ID, &i.ProjectID, &i.Title, &i.Body, &i.Status, &i.Priority,
			&assignee, &i.CreatedAt, &i.UpdatedAt, &completed)
	if err != nil {
		return nil, notFound(err)
	}
	i.AssigneeSessionID = assignee.String
	i.CompletedAt = completed.String
	return i, nil
}

// ListIssues returns issues matching the filter.
func ListIssues(ctx context.Context, q Querier, f IssueFilter) ([]domain.Issue, error) {
	var conds []string
	var args []any

	if f.ProjectID != 0 {
		conds = append(conds, "project_id = ?")
		args = append(args, f.ProjectID)
	}
	if len(f.Statuses) > 0 {
		ph := make([]string, len(f.Statuses))
		for j, s := range f.Statuses {
			ph[j] = "?"
			args = append(args, string(s))
		}
		conds = append(conds, "status IN ("+strings.Join(ph, ",")+")")
	}
	if f.Priority != "" {
		conds = append(conds, "priority = ?")
		args = append(args, string(f.Priority))
	}
	if f.UnassignedOnly {
		conds = append(conds, "assignee_session_id IS NULL")
	} else if f.AssigneeSessionID != "" {
		conds = append(conds, "assignee_session_id = ?")
		args = append(args, f.AssigneeSessionID)
	}
	if f.LabelName != "" {
		conds = append(conds, "EXISTS (SELECT 1 FROM issue_labels il JOIN labels l ON l.id = il.label_id WHERE il.issue_id = issues.id AND l.name = ?)")
		args = append(args, f.LabelName)
	}
	if f.Query != "" {
		conds = append(conds, "(title LIKE ? ESCAPE '\\' OR body LIKE ? ESCAPE '\\')")
		esc := strings.ReplaceAll(f.Query, "\\", "\\\\")
		esc = strings.ReplaceAll(esc, "%", "\\%")
		esc = strings.ReplaceAll(esc, "_", "\\_")
		like := "%" + esc + "%"
		args = append(args, like, like)
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	order := f.OrderBy
	if order == "" {
		order = "updated_at DESC"
	}
	query := fmt.Sprintf(`
		SELECT id, project_id, title, body, status, priority,
		       COALESCE(assignee_session_id, ''), created_at, updated_at, COALESCE(completed_at, '')
		FROM issues %s ORDER BY %s, id DESC`, where, order)
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var out []domain.Issue
	for rows.Next() {
		var i domain.Issue
		if err := rows.Scan(&i.ID, &i.ProjectID, &i.Title, &i.Body, &i.Status, &i.Priority,
			&i.AssigneeSessionID, &i.CreatedAt, &i.UpdatedAt, &i.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// UpdateIssue persists all mutable fields of i and stamps updatedAt.
// Status transitions are validated in the service layer, not here.
func UpdateIssue(ctx context.Context, q Querier, i *domain.Issue, updatedAt string) error {
	_, err := q.ExecContext(ctx, `
		UPDATE issues
		SET title = ?, body = ?, status = ?, priority = ?, assignee_session_id = ?,
		    updated_at = ?, completed_at = ?
		WHERE id = ?`,
		i.Title, i.Body, i.Status, i.Priority, orNull(i.AssigneeSessionID),
		updatedAt, orNull(i.CompletedAt), i.ID)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}
	return nil
}

// AttachIssueLabel links an issue and a label. Idempotent.
func AttachIssueLabel(ctx context.Context, q Querier, issueID, labelID int64) error {
	_, err := q.ExecContext(ctx, `
		INSERT OR IGNORE INTO issue_labels (issue_id, label_id) VALUES (?, ?)`,
		issueID, labelID)
	if err != nil {
		return fmt.Errorf("attach label: %w", err)
	}
	return nil
}

// ClearIssueLabels removes all label links of an issue.
func ClearIssueLabels(ctx context.Context, q Querier, issueID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM issue_labels WHERE issue_id = ?`, issueID)
	if err != nil {
		return fmt.Errorf("clear labels: %w", err)
	}
	return nil
}

// ListIssueLabels returns the labels attached to an issue, ordered by name.
func ListIssueLabels(ctx context.Context, q Querier, issueID int64) ([]domain.Label, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT l.id, l.project_id, l.name, l.color
		FROM issue_labels il JOIN labels l ON l.id = il.label_id
		WHERE il.issue_id = ? ORDER BY l.name`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list issue labels: %w", err)
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

// AddRelation links two issues. A relation in the opposite direction with the
// same type is treated as the same relation (blocks is directional, so this
// only deduplicates exact duplicates).
func AddRelation(ctx context.Context, q Querier, r *domain.IssueRelation) error {
	_, err := q.ExecContext(ctx, `
		INSERT OR IGNORE INTO issue_relations (issue_id, related_issue_id, relation_type, created_at)
		VALUES (?, ?, ?, ?)`,
		r.IssueID, r.RelatedIssueID, r.RelationType, Now())
	if err != nil {
		if strings.Contains(err.Error(), "CHECK constraint failed") {
			return fmt.Errorf("%w: cannot relate an issue to itself", domain.ErrInvalid)
		}
		return fmt.Errorf("insert relation: %w", err)
	}
	return nil
}

// ListIssueRelations returns forward relations of an issue, joined with the
// related issue's title for display.
func ListIssueRelations(ctx context.Context, q Querier, issueID int64) ([]domain.IssueRelation, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT r.issue_id, r.related_issue_id, r.relation_type,
		       i.project_id, i.title
		FROM issue_relations r JOIN issues i ON i.id = r.related_issue_id
		WHERE r.issue_id = ? ORDER BY r.relation_type, r.related_issue_id`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	defer rows.Close()

	var out []domain.IssueRelation
	for rows.Next() {
		var r domain.IssueRelation
		if err := rows.Scan(&r.IssueID, &r.RelatedIssueID, &r.RelationType, &r.RelatedProjectID, &r.RelatedTitle); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRelation removes one directed relation.
func DeleteRelation(ctx context.Context, q Querier, issueID, relatedID int64, t domain.RelationType) error {
	res, err := q.ExecContext(ctx, `
		DELETE FROM issue_relations WHERE issue_id = ? AND related_issue_id = ? AND relation_type = ?`,
		issueID, relatedID, t)
	if err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
