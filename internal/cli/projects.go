package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type project struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Context     string `json:"context"`
	UpdatedAt   string `json:"updated_at"`
	CreatedAt   string `json:"created_at"`
}

type bootstrap struct {
	Project        project `json:"project"`
	MyIssues       []issue `json:"my_issues"`
	OpenIssues     []issue `json:"open_issues"`
	RecentlyDone   []issue `json:"recently_done"`
	RecentComments []comment `json:"recent_comments"`
}

func runProjects(opts *Options, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, "usage: ledger projects list|get|context ...\n")
		return 2
	}
	c := &client{base: strings.TrimSuffix(opts.Server, "/")}
	ctx := context.Background()
	switch args[0] {
	case "create":
		return projectsCreate(opts, c, ctx, args[1:])
	case "list":
		return projectsList(opts, c, ctx, args[1:])
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: ledger projects get <id>")
			return 2
		}
		return projectGet(opts, c, ctx, args[1])
	case "context":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: ledger projects context get|update <id> ...")
			return 2
		}
		switch args[1] {
		case "get":
			return projectContextGet(opts, c, ctx, args[2])
		case "update":
			return projectContextUpdate(opts, c, ctx, args[2], args[3:])
		default:
			fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", args[1])
			return 2
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", args[0])
		return 2
	}
}

func projectsCreate(opts *Options, c *client, ctx context.Context, args []string) int {
	fs := newFlags(opts, "projects create")
	var name, description string
	stringFlag(fs, &name, "name", "n", "", "project name (required)")
	stringFlag(fs, &description, "description", "d", "", "short description")
	if err := fs.Parse(args); err != nil || name == "" {
		fmt.Fprintln(os.Stderr, "usage: ledger projects create -n NAME [-d DESCRIPTION]")
		return 2
	}
	var out project
	if err := c.post(ctx, "/api/projects", map[string]string{
		"name":        name,
		"description": description,
	}, &out); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(out)
	}
	fmt.Printf("#%d %s created\n", out.ID, out.Name)
	return 0
}

func projectsList(opts *Options, c *client, ctx context.Context, args []string) int {
	fs := newFlags(opts, "projects list")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var ps []project
	if err := c.get(ctx, "/api/projects", &ps); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(ps)
	}
	if len(ps) == 0 {
		fmt.Println("no projects")
		return 0
	}
	for _, p := range ps {
		fmt.Printf("#%-4d %s  %s\n", p.ID, shortTime(p.UpdatedAt), p.Name)
		if p.Description != "" {
			fmt.Printf("      %s\n", p.Description)
		}
	}
	return 0
}

func projectGet(opts *Options, c *client, ctx context.Context, idArg string) int {
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	var p project
	if err := c.get(ctx, fmt.Sprintf("/api/projects/%d", id), &p); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(p)
	}
	fmt.Printf("#%d %s\n%s\n\ncontext:\n%s", p.ID, p.Name, p.Description, p.Context)
	return 0
}

func projectContextGet(opts *Options, c *client, ctx context.Context, idArg string) int {
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	var body struct {
		Context string `json:"context"`
	}
	path := fmt.Sprintf("/api/projects/%d/context", id)
	if err := c.get(ctx, path, &body); err != nil {
		return runError(err)
	}
	fmt.Print(body.Context)
	return 0
}

func projectContextUpdate(opts *Options, c *client, ctx context.Context, idArg string, args []string) int {
	fs := newFlags(opts, "projects context update")
	var bodyFlag, fileFlag string
	stringFlag(fs, &bodyFlag, "body", "b", "", "context markdown via argument")
	stringFlag(fs, &fileFlag, "file", "f", "", "context markdown from file (- reads stdin)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	md, err := readBodyText(bodyFlag, fileFlag)
	if err != nil {
		return runError(err)
	}
	path := fmt.Sprintf("/api/projects/%d/context", id)
	out := project{}
	if err := c.apiRequest(ctx, "PUT", path, map[string]string{"context": md}, &out); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(out)
	}
	fmt.Printf("#%d %s: context updated (%d chars)\n", out.ID, out.Name, len(md))
	return 0
}

// runBootstrap prints the working bundle a new session needs. The leading
// positional argument is the project id; flags follow it.
func runBootstrap(opts *Options, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: ledger bootstrap <project_id> [-s SESSION]")
		return 2
	}
	fs := newFlags(opts, "bootstrap")
	var session string
	var openLimit, doneLimit, commentLimit int
	stringFlag(fs, &session, "session", "s", "", "session id; its assigned issues are prioritized")
	intFlag(fs, &openLimit, "open-limit", "", 0, "max open issues (default 20)")
	intFlag(fs, &doneLimit, "done-limit", "", 0, "max recently completed (default 5)")
	intFlag(fs, &commentLimit, "comment-limit", "", 0, "max recent comments (default 10)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	id, err := parseID(args[0])
	if err != nil {
		return runError(err)
	}
	q := url.Values{}
	if session != "" {
		q.Set("session_id", session)
	}
	if openLimit > 0 {
		q.Set("open_limit", fmt.Sprint(openLimit))
	}
	if doneLimit > 0 {
		q.Set("done_limit", fmt.Sprint(doneLimit))
	}
	if commentLimit > 0 {
		q.Set("comment_limit", fmt.Sprint(commentLimit))
	}
	c := &client{base: strings.TrimSuffix(opts.Server, "/")}
	var b bootstrap
	path := fmt.Sprintf("/api/projects/%d/bootstrap?%s", id, q.Encode())
	if err := c.get(context.Background(), path, &b); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(b)
	}
	fmt.Printf("#%d %s\n", b.Project.ID, b.Project.Name)
	if b.Project.Description != "" {
		fmt.Printf("%s\n", b.Project.Description)
	}
	if b.Project.Context == "" {
		fmt.Println("(no project context yet)")
	} else {
		fmt.Printf("\n== project context ==\n%s", b.Project.Context)
	}
	printSection("my issues (assigned to this session)", b.MyIssues)
	printSection("open issues", b.OpenIssues)
	printSection("recently done", b.RecentlyDone)
	if len(b.RecentComments) > 0 {
		fmt.Println("\n== recent comments ==")
		for _, cm := range b.RecentComments {
			fmt.Printf("issue #%d %s  %s%s\n  %s\n", cm.IssueID, shortTime(cm.CreatedAt),
				cm.Author, sessionSuffix(cm.SessionID), firstLine(cm.Body))
		}
	}
	return 0
}

func printSection(title string, issues []issue) {
	fmt.Printf("\n== %s ==\n", title)
	if len(issues) == 0 {
		fmt.Println("(none)")
		return
	}
	printIssueTable(issues)
}

// firstLine keeps preview comments to a single line.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + " ..."
	}
	return s
}

func sessionSuffix(sid string) string {
	if sid == "" {
		return ""
	}
	return " (" + sid + ")"
}

// parseID strictly parses numeric object ids.
func parseID(s string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(s, "%d", &id); err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id %q: expected a positive number", s)
	}
	return id, nil
}
