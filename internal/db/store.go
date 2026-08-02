package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("not found")

// CreateProject inserts a new project.
func (s *Store) CreateProject(ctx context.Context, name string) (Project, error) {
	const query = `
		INSERT INTO projects (name)
		VALUES ($1)
		RETURNING id, name, created_at`

	var project Project
	if err := s.pool.QueryRow(ctx, query, name).Scan(
		&project.ID,
		&project.Name,
		&project.CreatedAt,
	); err != nil {
		return Project{}, fmt.Errorf("create project: %w", err)
	}

	return project, nil
}

// ListProjects returns all projects ordered by creation time.
func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	const query = `
		SELECT id, name, created_at
		FROM projects
		ORDER BY created_at ASC`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.Name, &project.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

// GetProject returns a project by ID.
func (s *Store) GetProject(ctx context.Context, id string) (Project, error) {
	const query = `
		SELECT id, name, created_at
		FROM projects
		WHERE id = $1`

	var project Project
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&project.ID,
		&project.Name,
		&project.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("get project: %w", err)
	}

	return project, nil
}

// DeleteProject removes a project and cascades to its services and instances.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateService records desired state for a workload.
func (s *Store) CreateService(ctx context.Context, projectID, image string, replicas int) (Service, error) {
	const query = `
		INSERT INTO services (project_id, image, replicas, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, project_id, image, replicas, status, created_at, updated_at`

	var service Service
	if err := s.pool.QueryRow(ctx, query, projectID, image, replicas).Scan(
		&service.ID,
		&service.ProjectID,
		&service.Image,
		&service.Replicas,
		&service.Status,
		&service.CreatedAt,
		&service.UpdatedAt,
	); err != nil {
		return Service{}, fmt.Errorf("create service: %w", err)
	}

	return service, nil
}

// ListServices returns services, optionally filtered by project ID.
func (s *Store) ListServices(ctx context.Context, projectID string) ([]Service, error) {
	var (
		rows pgx.Rows
		err  error
	)

	if projectID != "" {
		const query = `
			SELECT id, project_id, image, replicas, status, created_at, updated_at
			FROM services
			WHERE project_id = $1
			ORDER BY created_at ASC`
		rows, err = s.pool.Query(ctx, query, projectID)
	} else {
		const query = `
			SELECT id, project_id, image, replicas, status, created_at, updated_at
			FROM services
			ORDER BY created_at ASC`
		rows, err = s.pool.Query(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	services := make([]Service, 0)
	for rows.Next() {
		var service Service
		if err := rows.Scan(
			&service.ID,
			&service.ProjectID,
			&service.Image,
			&service.Replicas,
			&service.Status,
			&service.CreatedAt,
			&service.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}

	return services, nil
}

// GetService returns a service by ID.
func (s *Store) GetService(ctx context.Context, id string) (Service, error) {
	const query = `
		SELECT id, project_id, image, replicas, status, created_at, updated_at
		FROM services
		WHERE id = $1`

	var service Service
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&service.ID,
		&service.ProjectID,
		&service.Image,
		&service.Replicas,
		&service.Status,
		&service.CreatedAt,
		&service.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Service{}, ErrNotFound
	}
	if err != nil {
		return Service{}, fmt.Errorf("get service: %w", err)
	}

	return service, nil
}

// UpdateServiceDesiredState updates image and/or replica count (desired state).
func (s *Store) UpdateServiceDesiredState(ctx context.Context, id string, image *string, replicas *int) (Service, error) {
	current, err := s.GetService(ctx, id)
	if err != nil {
		return Service{}, err
	}

	if image != nil {
		current.Image = *image
	}
	if replicas != nil {
		current.Replicas = *replicas
	}

	const query = `
		UPDATE services
		SET image = $2, replicas = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, project_id, image, replicas, status, created_at, updated_at`

	var service Service
	if err := s.pool.QueryRow(ctx, query, id, current.Image, current.Replicas).Scan(
		&service.ID,
		&service.ProjectID,
		&service.Image,
		&service.Replicas,
		&service.Status,
		&service.CreatedAt,
		&service.UpdatedAt,
	); err != nil {
		return Service{}, fmt.Errorf("update service: %w", err)
	}

	return service, nil
}

// DeleteService removes desired state for a workload.
func (s *Store) DeleteService(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM services WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListInstances returns container instances backing a service.
func (s *Store) ListInstances(ctx context.Context, serviceID string) ([]Instance, error) {
	const query = `
		SELECT id, service_id, container_id, node_id, created_at
		FROM instances
		WHERE service_id = $1
		ORDER BY created_at ASC`

	rows, err := s.pool.Query(ctx, query, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	instances := make([]Instance, 0)
	for rows.Next() {
		var instance Instance
		if err := rows.Scan(
			&instance.ID,
			&instance.ServiceID,
			&instance.ContainerID,
			&instance.NodeID,
			&instance.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		instances = append(instances, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instances: %w", err)
	}

	return instances, nil
}

// CreateInstance records an actual container backing a service (used by reconciler in Phase 3).
func (s *Store) CreateInstance(ctx context.Context, serviceID, containerID, nodeID string) (Instance, error) {
	const query = `
		INSERT INTO instances (service_id, container_id, node_id)
		VALUES ($1, $2, $3)
		RETURNING id, service_id, container_id, node_id, created_at`

	var instance Instance
	if err := s.pool.QueryRow(ctx, query, serviceID, containerID, nodeID).Scan(
		&instance.ID,
		&instance.ServiceID,
		&instance.ContainerID,
		&instance.NodeID,
		&instance.CreatedAt,
	); err != nil {
		return Instance{}, fmt.Errorf("create instance: %w", err)
	}

	return instance, nil
}

// DeleteInstance removes an instance record (used by reconciler in Phase 3).
func (s *Store) DeleteInstance(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM instances WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
