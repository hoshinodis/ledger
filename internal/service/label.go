package service

import (
	"context"

	"ledger/internal/domain"
	"ledger/internal/repository"
)

// ValidateLabelName checks that a label name is usable.
func ValidateLabelName(name string) error {
	if name == "" {
		return domain.NewInvalid("label name is required")
	}
	return nil
}

// ListLabels returns the labels of a project.
func (s *Service) ListLabels(ctx context.Context, projectID int64) ([]domain.Label, error) {
	if _, err := s.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	return repository.ListLabels(ctx, s.DB, projectID)
}

// CreateLabel adds a label to a project.
func (s *Service) CreateLabel(ctx context.Context, projectID int64, name, color string) (*domain.Label, error) {
	if err := ValidateLabelName(name); err != nil {
		return nil, err
	}
	if _, err := s.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	l := &domain.Label{ProjectID: projectID, Name: name, Color: color}
	id, err := repository.CreateLabel(ctx, s.DB, l)
	if err != nil {
		return nil, err
	}
	l.ID = id
	return l, nil
}

// DeleteLabel removes a label; removed labels detach from their issues.
func (s *Service) DeleteLabel(ctx context.Context, id int64) error {
	if _, err := repository.GetLabel(ctx, s.DB, id); err != nil {
		return err
	}
	return repository.DeleteLabel(ctx, s.DB, id)
}
