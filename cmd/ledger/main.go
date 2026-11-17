// Command ledger is the single binary of the ledger issue tracker: a server
// (ledger serve) and a CLI client for MCP-less environments.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"ledger/internal/cli"
	"ledger/internal/mcpserver"
	"ledger/internal/repository"
	"ledger/internal/serve"
	"ledger/internal/service"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		runServe(os.Args[2:])
		return
	}
	os.Exit(cli.Main(os.Args[1:]))
}

// runServe starts the HTTP server exposing REST, MCP and (static web UI).
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dbPath := fs.String("db", defaultDBPath(), "path to the SQLite database file")
	addr := fs.String("addr", defaultAddr(), "HTTP listen address")
	uiDir := fs.String("ui", "web/dist", "directory of the built web UI (empty string disables)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments after serve")
		fs.Usage()
		os.Exit(2)
	}

	db, err := repository.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := repository.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	svc := service.New(db)
	mux := http.NewServeMux()
	serve.New(svc).Register(mux)
	mux.Handle("/mcp", mcpserver.StreamHandler(svc))
	// Serve the built web UI when present (go run without a UI build has none).
	if ui := serve.StaticDir(*uiDir); ui != nil {
		mux.Handle("/", ui)
	}

	log.Printf("ledger listening on http://%s (db: %s)", *addr, *dbPath)
	srv := &http.Server{Addr: *addr, Handler: mux}
	log.Fatal(srv.ListenAndServe())
}

// defaultDBPath defaults the database location to the working directory
// unless LEDGER_DB points elsewhere.
func defaultDBPath() string {
	if v := os.Getenv("LEDGER_DB"); v != "" {
		return v
	}
	return "ledger.db"
}

// defaultAddr allows remote listeners via LEDGER_ADDR, e.g. "0.0.0.0:8080".
func defaultAddr() string {
	if v := os.Getenv("LEDGER_ADDR"); v != "" {
		return v
	}
	return "127.0.0.1:8080"
}
