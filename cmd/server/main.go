package main

import (
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/think-root/threads-connector/internal/config"
	"github.com/think-root/threads-connector/internal/server"
	"github.com/think-root/threads-connector/internal/threads"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it")
	}

	cfg := config.Load()
	if cfg.ThreadsUserID == "" || cfg.ThreadsAccessToken == "" || cfg.APIKey == "" {
		log.Fatal("THREADS_USER_ID, THREADS_ACCESS_TOKEN, and API_KEY must be set")
	}

	client := threads.NewClient(cfg.ThreadsUserID, cfg.ThreadsAccessToken)

	// Validate access token at startup
	tokenInfo, err := client.ValidateToken()
	if err != nil {
		log.Printf("Failed to validate Threads access token: %v", err)
	} else if !tokenInfo.IsValid {
		log.Println("Threads access token is invalid!")
	} else {
		expiresAt := time.Unix(tokenInfo.ExpiresAt, 0)
		daysLeft := int(time.Until(expiresAt).Hours() / 24)
		log.Printf("Threads access token is valid (expires: %s, %d days remaining)",
			expiresAt.Format("2006-01-02"), daysLeft)
		// log.Printf("Token scopes: %v", tokenInfo.Scopes)
	}

	srv := server.New(cfg, client)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
