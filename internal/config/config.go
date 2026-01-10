package config

import (
	"os"
)

type Config struct {
	ThreadsUserID      string
	ThreadsAccessToken string
	Port               string
	APIKey             string
}

func Load() *Config {
	return &Config{
		ThreadsUserID:      getEnv("THREADS_USER_ID", ""),
		ThreadsAccessToken: getEnv("THREADS_ACCESS_TOKEN", ""),
		Port:               getEnv("PORT", "8080"),
		APIKey:             getEnv("API_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
