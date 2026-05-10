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
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const defaultAPIBase = "https://api.yookassa.ru/v3"

type Client struct {
	shopID                string
	secretKey             string
	webhook               string
	apiBase               string
	http                  *http.Client
	receiptEmail          string
	receiptVATCode        int
	receiptPaymentSubject string
	receiptPaymentMode    string
}

func New(
	shopID,
	secretKey,
	webhookSecret,
	receiptEmail string,
	receiptVATCode int,
	receiptPaymentSubject,
	receiptPaymentMode string,
) *Client {
	return &Client{
		shopID:                shopID,
		secretKey:             secretKey,
		webhook:               webhookSecret,
		apiBase:               defaultAPIBase,
		http:                  &http.Client{Timeout: 15 * time.Second},
		receiptEmail:          strings.TrimSpace(receiptEmail),
		receiptVATCode:        receiptVATCode,
		receiptPaymentSubject: strings.TrimSpace(receiptPaymentSubject),
		receiptPaymentMode:    strings.TrimSpace(receiptPaymentMode),
	}
}

func (c *Client) SetAPIBase(apiBase string) {
	apiBase = strings.TrimSpace(apiBase)
	if apiBase == "" {
		c.apiBase = defaultAPIBase
		return
	}
	c.apiBase = strings.TrimRight(apiBase, "/")
}

func (c *Client) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.http = client
	}
}

type PaymentRequest struct {
	Amount                 float64
	TariffID               int
	GensCount              int
	UserVKID               int64
	OrderID                int64
	ReturnURL              string
	Description            string
	ReceiptItemDescription string
	Username               string
	FirstName              string
}

type PaymentResponse struct {
	PaymentID  string
	PaymentURL string
	HTTPStatus int
}

func (c *Client) CreatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	idempotencyKey := fmt.Sprintf("order-%d-%d", req.OrderID, time.Now().UnixNano())
	payload := c.buildCreatePaymentPayload(req)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("idempotency_key", idempotencyKey).
		Float64("amount", req.Amount).
		Int64("order_id", req.OrderID).
		Int64("user_vk_id", req.UserVKID).
		Int("tariff_id", req.TariffID).
		Str("return_url", req.ReturnURL).
		Bool("receipt_enabled", payload.Receipt != nil).
		Any("metadata", payload.Metadata).
		Msg("creating yookassa payment")

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/payments", bytes.NewReader(body))
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
		ID           string           `json:"id"`
		Status       string           `json:"status"`
		Confirmation paymentConfirm   `json:"confirmation"`
		Error        *apiErrorPayload `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("yookassa create payment: parse response: %s", string(respBody))
	}

	log.Info().
		Int("http_status", resp.StatusCode).
		Str("payment_id", result.ID).
		Str("payment_status", result.Status).
		Msg("yookassa create payment response")

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("yookassa create payment: %s", apiErrorDescription(result.Error, respBody))
	}
	if result.ID == "" || result.Confirmation.ConfirmationURL == "" {
		return nil, fmt.Errorf("yookassa create payment: missing payment id or confirmation url")
	}

	return &PaymentResponse{
		PaymentID:  result.ID,
		PaymentURL: result.Confirmation.ConfirmationURL,
		HTTPStatus: resp.StatusCode,
	}, nil
}

type PaymentDetails struct {
	ID       string         `json:"id"`
	Status   string         `json:"status"`
	Metadata map[string]any `json:"metadata"`
}

func (c *Client) GetPayment(ctx context.Context, paymentID string) (*PaymentDetails, error) {
	if strings.TrimSpace(paymentID) == "" {
		return nil, fmt.Errorf("payment id is required")
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.apiBase+"/payments/"+url.PathEscape(paymentID),
		nil,
	)
	if err != nil {
		return nil, err
	}
	httpReq.SetBasicAuth(c.shopID, c.secretKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		ID       string           `json:"id"`
		Status   string           `json:"status"`
		Metadata map[string]any   `json:"metadata"`
		Error    *apiErrorPayload `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("yookassa get payment: parse response: %s", string(respBody))
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("yookassa get payment: %s", apiErrorDescription(result.Error, respBody))
	}
	if result.ID == "" {
		return nil, fmt.Errorf("yookassa get payment: empty payment id in response")
	}

	return &PaymentDetails{
		ID:       result.ID,
		Status:   result.Status,
		Metadata: result.Metadata,
	}, nil
}

type WebhookEvent struct {
	Type   string `json:"type"`
	Event  string `json:"event"`
	Object struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Amount struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
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

// VerifyWebhookSignature checks the optional HMAC-SHA256 webhook signature.
func (c *Client) VerifyWebhookSignature(header string, body []byte) error {
	if c.webhook == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(c.webhook))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(header), []byte(expected)) {
		return fmt.Errorf("invalid yookassa webhook signature")
	}
	return nil
}

type createPaymentPayload struct {
	Amount       paymentAmount       `json:"amount"`
	Capture      bool                `json:"capture"`
	Confirmation paymentConfirm      `json:"confirmation"`
	Description  string              `json:"description"`
	Metadata     map[string]string   `json:"metadata"`
	Receipt      *paymentReceiptBody `json:"receipt,omitempty"`
}

type paymentAmount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type paymentConfirm struct {
	Type            string `json:"type"`
	ReturnURL       string `json:"return_url,omitempty"`
	ConfirmationURL string `json:"confirmation_url,omitempty"`
}

type paymentReceiptBody struct {
	Customer paymentReceiptCustomer `json:"customer"`
	Items    []paymentReceiptItem   `json:"items"`
}

type paymentReceiptCustomer struct {
	Email string `json:"email"`
}

type paymentReceiptItem struct {
	Description    string        `json:"description"`
	Quantity       string        `json:"quantity"`
	Amount         paymentAmount `json:"amount"`
	VATCode        int           `json:"vat_code"`
	PaymentSubject string        `json:"payment_subject"`
	PaymentMode    string        `json:"payment_mode"`
}

type apiErrorPayload struct {
	Type        string `json:"type"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

func (c *Client) buildCreatePaymentPayload(req PaymentRequest) createPaymentPayload {
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = fmt.Sprintf("\u0417\u0430\u043a\u0430\u0437 \u2116%d", req.OrderID)
	}

	payload := createPaymentPayload{
		Amount: paymentAmount{
			Value:    moneyValue(req.Amount),
			Currency: "RUB",
		},
		Capture: true,
		Confirmation: paymentConfirm{
			Type:      "redirect",
			ReturnURL: req.ReturnURL,
		},
		Description: description,
		Metadata: map[string]string{
			"order_id":   fmt.Sprintf("%d", req.OrderID),
			"tariff_id":  fmt.Sprintf("%d", req.TariffID),
			"gens_count": fmt.Sprintf("%d", req.GensCount),
			"vk_id":      fmt.Sprintf("%d", req.UserVKID),
		},
	}

	if username := strings.TrimSpace(req.Username); username != "" {
		payload.Metadata["username"] = username
	}
	if firstName := strings.TrimSpace(req.FirstName); firstName != "" {
		payload.Metadata["first_name"] = firstName
	}

	if strings.TrimSpace(c.receiptEmail) != "" {
		receiptDescription := strings.TrimSpace(req.ReceiptItemDescription)
		if receiptDescription == "" {
			receiptDescription = description
		}

		payload.Receipt = &paymentReceiptBody{
			Customer: paymentReceiptCustomer{
				Email: c.receiptEmail,
			},
			Items: []paymentReceiptItem{
				{
					Description: receiptDescription,
					Quantity:    "1.0",
					Amount: paymentAmount{
						Value:    moneyValue(req.Amount),
						Currency: "RUB",
					},
					VATCode:        c.receiptVATCode,
					PaymentSubject: fallbackReceiptValue(c.receiptPaymentSubject, "service"),
					PaymentMode:    fallbackReceiptValue(c.receiptPaymentMode, "full_prepayment"),
				},
			},
		}
	}

	return payload
}

func apiErrorDescription(apiErr *apiErrorPayload, rawBody []byte) string {
	if apiErr != nil && strings.TrimSpace(apiErr.Description) != "" {
		return apiErr.Description
	}
	return string(rawBody)
}

func moneyValue(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

func fallbackReceiptValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
