package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"observex-agent/api"
	"observex-agent/collector"
	"observex-agent/config"
)

// Build info
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Load config
	cfg := config.Load()

	// Validate config
	if cfg.APIKey == "" {
		log.Fatal("API_KEY required")
	}
	if cfg.APIURL == "" {
		log.Fatal("API_URL required")
	}

	log.Printf("ObserveX Agent %s (%s) built on %s", version, commit, date)
	log.Printf("API URL: %s", cfg.APIURL)
	log.Printf("Interval: %v", cfg.SendInterval)

	// Init sender
	sender := api.NewSender(cfg, version)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Run collector
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runCollector(ctx, sender, cfg.SendInterval)

	// Wait for signal
	<-stop
	log.Println("Shutting down...")
	cancel()
	time.Sleep(1 * time.Second) // Give time for cleanup if needed
}

// runCollector loop
func runCollector(ctx context.Context, sender *api.Sender, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send immediately
	sendMetrics(ctx, sender)

	for {
		select {
		case <-ticker.C:
			sendMetrics(ctx, sender)
		case <-ctx.Done():
			return
		}
	}
}

// sendMetrics invoke collection and send
func sendMetrics(ctx context.Context, sender *api.Sender) {
	metric, err := collector.CollectMetrics()
	if err != nil {
		log.Printf("Collection failed: %v", err)
		return
	}

	if err := sender.SendMetrics(ctx, metric); err != nil {
		log.Printf("Send failed: %v", err)
	}
}
