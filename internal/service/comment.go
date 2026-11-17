package service

import (
	"context"

	"ledger/internal/domain"
	"ledger/internal/repository"
)

// AddComment appends a comment to an issue. Callers may supply a session_id
// when the comment comes from an AI session.
func (s *Service) AddComment(ctx context.Context, issueID int64, c CommentInput) (*domain.Comment, error) {
	if err := domain.ValidateComment(c.Body); err != nil {
		return nil, err
	}
	if _, err := s.GetIssue(ctx, issueID); err != nil {
		return nil, err
	}
	comment := &domain.Comment{
		IssueID:   issueID,
		Author:    c.Author,
		SessionID: c.SessionID,
		Body:      c.Body,
		CreatedAt: s.Now(),
	}
	id, err := repository.CreateComment(ctx, s.DB, comment)
	if err != nil {
		return nil, err
	}
	comment.ID = id
	return comment, nil
}

// ListComments returns all comments of an issue, oldest first.
func (s *Service) ListComments(ctx context.Context, issueID int64) ([]domain.Comment, error) {
	if _, err := s.GetIssue(ctx, issueID); err != nil {
		return nil, err
	}
	return repository.ListCommentsByIssue(ctx, s.DB, issueID)
}
