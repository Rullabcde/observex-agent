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
	MaxLogSize   int
	HTTPTimeout  time.Duration
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env found, using env vars")
	}
	parseInt := func(key string, def int) int {
		if v := os.Getenv(key); v != "" {
			if i, err := strconv.Atoi(v); err == nil && i > 0 {
				return i
			}
		}
		return def
	}

	return &Config{
		APIKey:       getEnv("API_KEY", ""),
		APIURL:       getEnv("API_URL", ""),
		SendInterval: time.Duration(parseInt("SEND_INTERVAL_SECONDS", 5)) * time.Second,
		MaxLogSize:   parseInt("MAX_LOG_SIZE_BYTES", 400_000), // Default 400KB
		HTTPTimeout:  time.Duration(parseInt("HTTP_TIMEOUT_SECONDS", 10)) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
