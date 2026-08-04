package reconciler

import (
	"context"
	"fmt"
	"strings"

	"github.com/LKdev03/mini-tiny-cloud/internal/db"
	"github.com/LKdev03/mini-tiny-cloud/internal/docker"
)

// NOTE TO SELF: makes the same call for same instance in isInstanceHealthy and removeInstance which gains nothing. Okay for htis project but kinda sacrifices performance
func (r *Reconciler) reconcileService(ctx context.Context, service db.Service) error {
	instances, err := r.store.ListInstances(ctx, service.ID)
	if err != nil {
		return fmt.Errorf("list instances: %w", err)
	}

	var reconcileErr error
	healthy := make([]db.Instance, 0, len(instances))

	for _, instance := range instances {
		ok, err := r.isInstanceHealthy(ctx, service, instance)
		if err != nil {
			reconcileErr = err
			continue
		}
		if ok {
			healthy = append(healthy, instance)
			continue
		}

		if err := r.removeInstance(ctx, instance); err != nil {
			reconcileErr = fmt.Errorf("remove unhealthy instance %s: %w", instance.ID, err)
		}
	}

	for len(healthy) > service.Replicas {
		toRemove := healthy[len(healthy)-1]
		healthy = healthy[:len(healthy)-1]

		if err := r.removeInstance(ctx, toRemove); err != nil {
			reconcileErr = fmt.Errorf("scale down instance %s: %w", toRemove.ID, err)
			break
		}
	}

	needed := service.Replicas - len(healthy)
	for i := 0; i < needed; i++ {
		if _, err := r.createInstance(ctx, service, len(healthy)+i+1); err != nil {
			reconcileErr = fmt.Errorf("create instance: %w", err)
			break
		}
	}

	status := statusRunning
	if reconcileErr != nil {
		status = statusDegraded
	} else if service.Replicas > 0 {
		instances, err = r.store.ListInstances(ctx, service.ID)
		if err != nil {
			return fmt.Errorf("list instances after reconcile: %w", err)
		}
		if len(instances) < service.Replicas {
			status = statusPending
		}
	}

	if err := r.store.UpdateServiceStatus(ctx, service.ID, status); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return reconcileErr
}

// Kinda misleading name for what state it currently is
func (r *Reconciler) isInstanceHealthy(ctx context.Context, service db.Service, instance db.Instance) (bool, error) {
	runtime, err := r.docker.GetContainerRuntime(ctx, instance.ContainerID)
	if err != nil {
		return false, err
	}
	if !runtime.Exists {
		return false, nil
	}
	if !runtime.Running {
		return false, nil
	}
	if runtime.Labels[labelServiceID] != service.ID {
		return false, nil
	}
	if runtime.Labels[labelImage] != service.Image {
		return false, nil
	}
	return true, nil
}

func (r *Reconciler) createInstance(ctx context.Context, service db.Service, ordinal int) (db.Instance, error) {
	name := fmt.Sprintf("svc-%s-%d", shortID(service.ID), ordinal)
	labels := map[string]string{
		labelManaged:   "true",
		labelServiceID: service.ID,
		labelImage:     service.Image,
	}

	created, err := r.docker.CreateContainer(ctx, docker.CreateOptions{
		Name:   name,
		Image:  service.Image,
		Labels: labels,
	})
	if err != nil {
		return db.Instance{}, err
	}

	instance, err := r.store.CreateInstance(ctx, service.ID, created.ID, r.cfg.NodeID)
	if err != nil {
		_ = r.docker.DeleteContainer(ctx, created.ID)
		return db.Instance{}, err
	}

	return instance, nil
}

func (r *Reconciler) removeInstance(ctx context.Context, instance db.Instance) error {
	runtime, err := r.docker.GetContainerRuntime(ctx, instance.ContainerID)
	if err != nil {
		return err
	}
	if runtime.Exists {
		if err := r.docker.DeleteContainer(ctx, instance.ContainerID); err != nil {
			return err
		}
	}
	return r.store.DeleteInstance(ctx, instance.ID)
}

// DeleteServiceContainers stops and removes all Docker containers for a service.
// Instance rows are removed separately (e.g. by ON DELETE CASCADE when deleting the service).
func DeleteServiceContainers(ctx context.Context, dockerClient *docker.Client, store *db.Store, serviceID string) error {
	instances, err := store.ListInstances(ctx, serviceID)
	if err != nil {
		return err
	}

	for _, instance := range instances {
		runtime, err := dockerClient.GetContainerRuntime(ctx, instance.ContainerID)
		if err != nil {
			return err
		}
		if runtime.Exists {
			if err := dockerClient.DeleteContainer(ctx, instance.ContainerID); err != nil {
				return err
			}
		}
	}
	return nil
}

func shortID(id string) string {
	compact := strings.ReplaceAll(id, "-", "")
	if len(compact) > 8 {
		return compact[:8]
	}
	return compact
}
