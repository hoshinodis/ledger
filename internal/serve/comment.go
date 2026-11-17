package serve

import (
	"net/http"

	"ledger/internal/domain"
	"ledger/internal/service"
)

type commentInput struct {
	Author    string `json:"author"`
	SessionID string `json:"session_id"`
	Body      string `json:"body"`
}

func (h *Handler) listComments(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	cs, err := h.svc.ListComments(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if cs == nil {
		cs = []domain.Comment{}
	}
	writeJSON(w, http.StatusOK, cs)
}

func (h *Handler) addComment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var in commentInput
	if err := decode(&in, r); err != nil {
		writeErr(w, err)
		return
	}
	c, err := h.svc.AddComment(r.Context(), id, service.CommentInput{
		Author:    in.Author,
		SessionID: in.SessionID,
		Body:      in.Body,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}
