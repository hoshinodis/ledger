package serve

import (
	"net/http"

	"ledger/internal/domain"
	"ledger/internal/repository"
	"ledger/internal/service"
)

type issueInput struct {
	ProjectID         int64    `json:"project_id"`
	Title             string   `json:"title"`
	Body              string   `json:"body"`
	Priority          string   `json:"priority"`
	Labels            []string `json:"labels"`
	AssigneeSessionID string   `json:"assignee_session_id"`
}

type issuePatch struct {
	Title             *string    `json:"title"`
	Body              *string    `json:"body"`
	Priority          *string    `json:"priority"`
	Status            *string    `json:"status"`
	AssigneeSessionID *string    `json:"assignee_session_id"`
	Labels            *[]string  `json:"labels"`
}

type completeInput struct {
	Author    string `json:"author"`
	SessionID string `json:"session_id"`
	Body      string `json:"body"`
}

type assignInput struct {
	SessionID string `json:"session_id"`
}

func (h *Handler) listIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := repository.IssueFilter{
		AssigneeSessionID: q.Get("assignee_session_id"),
		LabelName:         q.Get("label"),
		Query:             q.Get("q"),
	}
	var err error
	if f.ProjectID, err = queryID(r, "project_id"); err != nil {
		writeErr(w, err)
		return
	}
	if f.Priority = domain.Priority(q.Get("priority")); f.Priority != "" {
		if err := domain.ValidatePriority(f.Priority); err != nil {
			writeErr(w, err)
			return
		}
	}
	for _, s := range splitList(q.Get("status")) {
		st := domain.Status(s)
		if err := domain.ValidateStatus(st); err != nil {
			writeErr(w, err)
			return
		}
		f.Statuses = append(f.Statuses, st)
	}
	if f.UnassignedOnly, err = readBoolQuery(r, "unassigned"); err != nil {
		writeErr(w, domain.NewInvalid("invalid unassigned"))
		return
	}
	if f.Limit, err = intQuery(r, "limit", 0); err != nil {
		writeErr(w, err)
		return
	}
	issues, err := h.svc.ListIssues(r.Context(), f)
	if err != nil {
		writeErr(w, err)
		return
	}
	if issues == nil {
		issues = []domain.Issue{}
	}
	writeJSON(w, http.StatusOK, issues)
}

func (h *Handler) createIssue(w http.ResponseWriter, r *http.Request) {
	var in issueInput
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	i, err := h.svc.CreateIssue(r.Context(), service.CreateIssueInput{
		ProjectID:         in.ProjectID,
		Title:             in.Title,
		Body:              in.Body,
		Priority:          domain.Priority(in.Priority),
		Labels:            in.Labels,
		AssigneeSessionID: in.AssigneeSessionID,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, i)
}

func (h *Handler) getIssue(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	i, err := h.svc.GetIssue(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, i)
}

func (h *Handler) updateIssue(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var in issuePatch
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	upd := service.UpdateIssueInput{
		Title:             in.Title,
		Body:              in.Body,
		AssigneeSessionID: in.AssigneeSessionID,
		Labels:            in.Labels,
	}
	if in.Priority != nil && *in.Priority != "" {
		p := domain.Priority(*in.Priority)
		upd.Priority = &p
	}
	if in.Status != nil && *in.Status != "" {
		st := domain.Status(*in.Status)
		upd.Status = &st
	}
	i, err := h.svc.UpdateIssue(r.Context(), id, upd)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, i)
}

func (h *Handler) completeIssue(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var in completeInput
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	i, err := h.svc.CompleteIssue(r.Context(), id, service.CommentInput{
		Author:    in.Author,
		SessionID: in.SessionID,
		Body:      in.Body,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, i)
}

func (h *Handler) assignIssue(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var in assignInput
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	i, err := h.svc.AssignIssue(r.Context(), id, in.SessionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, i)
}

func (h *Handler) unassignIssue(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	i, err := h.svc.UnassignIssue(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, i)
}

// ---- relations ----

type relationInput struct {
	RelatedIssueID int64  `json:"related_issue_id"`
	RelationType   string `json:"relation_type"`
}

func (h *Handler) listRelations(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	rels, err := h.svc.ListIssueRelations(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if rels == nil {
		rels = []domain.IssueRelation{}
	}
	writeJSON(w, http.StatusOK, rels)
}

func (h *Handler) addRelation(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var in relationInput
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	if err := h.svc.AddRelation(r.Context(), id, in.RelatedIssueID, domain.RelationType(in.RelationType)); err != nil {
		writeErr(w, err)
		return
	}
	rels, err := h.svc.ListIssueRelations(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rels)
}

func (h *Handler) deleteRelation(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	relatedID, err := queryID(r, "related_issue_id")
	if err != nil {
		writeErr(w, err)
		return
	}
	t := domain.RelationType(r.URL.Query().Get("relation_type"))
	if err := h.svc.DeleteRelation(r.Context(), id, relatedID, t); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
