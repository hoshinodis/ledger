package mcpserver_test

import (
	"context"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"ledger/internal/mcpserver"
	"ledger/internal/repository"
	"ledger/internal/service"
)

// startServer spins an in-memory ledger behind an MCP server over HTTP and
// returns both the session and the service (for direct test setup).
func startServer(t *testing.T) (*sdkmcp.ClientSession, *service.Service) {
	t.Helper()
	db, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := repository.Migrate(db); err != nil {
		t.Fatal(err)
	}
	svc := service.New(db)
	svc.Now = func() string { return "2026-01-01T00:00:00Z" }

	ts := httptest.NewServer(mcpserver.StreamHandler(svc))
	t.Cleanup(ts.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &sdkmcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, svc
}

// call runs one tool and returns its JSON text content.
func call(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

func TestMCPExposesExpectedTools(t *testing.T) {
	session, _ := startServer(t)
	ctx := context.Background()
	resp, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(resp.Tools))
	for _, tl := range resp.Tools {
		names = append(names, tl.Name)
	}
	// The SDK returns tools sorted by name.
	want := []string{
		"add_comment", "assign_issue", "complete_issue", "create_issue",
		"get_issue", "get_project", "get_project_bootstrap", "list_comments",
		"list_issues", "list_my_issues", "list_projects", "search_issues",
		"unassign_issue", "update_issue", "update_project_context",
	}
	slices.Sort(names)
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("tools mismatch:\n got  %v\n want %v", names, want)
	}
}

func TestMCPCreateCompleteFlow(t *testing.T) {
	session, svc := startServer(t)
	ctx := context.Background()

	// Projects are created via REST / the web UI, not via an MCP tool.
	if _, err := svc.CreateProject(ctx, service.CreateProjectInput{Name: "proj"}); err != nil {
		t.Fatal(err)
	}

	out := call(t, ctx, session, "create_issue", map[string]any{
		"project_id": 1, "title": "first", "labels": []string{"infra"}, "assignee_session_id": "sess-a",
	})
	if out == "" {
		t.Fatal("create_issue returned empty content")
	}

	// A fresh project needs to exist first; create_issue without project fails as a tool error.
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "create_issue",
		Arguments: map[string]any{"project_id": 99, "title": "x"},
	})
	if err != nil {
		t.Fatalf("call should not fail at transport level: %v", err)
	}
	if res != nil && !res.IsError {
		t.Fatal("create_issue on missing project should be a tool error")
	}

	// update_issue cannot set done.
	res, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "update_issue",
		Arguments: map[string]any{"id": 1, "status": "done"},
	})
	if err != nil {
		t.Fatalf("update_issue call: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("update_issue to done must be a tool error")
	}

	// complete without comment is a tool error.
	res, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "complete_issue",
		Arguments: map[string]any{"id": 1},
	})
	if err != nil {
		t.Fatalf("complete call: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("complete without body must be a tool error")
	}

	// bootstrap includes the created project and assigned issue.
	b := call(t, ctx, session, "get_project_bootstrap", map[string]any{"project_id": 1, "session_id": "sess-a"})
	if !strings.Contains(b, `"my_issues"`) || !strings.Contains(b, "first") {
		t.Fatalf("bootstrap missing assigned issue: %s", b)
	}

	// list_my_issues finds the assigned open issue.
	mine := call(t, ctx, session, "list_my_issues", map[string]any{"session_id": "sess-a"})
	if !strings.Contains(mine, "first") {
		t.Fatalf("list_my_issues: %s", mine)
	}
}
