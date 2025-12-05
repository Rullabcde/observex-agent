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

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cfg := config.Load()

	if cfg.APIKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}

	if cfg.APIURL == "" {
		log.Fatal("API_URL environment variable is required")
	}

	log.Printf("ObserveX Agent %s (%s) built on %s", version, commit, date)
	log.Printf("API URL: %s", cfg.APIURL)
	log.Printf("Send Interval: %v", cfg.SendInterval)

	sender := api.NewSender(cfg.APIURL, cfg.APIKey)

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Start metrics collection loop
	go runCollector(sender, cfg.SendInterval, stop)

	// Wait for shutdown signal
	<-stop
	log.Println("Shutting down agent...")
}

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
