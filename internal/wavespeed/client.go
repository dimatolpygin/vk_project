package wavespeed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Эндпоинты взяты напрямую из документации WaveSpeed API.
var modelEndpoints = map[string]string{
	"google/nano-banana-pro": "https://api.wavespeed.ai/api/v3/google/nano-banana-pro/edit",
	"google/nano-banana-2":   "https://api.wavespeed.ai/api/v3/google/nano-banana-2/edit",
	"openai/gpt-image-2":     "https://api.wavespeed.ai/api/v3/openai/gpt-image-2/edit",
}

const pollBaseURL = "https://api.wavespeed.ai/api/v3"

type Client struct {
	apiKey string
	http   *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 60 * time.Second},
	}
}

type SubmitRequest struct {
	Images       []string `json:"images"`
	Prompt       string   `json:"prompt"`
	Model        string   `json:"-"`
	Resolution   string   `json:"resolution,omitempty"`
	OutputFormat string   `json:"output_format,omitempty"`
	WebhookURL   string   `json:"webhook_url,omitempty"`
}

type PredictionStatus struct {
	ID      string   `json:"id"`
	Status  string   `json:"status"`
	Outputs []string `json:"outputs"`
	Error   string   `json:"error"`
}

func (c *Client) Submit(ctx context.Context, req SubmitRequest) (string, error) {
	endpoint, ok := modelEndpoints[req.Model]
	if !ok {
		return "", fmt.Errorf("неизвестная модель WaveSpeed: %q", req.Model)
	}

	if req.Resolution == "" {
		req.Resolution = "1k"
	}
	if req.OutputFormat == "" {
		req.OutputFormat = "jpeg"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	fmt.Printf("[wavespeed submit] status=%d body=%s\n", resp.StatusCode, string(respBody))

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("не удалось распарсить ответ WaveSpeed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	if result.Data.ID == "" {
		return "", fmt.Errorf("WaveSpeed не вернул task_id (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return result.Data.ID, nil
}

func (c *Client) Poll(ctx context.Context, taskID string) (*PredictionStatus, error) {
	endpoint := fmt.Sprintf("%s/predictions/%s", pollBaseURL, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	fmt.Printf("[wavespeed poll] url=%s status=%d body=%s\n", endpoint, resp.StatusCode, string(body))

	var result struct {
		Data PredictionStatus `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("не удалось распарсить статус (HTTP %d): %s", resp.StatusCode, string(body))
	}
	return &result.Data, nil
}

// PollUntilDone опрашивает задачу до завершения с интервалом.
func (c *Client) PollUntilDone(ctx context.Context, taskID string, interval time.Duration, maxAttempts int) (*PredictionStatus, error) {
	for i := 0; i < maxAttempts; i++ {
		status, err := c.Poll(ctx, taskID)
		if err != nil {
			return nil, err
		}
		switch status.Status {
		case "completed", "succeeded":
			return status, nil
		case "failed":
			return status, fmt.Errorf("генерация завершилась ошибкой: %s", status.Error)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
	return nil, fmt.Errorf("превышено время ожидания генерации")
}
