package serve

import (
	"errors"
	"net/http"

	"ledger/internal/domain"
	"ledger/internal/service"
)

type projectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Context     string `json:"context"`
}

type projectPatch struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.ListProjects(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if ps == nil {
		ps = []domain.Project{}
	}
	writeJSON(w, http.StatusOK, ps)
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	var in projectInput
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	p, err := h.svc.CreateProject(r.Context(), service.CreateProjectInput{
		Name:        in.Name,
		Description: in.Description,
		Context:     in.Context,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) getProject(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	p, err := h.svc.GetProject(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) updateProject(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var in projectPatch
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	p, err := h.svc.UpdateProject(r.Context(), id, service.UpdateProjectInput{
		Name:        in.Name,
		Description: in.Description,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type contextBody struct {
	Context string `json:"context"`
}

func (h *Handler) getProjectContext(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	md, err := h.svc.GetProjectContext(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contextBody{Context: md})
}

func (h *Handler) updateProjectContext(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var in contextBody
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	p, err := h.svc.UpdateProjectContext(r.Context(), id, in.Context)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	openLimit, err := intQuery(r, "open_limit", 20)
	if err != nil {
		writeErr(w, err)
		return
	}
	doneLimit, err := intQuery(r, "done_limit", 5)
	if err != nil {
		writeErr(w, err)
		return
	}
	commentN, err := intQuery(r, "comment_limit", 10)
	if err != nil {
		writeErr(w, err)
		return
	}
	b, err := h.svc.Bootstrap(r.Context(), id, r.URL.Query().Get("session_id"), service.BootstrapOptions{
		OpenIssueLimit:    openLimit,
		RecentlyDoneLimit: doneLimit,
		RecentCommentN:    commentN,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// ---- labels ----

type labelInput struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (h *Handler) listLabels(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	ls, err := h.svc.ListLabels(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if ls == nil {
		ls = []domain.Label{}
	}
	writeJSON(w, http.StatusOK, ls)
}

func (h *Handler) createLabel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var in labelInput
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	l, err := h.svc.CreateLabel(r.Context(), id, in.Name, in.Color)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (h *Handler) deleteLabel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	err = h.svc.DeleteLabel(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeErr(w, err)
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
