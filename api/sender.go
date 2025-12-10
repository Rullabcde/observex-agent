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

type Sender struct {
	apiURL string
	apiKey string
	client *http.Client
	maxLogSize int
	compressLogs bool
}

func NewSender(apiURL, apiKey string) *Sender {
	return &Sender{
		apiURL: apiURL,
		apiKey: apiKey,
		maxLogSize: 400_000,     // 400 KB batas aman
		compressLogs: true,      // aktifkan kompresi log
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

	// ==============================
	// 1. LIMIT SIZE LOG SEBELUM KIRIM
	// ==============================
	if len(metric.Logs.System) > s.maxLogSize {
		metric.Logs.System = metric.Logs.System[:s.maxLogSize] + "\n[TRUNCATED]"
	}
	if len(metric.Logs.Error) > s.maxLogSize {
		metric.Logs.Error = metric.Logs.Error[:s.maxLogSize] + "\n[TRUNCATED]"
	}
	if len(metric.Logs.Security) > s.maxLogSize {
		metric.Logs.Security = metric.Logs.Security[:s.maxLogSize] + "\n[TRUNCATED]"
	}

	// =====================================
	// 2. CONVERT KE PAYLOAD (SUDAH ADA LOG)
	// =====================================
	payload := metric.ToPayload()

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// =====================================
	// 3. OPSI: KOMPRESI GZIP LOG JIKA BESAR
	// =====================================
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

	// =====================================
	// 4. SIAPKAN REQUEST HTTP
	// =====================================
	req, err := http.NewRequest("POST", s.apiURL, &requestBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.apiKey)

	// beri tahu API kalau payload gzip
	if s.compressLogs {
		req.Header.Set("Content-Encoding", "gzip")
	}

	// =====================================
	// 5. KIRIM REQUEST
	// =====================================
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// =====================================
	// 6. HANDLE ERROR API
	// =====================================
	if resp.StatusCode >= 400 {
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err == nil {
			return fmt.Errorf("[API ERROR] %d: %s (%s)", resp.StatusCode, apiResp.Error, apiResp.Code)
		}
		return fmt.Errorf("[API ERROR] %d: %s", resp.StatusCode, string(body))
	}

	// =====================================
	// 7. API SUCCESS RESPONSE
	// =====================================
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Success {
		log.Printf("Metrics + Logs sent successfully for agent: %s", apiResp.Agent)
	}

	return nil
}
