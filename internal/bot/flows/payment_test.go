package flows

import (
	"context"
	"testing"

	"vk_neuro_bot/internal/repository"
)

type fakePaymentSettler struct {
	paymentID string
	result    *repository.PaymentSettlementResult
	err       error
}

func (f *fakePaymentSettler) SettleSuccessfulPayment(_ context.Context, paymentID string) (*repository.PaymentSettlementResult, error) {
	f.paymentID = paymentID
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

	if err := processSuccessfulPayment(context.Background(), &Deps{Sender: sender}, settler, "pay_1"); err != nil {
		t.Fatalf("processSuccessfulPayment returned error: %v", err)
	}

	if settler.paymentID != "pay_1" {
		t.Fatalf("unexpected payment id: %+v", settler)
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

	if err := processSuccessfulPayment(context.Background(), &Deps{Sender: sender}, settler, "pay_2"); err != nil {
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
