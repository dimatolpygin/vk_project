package flows

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/yukassa"
)

const referralBonusGens = 2

type paymentSettler interface {
	SettleSuccessfulPayment(ctx context.Context, paymentID string, userVKID int64, tariffID int) (*repository.PaymentSettlementResult, error)
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
	if d.TariffRepo == nil {
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

	order, err := d.OrderRepo.Create(ctx, fc.VkID, tariffID, tariff.Price)
	if err != nil {
		log.Error().Err(err).Msg("РЅРµ СѓРґР°Р»РѕСЃСЊ СЃРѕР·РґР°С‚СЊ Р·Р°РєР°Р·")
		_ = sendScreen(ctx, d, fc.VkID, "payment_order_error", ScreenOptions{})
		return
	}

	payment, err := d.Yukassa.CreatePayment(ctx, yukassa.PaymentRequest{
		Amount:    tariff.Price,
		TariffID:  tariffID,
		UserVKID:  fc.VkID,
		OrderID:   order.ID,
		ReturnURL: fmt.Sprintf("%s/vk/return", d.BotWebhookURL),
	})
	if err != nil {
		log.Error().Err(err).Msg("РѕС€РёР±РєР° СЃРѕР·РґР°РЅРёСЏ РїР»Р°С‚РµР¶Р° Р®Kassa")
		_ = sendScreen(ctx, d, fc.VkID, "payment_link_error", ScreenOptions{})
		return
	}

	_ = d.OrderRepo.SetPaymentID(ctx, order.ID, payment.PaymentID)
	_ = sendScreen(ctx, d, fc.VkID, "payment_checkout", ScreenOptions{
		Data: map[string]any{
			"TariffName":  tariff.Name,
			"Description": tariff.Description,
			"Price":       fmt.Sprintf("%.0fв‚Ѕ", tariff.Price),
		},
		Links: map[string]string{"payment_url": payment.PaymentURL},
	})
}

func ProcessSuccessfulPayment(ctx context.Context, d *Deps, paymentID string, userVKID int64, tariffID int) error {
	return processSuccessfulPayment(ctx, d, d.OrderRepo, paymentID, userVKID, tariffID)
}

func processSuccessfulPayment(ctx context.Context, d *Deps, settler paymentSettler, paymentID string, userVKID int64, tariffID int) error {
	result, err := settler.SettleSuccessfulPayment(ctx, paymentID, userVKID, tariffID)
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
