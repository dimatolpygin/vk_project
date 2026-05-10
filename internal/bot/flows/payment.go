package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/yukassa"
)

const referralBonusGens = 2

type paymentSettler interface {
	SettleSuccessfulPayment(ctx context.Context, paymentID string) (*repository.PaymentSettlementResult, error)
}

type paymentCanceler interface {
	CancelPayment(ctx context.Context, paymentID string) (*repository.PaymentCancellationResult, error)
}

func HandleShowTariffs(ctx context.Context, fc *Context, d *Deps) {
	if d.TariffRepo == nil {
		_ = sendScreen(ctx, d, fc.VkID, "payment_unavailable", ScreenOptions{})
		return
	}

	tariffs, err := d.TariffRepo.ListActive(ctx)
	if err != nil || len(tariffs) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "payment_unavailable", ScreenOptions{})
		return
	}

	_ = d.State.Set(ctx, fc.VkID, &State{
		Step:     StepTariffs,
		PrevStep: fc.State.Step,
		PhotoURL: fc.State.PhotoURL,
	})
	_ = sendScreen(ctx, d, fc.VkID, "tariffs", ScreenOptions{PrefixRows: tariffRows(tariffs)})
}

func HandleBuyTariff(ctx context.Context, fc *Context, d *Deps) {
	if d.TariffRepo == nil || d.OrderRepo == nil || d.Yukassa == nil {
		_ = sendScreen(ctx, d, fc.VkID, "payment_unavailable", ScreenOptions{})
		return
	}

	tariffID := fc.Callback.TariffID
	if tariffID == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "payment_invalid_tariff", ScreenOptions{})
		return
	}

	tariff, err := d.TariffRepo.GetByID(ctx, tariffID)
	if err != nil || tariff == nil {
		_ = sendScreen(ctx, d, fc.VkID, "payment_tariff_not_found", ScreenOptions{})
		return
	}

	returnURL := buildPaymentReturnURL(d.PublicBaseURL)
	if returnURL == "" {
		log.Error().
			Int64("vk_id", fc.VkID).
			Str("public_base_url", d.PublicBaseURL).
			Msg("public base url is not configured for payment return url")
		_ = sendScreen(ctx, d, fc.VkID, "payment_link_error", ScreenOptions{})
		return
	}

	order, err := d.OrderRepo.Create(ctx, fc.VkID, tariffID, tariff.Price)
	if err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("tariff_id", tariffID).Msg("failed to create payment order")
		_ = sendScreen(ctx, d, fc.VkID, "payment_order_error", ScreenOptions{})
		return
	}

	var (
		username  string
		firstName string
	)
	if d.UserRepo != nil {
		user, userErr := d.UserRepo.GetByVKID(ctx, fc.VkID)
		if userErr != nil {
			log.Warn().Err(userErr).Int64("vk_id", fc.VkID).Msg("failed to load user before creating yookassa payment")
		} else if user != nil {
			username = strings.TrimSpace(user.Username)
			firstName = strings.TrimSpace(user.FirstName)
		}
	}

	paymentDescription := fmt.Sprintf("\u0417\u0430\u043a\u0430\u0437 \u2116%d", order.ID)
	receiptDescription := strings.TrimSpace(tariff.Description)
	if receiptDescription == "" {
		receiptDescription = strings.TrimSpace(tariff.Name)
	}

	log.Info().
		Int64("order_id", order.ID).
		Int64("vk_id", fc.VkID).
		Int("tariff_id", tariffID).
		Str("return_url", returnURL).
		Msg("creating payment link")

	payment, err := d.Yukassa.CreatePayment(ctx, yukassa.PaymentRequest{
		Amount:                 tariff.Price,
		TariffID:               tariffID,
		UserVKID:               fc.VkID,
		OrderID:                order.ID,
		ReturnURL:              returnURL,
		Description:            paymentDescription,
		ReceiptItemDescription: receiptDescription,
		Username:               username,
		FirstName:              firstName,
	})
	if err != nil {
		log.Error().
			Err(err).
			Int64("order_id", order.ID).
			Int64("vk_id", fc.VkID).
			Int("tariff_id", tariffID).
			Msg("failed to create yookassa payment")
		_ = sendScreen(ctx, d, fc.VkID, "payment_link_error", ScreenOptions{})
		return
	}

	if err := d.OrderRepo.SetPaymentID(ctx, order.ID, payment.PaymentID); err != nil {
		log.Error().
			Err(err).
			Int64("order_id", order.ID).
			Str("payment_id", payment.PaymentID).
			Msg("failed to persist yookassa payment id")
		_ = sendScreen(ctx, d, fc.VkID, "payment_link_error", ScreenOptions{})
		return
	}

	log.Info().
		Int64("order_id", order.ID).
		Str("payment_id", payment.PaymentID).
		Str("payment_url", payment.PaymentURL).
		Str("return_url", returnURL).
		Msg("payment link created")

	_ = sendScreen(ctx, d, fc.VkID, "payment_checkout", ScreenOptions{
		Data: map[string]any{
			"TariffName":  tariff.Name,
			"Description": tariff.Description,
			"Price":       fmt.Sprintf("%.0f\u20bd", tariff.Price),
		},
		Links: map[string]string{"payment_url": payment.PaymentURL},
	})
}

func ProcessSuccessfulPayment(ctx context.Context, d *Deps, paymentID string) error {
	return processSuccessfulPayment(ctx, d, d.OrderRepo, paymentID)
}

func processSuccessfulPayment(ctx context.Context, d *Deps, settler paymentSettler, paymentID string) error {
	result, err := settler.SettleSuccessfulPayment(ctx, paymentID)
	if err != nil {
		return err
	}
	if result == nil || result.AlreadyProcessed {
		return nil
	}

	trackEvent(ctx, d, result.UserVKID, repository.ActivityEventPaymentSucceeded, "buy_tariff", "payment_success", map[string]any{
		"payment_id": paymentID,
		"tariff_id":  result.TariffID,
		"gens_count": result.PaidGensAdded,
	})

	if result.BonusGranted {
		trackEvent(ctx, d, result.ReferrerVKID, repository.ActivityEventReferralBonusAwarded, "referral", "referral_bonus_awarded", map[string]any{
			"referred_vk_id": result.UserVKID,
			"bonus_gens":     referralBonusGens,
		})
		_ = sendScreen(ctx, d, result.ReferrerVKID, "referral_bonus_awarded", ScreenOptions{})
	}

	_ = sendScreen(ctx, d, result.UserVKID, "payment_success", ScreenOptions{})
	return nil
}

func ProcessCanceledPayment(ctx context.Context, d *Deps, paymentID string) error {
	return processCanceledPayment(ctx, d, d.OrderRepo, paymentID)
}

func processCanceledPayment(ctx context.Context, d *Deps, canceler paymentCanceler, paymentID string) error {
	result, err := canceler.CancelPayment(ctx, paymentID)
	if err != nil {
		return err
	}
	if result == nil || result.AlreadyProcessed {
		return nil
	}

	_ = sendScreen(ctx, d, result.UserVKID, "payment_canceled", ScreenOptions{})
	return nil
}

func buildPaymentReturnURL(publicBaseURL string) string {
	publicBaseURL = strings.TrimSpace(publicBaseURL)
	if publicBaseURL == "" {
		return ""
	}
	return strings.TrimRight(publicBaseURL, "/") + "/vk/return"
}
