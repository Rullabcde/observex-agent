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

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Agent   string `json:"agent"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

func (s *Sender) SendMetrics(ctx context.Context, metric *models.Metric) error {

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
		return fmt.Errorf("marshal failed: %w", err)
	}

	// Gzip compression
	var requestBody bytes.Buffer

	if s.compressLogs {
		gzipWriter := gzip.NewWriter(&requestBody)
		if _, err := gzipWriter.Write(jsonData); err != nil {
			return fmt.Errorf("gzip write failed: %w", err)
		}
		if err := gzipWriter.Close(); err != nil {
			return fmt.Errorf("gzip close failed: %w", err)
		}
	} else {
		requestBody.Write(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, &requestBody)
	if err != nil {
		return fmt.Errorf("req creation failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.apiKey)

	if s.compressLogs {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Error response
	if resp.StatusCode >= 400 {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil {
			return fmt.Errorf("[API ERROR] %d: %s (%s)", resp.StatusCode, apiResp.Error, apiResp.Code)
		}
		return fmt.Errorf("[API ERROR] %d: %s", resp.StatusCode, string(body))
	}

	// Success log
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Success {
		log.Printf("Metrics sent for agent: %s", apiResp.Agent)
	}

	return nil
}
