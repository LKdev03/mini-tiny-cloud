package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/LKdev03/mini-tiny-cloud/internal/docker"
)

// RequestTimeout bounds Docker operations (image pull + create can be slow).
const RequestTimeout = 2 * time.Minute

//HUOM! Original requests (the handlers themselves like Health) dont have their own timeouts. Only their childcontexts do.

// Handlers exposes HTTP handlers for the control plane API.
type Handlers struct {
	docker *docker.Client
}

// NewHandlers builds handlers wired to the Docker client.
func NewHandlers(dockerClient *docker.Client) *Handlers {
	return &Handlers{docker: dockerClient}
}

// Health reports API and Docker connectivity.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	dockerStatus := "ok"
	if err := h.docker.Ping(ctx); err != nil {
		dockerStatus = "unreachable"
	}

	apiStatus := "ok"
	if dockerStatus != "ok" {
		apiStatus = "degraded"
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": apiStatus,
		"docker": dockerStatus,
	})
}

// Containers handles GET /containers and POST /containers.
func (h *Handlers) Containers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listContainers(w, r)
	case http.MethodPost:
		h.createContainer(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handlers) listContainers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	containers, err := h.docker.ListContainers(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, containers)
}

type createContainerRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Ports []int  `json:"ports"`
}

func (h *Handlers) createContainer(w http.ResponseWriter, r *http.Request) {
	var req createContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid JSON body"))
		return
	}

	if req.Name == "" || req.Image == "" {
		writeError(w, http.StatusBadRequest, errors.New("name and image are required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	created, err := h.docker.CreateContainer(ctx, docker.CreateOptions{
		Name:  req.Name,
		Image: req.Image,
		Ports: req.Ports,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// ContainerByID handles DELETE /containers/:id, POST /containers/:id/start, and POST /containers/:id/stop.
func (h *Handlers) ContainerByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/containers/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	id := parts[0]

	if len(parts) == 1 && r.Method == http.MethodDelete {
		h.deleteContainer(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "start" && r.Method == http.MethodPost {
		h.startContainer(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "stop" && r.Method == http.MethodPost {
		h.stopContainer(w, r, id)
		return
	}

	http.NotFound(w, r)
}

func (h *Handlers) deleteContainer(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	if err := h.docker.DeleteContainer(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) startContainer(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	if err := h.docker.StartContainer(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":     id,
		"status": "running",
	})
}

func (h *Handlers) stopContainer(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	if err := h.docker.StopContainer(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":     id,
		"status": "exited",
	})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{
		"error": err.Error(),
	})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
		"error": "method not allowed",
	})
}
