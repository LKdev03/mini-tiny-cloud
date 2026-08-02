package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Projects handles GET /projects and POST /projects.
func (h *Handlers) Projects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listProjects(w, r)
	case http.MethodPost:
		h.createProject(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handlers) listProjects(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DBRequestTimeout)
	defer cancel()

	projects, err := h.store.ListProjects(ctx)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, projects)
}

type createProjectRequest struct {
	Name string `json:"name"`
}

func (h *Handlers) createProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid JSON body"))
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DBRequestTimeout)
	defer cancel()

	project, err := h.store.CreateProject(ctx, req.Name)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

// ProjectByID handles GET /projects/:id and DELETE /projects/:id.
func (h *Handlers) ProjectByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/projects/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getProject(w, r, id)
	case http.MethodDelete:
		h.deleteProject(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handlers) getProject(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), DBRequestTimeout)
	defer cancel()

	project, err := h.store.GetProject(ctx, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h *Handlers) deleteProject(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), DBRequestTimeout)
	defer cancel()

	if err := h.store.DeleteProject(ctx, id); err != nil {
		writeStoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
