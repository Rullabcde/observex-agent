package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds agent configuration
type Config struct {
	APIKey       string
	APIURL       string
	SendInterval time.Duration
}

// Load reads config from environment variables
func Load() *Config {
	// Try loading .env file, otherwise use environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Parse interval, default 5 seconds
	intervalStr := os.Getenv("SEND_INTERVAL_SECONDS")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval < 1 {
		interval = 5
	}

	return &Config{
		APIKey:       getEnv("API_KEY", ""),
		APIURL:       getEnv("API_URL", ""),
		SendInterval: time.Duration(interval) * time.Second,
	}
}

// getEnv retrieves env var with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
