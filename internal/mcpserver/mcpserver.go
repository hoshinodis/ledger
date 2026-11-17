// Package mcpserver exposes the ledger service as MCP tools over streamable
// HTTP, so AI agents can create, list and complete issues directly.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"ledger/internal/domain"
	"ledger/internal/repository"
	"ledger/internal/service"
)

// New builds the ledger MCP server with all tools registered.
func New(svc *service.Service) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ledger", Version: "0.1.0"}, nil)
	registerTools(server, svc)
	return server
}

// StreamHandler serves the MCP server at one HTTP endpoint.
func StreamHandler(svc *service.Service) http.Handler {
	server := New(svc)
	return sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, nil)
}

func registerTools(server *sdkmcp.Server, s *service.Service) {
	// ---- project tools ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_projects",
		Description: "List all ledger projects, newest activity first.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.ListProjects(ctx))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_project",
		Description: "Get one project by id including its Markdown context.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ID int64 `json:"id" jsonschema:"project id"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.GetProject(ctx, in.ID))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_project_context",
		Description: "Replace the Markdown context of a project. Context holds durable knowledge: purpose, stack, decisions to keep and avoid.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ProjectID int64  `json:"project_id" jsonschema:"project id"`
		Context   string `json:"context" jsonschema:"full Markdown context content"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.UpdateProjectContext(ctx, in.ProjectID, in.Context))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_project_bootstrap",
		Description: "The first call a new session should make: project summary, Markdown context, issues assigned to session_id, open issues, recently completed issues and recent comments. Bounded, not full history.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ProjectID    int64  `json:"project_id" jsonschema:"project to bootstrap"`
		SessionID    string `json:"session_id,omitempty" jsonschema:"session id; issues assigned to it are prioritized"`
		OpenLimit    int    `json:"open_limit,omitempty" jsonschema:"max open issues (default 20)"`
		DoneLimit    int    `json:"done_limit,omitempty" jsonschema:"max recently completed (default 5)"`
		CommentLimit int    `json:"comment_limit,omitempty" jsonschema:"max recent comments (default 10)"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.Bootstrap(ctx, in.ProjectID, in.SessionID, service.BootstrapOptions{
			OpenIssueLimit:    in.OpenLimit,
			RecentlyDoneLimit: in.DoneLimit,
			RecentCommentN:    in.CommentLimit,
		}))
	})

	// ---- issue tools ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_issues",
		Description: "List issues with optional filters (project, status, priority, assignee, label, full text).",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ProjectID         int64    `json:"project_id,omitempty" jsonschema:"filter by project"`
		Status            []string `json:"status,omitempty" jsonschema:"subset of: todo, in_progress, done, cancelled"`
		Priority          string   `json:"priority,omitempty" jsonschema:"low, medium, high or urgent"`
		AssigneeSessionID string   `json:"assignee_session_id,omitempty" jsonschema:"filter by assigned session id"`
		Unassigned        bool     `json:"unassigned,omitempty" jsonschema:"only issues without an assigned session"`
		Label             string   `json:"label,omitempty" jsonschema:"filter by label name"`
		Query             string   `json:"query,omitempty" jsonschema:"full text filter over title and body"`
		Limit             int      `json:"limit,omitempty" jsonschema:"max issues returned"`
	}) (*sdkmcp.CallToolResult, any, error) {
		f := repository.IssueFilter{
			ProjectID:         in.ProjectID,
			Priority:          domain.Priority(in.Priority),
			AssigneeSessionID: in.AssigneeSessionID,
			UnassignedOnly:    in.Unassigned,
			LabelName:         in.Label,
			Query:             in.Query,
			Limit:             in.Limit,
		}
		for _, st := range in.Status {
			sv := domain.Status(st)
			if err := domain.ValidateStatus(sv); err != nil {
				return nil, nil, err
			}
			f.Statuses = append(f.Statuses, sv)
		}
		return jsonOut(s.ListIssues(ctx, f))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_issue",
		Description: "Get one issue by id with labels.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ID int64 `json:"id" jsonschema:"issue id"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.GetIssue(ctx, in.ID))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_issue",
		Description: "Create a todo issue. Body is Markdown: background, instructions, constraints and done criteria. Unknown labels are created on first use.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ProjectID         int64    `json:"project_id" jsonschema:"project for the issue"`
		Title             string   `json:"title" jsonschema:"short summary of the task"`
		Body              string   `json:"body,omitempty" jsonschema:"markdown details: background, instructions, constraints, done criteria"`
		Priority          string   `json:"priority,omitempty" jsonschema:"low, medium, high or urgent (default medium)"`
		Labels            []string `json:"labels,omitempty" jsonschema:"label names; created on first use"`
		AssigneeSessionID string   `json:"assignee_session_id,omitempty" jsonschema:"session id that will work on this issue"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.CreateIssue(ctx, service.CreateIssueInput{
			ProjectID:         in.ProjectID,
			Title:             in.Title,
			Body:              in.Body,
			Priority:          domain.Priority(in.Priority),
			Labels:            in.Labels,
			AssigneeSessionID: in.AssigneeSessionID,
		}))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_issue",
		Description: "Update issue fields. Only provided fields change. status accepts todo, in_progress and cancelled; use complete_issue for done.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ID                int64     `json:"id" jsonschema:"issue id"`
		Title             *string   `json:"title,omitempty"`
		Body              *string   `json:"body,omitempty"`
		Priority          *string   `json:"priority,omitempty" jsonschema:"low, medium, high or urgent"`
		Status            *string   `json:"status,omitempty" jsonschema:"todo, in_progress or cancelled"`
		AssigneeSessionID *string   `json:"assignee_session_id,omitempty" jsonschema:"new owning session; empty string to unassign"`
		Labels            *[]string `json:"labels,omitempty" jsonschema:"replaces the full label set"`
	}) (*sdkmcp.CallToolResult, any, error) {
		upd := service.UpdateIssueInput{
			Title:             in.Title,
			Body:              in.Body,
			AssigneeSessionID: in.AssigneeSessionID,
			Labels:            in.Labels,
		}
		if in.Priority != nil && *in.Priority != "" {
			v := domain.Priority(*in.Priority)
			upd.Priority = &v
		}
		if in.Status != nil && *in.Status != "" {
			v := domain.Status(*in.Status)
			upd.Status = &v
		}
		return jsonOut(s.UpdateIssue(ctx, in.ID, upd))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "complete_issue",
		Description: "Mark an issue done. body is a required comment recording the result: what was done, key decisions and remaining work.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ID        int64  `json:"id" jsonschema:"issue id to complete"`
		Body      string `json:"body" jsonschema:"required result comment describing what was done"`
		Author    string `json:"author,omitempty" jsonschema:"display name of the completer"`
		SessionID string `json:"session_id,omitempty" jsonschema:"session id posting the result"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.CompleteIssue(ctx, in.ID, service.CommentInput{
			Author:    in.Author,
			SessionID: in.SessionID,
			Body:      in.Body,
		}))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "search_issues",
		Description: "Full text search over issue titles and bodies.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		Query     string `json:"query" jsonschema:"text to search, matched on substrings"`
		ProjectID int64  `json:"project_id,omitempty" jsonschema:"restrict search to a project"`
		Limit     int    `json:"limit,omitempty" jsonschema:"max issues returned"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.ListIssues(ctx, repository.IssueFilter{
			ProjectID: in.ProjectID,
			Query:     in.Query,
			Limit:     in.Limit,
		}))
	})

	// ---- session tools ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_my_issues",
		Description: "List issues assigned to a session id. Defaults to open issues (todo, in_progress) when status is omitted.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		SessionID string   `json:"session_id" jsonschema:"session id to look up"`
		ProjectID int64    `json:"project_id,omitempty" jsonschema:"restrict to a project"`
		Status    []string `json:"status,omitempty" jsonschema:"subset of: todo, in_progress, done, cancelled"`
	}) (*sdkmcp.CallToolResult, any, error) {
		f := repository.IssueFilter{AssigneeSessionID: in.SessionID, ProjectID: in.ProjectID}
		if len(in.Status) == 0 {
			in.Status = []string{string(domain.StatusTodo), string(domain.StatusInProgress)}
		}
		for _, st := range in.Status {
			sv := domain.Status(st)
			if err := domain.ValidateStatus(sv); err != nil {
				return nil, nil, err
			}
			f.Statuses = append(f.Statuses, sv)
		}
		return jsonOut(s.ListIssues(ctx, f))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "assign_issue",
		Description: "Attach an issue to a session as its owner.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ID        int64  `json:"id" jsonschema:"issue id"`
		SessionID string `json:"session_id" jsonschema:"owning session id"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.AssignIssue(ctx, in.ID, in.SessionID))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "unassign_issue",
		Description: "Clear the owning session of an issue.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		ID int64 `json:"id" jsonschema:"issue id"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.UnassignIssue(ctx, in.ID))
	})

	// ---- comment tools ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add_comment",
		Description: "Add a work-history comment to an issue: findings, decisions, files changed, test results, remaining work.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		IssueID   int64  `json:"issue_id" jsonschema:"issue id"`
		Body      string `json:"body" jsonschema:"markdown comment content"`
		Author    string `json:"author,omitempty" jsonschema:"display name of the commenter"`
		SessionID string `json:"session_id,omitempty" jsonschema:"session id posting the comment"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.AddComment(ctx, in.IssueID, service.CommentInput{
			Author:    in.Author,
			SessionID: in.SessionID,
			Body:      in.Body,
		}))
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_comments",
		Description: "List all comments of an issue, oldest first.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in struct {
		IssueID int64 `json:"issue_id" jsonschema:"issue id"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return jsonOut(s.ListComments(ctx, in.IssueID))
	})
}

// jsonOut renders the result as JSON text content; errors become tool errors.
func jsonOut[T any](v T, err error) (*sdkmcp.CallToolResult, any, error) {
	if err != nil {
		return nil, nil, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal result: %w", err)
	}
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(b)}},
	}, nil, nil
}
