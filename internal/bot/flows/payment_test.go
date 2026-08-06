package flows

import (
	"context"
	"strings"
	"testing"

	"vk_neuro_bot/internal/repository"
)

type fakePaymentSettler struct {
	paymentID    string
	paidGensHint int
	result       *repository.PaymentSettlementResult
	err          error
}

func (f *fakePaymentSettler) SettleSuccessfulPayment(_ context.Context, paymentID string, paidGensHint int) (*repository.PaymentSettlementResult, error) {
	f.paymentID = paymentID
	f.paidGensHint = paidGensHint
	return f.result, f.err
}

type fakePaymentCanceler struct {
	paymentID string
	result    *repository.PaymentCancellationResult
	err       error
}

func (f *fakePaymentCanceler) CancelPayment(_ context.Context, paymentID string) (*repository.PaymentCancellationResult, error) {
	f.paymentID = paymentID
	return f.result, f.err
}

func TestProcessSuccessfulPaymentSendsBonusAndSuccessScreens(t *testing.T) {
	sender := &fakeSender{}
	settler := &fakePaymentSettler{
		result: &repository.PaymentSettlementResult{
			PaymentID:     "pay_1",
			UserVKID:      55,
			TariffID:      3,
			PaidGensAdded: 30,
			BonusGranted:  true,
			ReferrerVKID:  11,
		},
	}

	if err := processSuccessfulPayment(context.Background(), &Deps{Sender: sender}, settler, "pay_1", map[string]any{"gens_count": "30"}); err != nil {
		t.Fatalf("processSuccessfulPayment returned error: %v", err)
	}

	if settler.paymentID != "pay_1" {
		t.Fatalf("unexpected payment id: %+v", settler)
	}
	if settler.paidGensHint != 30 {
		t.Fatalf("expected gens_count hint 30, got %d", settler.paidGensHint)
	}
	if len(sender.screens) != 2 {
		t.Fatalf("expected 2 screens, got %d", len(sender.screens))
	}
	if sender.screens[0].Key != "referral_bonus_awarded" {
		t.Fatalf("expected first screen referral bonus, got %q", sender.screens[0].Key)
	}
	if sender.screens[1].Key != "payment_success" {
		t.Fatalf("expected second screen payment success, got %q", sender.screens[1].Key)
	}
	if got := sender.screens[1].Text; !strings.Contains(got, "30") {
		t.Fatalf("expected paid gens count in success screen, got %q", got)
	}
}

func TestProcessSuccessfulPaymentIgnoresDuplicateWebhook(t *testing.T) {
	sender := &fakeSender{}
	settler := &fakePaymentSettler{
		result: &repository.PaymentSettlementResult{
			PaymentID:        "pay_2",
			UserVKID:         77,
			TariffID:         4,
			AlreadyProcessed: true,
		},
	}

	if err := processSuccessfulPayment(context.Background(), &Deps{Sender: sender}, settler, "pay_2", nil); err != nil {
		t.Fatalf("processSuccessfulPayment returned error: %v", err)
	}
	if len(sender.screens) != 0 {
		t.Fatalf("expected no screens for duplicate webhook, got %d", len(sender.screens))
	}
}

func TestProcessCanceledPaymentSendsCanceledScreen(t *testing.T) {
	sender := &fakeSender{}
	canceler := &fakePaymentCanceler{
		result: &repository.PaymentCancellationResult{
			PaymentID: "pay_3",
			UserVKID:  88,
			TariffID:  5,
		},
	}

	if err := processCanceledPayment(context.Background(), &Deps{Sender: sender}, canceler, "pay_3"); err != nil {
		t.Fatalf("processCanceledPayment returned error: %v", err)
	}
	if canceler.paymentID != "pay_3" {
		t.Fatalf("unexpected payment id: %+v", canceler)
	}
	if len(sender.screens) != 1 {
		t.Fatalf("expected 1 screen, got %d", len(sender.screens))
	}
	if sender.screens[0].Key != "payment_canceled" {
		t.Fatalf("expected payment_canceled screen, got %q", sender.screens[0].Key)
	}
}

func TestProcessCanceledPaymentIgnoresAlreadyProcessedWebhook(t *testing.T) {
	sender := &fakeSender{}
	canceler := &fakePaymentCanceler{
		result: &repository.PaymentCancellationResult{
			PaymentID:        "pay_4",
			UserVKID:         89,
			TariffID:         6,
			AlreadyProcessed: true,
		},
	}

	if err := processCanceledPayment(context.Background(), &Deps{Sender: sender}, canceler, "pay_4"); err != nil {
		t.Fatalf("processCanceledPayment returned error: %v", err)
	}
	if len(sender.screens) != 0 {
		t.Fatalf("expected no screens for duplicate canceled webhook, got %d", len(sender.screens))
	}
}

func TestBuildReferralLinkUsesCommunityBotDeepLink(t *testing.T) {
	got := buildReferralLink(229805415, "ref_qm7e6euq")
	want := "https://vk.me/club229805415?ref=ref_qm7e6euq"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildPaymentReturnURLUsesConfiguredGroup(t *testing.T) {
	got := buildPaymentReturnURL(238989543)
	want := "https://vk.com/write-238989543"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	if got := buildPaymentReturnURL(0); got != "" {
		t.Fatalf("expected empty return url for unset group id, got %q", got)
	}
}

func TestGensCountFromPaymentMetadataSupportsSeveralTypes(t *testing.T) {
	cases := []struct {
		name     string
		metadata map[string]any
		want     int
	}{
		{name: "string", metadata: map[string]any{"gens_count": "12"}, want: 12},
		{name: "int", metadata: map[string]any{"gens_count": 7}, want: 7},
		{name: "float", metadata: map[string]any{"gens_count": 5.0}, want: 5},
		{name: "invalid", metadata: map[string]any{"gens_count": "oops"}, want: 0},
	}

	for _, tc := range cases {
		if got := gensCountFromPaymentMetadata(tc.metadata); got != tc.want {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.want, got)
		}
	}
}

func TestBuildPayRedirectURLUsesPublicBase(t *testing.T) {
	cases := []struct {
		name  string
		base  string
		token string
		want  string
	}{
		{name: "обычный", base: "https://admplaya.ru", token: "abc123", want: "https://admplaya.ru/pay/abc123"},
		{name: "со слэшем", base: "https://admplaya.ru/", token: "abc123", want: "https://admplaya.ru/pay/abc123"},
		{name: "без базы", base: "", token: "abc123", want: ""},
		{name: "без токена", base: "https://admplaya.ru", token: "", want: ""},
	}

	for _, tc := range cases {
		if got := buildPayRedirectURL(tc.base, tc.token); got != tc.want {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestNewPayTokenIsRandomAndHex(t *testing.T) {
	seen := make(map[string]bool, 50)
	for i := 0; i < 50; i++ {
		token, err := newPayToken()
		if err != nil {
			t.Fatalf("newPayToken returned error: %v", err)
		}
		if len(token) != 32 {
			t.Fatalf("expected 32 hex chars, got %d (%q)", len(token), token)
		}
		if seen[token] {
			t.Fatalf("duplicate token generated: %q", token)
		}
		seen[token] = true
	}
}
