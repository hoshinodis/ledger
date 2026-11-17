// Package serve exposes the ledger service over HTTP: JSON REST API.
// Errors from the service layer map to status codes in one place.
package serve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"ledger/internal/domain"
	"ledger/internal/service"
)

// Handler serves the REST API.
type Handler struct {
	svc *service.Service
}

// New creates a handler for svc.
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts all routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	// Projects
	mux.HandleFunc("GET /api/projects", h.listProjects)
	mux.HandleFunc("POST /api/projects", h.createProject)
	mux.HandleFunc("GET /api/projects/{id}", h.getProject)
	mux.HandleFunc("PATCH /api/projects/{id}", h.updateProject)
	mux.HandleFunc("GET /api/projects/{id}/context", h.getProjectContext)
	mux.HandleFunc("PUT /api/projects/{id}/context", h.updateProjectContext)
	mux.HandleFunc("GET /api/projects/{id}/bootstrap", h.bootstrap)

	// Labels
	mux.HandleFunc("GET /api/projects/{id}/labels", h.listLabels)
	mux.HandleFunc("POST /api/projects/{id}/labels", h.createLabel)
	mux.HandleFunc("DELETE /api/labels/{id}", h.deleteLabel)

	// Issues
	mux.HandleFunc("GET /api/issues", h.listIssues)
	mux.HandleFunc("POST /api/issues", h.createIssue)
	mux.HandleFunc("GET /api/issues/{id}", h.getIssue)
	mux.HandleFunc("PATCH /api/issues/{id}", h.updateIssue)
	mux.HandleFunc("POST /api/issues/{id}/complete", h.completeIssue)
	mux.HandleFunc("POST /api/issues/{id}/assign", h.assignIssue)
	mux.HandleFunc("POST /api/issues/{id}/unassign", h.unassignIssue)
	mux.HandleFunc("GET /api/issues/{id}/comments", h.listComments)
	mux.HandleFunc("POST /api/issues/{id}/comments", h.addComment)
	mux.HandleFunc("GET /api/issues/{id}/relations", h.listRelations)
	mux.HandleFunc("POST /api/issues/{id}/relations", h.addRelation)
	mux.HandleFunc("DELETE /api/issues/{id}/relations", h.deleteRelation)
}

// writeJSON renders v as JSON.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decode parses and validates the JSON body of r.
func decode(v any, r *http.Request) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.NewInvalid("request body is required")
		}
		return domain.NewInvalid("bad JSON body: %v", err)
	}
	return nil
}

// writeErr maps domain errors to HTTP status codes.
func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// pathID parses a numeric path parameter.
func pathID(r *http.Request, name string) (int64, error) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || v <= 0 {
		return 0, domain.NewInvalid("invalid %s", name)
	}
	return v, nil
}

// queryID parses a numeric query parameter; missing means 0.
func queryID(r *http.Request, name string) (int64, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, domain.NewInvalid("invalid %s", name)
	}
	return n, nil
}

// splitList splits a comma-separated query parameter.
func splitList(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// intQuery parses an integer query parameter with a default.
func intQuery(r *http.Request, name string, def int) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, domain.NewInvalid("invalid %s", name)
	}
	return n, nil
}

// readBoolQuery parses a boolean query flag.
func readBoolQuery(r *http.Request, name string) (bool, error) {
	v := r.URL.Query().Get(name)
	switch v {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	}
	return false, fmt.Errorf("invalid %s", name)
}
