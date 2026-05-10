package yukassa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreatePaymentIncludesReceiptAndReliableMetadata(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   map[string]any
	)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		defer r.Body.Close()

		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "pay_1",
			"status": "pending",
			"confirmation": map[string]any{
				"confirmation_url": "https://pay.example/confirm",
			},
		})
	}))
	defer api.Close()

	client := New("shop", "secret", "", "billing@example.com", 1, "service", "full_prepayment")
	client.SetAPIBase(api.URL)
	client.SetHTTPClient(api.Client())

	resp, err := client.CreatePayment(context.Background(), PaymentRequest{
		Amount:                 20000,
		TariffID:               1,
		GensCount:              30,
		UserVKID:               268215774,
		OrderID:                1778392748175,
		ReturnURL:              "https://sol-dobra.ru/vk/return",
		Description:            "Заказ №1778392748175",
		ReceiptItemDescription: "оплата услуги по созданию бота(автоматизации)",
		Username:               "rumynskiy545",
		FirstName:              "Aleksandr",
	})
	if err != nil {
		t.Fatalf("CreatePayment returned error: %v", err)
	}

	if gotMethod != http.MethodPost || gotPath != "/payments" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if resp.PaymentID != "pay_1" || resp.PaymentURL != "https://pay.example/confirm" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if gotDescription := gotBody["description"]; gotDescription != "Заказ №1778392748175" {
		t.Fatalf("unexpected description: %#v", gotDescription)
	}

	confirmation, ok := gotBody["confirmation"].(map[string]any)
	if !ok {
		t.Fatalf("confirmation is missing or invalid: %#v", gotBody["confirmation"])
	}
	if gotReturnURL := confirmation["return_url"]; gotReturnURL != "https://sol-dobra.ru/vk/return" {
		t.Fatalf("unexpected return_url: %#v", gotReturnURL)
	}

	metadata, ok := gotBody["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata is missing or invalid: %#v", gotBody["metadata"])
	}
	if metadata["order_id"] != "1778392748175" || metadata["tariff_id"] != "1" || metadata["gens_count"] != "30" || metadata["vk_id"] != "268215774" {
		t.Fatalf("unexpected reliable metadata: %#v", metadata)
	}
	if metadata["username"] != "rumynskiy545" || metadata["first_name"] != "Aleksandr" {
		t.Fatalf("unexpected optional metadata: %#v", metadata)
	}

	receipt, ok := gotBody["receipt"].(map[string]any)
	if !ok {
		t.Fatalf("receipt is missing or invalid: %#v", gotBody["receipt"])
	}
	customer, ok := receipt["customer"].(map[string]any)
	if !ok || customer["email"] != "billing@example.com" {
		t.Fatalf("unexpected receipt customer: %#v", receipt["customer"])
	}
	items, ok := receipt["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected receipt items: %#v", receipt["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected receipt item: %#v", items[0])
	}
	if item["description"] != "оплата услуги по созданию бота(автоматизации)" {
		t.Fatalf("unexpected item description: %#v", item["description"])
	}
	if item["payment_subject"] != "service" || item["payment_mode"] != "full_prepayment" {
		t.Fatalf("unexpected receipt flags: %#v", item)
	}
}

func TestParseWebhookReadsEventAndType(t *testing.T) {
	client := New("shop", "secret", "", "", 1, "service", "full_prepayment")

	event, err := client.ParseWebhook([]byte(`{
		"type":"notification",
		"event":"payment.succeeded",
		"object":{
			"id":"pay_42",
			"status":"succeeded",
			"metadata":{"order_id":"10"}
		}
	}`))
	if err != nil {
		t.Fatalf("ParseWebhook returned error: %v", err)
	}

	if event.Type != "notification" {
		t.Fatalf("unexpected type: %q", event.Type)
	}
	if event.Event != "payment.succeeded" {
		t.Fatalf("unexpected event: %q", event.Event)
	}
	if event.Object.ID != "pay_42" || event.Object.Status != "succeeded" {
		t.Fatalf("unexpected object: %+v", event.Object)
	}
}
