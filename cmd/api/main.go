package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/LKdev03/mini-tiny-cloud/internal/api"
	"github.com/LKdev03/mini-tiny-cloud/internal/docker"
)

func main() {
	dockerClient, err := docker.New()
	if err != nil {
		log.Fatalf("docker client: %v", err)
	}
	defer dockerClient.Close()

	handlers := api.NewHandlers(dockerClient)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/containers", handlers.Containers)
	mux.HandleFunc("/containers/", handlers.ContainerByID)

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,  // slowloris protection on headers
		ReadTimeout:       15 * time.Second,  // enough for small JSON bodies
		WriteTimeout:      api.RequestTimeout + 10*time.Second, // >= handler timeout + buffer
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("mini-tiny-cloud API listening on %s", addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
