package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/think-root/threads-connector/internal/config"
	"github.com/think-root/threads-connector/internal/server"
	"github.com/think-root/threads-connector/internal/threads"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it")
	}

	// Load configuration
	cfg := config.Load()
	if cfg.ThreadsUserID == "" || cfg.ThreadsAccessToken == "" || cfg.APIKey == "" {
		log.Fatal("THREADS_USER_ID, THREADS_ACCESS_TOKEN, and API_KEY must be set")
	}

	// Initialize Threads client
	client := threads.NewClient(cfg.ThreadsUserID, cfg.ThreadsAccessToken)

	// Initialize and start server
	srv := server.New(cfg, client)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
