package service

import (
	"context"

	"ledger/internal/domain"
	"ledger/internal/repository"
)

// CreateProjectInput carries fields for creating a project.
type CreateProjectInput struct {
	Name        string
	Description string
	Context     string
}

// UpdateProjectInput uses pointers so nil means "leave unchanged".
type UpdateProjectInput struct {
	Name        *string
	Description *string
}

// Bootstrap is the bundle a new session needs to start working. It is
// intentionally bounded: summaries and recent items, not full history.
type Bootstrap struct {
	Project        domain.Project   `json:"project"`
	MyIssues       []domain.Issue   `json:"my_issues"`
	OpenIssues     []domain.Issue   `json:"open_issues"`
	RecentlyDone   []domain.Issue   `json:"recently_done"`
	RecentComments []domain.Comment `json:"recent_comments"`
}

// CreateProject creates a project with the given fields.
func (s *Service) CreateProject(ctx context.Context, in CreateProjectInput) (*domain.Project, error) {
	if err := domain.ValidateProject(in.Name); err != nil {
		return nil, err
	}
	now := s.Now()
	p := &domain.Project{
		Name:        in.Name,
		Description: in.Description,
		Context:     in.Context,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	id, err := repository.CreateProject(ctx, s.DB, p)
	if err != nil {
		return nil, err
	}
	p.ID = id
	return p, nil
}

// ListProjects returns every project, newest activity first.
func (s *Service) ListProjects(ctx context.Context) ([]domain.Project, error) {
	return repository.ListProjects(ctx, s.DB)
}

// GetProject returns one project or domain.ErrNotFound.
func (s *Service) GetProject(ctx context.Context, id int64) (*domain.Project, error) {
	return repository.GetProject(ctx, s.DB, id)
}

// UpdateProject applies only the fields set in in.
func (s *Service) UpdateProject(ctx context.Context, id int64, in UpdateProjectInput) (*domain.Project, error) {
	var p *domain.Project
	err := s.Tx(ctx, func(ctx context.Context, q repository.Querier) error {
		var err error
		p, err = repository.GetProject(ctx, q, id)
		if err != nil {
			return err
		}
		if in.Name != nil {
			if err := domain.ValidateProject(*in.Name); err != nil {
				return err
			}
			p.Name = *in.Name
		}
		if in.Description != nil {
			p.Description = *in.Description
		}
		return repository.UpdateProject(ctx, q, p, s.Now())
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetProjectContext returns the Markdown context of a project.
func (s *Service) GetProjectContext(ctx context.Context, id int64) (string, error) {
	p, err := repository.GetProject(ctx, s.DB, id)
	if err != nil {
		return "", err
	}
	return p.Context, nil
}

// UpdateProjectContext replaces the Markdown context of a project.
func (s *Service) UpdateProjectContext(ctx context.Context, id int64, md string) (*domain.Project, error) {
	var p *domain.Project
	err := s.Tx(ctx, func(ctx context.Context, q repository.Querier) error {
		var err error
		p, err = repository.GetProject(ctx, q, id)
		if err != nil {
			return err
		}
		p.Context = md
		return repository.UpdateProject(ctx, q, p, s.Now())
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// BootstrapOptions tune how much detail Bootstrap returns.
type BootstrapOptions struct {
	OpenIssueLimit   int // default 20
	RecentlyDoneLimit int // default 5
	RecentCommentN   int // default 10
}

// Bootstrap returns the working context for a new session: the project with
// its context, the session's open issues, open issues of the project, recent
// completions and recent comments. Full history is deliberately not included.
func (s *Service) Bootstrap(ctx context.Context, projectID int64, sessionID string, opts BootstrapOptions) (*Bootstrap, error) {
	if opts.OpenIssueLimit <= 0 {
		opts.OpenIssueLimit = 20
	}
	if opts.RecentlyDoneLimit <= 0 {
		opts.RecentlyDoneLimit = 5
	}
	if opts.RecentCommentN <= 0 {
		opts.RecentCommentN = 10
	}
	p, err := repository.GetProject(ctx, s.DB, projectID)
	if err != nil {
		return nil, err
	}
	b := &Bootstrap{Project: *p}

	if sessionID != "" {
		my, err := repository.ListIssues(ctx, s.DB, repository.IssueFilter{
			ProjectID:         projectID,
			AssigneeSessionID: sessionID,
			Statuses:          []domain.Status{domain.StatusTodo, domain.StatusInProgress},
		})
		if err != nil {
			return nil, err
		}
		if b.MyIssues, err = s.attachLabels(ctx, s.DB, my); err != nil {
			return nil, err
		}
	}
	open, err := repository.ListIssues(ctx, s.DB, repository.IssueFilter{
		ProjectID: projectID,
		Statuses:  []domain.Status{domain.StatusTodo, domain.StatusInProgress},
		Limit:     opts.OpenIssueLimit,
	})
	if err != nil {
		return nil, err
	}
	if b.OpenIssues, err = s.attachLabels(ctx, s.DB, open); err != nil {
		return nil, err
	}
	done, err := repository.ListIssues(ctx, s.DB, repository.IssueFilter{
		ProjectID: projectID,
		Statuses:  []domain.Status{domain.StatusDone},
		OrderBy:   "completed_at DESC",
		Limit:     opts.RecentlyDoneLimit,
	})
	if err != nil {
		return nil, err
	}
	if b.RecentlyDone, err = s.attachLabels(ctx, s.DB, done); err != nil {
		return nil, err
	}
	b.RecentComments, err = repository.ListRecentCommentsByProject(ctx, s.DB, projectID, opts.RecentCommentN)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// attachLabels fills in Labels for each issue.
func (s *Service) attachLabels(ctx context.Context, q repository.Querier, issues []domain.Issue) ([]domain.Issue, error) {
	for i := range issues {
		labels, err := repository.ListIssueLabels(ctx, q, issues[i].ID)
		if err != nil {
			return nil, err
		}
		issues[i].Labels = labels
	}
	return issues, nil
}
