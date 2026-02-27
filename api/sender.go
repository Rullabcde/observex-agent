package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/uptime-id/agent/config"
	"github.com/uptime-id/agent/models"
)

const maxResponseBodySize = 1 * 1024 * 1024

type Sender struct {
	apiURL       string
	apiKey       string
	client       *http.Client
	maxLogSize   int
	compressLogs bool
	agentVersion string
	updateOnce   sync.Once
	retryMax     int
	retryBaseMs  int
}

func NewSender(cfg *config.Config, version string) *Sender {
	return &Sender{
		apiURL:       cfg.APIURL,
		apiKey:       cfg.APIKey,
		maxLogSize:   cfg.MaxLogSize,
		compressLogs: true,
		agentVersion: version,
		retryMax:     3,
		retryBaseMs:  500,
		client: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
	}
}

type APIConfig struct {
	Interval        int    `json:"interval"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable,omitempty"`
}

type APIResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message"`
	Agent   string    `json:"agent"`
	Config  APIConfig `json:"config"`
	Error   string    `json:"error,omitempty"`
	Code    string    `json:"code,omitempty"`
}

func (s *Sender) SendMetrics(ctx context.Context, metric *models.Metric) (time.Duration, error) {

	if len(metric.Logs.System) > s.maxLogSize {
		metric.Logs.System = metric.Logs.System[:s.maxLogSize] + "\n[TRUNCATED]"
	}

	if len(metric.Logs.Security) > s.maxLogSize {
		metric.Logs.Security = metric.Logs.Security[:s.maxLogSize] + "\n[TRUNCATED]"
	}

	payload := metric.ToPayload(s.agentVersion)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal failed: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= s.retryMax; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(float64(s.retryBaseMs)*math.Pow(2, float64(attempt-1))) * time.Millisecond
			log.Printf("Retry %d/%d after %v", attempt, s.retryMax, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}

		result, err := s.doSend(ctx, jsonData)
		if err != nil {
			lastErr = err
			if !isRetryable(err) {
				return 0, err
			}
			continue
		}
		return result, nil
	}

	return 0, fmt.Errorf("all %d retries exhausted: %w", s.retryMax, lastErr)
}

func isRetryable(err error) bool {
	errStr := err.Error()
	for _, code := range []string{"400", "401", "403", "404", "422"} {
		if len(errStr) > 12 && contains(errStr, code) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (s *Sender) doSend(ctx context.Context, jsonData []byte) (time.Duration, error) {
	var requestBody bytes.Buffer

	if s.compressLogs {
		gzipWriter := gzip.NewWriter(&requestBody)
		if _, err := gzipWriter.Write(jsonData); err != nil {
			return 0, fmt.Errorf("gzip write failed: %w", err)
		}
		if err := gzipWriter.Close(); err != nil {
			return 0, fmt.Errorf("gzip close failed: %w", err)
		}
	} else {
		requestBody.Write(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, &requestBody)
	if err != nil {
		return 0, fmt.Errorf("req creation failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.apiKey)

	if s.compressLogs {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, int64(maxResponseBodySize))
	body, _ := io.ReadAll(limitedReader)

	if resp.StatusCode >= 400 {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil {
			return 0, fmt.Errorf("[API ERROR] %d: %s (%s)", resp.StatusCode, apiResp.Error, apiResp.Code)
		}
		return 0, fmt.Errorf("[API ERROR] %d: %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Success {
		log.Printf("Metrics sent for agent: %s", apiResp.Agent)

		if apiResp.Config.UpdateAvailable && apiResp.Config.LatestVersion != "" {
			s.updateOnce.Do(func() {
				log.Printf("⚠️  UPDATE AVAILABLE: Current=%s, Latest=%s. Download: https://github.com/uptime-id/agent/releases/latest",
					s.agentVersion, apiResp.Config.LatestVersion)
			})
		}

		if apiResp.Config.Interval > 0 {
			return time.Duration(apiResp.Config.Interval) * time.Second, nil
		}
	}

	return 0, nil
}
