package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"observex-agent/models"
)

// Sender sends metrics to API
type Sender struct {
	apiURL       string
	apiKey       string
	client       *http.Client
	maxLogSize   int
	compressLogs bool
}

// NewSender creates a new sender instance
func NewSender(apiURL, apiKey string) *Sender {
	return &Sender{
		apiURL:       apiURL,
		apiKey:       apiKey,
		maxLogSize:   400_000, // 400 KB limit
		compressLogs: true,   // enable gzip
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// APIResponse defines server response
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Agent   string `json:"agent"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

// SendMetrics sends metrics to the API server
func (s *Sender) SendMetrics(metric *models.Metric) error {

	// Truncate large logs
	if len(metric.Logs.System) > s.maxLogSize {
		metric.Logs.System = metric.Logs.System[:s.maxLogSize] + "\n[TRUNCATED]"
	}

	if len(metric.Logs.Security) > s.maxLogSize {
		metric.Logs.Security = metric.Logs.Security[:s.maxLogSize] + "\n[TRUNCATED]"
	}

	// Convert to payload format
	payload := metric.ToPayload()

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Compress with gzip
	var requestBody bytes.Buffer

	if s.compressLogs {
		gzipWriter := gzip.NewWriter(&requestBody)
		_, err := gzipWriter.Write(jsonData)
		gzipWriter.Close()

		if err != nil {
			return fmt.Errorf("failed to gzip payload: %w", err)
		}
	} else {
		requestBody.Write(jsonData)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", s.apiURL, &requestBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.apiKey)

	// Set gzip header if enabled
	if s.compressLogs {
		req.Header.Set("Content-Encoding", "gzip")
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Handle API error
	if resp.StatusCode >= 400 {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil {
			return fmt.Errorf("[API ERROR] %d: %s (%s)", resp.StatusCode, apiResp.Error, apiResp.Code)
		}
		return fmt.Errorf("[API ERROR] %d: %s", resp.StatusCode, string(body))
	}

	// Log success
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Success {
		log.Printf("Metrics + Logs sent successfully for agent: %s", apiResp.Agent)
	}

	return nil
}
