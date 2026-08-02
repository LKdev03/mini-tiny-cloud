package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/LKdev03/mini-tiny-cloud/internal/api"
	"github.com/LKdev03/mini-tiny-cloud/internal/db"
	"github.com/LKdev03/mini-tiny-cloud/internal/docker"
)

func main() {
	ctx := context.Background()

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
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
