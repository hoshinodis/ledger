package service_test

import (
	"context"
	"errors"
	"testing"

	"ledger/internal/domain"
	"ledger/internal/repository"
	"ledger/internal/service"
)

const testNow = "2026-01-01T00:00:00Z"

func newTestService(t *testing.T) *service.Service {
	t.Helper()
	db, err := repository.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := repository.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := service.New(db)
	s.Now = func() string { return testNow }
	return s
}

func mustCreateProject(t *testing.T, s *service.Service, name string) *domain.Project {
	t.Helper()
	p, err := s.CreateProject(context.Background(), service.CreateProjectInput{Name: name})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return p
}

func mustCreateIssue(t *testing.T, s *service.Service, projectID int64, title string) *domain.Issue {
	t.Helper()
	i, err := s.CreateIssue(context.Background(), service.CreateIssueInput{ProjectID: projectID, Title: title})
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	return i
}

func TestCreateProjectAndGet(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p, err := s.CreateProject(ctx, service.CreateProjectInput{Name: "demo", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == 0 || p.Name != "demo" {
		t.Fatalf("unexpected project: %+v", p)
	}
	got, err := s.GetProject(ctx, p.ID)
	if err != nil || got.Name != "demo" {
		t.Fatalf("get project: %v %+v", err, got)
	}
}

func TestCreateProjectValidation(t *testing.T) {
	s := newTestService(t)
	if _, err := s.CreateProject(context.Background(), service.CreateProjectInput{Name: ""}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}

func TestUpdateProjectPartial(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p := mustCreateProject(t, s, "a")
	newDesc := "desc"
	upd, err := s.UpdateProject(ctx, p.ID, service.UpdateProjectInput{Description: &newDesc})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Name != "a" || upd.Description != "desc" {
		t.Fatalf("unexpected update result: %+v", upd)
	}
}

func TestIssueStatusRules(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p := mustCreateProject(t, s, "proj")
	i := mustCreateIssue(t, s, p.ID, "task")
	if i.Status != domain.StatusTodo || i.Priority != domain.PriorityMedium {
		t.Fatalf("unexpected defaults: %+v", i)
	}

	// Moving to done through update_issue is rejected.
	done := domain.StatusDone
	_, err := s.UpdateIssue(ctx, i.ID, service.UpdateIssueInput{Status: &done})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("done via update should be rejected, got %v", err)
	}

	// Invalid status values are rejected.
	bad := domain.Status("finished")
	if _, err := s.UpdateIssue(ctx, i.ID, service.UpdateIssueInput{Status: &bad}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad status should be rejected, got %v", err)
	}
}

func TestCompleteIssueRequiresComment(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p := mustCreateProject(t, s, "proj")
	i := mustCreateIssue(t, s, p.ID, "task")

	// No comment -> no completion.
	if _, err := s.CompleteIssue(ctx, i.ID, service.CommentInput{}); err == nil {
		t.Fatal("completion without comment must fail")
	}

	// With comment -> done, completed_at set, comment stored.
	out, err := s.CompleteIssue(ctx, i.ID, service.CommentInput{Author: "ai", SessionID: "s1", Body: "did it"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != domain.StatusDone || out.CompletedAt == "" {
		t.Fatalf("unexpected completed issue: %+v", out)
	}
	cs, err := s.ListComments(ctx, i.ID)
	if err != nil || len(cs) != 1 || cs[0].SessionID != "s1" {
		t.Fatalf("comment not stored: %v %+v", err, cs)
	}

	// Reopen resets completed_at.
	inProg := domain.StatusInProgress
	re, err := s.UpdateIssue(ctx, i.ID, service.UpdateIssueInput{Status: &inProg})
	if err != nil {
		t.Fatal(err)
	}
	if re.CompletedAt != "" || re.Status != domain.StatusInProgress {
		t.Fatalf("reopen should clear completed_at: %+v", re)
	}
}

func TestIssueLabelsAutoCreate(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p := mustCreateProject(t, s, "proj")
	i, err := s.CreateIssue(ctx, service.CreateIssueInput{ProjectID: p.ID, Title: "x", Labels: []string{"bug", "api"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(i.Labels) != 2 {
		t.Fatalf("labels: %+v", i.Labels)
	}
	// Same label name later resolves to the existing label (labels are
	// returned sorted by name).
	var bugID int64
	for _, l := range i.Labels {
		if l.Name == "bug" {
			bugID = l.ID
		}
	}
	i2, err := s.CreateIssue(ctx, service.CreateIssueInput{ProjectID: p.ID, Title: "y", Labels: []string{"bug"}})
	if err != nil || len(i2.Labels) != 1 || i2.Labels[0].Name != "bug" || i2.Labels[0].ID != bugID {
		t.Fatalf("label reuse failed: %v %+v (want bug id %d)", err, i2.Labels, bugID)
	}
}

func TestUpdateIssueLabelReplace(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p := mustCreateProject(t, s, "proj")
	i, err := s.CreateIssue(ctx, service.CreateIssueInput{ProjectID: p.ID, Title: "x", Labels: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	only := []string{"c"}
	upd, err := s.UpdateIssue(ctx, i.ID, service.UpdateIssueInput{Labels: &only})
	if err != nil {
		t.Fatal(err)
	}
	if len(upd.Labels) != 1 || upd.Labels[0].Name != "c" {
		t.Fatalf("labels not replaced: %+v", upd.Labels)
	}
}

func TestAssignIssue(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p := mustCreateProject(t, s, "proj")

	mine, err := s.CreateIssue(ctx, service.CreateIssueInput{ProjectID: p.ID, Title: "mine", AssigneeSessionID: "sess-a"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateIssue(ctx, service.CreateIssueInput{ProjectID: p.ID, Title: "other"})
	if err != nil {
		t.Fatal(err)
	}

	issues, err := s.ListIssues(ctx, repository.IssueFilter{AssigneeSessionID: "sess-a"})
	if err != nil || len(issues) != 1 || issues[0].ID != mine.ID {
		t.Fatalf("list by assignee: %v %+v", err, issues)
	}
	if _, err := s.AssignIssue(ctx, other.ID, "sess-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AssignIssue(ctx, other.ID, ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("empty session assign should fail: %v", err)
	}
	// Unassign clears the owner.
	if _, err := s.UnassignIssue(ctx, other.ID); err != nil {
		t.Fatal(err)
	}
	issues, err = s.ListIssues(ctx, repository.IssueFilter{ProjectID: p.ID, UnassignedOnly: true})
	if err != nil || len(issues) != 1 {
		t.Fatalf("unassign issue not found: %v %+v", err, issues)
	}
}

func TestRelations(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p := mustCreateProject(t, s, "proj")
	a := mustCreateIssue(t, s, p.ID, "a")
	b := mustCreateIssue(t, s, p.ID, "b")

	if err := s.AddRelation(ctx, a.ID, b.ID, domain.RelationBlocks); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRelation(ctx, a.ID, a.ID, domain.RelationBlocks); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("self relation should fail: %v", err)
	}
	if err := s.AddRelation(ctx, a.ID, 99999, domain.RelationBlocks); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing related issue should fail: %v", err)
	}
	if err := s.AddRelation(ctx, a.ID, b.ID, domain.RelationType("weird")); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad relation type should fail: %v", err)
	}
	rels, err := s.ListIssueRelations(ctx, a.ID)
	if err != nil || len(rels) != 1 || rels[0].RelatedTitle != "b" {
		t.Fatalf("relations: %v %+v", err, rels)
	}
	if err := s.DeleteRelation(ctx, a.ID, b.ID, domain.RelationBlocks); err != nil {
		t.Fatal(err)
	}
	if rels, _ := s.ListIssueRelations(ctx, a.ID); len(rels) != 0 {
		t.Fatalf("relation not deleted: %+v", rels)
	}
}

func TestBootstrap(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p, err := s.CreateProject(ctx, service.CreateProjectInput{Name: "proj", Context: "# goal\nkeep simple"})
	if err != nil {
		t.Fatal(err)
	}

	mine, err := s.CreateIssue(ctx, service.CreateIssueInput{ProjectID: p.ID, Title: "mine", AssigneeSessionID: "sess-a"})
	if err != nil {
		t.Fatal(err)
	}
	other := mustCreateIssue(t, s, p.ID, "open")
	if _, err := s.CompleteIssue(ctx, other.ID, service.CommentInput{Body: "done"}); err != nil {
		t.Fatal(err)
	}
	second := mustCreateIssue(t, s, p.ID, "second")
	if _, err := s.AddComment(ctx, second.ID, service.CommentInput{Body: "watch out"}); err != nil {
		t.Fatal(err)
	}

	b, err := s.Bootstrap(ctx, p.ID, "sess-a", service.BootstrapOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if b.Project.ID != p.ID || b.Project.Context == "" {
		t.Fatalf("bootstrap project: %+v", b.Project)
	}
	if len(b.MyIssues) != 1 || b.MyIssues[0].ID != mine.ID {
		t.Fatalf("my_issues: %+v", b.MyIssues)
	}
	// Open issues exclude the done one but include mine and "second".
	if len(b.OpenIssues) != 2 {
		t.Fatalf("open_issues should exclude done: %+v", b.OpenIssues)
	}
	if len(b.RecentlyDone) != 1 || b.RecentlyDone[0].ID != other.ID {
		t.Fatalf("recently_done: %+v", b.RecentlyDone)
	}
	if len(b.RecentComments) != 2 {
		t.Fatalf("recent_comments: %+v", b.RecentComments)
	}
}

func TestBootstrapUnknownProject(t *testing.T) {
	s := newTestService(t)
	if _, err := s.Bootstrap(context.Background(), 42, "", service.BootstrapOptions{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSearchIssues(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p := mustCreateProject(t, s, "proj")
	mustCreateIssue(t, s, p.ID, "fix login bug")
	mustCreateIssue(t, s, p.ID, "write docs")

	got, err := s.ListIssues(ctx, repository.IssueFilter{ProjectID: p.ID, Query: "login"})
	if err != nil || len(got) != 1 || got[0].Title != "fix login bug" {
		t.Fatalf("query: %v %+v", err, got)
	}
}
