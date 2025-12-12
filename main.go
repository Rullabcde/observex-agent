package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"observex-agent/api"
	"observex-agent/collector"
	"observex-agent/config"
)

// Build info (injected via ldflags)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Load config from env
	cfg := config.Load()

	// Validate required config
	if cfg.APIKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}
	if cfg.APIURL == "" {
		log.Fatal("API_URL environment variable is required")
	}

	log.Printf("ObserveX Agent %s (%s) built on %s", version, commit, date)
	log.Printf("API URL: %s", cfg.APIURL)
	log.Printf("Send Interval: %v", cfg.SendInterval)

	// Initialize sender
	sender := api.NewSender(cfg.APIURL, cfg.APIKey)

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Start collector in goroutine
	go runCollector(sender, cfg.SendInterval, stop)

	// Wait for shutdown signal
	<-stop
	log.Println("Shutting down agent...")
}

// runCollector loops to send metrics at valid intervals
func runCollector(sender *api.Sender, interval time.Duration, stop chan os.Signal) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send immediately on startup
	sendMetrics(sender)

	for {
		select {
		case <-ticker.C:
			sendMetrics(sender)
		case <-stop:
			return
		}
	}
}

// sendMetrics collects and sends metrics
func sendMetrics(sender *api.Sender) {
	metric, err := collector.CollectMetrics()
	if err != nil {
		log.Printf("Failed to collect metrics: %v", err)
		return
	}

	if err := sender.SendMetrics(metric); err != nil {
		log.Printf("Failed to send metrics: %v", err)
	}
}
