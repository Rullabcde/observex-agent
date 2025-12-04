package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"observex-agent/models"
)

type Sender struct {
	apiURL string
	apiKey string
	client *http.Client
}

func NewSender(apiURL, apiKey string) *Sender {
	return &Sender{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
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

func (s *Sender) SendMetrics(metric *models.Metric) error {
	payload := metric.ToPayload()
	
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil {
			return fmt.Errorf("API error (%d): %s [%s]", resp.StatusCode, apiResp.Error, apiResp.Code)
		}
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Success {
		log.Printf("Metrics sent successfully for agent: %s", apiResp.Agent)
	}

	return nil
}
