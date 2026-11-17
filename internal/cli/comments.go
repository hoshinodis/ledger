package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func runComments(opts *Options, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, "usage: ledger comments list <issue_id> | comments add <issue_id> [-b BODY|-f FILE]\n")
		return 2
	}
	c := &client{base: strings.TrimSuffix(opts.Server, "/")}
	ctx := context.Background()
	switch args[0] {
	case "list":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: ledger comments list <issue_id>")
			return 2
		}
		return commentsList(opts, c, ctx, args[1])
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: ledger comments add <issue_id> -b BODY|-f FILE")
			return 2
		}
		return commentsAdd(opts, c, ctx, args[1], args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", args[0])
		return 2
	}
}

func commentsList(opts *Options, c *client, ctx context.Context, idArg string) int {
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	var cs []comment
	if err := c.get(ctx, fmt.Sprintf("/api/issues/%d/comments", id), &cs); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(cs)
	}
	if len(cs) == 0 {
		fmt.Println("no comments")
		return 0
	}
	for _, cm := range cs {
		fmt.Printf("#%d %s %s%s\n", cm.ID, shortTime(cm.CreatedAt), authorLine(cm), cm.Body)
	}
	return 0
}

func commentsAdd(opts *Options, c *client, ctx context.Context, idArg string, args []string) int {
	fs := newFlags(opts, "comments add")
	var bodyFlag, fileFlag, author, session string
	stringFlag(fs, &bodyFlag, "body", "b", "", "comment markdown")
	stringFlag(fs, &fileFlag, "body-file", "f", "", "comment from file (- reads stdin)")
	stringFlag(fs, &author, "author", "a", "", "display name of the commenter")
	stringFlag(fs, &session, "session", "s", "", "session id posting the comment")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	body, err := readBodyText(bodyFlag, fileFlag)
	if err != nil {
		return runError(err)
	}
	if body == "" {
		fmt.Fprintln(os.Stderr, "ledger: comment body is empty (-b TEXT or -f FILE|-)")
		return 2
	}
	id, err := parseID(idArg)
	if err != nil {
		return runError(err)
	}
	payload := map[string]any{"body": body, "author": author, "session_id": session}
	var out comment
	if err := c.post(ctx, fmt.Sprintf("/api/issues/%d/comments", id), payload, &out); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(out)
	}
	fmt.Printf("comment #%d added to issue #%d\n", out.ID, out.IssueID)
	return 0
}

// authorLine renders "author (session): " prefix when present.
func authorLine(cm comment) string {
	parts := make([]string, 0, 2)
	if cm.Author != "" {
		parts = append(parts, cm.Author)
	}
	if cm.SessionID != "" {
		parts = append(parts, cm.SessionID)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ") + ": "
}

func runLabels(opts *Options, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, "usage: ledger labels list <project_id> | labels create <project_id> <name>\n")
		return 2
	}
	c := &client{base: strings.TrimSuffix(opts.Server, "/")}
	ctx := context.Background()
	switch args[0] {
	case "list":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: ledger labels list <project_id>")
			return 2
		}
		return labelsList(opts, c, ctx, args[1])
	case "create":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: ledger labels create <project_id> <name> [--color COLOR]")
			return 2
		}
		return labelsCreate(opts, c, ctx, args[1], args[2], args[3:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", args[0])
		return 2
	}
}

func labelsList(opts *Options, c *client, ctx context.Context, projectIDArg string) int {
	id, err := parseID(projectIDArg)
	if err != nil {
		return runError(err)
	}
	var ls []label
	if err := c.get(ctx, fmt.Sprintf("/api/projects/%d/labels", id), &ls); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(ls)
	}
	if len(ls) == 0 {
		fmt.Println("no labels")
		return 0
	}
	for _, l := range ls {
		color := ""
		if l.Color != "" {
			color = " " + l.Color
		}
		fmt.Printf("#%-4d %s%s\n", l.ID, l.Name, color)
	}
	return 0
}

func labelsCreate(opts *Options, c *client, ctx context.Context, projectIDArg, name string, args []string) int {
	fs := newFlags(opts, "labels create")
	var color string
	stringFlag(fs, &color, "color", "c", "", "optional color, e.g. #d73a49")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	id, err := parseID(projectIDArg)
	if err != nil {
		return runError(err)
	}
	var out label
	if err := c.post(ctx, fmt.Sprintf("/api/projects/%d/labels", id), map[string]string{"name": name, "color": color}, &out); err != nil {
		return runError(err)
	}
	if opts.JSON {
		return printJSON(out)
	}
	fmt.Printf("label #%d created: %s\n", out.ID, out.Name)
	return 0
}
