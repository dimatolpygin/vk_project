package wavespeed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Эндпоинты взяты напрямую из документации WaveSpeed API.
var modelEndpoints = map[string]string{
	"google/nano-banana-pro": "https://api.wavespeed.ai/api/v3/google/nano-banana-pro/edit",
	"google/nano-banana-2":   "https://api.wavespeed.ai/api/v3/google/nano-banana-2/edit",
	"openai/gpt-image-2":     "https://api.wavespeed.ai/api/v3/openai/gpt-image-2/edit",
	VideoModelSeedance:       "https://api.wavespeed.ai/api/v3/bytedance/seedance-2.0/image-to-video",
}

// VideoModelSeedance — image-to-video модель этапа 10.
const VideoModelSeedance = "bytedance/seedance-2.0/image-to-video"

// Параметры видео заданы заказчиком и пользователю не предлагаются.
const (
	VideoAspectRatio = "9:16"
	VideoResolution  = "720p"
	VideoDuration    = 10
)

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
	AspectRatio  string   `json:"aspect_ratio,omitempty"`
	Quality      string   `json:"quality,omitempty"`
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

	req.Images = normalizeSubmitImages(req.Images)
	if len(req.Images) == 0 {
		return "", fmt.Errorf("WaveSpeed submit: images is empty")
	}

	if req.Resolution == "" {
		req.Resolution = "1k"
	}
	if req.OutputFormat == "" {
		req.OutputFormat = "jpeg"
	}

	return c.submit(ctx, endpoint, req)
}

// SubmitVideoRequest — тело задачи image-to-video. От фото-запроса отличается
// всем: одно изображение вместо списка, длительность, звук.
type SubmitVideoRequest struct {
	Image         string `json:"image"`
	Prompt        string `json:"prompt"`
	AspectRatio   string `json:"aspect_ratio,omitempty"`
	Resolution    string `json:"resolution,omitempty"`
	Duration      int    `json:"duration,omitempty"`
	GenerateAudio bool   `json:"generate_audio"`
}

// SubmitVideo ставит задачу видео-модели. Изображение — уже готовая сцена,
// собранная фото-моделью первым звеном цепочки.
func (c *Client) SubmitVideo(ctx context.Context, req SubmitVideoRequest) (string, error) {
	endpoint, ok := modelEndpoints[VideoModelSeedance]
	if !ok {
		return "", fmt.Errorf("неизвестная видео-модель WaveSpeed: %q", VideoModelSeedance)
	}
	if strings.TrimSpace(req.Image) == "" {
		return "", fmt.Errorf("WaveSpeed submit video: image is empty")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return "", fmt.Errorf("WaveSpeed submit video: prompt is empty")
	}

	if req.AspectRatio == "" {
		req.AspectRatio = VideoAspectRatio
	}
	if req.Resolution == "" {
		req.Resolution = VideoResolution
	}
	if req.Duration <= 0 {
		req.Duration = VideoDuration
	}

	return c.submit(ctx, endpoint, req)
}

func (c *Client) submit(ctx context.Context, endpoint string, req any) (string, error) {
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

	fmt.Printf("[wavespeed submit] endpoint=%s status=%d body=%s\n", endpoint, resp.StatusCode, string(respBody))

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

func normalizeSubmitImages(images []string) []string {
	if len(images) == 0 {
		return nil
	}

	out := make([]string, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, image := range images {
		trimmed := strings.TrimSpace(image)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func (c *Client) Poll(ctx context.Context, taskID string) (*PredictionStatus, error) {
	endpoint := fmt.Sprintf("%s/predictions/%s/result", pollBaseURL, taskID)
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
		// Терминальных статусов у модели четыре, а не один: задача на минуты
		// вполне доживает до cancelled и timeout, и ждать после них нечего.
		case "failed", "cancelled", "timeout":
			return status, fmt.Errorf("генерация завершилась со статусом %s: %s", status.Status, status.Error)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
	return nil, fmt.Errorf("превышено время ожидания генерации")
}
