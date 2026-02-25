package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"observex-agent/config"
	"observex-agent/models"
)

type Sender struct {
	apiURL       string
	apiKey       string
	client       *http.Client
	maxLogSize   int
	compressLogs bool
	agentVersion string
	updateOnce   sync.Once
}

func NewSender(cfg *config.Config, version string) *Sender {
	return &Sender{
		apiURL:       cfg.APIURL,
		apiKey:       cfg.APIKey,
		maxLogSize:   cfg.MaxLogSize,
		compressLogs: true,
		agentVersion: version,
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

// SendMetrics sends metrics to the API and returns the new polling interval if provided.
func (s *Sender) SendMetrics(ctx context.Context, metric *models.Metric) (time.Duration, error) {

	if len(metric.Logs.System) > s.maxLogSize {
		metric.Logs.System = metric.Logs.System[:s.maxLogSize] + "\n[TRUNCATED]"
	}

	if len(metric.Logs.Security) > s.maxLogSize {
		metric.Logs.Security = metric.Logs.Security[:s.maxLogSize] + "\n[TRUNCATED]"
	}

	payload := metric.ToPayload(s.agentVersion)

	// Marshal JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal failed: %w", err)
	}

	// Gzip compression
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

	body, _ := io.ReadAll(resp.Body)

	// Error response
	if resp.StatusCode >= 400 {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil {
			return 0, fmt.Errorf("[API ERROR] %d: %s (%s)", resp.StatusCode, apiResp.Error, apiResp.Code)
		}
		return 0, fmt.Errorf("[API ERROR] %d: %s", resp.StatusCode, string(body))
	}

	// Success: parse response for config
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Success {
		log.Printf("Metrics sent for agent: %s", apiResp.Agent)

		// Log update warning once per session
		if apiResp.Config.UpdateAvailable && apiResp.Config.LatestVersion != "" {
			s.updateOnce.Do(func() {
				log.Printf("⚠️  UPDATE AVAILABLE: Current=%s, Latest=%s. Download: https://github.com/Rullabcde/observex-agent/releases/latest",
					s.agentVersion, apiResp.Config.LatestVersion)
			})
		}

		if apiResp.Config.Interval > 0 {
			return time.Duration(apiResp.Config.Interval) * time.Second, nil
		}
	}

	return 0, nil
}
