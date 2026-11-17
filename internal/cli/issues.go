package cli

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func runIssues(opts *Options, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, "usage: ledger issues list|get|create|update|complete|search|list-my|assign|unassign ...\n")
		return 2
	}
	c := &client{base: strings.TrimSuffix(opts.Server, "/")}
	ctx := context.Background()
	verb := args[0]
	rest := args[1:]

	switch verb {
	case "list":
		return issuesList(opts, c, ctx, rest)
	case "get":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: ledger issues get <id>")
			return 2
		}
		return issuesGet(opts, c, ctx, rest[0])
	case "create":
		return issuesCreate(opts, c, ctx, rest)
	case "update":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "usage: ledger issues update <id> [flags]")
			return 2
		}
		return issuesUpdate(opts, c, ctx, rest[0], rest[1:])
	case "complete":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "usage: ledger issues complete <id> -b BODY|-f FILE")
			return 2
		}
		return issuesComplete(opts, c, ctx, rest[0], rest[1:])
	case "search":
		return issuesSearch(opts, c, ctx, rest)
	case "list-my":
		return issuesListMy(opts, c, ctx, rest)
	case "assign":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: ledger issues assign <id> -s SESSION")
			return 2
		}
		return issuesAssign(opts, c, ctx, rest[0], rest[1:])
	case "unassign":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: ledger issues unassign <id>")
			return 2
		}
		return issuesUnassign(opts, c, ctx, rest[0])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", verb)
		return 2
	}
}

// listFilter holds the shared issue listing flags.
type listFilter struct {
	projectID  string
	status     string
	priority   string
	assignee   string
	unassigned bool
	label      string
	query      string
	limit      int
}

// addListFlags registers the shared filters on fs.
func addListFlags(fs *flag.FlagSet, f *listFilter) {
	stringFlag(fs, &f.projectID, "project", "p", "", "filter by project id")
	stringFlag(fs, &f.status, "status", "", "", "comma-separated: todo,in_progress,done,cancelled")
	stringFlag(fs, &f.priority, "priority", "", "", "low, medium, high or urgent")
	stringFlag(fs, &f.assignee, "assignee", "", "", "filter by assigned session id")
	boolFlag(fs, &f.unassigned, "unassigned", "", false, "only issues without an assignee")
	stringFlag(fs, &f.label, "label", "l", "", "filter by label name")
	stringFlag(fs, &f.query, "query", "q", "", "full text filter over title and body")
	intFlag(fs, &f.limit, "limit", "", 0, "max issues returned")
}

// queryValues renders the filter as an /api/issues query string.
func (f *listFilter) queryValues() url.Values {
	q := url.Values{}
	if f.projectID != "" {
		q.Set("project_id", f.projectID)
	}
	if f.status != "" {
		q.Set("status", f.status)
	}
	if f.priority != "" {
		q.Set("priority", f.priority)
	}
	if f.assignee != "" {
		q.Set("assignee_session_id", f.assignee)
	}
	if f.unassigned {
		q.Set("unassigned", "true")
	}
	if f.label != "" {
		q.Set("label", f.label)
	}
	if f.query != "" {
		q.Set("q", f.query)
	}
	if f.limit > 0 {
		q.Set("limit", fmt.Sprint(f.limit))
	}
	return q
}

func fetchIssues(ctx context.Context, c *client, q url.Values) ([]issue, error) {
	var issues []issue
	if err := c.get(ctx, "/api/issues?"+q.Encode(), &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

func printIssues(opts *Options, issues []issue, noneMsg string) int {
	if opts.JSON {
		return printJSON(issues)
	}
	if len(issues) == 0 {
		fmt.Println(noneMsg)
		return 0
	}
	printIssueTable(issues)
	return 0
}

func issuesList(opts *Options, c *client, ctx context.Context, args []string) int {
	fs := newFlags(opts, "issues list")
	var f listFilter
	addListFlags(fs, &f)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	issues, err := fetchIssues(ctx, c, f.queryValues())
	if err != nil {
		return runError(err)
	}
	return printIssues(opts, issues, "no issues")
}

func issuesGet(opts *Options, c *client, ctx context.Context, idArg string) int {
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	var i issue
	if err := c.get(ctx, fmt.Sprintf("/api/issues/%d", id), &i); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(i)
	}
	fmt.Printf("#%d %s  [%s]  %s\n", i.ID, i.Status, i.Priority, i.Title)
	if len(i.Labels) > 0 {
		fmt.Printf("labels: %s\n", labelNames(i.Labels))
	}
	if i.AssigneeSessionID != "" {
		fmt.Printf("session: %s\n", i.AssigneeSessionID)
	}
	if i.CompletedAt != "" {
		fmt.Printf("completed: %s\n", i.CompletedAt)
	}
	fmt.Printf("updated: %s\n", i.UpdatedAt)
	return 0
}

func issuesCreate(opts *Options, c *client, ctx context.Context, args []string) int {
	fs := newFlags(opts, "issues create")
	var projectID, title, bodyFlag, fileFlag, priority, labelsCSV, session string
	stringFlag(fs, &projectID, "project", "p", "", "project id (required)")
	stringFlag(fs, &title, "title", "t", "", "issue title (required)")
	stringFlag(fs, &bodyFlag, "body", "b", "", "issue body markdown")
	stringFlag(fs, &fileFlag, "body-file", "f", "", "issue body from file (- reads stdin)")
	stringFlag(fs, &priority, "priority", "", "", "low, medium, high or urgent")
	stringFlag(fs, &labelsCSV, "labels", "l", "", "comma separated label names")
	stringFlag(fs, &session, "session", "s", "", "assign to session id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if projectID == "" || title == "" {
		fmt.Fprintln(os.Stderr, "usage: ledger issues create -p <project_id> -t <title> [-b BODY|-f FILE] [-l a,b] [-s SESSION]")
		return 2
	}
	pid, err := parseID(projectID)
	if err != nil {
		return runError(err)
	}
	body, err := readBodyText(bodyFlag, fileFlag)
	if err != nil {
		return runError(err)
	}
	payload := map[string]any{
		"project_id": pid,
		"title":      title,
		"body":       body,
	}
	if priority != "" {
		payload["priority"] = priority
	}
	if labelsCSV != "" {
		payload["labels"] = splitCSV(labelsCSV)
	}
	if session != "" {
		payload["assignee_session_id"] = session
	}
	var out issue
	if err := c.post(ctx, "/api/issues", payload, &out); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(out)
	}
	fmt.Printf("#%d %s created\n", out.ID, out.Title)
	return 0
}

func issuesUpdate(opts *Options, c *client, ctx context.Context, idArg string, args []string) int {
	fs := newFlags(opts, "issues update")
	var title, bodyFlag, fileFlag, priority, status, labelsCSV, session string
	var unassign bool
	stringFlag(fs, &title, "title", "t", "", "new title")
	stringFlag(fs, &bodyFlag, "body", "b", "", "new body markdown")
	stringFlag(fs, &fileFlag, "body-file", "f", "", "new body from file (- reads stdin)")
	stringFlag(fs, &priority, "priority", "", "", "low, medium, high or urgent")
	stringFlag(fs, &status, "status", "", "", "todo, in_progress or cancelled (done requires: issues complete)")
	stringFlag(fs, &labelsCSV, "labels", "l", "", "comma separated names; replaces the set")
	stringFlag(fs, &session, "session", "s", "", "assign to session id")
	boolFlag(fs, &unassign, "unassign", "", false, "clear the owning session")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	body, err := readBodyText(bodyFlag, fileFlag)
	if err != nil {
		return runError(err)
	}
	// A body-file of "-" may legitimately carry an empty body, so the presence
	// of the flag (not the text) counts as a change.
	changed := title != "" || body != "" || fileFlag != "" || priority != "" ||
		status != "" || labelsCSV != "" || session != "" || unassign
	if !changed {
		fmt.Fprintln(os.Stderr, "ledger: nothing to update: pass at least one flag")
		return 2
	}
	payload := map[string]any{}
	if title != "" {
		payload["title"] = title
	}
	if body != "" || fileFlag == "-" {
		payload["body"] = body
	}
	if priority != "" {
		payload["priority"] = priority
	}
	if status != "" {
		payload["status"] = status
	}
	if labelsCSV != "" {
		payload["labels"] = splitCSV(labelsCSV)
	}
	switch {
	case unassign:
		payload["assignee_session_id"] = ""
	case session != "":
		payload["assignee_session_id"] = session
	}
	var out issue
	if err := c.patch(ctx, fmt.Sprintf("/api/issues/%d", id), payload, &out); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(out)
	}
	fmt.Printf("#%d %s updated\n", out.ID, out.Title)
	return 0
}

func issuesComplete(opts *Options, c *client, ctx context.Context, idArg string, args []string) int {
	fs := newFlags(opts, "issues complete")
	var bodyFlag, fileFlag, author, session string
	stringFlag(fs, &bodyFlag, "body", "b", "", "result comment markdown (required)")
	stringFlag(fs, &fileFlag, "body-file", "f", "", "result comment from file (- reads stdin)")
	stringFlag(fs, &author, "author", "a", "", "display name of the completer")
	stringFlag(fs, &session, "session", "s", "", "session id posting the result")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	body, err := readBodyText(bodyFlag, fileFlag)
	if err != nil {
		return runError(err)
	}
	if body == "" {
		fmt.Fprintln(os.Stderr, "ledger: completing requires a result comment (-b TEXT or -f FILE|-)")
		return 2
	}
	payload := map[string]any{"body": body, "author": author, "session_id": session}
	var out issue
	if err := c.post(ctx, fmt.Sprintf("/api/issues/%d/complete", id), payload, &out); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(out)
	}
	fmt.Printf("#%d %s done (completed %s)\n", out.ID, out.Title, out.CompletedAt)
	return 0
}

func issuesSearch(opts *Options, c *client, ctx context.Context, args []string) int {
	fs := newFlags(opts, "issues search")
	var f listFilter
	addListFlags(fs, &f)
	fs.BoolVar(&f.unassigned, "unassigned", false, "")
	if err := fs.Parse(args); err != nil || f.query == "" {
		fmt.Fprintln(os.Stderr, "usage: ledger issues search -q QUERY [-p ID] [--limit N]")
		return 2
	}
	issues, err := fetchIssues(ctx, c, f.queryValues())
	if err != nil {
		return runError(err)
	}
	return printIssues(opts, issues, "no matching issues")
}

func issuesListMy(opts *Options, c *client, ctx context.Context, args []string) int {
	fs := newFlags(opts, "issues list-my")
	var session, status, projectID string
	stringFlag(fs, &session, "session", "s", "", "session id (required)")
	stringFlag(fs, &status, "status", "", "", "comma-separated statuses (default todo,in_progress)")
	stringFlag(fs, &projectID, "project", "p", "", "restrict to project id")
	if err := fs.Parse(args); err != nil || session == "" {
		fmt.Fprintln(os.Stderr, "usage: ledger issues list-my -s SESSION [-p ID] [--status todo,in_progress]")
		return 2
	}
	if status == "" {
		status = "todo,in_progress"
	}
	f := listFilter{projectID: projectID, assignee: session, status: status}
	issues, err := fetchIssues(ctx, c, f.queryValues())
	if err != nil {
		return runError(err)
	}
	return printIssues(opts, issues, "no issues assigned to this session")
}

func issuesAssign(opts *Options, c *client, ctx context.Context, idArg string, args []string) int {
	fs := newFlags(opts, "issues assign")
	var session string
	stringFlag(fs, &session, "session", "s", "", "owning session id (required)")
	if err := fs.Parse(args); err != nil || session == "" {
		fmt.Fprintln(os.Stderr, "usage: ledger issues assign <id> -s SESSION")
		return 2
	}
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	var out issue
	path := fmt.Sprintf("/api/issues/%d/assign", id)
	if err := c.apiRequest(ctx, http.MethodPost, path, map[string]string{"session_id": session}, &out); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(out)
	}
	fmt.Printf("#%d assigned to %s\n", out.ID, session)
	return 0
}

func issuesUnassign(opts *Options, c *client, ctx context.Context, idArg string) int {
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	var out issue
	if err := c.post(ctx, fmt.Sprintf("/api/issues/%d/unassign", id), nil, &out); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(out)
	}
	fmt.Printf("#%d unassigned\n", out.ID)
	return 0
}

// splitCSV splits a comma separated argument.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
