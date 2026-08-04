package reconciler

import (
	"context"
	"log"
	"time"

	"github.com/LKdev03/mini-tiny-cloud/internal/db"
	"github.com/LKdev03/mini-tiny-cloud/internal/docker"
)

const (
	defaultInterval  = 5 * time.Second
	defaultNodeID    = "local"
	reconcileTimeout = 2 * time.Minute
	statusPending    = "pending"
	statusRunning    = "running"
	statusDegraded   = "degraded"

	labelManaged   = "mini-cloud.managed"
	labelServiceID = "mini-cloud.service_id"
	labelImage     = "mini-cloud.image"
)

// Config controls how often reconciliation runs.
type Config struct {
	Interval time.Duration
	NodeID   string
}

// Reconciler compares desired service state in the database with Docker and converges them.
type Reconciler struct {
	store  *db.Store
	docker *docker.Client
	cfg    Config
}

// New builds a reconciler wired to the state store and Docker.
func New(store *db.Store, dockerClient *docker.Client, cfg Config) *Reconciler {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.NodeID == "" {
		cfg.NodeID = defaultNodeID
	}
	return &Reconciler{
		store:  store,
		docker: dockerClient,
		cfg:    cfg,
	}
}

// Run executes reconciliation on a fixed interval until ctx is cancelled.
func (r *Reconciler) Run(ctx context.Context) {
	log.Printf("reconciler: started (interval=%s, node=%s)", r.cfg.Interval, r.cfg.NodeID)

	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	r.reconcileAll(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Printf("reconciler: stopped")
			return
		case <-ticker.C:
			r.reconcileAll(ctx)
		}
	}
}

func (r *Reconciler) reconcileAll(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	services, err := r.store.ListServices(runCtx, "")
	if err != nil {
		log.Printf("reconciler: list services: %v", err)
		return
	}

	for _, service := range services {
		if err := r.reconcileService(runCtx, service); err != nil {
			log.Printf("reconciler: service %s: %v", service.ID, err)
		}
	}
}
