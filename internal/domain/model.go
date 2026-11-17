// Package domain defines the core models, enums and validation rules of ledger.
package domain

// Status is the lifecycle state of an Issue.
type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusCancelled  Status = "cancelled"
)

// Priority is the urgency of an Issue.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// RelationType is a minimal set of issue relations.
type RelationType string

const (
	RelationRelatesTo RelationType = "relates_to"
	RelationBlocks    RelationType = "blocks"
)

// Valid reports whether s is one of the known issue statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone, StatusCancelled:
		return true
	}
	return false
}

// Valid reports whether p is one of the known priorities.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

// Valid reports whether r is one of the known relation types.
func (r RelationType) Valid() bool {
	switch r {
	case RelationRelatesTo, RelationBlocks:
		return true
	}
	return false
}

// Project is a long-lived unit of work with its own context.
type Project struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Context     string `json:"context"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Issue is the central unit of work, tracked through status and assigned session.
type Issue struct {
	ID                int64    `json:"id"`
	ProjectID         int64    `json:"project_id"`
	Title             string   `json:"title"`
	Body              string   `json:"body"`
	Status            Status   `json:"status"`
	Priority          Priority `json:"priority"`
	AssigneeSessionID string   `json:"assignee_session_id,omitempty"`
	Labels            []Label  `json:"labels,omitempty"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
	CompletedAt       string   `json:"completed_at,omitempty"`
}

// Comment records work history and decisions for an Issue.
type Comment struct {
	ID        int64  `json:"id"`
	IssueID   int64  `json:"issue_id"`
	Author    string `json:"author,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// Label is a project-scoped tag attached to issues.
type Label struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
	Color     string `json:"color,omitempty"`
}

// IssueRelation links two issues with a minimal typed relation.
type IssueRelation struct {
	IssueID         int64        `json:"issue_id"`
	RelatedIssueID  int64        `json:"related_issue_id"`
	RelationType    RelationType `json:"relation_type"`
	RelatedProjectID int64       `json:"related_project_id,omitempty"`
	RelatedTitle    string       `json:"related_title,omitempty"`
}

// ValidateStatus checks that s is a known status.
func ValidateStatus(s Status) error {
	if !s.Valid() {
		return NewInvalid("invalid status %q: must be one of todo, in_progress, done, cancelled", s)
	}
	return nil
}

// ValidatePriority checks that p is a known priority.
func ValidatePriority(p Priority) error {
	if !p.Valid() {
		return NewInvalid("invalid priority %q: must be one of low, medium, high, urgent", p)
	}
	return nil
}

// ValidateRelationType checks that r is a known relation type.
func ValidateRelationType(r RelationType) error {
	if !r.Valid() {
		return NewInvalid("invalid relation_type %q: must be one of relates_to, blocks", r)
	}
	return nil
}

// ValidateProject checks fields for creating or updating a project.
func ValidateProject(name string) error {
	if name == "" {
		return NewInvalid("name is required")
	}
	if len(name) > 200 {
		return NewInvalid("name must be 200 characters or fewer")
	}
	return nil
}

// ValidateIssue checks fields for creating an issue.
func ValidateIssue(title string, priority Priority) error {
	if title == "" {
		return NewInvalid("title is required")
	}
	if len(title) > 500 {
		return NewInvalid("title must be 500 characters or fewer")
	}
	return ValidatePriority(priority)
}

// ValidateComment checks fields for creating a comment.
func ValidateComment(body string) error {
	if body == "" {
		return NewInvalid("body is required")
	}
	return nil
}
