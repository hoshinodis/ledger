package service

import (
	"context"

	"ledger/internal/domain"
	"ledger/internal/repository"
)

// CreateIssueInput carries fields for creating an issue. Unknown label names
// are created on the fly in the project.
type CreateIssueInput struct {
	ProjectID         int64
	Title             string
	Body              string
	Priority          domain.Priority // empty -> medium
	Labels            []string
	AssigneeSessionID string
}

// UpdateIssueInput uses pointers so nil means "leave unchanged".
// Labels, when set, replaces the full label set.
type UpdateIssueInput struct {
	Title             *string
	Body              *string
	Priority          *domain.Priority
	Status            *domain.Status
	AssigneeSessionID *string // empty string clears the assignee
	Labels            *[]string
}

// CreateIssue creates an issue in the todo state.
func (s *Service) CreateIssue(ctx context.Context, in CreateIssueInput) (*domain.Issue, error) {
	if in.Priority == "" {
		in.Priority = domain.PriorityMedium
	}
	if err := domain.ValidateIssue(in.Title, in.Priority); err != nil {
		return nil, err
	}
	i := &domain.Issue{
		ProjectID:         in.ProjectID,
		Title:             in.Title,
		Body:              in.Body,
		Status:            domain.StatusTodo,
		Priority:          in.Priority,
		AssigneeSessionID: in.AssigneeSessionID,
	}
	err := s.Tx(ctx, func(ctx context.Context, q repository.Querier) error {
		if _, err := repository.GetProject(ctx, q, in.ProjectID); err != nil {
			return err
		}
		now := s.Now()
		i.CreatedAt = now
		i.UpdatedAt = now
		id, err := repository.CreateIssue(ctx, q, i)
		if err != nil {
			return err
		}
		i.ID = id
		if err := s.applyLabelNames(ctx, q, i.ID, in.ProjectID, in.Labels); err != nil {
			return err
		}
		labels, err := repository.ListIssueLabels(ctx, q, i.ID)
		if err != nil {
			return err
		}
		i.Labels = labels
		return nil
	})
	if err != nil {
		return nil, err
	}
	return i, nil
}

// GetIssue returns one issue with its labels, or domain.ErrNotFound.
func (s *Service) GetIssue(ctx context.Context, id int64) (*domain.Issue, error) {
	i, err := repository.GetIssue(ctx, s.DB, id)
	if err != nil {
		return nil, err
	}
	if i.Labels, err = repository.ListIssueLabels(ctx, s.DB, i.ID); err != nil {
		return nil, err
	}
	return i, nil
}

// ListIssues returns issues matching filter, with labels attached.
func (s *Service) ListIssues(ctx context.Context, f repository.IssueFilter) ([]domain.Issue, error) {
	if f.Priority != "" {
		if err := domain.ValidatePriority(f.Priority); err != nil {
			return nil, err
		}
	}
	issues, err := repository.ListIssues(ctx, s.DB, f)
	if err != nil {
		return nil, err
	}
	return s.attachLabels(ctx, s.DB, issues)
}

// UpdateIssue applies only the fields set in in. Moving to status "done" is
// rejected: callers must go through CompleteIssue, whose comment captures the
// result of the work.
func (s *Service) UpdateIssue(ctx context.Context, id int64, in UpdateIssueInput) (*domain.Issue, error) {
	if in.Status != nil {
		if err := domain.ValidateStatus(*in.Status); err != nil {
			return nil, err
		}
		if *in.Status == domain.StatusDone {
			return nil, domain.NewInvalid("status done requires complete_issue (with a result comment)")
		}
	}
	if in.Priority != nil && *in.Priority != "" {
		if err := domain.ValidatePriority(*in.Priority); err != nil {
			return nil, err
		}
	}
	var out *domain.Issue
	err := s.Tx(ctx, func(ctx context.Context, q repository.Querier) error {
		i, err := repository.GetIssue(ctx, q, id)
		if err != nil {
			return err
		}
		if in.Title != nil {
			i.Title = *in.Title
			if err := domain.ValidateIssue(i.Title, i.Priority); err != nil {
				return err
			}
		}
		if in.Body != nil {
			i.Body = *in.Body
		}
		if in.Priority != nil && *in.Priority != "" {
			i.Priority = *in.Priority
		}
		if in.Status != nil {
			i.Status = *in.Status
			// Leaving done (or any completed state) resets completion time.
			if *in.Status != domain.StatusDone {
				i.CompletedAt = ""
			}
		}
		if in.AssigneeSessionID != nil {
			i.AssigneeSessionID = *in.AssigneeSessionID
		}
		i.UpdatedAt = s.Now()
		if err := repository.UpdateIssue(ctx, q, i, i.UpdatedAt); err != nil {
			return err
		}
		if in.Labels != nil {
			if err := s.applyLabelNames(ctx, q, i.ID, i.ProjectID, *in.Labels); err != nil {
				return err
			}
		}
		if i.Labels, err = repository.ListIssueLabels(ctx, q, i.ID); err != nil {
			return err
		}
		out = i
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CommentInput is a comment payload attached to a complete or comment op.
type CommentInput struct {
	Author    string
	SessionID string
	Body      string
}

// CompleteIssue moves an issue to done and records the required result
// comment in one transaction. A comment without body is rejected.
func (s *Service) CompleteIssue(ctx context.Context, id int64, c CommentInput) (*domain.Issue, error) {
	if err := domain.ValidateComment(c.Body); err != nil {
		return nil, err
	}
	var out *domain.Issue
	err := s.Tx(ctx, func(ctx context.Context, q repository.Querier) error {
		i, err := repository.GetIssue(ctx, q, id)
		if err != nil {
			return err
		}
		now := s.Now()
		i.Status = domain.StatusDone
		i.CompletedAt = now
		i.UpdatedAt = now
		if err := repository.UpdateIssue(ctx, q, i, now); err != nil {
			return err
		}
		comment := &domain.Comment{
			IssueID:   id,
			Author:    c.Author,
			SessionID: c.SessionID,
			Body:      c.Body,
			CreatedAt: now,
		}
		if _, err = repository.CreateComment(ctx, q, comment); err != nil {
			return err
		}
		if i.Labels, err = repository.ListIssueLabels(ctx, q, i.ID); err != nil {
			return err
		}
		out = i
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AssignIssue sets the owning session of an issue.
func (s *Service) AssignIssue(ctx context.Context, id int64, sessionID string) (*domain.Issue, error) {
	if sessionID == "" {
		return nil, domain.NewInvalid("session_id is required")
	}
	sid := sessionID
	return s.UpdateIssue(ctx, id, UpdateIssueInput{AssigneeSessionID: &sid})
}

// UnassignIssue clears the owning session of an issue.
func (s *Service) UnassignIssue(ctx context.Context, id int64) (*domain.Issue, error) {
	empty := ""
	return s.UpdateIssue(ctx, id, UpdateIssueInput{AssigneeSessionID: &empty})
}

// AddRelation links two issues with a minimal relation type. The related
// issue must exist; self-relations are rejected.
func (s *Service) AddRelation(ctx context.Context, issueID, relatedID int64, t domain.RelationType) error {
	if err := domain.ValidateRelationType(t); err != nil {
		return err
	}
	if issueID == relatedID {
		return domain.NewInvalid("cannot relate an issue to itself")
	}
	return s.Tx(ctx, func(ctx context.Context, q repository.Querier) error {
		if _, err := repository.GetIssue(ctx, q, issueID); err != nil {
			return err
		}
		if _, err := repository.GetIssue(ctx, q, relatedID); err != nil {
			return err
		}
		return repository.AddRelation(ctx, q, &domain.IssueRelation{
			IssueID:        issueID,
			RelatedIssueID: relatedID,
			RelationType:   t,
		})
	})
}

// ListIssueRelations returns forward relations of an issue.
func (s *Service) ListIssueRelations(ctx context.Context, issueID int64) ([]domain.IssueRelation, error) {
	if _, err := s.GetIssue(ctx, issueID); err != nil {
		return nil, err
	}
	return repository.ListIssueRelations(ctx, s.DB, issueID)
}

// DeleteRelation removes one directed relation.
func (s *Service) DeleteRelation(ctx context.Context, issueID, relatedID int64, t domain.RelationType) error {
	if err := domain.ValidateRelationType(t); err != nil {
		return err
	}
	return s.Tx(ctx, func(ctx context.Context, q repository.Querier) error {
		if _, err := repository.GetIssue(ctx, q, issueID); err != nil {
			return err
		}
		return repository.DeleteRelation(ctx, q, issueID, relatedID, t)
	})
}

// applyLabelNames ensures each named label exists in the project and links it
// to the issue, replacing any previous label set.
func (s *Service) applyLabelNames(ctx context.Context, q repository.Querier, issueID, projectID int64, names []string) error {
	if err := repository.ClearIssueLabels(ctx, q, issueID); err != nil {
		return err
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		l, err := repository.EnsureLabel(ctx, q, projectID, name)
		if err != nil {
			return err
		}
		if err := repository.AttachIssueLabel(ctx, q, issueID, l.ID); err != nil {
			return err
		}
	}
	return nil
}
