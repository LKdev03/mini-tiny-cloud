package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/LKdev03/mini-tiny-cloud/internal/reconciler"
)

// Services handles GET /services and POST /services.
func (h *Handlers) Services(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listServices(w, r)
	case http.MethodPost:
		h.createService(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handlers) listServices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DBRequestTimeout)
	defer cancel()

	services, err := h.store.ListServices(ctx, r.URL.Query().Get("project_id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, services)
}

type createServiceRequest struct {
	ProjectID string `json:"project_id"`
	Image     string `json:"image"`
	Replicas  int    `json:"replicas"`
}

func (h *Handlers) createService(w http.ResponseWriter, r *http.Request) {
	var req createServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid JSON body"))
		return
	}
	if req.ProjectID == "" || req.Image == "" {
		writeError(w, http.StatusBadRequest, errors.New("project_id and image are required"))
		return
	}
	if req.Replicas < 0 {
		writeError(w, http.StatusBadRequest, errors.New("replicas must be >= 0"))
		return
	}
	if req.Replicas == 0 {
		req.Replicas = 1
	}

	ctx, cancel := context.WithTimeout(r.Context(), DBRequestTimeout)
	defer cancel()

	if _, err := h.store.GetProject(ctx, req.ProjectID); err != nil {
		writeStoreError(w, err)
		return
	}

	service, err := h.store.CreateService(ctx, req.ProjectID, req.Image, req.Replicas)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, service)
}

// ServiceByID handles service routes under /services/:id.
func (h *Handlers) ServiceByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/services/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	id := parts[0]

	if len(parts) == 2 && parts[1] == "instances" && r.Method == http.MethodGet {
		h.listServiceInstances(w, r, id)
		return
	}

	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getService(w, r, id)
	case http.MethodPatch:
		h.updateService(w, r, id)
	case http.MethodDelete:
		h.deleteService(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handlers) getService(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), DBRequestTimeout)
	defer cancel()

	service, err := h.store.GetService(ctx, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, service)
}

type updateServiceRequest struct {
	Image    *string `json:"image"`
	Replicas *int    `json:"replicas"`
}

func (h *Handlers) updateService(w http.ResponseWriter, r *http.Request, id string) {
	var req updateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid JSON body"))
		return
	}
	if req.Image == nil && req.Replicas == nil {
		writeError(w, http.StatusBadRequest, errors.New("at least one of image or replicas is required"))
		return
	}
	if req.Replicas != nil && *req.Replicas < 0 {
		writeError(w, http.StatusBadRequest, errors.New("replicas must be >= 0"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DBRequestTimeout)
	defer cancel()

	service, err := h.store.UpdateServiceDesiredState(ctx, id, req.Image, req.Replicas)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, service)
}

func (h *Handlers) deleteService(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	if _, err := h.store.GetService(ctx, id); err != nil {
		writeStoreError(w, err)
		return
	}

	if err := reconciler.DeleteServiceContainers(ctx, h.docker, h.store, id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := h.store.DeleteService(ctx, id); err != nil {
		writeStoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) listServiceInstances(w http.ResponseWriter, r *http.Request, serviceID string) {
	ctx, cancel := context.WithTimeout(r.Context(), DBRequestTimeout)
	defer cancel()

	if _, err := h.store.GetService(ctx, serviceID); err != nil {
		writeStoreError(w, err)
		return
	}

	instances, err := h.store.ListInstances(ctx, serviceID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, instances)
}
