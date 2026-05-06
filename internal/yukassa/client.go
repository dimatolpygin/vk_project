package yukassa

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiBase = "https://api.yookassa.ru/v3"

type Client struct {
	shopID    string
	secretKey string
	webhook   string
	http      *http.Client
}

func New(shopID, secretKey, webhookSecret string) *Client {
	return &Client{
		shopID:    shopID,
		secretKey: secretKey,
		webhook:   webhookSecret,
		http:      &http.Client{Timeout: 15 * time.Second},
	}
}

type PaymentRequest struct {
	Amount    float64
	TariffID  int
	UserVKID  int64
	OrderID   int64
	ReturnURL string
}

type PaymentResponse struct {
	PaymentID  string
	PaymentURL string
}

func (c *Client) CreatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	idempotencyKey := fmt.Sprintf("order-%d-%d", req.OrderID, time.Now().UnixNano())

	body, _ := json.Marshal(map[string]any{
		"amount": map[string]any{
			"value":    fmt.Sprintf("%.2f", req.Amount),
			"currency": "RUB",
		},
		"payment_method_data": map[string]string{"type": "bank_card"},
		"confirmation": map[string]string{
			"type":       "redirect",
			"return_url": req.ReturnURL,
		},
		"capture": true,
		"metadata": map[string]any{
			"vk_id":     req.UserVKID,
			"tariff_id": req.TariffID,
			"order_id":  req.OrderID,
		},
		"description": fmt.Sprintf("Покупка тарифа #%d", req.TariffID),
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/payments", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.SetBasicAuth(c.shopID, c.secretKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotence-Key", idempotencyKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		ID           string `json:"id"`
		Confirmation struct {
			ConfirmationURL string `json:"confirmation_url"`
		} `json:"confirmation"`
		Error struct {
			Description string `json:"description"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("ЮKassa: ошибка парсинга ответа: %s", string(respBody))
	}
	if result.ID == "" {
		return nil, fmt.Errorf("ЮKassa: %s", result.Error.Description)
	}
	return &PaymentResponse{
		PaymentID:  result.ID,
		PaymentURL: result.Confirmation.ConfirmationURL,
	}, nil
}

type WebhookEvent struct {
	Type   string `json:"type"`
	Object struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Amount struct {
			Value string `json:"value"`
		} `json:"amount"`
		Metadata map[string]any `json:"metadata"`
	} `json:"object"`
}

func (c *Client) ParseWebhook(body []byte) (*WebhookEvent, error) {
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// VerifyWebhookSignature проверяет HMAC-SHA256 подпись вебхука ЮKassa.
func (c *Client) VerifyWebhookSignature(header string, body []byte) error {
	if c.webhook == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(c.webhook))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(header), []byte(expected)) {
		return fmt.Errorf("неверная подпись вебхука ЮKassa")
	}
	return nil
}
