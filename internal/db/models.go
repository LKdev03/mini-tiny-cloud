package db

import "time"

// Project groups services (desired infrastructure for one application/tenant).
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Service is desired state: which image to run and how many replicas.
type Service struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Replicas  int       `json:"replicas"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Instance links a service to an actual Docker container on a worker node.
type Instance struct {
	ID          string    `json:"id"`
	ServiceID   string    `json:"service_id"`
	ContainerID string    `json:"container_id"`
	NodeID      string    `json:"node_id"`
	CreatedAt   time.Time `json:"created_at"`
}
