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

// Build info (di-inject pas build pake ldflags)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Load config dari env
	cfg := config.Load()

	// Validasi config wajib
	if cfg.APIKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}
	if cfg.APIURL == "" {
		log.Fatal("API_URL environment variable is required")
	}

	log.Printf("ObserveX Agent %s (%s) built on %s", version, commit, date)
	log.Printf("API URL: %s", cfg.APIURL)
	log.Printf("Send Interval: %v", cfg.SendInterval)

	// Bikin sender
	sender := api.NewSender(cfg.APIURL, cfg.APIKey)

	// Setup graceful shutdown (Ctrl+C, SIGTERM)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Jalanin collector di goroutine
	go runCollector(sender, cfg.SendInterval, stop)

	// Tunggu sampe ada signal shutdown
	<-stop
	log.Println("Shutting down agent...")
}

// runCollector loop kirim metrik tiap interval
func runCollector(sender *api.Sender, interval time.Duration, stop chan os.Signal) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Kirim langsung pas startup
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

// sendMetrics kumpulin + kirim metrik
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
