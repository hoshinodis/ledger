// Package cli implements the ledger command line client. It talks to a
// running ledger server over REST, so MCP-less agent environments can still
// drive every operation through plain shell commands.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Options are the global client options shared by all subcommands.
type Options struct {
	Server string
	JSON   bool
}

const usageText = `ledger — local-first issue tracker for AI agent sessions

Usage:
  ledger serve [-db PATH] [-addr HOST:PORT]   start the server (REST + MCP + web UI)
  ledger <command> [flags]

Commands:
  projects list [flags]                       list projects
  projects get <id>                           project with context
  projects context get <id>                   print context markdown
  projects context update <id> [-f FILE|-]    replace context markdown
  bootstrap <project_id> [-s SESSION]         session bootstrap bundle
  issues list [flags]                         filter and list issues
  issues get <id>                             one issue with labels
  issues create -p ID -t TITLE [flags]        create an issue
  issues update <id> [flags]                  partial update (done needs complete)
  issues complete <id> [-b BODY|-f FILE]      mark done with required comment
  issues search -q QUERY [flags]              full text search
  issues list-my -s SESSION [flags]           issues assigned to a session
  issues assign <id> -s SESSION               set owning session
  issues unassign <id>                        clear owning session
  comments list <issue_id>                    list comments
  comments add <issue_id> [-b BODY|-f FILE]   add a comment
  labels list <project_id>                    list labels
  labels create <project_id> <name>           create a label

Global flags (most commands): -server URL (env LEDGER_URL), -json.
Server defaults to http://127.0.0.1:8080.
Body content via -b TEXT, -f FILE where "-f -" reads stdin.
`

// Main dispatches a CLI invocation and returns the process exit code.
func Main(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	opts := &Options{Server: defaultServer()}
	switch args[0] {
	case "projects":
		return runProjects(opts, args[1:])
	case "bootstrap":
		return runBootstrap(opts, args[1:])
	case "issues":
		return runIssues(opts, args[1:])
	case "comments":
		return runComments(opts, args[1:])
	case "labels":
		return runLabels(opts, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", args[0], usageText)
		return 2
	}
}

// defaultServer resolves the server URL from the environment.
func defaultServer() string {
	if v := os.Getenv("LEDGER_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:8080"
}

// newFlags adds global flags to a subcommand flag set.
func newFlags(opts *Options, name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.StringVar(&opts.Server, "server", opts.Server, "ledger server URL")
	fs.BoolVar(&opts.JSON, "json", opts.JSON, "print raw JSON output")
	return fs
}

// runError reports a failed command.
func runError(err error) int {
	fmt.Fprintf(os.Stderr, "ledger: %v\n", err)
	return 1
}

// ---- flag helpers registering long and short names for one value ----

func stringFlag(fs *flag.FlagSet, p *string, long, short, def, usage string) {
	fs.StringVar(p, long, def, usage)
	if short != "" {
		fs.StringVar(p, short, def, "shorthand for -"+long)
	}
}

func boolFlag(fs *flag.FlagSet, p *bool, long, short string, def bool, usage string) {
	fs.BoolVar(p, long, def, usage)
	if short != "" {
		fs.BoolVar(p, short, def, "shorthand for -"+long)
	}
}

func intFlag(fs *flag.FlagSet, p *int, long, short string, def int, usage string) {
	fs.IntVar(p, long, def, usage)
	if short != "" {
		fs.IntVar(p, short, def, "shorthand for -"+long)
	}
}

// ---- API client ----

// client calls the ledger REST API.
type client struct {
	base string
}

// apiRequest performs one HTTP call and decodes the JSON response.
func (c *client) apiRequest(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach ledger server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *client) get(ctx context.Context, path string, out any) error {
	return c.apiRequest(ctx, http.MethodGet, path, nil, out)
}

func (c *client) post(ctx context.Context, path string, body, out any) error {
	return c.apiRequest(ctx, http.MethodPost, path, body, out)
}

func (c *client) patch(ctx context.Context, path string, body, out any) error {
	return c.apiRequest(ctx, http.MethodPatch, path, body, out)
}

func (c *client) delete(ctx context.Context, path string) error {
	return c.apiRequest(ctx, http.MethodDelete, path, nil, nil)
}

// ---- shared DTO mirroring the REST responses ----

type label struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

type issue struct {
	ID                int64   `json:"id"`
	ProjectID         int64   `json:"project_id"`
	Title             string  `json:"title"`
	Status            string  `json:"status"`
	Priority          string  `json:"priority"`
	AssigneeSessionID string  `json:"assignee_session_id,omitempty"`
	Labels            []label `json:"labels,omitempty"`
	UpdatedAt         string  `json:"updated_at"`
	CompletedAt       string  `json:"completed_at,omitempty"`
}

type comment struct {
	ID        int64  `json:"id"`
	IssueID   int64  `json:"issue_id"`
	Author    string `json:"author,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// ---- helpers ----

// printJSON emits raw JSON when -json was given.
func printJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return runError(err)
	}
	return 0
}

// shortTime trims RFC3339 timestamps for display.
func shortTime(s string) string {
	if len(s) < 19 {
		return s
	}
	return s[:19]
}

// labelNames folds label structs into "a,b".
func labelNames(ls []label) string {
	parts := make([]string, 0, len(ls))
	for _, l := range ls {
		parts = append(parts, l.Name)
	}
	return strings.Join(parts, ",")
}

// printIssueTable renders issues as a compact table.
func printIssueTable(issues []issue) {
	for _, i := range issues {
		priority := i.Priority
		if priority == "" {
			priority = "medium"
		}
		fmt.Printf("#%-4d %-11s %-6s %s  [%s] %s\n",
			i.ID, i.Status, priority, shortTime(i.UpdatedAt),
			labelNames(i.Labels), i.Title)
		if i.AssigneeSessionID != "" {
			fmt.Printf("      session: %s\n", i.AssigneeSessionID)
		}
	}
}

// readBodyText resolves a -b/-f body argument; -f - reads stdin.
func readBodyText(bodyFlag, fileFlag string) (string, error) {
	switch {
	case bodyFlag != "" && fileFlag != "":
		return "", fmt.Errorf("use only one of --body and --body-file")
	case bodyFlag != "":
		return bodyFlag, nil
	case fileFlag == "":
		return "", nil
	case fileFlag != "-":
		b, err := os.ReadFile(fileFlag)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
