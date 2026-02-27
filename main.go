package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uptime-id/agent/api"
	"github.com/uptime-id/agent/collector"
	"github.com/uptime-id/agent/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cfg := config.Load()

	if cfg.APIKey == "" {
		log.Fatal("API_KEY required")
	}
	if cfg.APIURL == "" {
		log.Fatal("API_URL required")
	}

	log.Printf("UptimeID Agent %s (%s) built on %s", version, commit, date)
	log.Printf("API URL: %s", cfg.APIURL)
	log.Printf("Interval: %v", cfg.SendInterval)

	sender := api.NewSender(cfg, version)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runCollector(ctx, sender, cfg.SendInterval)

	sig := <-stop
	log.Printf("Received signal %v, shutting down gracefully...", sig)
	cancel()

	log.Println("Flushing final metrics...")
	flushCtx, flushCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer flushCancel()

	metric, err := collector.CollectMetrics()
	if err == nil {
		if _, err := sender.SendMetrics(flushCtx, metric); err != nil {
			log.Printf("Final flush failed: %v", err)
		} else {
			log.Println("Final metrics flushed successfully")
		}
	}

	log.Println("Agent stopped")
}

func runCollector(ctx context.Context, sender *api.Sender, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	currentInterval := interval

	newInterval := sendMetrics(ctx, sender)
	if newInterval > 0 && newInterval != currentInterval {
		log.Printf("Interval updated: %v -> %v", currentInterval, newInterval)
		currentInterval = newInterval
		ticker.Reset(currentInterval)
	}

	for {
		select {
		case <-ticker.C:
			newInterval := sendMetrics(ctx, sender)
			if newInterval > 0 && newInterval != currentInterval {
				log.Printf("Interval updated: %v -> %v", currentInterval, newInterval)
				currentInterval = newInterval
				ticker.Reset(currentInterval)
			}
		case <-ctx.Done():
			return
		}
	}
}

func sendMetrics(ctx context.Context, sender *api.Sender) time.Duration {
	metric, err := collector.CollectMetrics()
	if err != nil {
		log.Printf("Collection failed: %v", err)
		return 0
	}

	newInterval, err := sender.SendMetrics(ctx, metric)
	if err != nil {
		log.Printf("Send failed: %v", err)
		return 0
	}
	return newInterval
}
