package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/config"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/yukassa"
)

type fakeServerSender struct {
	screens []*flows.ScreenMessage
}

func (f *fakeServerSender) SendMsg(context.Context, int64, string, string) error { return nil }

func (f *fakeServerSender) SendText(context.Context, int64, string, string) error { return nil }

func (f *fakeServerSender) SendPhoto(context.Context, int64, string, string, string) error {
	return nil
}

func (f *fakeServerSender) SendScreen(_ context.Context, _ int64, screen *flows.ScreenMessage) error {
	if screen == nil {
		return nil
	}
	cloned := *screen
	f.screens = append(f.screens, &cloned)
	return nil
}

func (f *fakeServerSender) SendScreenText(context.Context, int64, string, map[string]any) error {
	return nil
}

func (f *fakeServerSender) SendPhotoResult(context.Context, int64, string, string, string, string) error {
	return nil
}

type fakeServerOrderStore struct {
	settleCalls  []string
	settleHints  []int
	cancelCalls  []string
	settleResult *repository.PaymentSettlementResult
	cancelResult *repository.PaymentCancellationResult
	settleErr    error
	cancelErr    error

	payTokenCalls []string
	payTokenOrder *repository.Order
	payTokenErr   error
}

func (f *fakeServerOrderStore) Create(context.Context, int64, int, float64) (*repository.Order, error) {
	return nil, nil
}

func (f *fakeServerOrderStore) SetPaymentID(context.Context, int64, string) error {
	return nil
}

func (f *fakeServerOrderStore) SetPaymentLink(context.Context, int64, string, string, string) error {
	return nil
}

func (f *fakeServerOrderStore) GetByPayToken(_ context.Context, payToken string) (*repository.Order, error) {
	f.payTokenCalls = append(f.payTokenCalls, payToken)
	return f.payTokenOrder, f.payTokenErr
}

func (f *fakeServerOrderStore) SettleSuccessfulPayment(_ context.Context, paymentID string, paidGensHint int) (*repository.PaymentSettlementResult, error) {
	f.settleCalls = append(f.settleCalls, paymentID)
	f.settleHints = append(f.settleHints, paidGensHint)
	return f.settleResult, f.settleErr
}

func (f *fakeServerOrderStore) CancelPayment(_ context.Context, paymentID string) (*repository.PaymentCancellationResult, error) {
	f.cancelCalls = append(f.cancelCalls, paymentID)
	return f.cancelResult, f.cancelErr
}

func TestHandleYukassaWebhookProcessesSucceededEvent(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/payments/pay_1" {
			t.Fatalf("unexpected verification request: %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "pay_1",
			"status": "succeeded",
			"metadata": map[string]any{
				"order_id":   "3",
				"gens_count": "10",
			},
		})
	}))
	defer api.Close()

	ykClient := yukassa.New("shop", "secret", "", "", 1, "service", "full_prepayment")
	ykClient.SetAPIBase(api.URL + "/v3")
	ykClient.SetHTTPClient(api.Client())

	sender := &fakeServerSender{}
	orderRepo := &fakeServerOrderStore{
		settleResult: &repository.PaymentSettlementResult{
			PaymentID:     "pay_1",
			UserVKID:      55,
			TariffID:      3,
			PaidGensAdded: 10,
		},
	}

	server := NewServer(&config.Config{}, nil, ykClient, &flows.Deps{
		Sender:    sender,
		OrderRepo: orderRepo,
	})

	body := `{"type":"notification","event":"payment.succeeded","object":{"id":"pay_1","status":"succeeded","metadata":{"order_id":"3","gens_count":"10"}}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/yukassa", strings.NewReader(body))
	rec := httptest.NewRecorder()

	server.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if len(orderRepo.settleCalls) != 1 || orderRepo.settleCalls[0] != "pay_1" {
		t.Fatalf("expected settle call for pay_1, got %#v", orderRepo.settleCalls)
	}
	if len(orderRepo.settleHints) != 1 || orderRepo.settleHints[0] != 10 {
		t.Fatalf("expected settle hint 10, got %#v", orderRepo.settleHints)
	}
	if len(orderRepo.cancelCalls) != 0 {
		t.Fatalf("expected no cancel calls, got %#v", orderRepo.cancelCalls)
	}
	if len(sender.screens) != 1 || sender.screens[0].Key != "payment_success" {
		t.Fatalf("expected payment_success screen, got %#v", sender.screens)
	}
	if !strings.Contains(sender.screens[0].Text, "10") {
		t.Fatalf("expected success screen to mention 10 generations, got %#v", sender.screens[0])
	}
}

func TestHandleYukassaWebhookProcessesCanceledEvent(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/payments/pay_2" {
			t.Fatalf("unexpected verification request: %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "pay_2",
			"status": "canceled",
			"metadata": map[string]any{
				"order_id": "4",
			},
		})
	}))
	defer api.Close()

	ykClient := yukassa.New("shop", "secret", "", "", 1, "service", "full_prepayment")
	ykClient.SetAPIBase(api.URL + "/v3")
	ykClient.SetHTTPClient(api.Client())

	sender := &fakeServerSender{}
	orderRepo := &fakeServerOrderStore{
		cancelResult: &repository.PaymentCancellationResult{
			PaymentID: "pay_2",
			UserVKID:  77,
			TariffID:  6,
		},
	}

	server := NewServer(&config.Config{}, nil, ykClient, &flows.Deps{
		Sender:    sender,
		OrderRepo: orderRepo,
	})

	body := `{"type":"notification","event":"payment.canceled","object":{"id":"pay_2","status":"canceled","metadata":{"order_id":"4"}}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/yukassa", strings.NewReader(body))
	rec := httptest.NewRecorder()

	server.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if len(orderRepo.cancelCalls) != 1 || orderRepo.cancelCalls[0] != "pay_2" {
		t.Fatalf("expected cancel call for pay_2, got %#v", orderRepo.cancelCalls)
	}
	if len(orderRepo.settleCalls) != 0 {
		t.Fatalf("expected no settle calls, got %#v", orderRepo.settleCalls)
	}
	if len(sender.screens) != 1 || sender.screens[0].Key != "payment_canceled" {
		t.Fatalf("expected payment_canceled screen, got %#v", sender.screens)
	}
}

func TestHandleVKReturnRendersSuccessPage(t *testing.T) {
	ykClient := yukassa.New("shop", "secret", "", "", 1, "service", "full_prepayment")
	server := NewServer(&config.Config{}, nil, ykClient, &flows.Deps{})

	req := httptest.NewRequest(http.MethodGet, "/vk/return", nil)
	rec := httptest.NewRecorder()

	server.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Оплата принята") {
		t.Fatalf("expected return page body, got %q", body)
	}
}

func TestHandlePayRedirectServesPaymentPage(t *testing.T) {
	paymentURL := "https://yoomoney.ru/checkout/payments/v2/contract?orderId=31f9998c"
	orderRepo := &fakeServerOrderStore{
		payTokenOrder: &repository.Order{
			ID:         594,
			UserVKID:   170333486,
			Amount:     90,
			Status:     "pending",
			PaymentURL: &paymentURL,
		},
	}

	server := NewServer(&config.Config{}, nil, nil, &flows.Deps{OrderRepo: orderRepo})

	req := httptest.NewRequest(http.MethodGet, "/pay/deadbeef", nil)
	rec := httptest.NewRecorder()
	server.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if len(orderRepo.payTokenCalls) != 1 || orderRepo.payTokenCalls[0] != "deadbeef" {
		t.Fatalf("expected lookup by token deadbeef, got %#v", orderRepo.payTokenCalls)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "href=\"https://yoomoney.ru/checkout/payments/v2/contract?orderId=31f9998c\"") {
		t.Fatalf("expected payment url in the button href, got: %s", body)
	}
	if !strings.Contains(body, "Перейти к оплате") {
		t.Fatalf("expected visible payment button, got: %s", body)
	}
	if !strings.Contains(body, "90") {
		t.Fatalf("expected order amount on the page, got: %s", body)
	}

	// Авто-редирект запрещён осознанно: он уносил человека на страницу ЮKassa
	// раньше, чем тот успевал увидеть кнопку, и при белом экране у него не
	// оставалось ни одного рабочего пути.
	for _, forbidden := range []string{"http-equiv=\"refresh\"", "location.replace", "<script"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("page must not auto-redirect, found %q in: %s", forbidden, body)
		}
	}
}

func TestHandlePayRedirectUnknownTokenReturns404(t *testing.T) {
	orderRepo := &fakeServerOrderStore{}
	server := NewServer(&config.Config{}, nil, nil, &flows.Deps{OrderRepo: orderRepo})

	req := httptest.NewRequest(http.MethodGet, "/pay/nope", nil)
	rec := httptest.NewRecorder()
	server.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}
