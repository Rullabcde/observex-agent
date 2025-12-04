package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	APIKey       string
	APIURL       string
	SendInterval time.Duration
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	intervalStr := os.Getenv("SEND_INTERVAL_SECONDS")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval < 1 {
		interval = 5 // Default
	}

	return &Config{
		APIKey:       getEnv("API_KEY", ""),
		APIURL:       getEnv("API_URL", ""),
		SendInterval: time.Duration(interval) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
