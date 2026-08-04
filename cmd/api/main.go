package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LKdev03/mini-tiny-cloud/internal/api"
	"github.com/LKdev03/mini-tiny-cloud/internal/db"
	"github.com/LKdev03/mini-tiny-cloud/internal/docker"
	"github.com/LKdev03/mini-tiny-cloud/internal/reconciler"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer store.Close()

	dockerClient, err := docker.New()
	if err != nil {
		log.Fatalf("docker client: %v", err)
	}
	defer dockerClient.Close()

	handlers := api.NewHandlers(dockerClient, store)

	rec := reconciler.New(store, dockerClient, reconciler.Config{})
	go rec.Run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/containers", handlers.Containers)
	mux.HandleFunc("/containers/", handlers.ContainerByID)
	mux.HandleFunc("/projects", handlers.Projects)
	mux.HandleFunc("/projects/", handlers.ProjectByID)
	mux.HandleFunc("/services", handlers.Services)
	mux.HandleFunc("/services/", handlers.ServiceByID)

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      api.RequestTimeout + 10*time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("mini-tiny-cloud API listening on %s", addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
